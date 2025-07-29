package keeper_test

import (
	"fmt"
	"time"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	markertypes "github.com/provenance-io/provenance/x/marker/types"

	"github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/types"
)

func (s *TestSuite) TestKeeper_ReconcileVaultInterest() {
	twoMonths := -24 * 60 * time.Hour
	shareDenom := "vaultshares"
	underlying := sdk.NewInt64Coin("underlying", 1_000_000_000)
	vaultAddress := types.GetVaultAddress(shareDenom)
	testBlockTime := time.Now()
	futureTime := testBlockTime.Add(100 * time.Second)
	pastTime := testBlockTime.Add(twoMonths)

	setup := func(interestRate string, periodStartSeconds int64) {
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
		s.k.AuthKeeper.SetAccount(s.ctx, vault)
		err = FundAccount(s.ctx, s.simApp.BankKeeper, vaultAddress, sdk.NewCoins(underlying))
		s.Require().NoError(err)
		err = FundAccount(s.ctx, s.simApp.BankKeeper, markertypes.MustGetMarkerAddress(shareDenom), sdk.NewCoins(underlying))
		s.Require().NoError(err)
		s.ctx = s.ctx.WithBlockTime(testBlockTime)
		if periodStartSeconds != 0 {
			s.Require().NoError(s.k.VaultInterestDetails.Set(s.ctx, vaultAddress, types.VaultInterestDetails{
				PeriodStart: periodStartSeconds,
			}))
		}
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
				setup("0.25", 0)
			},
			posthander: func() {
				s.assertVaultInterestPeriod(vaultAddress, testBlockTime.Unix())
				s.assertVaultAndMarkerBalances(vaultAddress, shareDenom, underlying.Denom, underlying.Amount, underlying.Amount)
			},
			expectedEvents: sdk.Events{},
		},
		{
			name: "interest period start in future, should return nil and do nothing",
			setup: func() {
				setup("0.25", futureTime.Unix())
			},
			posthander: func() {
				s.assertVaultInterestPeriod(vaultAddress, futureTime.Unix())
				s.assertVaultAndMarkerBalances(vaultAddress, shareDenom, underlying.Denom, underlying.Amount, underlying.Amount)
			},
			expectedEvents: sdk.Events{},
		},
		{
			name: "interest period has elasped, should pay interest and update period start",
			setup: func() {
				setup("0.25", pastTime.Unix())
			},
			posthander: func() {
				s.assertVaultInterestPeriod(vaultAddress, testBlockTime.Unix())
				s.assertVaultAndMarkerBalances(vaultAddress, shareDenom, underlying.Denom, sdkmath.NewInt(958047987), sdkmath.NewInt(1041952013))
			},
			expectedEvents: createReconcileEvents(vaultAddress, markertypes.MustGetMarkerAddress(shareDenom), sdkmath.NewInt(41952013), sdkmath.NewInt(1_000_000_000), sdkmath.NewInt(1041952013), underlying.Denom, "0.25", 5_184_000),
		},
		{
			name: "interest period has elasped, should pay negative interest and update period start",
			setup: func() {
				setup("-0.25", pastTime.Unix())
			},
			posthander: func() {
				s.assertVaultInterestPeriod(vaultAddress, testBlockTime.Unix())
				s.assertVaultAndMarkerBalances(vaultAddress, shareDenom, underlying.Denom, sdkmath.NewInt(1040262904), sdkmath.NewInt(959737096))
			},
			expectedEvents: createReconcileEvents(vaultAddress, markertypes.MustGetMarkerAddress(shareDenom), sdkmath.NewInt(-40262904), sdkmath.NewInt(1_000_000_000), sdkmath.NewInt(959737096), underlying.Denom, "-0.25", 5_184_000),
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			if tc.setup != nil {
				tc.setup()
			}

			vault, err := s.k.GetVault(s.ctx, vaultAddress)
			s.Require().NoError(err, "failed to get vault for test setup")
			err = s.k.ReconcileVaultInterest(s.ctx, vault)

			if tc.posthander != nil {
				tc.posthander()
			}
			if len(tc.expectedErrSubstr) > 0 {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.expectedErrSubstr)
			}

			s.Assert().Equal(
				normalizeEvents(tc.expectedEvents),
				normalizeEvents(s.ctx.EventManager().Events()))
		})
	}
}

