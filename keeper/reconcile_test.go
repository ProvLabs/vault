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
		vault.FeePeriodStart = periodStartSeconds
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
				s.assertInPayoutVerificationQueue(vaultAddress, true)
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
				// Fee is calculated on TVV AFTER interest transfer.
				// Initial TVV: 1,000,000,000. Interest: 41,952,013.
				// TVV after interest: 1,041,952,013.
				// Fee: 1,041,952,013 * 0.0015 * 5,184,000 / 31,536,000 = 256,919
				// Total marker change: 41,952,013 - 256,919 = 41,695,094
				s.assertVaultAndMarkerBalances(vaultAddress, shareDenom, underlying.Denom, sdkmath.NewInt(958047987), sdkmath.NewInt(1041695094))
			},
			expectedEvents: func() sdk.Events {
				markerAddr := markertypes.MustGetMarkerAddress(shareDenom)
				ev := createReconcileEvents(
					vaultAddress,
					markerAddr,
					sdkmath.NewInt(41952013),
					sdkmath.NewInt(1_000_000_000),
					sdkmath.NewInt(1_041_952_013),
					underlying.Denom,
					"0.25",
					5_184_000,
				)
				provlabsAddr, _ := types.GetProvLabsFeeAddress(s.ctx.ChainID())
				feeEvs := createSendCoinEvents(markerAddr.String(), provlabsAddr.String(), "256919underlying")
				feeEv := sdk.NewEvent("provlabs.vault.v1.EventVaultFeeCollected",
					sdk.NewAttribute("aum_snapshot", "1041952013underlying"),
					sdk.NewAttribute("collected_amount", "256919underlying"),
					sdk.NewAttribute("duration_seconds", "5184000"),
					sdk.NewAttribute("outstanding_amount", "0underlying"),
					sdk.NewAttribute("requested_amount", "256919underlying"),
					sdk.NewAttribute("vault_address", vaultAddress.String()),
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
			}(),
		},
		{
			name: "interest period has elasped, should pay negative interest and update period start",
			setup: func() {
				setup("-0.25", pastTime.Unix(), false)
			},
			posthander: func() {
				s.assertInPayoutVerificationQueue(vaultAddress, true)
				// Initial TVV: 1,000,000,000. Interest: -40,262,904.
				// TVV after interest: 959,737,096.
				// Fee: 959,737,096 * 0.0015 * 5,184,000 / 31,536,000 = 236,647
				// Total marker change: -40,262,904 - 236,647 = -40,499,551
				// Marker: 1,000,000,000 - 40,499,551 = 959,500,449
				// Vault: 1,000,000,000 + 40,262,904 = 1,040,262,904
				s.assertVaultAndMarkerBalances(vaultAddress, shareDenom, underlying.Denom, sdkmath.NewInt(1040262904), sdkmath.NewInt(959500449))
			},
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
				provlabsAddr, _ := types.GetProvLabsFeeAddress(s.ctx.ChainID())
				feeEvs := createSendCoinEvents(markerAddr.String(), provlabsAddr.String(), "236647underlying")
				feeEv := sdk.NewEvent("provlabs.vault.v1.EventVaultFeeCollected",
					sdk.NewAttribute("aum_snapshot", "959737096underlying"),
					sdk.NewAttribute("collected_amount", "236647underlying"),
					sdk.NewAttribute("duration_seconds", "5184000"),
					sdk.NewAttribute("outstanding_amount", "0underlying"),
					sdk.NewAttribute("requested_amount", "236647underlying"),
					sdk.NewAttribute("vault_address", vaultAddress.String()),
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
				provlabsAddr, _ := types.GetProvLabsFeeAddress(s.ctx.ChainID())
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
					// AUM Fee events
					sdk.NewEvent("coin_spent",
						sdk.NewAttribute("spender", markerAddr.String()),
						sdk.NewAttribute("amount", "256919underlying"),
					),
					sdk.NewEvent("coin_received",
						sdk.NewAttribute("receiver", provlabsAddr.String()),
						sdk.NewAttribute("amount", "256919underlying"),
					),
					sdk.NewEvent("transfer",
						sdk.NewAttribute("recipient", provlabsAddr.String()),
						sdk.NewAttribute("sender", markerAddr.String()),
						sdk.NewAttribute("amount", "256919underlying"),
					),
					sdk.NewEvent("message",
						sdk.NewAttribute("sender", markerAddr.String()),
					),
					sdk.NewEvent("provlabs.vault.v1.EventVaultFeeCollected",
						sdk.NewAttribute("aum_snapshot", "1041952013underlying"),
						sdk.NewAttribute("collected_amount", "256919underlying"),
						sdk.NewAttribute("duration_seconds", "5184000"),
						sdk.NewAttribute("outstanding_amount", "0underlying"),
						sdk.NewAttribute("requested_amount", "256919underlying"),
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

	s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, vaultAddr, sdk.NewCoins(sdk.NewCoin(underlyingDenom, sdkmath.NewInt(1_000_000)))), "failed to fund composite vault in TestKeeper_CanPayoutDuration_NegativeInterest_Composite_InsufficientUnderlying")

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

	err = s.k.TestAccessor_reconcileVaultInterest(s.T(), s.ctx, vault)
	s.Require().NoError(err, "failed to reconcile vault interest")

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
	vault.FeePeriodStart = periodStart
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

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

	err = s.k.TestAccessor_reconcileVaultInterest(s.T(), s.ctx, vault)
	s.Require().NoError(err, "expected reconcileVaultInterest to succeed")

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

	err = s.k.TestAccessor_reconcileVaultInterest(s.T(), s.ctx, vault)
	s.Require().NoError(err, "ReconcileVaultInterest should not error during partial liquidation")

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
	vault.FeePeriodStart = periodStart
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, vaultAddr, sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, 1_000_000))), "Funding vault should succeed")

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
	s.SetupTest()

	shareDenom := "fee.shares"
	underlyingDenom := "uylds.fcc.receipt"
	paymentDenom := "uylds.fcc"

	underlying := sdk.NewInt64Coin(underlyingDenom, 1_000_000_000)
	vaultAddr := types.GetVaultAddress(shareDenom)
	markerAddr := markertypes.MustGetMarkerAddress(shareDenom)
	now := time.Now().UTC()
	testBlockTime := now
	twoMonthsAgo := now.Add(-60 * 24 * time.Hour).Unix()

	s.requireAddFinalizeAndActivateMarker(underlying, s.adminAddr)

	_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
		Admin:           s.adminAddr.String(),
		ShareDenom:      shareDenom,
		UnderlyingAsset: underlyingDenom,
		PaymentDenom:    paymentDenom,
	})
	s.Require().NoError(err, "CreateVault should succeed")

	vault, err := s.k.GetVault(s.ctx, vaultAddr)
	s.Require().NoError(err, "should successfully get vault")

	vault.CurrentInterestRate = "0.0"
	vault.DesiredInterestRate = "0.0"
	vault.FeePeriodStart = twoMonthsAgo
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	// Fund principal with 1,000,000,000 underlying
	s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, markerAddr, sdk.NewCoins(underlying)), "funding marker underlying should not error")

	// 1. Partial collection: marker has 100,000 payment, but fee is ~250,000
	paymentAmount := sdkmath.NewInt(100_000)
	s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, markerAddr, sdk.NewCoins(sdk.NewCoin(paymentDenom, paymentAmount))), "funding marker payment denom should not error")

	s.ctx = s.ctx.WithBlockTime(testBlockTime).WithEventManager(sdk.NewEventManager())

	err = s.k.PerformVaultFeeTransfer(s.ctx, vault)
	s.Require().NoError(err, "PerformVaultFeeTransfer should not error during partial collection")

	// Marker balance should be 0
	endMarkerPayment := s.simApp.BankKeeper.GetBalance(s.ctx, markerAddr, paymentDenom).Amount
	s.Require().True(endMarkerPayment.IsZero(), "marker payment balance should be fully depleted; expected 0, got %s", endMarkerPayment)

	// ProvLabs should have 100,000
	provlabsAddr, _ := types.GetProvLabsFeeAddress(s.ctx.ChainID())
	feeCollected := s.simApp.BankKeeper.GetBalance(s.ctx, provlabsAddr, paymentDenom).Amount
	s.Require().Equal(paymentAmount, feeCollected, "provlabs should receive exactly the available liquidity; expected %s, got %s", paymentAmount, feeCollected)

	// TVV = 1,000,000,000 underlying + 100,000 payment = 1,000,100,000
	// Fee = 1,000,100,000 * 0.0015 * 5,184,000 / 31,536,000 = 246,600
	expectedFeeTotal := sdkmath.NewInt(246_600)
	expectedOutstanding := expectedFeeTotal.Sub(paymentAmount)
	s.Require().Equal(expectedOutstanding, vault.OutstandingAumFee.Amount, "outstanding fee balance mismatch; expected %s, got %s", expectedOutstanding, vault.OutstandingAumFee.Amount)

	// Check event
	events := normalizeEvents(s.ctx.EventManager().Events())
	found := false
	for _, ev := range events {
		if ev.Type == "provlabs.vault.v1.EventVaultFeeCollected" {
			found = true
			s.Require().Equal(vaultAddr.String(), getAttribute(ev, "vault_address"), "event vault_address mismatch")
			s.Require().Equal(sdk.NewCoin(paymentDenom, paymentAmount).String(), getAttribute(ev, "collected_amount"), "event collected_amount mismatch")
			s.Require().Equal(expectedFeeTotal.String()+paymentDenom, getAttribute(ev, "requested_amount"), "event requested_amount mismatch")
			s.Require().Equal("1000100000"+underlyingDenom, getAttribute(ev, "aum_snapshot"), "event aum_snapshot mismatch")
			s.Require().Equal(expectedOutstanding.String()+paymentDenom, getAttribute(ev, "outstanding_amount"), "event outstanding_amount mismatch")
		}
	}
	s.Require().True(found, "EventVaultFeeCollected should be emitted during partial collection")

	// 2. Second collection: fund marker more, collect outstanding
	// Advance time by 1 second so duration > 0
	s.ctx = s.ctx.WithBlockTime(testBlockTime.Add(time.Second)).WithEventManager(sdk.NewEventManager())
	morePayment := sdkmath.NewInt(1_000_000)
	s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, markerAddr, sdk.NewCoins(sdk.NewCoin(paymentDenom, morePayment))), "funding marker payment denom for second collection should not error")

	err = s.k.PerformVaultFeeTransfer(s.ctx, vault)
	s.Require().NoError(err, "PerformVaultFeeTransfer should not error during second collection")

	vault, _ = s.k.GetVault(s.ctx, vaultAddr)
	// After second collection, FeePeriodStart should be updated to current context time
	s.Require().Equal(s.ctx.BlockTime().Unix(), vault.FeePeriodStart, "FeePeriodStart should be updated after collection")
	s.Require().True(vault.OutstandingAumFee.IsZero(), "outstanding fee should be fully cleared; got %s", vault.OutstandingAumFee)
	endMarkerPayment = s.simApp.BankKeeper.GetBalance(s.ctx, markerAddr, paymentDenom).Amount
	s.Require().Equal(sdkmath.NewInt(853400), endMarkerPayment, "marker payment balance mismatch after clearing outstanding fees; expected 853,400, got %s", endMarkerPayment)
}

