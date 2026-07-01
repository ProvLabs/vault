package keeper_test

import (
	"fmt"
	"math"
	"math/big"
	"time"

	"cosmossdk.io/collections"
	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	markertypes "github.com/provenance-io/provenance/x/marker/types"

	"github.com/provlabs/vault/interest"
	"github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/types"
)

func (s *TestSuite) TestKeeper_ReconcileVault() {
	shareDenom := "vaultshares"
	underlying := sdk.NewInt64Coin("underlying", 1_000_000_000)
	totalShares := sdk.NewInt64Coin(shareDenom, 1_000_000_000_000_000)
	vaultAddress := types.GetVaultAddress(shareDenom)
	testBlockTime := time.Now()
	futureTime := testBlockTime.Add(100 * time.Second)
	pastTime := testBlockTime.Add(-60 * 24 * time.Hour) // ~2 months

	tests := []struct {
		name              string
		interestRate      string
		periodStart       int64
		paused            bool
		expectedVaultAmt  sdkmath.Int
		expectedMarkerAmt sdkmath.Int
		inPayoutQueue     bool
		expectedErrSubstr string
		expectedEvents    func() sdk.Events
	}{
		{
			name:              "no start period found, should set period start and return no error",
			interestRate:      "0.25",
			periodStart:       0,
			expectedVaultAmt:  underlying.Amount,
			expectedMarkerAmt: underlying.Amount,
			inPayoutQueue:     true,
		},
		{
			name:              "interest period start in future, should return nil and do nothing",
			interestRate:      "0.25",
			periodStart:       futureTime.Unix(),
			expectedVaultAmt:  underlying.Amount,
			expectedMarkerAmt: underlying.Amount,
			inPayoutQueue:     true,
		},
		{
			name:         "interest period has elasped, should pay interest and update period start",
			interestRate: "0.25",
			periodStart:  pastTime.Unix(),
			// Fee is calculated on TVV AFTER interest transfer.
			// Initial TVV: 1,000,000,000. Interest: 41,952,013.
			// TVV after interest: 1,041,952,013.
			// Fee: 1,041,952,013 * 0.0015 * 5,184,000 / 31,536,000 = 256,919
			// Total marker change: 41,952,013 - 256,919 = 41,695,094
			expectedVaultAmt:  sdkmath.NewInt(958_047_987),
			expectedMarkerAmt: sdkmath.NewInt(1_041_695_094),
			inPayoutQueue:     true,
			expectedEvents: func() sdk.Events {
				markerAddr := markertypes.MustGetMarkerAddress(shareDenom)
				ev := createReconcileEvents(
					vaultAddress,
					markerAddr,
					sdkmath.NewInt(41_952_013),
					sdkmath.NewInt(1_000_000_000),
					sdkmath.NewInt(1_041_952_013),
					underlying.Denom,
					"0.25",
					5_184_000,
				)
				provlabsAddr, err := s.k.GetAUMFeeAddress(s.ctx)
				s.Require().NoError(err, "failed to get AUM fee address")

				feeEvs := createSendCoinEvents(markerAddr.String(), provlabsAddr.String(), "256919underlying")
				feeEv := createVaultFeeCollectedEvent(
					vaultAddress,
					sdk.NewCoin(underlying.Denom, sdkmath.NewInt(1_041_952_013)),
					sdk.NewCoin(underlying.Denom, sdkmath.NewInt(256_919)),
					sdk.NewCoin(underlying.Denom, sdkmath.NewInt(256_919)),
					sdk.NewCoin(underlying.Denom, sdkmath.ZeroInt()),
					5_184_000,
				)

				nav := createMarkerSetNAV(
					shareDenom,
					sdk.NewCoin(underlying.Denom, sdkmath.NewInt(1_041_695_094)), // NAV updated with fee
					"vault",
					totalShares.Amount.Uint64(),
				)
				all := append(ev, feeEvs...)
				all = append(all, feeEv, nav)
				return all
			},
		},
		{
			name:         "interest period has elasped, should pay negative interest and update period start",
			interestRate: "-0.25",
			periodStart:  pastTime.Unix(),
			// Initial TVV: 1,000,000,000. Interest: -40,262,904.
			// TVV after interest: 959,737,096.
			// Fee: 959,737,096 * 0.0015 * 5,184,000 / 31,536,000 = 236,647
			// Total marker change: -40,262,904 - 236,647 = -40,499,551
			// Marker: 1,000,000,000 - 40,499,551 = 959,500,449
			// Vault: 1,000,000,000 + 40,262,904 = 1,040,262,904
			expectedVaultAmt:  sdkmath.NewInt(1_040_262_904),
			expectedMarkerAmt: sdkmath.NewInt(959_500_449),
			inPayoutQueue:     true,
			expectedEvents: func() sdk.Events {
				markerAddr := markertypes.MustGetMarkerAddress(shareDenom)
				ev := createReconcileEvents(
					vaultAddress,
					markerAddr,
					sdkmath.NewInt(-40_262_904),
					sdkmath.NewInt(1_000_000_000),
					sdkmath.NewInt(959_737_096),
					underlying.Denom,
					"-0.25",
					5_184_000,
				)
				provlabsAddr, err := s.k.GetAUMFeeAddress(s.ctx)
				s.Require().NoError(err, "failed to get AUM fee address")

				feeEvs := createSendCoinEvents(markerAddr.String(), provlabsAddr.String(), "236647underlying")
				feeEv := createVaultFeeCollectedEvent(
					vaultAddress,
					sdk.NewCoin(underlying.Denom, sdkmath.NewInt(959_737_096)),
					sdk.NewCoin(underlying.Denom, sdkmath.NewInt(236_647)),
					sdk.NewCoin(underlying.Denom, sdkmath.NewInt(236_647)),
					sdk.NewCoin(underlying.Denom, sdkmath.ZeroInt()),
					5_184_000,
				)

				nav := createMarkerSetNAV(
					shareDenom,
					sdk.NewCoin(underlying.Denom, sdkmath.NewInt(959_500_449)), // NAV updated with fee
					"vault",
					totalShares.Amount.Uint64(),
				)
				all := append(ev, feeEvs...)
				all = append(all, feeEv, nav)
				return all
			},
		},
		{
			name:              "paused vault, should do nothing",
			interestRate:      "0.25",
			periodStart:       pastTime.Unix(),
			paused:            true,
			expectedVaultAmt:  underlying.Amount,
			expectedMarkerAmt: underlying.Amount,
			inPayoutQueue:     false,
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			s.setupReconcileVault(tc.interestRate, tc.periodStart, tc.paused, underlying, shareDenom, totalShares, testBlockTime)

			vault, err := s.k.GetVault(s.ctx, vaultAddress)
			s.Require().NoError(err, "failed to get vault %s before reconcile", vaultAddress.String())
			err = s.k.TestAccessor_reconcileVault(s.T(), s.ctx, vault)

			if len(tc.expectedErrSubstr) > 0 {
				s.Require().Error(err, "expected error from ReconcileVault for case: %s", tc.name)
				s.Require().Contains(err.Error(), tc.expectedErrSubstr, "error substring mismatch for case: %s", tc.name)
			} else {
				s.Require().NoError(err, "ReconcileVault should not error for case: %s", tc.name)
			}

			s.assertInPayoutVerificationQueue(vaultAddress, tc.inPayoutQueue)
			s.assertVaultAndMarkerBalances(vaultAddress, shareDenom, underlying.Denom, tc.expectedVaultAmt, tc.expectedMarkerAmt)

			if tc.paused {
				updatedVault, err := s.k.GetVault(s.ctx, vaultAddress)
				s.Require().NoError(err, "failed to get vault %s after reconcile", vaultAddress.String())
				s.Require().Equal(tc.periodStart, updatedVault.PeriodStart, "PeriodStart should remain unchanged when paused for case: %s", tc.name)
			}

			var expectedEvents sdk.Events = sdk.Events{}
			if tc.expectedEvents != nil {
				expectedEvents = tc.expectedEvents()
			}
			s.Assert().Equal(
				normalizeEvents(expectedEvents),
				normalizeEvents(s.ctx.EventManager().Events()),
				"events mismatch for case: %s", tc.name,
			)
		})
	}
}

func (s *TestSuite) TestKeeper_PerformVaultReconcile_CompositeWithOutstandingFee() {
	shareDenom := "composite.shares"
	underlyingDenom := "underlying"
	paymentDenom := "payment"
	vaultAddress := types.GetVaultAddress(shareDenom)
	testBlockTime := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	sixtyDays := 60 * 24 * time.Hour
	pastTime := testBlockTime.Add(-sixtyDays)

	// Local helper for complex composite setup
	setup := func(outstanding sdk.Coin, markerLiquidity sdk.Coins, navPrice *sdk.Coin) *types.VaultAccount {
		s.SetupTest()
		s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlyingDenom, 10_000_000_000), s.adminAddr)
		s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(paymentDenom, 10_000_000_000), s.adminAddr)

		vault := s.CreateVaultWithParams(shareDenom, underlyingDenom, paymentDenom)
		vault.CurrentInterestRate = "0.25"
		vault.DesiredInterestRate = "0.25"
		vault.PeriodStart = pastTime.Unix()
		vault.FeePeriodStart = pastTime.Unix()
		vault.OutstandingAumFee = outstanding
		s.k.AuthKeeper.SetAccount(s.ctx, vault)

		markerAddr := markertypes.MustGetMarkerAddress(shareDenom)
		s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, markerAddr, markerLiquidity), "failed to fund marker account with liquidity")
		// Fund reserves to pay interest
		s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, vaultAddress, sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, 1_000_000_000))), "failed to fund vault reserves")

		if navPrice == nil {
			navPrice = &sdk.Coin{Denom: underlyingDenom, Amount: sdkmath.NewInt(1)}
		}

		s.setVaultNAV(vault, paymentDenom, *navPrice, 1)

		s.ctx = s.ctx.WithBlockTime(testBlockTime).WithEventManager(sdk.NewEventManager())
		return vault
	}

	s.Run("Case 1: Full Collection (Sufficient Liquidity)", func() {
		outstanding := sdk.NewInt64Coin(paymentDenom, 100_000)
		markerLiquidity := sdk.NewCoins(
			sdk.NewInt64Coin(underlyingDenom, 1_000_000_000),
			sdk.NewInt64Coin(paymentDenom, 1_000_000), // Plenty of payment tokens
		)
		vault := setup(outstanding, markerLiquidity, nil) // 1:1 Fast-path

		// Interest on GROSS TVV (1,000,000,000 + 1,000,000 = 1,001,000,000)
		// 1,001,000,000 * (e^(0.25 * 5184000/31536000) - 1) = 41,993,965
		expectedInterest := sdkmath.NewInt(41_993_965)

		err := s.k.TestAccessor_reconcileVault(s.T(), s.ctx, vault)
		s.Require().NoError(err, "reconcileVault should not error for case: Full Collection")

		// Verify event shows Gross principal
		events := normalizeEvents(s.ctx.EventManager().Events())
		var reconcileEv *sdk.Event
		for i := range events {
			if events[i].Type == "provlabs.vault.v1.EventVaultReconcile" {
				reconcileEv = &events[i]
				break
			}
		}
		s.Require().NotNil(reconcileEv, "EventVaultReconcile should be emitted in Case 1 (events: %v)", events)
		s.Require().Equal("1001000000underlying", getAttribute(*reconcileEv, "principal_before"), "principal_before mismatch in Case 1")
		s.Require().Equal(expectedInterest.String()+"underlying", getAttribute(*reconcileEv, "interest_earned"), "interest_earned mismatch in Case 1")

		updatedVault, err := s.k.GetVault(s.ctx, vaultAddress)
		s.Require().NoError(err, "failed to get updated vault")
		s.Require().True(updatedVault.OutstandingAumFee.IsZero(), "all fees should be cleared")

		provlabsAddr, err := s.k.GetAUMFeeAddress(s.ctx)
		s.Require().NoError(err, "failed to get AUM fee address")
		// TVV after interest = 1,001,000,000 + 41,993,965 = 1,042,993,965
		// Current Fee (15bps): 1,042,993,965 * 0.0015 * 5184000/31536000 = 257,176
		// Total Debt = 257,176 (current) + 100,000 (outstanding) = 357,176
		s.assertBalance(provlabsAddr, paymentDenom, sdkmath.NewInt(357_176))
	})

	s.Run("Case 2: Partial Collection (Insufficient Liquidity)", func() {
		outstanding := sdk.NewInt64Coin(paymentDenom, 1_000_000)
		markerLiquidity := sdk.NewCoins(
			sdk.NewInt64Coin(underlyingDenom, 1_000_000_000),
			sdk.NewInt64Coin(paymentDenom, 100_000), // Only 100k available
		)
		vault := setup(outstanding, markerLiquidity, nil)

		err := s.k.TestAccessor_reconcileVault(s.T(), s.ctx, vault)
		s.Require().NoError(err, "reconcileVault should not error for case: Partial Collection")

		// TVV = 1,000,100,000
		// Interest on 1,000,100,000 = 41,956,208
		// Updated TVV = 1,042,056,208
		// Fee = 1,042,056,208 * 0.0015 * 5184000/31536000 = 256,945
		// Total Debt = 256,945 (current on Gross) + 1,000_000 (old) = 1,256,945
		// Collected = 100,000 (all available)
		// Remaining = 1,156,945
		updatedVault, err := s.k.GetVault(s.ctx, vaultAddress)
		s.Require().NoError(err, "failed to get updated vault")
		s.Require().Equal(sdkmath.NewInt(1_156_945), updatedVault.OutstandingAumFee.Amount, "outstanding fee balance mismatch in Case 2")

		provlabsAddr, err := s.k.GetAUMFeeAddress(s.ctx)
		s.Require().NoError(err, "failed to get AUM fee address")
		s.assertBalance(provlabsAddr, paymentDenom, sdkmath.NewInt(100_000))
	})

	s.Run("Case 3: Interest Accrual on Debted Assets (Verify Gross Logic)", func() {
		// Vault has 1,000,000,000 underlying
		// It OWES 500,000,000 payment (Outstanding fee = 500m underlying value)
		// But those 500m are STILL in the marker (Gross = 1.5b)
		outstanding := sdk.NewInt64Coin(paymentDenom, 500_000_000)
		markerLiquidity := sdk.NewCoins(
			sdk.NewInt64Coin(underlyingDenom, 1_000_000_000),
			sdk.NewInt64Coin(paymentDenom, 500_000_000),
		)
		vault := setup(outstanding, markerLiquidity, nil)

		// Interest on 1.5b (Gross), not 1.0b (Net)
		// 1,500,000,000 * (e^(0.25 * 5184000/31536000) - 1) = 62,928,020
		expectedInterest := sdkmath.NewInt(62_928_020)

		err := s.k.TestAccessor_reconcileVault(s.T(), s.ctx, vault)
		s.Require().NoError(err, "reconcileVault should not error for case: Interest Accrual on Debted Assets")

		events := normalizeEvents(s.ctx.EventManager().Events())
		var reconcileEv *sdk.Event
		for i := range events {
			if events[i].Type == "provlabs.vault.v1.EventVaultReconcile" {
				reconcileEv = &events[i]
				break
			}
		}
		s.Require().NotNil(reconcileEv, "EventVaultReconcile should be emitted in Case 3 (events: %v)", events)
		s.Require().Equal("1500000000underlying", getAttribute(*reconcileEv, "principal_before"), "principal_before mismatch in Case 3")
		s.Require().Equal(expectedInterest.String()+"underlying", getAttribute(*reconcileEv, "interest_earned"), "interest_earned mismatch in Case 3")
	})

	s.Run("Case 4: Net Valuation for Share Pricing", func() {
		outstanding := sdk.NewInt64Coin(paymentDenom, 100_000_000)
		markerLiquidity := sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, 1_000_000_000))
		vault := setup(outstanding, markerLiquidity, nil)

		// GROSS = 1,000,000,000
		// NET = 1,000,000,000 - 100,000,000 = 900,000_000

		tvv, err := s.k.GetTVVInUnderlyingAsset(s.ctx, *vault)
		s.Require().NoError(err, "failed to get TVV in underlying asset")
		s.Require().Equal(sdkmath.NewInt(1_000_000_000), tvv, "GetTVV returns Gross")

		// Valuation should return Net
		// Interest on Gross (1b) = 41,952,013
		// Fee on Updated Gross (1,041,952,013) = 256,919
		// Debt = 100,000,000
		// Net Valuation = (1,000,000,000 + 41,952,013) - 256,919 - 100,000,000 = 941,695,094
		expectedNetValuation := sdkmath.NewInt(941_695_094)

		val, err := s.k.CalculateVaultTotalAssets(s.ctx, vault, sdk.NewInt64Coin(underlyingDenom, 1_000_000_000))
		s.Require().NoError(err, "CalculateVaultTotalAssets should not error for case: Net Valuation for Share Pricing")
		s.Require().Equal(expectedNetValuation, val, "valuation should be Net of outstanding fees")
	})

	s.Run("Case 5: Negative Interest with Liabilities", func() {
		outstanding := sdk.NewInt64Coin(paymentDenom, 100_000)
		markerLiquidity := sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, 1_000_000_000))
		vault := setup(outstanding, markerLiquidity, nil)
		vault.CurrentInterestRate = "-0.25"
		vault.DesiredInterestRate = "-0.25"
		s.k.AuthKeeper.SetAccount(s.ctx, vault)

		// Negative Interest on Gross (1,000,000,000)
		// 1,000,000,000 * (e^(-0.25 * 5184000/31536000) - 1) = -40,262,904
		expectedRefund := sdkmath.NewInt(40_262_904)

		err := s.k.TestAccessor_reconcileVault(s.T(), s.ctx, vault)
		s.Require().NoError(err, "reconcileVault should not error for case: Negative Interest with Liabilities")

		events := normalizeEvents(s.ctx.EventManager().Events())
		var reconcileEv *sdk.Event
		for i := range events {
			if events[i].Type == "provlabs.vault.v1.EventVaultReconcile" {
				reconcileEv = &events[i]
				break
			}
		}
		s.Require().NotNil(reconcileEv, "EventVaultReconcile should be emitted in Case 5 (events: %v)", events)
		s.Require().Equal(expectedRefund.Neg().String()+"underlying", getAttribute(*reconcileEv, "interest_earned"), "interest_earned mismatch in Case 5")

		// Vault reserves should increase by refund
		s.assertBalance(vaultAddress, underlyingDenom, sdkmath.NewInt(1_040_262_904))
	})

	s.Run("Case 6: Non-1:1 NAV Composite Conversion", func() {
		// Outstanding in Payment tokens
		// NAV: 1 payment = 2 underlying
		outstanding := sdk.NewInt64Coin(paymentDenom, 50_000_000) // Value 100m underlying
		markerLiquidity := sdk.NewCoins(
			sdk.NewInt64Coin(underlyingDenom, 500_000_000),
			sdk.NewInt64Coin(paymentDenom, 250_000_000), // Value 500m underlying
		)
		// Gross TVV = 500m + (250m * 2) = 1,000,000,000
		vault := setup(outstanding, markerLiquidity, &sdk.Coin{Denom: underlyingDenom, Amount: sdkmath.NewInt(2)})

		// Accruals on Gross (1b)
		// Interest = 41,952,013
		// Fee = 256,919
		// Debt (Net of NAV) = 50,000,000 * 2 = 100,000,000
		// Net Valuation = (1,000,000,000 + 41,952,013) - 256,919 - 100,000,000 = 941,695,094

		val, err := s.k.CalculateVaultTotalAssets(s.ctx, vault, sdk.NewInt64Coin(underlyingDenom, 1_000_000_000))
		s.Require().NoError(err, "CalculateVaultTotalAssets should not error for case: Non-1:1 NAV Composite Conversion")
		s.Require().Equal(sdkmath.NewInt(941_695_094), val, "valuation should correctly convert secondary debt via NAV")
	})
}

