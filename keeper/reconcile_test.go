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
		err = FundAccount(s.ctx, s.simApp.BankKeeper, vaultAddress, sdk.NewCoins(sdk.NewCoin(underlying.Denom, sdkmath.NewInt(500_000_000))))
		s.Require().NoError(err)
		err = FundAccount(s.ctx, s.simApp.BankKeeper, markertypes.MustGetMarkerAddress(shareDenom), sdk.NewCoins(underlying))
		s.Require().NoError(err)
		s.ctx = s.ctx.WithBlockTime(time.Now())
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
				interestDetails, err := s.k.VaultInterestDetails.Get(s.ctx, vaultAddress)
				s.Require().NoError(err, "failed to get interest details after setup")
				s.Require().Equal(s.ctx.BlockTime().Unix(), interestDetails.PeriodStart, "period start should be set to current time")
			},
			expectedErrSubstr: "insufficient reserves",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			if tc.setup != nil {
				tc.setup()
			}

			vault, err := s.k.GetVault(s.ctx, vaultAddress)
			s.Require().NoError(err, "failed to get vault for test setup")
			err = s.k.ReconcileVaultInterest(s.ctx, vault)

			s.Require().Error(err)
			s.Require().Contains(err.Error(), tc.expectedErrSubstr)
		})
	}
}