func (s *TestSuite) TestKeeper_CanPayoutDuration_WithAUMFee() {
	s.SetupTest()

	shareDenom := "fee.payout.shares"
	underlyingDenom := "underlying"
	paymentDenom := "uylds.fcc" // Using uylds.fcc for 1:1 conversion fast-path

	underlying := sdk.NewInt64Coin(underlyingDenom, 1_000_000_000)
	vaultAddr := types.GetVaultAddress(shareDenom)
	markerAddr := markertypes.MustGetMarkerAddress(shareDenom)

	s.requireAddFinalizeAndActivateMarker(underlying, s.adminAddr)

	_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
		Admin:           s.adminAddr.String(),
		ShareDenom:      shareDenom,
		UnderlyingAsset: underlyingDenom,
		PaymentDenom:    paymentDenom,
	})
	s.Require().NoError(err)

	vault, err := s.k.GetVault(s.ctx, vaultAddr)
	s.Require().NoError(err)

	vault.CurrentInterestRate = "0.0"
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	// Fund principal
	s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, markerAddr, sdk.NewCoins(underlying)))

	year := int64(365 * 24 * time.Hour / time.Second)

	// 1. No payment denom liquidity -> succeeds (because fees are deferred)
	ok, err := s.k.CanPayoutDuration(s.ctx, vault, year)
	s.Require().NoError(err)
	s.Require().True(ok, "should succeed even when no payment denom liquidity for fees (deferred)")
}