func (s *TestSuite) TestKeeper_ReconcileLeavesUncollectedFee_PricesOffNetTVV() {
	shareDenom := "underfunded.shares"
	underlyingDenom := "underlying"
	paymentDenom := "payment"
	vaultAddress := types.GetVaultAddress(shareDenom)
	testBlockTime := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	pastTime := testBlockTime.Add(-60 * 24 * time.Hour)

	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlyingDenom, 10_000_000_000), s.adminAddr)
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(paymentDenom, 10_000_000_000), s.adminAddr)

	vault := s.CreateVaultWithParams(shareDenom, underlyingDenom, paymentDenom)
	vault.CurrentInterestRate = "0"
	vault.DesiredInterestRate = "0"
	vault.AumFeeBips = 0
	vault.PeriodStart = pastTime.Unix()
	vault.FeePeriodStart = pastTime.Unix()
	vault.OutstandingAumFee = sdk.NewInt64Coin(paymentDenom, 100_000_000)
	totalShares := sdk.NewInt64Coin(shareDenom, 1_000_000)
	vault.TotalShares = totalShares
	s.k.AuthKeeper.SetAccount(s.ctx, vault)
	s.Require().NoError(s.k.MarkerKeeper.MintCoin(s.ctx, vault.GetAddress(), totalShares), "should mint share supply")

	paymentMarkerAddr := markertypes.MustGetMarkerAddress(paymentDenom)
	paymentMarkerAccount, err := s.k.MarkerKeeper.GetMarker(s.ctx, paymentMarkerAddr)
	s.Require().NoError(err, "should fetch payment marker for NAV setup")
	s.Require().NoError(s.k.MarkerKeeper.SetNetAssetValue(s.ctx, paymentMarkerAccount, markertypes.NetAssetValue{
		Price:  sdk.NewInt64Coin(underlyingDenom, 1),
		Volume: 1,
	}, "test"), "should set payment->underlying NAV at 1:1")

	s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, vault.PrincipalMarkerAddress(), sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 1_000_000_000),
	)), "should fund principal marker with underlying only, leaving no payment-denom liquidity to pay the fee")

	s.SetCtxBlockTime(testBlockTime)

	err = s.k.TestAccessor_reconcileVault(s.T(), s.ctx, vault)
	s.Require().NoError(err, "reconcileVault should not error")

	reconciled, err := s.k.GetVault(s.ctx, vaultAddress)
	s.Require().NoError(err, "should fetch reconciled vault")

	s.Require().Equal(sdkmath.NewInt(100_000_000), reconciled.OutstandingAumFee.Amount, "fee could not be collected, so the full liability survives reconcile")
	provlabsAddr, err := s.k.GetAUMFeeAddress(s.ctx)
	s.Require().NoError(err, "should get AUM fee address")
	s.assertBalance(provlabsAddr, paymentDenom, sdkmath.ZeroInt())

	grossTVV, err := s.k.GetTVVInUnderlyingAsset(s.ctx, *reconciled)
	s.Require().NoError(err, "should compute gross TVV")
	s.Require().Equal(sdkmath.NewInt(1_000_000_000), grossTVV, "gross TVV still counts the assets backing the unpaid fee")
	netTVV, err := s.k.GetNetTVVInUnderlyingAsset(s.ctx, *reconciled)
	s.Require().NoError(err, "should compute net TVV")
	s.Require().Equal(sdkmath.NewInt(900_000_000), netTVV, "net TVV = gross 1,000,000,000 minus outstanding fee 100,000,000, so gross > net even after reconcile")

	grossView := *reconciled
	grossView.OutstandingAumFee = sdk.NewInt64Coin(paymentDenom, 0)

	navNet, err := s.k.GetNAVPerShareInUnderlyingAsset(s.ctx, *reconciled)
	s.Require().NoError(err, "should compute net NAV per share")
	s.Require().Equal(sdkmath.NewInt(900), navNet, "net NAV per share = 900,000,000 / 1,000,000")
	navGross, err := s.k.GetNAVPerShareInUnderlyingAsset(s.ctx, grossView)
	s.Require().NoError(err, "should compute gross NAV per share")
	s.Require().Equal(sdkmath.NewInt(1_000), navGross, "gross NAV per share = 1,000,000,000 / 1,000,000")

	deposit := sdk.NewInt64Coin(underlyingDenom, 1_000_000)
	netMint, err := s.k.ConvertDepositToSharesInUnderlyingAsset(s.ctx, *reconciled, deposit)
	s.Require().NoError(err, "net deposit conversion should succeed")
	grossMint, err := s.k.ConvertDepositToSharesInUnderlyingAsset(s.ctx, grossView, deposit)
	s.Require().NoError(err, "gross deposit conversion should succeed")
	s.Require().True(netMint.Amount.GT(grossMint.Amount), "net pricing mints more shares per deposit than gross would (gross overstates TVV)")

	redeemShares := sdkmath.NewInt(100_000)
	netRedeem, err := s.k.ConvertSharesToRedeemCoin(s.ctx, *reconciled, redeemShares, underlyingDenom)
	s.Require().NoError(err, "net redeem conversion should succeed")
	grossRedeem, err := s.k.ConvertSharesToRedeemCoin(s.ctx, grossView, redeemShares, underlyingDenom)
	s.Require().NoError(err, "gross redeem conversion should succeed")
	s.Require().True(netRedeem.Amount.LT(grossRedeem.Amount), "net pricing pays out less per share than gross would (gross overstates TVV)")
}

func (s *TestSuite) TestKeeper_CalculateVaultTotalAssets() {
	shareDenom := "vaultshares"
	underlying := sdk.NewInt64Coin("underlying", 1_000_000_000)
	vaultAddress := types.GetVaultAddress(shareDenom)
	testBlockTime := time.Now()
	pastTime := testBlockTime.Add(-60 * 24 * time.Hour) // ~2 months

	setup := func(interestRate string, periodStartSeconds int64) *types.VaultAccount {
		s.requireAddFinalizeAndActivateMarker(underlying, s.adminAddr)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           s.adminAddr.String(),
			ShareDenom:      shareDenom,
			UnderlyingAsset: underlying.Denom,
		})
		s.Require().NoError(err, "CreateVault should not error in CalculateVaultTotalAssets setup")

		vault, err := s.k.GetVault(s.ctx, vaultAddress)
		s.Require().NoError(err, "GetVault should not error in CalculateVaultTotalAssets setup")
		vault.CurrentInterestRate = interestRate
		vault.PeriodStart = periodStartSeconds
		vault.FeePeriodStart = testBlockTime.Unix()
		s.k.AuthKeeper.SetAccount(s.ctx, vault)

		s.ctx = s.ctx.WithBlockTime(testBlockTime)
		return vault
	}

	tests := []struct {
		name             string
		rate             string
		startTime        int64
		expectedIncrease sdkmath.Int
		expectErr        bool
	}{
		{
			name:             "no interest rate set",
			rate:             "",
			startTime:        pastTime.Unix(),
			expectedIncrease: sdkmath.NewInt(1_000_000_000),
		},
		{
			name:             "zero interest rate",
			rate:             types.ZeroInterestRate,
			startTime:        pastTime.Unix(),
			expectedIncrease: sdkmath.NewInt(1_000_000_000),
		},
		{
			name:             "no start time",
			rate:             "1.0",
			startTime:        0,
			expectedIncrease: sdkmath.NewInt(1_000_000_000),
		},
		{
			name:             "interest accrues positively",
			rate:             "0.25",
			startTime:        pastTime.Unix(),
			expectedIncrease: sdkmath.NewInt(1_041_952_013),
		},
		{
			name:             "negative interest accrues",
			rate:             "-0.25",
			startTime:        pastTime.Unix(),
			expectedIncrease: sdkmath.NewInt(959_737_096),
		},
		{
			name:             "period in future returns original amount",
			rate:             "0.25",
			startTime:        testBlockTime.Add(100 * time.Second).Unix(),
			expectedIncrease: sdkmath.NewInt(1_000_000_000),
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			vault := setup(tc.rate, tc.startTime)
			est, err := s.k.CalculateVaultTotalAssets(s.ctx, vault, underlying)

			if tc.expectErr {
				s.Require().Error(err, "test case %s: expected error from CalculateVaultTotalAssets", tc.name)
			} else {
				s.Require().NoError(err, "test case %s: CalculateVaultTotalAssets should not error", tc.name)
				s.Require().Equal(tc.expectedIncrease, est, "test case %s: total assets estimation mismatch", tc.name)
			}
		})
	}
}

