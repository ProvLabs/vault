package keeper_test

import (
	"fmt"
	"math"
	"math/big"
	"time"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	markertypes "github.com/provenance-io/provenance/x/marker/types"

	"github.com/provlabs/vault/interest"
	"github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/types"
)

func (s *TestSuite) TestKeeper_ReconcileVaultInterest() {
	twoMonths := -24 * 60 * time.Hour
	shareDenom := "vaultshares"
	underlying := sdk.NewInt64Coin("underlying", 1_000_000_000)
	totalShares := sdk.NewInt64Coin(shareDenom, 1_000_000_000_000_000)
	vaultAddress := types.GetVaultAddress(shareDenom)
	testBlockTime := time.Now()
	futureTime := testBlockTime.Add(100 * time.Second)
	pastTime := testBlockTime.Add(twoMonths)

	setup := func(interestRate string, periodStartSeconds int64, paused bool) {
		s.requireAddFinalizeAndActivateMarker(underlying, s.adminAddr)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           s.adminAddr.String(),
			ShareDenom:      shareDenom,
			UnderlyingAsset: underlying.Denom,
		})
		s.Require().NoError(err, "CreateVault should not error")

		vault, err := s.k.GetVault(s.ctx, vaultAddress)
		s.Require().NoError(err, "GetVault should not error in setup")
		vault.CurrentInterestRate = interestRate
		vault.DesiredInterestRate = interestRate
		vault.PeriodStart = periodStartSeconds
		vault.Paused = paused
		vault.TotalShares = totalShares
		s.k.AuthKeeper.SetAccount(s.ctx, vault)

		err = FundAccount(s.ctx, s.simApp.BankKeeper, vaultAddress, sdk.NewCoins(underlying))
		s.Require().NoError(err, "funding vault account should not error")
		err = FundAccount(s.ctx, s.simApp.BankKeeper, markertypes.MustGetMarkerAddress(shareDenom), sdk.NewCoins(underlying))
		s.Require().NoError(err, "funding share marker account should not error")

		s.ctx = s.ctx.WithBlockTime(testBlockTime)
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	}

	tests := []struct {
		name              string
		setup             func()
		posthander        func()
		expectedErrSubstr string
		expectedEvents    sdk.Events
	}{
		{
			name: "no start period found, should set period start and return no error",
			setup: func() {
				setup("0.25", 0, false)
			},
			posthander: func() {
				s.assertInPayoutVerificationQueue(vaultAddress, true)
				s.assertVaultAndMarkerBalances(vaultAddress, shareDenom, underlying.Denom, underlying.Amount, underlying.Amount)
			},
			expectedEvents: sdk.Events{},
		},
		{
			name: "interest period start in future, should return nil and do nothing",
			setup: func() {
				setup("0.25", futureTime.Unix(), false)
			},
			posthander: func() {
				s.assertInPayoutVerificationQueue(vaultAddress, false)
				s.assertVaultAndMarkerBalances(vaultAddress, shareDenom, underlying.Denom, underlying.Amount, underlying.Amount)
			},
			expectedEvents: sdk.Events{},
		},
		{
			name: "interest period has elasped, should pay interest and update period start",
			setup: func() {
				setup("0.25", pastTime.Unix(), false)
			},
			posthander: func() {
				s.assertInPayoutVerificationQueue(vaultAddress, true)
				// AUM Fee for 2 months (1B AUM): (1,000,000,000 * 0.0015 * 5,184,000) / 31,536,000 = 246,575.34 -> 246,575
				// Interest for 2 months (1B AUM, 25% APR): 41,952,013
				// Total deduction from reserves: 246,575 + 41,952,013 = 42,198,588
				// Remaining reserves: 1,000_000_000 - 42,198,588 = 957,801,412
				// Marker principal: 1,000_000_000 + 41,952,013 = 1,041,952,013
				s.assertVaultAndMarkerBalances(vaultAddress, shareDenom, underlying.Denom, sdkmath.NewInt(957_801_412), sdkmath.NewInt(1_041_952_013))
			},
			expectedEvents: func() sdk.Events {
				feeAddr, err := types.GetProvLabsFeeAddress(s.ctx.ChainID())
				s.Require().NoError(err)
				feeEv := createFeeEvents(
					vaultAddress,
					feeAddr,
					sdk.NewCoin(underlying.Denom, sdkmath.NewInt(246_575)),
					sdk.NewCoin(underlying.Denom, sdkmath.NewInt(1_000_000_000)),
					5_184_000,
				)
				ev := createReconcileEvents(
					vaultAddress,
					markertypes.MustGetMarkerAddress(shareDenom),
					sdkmath.NewInt(41_952_013),
					sdkmath.NewInt(1_000_000_000),
					sdkmath.NewInt(1_041_952_013),
					underlying.Denom,
					"0.25",
					5_184_000,
				)
				nav := createMarkerSetNAV(
					shareDenom,
					sdk.NewCoin(underlying.Denom, sdkmath.NewInt(1_041_952_013)),
					"vault",
					totalShares.Amount.Uint64(),
				)
				res := append(feeEv, ev...)
				return append(res, nav)
			}(),
		},
		{
			name: "interest period has elasped, should pay negative interest and update period start",
			setup: func() {
				setup("-0.25", pastTime.Unix(), false)
			},
			posthander: func() {
				s.assertInPayoutVerificationQueue(vaultAddress, true)
				// AUM Fee for 2 months (1B AUM): 246,575
				// Negative Interest for 2 months: -40,262,904
				// Total vault change: -246,575 (fee) + 40,262,904 (refund) = +40,016,329
				// Vault balance: 1,000,000,000 + 40,016,329 = 1,040,016,329
				// Marker principal: 1,000,000,000 - 40,262,904 = 959,737,096
				s.assertVaultAndMarkerBalances(vaultAddress, shareDenom, underlying.Denom, sdkmath.NewInt(1_040_016_329), sdkmath.NewInt(959_737_096))
			},
			expectedEvents: func() sdk.Events {
				feeAddr, err := types.GetProvLabsFeeAddress(s.ctx.ChainID())
				s.Require().NoError(err)
				feeEv := createFeeEvents(
					vaultAddress,
					feeAddr,
					sdk.NewCoin(underlying.Denom, sdkmath.NewInt(246_575)),
					sdk.NewCoin(underlying.Denom, sdkmath.NewInt(1_000_000_000)),
					5_184_000,
				)
				ev := createReconcileEvents(
					vaultAddress,
					markertypes.MustGetMarkerAddress(shareDenom),
					sdkmath.NewInt(-40_262_904),
					sdkmath.NewInt(1_000_000_000),
					sdkmath.NewInt(959_737_096),
					underlying.Denom,
					"-0.25",
					5_184_000,
				)
				nav := createMarkerSetNAV(
					shareDenom,
					sdk.NewCoin(underlying.Denom, sdkmath.NewInt(959_737_096)),
					"vault",
					totalShares.Amount.Uint64(),
				)
				res := append(feeEv, ev...)
				return append(res, nav)
			}(),
		},
		{
			name: "paused vault, should do nothing",
			setup: func() {
				setup("0.25", pastTime.Unix(), true)
			},
			posthander: func() {
				s.assertInPayoutVerificationQueue(vaultAddress, false)
				vault, err := s.k.GetVault(s.ctx, vaultAddress)
				s.Require().NoError(err, "GetVault should not error when paused")
				s.Require().Equal(pastTime.Unix(), vault.PeriodStart, "PeriodStart should remain unchanged when paused")
			},
			expectedEvents: sdk.Events{},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			if tc.setup != nil {
				tc.setup()
			}

			vault, err := s.k.GetVault(s.ctx, vaultAddress)
			s.Require().NoError(err, "GetVault should not error before reconcile")
			err = s.k.TestAccessor_reconcileVaultInterest(s.T(), s.ctx, vault)

			if tc.posthander != nil {
				tc.posthander()
			}
			if len(tc.expectedErrSubstr) > 0 {
				s.Require().Error(err, "expected error from ReconcileVaultInterest")
				s.Require().Contains(err.Error(), tc.expectedErrSubstr, "error substring mismatch")
			} else {
				s.Require().NoError(err, "ReconcileVaultInterest should not error")
			}

			s.Assert().Equal(
				normalizeEvents(tc.expectedEvents),
				normalizeEvents(s.ctx.EventManager().Events()),
				"events mismatch for %s", tc.name,
			)
		})
	}
}