func (s *TestSuite) TestKeeper_HandleVaultFeeTimeouts() {
	s.SetupTest()

	shareDenom := "fee.timeout.shares"
	underlyingDenom := "underlying"
	paymentDenom := "uylds.fcc" // 1:1 conversion path

	underlying := sdk.NewInt64Coin(underlyingDenom, 1_000_000_000)
	vaultAddr := types.GetVaultAddress(shareDenom)
	markerAddr := markertypes.MustGetMarkerAddress(shareDenom)

	now := s.ctx.BlockTime()
	twoMonthsAgo := now.Add(-60 * 24 * time.Hour)

	s.requireAddFinalizeAndActivateMarker(underlying, s.adminAddr)

	_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
		Admin:           s.adminAddr.String(),
		ShareDenom:      shareDenom,
		UnderlyingAsset: underlyingDenom,
		PaymentDenom:    paymentDenom,
	})
	s.Require().NoError(err)

	vault, err := s.k.GetVault(s.ctx, vaultAddr)
	s.Require().NoError(err)

	vault.FeePeriodStart = twoMonthsAgo.Unix()
	vault.FeePeriodTimeout = twoMonthsAgo.Unix()
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	// Fund principal
	s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, markerAddr, sdk.NewCoins(underlying)))
	// Fund for fees
	s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, markerAddr, sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 10_000_000))))

	// Enqueue timeout in the past
	s.Require().NoError(s.k.FeeTimeoutQueue.Enqueue(s.ctx, twoMonthsAgo.Unix(), vaultAddr))

	// Run handler at now
	err = s.k.TestAccessor_handleVaultFeeTimeouts(s.T(), s.ctx)
	s.Require().NoError(err)

	// Check if fee was collected (AUM ~1,000,000,000 * 0.0015 * 60 days / 365 days = ~246,575)
	provlabsAddr, _ := types.GetProvLabsFeeAddress(s.ctx.ChainID())
	feeCollected := s.simApp.BankKeeper.GetBalance(s.ctx, provlabsAddr, paymentDenom).Amount
	s.Require().True(feeCollected.IsPositive(), "fee should be collected")

	// Check if new timeout is enqueued
	found := false
	s.k.FeeTimeoutQueue.Walk(s.ctx, func(timeout uint64, addr sdk.AccAddress) (bool, error) {
		if addr.Equals(vaultAddr) {
			found = true
			s.Require().Greater(int64(timeout), now.Unix())
		}
		return false, nil
	})
	s.Require().True(found, "new fee timeout should be enqueued")

	// Refresh vault to ensure state was persisted
	vault, err = s.k.GetVault(s.ctx, vaultAddr)
	s.Require().NoError(err)
	s.Require().Equal(s.ctx.BlockTime().Unix(), vault.FeePeriodStart, "FeePeriodStart should be updated to block time")
}