func (s *TestSuite) TestKeeper_HandleVaultInterestTimeouts() {
	now := time.Now()
	twoMonthsAgo := now.Add(-60 * 24 * time.Hour).Unix()
	testBlockTime := now
	shareDenom := "vaultshares"
	underlying := sdk.NewInt64Coin("underlying", 1_000_000_000)
	vaultAddr := types.GetVaultAddress(shareDenom)
	markerAddr := markertypes.MustGetMarkerAddress(shareDenom)

	tests := []struct {
		name           string
		setup          func()
		checkAddr      sdk.AccAddress
		expectExists   bool
		expectDeleted  bool
		expectRate     string
		expectedEvents sdk.Events
	}{
		{
			name: "happy path: interest paid and period reset",
			setup: func() {
				s.requireAddFinalizeAndActivateMarker(underlying, s.adminAddr)
				_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
					Admin:           s.adminAddr.String(),
					ShareDenom:      shareDenom,
					UnderlyingAsset: underlying.Denom,
				})
				s.Require().NoError(err, "happy path: CreateVault should not error")
				vault, err := s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err, "happy path: GetVault should not error")
				vault.CurrentInterestRate = "0.25"
				vault.DesiredInterestRate = "0.25"
				vault.PeriodStart = twoMonthsAgo
				vault.FeePeriodStart = twoMonthsAgo
				s.k.AuthKeeper.SetAccount(s.ctx, vault)
				s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, vaultAddr, sdk.NewCoins(underlying)), "happy path: funding vault account should not error")
				s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, markerAddr, sdk.NewCoins(underlying)), "happy path: funding marker account should not error")
				s.Require().NoError(s.k.PayoutTimeoutQueue.Enqueue(s.ctx, testBlockTime.Unix(), vault.GetAddress()), "happy path: enqueuing payout timeout should not error")
				s.ctx = s.ctx.WithBlockTime(testBlockTime).WithEventManager(sdk.NewEventManager())
			},
			checkAddr:     vaultAddr,
			expectExists:  true,
			expectDeleted: false,
			expectRate:    "0.25",
			expectedEvents: func() sdk.Events {
				markerAddr := markertypes.MustGetMarkerAddress(shareDenom)
				evs := sdk.Events{
					sdk.NewEvent("coin_spent",
						sdk.NewAttribute("spender", vaultAddr.String()),
						sdk.NewAttribute("amount", "41952013underlying"),
					),
					sdk.NewEvent("coin_received",
						sdk.NewAttribute("receiver", markerAddr.String()),
						sdk.NewAttribute("amount", "41952013underlying"),
					),
					sdk.NewEvent("transfer",
						sdk.NewAttribute("recipient", markerAddr.String()),
						sdk.NewAttribute("sender", vaultAddr.String()),
						sdk.NewAttribute("amount", "41952013underlying"),
					),
					sdk.NewEvent("message",
						sdk.NewAttribute("sender", vaultAddr.String()),
					),
					sdk.NewEvent("provlabs.vault.v1.EventVaultReconcile",
						sdk.NewAttribute("interest_earned", "41952013underlying"),
						sdk.NewAttribute("principal_after", "1041952013underlying"),
						sdk.NewAttribute("principal_before", "1000000000underlying"),
						sdk.NewAttribute("rate", "0.25"),
						sdk.NewAttribute("time", "5184000"),
						sdk.NewAttribute("vault_address", vaultAddr.String()),
					),
				}
				return evs
			}(),
		},
		{
			name: "vault cannot pay: interest set to 0 and record deleted",
			setup: func() {
				s.requireAddFinalizeAndActivateMarker(underlying, s.adminAddr)
				_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
					Admin:           s.adminAddr.String(),
					ShareDenom:      shareDenom,
					UnderlyingAsset: underlying.Denom,
				})
				s.Require().NoError(err, "vault cannot pay: CreateVault should not error")
				vault, err := s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err, "vault cannot pay: GetVault should not error")
				vault.CurrentInterestRate = "0.25"
				vault.DesiredInterestRate = "0.25"
				s.k.AuthKeeper.SetAccount(s.ctx, vault)
				s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, markerAddr, sdk.NewCoins(underlying)), "vault cannot pay: funding marker account should not error")
				s.Require().NoError(s.k.SafeAddPayoutVerification(s.ctx, vault), "vault cannot pay: SafeAddPayoutVerification should not error")
				s.Require().NoError(s.k.PayoutTimeoutQueue.Enqueue(s.ctx, testBlockTime.Unix(), vault.GetAddress()), "vault cannot pay: enqueuing payout timeout should not error")
				s.ctx = s.ctx.WithBlockTime(testBlockTime).WithEventManager(sdk.NewEventManager())
			},
			checkAddr:     vaultAddr,
			expectExists:  false,
			expectDeleted: true,
			expectRate:    types.ZeroInterestRate,
			expectedEvents: sdk.Events{
				sdk.NewEvent("provlabs.vault.v1.EventVaultInterestChange",
					sdk.NewAttribute("current_rate", types.ZeroInterestRate),
					sdk.NewAttribute("desired_rate", "0.25"),
					sdk.NewAttribute("vault_address", vaultAddr.String()),
				),
			},
		},
		{
			name: "paused vault is skipped and remains in queue",
			setup: func() {
				s.requireAddFinalizeAndActivateMarker(underlying, s.adminAddr)
				_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
					Admin:           s.adminAddr.String(),
					ShareDenom:      shareDenom,
					UnderlyingAsset: underlying.Denom,
				})
				s.Require().NoError(err, "paused vault: CreateVault should not error")
				vault, err := s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err, "paused vault: GetVault should not error")
				vault.Paused = true
				s.k.AuthKeeper.SetAccount(s.ctx, vault)
				s.Require().NoError(s.k.PayoutTimeoutQueue.Enqueue(s.ctx, testBlockTime.Unix(), vault.GetAddress()), "paused vault: enqueuing payout timeout should not error")
				s.ctx = s.ctx.WithBlockTime(testBlockTime).WithEventManager(sdk.NewEventManager())
			},
			checkAddr:      vaultAddr,
			expectExists:   true,
			expectDeleted:  false,
			expectedEvents: sdk.Events{},
		},
		{
			name: "non-vault address in interest details does nothing",
			setup: func() {
				s.Require().NoError(s.k.PayoutVerificationSet.Set(s.ctx, markerAddr), "non-vault: PayoutVerificationSet.Set should not error")
				s.Require().NoError(s.k.PayoutTimeoutQueue.Enqueue(s.ctx, testBlockTime.Unix(), markerAddr), "non-vault: enqueuing payout timeout should not error")
				s.ctx = s.ctx.WithBlockTime(testBlockTime).WithEventManager(sdk.NewEventManager())
			},
			checkAddr:      markerAddr,
			expectExists:   true,
			expectDeleted:  false,
			expectRate:     "",
			expectedEvents: sdk.Events{},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			tc.setup()
			err := s.k.TestAccessor_handleVaultInterestTimeouts(s.T(), s.ctx)
			s.Require().NoError(err, "test case %s: handleVaultInterestTimeouts should not error", tc.name)
			if tc.expectRate != "" {
				vault, err := s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err, "test case %s: GetVault should not error after timeout handling", tc.name)
				s.Require().Equal(tc.expectRate, vault.CurrentInterestRate, "test case %s: current interest rate mismatch", tc.name)
			}
			em := s.ctx.EventManager()
			s.Assert().Equalf(
				normalizeEvents(tc.expectedEvents),
				normalizeEvents(em.Events()),
				"test case %s: beginblocker events mismatch", tc.name,
			)
		})
	}
}

func (s *TestSuite) TestKeeper_HandleReconciledVaults() {
	v1, v2 := NewVaultInfo(1), NewVaultInfo(2)

	testBlockTime := time.Now().UTC().Truncate(time.Second)

	AssertIsPayable := func(info VaultInfo, expectedRate string) {
		s.T().Helper()
		vault, err := s.k.GetVault(s.ctx, info.vaultAddr)
		s.Require().NoError(err, "GetVault should not error for payable vault %s", info.vaultAddr)
		s.Require().NotNil(vault, "vault %s should not be nil", info.vaultAddr)
		s.Assert().Equal(expectedRate, vault.CurrentInterestRate, "interest rate should not change for payable vault %s", info.vaultAddr)
	}

	AssertIsDepleted := func(info VaultInfo) {
		s.T().Helper()
		vault, err := s.k.GetVault(s.ctx, info.vaultAddr)
		s.Require().NoError(err, "GetVault should not error for depleted vault %s", info.vaultAddr)
		s.Require().NotNil(vault, "vault %s should not be nil", info.vaultAddr)
		s.Assert().Equal(types.ZeroInterestRate, vault.CurrentInterestRate, "interest rate should be zeroed out for depleted vault %s", info.vaultAddr)
		s.assertInPayoutVerificationQueue(vault.GetAddress(), false)
	}

	tests := []struct {
		name           string
		setup          func()
		postCheck      func()
		expectErr      bool
		expectedEvents sdk.Events
	}{
		{
			name: "no vaults in store",
			setup: func() {
				// No setup needed, store is empty
			},
			postCheck: func() {
				// No vaults to check
				vaults, err := s.k.GetVaults(s.ctx)
				s.Require().NoError(err, "no vaults: GetVaults should not error")
				s.Require().Empty(vaults, "no vaults: vaults should be empty")
			},
			expectErr:      false,
			expectedEvents: sdk.Events{},
		},
		{
			name: "one vault reconciled, sufficient funds",
			setup: func() {
				createVaultWithInterest(s, v1, "0.1", testBlockTime.Unix(), testBlockTime.Unix(), true, true)
			},
			postCheck: func() {
				AssertIsPayable(v1, "0.1")
			},
			expectErr:      false,
			expectedEvents: sdk.Events{},
		},
		{
			name: "one vault reconciled, insufficient funds",
			setup: func() {
				createVaultWithInterest(s, v1, "0.1", testBlockTime.Unix(), testBlockTime.Unix(), false, true)
			},
			postCheck: func() {
				AssertIsDepleted(v1)
			},
			expectErr: false,
			expectedEvents: sdk.Events{
				sdk.NewEvent(
					"provlabs.vault.v1.EventVaultInterestChange",
					sdk.NewAttribute("current_rate", types.ZeroInterestRate),
					sdk.NewAttribute("desired_rate", "0.1"),
					sdk.NewAttribute("vault_address", v1.vaultAddr.String()),
				),
			},
		},
		{
			name: "two vaults reconciled, one payable, one depleted",
			setup: func() {
				// Vault 1: payable
				createVaultWithInterest(s, v1, "0.1", testBlockTime.Unix(), testBlockTime.Unix(), true, true)
				// Vault 2: depleted
				createVaultWithInterest(s, v2, "0.1", testBlockTime.Unix(), testBlockTime.Unix(), false, true)
			},
			postCheck: func() {
				AssertIsPayable(v1, "0.1")
				AssertIsDepleted(v2)
			},
			expectErr: false,
			expectedEvents: sdk.Events{
				sdk.NewEvent(
					"provlabs.vault.v1.EventVaultInterestChange",
					sdk.NewAttribute("current_rate", types.ZeroInterestRate),
					sdk.NewAttribute("desired_rate", "0.1"),
					sdk.NewAttribute("vault_address", v2.vaultAddr.String()),
				),
			},
		},
		{
			name: "paused vault is skipped",
			setup: func() {
				vault := createVaultWithInterest(s, v1, "0.1", testBlockTime.Unix(), testBlockTime.Unix(), true, true)
				vault.Paused = true
				s.k.AuthKeeper.SetAccount(s.ctx, vault)
			},
			postCheck: func() {
				s.assertInPayoutVerificationQueue(v1.vaultAddr, true) // Should still be in the queue
			},
			expectErr:      false,
			expectedEvents: sdk.Events{},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			s.ctx = s.ctx.WithBlockTime(testBlockTime)
			if tc.setup != nil {
				tc.setup()
			}
			s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())

			err := s.k.TestAccessor_handleReconciledVaults(s.T(), s.ctx)

			if tc.expectErr {
				s.Require().Error(err, "test case %s: expected error from handleReconciledVaults", tc.name)
			} else {
				s.Require().NoError(err, "test case %s: handleReconciledVaults should not error", tc.name)
			}

			actualEvents := normalizeEvents(s.ctx.EventManager().Events())
			expectedEvents := normalizeEvents(tc.expectedEvents)
			s.Assert().Equal(expectedEvents, actualEvents, "test case %s: emitted events mismatch", tc.name)

			if tc.postCheck != nil {
				tc.postCheck()
			}
		})
	}
}

func (s *TestSuite) TestKeeper_handlePayableVaults() {
	v1, v2 := NewVaultInfo(1), NewVaultInfo(2)
	testBlockTime := time.Now().UTC().Truncate(time.Second)

	assertSingleTimeoutAt := func(addr sdk.AccAddress, expected int64) {
		count := 0
		found := false
		err := s.k.PayoutTimeoutQueue.WalkDue(s.ctx, math.MaxInt64, func(t uint64, a sdk.AccAddress) (bool, error) {
			if a.Equals(addr) {
				count++
				if t == uint64(expected) {
					found = true
				}
			}
			return false, nil
		})
		s.Require().NoError(err, "WalkDue should not error for address %s", addr)
		s.Assert().True(found, "missing timeout entry at expected time %d for address %s", expected, addr)
		s.Assert().Equal(1, count, "should be exactly one timeout entry for vault %s", addr)
	}

	tests := []struct {
		name      string
		setup     func() []*types.VaultAccount
		postCheck func(vaults []*types.VaultAccount)
	}{
		{
			name: "single payable vault",
			setup: func() []*types.VaultAccount {
				vault := createVaultWithInterest(s, v1, "0.1", testBlockTime.Unix(), 0, true, true)
				return []*types.VaultAccount{vault}
			},
			postCheck: func(vaults []*types.VaultAccount) {
				addr := vaults[0].GetAddress()
				vault, err := s.k.GetVault(s.ctx, addr)
				s.Require().NoError(err, "single payable: GetVault should not error")
				expectedExpireTime := testBlockTime.Unix() + keeper.AutoReconcileTimeout
				s.Assert().Equal(expectedExpireTime, vault.PeriodTimeout, "single payable: PeriodTimeout mismatch")
				assertSingleTimeoutAt(addr, expectedExpireTime)
			},
		},
		{
			name: "multiple payable vaults",
			setup: func() []*types.VaultAccount {
				vault1 := createVaultWithInterest(s, v1, "0.1", testBlockTime.Unix(), 0, true, true)
				vault2 := createVaultWithInterest(s, v2, "0.2", testBlockTime.Unix(), 0, true, true)
				return []*types.VaultAccount{vault1, vault2}
			},
			postCheck: func(vaults []*types.VaultAccount) {
				expectedExpireTime := testBlockTime.Unix() + keeper.AutoReconcileTimeout

				addr1 := vaults[0].GetAddress()
				v1r, err := s.k.GetVault(s.ctx, addr1)
				s.Require().NoError(err, "multiple payable (vault 1): GetVault should not error")
				s.Assert().Equal(expectedExpireTime, v1r.PeriodTimeout, "multiple payable (vault 1): PeriodTimeout mismatch")
				assertSingleTimeoutAt(addr1, expectedExpireTime)

				addr2 := vaults[1].GetAddress()
				v2r, err := s.k.GetVault(s.ctx, addr2)
				s.Require().NoError(err, "multiple payable (vault 2): GetVault should not error")
				s.Assert().Equal(expectedExpireTime, v2r.PeriodTimeout, "multiple payable (vault 2): PeriodTimeout mismatch")
				assertSingleTimeoutAt(addr2, expectedExpireTime)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			s.ctx = s.ctx.WithBlockTime(testBlockTime)
			vaults := tc.setup()
			s.k.TestAccessor_handlePayableVaults(s.T(), s.ctx, vaults)
			if tc.postCheck != nil {
				tc.postCheck(vaults)
			}
		})
	}
}

