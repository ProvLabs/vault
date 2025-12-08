package keeper_test

import (
	"fmt"
	"math"
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
				s.assertVaultAndMarkerBalances(vaultAddress, shareDenom, underlying.Denom, sdkmath.NewInt(958047987), sdkmath.NewInt(1041952013))
			},
			expectedEvents: func() sdk.Events {
				ev := createReconcileEvents(
					vaultAddress,
					markertypes.MustGetMarkerAddress(shareDenom),
					sdkmath.NewInt(41952013),
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
				return append(ev, nav)
			}(),
		},
		{
			name: "interest period has elasped, should pay negative interest and update period start",
			setup: func() {
				setup("-0.25", pastTime.Unix(), false)
			},
			posthander: func() {
				s.assertInPayoutVerificationQueue(vaultAddress, true)
				s.assertVaultAndMarkerBalances(vaultAddress, shareDenom, underlying.Denom, sdkmath.NewInt(1_040_262_904), sdkmath.NewInt(959_737_096))
			},
			expectedEvents: func() sdk.Events {
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
				return append(ev, nav)
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
		s.Require().NoError(err)

		vault, err := s.k.GetVault(s.ctx, vaultAddress)
		s.Require().NoError(err)
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
				s.Require().Error(err)
			} else {
				s.Require().NoError(err)
				s.Require().Equal(tc.expectedIncrease, est)
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
				s.Require().NoError(err)
				vault, err := s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err)
				vault.CurrentInterestRate = "0.25"
				vault.DesiredInterestRate = "0.25"
				vault.PeriodStart = twoMonthsAgo
				s.k.AuthKeeper.SetAccount(s.ctx, vault)
				s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, vaultAddr, sdk.NewCoins(underlying)))
				s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, markerAddr, sdk.NewCoins(underlying)))
				s.Require().NoError(s.k.PayoutTimeoutQueue.Enqueue(s.ctx, testBlockTime.Unix(), vault.GetAddress()))
				s.ctx = s.ctx.WithBlockTime(testBlockTime).WithEventManager(sdk.NewEventManager())
			},
			checkAddr:     vaultAddr,
			expectExists:  true,
			expectDeleted: false,
			expectRate:    "0.25",
			expectedEvents: sdk.Events{
				sdk.NewEvent("coin_spent",
					sdk.NewAttribute("spender", vaultAddr.String()),
					sdk.NewAttribute("amount", "41952013underlying"),
				),
				sdk.NewEvent("coin_received",
					sdk.NewAttribute("receiver", markertypes.MustGetMarkerAddress(shareDenom).String()),
					sdk.NewAttribute("amount", "41952013underlying"),
				),
				sdk.NewEvent("transfer",
					sdk.NewAttribute("recipient", markertypes.MustGetMarkerAddress(shareDenom).String()),
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
			},
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
				s.Require().NoError(err)
				vault, err := s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err)
				vault.CurrentInterestRate = "0.25"
				vault.DesiredInterestRate = "0.25"
				s.k.AuthKeeper.SetAccount(s.ctx, vault)
				s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, markerAddr, sdk.NewCoins(underlying)))
				s.Require().NoError(s.k.SafeAddVerification(s.ctx, vault))
				s.Require().NoError(s.k.PayoutTimeoutQueue.Enqueue(s.ctx, testBlockTime.Unix(), vault.GetAddress()))
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
				s.Require().NoError(err)
				vault, err := s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err)
				vault.Paused = true
				s.k.AuthKeeper.SetAccount(s.ctx, vault)
				s.Require().NoError(s.k.PayoutTimeoutQueue.Enqueue(s.ctx, testBlockTime.Unix(), vault.GetAddress()))
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
				s.Require().NoError(s.k.PayoutVerificationSet.Set(s.ctx, markerAddr))
				s.Require().NoError(s.k.PayoutTimeoutQueue.Enqueue(s.ctx, testBlockTime.Unix(), markerAddr))
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
			s.Require().NoError(err)
			if tc.expectRate != "" {
				vault, err := s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err)
				s.Require().Equal(tc.expectRate, vault.CurrentInterestRate)
			}
			em := s.ctx.EventManager()
			s.Assert().Equalf(
				normalizeEvents(tc.expectedEvents),
				normalizeEvents(em.Events()),
				"beginblocker events",
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
		s.Require().NoError(err)
		s.Require().NotNil(vault)
		s.Assert().Equal(expectedRate, vault.CurrentInterestRate, "interest rate should not change for payable vault")
	}

	AssertIsDepleted := func(info VaultInfo) {
		s.T().Helper()
		vault, err := s.k.GetVault(s.ctx, info.vaultAddr)
		s.Require().NoError(err)
		s.Require().NotNil(vault)
		s.Assert().Equal(types.ZeroInterestRate, vault.CurrentInterestRate, "interest rate should be zeroed out for depleted vault")
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
				s.Require().NoError(err)
				s.Require().Empty(vaults)
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
				s.Require().Error(err)
			} else {
				s.Require().NoError(err)
			}

			actualEvents := normalizeEvents(s.ctx.EventManager().Events())
			expectedEvents := normalizeEvents(tc.expectedEvents)
			s.Assert().Equal(actualEvents, expectedEvents, "emitted events should contain all expected events")

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
		s.Require().NoError(err)
		s.Assert().True(found, "missing timeout entry at expected time")
		s.Assert().Equal(1, count, "should be exactly one timeout entry for vault")
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
				s.Require().NoError(err)
				expectedExpireTime := testBlockTime.Unix() + keeper.AutoReconcileTimeout
				s.Assert().Equal(expectedExpireTime, vault.PeriodTimeout)
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
				s.Require().NoError(err)
				s.Assert().Equal(expectedExpireTime, v1r.PeriodTimeout)
				assertSingleTimeoutAt(addr1, expectedExpireTime)

				addr2 := vaults[1].GetAddress()
				v2r, err := s.k.GetVault(s.ctx, addr2)
				s.Require().NoError(err)
				s.Assert().Equal(expectedExpireTime, v2r.PeriodTimeout)
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
				s.Require().NoError(err)
				s.Assert().Equal(types.ZeroInterestRate, updatedVault.CurrentInterestRate, "interest rate should be zeroed")
				s.Assert().Equal(initialRate, updatedVault.DesiredInterestRate, "desired rate should remain unchanged")
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
					s.Require().NoError(err)
					s.Assert().Equal(types.ZeroInterestRate, updatedVault.CurrentInterestRate)
					s.Assert().Equal(v.DesiredInterestRate, updatedVault.DesiredInterestRate)
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
	desiredRate := "0.3"

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
				s.Require().NoError(err)
				s.Require().NotNil(updatedVault)
				s.Assert().Equal(newRate, updatedVault.CurrentInterestRate)
				s.Assert().Equal(desiredRate, updatedVault.DesiredInterestRate)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			vault := tc.setup()

			s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
			s.k.UpdateInterestRates(s.ctx, vault, tc.currentRate, tc.desiredRate)

			s.Assert().Equal(
				normalizeEvents(tc.expectedEvents),
				normalizeEvents(s.ctx.EventManager().Events()),
			)

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
	s.Require().NoError(err)

	vault.CurrentInterestRate = interestRate
	vault.DesiredInterestRate = interestRate
	vault.PeriodStart = periodStart
	vault.PeriodTimeout = periodTimeout
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	if fundReserves {
		// Fund with enough to cover a day's interest
		err = FundAccount(s.ctx, s.simApp.BankKeeper, info.vaultAddr, sdk.NewCoins(sdk.NewInt64Coin(info.underlying.Denom, 1_000_000)))
		s.Require().NoError(err)
	}
	if fundPrincipal {
		err = FundAccount(s.ctx, s.simApp.BankKeeper, markertypes.MustGetMarkerAddress(info.shareDenom), sdk.NewCoins(info.underlying))
		s.Require().NoError(err)
	}

	s.Require().NoError(s.k.SafeAddVerification(s.ctx, vault))
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
		s.Require().NoError(err)
		vault, err := s.k.GetVault(s.ctx, vaultAddr)
		s.Require().NoError(err)
		vault.CurrentInterestRate = rate
		s.k.AuthKeeper.SetAccount(s.ctx, vault)
		return vault
	}

	tests := []struct {
		name            string
		rate            string
		duration        int64
		fundReserves    sdkmath.Int
		fundPrincipal   sdkmath.Int
		expectOK        bool
		expectErrSubstr string
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
			duration:      1_000,
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
			duration:      int64(24 * time.Hour / time.Second),
			fundReserves:  sdkmath.NewInt(10),
			fundPrincipal: sdkmath.NewInt(100_000),
			expectOK:      false,
		},
		{
			name:          "negative interest, principal available",
			rate:          "-1.0",
			duration:      int64(24 * time.Hour / time.Second * 365),
			fundReserves:  sdkmath.NewInt(0),
			fundPrincipal: sdkmath.NewInt(100_000),
			expectOK:      true,
		},
		{
			name:          "negative interest, interest more than principal, account will be liquidated",
			rate:          "-100.0",
			duration:      int64(24 * time.Hour / time.Second * 365),
			fundReserves:  sdkmath.NewInt(0),
			fundPrincipal: sdkmath.NewInt(1),
			expectOK:      true,
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			vault := createVaultWithRate(tc.rate)
			if !tc.fundReserves.IsZero() {
				s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, vaultAddr, sdk.NewCoins(sdk.NewCoin(underlying.Denom, tc.fundReserves))))
			}
			if !tc.fundPrincipal.IsZero() {
				s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, markerAddr, sdk.NewCoins(sdk.NewCoin(underlying.Denom, tc.fundPrincipal))))
			}

			ok, err := s.k.CanPayoutDuration(s.ctx, vault, tc.duration)
			if tc.expectErrSubstr != "" {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.expectErrSubstr)
			} else {
				s.Require().NoError(err)
				s.Require().Equal(tc.expectOK, ok)
			}
		})
	}
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
	s.Require().NoError(err)

	vault.CurrentInterestRate = "0.25"
	vault.DesiredInterestRate = "0.25"
	vault.PeriodStart = periodStart
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	s.Require().NoError(
		FundAccount(s.ctx, s.simApp.BankKeeper, vaultAddr, sdk.NewCoins(underlying)),
		"failed to fund vault account",
	)
	s.Require().NoError(
		FundAccount(s.ctx, s.simApp.BankKeeper, markerAddr, sdk.NewCoins(underlying)),
		"failed to fund marker account",
	)

	s.ctx = s.ctx.WithBlockTime(now).WithEventManager(sdk.NewEventManager())

	startVault := s.simApp.BankKeeper.GetBalance(s.ctx, vaultAddr, underlying.Denom).Amount
	startMarker := s.simApp.BankKeeper.GetBalance(s.ctx, markerAddr, underlying.Denom).Amount
	s.Require().Equal(underlying.Amount, startVault)
	s.Require().Equal(underlying.Amount, startMarker)

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
				fmt.Sprintf("%s%s", endMarker.String(), underlying.Denom),
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

	s.Require().Equal(expectedVault, endVault, "expected vault reserves to decrease by TVV-based interest")
	s.Require().Equal(expectedMarkerUnderlying, endMarkerUnderlying, "expected marker underlying balance to increase by TVV-based interest")
	s.Require().Equal(startMarkerPayment, endMarkerPayment, "expected marker payment token balance to remain unchanged")

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

			s.Require().Equal(
				fmt.Sprintf("%s%s", startMarkerUnderlying.String(), underlying.Denom),
				principalBeforeStr,
				"expected principal_before to reflect starting underlying principal",
			)

			s.Require().Equal(
				fmt.Sprintf("%s%s", endMarkerUnderlying.String(), underlying.Denom),
				principalAfterStr,
				"expected principal_after to reflect ending underlying principal",
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
