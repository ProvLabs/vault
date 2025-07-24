package keeper_test

import (
	"fmt"
	"time"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	markertypes "github.com/provenance-io/provenance/x/marker/types"

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
		vault.InterestRate = interestRate
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
		vault.InterestRate = interestRate
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
				vault.InterestRate = "0.25"
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