func createReconcileEvents(vaultAddr, markerAddr sdk.AccAddress, interest, principle, principleAfter sdkmath.Int, denom, rate string, durations int64) []sdk.Event {
	var allEvents []sdk.Event

	r, err := sdkmath.LegacyNewDecFromStr(rate)
	if err != nil {
		panic(fmt.Sprintf("invalid rate %s: %v", rate, err))
	}
	var fromAddress string
	var toAddress string
	if r.IsNegative() {
		fromAddress = markerAddr.String()
		toAddress = vaultAddr.String()
	} else {
		fromAddress = vaultAddr.String()
		toAddress = markerAddr.String()
	}
	sendToMarkerEvents := createSendCoinEvents(fromAddress, toAddress, sdk.NewCoin(denom, interest.Abs()).String())
	allEvents = append(allEvents, sendToMarkerEvents...)

	reconcileEvent := sdk.NewEvent("vault.v1.EventVaultReconcile",
		sdk.NewAttribute("interest_earned", CoinToJSON(sdk.Coin{Denom: denom, Amount: interest})),
		sdk.NewAttribute("principal_after", CoinToJSON(sdk.NewCoin(denom, principleAfter))),
		sdk.NewAttribute("principal_before", CoinToJSON(sdk.NewCoin(denom, principle))),
		sdk.NewAttribute("rate", rate),
		sdk.NewAttribute("time", fmt.Sprintf("%v", durations)),
		sdk.NewAttribute("vault_address", vaultAddr.String()),
	)
	allEvents = append(allEvents, reconcileEvent)
	return allEvents
}

func (s *TestSuite) TestKeeper_EstimateVaultTotalAssets() {
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
		s.k.AuthKeeper.SetAccount(s.ctx, vault)

		if periodStartSeconds != 0 {
			s.Require().NoError(s.k.VaultInterestDetails.Set(s.ctx, vaultAddress, types.VaultInterestDetails{
				PeriodStart: periodStartSeconds,
			}))
		}
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
			rate:             "0.0",
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
			est, err := s.k.EstimateVaultTotalAssets(s.ctx, vault, underlying)

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
	twoMonthsAgo := time.Now().Add(-60 * 24 * time.Hour).Unix()
	shareDenom := "vaultshares"
	underlying := sdk.NewInt64Coin("underlying", 1_000_000_000)
	vaultAddr := types.GetVaultAddress(shareDenom)
	testBlockTime := time.Now()

	tests := []struct {
		name          string
		setup         func()
		expectedExist bool
	}{
		{
			name: "valid vault with expired interest triggers reconciliation",
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
				s.k.AuthKeeper.SetAccount(s.ctx, vault)

				s.Require().NoError(s.k.VaultInterestDetails.Set(s.ctx, vaultAddr, types.VaultInterestDetails{
					PeriodStart: twoMonthsAgo,
					ExpireTime:  testBlockTime.Unix(),
				}))

				s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, vaultAddr, sdk.NewCoins(underlying)))
				s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, markertypes.MustGetMarkerAddress(shareDenom), sdk.NewCoins(underlying)))
				s.ctx = s.ctx.WithBlockTime(testBlockTime)
			},
			expectedExist: true,
		},
		{
			name: "missing vault deletes interest record",
			setup: func() {
				s.Require().NoError(s.k.VaultInterestDetails.Set(s.ctx, vaultAddr, types.VaultInterestDetails{
					PeriodStart: twoMonthsAgo,
					ExpireTime:  testBlockTime.Unix(),
				}))
				s.ctx = s.ctx.WithBlockTime(testBlockTime)
			},
			expectedExist: false,
		},
		// {
		// 	name: "malformed value triggers deletion",
		// 	setup: func() {
		// 		store := s.ctx.KVStore(s.storeKey)
		// 		key, err := s.k.VaultInterestDetails.KeyCodec().Encode(vaultAddr)
		// 		s.Require().NoError(err)
		// 		store.Set(key, []byte("garbage"))
		// 		s.ctx = s.ctx.WithBlockTime(testBlockTime)
		// 	},
		// 	expectedExist: false,
		// },
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			tc.setup()

			s.Require().NoError(s.k.HandleVaultInterestTimeouts(s.ctx))

			exists, err := s.k.VaultInterestDetails.Has(s.ctx, vaultAddr)
			s.Require().NoError(err)
			s.Require().Equal(tc.expectedExist, exists)
		})
	}
}