func getAttribute(ev sdk.Event, key string) string {
	for _, attr := range ev.Attributes {
		if string(attr.Key) == key {
			return string(attr.Value)
		}
	}
	return ""
}

func (s *TestSuite) TestKeeper_EstimationMethods() {
	shareDenom := "vaultshares"
	underlyingDenom := "underlying"
	underlying := sdk.NewInt64Coin(underlyingDenom, 1_000_000_000)
	vaultAddress := types.GetVaultAddress(shareDenom)
	testBlockTime := time.Now().UTC()
	pastTime := testBlockTime.Add(-60 * 24 * time.Hour) // ~2 months

	setup := func() *types.VaultAccount {
		s.requireAddFinalizeAndActivateMarker(underlying, s.adminAddr)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           s.adminAddr.String(),
			ShareDenom:      shareDenom,
			UnderlyingAsset: underlyingDenom,
		})
		s.Require().NoError(err)

		vault, err := s.k.GetVault(s.ctx, vaultAddress)
		s.Require().NoError(err)
		s.ctx = s.ctx.WithBlockTime(testBlockTime)
		return vault
	}

	s.Run("EstimateAccruedInterest", func() {
		s.SetupTest()
		vault := setup()

		// Case 1: No interest rate
		amt, err := s.k.EstimateAccruedInterest(s.ctx, *vault, underlying)
		s.Require().NoError(err)
		s.Require().True(amt.IsZero())

		// Case 2: Positive interest
		vault.CurrentInterestRate = "0.25"
		vault.PeriodStart = pastTime.Unix()
		amt, err = s.k.EstimateAccruedInterest(s.ctx, *vault, underlying)
		s.Require().NoError(err)
		s.Require().Equal(sdkmath.NewInt(41_952_013), amt)

		// Case 3: Negative interest
		vault.CurrentInterestRate = "-0.25"
		amt, err = s.k.EstimateAccruedInterest(s.ctx, *vault, underlying)
		s.Require().NoError(err)
		s.Require().Equal(sdkmath.NewInt(-40_262_904), amt)

		// Case 4: Future period start
		vault.PeriodStart = testBlockTime.Add(time.Hour).Unix()
		amt, err = s.k.EstimateAccruedInterest(s.ctx, *vault, underlying)
		s.Require().NoError(err)
		s.Require().True(amt.IsZero())
	})

	s.Run("EstimateAccruedAUMFee", func() {
		s.SetupTest()
		vault := setup()

		// Case 1: No fee period start
		vault.FeePeriodStart = 0
		amt, err := s.k.EstimateAccruedAUMFee(s.ctx, *vault, underlying.Amount)
		s.Require().NoError(err)
		s.Require().True(amt.IsZero())

		// Case 2: Positive fee
		vault.FeePeriodStart = pastTime.Unix()
		amt, err = s.k.EstimateAccruedAUMFee(s.ctx, *vault, underlying.Amount)
		s.Require().NoError(err)
		// Fee = 1,000,000,000 * 0.0015 * 5,184,000 / 31,536,000 = 246,575
		s.Require().Equal(sdkmath.NewInt(246_575), amt)

		// Case 3: Zero assets
		amt, err = s.k.EstimateAccruedAUMFee(s.ctx, *vault, sdkmath.ZeroInt())
		s.Require().NoError(err)
		s.Require().True(amt.IsZero())
	})

	s.Run("CalculateOutstandingFeeUnderlying", func() {
		s.SetupTest()
		vault := setup()

		// Case 1: No outstanding fee
		amt, err := s.k.CalculateOutstandingFeeUnderlying(s.ctx, *vault)
		s.Require().NoError(err)
		s.Require().True(amt.IsZero())

		// Case 2: Outstanding fee in underlying
		vault.OutstandingAumFee = sdk.NewInt64Coin(underlyingDenom, 500)
		amt, err = s.k.CalculateOutstandingFeeUnderlying(s.ctx, *vault)
		s.Require().NoError(err)
		s.Require().Equal(sdkmath.NewInt(500), amt)

		// Case 3: Outstanding fee in payment denom (1:1 fast path)
		vault.PaymentDenom = "uylds.fcc"
		vault.OutstandingAumFee = sdk.NewInt64Coin("uylds.fcc", 1000)
		amt, err = s.k.CalculateOutstandingFeeUnderlying(s.ctx, *vault)
		s.Require().NoError(err)
		s.Require().Equal(sdkmath.NewInt(1000), amt)
	})

	s.Run("EstimateAccruedAUMFeePayment", func() {
		s.SetupTest()
		vault := setup()

		// Case 1: No fee period start
		vault.FeePeriodStart = 0
		vault.PaymentDenom = underlyingDenom
		amt, err := s.k.EstimateAccruedAUMFeePayment(s.ctx, *vault, underlying.Amount)
		s.Require().NoError(err)
		s.Require().True(amt.IsZero())
		s.Require().Equal(underlyingDenom, amt.Denom)

		// Case 2: Positive fee, 1:1 payment denom
		vault.FeePeriodStart = pastTime.Unix()
		vault.PaymentDenom = underlyingDenom
		amt, err = s.k.EstimateAccruedAUMFeePayment(s.ctx, *vault, underlying.Amount)
		s.Require().NoError(err)
		s.Require().Equal(sdkmath.NewInt(246_575), amt.Amount)
		s.Require().Equal(underlyingDenom, amt.Denom)

		// Case 3: Positive fee, 1:2 payment denom
		paymentDenom := "usdc"
		s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(paymentDenom, 1_000_000), s.adminAddr)
		pmtMarkerAddr := markertypes.MustGetMarkerAddress(paymentDenom)
		pmtMarkerAcct, _ := s.k.MarkerKeeper.GetMarker(s.ctx, pmtMarkerAddr)
		s.k.MarkerKeeper.SetNetAssetValue(s.ctx, pmtMarkerAcct, markertypes.NetAssetValue{
			Price:  sdk.NewInt64Coin(underlyingDenom, 1),
			Volume: 2,
		}, "test")

		vault.PaymentDenom = paymentDenom
		amt, err = s.k.EstimateAccruedAUMFeePayment(s.ctx, *vault, underlying.Amount)
		s.Require().NoError(err)
		// FeeUnderlying = 246,575
		// FeePayment = 246,575 * 2 / 1 = 493,150
		s.Require().Equal(sdkmath.NewInt(493_150), amt.Amount)
		s.Require().Equal(paymentDenom, amt.Denom)
	})

	s.Run("CalculateVaultTotalAssets", func() {
		s.SetupTest()
		vault := setup()

		// Case 1: Simple principal, no interest, no fees
		vault.CurrentInterestRate = "0.0"
		vault.PeriodStart = 0
		vault.FeePeriodStart = 0
		vault.OutstandingAumFee = sdk.NewCoin(underlyingDenom, sdkmath.ZeroInt())

		amt, err := s.k.CalculateVaultTotalAssets(s.ctx, vault, underlying)
		s.Require().NoError(err)
		s.Require().Equal(underlying.Amount, amt)

		// Case 2: Principal + Interest - Fees - Outstanding
		vault.CurrentInterestRate = "0.25"
		vault.PeriodStart = pastTime.Unix()
		vault.FeePeriodStart = pastTime.Unix()
		vault.OutstandingAumFee = sdk.NewInt64Coin(underlyingDenom, 1000)

		amt, err = s.k.CalculateVaultTotalAssets(s.ctx, vault, underlying)
		s.Require().NoError(err)

		// ExpectedInterest = 41,952,013
		// IntermediateSum = 1,000,000,000 + 41,952,013 = 1,041,952,013
		// ExpectedFee = 1,041,952,013 * 0.0015 * 5,184,000 / 31,536,000 = 256,919 (truncated dec)
		// Result = 1,041,952,013 - 256,919 - 1000 = 1,041,694,094
		s.Require().Equal(sdkmath.NewInt(1_041_694_094), amt)
	})
}