func (s *TestSuite) TestPerformVaultFeeTransfer() {
	shareDenom := "vaultshares"
	underlyingDenom := "underlying"
	paymentDenom := "payment"
	vaultAddr := types.GetVaultAddress(shareDenom)
	recipientAddr, _ := types.GetProvLabsFeeAddress(s.ctx.ChainID())

	setup := func(payment string, aum, reserves int64, periodStart int64) *types.VaultAccount {
		s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlyingDenom, aum), s.adminAddr)
		if payment != underlyingDenom {
			s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(payment, reserves), s.adminAddr)
		}

		vault, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           s.adminAddr.String(),
			ShareDenom:      shareDenom,
			UnderlyingAsset: underlyingDenom,
			PaymentDenom:    payment,
		})
		s.Require().NoError(err, "failed to create vault for fee test")

		vault.PeriodStart = periodStart
		s.k.AuthKeeper.SetAccount(s.ctx, vault)

		s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, vault.PrincipalMarkerAddress(), sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, aum))), "failed to fund principal")
		s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, vaultAddr, sdk.NewCoins(sdk.NewInt64Coin(payment, reserves))), "failed to fund reserves")

		return vault
	}

	s.Run("successful fee collection - same denom", func() {
		s.SetupTest()
		now := time.Now().UTC()
		oneYearAgo := now.AddDate(-1, 0, 0).Unix()
		s.ctx = s.ctx.WithBlockTime(now).WithEventManager(sdk.NewEventManager())

		// 1B AUM, 15 bps = 1.5M fee
		vault := setup(underlyingDenom, 1_000_000_000, 2_000_000, oneYearAgo)

		err := s.k.PerformVaultFeeTransfer(s.ctx, vault)
		s.Require().NoError(err, "fee transfer should succeed when reserves are sufficient")

		feeBalance := s.simApp.BankKeeper.GetBalance(s.ctx, recipientAddr, underlyingDenom)
		s.Require().Equal(sdkmath.NewInt(1_500_000), feeBalance.Amount, "ProvLabs should receive exactly 15 bps of AUM per year")

		vaultBalance := s.simApp.BankKeeper.GetBalance(s.ctx, vaultAddr, underlyingDenom)
		s.Require().Equal(sdkmath.NewInt(500_000), vaultBalance.Amount, "vault reserves should be reduced by the collected fee amount")
	})

	s.Run("successful fee collection - different denom", func() {
		s.SetupTest()
		now := time.Now().UTC()
		oneYearAgo := now.AddDate(-1, 0, 0).Unix()
		s.ctx = s.ctx.WithBlockTime(now).WithEventManager(sdk.NewEventManager())

		vault := setup(paymentDenom, 1_000_000_000, 1_000_000, oneYearAgo)

		// Setup NAV: 1 payment = 2 underlying
		// 1B AUM (underlying), 15 bps = 1.5M fee (underlying)
		// 1.5M underlying = 0.75M payment
		pmtMarkerAddr := markertypes.MustGetMarkerAddress(paymentDenom)
		pmtMarker, err := s.k.MarkerKeeper.GetMarker(s.ctx, pmtMarkerAddr)
		s.Require().NoError(err, "failed to get payment marker")
		s.Require().NoError(s.k.MarkerKeeper.SetNetAssetValue(s.ctx, pmtMarker, markertypes.NetAssetValue{
			Price:  sdk.NewInt64Coin(underlyingDenom, 2),
			Volume: 1,
		}, "test"), "failed to set NAV for payment denom")

		err = s.k.PerformVaultFeeTransfer(s.ctx, vault)
		s.Require().NoError(err, "fee transfer should succeed with payment denom conversion")

		feeBalance := s.simApp.BankKeeper.GetBalance(s.ctx, recipientAddr, paymentDenom)
		s.Require().Equal(sdkmath.NewInt(750_000), feeBalance.Amount, "fee recipient should receive converted fee amount in payment denom")
	})

	s.Run("insufficient reserves", func() {
		s.SetupTest()
		now := time.Now().UTC()
		oneYearAgo := now.AddDate(-1, 0, 0).Unix()
		s.ctx = s.ctx.WithBlockTime(now).WithEventManager(sdk.NewEventManager())

		// 1B AUM, 15 bps = 1.5M fee, but only 1M in reserves
		vault := setup(underlyingDenom, 1_000_000_000, 1_000_000, oneYearAgo)

		err := s.k.PerformVaultFeeTransfer(s.ctx, vault)
		s.Require().Error(err, "fee transfer should fail when reserves are less than the required fee")
		s.Require().Contains(err.Error(), "insufficient reserves", "error message should clearly indicate reserve deficiency")
	})

	s.Run("insufficient reserves - different denom", func() {
		s.SetupTest()
		now := time.Now().UTC()
		oneYearAgo := now.AddDate(-1, 0, 0).Unix()
		s.ctx = s.ctx.WithBlockTime(now).WithEventManager(sdk.NewEventManager())

		// Setup vault with paymentDenom != underlyingDenom
		// 1B AUM, enough underlying for fee but 0 paymentDenom
		vault := setup(paymentDenom, 1_000_000_000, 0, oneYearAgo)
		s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, vaultAddr, sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, 2_000_000))), "fund underlying")

		// Setup NAV: 1 payment = 1 underlying
		pmtMarkerAddr := markertypes.MustGetMarkerAddress(paymentDenom)
		pmtMarker, _ := s.k.MarkerKeeper.GetMarker(s.ctx, pmtMarkerAddr)
		s.Require().NoError(s.k.MarkerKeeper.SetNetAssetValue(s.ctx, pmtMarker, markertypes.NetAssetValue{
			Price:  sdk.NewInt64Coin(underlyingDenom, 1),
			Volume: 1,
		}, "test"))

		err := s.k.PerformVaultFeeTransfer(s.ctx, vault)
		s.Require().Error(err, "fee transfer should fail when payment denom reserves are insufficient")
		s.Require().Contains(err.Error(), "insufficient reserves", "error should mention reserves")
		s.Require().Contains(err.Error(), paymentDenom, "error should mention the missing denom")
	})

	s.Run("zero fee - short duration", func() {
		s.SetupTest()
		now := time.Now().UTC()
		periodStart := now.Unix() // Zero duration
		s.ctx = s.ctx.WithBlockTime(now).WithEventManager(sdk.NewEventManager())

		vault := setup(underlyingDenom, 1_000_000_000, 1_000_000, periodStart)

		err := s.k.PerformVaultFeeTransfer(s.ctx, vault)
		s.Require().NoError(err, "fee transfer should be a no-op for zero duration")

		feeBalance := s.simApp.BankKeeper.GetBalance(s.ctx, recipientAddr, underlyingDenom)
		s.Require().True(feeBalance.IsZero(), "no fee should be collected for zero duration")
	})
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
				feeAddr, err := types.GetProvLabsFeeAddress(s.ctx.ChainID())
				s.Require().NoError(err)
				feeEv := createFeeEvents(
					vaultAddr,
					feeAddr,
					sdk.NewCoin(underlying.Denom, sdkmath.NewInt(246_575)),
					sdk.NewCoin(underlying.Denom, sdkmath.NewInt(1_000_000_000)),
					5_184_000,
				)
				ev := createReconcileEvents(
					vaultAddr,
					markertypes.MustGetMarkerAddress(shareDenom),
					sdkmath.NewInt(41_952_013),
					sdkmath.NewInt(1_000_000_000),
					sdkmath.NewInt(1_041_952_013),
					underlying.Denom,
					"0.25",
					5_184_000,
				)
				return append(feeEv, ev...)
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
				s.Require().NoError(s.k.SafeAddVerification(s.ctx, vault), "vault cannot pay: SafeAddVerification should not error")
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

	s.Require().NoError(s.k.SafeAddVerification(s.ctx, vault), "failed to safe add verification in createVaultWithInterest for vault %s", info.vaultAddr)
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