func (s *TestSuite) TestKeeper_HandleReconciledVaults() {
	v1, v2, v3 := NewVaultInfo(1), NewVaultInfo(2), NewVaultInfo(3)
	v4, v5, v6 := NewVaultInfo(4), NewVaultInfo(5), NewVaultInfo(6)

	testBlockTime := time.Now().UTC().Truncate(time.Second)
	pastTime := testBlockTime.Add(-1 * time.Hour)

	tests := []struct {
		name      string
		setup     func()
		postCheck func()
		expectErr bool
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
			expectErr: false,
		},
		{
			name: "one vault reconciled, sufficient funds",
			setup: func() {
				createVaultWithInterest(s, v1, "0.1", testBlockTime.Unix(), testBlockTime.Unix(), true, true)
			},
			postCheck: func() {
				// Vault interest details should be updated
				details, err := s.k.VaultInterestDetails.Get(s.ctx, v1.vaultAddr)
				s.Require().NoError(err)
				s.Assert().Equal(testBlockTime.Unix(), details.PeriodStart, "period start should not change")
				expectedExpireTime := testBlockTime.Unix() + keeper.AutoReconcileTimeout
				s.Assert().Equal(expectedExpireTime, details.ExpireTime, "expire time should be extended")

				// Vault interest rate should not change
				vault, err := s.k.GetVault(s.ctx, v1.vaultAddr)
				s.Require().NoError(err)
				s.Require().NotNil(vault)
				s.Assert().Equal("0.1", vault.CurrentInterestRate, "interest rate should not change")
			},
			expectErr: false,
		},
		{
			name: "one vault reconciled, insufficient funds",
			setup: func() {
				createVaultWithInterest(s, v1, "0.1", testBlockTime.Unix(), testBlockTime.Unix(), false, true)
			},
			postCheck: func() {
				// Vault should be depleted
				vault, err := s.k.GetVault(s.ctx, v1.vaultAddr)
				s.Require().NoError(err)
				s.Require().NotNil(vault)
				s.Assert().Equal("0", vault.CurrentInterestRate, "interest rate should be zeroed out")

				// Interest details should be removed
				has, err := s.k.VaultInterestDetails.Has(s.ctx, v1.vaultAddr)
				s.Require().NoError(err)
				s.Assert().False(has, "interest details should be removed for depleted vault")
			},
			expectErr: false,
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
				// Check vault 1 (payable)
				details1, err := s.k.VaultInterestDetails.Get(s.ctx, v1.vaultAddr)
				s.Require().NoError(err)
				expectedExpireTime := testBlockTime.Unix() + keeper.AutoReconcileTimeout
				s.Assert().Equal(expectedExpireTime, details1.ExpireTime, "vault 1 expire time should be extended")
				vault1, err := s.k.GetVault(s.ctx, v1.vaultAddr)
				s.Require().NoError(err)
				s.Assert().Equal("0.1", vault1.CurrentInterestRate, "vault 1 interest rate should not change")

				// Check vault 2 (depleted)
				vault2, err := s.k.GetVault(s.ctx, v2.vaultAddr)
				s.Require().NoError(err)
				s.Assert().Equal("0", vault2.CurrentInterestRate, "vault 2 interest rate should be zeroed out")
				has, err := s.k.VaultInterestDetails.Has(s.ctx, v2.vaultAddr)
				s.Require().NoError(err)
				s.Assert().False(has, "vault 2 interest details should be removed")
			},
			expectErr: false,
		},
		{
			name: "one vault, not reconciled",
			setup: func() {
				createVaultWithInterest(s, v1, "0.1", pastTime.Unix(), testBlockTime.Unix(), true, true)
			},
			postCheck: func() {
				// Vault interest details should NOT be updated
				details, err := s.k.VaultInterestDetails.Get(s.ctx, v1.vaultAddr)
				s.Require().NoError(err)
				s.Assert().Equal(pastTime.Unix(), details.PeriodStart, "period start should not change")
				s.Assert().Equal(testBlockTime.Unix(), details.ExpireTime, "expire time should not change")

				// Vault interest rate should not change
				vault, err := s.k.GetVault(s.ctx, v1.vaultAddr)
				s.Require().NoError(err)
				s.Require().NotNil(vault)
				s.Assert().Equal("0.1", vault.CurrentInterestRate, "interest rate should not change")
			},
			expectErr: false,
		},
		{
			name: "three vaults, two reconciled (one payable, one depleted), one not reconciled",
			setup: func() {
				// Vault 1: payable, reconciled
				createVaultWithInterest(s, v1, "0.1", testBlockTime.Unix(), testBlockTime.Unix(), true, true)
				// Vault 2: depleted, reconciled
				createVaultWithInterest(s, v2, "0.1", testBlockTime.Unix(), testBlockTime.Unix(), false, true)
				// Vault 3: not reconciled
				createVaultWithInterest(s, v3, "0.1", pastTime.Unix(), testBlockTime.Unix(), true, true)
			},
			postCheck: func() {
				// Check vault 1 (payable)
				details1, err := s.k.VaultInterestDetails.Get(s.ctx, v1.vaultAddr)
				s.Require().NoError(err)
				expectedExpireTime := testBlockTime.Unix() + keeper.AutoReconcileTimeout
				s.Assert().Equal(expectedExpireTime, details1.ExpireTime, "vault 1 expire time should be extended")
				vault1, err := s.k.GetVault(s.ctx, v1.vaultAddr)
				s.Require().NoError(err)
				s.Assert().Equal("0.1", vault1.CurrentInterestRate, "vault 1 interest rate should not change")

				// Check vault 2 (depleted)
				vault2, err := s.k.GetVault(s.ctx, v2.vaultAddr)
				s.Require().NoError(err)
				s.Assert().Equal("0", vault2.CurrentInterestRate, "vault 2 interest rate should be zeroed out")
				has, err := s.k.VaultInterestDetails.Has(s.ctx, v2.vaultAddr)
				s.Require().NoError(err)
				s.Assert().False(has, "vault 2 interest details should be removed")

				// Check vault 3 (not reconciled)
				details3, err := s.k.VaultInterestDetails.Get(s.ctx, v3.vaultAddr)
				s.Require().NoError(err)
				s.Assert().Equal(pastTime.Unix(), details3.PeriodStart, "vault 3 period start should not change")
				s.Assert().Equal(testBlockTime.Unix(), details3.ExpireTime, "vault 3 expire time should not change")
				vault3, err := s.k.GetVault(s.ctx, v3.vaultAddr)
				s.Require().NoError(err)
				s.Assert().Equal("0.1", vault3.CurrentInterestRate, "vault 3 interest rate should not change")
			},
			expectErr: false,
		},
		{
			name: "six vaults, two payable, two depleted, two not reconciled",
			setup: func() {
				// Vaults 1 & 4: payable, reconciled
				createVaultWithInterest(s, v1, "0.1", testBlockTime.Unix(), testBlockTime.Unix(), true, true)
				createVaultWithInterest(s, v4, "0.1", testBlockTime.Unix(), testBlockTime.Unix(), true, true)
				// Vaults 2 & 5: depleted, reconciled
				createVaultWithInterest(s, v2, "0.1", testBlockTime.Unix(), testBlockTime.Unix(), false, true)
				createVaultWithInterest(s, v5, "0.1", testBlockTime.Unix(), testBlockTime.Unix(), false, true)
				// Vaults 3 & 6: not reconciled
				createVaultWithInterest(s, v3, "0.1", pastTime.Unix(), testBlockTime.Unix(), true, true)
				createVaultWithInterest(s, v6, "0.1", pastTime.Unix(), testBlockTime.Unix(), true, true)
			},
			postCheck: func() {
				expectedExpireTime := testBlockTime.Unix() + keeper.AutoReconcileTimeout

				// Check vault 1 (payable)
				details1, err := s.k.VaultInterestDetails.Get(s.ctx, v1.vaultAddr)
				s.Require().NoError(err)
				s.Assert().Equal(expectedExpireTime, details1.ExpireTime, "vault 1 expire time should be extended")
				vault1, err := s.k.GetVault(s.ctx, v1.vaultAddr)
				s.Require().NoError(err)
				s.Assert().Equal("0.1", vault1.CurrentInterestRate, "vault 1 interest rate should not change")

				// Check vault 4 (payable)
				details4, err := s.k.VaultInterestDetails.Get(s.ctx, v4.vaultAddr)
				s.Require().NoError(err)
				s.Assert().Equal(expectedExpireTime, details4.ExpireTime, "vault 4 expire time should be extended")
				vault4, err := s.k.GetVault(s.ctx, v4.vaultAddr)
				s.Require().NoError(err)
				s.Assert().Equal("0.1", vault4.CurrentInterestRate, "vault 4 interest rate should not change")

				// Check vault 2 (depleted)
				vault2, err := s.k.GetVault(s.ctx, v2.vaultAddr)
				s.Require().NoError(err)
				s.Assert().Equal("0", vault2.CurrentInterestRate, "vault 2 interest rate should be zeroed out")
				has2, err := s.k.VaultInterestDetails.Has(s.ctx, v2.vaultAddr)
				s.Require().NoError(err)
				s.Assert().False(has2, "vault 2 interest details should be removed")

				// Check vault 5 (depleted)
				vault5, err := s.k.GetVault(s.ctx, v5.vaultAddr)
				s.Require().NoError(err)
				s.Assert().Equal("0", vault5.CurrentInterestRate, "vault 5 interest rate should be zeroed out")
				has5, err := s.k.VaultInterestDetails.Has(s.ctx, v5.vaultAddr)
				s.Require().NoError(err)
				s.Assert().False(has5, "vault 5 interest details should be removed")

				// Check vault 3 (not reconciled)
				details3, err := s.k.VaultInterestDetails.Get(s.ctx, v3.vaultAddr)
				s.Require().NoError(err)
				s.Assert().Equal(pastTime.Unix(), details3.PeriodStart, "vault 3 period start should not change")
				s.Assert().Equal(testBlockTime.Unix(), details3.ExpireTime, "vault 3 expire time should not change")
				vault3, err := s.k.GetVault(s.ctx, v3.vaultAddr)
				s.Require().NoError(err)
				s.Assert().Equal("0.1", vault3.CurrentInterestRate, "vault 3 interest rate should not change")

				// Check vault 6 (not reconciled)
				details6, err := s.k.VaultInterestDetails.Get(s.ctx, v6.vaultAddr)
				s.Require().NoError(err)
				s.Assert().Equal(pastTime.Unix(), details6.PeriodStart, "vault 6 period start should not change")
				s.Assert().Equal(testBlockTime.Unix(), details6.ExpireTime, "vault 6 expire time should not change")
				vault6, err := s.k.GetVault(s.ctx, v6.vaultAddr)
				s.Require().NoError(err)
				s.Assert().Equal("0.1", vault6.CurrentInterestRate, "vault 6 interest rate should not change")
			},
			expectErr: false,
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			s.ctx = s.ctx.WithBlockTime(testBlockTime)
			if tc.setup != nil {
				tc.setup()
			}

			err := s.k.HandleReconciledVaults(s.ctx)

			if tc.expectErr {
				s.Require().Error(err)
			} else {
				s.Require().NoError(err)
			}

			if tc.postCheck != nil {
				tc.postCheck()
			}
		})
	}
}