func (s *TestSuite) TestKeeper_handleDepletedVaults() {
	v1, v2 := NewVaultInfo(1), NewVaultInfo(2)
	initialRate := "0.1"

	tests := []struct {
		name           string
		setup          func() []*types.VaultAccount
		postCheck      func(vaults []*types.VaultAccount)
		expectedEvents sdk.Events
	}{
		{
			name: "single depleted vault",
			setup: func() []*types.VaultAccount {
				vault := createVaultWithInterest(s, v1, initialRate, 0, 0, false, true)
				return []*types.VaultAccount{vault}
			},
			postCheck: func(vaults []*types.VaultAccount) {
				addr := vaults[0].GetAddress()
				updatedVault, err := s.k.GetVault(s.ctx, addr)
				s.Require().NoError(err, "single depleted: GetVault should not error")
				s.Assert().Equal(types.ZeroInterestRate, updatedVault.CurrentInterestRate, "single depleted: interest rate should be zeroed")
				s.Assert().Equal(initialRate, updatedVault.DesiredInterestRate, "single depleted: desired rate should remain unchanged")
			},
			expectedEvents: sdk.Events{
				sdk.NewEvent(
					"provlabs.vault.v1.EventVaultInterestChange",
					sdk.NewAttribute("current_rate", types.ZeroInterestRate),
					sdk.NewAttribute("desired_rate", initialRate),
					sdk.NewAttribute("vault_address", v1.vaultAddr.String()),
				),
			},
		},
		{
			name: "multiple depleted vaults",
			setup: func() []*types.VaultAccount {
				vault1 := createVaultWithInterest(s, v1, initialRate, 0, 0, false, true)
				vault2 := createVaultWithInterest(s, v2, "0.2", 0, 0, false, true)
				return []*types.VaultAccount{vault1, vault2}
			},
			postCheck: func(vaults []*types.VaultAccount) {
				for _, v := range vaults {
					addr := v.GetAddress()
					updatedVault, err := s.k.GetVault(s.ctx, addr)
					s.Require().NoError(err, "multiple depleted: GetVault should not error for address %s", addr)
					s.Assert().Equal(types.ZeroInterestRate, updatedVault.CurrentInterestRate, "multiple depleted: interest rate should be zeroed for address %s", addr)
					s.Assert().Equal(v.DesiredInterestRate, updatedVault.DesiredInterestRate, "multiple depleted: desired rate mismatch for address %s", addr)
				}
			},
			expectedEvents: sdk.Events{
				sdk.NewEvent(
					"provlabs.vault.v1.EventVaultInterestChange",
					sdk.NewAttribute("current_rate", types.ZeroInterestRate),
					sdk.NewAttribute("desired_rate", initialRate),
					sdk.NewAttribute("vault_address", v1.vaultAddr.String()),
				),
				sdk.NewEvent(
					"provlabs.vault.v1.EventVaultInterestChange",
					sdk.NewAttribute("current_rate", types.ZeroInterestRate),
					sdk.NewAttribute("desired_rate", "0.2"),
					sdk.NewAttribute("vault_address", v2.vaultAddr.String()),
				),
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			vaults := tc.setup()

			s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
			s.k.TestAccessor_handleDepletedVaults(s.T(), s.ctx, vaults)

			s.Assert().Equal(
				normalizeEvents(tc.expectedEvents),
				normalizeEvents(s.ctx.EventManager().Events()),
				"test case %s: emitted events mismatch", tc.name,
			)

			if tc.postCheck != nil {
				tc.postCheck(vaults)
			}
		})
	}
}

func (s *TestSuite) TestKeeper_UpdateInterestRates() {
	v1 := NewVaultInfo(1)
	initialRate := "0.1"
	newRate := "0.2"
	desiredRate := "0.2"

	tests := []struct {
		name           string
		setup          func() *types.VaultAccount
		currentRate    string
		desiredRate    string
		expectedEvents sdk.Events
		postCheck      func(vault *types.VaultAccount)
	}{
		{
			name: "successful rate change",
			setup: func() *types.VaultAccount {
				return createVaultWithInterest(s, v1, initialRate, 0, 0, false, false)
			},
			currentRate: newRate,
			desiredRate: desiredRate,
			expectedEvents: sdk.Events{
				sdk.NewEvent(
					"provlabs.vault.v1.EventVaultInterestChange",
					sdk.NewAttribute("current_rate", newRate),
					sdk.NewAttribute("desired_rate", desiredRate),
					sdk.NewAttribute("vault_address", v1.vaultAddr.String()),
				),
			},
			postCheck: func(vault *types.VaultAccount) {
				updatedVault, err := s.k.GetVault(s.ctx, vault.GetAddress())
				s.Require().NoError(err, "retrieving updated vault should not error")
				s.Require().NotNil(updatedVault, "updated vault should not be nil")
				s.Assert().Equal(newRate, updatedVault.CurrentInterestRate, "current interest rate should be updated")
				s.Assert().Equal(desiredRate, updatedVault.DesiredInterestRate, "desired interest rate should be updated")
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			vault := tc.setup()

			s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
			s.Require().NoError(s.k.UpdateInterestRates(s.ctx, vault, tc.currentRate, tc.desiredRate), "test case %s: interest rate update should succeed", tc.name)

			s.Assert().Equal(
				normalizeEvents(tc.expectedEvents),
				normalizeEvents(s.ctx.EventManager().Events()),
				"test case %s: emitted events mismatch", tc.name)

			if tc.postCheck != nil {
				tc.postCheck(vault)
			}
		})
	}
}

func createVaultWithInterest(s *TestSuite, info VaultInfo, interestRate string, periodStart, periodTimeout int64, fundReserves, fundPrincipal bool) *types.VaultAccount {
	s.requireAddFinalizeAndActivateMarker(info.underlying, s.adminAddr)
	vault, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
		Admin:           s.adminAddr.String(),
		ShareDenom:      info.shareDenom,
		UnderlyingAsset: info.underlying.Denom,
	})
	s.Require().NoError(err, "failed to create vault in createVaultWithInterest for share denom %s", info.shareDenom)

	vault.CurrentInterestRate = interestRate
	vault.DesiredInterestRate = interestRate
	vault.PeriodStart = periodStart
	vault.PeriodTimeout = periodTimeout
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	if fundReserves {
		// Fund with enough to cover a day's interest
		err = FundAccount(s.ctx, s.simApp.BankKeeper, info.vaultAddr, sdk.NewCoins(sdk.NewInt64Coin(info.underlying.Denom, 1_000_000)))
		s.Require().NoError(err, "failed to fund vault reserves in createVaultWithInterest for vault %s", info.vaultAddr)
	}
	if fundPrincipal {
		err = FundAccount(s.ctx, s.simApp.BankKeeper, markertypes.MustGetMarkerAddress(info.shareDenom), sdk.NewCoins(info.underlying))
		s.Require().NoError(err, "failed to fund marker principal in createVaultWithInterest for marker %s", info.shareDenom)
	}

	s.Require().NoError(s.k.SafeAddPayoutVerification(s.ctx, vault), "failed to safe add payout verification in createVaultWithInterest for vault %s", info.vaultAddr)
	return vault
}

type VaultInfo struct {
	shareDenom string
	underlying sdk.Coin
	vaultAddr  sdk.AccAddress
}

func NewVaultInfo(id int) VaultInfo {
	shareDenom := fmt.Sprintf("vault%d", id)
	return VaultInfo{
		shareDenom: shareDenom,
		underlying: sdk.NewInt64Coin(fmt.Sprintf("underlying%d", id), 1_000_000_000),
		vaultAddr:  types.GetVaultAddress(shareDenom),
	}
}

func (s *TestSuite) TestKeeper_CanPayInterestDuration() {
	shareDenom := "vaultshares"
	underlying := sdk.NewInt64Coin("underlying", 1_000_000_000)
	vaultAddr := types.GetVaultAddress(shareDenom)
	markerAddr := markertypes.MustGetMarkerAddress(shareDenom)

	createVaultWithRate := func(rate string) *types.VaultAccount {
		s.requireAddFinalizeAndActivateMarker(underlying, s.adminAddr)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           s.adminAddr.String(),
			ShareDenom:      shareDenom,
			UnderlyingAsset: underlying.Denom,
		})
		s.Require().NoError(err, "failed to create vault in createVaultWithRate")

		vault, err := s.k.GetVault(s.ctx, vaultAddr)
		s.Require().NoError(err, "failed to get vault in createVaultWithRate")
		vault.CurrentInterestRate = rate
		s.k.AuthKeeper.SetAccount(s.ctx, vault)
		return vault
	}

	oneDay := int64(24 * time.Hour / time.Second)

	tests := []struct {
		name          string
		rate          string
		duration      int64
		fundReserves  sdkmath.Int
		fundPrincipal sdkmath.Int
		expectOK      bool
	}{
		{
			name:          "zero duration always OK",
			rate:          "0.25",
			duration:      0,
			fundReserves:  sdkmath.NewInt(0),
			fundPrincipal: sdkmath.NewInt(0),
			expectOK:      true,
		},
		{
			name:          "zero interest accrual is OK",
			rate:          "0.0",
			duration:      oneDay,
			fundReserves:  sdkmath.NewInt(0),
			fundPrincipal: sdkmath.NewInt(0),
			expectOK:      true,
		},
		{
			name:          "positive interest, sufficient reserves",
			rate:          "1.0",
			duration:      1,
			fundReserves:  sdkmath.NewInt(1_000_000_000),
			fundPrincipal: sdkmath.NewInt(100_000),
			expectOK:      true,
		},
		{
			name:          "positive interest, insufficient reserves",
			rate:          "1.0",
			duration:      oneDay,
			fundReserves:  sdkmath.NewInt(10),
			fundPrincipal: sdkmath.NewInt(100_000),
			expectOK:      false,
		},
		{
			name:          "negative interest, principal available",
			rate:          "-1.0",
			duration:      oneDay,
			fundReserves:  sdkmath.NewInt(0),
			fundPrincipal: sdkmath.NewInt(100_000),
			expectOK:      true,
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()

			vault := createVaultWithRate(tc.rate)

			if tc.fundReserves.IsPositive() {
				s.Require().NoError(FundAccount(
					s.ctx,
					s.simApp.BankKeeper,
					vaultAddr,
					sdk.NewCoins(sdk.NewCoin(underlying.Denom, tc.fundReserves)),
				), "failed to fund vault reserves for test case %s", tc.name)
			}

			if tc.fundPrincipal.IsPositive() {
				s.Require().NoError(FundAccount(
					s.ctx,
					s.simApp.BankKeeper,
					markerAddr,
					sdk.NewCoins(sdk.NewCoin(underlying.Denom, tc.fundPrincipal)),
				), "failed to fund marker principal for test case %s", tc.name)
			}

			ok, err := s.k.CanPayInterestDuration(s.ctx, vault, tc.duration)
			s.Require().NoError(err, "error checking CanPayInterestDuration")
			s.Require().Equal(tc.expectOK, ok, "unexpected CanPayInterestDuration result")
		})
	}
}

func (s *TestSuite) TestKeeper_CanPayInterestDuration_NegativeInterest_Composite_InsufficientUnderlying() {
	s.SetupTest()

	shareDenom := "vaultshares.composite"
	underlyingDenom := "uylds.fcc.receipt.token"
	paymentDenom := "uylds.fcc"

	underlying := sdk.NewInt64Coin(underlyingDenom, 1_000_000_000)
	vaultAddr := types.GetVaultAddress(shareDenom)
	markerAddr := markertypes.MustGetMarkerAddress(shareDenom)

	s.requireAddFinalizeAndActivateMarker(underlying, s.adminAddr)
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(paymentDenom, 10_000_000_000_000), s.adminAddr)

	_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
		Admin:           s.adminAddr.String(),
		ShareDenom:      shareDenom,
		UnderlyingAsset: underlyingDenom,
		PaymentDenom:    paymentDenom,
		InitialPaymentNav: &types.InitialVaultNAV{
			Price:  sdk.NewInt64Coin(underlyingDenom, 1),
			Volume: sdkmath.OneInt(),
		},
	})
	s.Require().NoError(err, "failed to create composite vault in TestKeeper_CanPayInterestDuration_NegativeInterest_Composite_InsufficientUnderlying")

	vault, err := s.k.GetVault(s.ctx, vaultAddr)
	s.Require().NoError(err, "failed to get composite vault in TestKeeper_CanPayInterestDuration_NegativeInterest_Composite_InsufficientUnderlying")

	vault.CurrentInterestRate = "-0.5"
	vault.DesiredInterestRate = "-0.5"
	s.k.AuthKeeper.SetAccount(s.ctx, vault)
	s.setVaultNAV(vault, paymentDenom, sdk.NewInt64Coin(underlyingDenom, 1), 1)

	s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, vaultAddr, sdk.NewCoins(sdk.NewCoin(underlyingDenom, sdkmath.NewInt(1_000_000)))), "failed to fund composite vault in TestKeeper_CanPayInterestDuration_NegativeInterest_Composite_InsufficientUnderlying")

	tinyUnderlying := sdkmath.NewInt(10_000_000)
	hugePayment := sdkmath.NewInt(10_000_000_000_000)

	s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, markerAddr, sdk.NewCoins(sdk.NewCoin(underlyingDenom, tinyUnderlying), sdk.NewCoin(paymentDenom, hugePayment))), "failed to fund marker account")

	year := int64(365 * 24 * time.Hour / time.Second)

	canPayLong, err := s.k.CanPayInterestDuration(s.ctx, vault, year)
	s.Require().NoError(err, "error checking CanPayInterestDuration for long duration")
	s.Require().False(canPayLong, "expected CanPayInterestDuration to be false for long duration with insufficient underlying")

	smallDuration := int64(1)

	canPayShort, err := s.k.CanPayInterestDuration(s.ctx, vault, smallDuration)
	s.Require().NoError(err, "error checking CanPayInterestDuration for short duration")
	s.Require().True(canPayShort, "expected CanPayInterestDuration to be true for short duration with sufficient underlying")
}

