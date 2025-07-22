package keeper_test

import (
	"time"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	markertypes "github.com/provenance-io/provenance/x/marker/types"

	"github.com/provlabs/vault/types"
)

func (s *TestSuite) TestKeeper_ReconcileVaultInterest() {
	shareDenom := "vaultshares"
	underlying := sdk.NewInt64Coin("underlying", 1_000_000_000)
	vaultAddress := types.GetVaultAddress(shareDenom)
	testBlockTime := time.Now()
	futureTime := testBlockTime.Add(100 * time.Second)
	pastTime := testBlockTime.Add(-24 * 60 * time.Hour)

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
	}

	tests := []struct {
		name              string
		setup             func()
		posthander        func()
		expectedErrSubstr string
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
		})
	}
}

func (s *TestSuite) assertVaultInterestPeriod(vaultAddr sdk.AccAddress, expectedPeriod int64) {
	interestDetails, err := s.k.VaultInterestDetails.Get(s.ctx, vaultAddr)
	s.Require().NoError(err, "failed to get interest details")
	s.Assert().Equal(expectedPeriod, interestDetails.PeriodStart, "unexpected interest period start")
}

func (s *TestSuite) assertBalance(addr sdk.AccAddress, denom string, expectedAmt sdkmath.Int) {
	balance := s.simApp.BankKeeper.GetBalance(s.ctx, addr, denom)
	s.Assert().Equal(expectedAmt.String(), balance.Amount.String(), "unexpected balance for %s", addr.String())
}

func (s *TestSuite) assertVaultAndMarkerBalances(vaultAddr sdk.AccAddress, shareDenom string, denom string, expectedVaultAmt, expectedMarkerAmt sdkmath.Int) {
	markerAddr := markertypes.MustGetMarkerAddress(shareDenom)

	s.assertBalance(vaultAddr, denom, expectedVaultAmt)
	s.assertBalance(markerAddr, denom, expectedMarkerAmt)
}