func (s *TestSuite) TestKeeper_CanPayoutDuration() {
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
				))
			}

			if tc.fundPrincipal.IsPositive() {
				s.Require().NoError(FundAccount(
					s.ctx,
					s.simApp.BankKeeper,
					markerAddr,
					sdk.NewCoins(sdk.NewCoin(underlying.Denom, tc.fundPrincipal)),
				))
			}

			ok, err := s.k.CanPayoutDuration(s.ctx, vault, tc.duration)
			s.Require().NoError(err, "error checking CanPayoutDuration")
			s.Require().Equal(tc.expectOK, ok, "unexpected CanPayoutDuration result")
		})
	}
}

func (s *TestSuite) TestKeeper_CanPayoutDuration_NegativeInterest_Composite_InsufficientUnderlying() {
	s.SetupTest()

	shareDenom := "vaultshares.composite"
	underlyingDenom := "uylds.fcc.receipt.token"
	paymentDenom := "uylds.fcc"

	underlying := sdk.NewInt64Coin(underlyingDenom, 1_000_000_000)
	vaultAddr := types.GetVaultAddress(shareDenom)
	markerAddr := markertypes.MustGetMarkerAddress(shareDenom)

	s.requireAddFinalizeAndActivateMarker(underlying, s.adminAddr)

	_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{Admin: s.adminAddr.String(), ShareDenom: shareDenom, UnderlyingAsset: underlyingDenom, PaymentDenom: paymentDenom})
	s.Require().NoError(err, "failed to create composite vault in TestKeeper_CanPayoutDuration_NegativeInterest_Composite_InsufficientUnderlying")

	vault, err := s.k.GetVault(s.ctx, vaultAddr)
	s.Require().NoError(err, "failed to get composite vault in TestKeeper_CanPayoutDuration_NegativeInterest_Composite_InsufficientUnderlying")

	vault.CurrentInterestRate = "-0.5"
	vault.DesiredInterestRate = "-0.5"
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, vaultAddr, sdk.NewCoins(
		sdk.NewCoin(underlyingDenom, sdkmath.NewInt(1_000_000)),
		sdk.NewCoin(paymentDenom, sdkmath.NewInt(1_000_000)),
	)), "failed to fund composite vault in TestKeeper_CanPayoutDuration_NegativeInterest_Composite_InsufficientUnderlying")

	tinyUnderlying := sdkmath.NewInt(10_000_000)
	hugePayment := sdkmath.NewInt(10_000_000_000_000)

	s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, markerAddr, sdk.NewCoins(sdk.NewCoin(underlyingDenom, tinyUnderlying), sdk.NewCoin(paymentDenom, hugePayment))), "failed to fund marker account")

	year := int64(365 * 24 * time.Hour / time.Second)

	canPayLong, err := s.k.CanPayoutDuration(s.ctx, vault, year)
	s.Require().NoError(err, "error checking CanPayoutDuration for long duration")
	s.Require().False(canPayLong, "expected CanPayoutDuration to be false for long duration with insufficient underlying")

	smallDuration := int64(1)

	canPayShort, err := s.k.CanPayoutDuration(s.ctx, vault, smallDuration)
	s.Require().NoError(err, "error checking CanPayoutDuration for short duration")
	s.Require().True(canPayShort, "expected CanPayoutDuration to be true for short duration with sufficient underlying")
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

	feeAmount, err := interest.CalculateAUMFee(principalTvv, periodDuration)
	s.Require().NoError(err, "failed to calculate AUM fee")

	err = s.k.TestAccessor_reconcileVaultInterest(s.T(), s.ctx, vault)
	s.Require().NoError(err, "failed to reconcile vault interest")

	endVault := s.simApp.BankKeeper.GetBalance(s.ctx, vaultAddr, underlying.Denom).Amount
	endMarker := s.simApp.BankKeeper.GetBalance(s.ctx, markerAddr, underlying.Denom).Amount

	expectedVault := startVault.Sub(interestEarned).Sub(feeAmount)
	expectedMarker := startMarker.Add(interestEarned)

	s.Require().Equal(expectedVault, endVault, "vault reserves mismatch")
	s.Require().Equal(expectedMarker, endMarker, "marker principal mismatch")

	s.assertVaultAndMarkerBalances(
		vaultAddr,
		shareDenom,
		underlying.Denom,
		expectedVault,
		expectedMarker,
	)

	events := normalizeEvents(s.ctx.EventManager().Events())

	foundReconcile := false
	foundFee := false
	for _, event := range events {
		if event.Type == "provlabs.vault.v1.EventVaultReconcile" {
			foundReconcile = true

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
				fmt.Sprintf("%s%s", endMarker.String(), underlying.Denom),
				principalAfterStr,
				"principal after mismatch",
			)

			s.Require().Equal(
				fmt.Sprintf("%s%s", interestEarned.String(), underlying.Denom),
				interestStr,
				"interest earned mismatch",
			)
		}
		if event.Type == "provlabs.vault.v1.EventVaultFeeCollected" {
			foundFee = true
		}
	}
	s.Require().True(foundReconcile, "expected EventVaultReconcile to be emitted")
	s.Require().True(foundFee, "expected EventVaultFeeCollected to be emitted")
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

	_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
		Admin:           s.adminAddr.String(),
		ShareDenom:      shareDenom,
		UnderlyingAsset: underlying.Denom,
		PaymentDenom:    paymentDenom,
	})
	s.Require().NoError(err, "expected CreateVault to succeed")

	vault, err := s.k.GetVault(s.ctx, vaultAddr)
	s.Require().NoError(err, "expected GetVault to succeed after CreateVault")

	vault.CurrentInterestRate = "0.25"
	vault.DesiredInterestRate = "0.25"
	vault.PeriodStart = periodStart
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	s.Require().NoError(
		FundAccount(s.ctx, s.simApp.BankKeeper, vaultAddr, sdk.NewCoins(underlying, sdk.NewInt64Coin(paymentDenom, 1_000_000))),
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

	startVaultUnderlying := s.simApp.BankKeeper.GetBalance(s.ctx, vaultAddr, underlying.Denom).Amount
	startVaultPayment := s.simApp.BankKeeper.GetBalance(s.ctx, vaultAddr, paymentDenom).Amount
	startMarkerUnderlying := s.simApp.BankKeeper.GetBalance(s.ctx, markerAddr, underlying.Denom).Amount
	startMarkerPayment := s.simApp.BankKeeper.GetBalance(s.ctx, markerAddr, paymentDenom).Amount

	s.Require().Equal(underlying.Amount, startVaultUnderlying, "expected initial vault underlying reserves to equal funded amount")
	s.Require().Equal(sdkmath.NewInt(1_000_000), startVaultPayment, "expected initial vault payment reserves to equal funded amount")
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

	feeAmountInUnderlying, err := interest.CalculateAUMFee(principalTvv, periodDuration)
	s.Require().NoError(err, "expected CalculateAUMFee to succeed")

	feeAmount, err := s.k.FromUnderlyingAssetAmount(s.ctx, *vault, feeAmountInUnderlying, paymentDenom)
	s.Require().NoError(err, "expected FromUnderlyingAssetAmount to succeed for fee")

	err = s.k.TestAccessor_reconcileVaultInterest(s.T(), s.ctx, vault)
	s.Require().NoError(err, "expected reconcileVaultInterest to succeed")

	endVaultUnderlying := s.simApp.BankKeeper.GetBalance(s.ctx, vaultAddr, underlying.Denom).Amount
	endVaultPayment := s.simApp.BankKeeper.GetBalance(s.ctx, vaultAddr, paymentDenom).Amount
	endMarkerUnderlying := s.simApp.BankKeeper.GetBalance(s.ctx, markerAddr, underlying.Denom).Amount
	endMarkerPayment := s.simApp.BankKeeper.GetBalance(s.ctx, markerAddr, paymentDenom).Amount

	expectedVaultUnderlying := startVaultUnderlying.Sub(interestEarned)
	expectedVaultPayment := startVaultPayment.Sub(feeAmount)
	expectedMarkerUnderlying := startMarkerUnderlying.Add(interestEarned)

	s.Require().Equal(expectedVaultUnderlying, endVaultUnderlying, "expected vault underlying reserves to decrease by TVV-based interest")
	s.Require().Equal(expectedVaultPayment, endVaultPayment, "expected vault payment reserves to decrease by AUM fee")
	s.Require().Equal(expectedMarkerUnderlying, endMarkerUnderlying, "expected marker underlying balance to increase by TVV-based interest")
	s.Require().Equal(startMarkerPayment, endMarkerPayment, "expected marker payment token balance to remain unchanged")

	s.assertVaultAndMarkerBalances(
		vaultAddr,
		shareDenom,
		underlying.Denom,
		expectedVaultUnderlying,
		expectedMarkerUnderlying,
	)

	events := normalizeEvents(s.ctx.EventManager().Events())

	foundReconcile := false
	foundFee := false
	for _, ev := range events {
		if ev.Type == "provlabs.vault.v1.EventVaultReconcile" {
			foundReconcile = true

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
			principalTvvAfter, err := s.k.GetTVVInUnderlyingAsset(s.ctx, *vault)
			s.Require().NoError(err, "expected GetTVVInUnderlyingAsset after reconcile to succeed")
			expectedPrincipalAfter := sdk.NewCoin(underlying.Denom, principalTvvAfter)

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
		}
		if ev.Type == "provlabs.vault.v1.EventVaultFeeCollected" {
			foundFee = true
		}
	}
	s.Require().True(foundReconcile, "expected EventVaultReconcile to be emitted for composite principal TVV transfer")
	s.Require().True(foundFee, "expected EventVaultFeeCollected to be emitted for composite principal TVV transfer")
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
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, vaultAddr, sdk.NewCoins(underlying)), "Funding vault should succeed")

	smallPrincipal := sdk.NewInt64Coin(underlying.Denom, 100_000)
	s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, markerAddr, sdk.NewCoins(smallPrincipal)), "Funding marker with small principal should succeed")

	s.ctx = s.ctx.WithBlockTime(now).WithEventManager(sdk.NewEventManager())

	// AUM fee for 1 year (100k AUM): (100,000 * 0.0015 * 31,536,000) / 31,536,000 = 150
	feeAmount, _ := interest.CalculateAUMFee(smallPrincipal.Amount, 31_536_000)

	err = s.k.TestAccessor_reconcileVaultInterest(s.T(), s.ctx, vault)
	s.Require().NoError(err, "ReconcileVaultInterest should not error during partial liquidation")

	endMarker := s.simApp.BankKeeper.GetBalance(s.ctx, markerAddr, underlying.Denom)
	s.Require().True(endMarker.IsZero(), "Marker balance should be fully liquidated to zero")

	endVault := s.simApp.BankKeeper.GetBalance(s.ctx, vaultAddr, underlying.Denom)
	// Vault change: -fee + refund = -150 + 100,000 = +99,850
	expectedVaultBalance := underlying.Amount.Add(smallPrincipal.Amount).Sub(feeAmount)
	s.Require().Equal(expectedVaultBalance, endVault.Amount, "Vault should receive exactly the available marker balance minus AUM fee")

	events := normalizeEvents(s.ctx.EventManager().Events())
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

	_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
		Admin:           s.adminAddr.String(),
		ShareDenom:      shareDenom,
		UnderlyingAsset: underlyingDenom,
		PaymentDenom:    paymentDenom,
	})
	s.Require().NoError(err, "CreateVault with composite structure should succeed")

	vault, err := s.k.GetVault(s.ctx, vaultAddr)
	s.Require().NoError(err, "GetVault should return the created vault")

	vault.CurrentInterestRate = "-0.5"
	vault.DesiredInterestRate = "-0.5"
	vault.PeriodStart = periodStart
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	// Fund with enough for both fee and interest if needed
	s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, vaultAddr, sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 1_000_000),
		sdk.NewInt64Coin(paymentDenom, 10_000_000),
	)), "Funding vault should succeed")

	hugeOtherBalance := sdk.NewInt64Coin(paymentDenom, 1_000_000_000)
	tinyUnderlyingBalance := sdk.NewInt64Coin(underlyingDenom, 10)

	s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, markerAddr, sdk.NewCoins(hugeOtherBalance, tinyUnderlyingBalance)), "Funding marker with composite assets should succeed")

	s.ctx = s.ctx.WithBlockTime(now).WithEventManager(sdk.NewEventManager())

	tvv, err := s.k.GetTVVInUnderlyingAsset(s.ctx, *vault)
	s.Require().NoError(err, "GetTVVInUnderlyingAsset should succeed")
	s.Require().True(tvv.GT(sdkmath.NewInt(100_000)), "TVV should be significantly higher than the underlying balance due to secondary assets")

	err = s.k.TestAccessor_reconcileVaultInterest(s.T(), s.ctx, vault)
	s.Require().NoError(err, "ReconcileVaultInterest should not error even if underlying liquidity is insufficient for full negative interest")

	endUnderlying := s.simApp.BankKeeper.GetBalance(s.ctx, markerAddr, underlyingDenom)
	endOther := s.simApp.BankKeeper.GetBalance(s.ctx, markerAddr, paymentDenom)

	s.Require().True(endUnderlying.IsZero(), "Underlying asset should be fully depleted")
	s.Require().Equal(hugeOtherBalance.Amount, endOther.Amount, "Secondary asset balance should remain unchanged")
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