func (s *TestSuite) TestKeeper_PerformVaultInterestTransfer_PositiveInterest_UsesTVV() {
	s.SetupTest()

	shareDenom := "nvylds.shares"
	underlying := sdk.NewInt64Coin("uylds.fcc", 1_000_000_000)
	vaultAddr := types.GetVaultAddress(shareDenom)
	markerAddr := markertypes.MustGetMarkerAddress(shareDenom)
	now := time.Now().UTC()
	periodStart := now.Add(-60 * 24 * time.Hour).Unix()

	s.requireAddFinalizeAndActivateMarker(underlying, s.adminAddr)

	_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
		Admin:           s.adminAddr.String(),
		ShareDenom:      shareDenom,
		UnderlyingAsset: underlying.Denom,
	})
	s.Require().NoError(err, "failed to create vault")

	vault, err := s.k.GetVault(s.ctx, vaultAddr)
	s.Require().NoError(err, "failed to get vault in TestKeeper_PerformVaultInterestTransfer_PositiveInterest_UsesTVV")

	vault.CurrentInterestRate = "0.25"
	vault.DesiredInterestRate = "0.25"
	vault.PeriodStart = periodStart
	vault.FeePeriodStart = periodStart
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	s.Require().NoError(
		FundAccount(s.ctx, s.simApp.BankKeeper, vaultAddr, sdk.NewCoins(underlying)),
		"failed to fund vault account in TestKeeper_PerformVaultInterestTransfer_PositiveInterest_UsesTVV",
	)
	s.Require().NoError(
		FundAccount(s.ctx, s.simApp.BankKeeper, markerAddr, sdk.NewCoins(underlying)),
		"failed to fund marker account in TestKeeper_PerformVaultInterestTransfer_PositiveInterest_UsesTVV",
	)

	s.ctx = s.ctx.WithBlockTime(now).WithEventManager(sdk.NewEventManager())

	startVault := s.simApp.BankKeeper.GetBalance(s.ctx, vaultAddr, underlying.Denom).Amount
	startMarker := s.simApp.BankKeeper.GetBalance(s.ctx, markerAddr, underlying.Denom).Amount
	s.Require().Equal(underlying.Amount, startVault, "initial vault reserves should match funded amount")
	s.Require().Equal(underlying.Amount, startMarker, "initial marker principal should match funded amount")

	principalTvv, err := s.k.GetTVVInUnderlyingAsset(s.ctx, *vault)
	s.Require().NoError(err, "failed to get TVV in underlying asset")
	s.Require().True(principalTvv.GT(sdkmath.ZeroInt()), "expected positive TVV")

	principalCoin := sdk.NewCoin(underlying.Denom, principalTvv)
	periodDuration := s.ctx.BlockTime().Unix() - vault.PeriodStart

	interestEarned, err := interest.CalculateInterestEarned(principalCoin, vault.CurrentInterestRate, periodDuration)
	s.Require().NoError(err, "failed to calculate interest earned")
	s.Require().True(interestEarned.IsPositive(), "expected positive interest earned")

	err = s.k.TestAccessor_reconcileVault(s.T(), s.ctx, vault)
	s.Require().NoError(err, "failed to reconcile vault")

	endVault := s.simApp.BankKeeper.GetBalance(s.ctx, vaultAddr, underlying.Denom).Amount
	endMarker := s.simApp.BankKeeper.GetBalance(s.ctx, markerAddr, underlying.Denom).Amount

	expectedVault := startVault.Sub(interestEarned)
	// Fee for 1,041,952,013 TVV for 60 days is 256,919.
	expectedMarker := startMarker.Add(interestEarned).Sub(sdkmath.NewInt(256_919))

	s.Require().Equal(expectedVault, endVault, "vault reserves mismatch")
	s.Require().Equal(sdkmath.NewInt(1_041_695_094), endMarker, "marker principal mismatch")

	s.assertVaultAndMarkerBalances(
		vaultAddr,
		shareDenom,
		underlying.Denom,
		expectedVault,
		expectedMarker,
	)

	events := normalizeEvents(s.ctx.EventManager().Events())

	found := false
	for _, event := range events {
		if event.Type == "provlabs.vault.v1.EventVaultReconcile" {
			found = true

			var principalBeforeStr, principalAfterStr, interestStr string
			for _, attr := range event.Attributes {
				switch string(attr.Key) {
				case "principal_before":
					principalBeforeStr = string(attr.Value)
				case "principal_after":
					principalAfterStr = string(attr.Value)
				case "interest_earned":
					interestStr = string(attr.Value)
				}
			}

			s.Require().Equal(
				fmt.Sprintf("%s%s", startMarker.String(), underlying.Denom),
				principalBeforeStr,
				"principal before mismatch",
			)

			s.Require().Equal(
				fmt.Sprintf("%s%s", startMarker.Add(interestEarned).String(), underlying.Denom),
				principalAfterStr,
				"principal after mismatch",
			)

			s.Require().Equal(
				fmt.Sprintf("%s%s", interestEarned.String(), underlying.Denom),
				interestStr,
				"interest earned mismatch",
			)

			break
		}
	}
	s.Require().True(found, "expected EventVaultReconcile to be emitted")
}

func (s *TestSuite) TestKeeper_PerformVaultInterestTransfer_PositiveInterest_UsesCompositeTVV() {
	s.SetupTest()

	shareDenom := "nvylds.shares.composite"
	underlying := sdk.NewInt64Coin("uylds.fcc.receipt.token", 1_000_000_000)
	paymentDenom := "uylds.fcc"

	vaultAddr := types.GetVaultAddress(shareDenom)
	markerAddr := markertypes.MustGetMarkerAddress(shareDenom)
	now := time.Now().UTC()
	periodStart := now.Add(-60 * 24 * time.Hour).Unix()

	s.requireAddFinalizeAndActivateMarker(underlying, s.adminAddr)
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(paymentDenom, 1_000_000_000), s.adminAddr)

	_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
		Admin:           s.adminAddr.String(),
		ShareDenom:      shareDenom,
		UnderlyingAsset: underlying.Denom,
		PaymentDenom:    paymentDenom,
		InitialPaymentNav: &types.InitialVaultNAV{
			Price:  sdk.NewInt64Coin(underlying.Denom, 1),
			Volume: sdkmath.OneInt(),
		},
	})
	s.Require().NoError(err, "expected CreateVault to succeed")

	vault, err := s.k.GetVault(s.ctx, vaultAddr)
	s.Require().NoError(err, "expected GetVault to succeed after CreateVault")

	vault.CurrentInterestRate = "0.25"
	vault.DesiredInterestRate = "0.25"
	vault.PeriodStart = periodStart
	vault.FeePeriodStart = periodStart
	s.k.AuthKeeper.SetAccount(s.ctx, vault)
	s.setVaultNAV(vault, paymentDenom, sdk.NewInt64Coin(underlying.Denom, 1), 1)

	s.Require().NoError(
		FundAccount(s.ctx, s.simApp.BankKeeper, vaultAddr, sdk.NewCoins(underlying)),
		"expected funding vault reserves to succeed",
	)

	receiptPortion := sdkmath.NewInt(950_000_000)
	paymentPortion := sdkmath.NewInt(50_000_000)

	s.Require().NoError(
		FundAccount(s.ctx, s.simApp.BankKeeper, markerAddr, sdk.NewCoins(
			sdk.NewCoin(underlying.Denom, receiptPortion),
			sdk.NewCoin(paymentDenom, paymentPortion),
		)),
		"expected funding composite principal to succeed",
	)

	s.ctx = s.ctx.WithBlockTime(now).WithEventManager(sdk.NewEventManager())

	startVault := s.simApp.BankKeeper.GetBalance(s.ctx, vaultAddr, underlying.Denom).Amount
	startMarkerUnderlying := s.simApp.BankKeeper.GetBalance(s.ctx, markerAddr, underlying.Denom).Amount
	startMarkerPayment := s.simApp.BankKeeper.GetBalance(s.ctx, markerAddr, paymentDenom).Amount

	s.Require().Equal(underlying.Amount, startVault, "expected initial vault reserves to equal funded amount")
	s.Require().Equal(receiptPortion, startMarkerUnderlying, "expected marker underlying balance to equal receipt portion")
	s.Require().Equal(paymentPortion, startMarkerPayment, "expected marker payment balance to equal payment portion")

	principalTvv, err := s.k.GetTVVInUnderlyingAsset(s.ctx, *vault)
	s.Require().NoError(err, "expected GetTVVInUnderlyingAsset to succeed")
	s.Require().True(principalTvv.GT(sdkmath.ZeroInt()), "expected TVV principal to be positive for composite principal")

	principalCoin := sdk.NewCoin(underlying.Denom, principalTvv)
	periodDuration := s.ctx.BlockTime().Unix() - vault.PeriodStart

	interestEarned, err := interest.CalculateInterestEarned(principalCoin, vault.CurrentInterestRate, periodDuration)
	s.Require().NoError(err, "expected CalculateInterestEarned to succeed")
	s.Require().True(interestEarned.IsPositive(), "expected interest earned to be positive for positive rate")

	err = s.k.TestAccessor_reconcileVault(s.T(), s.ctx, vault)
	s.Require().NoError(err, "expected reconcileVault to succeed")

	endVault := s.simApp.BankKeeper.GetBalance(s.ctx, vaultAddr, underlying.Denom).Amount
	endMarkerUnderlying := s.simApp.BankKeeper.GetBalance(s.ctx, markerAddr, underlying.Denom).Amount
	endMarkerPayment := s.simApp.BankKeeper.GetBalance(s.ctx, markerAddr, paymentDenom).Amount

	expectedVault := startVault.Sub(interestEarned)
	expectedMarkerUnderlying := startMarkerUnderlying.Add(interestEarned)
	// Fee: 1,041,952,013 * 0.0015 * 5,184,000 / 31,536,000 = 256,920

	s.Require().Equal(expectedVault, endVault, "expected vault reserves to decrease by TVV-based interest")
	s.Require().Equal(expectedMarkerUnderlying, endMarkerUnderlying, "expected marker underlying balance to increase by TVV-based interest")
	s.Require().Equal(sdkmath.NewInt(49_743_081), endMarkerPayment, "expected marker payment token balance to decrease by AUM fee")

	s.assertVaultAndMarkerBalances(
		vaultAddr,
		shareDenom,
		underlying.Denom,
		expectedVault,
		expectedMarkerUnderlying,
	)

	events := normalizeEvents(s.ctx.EventManager().Events())

	found := false
	for _, ev := range events {
		if ev.Type == "provlabs.vault.v1.EventVaultReconcile" {
			found = true

			var principalBeforeStr, principalAfterStr, interestStr string
			for _, attr := range ev.Attributes {
				switch string(attr.Key) {
				case "principal_before":
					principalBeforeStr = string(attr.Value)
				case "principal_after":
					principalAfterStr = string(attr.Value)
				case "interest_earned":
					interestStr = string(attr.Value)
				}
			}

			expectedPrincipalBefore := sdk.NewCoin(underlying.Denom, principalTvv)
			// principal_after in EventVaultReconcile reflects state AFTER interest but BEFORE AUM fees.
			expectedPrincipalAfter := expectedPrincipalBefore.Add(sdk.NewCoin(underlying.Denom, interestEarned))

			s.Require().Equal(
				expectedPrincipalBefore.String(),
				principalBeforeStr,
				"expected principal_before to reflect TVV-based principal before transfer",
			)

			s.Require().Equal(
				expectedPrincipalAfter.String(),
				principalAfterStr,
				"expected principal_after to reflect TVV-based principal after transfer",
			)

			s.Require().Equal(
				fmt.Sprintf("%s%s", interestEarned.String(), underlying.Denom),
				interestStr,
				"expected interest_earned to reflect TVV-based interest amount",
			)

			break
		}
	}
	s.Require().True(found, "expected EventVaultReconcile to be emitted for composite principal TVV transfer")
}

func (s *TestSuite) TestKeeper_PerformVaultInterestTransfer_NegativeInterest_PartialLiquidation() {
	s.SetupTest()

	shareDenom := "nvylds.shares.liquid"
	underlying := sdk.NewInt64Coin("uylds.fcc", 1_000_000_000)
	vaultAddr := types.GetVaultAddress(shareDenom)
	markerAddr := markertypes.MustGetMarkerAddress(shareDenom)
	now := time.Now().UTC()
	periodStart := now.Add(-365 * 24 * time.Hour).Unix()

	s.requireAddFinalizeAndActivateMarker(underlying, s.adminAddr)

	_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
		Admin:           s.adminAddr.String(),
		ShareDenom:      shareDenom,
		UnderlyingAsset: underlying.Denom,
	})
	s.Require().NoError(err, "CreateVault should succeed")

	vault, err := s.k.GetVault(s.ctx, vaultAddr)
	s.Require().NoError(err, "GetVault should return the created vault")

	vault.CurrentInterestRate = "-10.0"
	vault.DesiredInterestRate = "-10.0"
	vault.PeriodStart = periodStart
	vault.FeePeriodStart = now.Unix()
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, vaultAddr, sdk.NewCoins(underlying)), "Funding vault should succeed")

	smallPrincipal := sdk.NewInt64Coin(underlying.Denom, 100_000)
	s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, markerAddr, sdk.NewCoins(smallPrincipal)), "Funding marker with small principal should succeed")

	s.ctx = s.ctx.WithBlockTime(now).WithEventManager(sdk.NewEventManager())

	err = s.k.TestAccessor_reconcileVault(s.T(), s.ctx, vault)
	s.Require().NoError(err, "ReconcileVault should not error during partial liquidation")

	endMarker := s.simApp.BankKeeper.GetBalance(s.ctx, markerAddr, underlying.Denom)
	s.Require().True(endMarker.IsZero(), "Marker balance should be fully liquidated to zero")

	endVault := s.simApp.BankKeeper.GetBalance(s.ctx, vaultAddr, underlying.Denom)
	s.T().Logf("End vault balance: %s", endVault.String())

	events := normalizeEvents(s.ctx.EventManager().Events())
	for _, ev := range events {
		s.T().Logf("Event: %s", ev.Type)
		for _, attr := range ev.Attributes {
			s.T().Logf("  %s: %s", attr.Key, attr.Value)
		}
	}

	expectedVaultBalance := underlying.Amount.Add(smallPrincipal.Amount)
	s.Require().Equal(expectedVaultBalance, endVault.Amount, "Vault should receive exactly the available marker balance")
	found := false
	for _, ev := range events {
		if ev.Type == "provlabs.vault.v1.EventVaultReconcile" {
			found = true
			for _, attr := range ev.Attributes {
				if string(attr.Key) == "interest_earned" {
					expectedStr := smallPrincipal.Amount.Neg().String() + underlying.Denom
					s.Require().Equal(expectedStr, string(attr.Value), "Event interest_earned should reflect the capped liquidation amount")
				}
			}
		}
	}
	s.Require().True(found, "EventVaultReconcile should be emitted")
}