func (s *TestSuite) TestKeeper_PartitionReconciledVaults() {
	v1, v2, v3, v4 := NewVaultInfo(1), NewVaultInfo(2), NewVaultInfo(3), NewVaultInfo(4)
	testBlockTime := time.Now().UTC().Truncate(time.Second)

	tests := []struct {
		name             string
		setup            func() []keeper.ReconciledVault
		expectedPayable  []sdk.AccAddress
		expectedDepleted []sdk.AccAddress
	}{
		{
			name: "empty list of vaults",
			setup: func() []keeper.ReconciledVault {
				return []keeper.ReconciledVault{}
			},
			expectedPayable:  []sdk.AccAddress{},
			expectedDepleted: []sdk.AccAddress{},
		},
		{
			name: "one payable vault",
			setup: func() []keeper.ReconciledVault {
				vault := createVaultWithInterest(s, v1, "0.1", 0, 0, true, true)
				details, err := s.k.VaultInterestDetails.Get(s.ctx, v1.vaultAddr)
				s.Require().NoError(err)
				return []keeper.ReconciledVault{{
					Vault:           vault,
					InterestDetails: &details,
				}}
			},
			expectedPayable:  []sdk.AccAddress{v1.vaultAddr},
			expectedDepleted: []sdk.AccAddress{},
		},
		{
			name: "one depleted vault",
			setup: func() []keeper.ReconciledVault {
				vault := createVaultWithInterest(s, v1, "0.1", 0, 0, false, true)
				details, err := s.k.VaultInterestDetails.Get(s.ctx, v1.vaultAddr)
				s.Require().NoError(err)
				return []keeper.ReconciledVault{{
					Vault:           vault,
					InterestDetails: &details,
				}}
			},
			expectedPayable:  []sdk.AccAddress{},
			expectedDepleted: []sdk.AccAddress{v1.vaultAddr},
		},
		{
			name: "multiple payable and depleted vaults",
			setup: func() []keeper.ReconciledVault {
				vault1 := createVaultWithInterest(s, v1, "0.1", 0, 0, true, true)
				details1, err := s.k.VaultInterestDetails.Get(s.ctx, v1.vaultAddr)
				s.Require().NoError(err)
				vault2 := createVaultWithInterest(s, v2, "0.1", 0, 0, false, true)
				details2, err := s.k.VaultInterestDetails.Get(s.ctx, v2.vaultAddr)
				s.Require().NoError(err)
				vault3 := createVaultWithInterest(s, v3, "0.1", 0, 0, true, true)
				details3, err := s.k.VaultInterestDetails.Get(s.ctx, v3.vaultAddr)
				s.Require().NoError(err)
				vault4 := createVaultWithInterest(s, v4, "0.1", 0, 0, false, true)
				details4, err := s.k.VaultInterestDetails.Get(s.ctx, v4.vaultAddr)
				s.Require().NoError(err)
				return []keeper.ReconciledVault{
					{Vault: vault1, InterestDetails: &details1},
					{Vault: vault2, InterestDetails: &details2},
					{Vault: vault3, InterestDetails: &details3},
					{Vault: vault4, InterestDetails: &details4},
				}
			},
			expectedPayable:  []sdk.AccAddress{v1.vaultAddr, v3.vaultAddr},
			expectedDepleted: []sdk.AccAddress{v2.vaultAddr, v4.vaultAddr},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			s.ctx = s.ctx.WithBlockTime(testBlockTime)
			reconciledVaults := tc.setup()

			payable, depleted := s.k.TestAccessor_partitionReconciledVaults(s.ctx, reconciledVaults)

			s.Assert().Len(payable, len(tc.expectedPayable), "payable vaults count")
			s.Assert().Len(depleted, len(tc.expectedDepleted), "depleted vaults count")
		})
	}
}