func (s *TestSuite) TestKeeper_PerformVaultInterestTransfer_NegativeInterest_Composite_DepletesUnderlying() {
	s.SetupTest()

	shareDenom := "nvylds.shares.composite.neg"
	underlyingDenom := "uylds.fcc.receipt"
	paymentDenom := "uylds.fcc"

	vaultAddr := types.GetVaultAddress(shareDenom)
	markerAddr := markertypes.MustGetMarkerAddress(shareDenom)
	now := time.Now().UTC()
	periodStart := now.Add(-365 * 24 * time.Hour).Unix()

	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlyingDenom, 1000), s.adminAddr)
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(paymentDenom, 1_000_000_000), s.adminAddr)

	_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
		Admin:           s.adminAddr.String(),
		ShareDenom:      shareDenom,
		UnderlyingAsset: underlyingDenom,
		PaymentDenom:    paymentDenom,
		InitialPaymentNav: &types.InitialVaultNAV{
			Price:  sdk.NewInt64Coin(underlyingDenom, 1),
			Volume: sdkmath.OneInt(),
		},
	})
	s.Require().NoError(err, "CreateVault with composite structure should succeed")

	vault, err := s.k.GetVault(s.ctx, vaultAddr)
	s.Require().NoError(err, "GetVault should return the created vault")

	vault.CurrentInterestRate = "-0.5"
	vault.DesiredInterestRate = "-0.5"
	vault.PeriodStart = periodStart
	vault.FeePeriodStart = periodStart
	s.k.AuthKeeper.SetAccount(s.ctx, vault)
	s.setVaultNAV(vault, paymentDenom, sdk.NewInt64Coin(underlyingDenom, 1), 1)

	s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, vaultAddr, sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, 1_000_000))), "Funding vault should succeed")

	hugeOtherBalance := sdk.NewInt64Coin(paymentDenom, 1_000_000_000)
	tinyUnderlyingBalance := sdk.NewInt64Coin(underlyingDenom, 10)

	s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, markerAddr, sdk.NewCoins(hugeOtherBalance, tinyUnderlyingBalance)), "Funding marker with composite assets should succeed")

	s.ctx = s.ctx.WithBlockTime(now).WithEventManager(sdk.NewEventManager())

	tvv, err := s.k.GetTVVInUnderlyingAsset(s.ctx, *vault)
	s.Require().NoError(err, "GetTVVInUnderlyingAsset should succeed")
	s.Require().True(tvv.GT(sdkmath.NewInt(100_000)), "TVV should be significantly higher than the underlying balance due to secondary assets")

	err = s.k.TestAccessor_reconcileVault(s.T(), s.ctx, vault)
	s.Require().NoError(err, "ReconcileVault should not error even if underlying liquidity is insufficient for full negative interest")

	endUnderlying := s.simApp.BankKeeper.GetBalance(s.ctx, markerAddr, underlyingDenom)
	endOther := s.simApp.BankKeeper.GetBalance(s.ctx, markerAddr, paymentDenom)

	s.Require().True(endUnderlying.IsZero(), "Underlying asset should be fully depleted")
	// TVV: 1,000,000,000 other + 10 underlying = 1,000,000,010
	// Fee = 1,000,000,010 * 0.0015 * 1 year / 1 year = 1,500,000
	expectedOther := hugeOtherBalance.Amount.Sub(sdkmath.NewInt(1_500_000))
	s.Require().Equal(expectedOther, endOther.Amount, "Secondary asset balance should decrease by AUM fee")
}

func (s *TestSuite) TestKeeper_setShareDenomNAV() {
	shareDenom := "vaultshares"
	underlyingDenom := "underlying"
	vaultAddr := types.GetVaultAddress(shareDenom)
	markerAddr := markertypes.MustGetMarkerAddress(shareDenom)

	validShares := sdk.NewInt64Coin(shareDenom, 1_000_000)

	maxU64 := new(big.Int).SetUint64(^uint64(0))
	overflowShares := sdk.NewCoin(
		shareDenom,
		sdkmath.NewIntFromBigInt(new(big.Int).Add(maxU64, big.NewInt(1))),
	)

	tvv := sdkmath.NewInt(123_456)

	setup := func(shares sdk.Coin) (*types.VaultAccount, markertypes.MarkerAccountI) {
		s.requireAddFinalizeAndActivateMarker(
			sdk.NewInt64Coin(underlyingDenom, 1),
			s.adminAddr,
		)

		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           s.adminAddr.String(),
			ShareDenom:      shareDenom,
			UnderlyingAsset: underlyingDenom,
		})
		s.Require().NoError(err, "CreateVault should not error in setup")

		vault, err := s.k.GetVault(s.ctx, vaultAddr)
		s.Require().NoError(err, "GetVault should not error in setup")
		s.Require().NotNil(vault, "vault should not be nil in setup")

		vault.TotalShares = shares
		s.k.AuthKeeper.SetAccount(s.ctx, vault)

		marker, err := s.k.MarkerKeeper.GetMarker(s.ctx, markerAddr)
		s.Require().NoError(err, "GetMarker should not error in setup")
		s.Require().NotNil(marker, "marker should not be nil in setup")

		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
		return vault, marker
	}

	tests := []struct {
		name           string
		shares         sdk.Coin
		expectErr      bool
		expectNAVEvent bool
	}{
		{
			name:           "valid shares publishes NAV",
			shares:         validShares,
			expectErr:      false,
			expectNAVEvent: true,
		},
		{
			name:           "overflow shares returns error and skips NAV",
			shares:         overflowShares,
			expectErr:      true,
			expectNAVEvent: false,
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			vault, marker := setup(tc.shares)

			s.Require().NotNil(vault, "vault should not be nil for test case %q", tc.name)
			s.Require().NotNil(marker, "marker should not be nil for test case %q", tc.name)

			var err error
			s.Require().NotPanics(
				func() {
					err = s.k.TestAccessor_setShareDenomNAV(
						s.T(),
						s.ctx,
						vault,
						marker,
						tvv,
					)
				},
				"setShareDenomNAV should not panic for test case %q (shares=%s)",
				tc.name,
				tc.shares.String(),
			)

			if tc.expectErr {
				s.Require().Error(err, "expected setShareDenomNAV to return an error for test case %q (shares=%s)", tc.name, tc.shares.String())
			} else {
				s.Require().NoError(err, "expected setShareDenomNAV to succeed for test case %q (shares=%s)", tc.name, tc.shares.String())
			}

			events := normalizeEvents(s.ctx.EventManager().Events())

			found := false
			for _, ev := range events {
				if ev.Type == "provenance.marker.v1.EventSetNetAssetValue" {
					found = true
					break
				}
			}

			s.Require().Equal(
				tc.expectNAVEvent,
				found,
				"NAV event presence mismatch for test case %q: expected=%t got=%t (shares=%s)",
				tc.name,
				tc.expectNAVEvent,
				found,
				tc.shares.String(),
			)
		})
	}
}

func (s *TestSuite) TestKeeper_PerformVaultFeeTransfer() {
	tests := []struct {
		name                string
		paymentDenom        string
		aumFeeBips          uint32
		initialLiquidity    sdkmath.Int
		expectedFeeTotal    sdkmath.Int
		expectedCollected   sdkmath.Int
		expectedOutstanding sdkmath.Int
		secondLiquidity     sdkmath.Int
		expectedFinalMarker sdkmath.Int
		aumSnapshot         string
	}{
		{
			name:                "partial collection then full (default 15 bps)",
			paymentDenom:        "uylds.fcc",
			aumFeeBips:          15,
			initialLiquidity:    sdkmath.NewInt(100_000),
			expectedFeeTotal:    sdkmath.NewInt(246_600),
			expectedCollected:   sdkmath.NewInt(100_000),
			expectedOutstanding: sdkmath.NewInt(146_600),
			secondLiquidity:     sdkmath.NewInt(1_000_000),
			expectedFinalMarker: sdkmath.NewInt(853_400),
			aumSnapshot:         "1000100000uylds.fcc.receipt",
		},
		{
			name:                "partial collection then full (100 bps)",
			paymentDenom:        "uylds.fcc",
			aumFeeBips:          100,
			initialLiquidity:    sdkmath.NewInt(100_000),
			expectedFeeTotal:    sdkmath.NewInt(1_644_000),
			expectedCollected:   sdkmath.NewInt(100_000),
			expectedOutstanding: sdkmath.NewInt(1_544_000),
			secondLiquidity:     sdkmath.NewInt(2_000_000),
			expectedFinalMarker: sdkmath.NewInt(456_000),
			aumSnapshot:         "1000100000uylds.fcc.receipt",
		},
		{
			name:                "zero fee collection",
			paymentDenom:        "uylds.fcc",
			aumFeeBips:          0,
			initialLiquidity:    sdkmath.NewInt(100_000),
			expectedFeeTotal:    sdkmath.NewInt(0),
			expectedCollected:   sdkmath.NewInt(0),
			expectedOutstanding: sdkmath.NewInt(0),
			secondLiquidity:     sdkmath.NewInt(0),
			expectedFinalMarker: sdkmath.NewInt(100_000),
			aumSnapshot:         "1000100000uylds.fcc.receipt",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			shareDenom := "fee.shares"
			underlyingDenom := "uylds.fcc.receipt"
			underlying := sdk.NewInt64Coin(underlyingDenom, 1_000_000_000)
			vaultAddr := types.GetVaultAddress(shareDenom)
			now := s.ctx.BlockTime()
			twoMonthsAgo := now.Add(-60 * 24 * time.Hour)

			s.requireAddFinalizeAndActivateMarker(underlying, s.adminAddr)
			s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(tc.paymentDenom, 1_000_000_000), s.adminAddr)
			vault := s.CreateVaultWithParams(shareDenom, underlyingDenom, tc.paymentDenom)
			vault.AumFeeBips = tc.aumFeeBips
			s.SetVaultRatesAndPeriod(vault, "0.0", "0.0", twoMonthsAgo.Unix(), 0)
			s.setVaultNAV(vault, tc.paymentDenom, sdk.NewInt64Coin(underlyingDenom, 1), 1)

			s.FundMarker(shareDenom, sdk.NewCoins(underlying))
			s.FundMarker(shareDenom, sdk.NewCoins(sdk.NewCoin(tc.paymentDenom, tc.initialLiquidity)))

			s.SetCtxBlockTime(now)

			err := s.k.PerformVaultFeeTransfer(s.ctx, vault)
			s.Require().NoError(err, "PerformVaultFeeTransfer should not error during initial collection")
			s.Require().NoError(s.k.SetVaultAccount(s.ctx, vault), "SetVaultAccount should succeed after initial collection")

			// Verify partial collection
			provlabsAddr, err := s.k.GetAUMFeeAddress(s.ctx)
			s.Require().NoError(err, "failed to get AUM fee address from GetAUMFeeAddress")

			s.assertBalance(provlabsAddr, tc.paymentDenom, tc.expectedCollected)
			s.Require().Equal(tc.expectedOutstanding, vault.OutstandingAumFee.Amount, "outstanding fee balance mismatch")

			// Check event
			events := normalizeEvents(s.ctx.EventManager().Events())
			found := false
			for _, ev := range events {
				if ev.Type == "provlabs.vault.v1.EventVaultFeeCollected" {
					found = true
					snapshot, err := sdk.ParseCoinNormalized(tc.aumSnapshot)
					s.Require().NoError(err, "failed to parse aum snapshot coin")
					expectedEv := createVaultFeeCollectedEvent(
						vaultAddr,
						snapshot,
						sdk.NewCoin(tc.paymentDenom, tc.expectedCollected),
						sdk.NewCoin(tc.paymentDenom, tc.expectedFeeTotal),
						sdk.NewCoin(tc.paymentDenom, tc.expectedOutstanding),
						int64(60*24*time.Hour/time.Second),
					)
					s.Assert().Equal(normalizeEvent(expectedEv), ev, "EventVaultFeeCollected mismatch")
				}
			}
			s.Require().Equal(!tc.expectedFeeTotal.IsZero(), found, "EventVaultFeeCollected emission mismatch")

			if tc.expectedFeeTotal.IsZero() {
				return
			}

			// Second collection
			s.SetCtxBlockTime(now.Add(time.Second))
			s.FundMarker(shareDenom, sdk.NewCoins(sdk.NewCoin(tc.paymentDenom, tc.secondLiquidity)))

			err = s.k.PerformVaultFeeTransfer(s.ctx, vault)
			s.Require().NoError(err, "PerformVaultFeeTransfer should not error during second collection")
			s.Require().NoError(s.k.SetVaultAccount(s.ctx, vault), "SetVaultAccount should succeed after second collection")

			vault, err = s.k.GetVault(s.ctx, types.GetVaultAddress(shareDenom))
			s.Require().NoError(err, "failed to get vault for share denom %s", shareDenom)
			s.Require().Equal(s.ctx.BlockTime().Unix(), vault.FeePeriodStart, "FeePeriodStart should be updated")
			s.Require().True(vault.OutstandingAumFee.IsZero(), "outstanding fee should be cleared")
			s.assertBalance(markertypes.MustGetMarkerAddress(shareDenom), tc.paymentDenom, tc.expectedFinalMarker)
		})
	}
}

func (s *TestSuite) TestKeeper_PerformVaultFeeTransfer_OversizedTVVDegradesToError() {
	vault, _, underlyingDenom, paymentDenom := s.setupOversizedNAVVault()
	s.seedOversizedNAV(vault, paymentDenom, underlyingDenom, maxValidNAVPrice(), sdkmath.OneInt())

	principalAddress := vault.PrincipalMarkerAddress()
	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, principalAddress, sdk.NewCoins(
		sdk.NewInt64Coin(paymentDenom, 1),
	)), "funding principal with one payment unit should drive TVV to the 256-bit ceiling")

	now := s.ctx.BlockTime()
	twoMonthsAgo := now.Add(-60 * 24 * time.Hour)
	vault.AumFeeBips = 10_000
	s.SetVaultRatesAndPeriod(vault, "0.0", "0.0", twoMonthsAgo.Unix(), 0)
	s.SetCtxBlockTime(now)

	err := s.k.PerformVaultFeeTransfer(s.ctx, vault)
	s.Require().Error(err, "an oversized TVV must degrade to an error, not panic the BeginBlock fee hook")
	s.Require().ErrorContains(err, "overflow", "error should originate from the CalculateAUMFee recover guard")
}

func (s *TestSuite) TestKeeper_PerformVaultFeeTransfer_OutstandingFeeOverflowDegradesToError() {
	s.SetupTest()
	shareDenom := "fee.shares"
	underlyingDenom := "uylds.fcc.receipt"
	paymentDenom := "uylds.fcc"
	underlying := sdk.NewInt64Coin(underlyingDenom, 1_000_000_000)
	now := s.ctx.BlockTime()
	twoMonthsAgo := now.Add(-60 * 24 * time.Hour)

	s.requireAddFinalizeAndActivateMarker(underlying, s.adminAddr)
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(paymentDenom, 1_000_000_000), s.adminAddr)
	vault := s.CreateVaultWithParams(shareDenom, underlyingDenom, paymentDenom)
	vault.AumFeeBips = 100
	vault.OutstandingAumFee = sdk.NewCoin(paymentDenom, maxValidNAVPrice())
	s.SetVaultRatesAndPeriod(vault, "0.0", "0.0", twoMonthsAgo.Unix(), 0)

	s.FundMarker(shareDenom, sdk.NewCoins(underlying))
	s.FundMarker(shareDenom, sdk.NewCoins(sdk.NewCoin(paymentDenom, sdkmath.NewInt(100_000))))
	s.SetCtxBlockTime(now)

	err := s.k.PerformVaultFeeTransfer(s.ctx, vault)
	s.Require().Error(err, "an outstanding fee at the 256-bit ceiling must degrade to an error, not panic the BeginBlock fee hook")
	s.Require().ErrorContains(err, "overflow", "error should originate from the SafeAdd guard on outstanding AUM fee")
}

func (s *TestSuite) TestKeeper_CanPayInterestDuration_WithAUMFee() {
	s.SetupTest()
	shareDenom := "fee.payout.shares"
	underlyingDenom := "underlying"
	paymentDenom := "uylds.fcc"
	underlying := sdk.NewInt64Coin(underlyingDenom, 1_000_000_000)

	s.requireAddFinalizeAndActivateMarker(underlying, s.adminAddr)
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(paymentDenom, 1_000_000_000), s.adminAddr)
	vault := s.CreateVaultWithParams(shareDenom, underlyingDenom, paymentDenom)
	s.SetVaultRatesAndPeriod(vault, "0.0", "", 0, 0)
	s.FundMarker(shareDenom, sdk.NewCoins(underlying))

	year := int64(365 * 24 * time.Hour / time.Second)

	ok, err := s.k.CanPayInterestDuration(s.ctx, vault, year)
	s.Require().NoError(err, "CanPayInterestDuration returned error for vault=%s year=%d", vault.Address, year)
	s.Require().True(ok, "should succeed even when no payment denom liquidity for fees (deferred)")
}

func (s *TestSuite) TestKeeper_HandleVaultFeeTimeouts() {
	s.SetupTest()
	shareDenom := "fee.timeout.shares"
	underlyingDenom := "underlying"
	paymentDenom := "uylds.fcc"
	underlying := sdk.NewInt64Coin(underlyingDenom, 1_000_000_000)
	vaultAddr := types.GetVaultAddress(shareDenom)

	now := s.ctx.BlockTime()
	twoMonthsAgo := now.Add(-60 * 24 * time.Hour)

	s.requireAddFinalizeAndActivateMarker(underlying, s.adminAddr)
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(paymentDenom, 1_000_000_000), s.adminAddr)
	vault := s.CreateVaultWithParams(shareDenom, underlyingDenom, paymentDenom)
	s.SetVaultRatesAndPeriod(vault, "0.0", "0.0", twoMonthsAgo.Unix(), twoMonthsAgo.Unix())
	s.setVaultNAV(vault, paymentDenom, sdk.NewInt64Coin(underlyingDenom, 1), 1)
	s.FundMarker(shareDenom, sdk.NewCoins(underlying, sdk.NewInt64Coin(paymentDenom, 10_000_000)))

	s.Require().NoError(s.k.FeeTimeoutQueue.Enqueue(s.ctx, twoMonthsAgo.Unix(), vaultAddr), "failed to enqueue vault in FeeTimeoutQueue")

	err := s.k.TestAccessor_handleVaultFeeTimeouts(s.T(), s.ctx)
	s.Require().NoError(err, "handleVaultFeeTimeouts should succeed")

	provlabsAddr, err := s.k.GetAUMFeeAddress(s.ctx)
	s.Require().NoError(err, "failed to get AUM fee address from GetAUMFeeAddress")

	feeCollected := s.simApp.BankKeeper.GetBalance(s.ctx, provlabsAddr, paymentDenom).Amount
	s.Require().True(feeCollected.IsPositive(), "fee should be collected for address %s", provlabsAddr)

	found := false
	err = s.k.FeeTimeoutQueue.Walk(s.ctx, func(timeout uint64, addr sdk.AccAddress) (bool, error) {
		if addr.Equals(vaultAddr) {
			found = true
			s.Require().Greater(int64(timeout), now.Unix(), "new fee timeout should be in the future")
		}
		return false, nil
	})
	s.Require().NoError(err, "FeeTimeoutQueue.Walk returned unexpected error during verification")
	s.Require().True(found, "new fee timeout should be enqueued")

	vault, err = s.k.GetVault(s.ctx, vaultAddr)
	s.Require().NoError(err, "failed to get vault for address %s", vaultAddr)
	s.Require().Equal(s.ctx.BlockTime().Unix(), vault.FeePeriodStart, "FeePeriodStart should be updated")
}

func (s *TestSuite) TestKeeper_AccrualCalculations() {
	shareDenom := "vaultshares"
	underlyingDenom := "underlying"
	underlying := sdk.NewInt64Coin(underlyingDenom, 1_000_000_000)
	vaultAddress := types.GetVaultAddress(shareDenom)
	testBlockTime := time.Now().UTC()
	sixtyDays := -60 * 24 * time.Hour
	pastTime := testBlockTime.Add(sixtyDays)

	setup := func() *types.VaultAccount {
		s.requireAddFinalizeAndActivateMarker(underlying, s.adminAddr)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           s.adminAddr.String(),
			ShareDenom:      shareDenom,
			UnderlyingAsset: underlyingDenom,
		})
		s.Require().NoError(err, "failed to create vault in setup")

		vault, err := s.k.GetVault(s.ctx, vaultAddress)
		s.Require().NoError(err, "failed to get vault in setup")
		s.ctx = s.ctx.WithBlockTime(testBlockTime)
		return vault
	}

	s.Run("CalculateAccruedInterest", func() {
		s.SetupTest()
		vault := setup()

		tests := []struct {
			name     string
			rate     string
			start    int64
			expected sdkmath.Int
		}{
			{
				name:     "no interest rate set, should return zero interest",
				rate:     "",
				start:    pastTime.Unix(),
				expected: sdkmath.ZeroInt(),
			},
			{
				name:     "positive interest rate, should return correct accrual",
				rate:     "0.25",
				start:    pastTime.Unix(),
				expected: sdkmath.NewInt(41_952_013),
			},
			{
				name:     "negative interest rate, should return negative accrual",
				rate:     "-0.25",
				start:    pastTime.Unix(),
				expected: sdkmath.NewInt(-40_262_904),
			},
			{
				name:     "period start in the future, should return zero interest",
				rate:     "0.25",
				start:    testBlockTime.Add(time.Hour).Unix(),
				expected: sdkmath.ZeroInt(),
			},
		}

		for _, tc := range tests {
			s.Run(tc.name, func() {
				vault.CurrentInterestRate = tc.rate
				vault.PeriodStart = tc.start
				amt, err := s.k.CalculateAccruedInterest(s.ctx, *vault, underlying)
				s.Require().NoError(err, "accrued interest calculation failed for case: %s", tc.name)
				s.Require().Equal(tc.expected, amt, "interest accrual mismatch for case: %s", tc.name)
			})
		}
	})

	s.Run("CalculateAccruedAUMFee", func() {
		s.SetupTest()
		vault := setup()

		tests := []struct {
			name      string
			start     int64
			assets    sdkmath.Int
			remainder string
			expected  sdkmath.Int
		}{
			{
				name:     "no fee period start, should return zero fee",
				start:    0,
				assets:   underlying.Amount,
				expected: sdkmath.ZeroInt(),
			},
			{
				name:     "valid period elapsed, should return 15bps annual fee",
				start:    pastTime.Unix(),
				assets:   underlying.Amount,
				expected: sdkmath.NewInt(246_575),
			},
			{
				name:     "zero assets, should return zero fee",
				start:    pastTime.Unix(),
				assets:   sdkmath.ZeroInt(),
				expected: sdkmath.ZeroInt(),
			},
			{
				name:      "carried remainder crosses a whole unit, should recognize the extra unit",
				start:     pastTime.Unix(),
				assets:    underlying.Amount,
				remainder: "0.7",
				expected:  sdkmath.NewInt(246_576),
			},
			{
				name:      "carried remainder stays sub-unit, should not add a whole unit",
				start:     pastTime.Unix(),
				assets:    underlying.Amount,
				remainder: "0.5",
				expected:  sdkmath.NewInt(246_575),
			},
		}

		for _, tc := range tests {
			s.Run(tc.name, func() {
				vault.FeePeriodStart = tc.start
				vault.FeeRemainder = tc.remainder
				amt, err := s.k.CalculateAccruedAUMFee(s.ctx, *vault, tc.assets)
				s.Require().NoError(err, "accrued AUM fee calculation failed for case: %s", tc.name)
				s.Require().Equal(tc.expected, amt, "AUM fee mismatch for case: %s", tc.name)
			})
		}
	})

	s.Run("CalculateOutstandingFeeUnderlying", func() {
		s.SetupTest()
		vault := setup()

		tests := []struct {
			name         string
			paymentDenom string
			navPrice     int64
			navVolume    int64
			outstanding  sdk.Coin
			expected     sdkmath.Int
		}{
			{
				name:        "no outstanding fee, should return zero",
				outstanding: sdk.NewInt64Coin(underlyingDenom, 0),
				expected:    sdkmath.ZeroInt(),
			},
			{
				name:        "outstanding fee in underlying denom, should return same amount",
				outstanding: sdk.NewInt64Coin(underlyingDenom, 500),
				expected:    sdkmath.NewInt(500),
			},
			{
				name:         "outstanding fee in payment denom with 1:1 internal NAV, should return 1:1 amount",
				paymentDenom: "uylds.fcc",
				navPrice:     1,
				navVolume:    1,
				outstanding:  sdk.NewInt64Coin("uylds.fcc", 1000),
				expected:     sdkmath.NewInt(1000),
			},
		}

		for _, tc := range tests {
			s.Run(tc.name, func() {
				if tc.paymentDenom != "" {
					s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(tc.paymentDenom, 1_000_000), s.adminAddr)
					s.setVaultPaymentDenomWithNAV(vault, tc.paymentDenom, sdk.NewInt64Coin(underlyingDenom, tc.navPrice), tc.navVolume)
				}
				vault.OutstandingAumFee = tc.outstanding
				amt, err := s.k.CalculateOutstandingFeeUnderlying(s.ctx, *vault)
				s.Require().NoError(err, "outstanding fee conversion failed for case: %s", tc.name)
				s.Require().Equal(tc.expected, amt, "outstanding fee underlying amount mismatch for case: %s", tc.name)
			})
		}
	})

	s.Run("AccrueAUMFeePayment", func() {
		s.SetupTest()
		vault := setup()

		tests := []struct {
			name         string
			start        int64
			paymentDenom string
			setupNav     bool
			expected     sdk.Coin
		}{
			{
				name:         "no fee period start, should return zero coin",
				start:        0,
				paymentDenom: underlyingDenom,
				expected:     sdk.NewInt64Coin(underlyingDenom, 0),
			},
			{
				name:         "valid period with 1:1 payment denom, should return same amount",
				start:        pastTime.Unix(),
				paymentDenom: underlyingDenom,
				expected:     sdk.NewInt64Coin(underlyingDenom, 246_575),
			},
			{
				name:         "valid period with 1:2 NAV record, should return doubled amount",
				start:        pastTime.Unix(),
				paymentDenom: "usdc",
				setupNav:     true,
				expected:     sdk.NewInt64Coin("usdc", 493_150),
			},
		}

		for _, tc := range tests {
			s.Run(tc.name, func() {
				vault.FeePeriodStart = tc.start
				vault.PaymentDenom = tc.paymentDenom
				vault.FeeRemainder = ""
				s.k.AuthKeeper.SetAccount(s.ctx, vault)

				if tc.setupNav {
					s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(tc.paymentDenom, 1_000_000), s.adminAddr)
					// price=1 underlying per volume=2 payment → 1 payment redeems for 0.5 underlying,
					// so 1 underlying converts to 2 payment.
					s.setVaultNAV(vault, tc.paymentDenom, sdk.NewInt64Coin(underlyingDenom, 1), 2)
				}

				amt, err := s.k.AccrueAUMFeePayment(s.ctx, vault, underlying.Amount)
				s.Require().NoError(err, "accrued AUM fee payment calculation failed for case: %s", tc.name)
				s.Require().Equal(tc.expected, amt, "accrued AUM fee payment coin mismatch for case: %s", tc.name)
			})
		}
	})

	s.Run("AccrueAUMFeePayment dust accumulation preserves revenue across short windows", func() {
		s.SetupTest()
		vault := setup()
		vault.AumFeeBips = 15

		aum := underlying.Amount
		blockTime := int64(6)
		iterations := int64(100)
		base := pastTime.Unix()

		naiveTruncatedSum := sdkmath.ZeroInt()
		accumulatedSum := sdkmath.ZeroInt()
		for i := range iterations {
			naive, err := interest.CalculateAUMFee(aum, vault.AumFeeBips, blockTime)
			s.Require().NoError(err, "naive per-window fee calculation failed at iteration %d", i)
			naiveTruncatedSum = naiveTruncatedSum.Add(naive)

			vault.FeePeriodStart = base + i*blockTime
			s.ctx = s.ctx.WithBlockTime(time.Unix(base+(i+1)*blockTime, 0).UTC())
			collected, err := s.k.AccrueAUMFeePayment(s.ctx, vault, aum)
			s.Require().NoError(err, "AccrueAUMFeePayment failed at iteration %d", i)
			accumulatedSum = accumulatedSum.Add(collected.Amount)
		}

		perWindowDec, err := interest.CalculateAUMFeeDec(aum, vault.AumFeeBips, blockTime)
		s.Require().NoError(err, "per-window decimal fee calculation failed")
		expectedWhole := perWindowDec.MulInt64(iterations).TruncateInt()

		s.Require().True(naiveTruncatedSum.IsZero(),
			"per-window integer truncation should collect nothing over short windows, got %s", naiveTruncatedSum)
		s.Require().True(expectedWhole.IsPositive(),
			"the accumulated fee should recover a positive whole amount, got %s", expectedWhole)
		s.Require().Equal(expectedWhole, accumulatedSum,
			"accumulated fee should equal the truncated cumulative decimal fee (recovered=%s, expected=%s)", accumulatedSum, expectedWhole)

		remainder, err := sdkmath.LegacyNewDecFromStr(vault.FeeRemainder)
		s.Require().NoError(err, "final fee remainder %q should be parseable", vault.FeeRemainder)
		s.Require().True(!remainder.IsNegative() && remainder.LT(sdkmath.LegacyOneDec()),
			"fee remainder must be a fraction in [0,1), got %s", remainder)
	})

	s.Run("CalculateVaultTotalAssets", func() {
		s.SetupTest()
		vault := setup()

		tests := []struct {
			name        string
			rate        string
			periodStart int64
			feeStart    int64
			outstanding sdk.Coin
			expected    sdkmath.Int
		}{
			{
				name:        "static vault with no accruals, should return principal",
				rate:        "0.0",
				periodStart: 0,
				feeStart:    0,
				outstanding: sdk.NewInt64Coin(underlyingDenom, 0),
				expected:    underlying.Amount,
			},
			{
				name:        "active vault with interest and fees, should return net assets after all accruals",
				rate:        "0.25",
				periodStart: pastTime.Unix(),
				feeStart:    pastTime.Unix(),
				outstanding: sdk.NewInt64Coin(underlyingDenom, 1000),
				expected:    sdkmath.NewInt(1_041_694_094),
			},
		}

		for _, tc := range tests {
			s.Run(tc.name, func() {
				vault.CurrentInterestRate = tc.rate
				vault.PeriodStart = tc.periodStart
				vault.FeePeriodStart = tc.feeStart
				vault.OutstandingAumFee = tc.outstanding

				amt, err := s.k.CalculateVaultTotalAssets(s.ctx, vault, underlying)
				s.Require().NoError(err, "total assets calculation failed for case: %s", tc.name)
				s.Require().Equal(tc.expected, amt, "total vault assets mismatch for case: %s", tc.name)
			})
		}
	})
}