func (s *TestSuite) TestKeeper_SetInterestRate() {
	v1 := NewVaultInfo(1)
	initialRate := "0.1"
	newRate := "0.2"

	tests := []struct {
		name           string
		setup          func() *types.VaultAccount
		rateToSet      string
		expectedEvents sdk.Events
		postCheck      func(vault *types.VaultAccount)
	}{
		{
			name: "successful rate change",
			setup: func() *types.VaultAccount {
				return createVaultWithInterest(s, v1, initialRate, 0, 0, false, false)
			},
			rateToSet: newRate,
			expectedEvents: sdk.Events{
				sdk.NewEvent(
					"vault.v1.EventVaultInterestChange",
					sdk.NewAttribute("new_rate", newRate),
					sdk.NewAttribute("previous_rate", initialRate),
					sdk.NewAttribute("vault_address", v1.vaultAddr.String()),
				),
			},
			postCheck: func(vault *types.VaultAccount) {
				updatedVault, err := s.k.GetVault(s.ctx, vault.GetAddress())
				s.Require().NoError(err)
				s.Require().NotNil(updatedVault)
				s.Assert().Equal(newRate, updatedVault.CurrentInterestRate)
			},
		},
		{
			name: "rate is the same, no change",
			setup: func() *types.VaultAccount {
				return createVaultWithInterest(s, v1, initialRate, 0, 0, false, false)
			},
			rateToSet:      initialRate,
			expectedEvents: sdk.Events{},
			postCheck: func(vault *types.VaultAccount) {
				updatedVault, err := s.k.GetVault(s.ctx, vault.GetAddress())
				s.Require().NoError(err)
				s.Require().NotNil(updatedVault)
				s.Assert().Equal(initialRate, updatedVault.CurrentInterestRate)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			vault := tc.setup()

			s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
			s.k.SetInterestRate(s.ctx, vault, tc.rateToSet)

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

func createVaultWithInterest(s *TestSuite, info VaultInfo, interestRate string, periodStart, expireTime int64, fundReserves, fundPrincipal bool) *types.VaultAccount {
	s.requireAddFinalizeAndActivateMarker(info.underlying, s.adminAddr)
	vault, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
		Admin:           s.adminAddr.String(),
		ShareDenom:      info.shareDenom,
		UnderlyingAsset: info.underlying.Denom,
	})
	s.Require().NoError(err)

	vault.CurrentInterestRate = interestRate
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

	s.Require().NoError(s.k.VaultInterestDetails.Set(s.ctx, info.vaultAddr, types.VaultInterestDetails{
		PeriodStart: periodStart,
		ExpireTime:  expireTime,
	}))
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