func (s *TestSuite) TestKeeper_HandleVaultFeeTimeouts_RetryOnFailure() {
	s.SetupTest()
	shareDenom := "fee.timeout.shares"
	underlyingDenom := "underlying"
	paymentDenom := "other" // not uylds.fcc, so it needs a NAV
	underlying := sdk.NewInt64Coin(underlyingDenom, 1_000_000_000)
	vaultAddr := types.GetVaultAddress(shareDenom)

	now := s.ctx.BlockTime()
	twoMonthsAgo := now.Add(-60 * 24 * time.Hour)

	s.requireAddFinalizeAndActivateMarker(underlying, s.adminAddr)
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(paymentDenom, 1_000_000_000), s.adminAddr)

	vault := s.CreateVaultWithParams(shareDenom, underlyingDenom, paymentDenom)
	// Remove the bootstrap NAV so PerformVaultFeeTransfer hits the missing-NAV path.
	s.Require().NoError(s.k.NAVs.Remove(s.ctx, collections.Join(vault.GetAddress(), paymentDenom)),
		"removing bootstrap NAV for %q should succeed", paymentDenom)

	// CreateVaultWithParams enqueues an initial timeout, we must remove it to have a clean test
	s.Require().NoError(s.k.FeeTimeoutQueue.Dequeue(s.ctx, vault.FeePeriodTimeout, vaultAddr), "failed to dequeue vault from FeeTimeoutQueue")

	s.SetVaultRatesAndPeriod(vault, "0.0", "0.0", twoMonthsAgo.Unix(), twoMonthsAgo.Unix())

	// Fund marker with some underlying so TVV is positive
	s.FundMarker(shareDenom, sdk.NewCoins(underlying))

	// Enqueue it
	s.Require().NoError(s.k.FeeTimeoutQueue.Enqueue(s.ctx, twoMonthsAgo.Unix(), vaultAddr), "failed to enqueue vault in FeeTimeoutQueue")

	// Call handleVaultFeeTimeouts.
	// PerformVaultFeeTransfer calls GetTVVInUnderlyingAsset.
	// GetTVVInUnderlyingAsset calls ToUnderlyingAssetAmount for all balances.
	// If we have a balance in paymentDenom ("other"), it will try to find a NAV to underlyingDenom.
	// Since there is no NAV, it should fail.

	// We don't fund the marker with 'other' denom here because that would make GetTVVInUnderlyingAsset fail.
	// Instead, we rely on the missing NAV for 'other' during fee payment conversion in PerformVaultFeeTransfer.

	err := s.k.TestAccessor_handleVaultFeeTimeouts(s.T(), s.ctx)
	s.Require().NoError(err, "handleVaultFeeTimeouts should not return error even if a vault fails")

	// Verify the vault is RE-ENQUEUED with a NEW timeout because we don't continue on PerformVaultFeeTransfer failure
	expectedTimeout := uint64(s.ctx.BlockTime().Unix() + keeper.AutoReconcileTimeout)
	found := false
	err = s.k.FeeTimeoutQueue.Walk(s.ctx, func(timeout uint64, addr sdk.AccAddress) (bool, error) {
		if addr.Equals(vaultAddr) {
			found = true
			s.Require().Equal(expectedTimeout, timeout, "vault should be rescheduled with new timeout")
		}
		return false, nil
	})
	s.Require().NoError(err, "FeeTimeoutQueue.Walk returned unexpected error during retry verification")
	s.Require().True(found, "vault should be in the fee timeout queue with new timeout")

	// Verify FeePeriodStart is preserved
	updatedVault, err := s.k.GetVault(s.ctx, vaultAddr)
	s.Require().NoError(err)
	s.Require().Equal(twoMonthsAgo.Unix(), updatedVault.FeePeriodStart, "FeePeriodStart should be preserved on transient failure")
}

func (s *TestSuite) TestKeeper_HandleVaultFeeTimeouts_Success() {
	s.SetupTest()
	shareDenom := "fee.success.shares"
	underlyingDenom := "underlying"
	paymentDenom := "uylds.fcc"
	underlying := sdk.NewInt64Coin(underlyingDenom, 1_000_000_000)
	vaultAddr := types.GetVaultAddress(shareDenom)

	now := s.ctx.BlockTime()
	twoMonthsAgo := now.Add(-60 * 24 * time.Hour)

	s.requireAddFinalizeAndActivateMarker(underlying, s.adminAddr)
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(paymentDenom, 1_000_000_000), s.adminAddr)

	vault := s.CreateVaultWithParams(shareDenom, underlyingDenom, paymentDenom)

	// CreateVaultWithParams enqueues an initial timeout, we must remove it to have a clean test
	s.Require().NoError(s.k.FeeTimeoutQueue.Dequeue(s.ctx, vault.FeePeriodTimeout, vaultAddr), "failed to dequeue vault from FeeTimeoutQueue")

	s.SetVaultRatesAndPeriod(vault, "0.0", "0.0", twoMonthsAgo.Unix(), twoMonthsAgo.Unix())

	// Fund marker with some underlying so TVV is positive
	s.FundMarker(shareDenom, sdk.NewCoins(underlying))

	// Enqueue it
	s.Require().NoError(s.k.FeeTimeoutQueue.Enqueue(s.ctx, twoMonthsAgo.Unix(), vaultAddr), "failed to enqueue vault in FeeTimeoutQueue")

	// Call handleVaultFeeTimeouts. Success this time (uylds.fcc doesn't need NAV)
	err := s.k.TestAccessor_handleVaultFeeTimeouts(s.T(), s.ctx)
	s.Require().NoError(err, "handleVaultFeeTimeouts should succeed")

	// Verify the vault is DEQUEUED from old timeout and ENQUEUED with new timeout
	foundOld := false
	foundNew := false
	err = s.k.FeeTimeoutQueue.Walk(s.ctx, func(timeout uint64, addr sdk.AccAddress) (bool, error) {
		if addr.Equals(vaultAddr) {
			if timeout == uint64(twoMonthsAgo.Unix()) {
				foundOld = true
			} else if timeout > uint64(now.Unix()) {
				foundNew = true
			}
		}
		return false, nil
	})
	s.Require().NoError(err, "FeeTimeoutQueue.Walk returned unexpected error during verification")
	s.Require().False(foundOld, "vault should be dequeued from old timeout")
	s.Require().True(foundNew, "vault should be enqueued with new timeout")
}

func (s *TestSuite) TestKeeper_HandleVaultInterestTimeouts_RetryOnFailure() {
	s.SetupTest()
	shareDenom := "interest.timeout.shares"
	underlyingDenom := "underlying"
	paymentDenom := "other" // not uylds.fcc, so it needs a NAV
	underlying := sdk.NewInt64Coin(underlyingDenom, 1_000_000_000)
	vaultAddr := types.GetVaultAddress(shareDenom)

	now := s.ctx.BlockTime()
	twoMonthsAgo := now.Add(-60 * 24 * time.Hour)

	s.requireAddFinalizeAndActivateMarker(underlying, s.adminAddr)
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(paymentDenom, 1_000_000_000), s.adminAddr)

	vault := s.CreateVaultWithParams(shareDenom, underlyingDenom, paymentDenom)
	// Remove the bootstrap NAV so CanPayInterestDuration hits the missing-NAV path
	// when GetTVVInUnderlyingAsset iterates the principal balance.
	s.Require().NoError(s.k.NAVs.Remove(s.ctx, collections.Join(vault.GetAddress(), paymentDenom)),
		"removing bootstrap NAV for %q should succeed", paymentDenom)

	// CreateVaultWithParams enqueues an initial fee timeout, and also sets up the vault.
	// We want to test interest timeouts, which use PayoutTimeoutQueue.

	s.SetVaultRatesAndPeriod(vault, "0.1", "0.1", twoMonthsAgo.Unix(), twoMonthsAgo.Unix())
	vault.PeriodStart = twoMonthsAgo.Unix()
	vault.PeriodTimeout = twoMonthsAgo.Unix()
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	// Fund marker with some underlying so TVV is positive
	s.FundMarker(shareDenom, sdk.NewCoins(underlying))

	// Enqueue it in PayoutTimeoutQueue
	s.Require().NoError(s.k.PayoutTimeoutQueue.Enqueue(s.ctx, twoMonthsAgo.Unix(), vaultAddr), "failed to enqueue vault")

	// Trigger an error in CanPayInterestDuration by funding the principal marker with a denom
	// that lacks a NAV conversion rate to the underlying asset.
	// This simulates a transient error without making the VaultAccount itself invalid.
	s.FundMarker(shareDenom, sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 1000)))

	err := s.k.TestAccessor_handleVaultInterestTimeouts(s.T(), s.ctx)
	s.Require().NoError(err, "handleVaultInterestTimeouts should not return error")

	// Verify the vault is RE-ENQUEUED with a NEW timeout because we reschedule on failure
	expectedTimeout := uint64(s.ctx.BlockTime().Unix() + keeper.AutoReconcileTimeout)
	found := false
	err = s.k.PayoutTimeoutQueue.Walk(s.ctx, func(timeout uint64, addr sdk.AccAddress) (bool, error) {
		if addr.Equals(vaultAddr) {
			found = true
			s.Require().Equal(expectedTimeout, timeout, "vault should be rescheduled with new timeout")
		}
		return false, nil
	})
	s.Require().NoError(err, "PayoutTimeoutQueue.Walk returned unexpected error during retry verification")
	s.Require().True(found, "vault should be in the interest timeout queue with new timeout")

	// Verify PeriodStart is preserved
	updatedVault, err := s.k.GetVault(s.ctx, vaultAddr)
	s.Require().NoError(err)
	s.Require().Equal(twoMonthsAgo.Unix(), updatedVault.PeriodStart, "PeriodStart should be preserved on transient failure")
}

func (s *TestSuite) TestReconcileVault_BootstrapFeePeriod() {
	shareDenom := "vaultshares"
	underlying := sdk.NewInt64Coin("underlying", 1_000_000_000)
	totalShares := sdk.NewInt64Coin(shareDenom, 1_000_000_000_000_000)
	testBlockTime := time.Now()
	periodStart := testBlockTime.Add(-1 * time.Hour).Unix()

	// Setup vault with PeriodStart set but FeePeriodStart = 0
	vaultAddr, vault := s.setupReconcileVault("0.25", periodStart, false, underlying, shareDenom, totalShares, testBlockTime)

	// Manually set FeePeriodStart to 0 to simulate the bug scenario
	vault.FeePeriodStart = 0
	vault.FeePeriodTimeout = 0
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	// Verify initial state
	v, err := s.k.GetVault(s.ctx, vaultAddr)
	s.Require().NoError(err)
	s.Require().Equal(int64(0), v.FeePeriodStart, "FeePeriodStart should be 0 initially")

	// Call reconcileVault
	err = s.k.TestAccessor_reconcileVault(s.T(), s.ctx, v)
	s.Require().NoError(err)

	// Verify FeePeriodStart is bootstrapped (should be equal to current block time)
	v, err = s.k.GetVault(s.ctx, vaultAddr)
	s.Require().NoError(err)
	s.Require().NotEqual(int64(0), v.FeePeriodStart, "FeePeriodStart should have been bootstrapped")
	s.Require().Equal(testBlockTime.Unix(), v.FeePeriodStart, "FeePeriodStart should be set to current block time")
}
