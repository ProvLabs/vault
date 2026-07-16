package simulation_test

import (
	"time"

	"github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/simulation"
	"github.com/provlabs/vault/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
)

// newAccuracyVault creates a vault for estimate-accuracy testing. It disables the AUM fee and
// sets a zero withdrawal delay so that the estimate query and the settled swap evaluate against
// identical vault state at the same block height. AUM fees are disabled because the estimate
// reports a net-of-fee value while the settled swap reads gross post-reconcile value; leaving the
// default fee enabled would inject a fee-accounting delta that is unrelated to swap math accuracy.
func (s *VaultSimTestSuite) newAccuracyVault(admin simtypes.Account, underlying, share string) *types.VaultAccount {
	err := simulation.CreateVault(s.ctx, s.app.VaultKeeper, s.app.AccountKeeper, s.app.BankKeeper, s.app.MarkerKeeper, underlying, share, admin, s.accs)
	s.Require().NoError(err, "CreateVault for share denom %s", share)

	vault, err := s.app.VaultKeeper.GetVault(s.ctx, types.GetVaultAddress(share))
	s.Require().NoError(err, "GetVault for share denom %s", share)
	s.Require().NotNil(vault, "vault for share denom %s should exist after creation", share)

	vault.AumFeeBips = 0
	vault.WithdrawalDelaySeconds = 0
	s.Require().NoError(s.app.VaultKeeper.SetVaultAccount(s.ctx, vault), "SetVaultAccount for share denom %s", share)
	return vault
}

// accrueInterest configures a vault so that positive interest has been accruing for the given
// elapsed duration as of the current block time, and funds vault reserves so the settlement
// reconcile can fully realize the projected interest. It re-reads the vault from state to avoid
// clobbering fields (e.g. TotalShares) mutated by any preceding swap-in.
func (s *VaultSimTestSuite) accrueInterest(vaultAddr sdk.AccAddress, rate string, elapsed time.Duration, reserves sdk.Coin) {
	vault, err := s.app.VaultKeeper.GetVault(s.ctx, vaultAddr)
	s.Require().NoError(err, "GetVault %s for interest setup", vaultAddr)
	s.Require().NotNil(vault, "vault %s should exist for interest setup", vaultAddr)

	vault.CurrentInterestRate = rate
	vault.DesiredInterestRate = rate
	vault.PeriodStart = s.ctx.BlockTime().Add(-elapsed).Unix()
	s.Require().NoError(s.app.VaultKeeper.SetVaultAccount(s.ctx, vault), "SetVaultAccount with interest for vault %s", vaultAddr)
	s.Require().NoError(FundAccount(s.ctx, s.app.BankKeeper, vaultAddr, sdk.NewCoins(reserves)), "fund reserves %s for vault %s", reserves, vaultAddr)
}

// TestEstimateSwapInAccuracy validates that the EstimateSwapIn query reports the same number of
// shares a depositor actually receives from SwapIn, across a range of vault states. Both paths run
// against the same vault state at the same block height, so an accurate estimate must match the
// settled result exactly.
func (s *VaultSimTestSuite) TestEstimateSwapInAccuracy() {
	tests := []struct {
		name string
		// setup configures the vault and returns the depositor, the vault address, and the deposit.
		setup func() (depositor simtypes.Account, vaultAddr sdk.AccAddress, deposit sdk.Coin)
	}{
		{
			name: "underlying deposit into empty vault",
			setup: func() (simtypes.Account, sdk.AccAddress, sdk.Coin) {
				s.SetupTest()
				vault := s.newAccuracyVault(s.accs[0], "underlying2vx", "accswapinA")
				return s.accs[1], vault.GetAddress(), sdk.NewInt64Coin("underlying2vx", 1_000_000)
			},
		},
		{
			name: "underlying deposit into seeded vault",
			setup: func() (simtypes.Account, sdk.AccAddress, sdk.Coin) {
				s.SetupTest()
				vault := s.newAccuracyVault(s.accs[0], "underlying2vx", "accswapinB")
				s.Require().NoError(simulation.SwapIn(s.ctx, s.app.VaultKeeper, s.accs[1], "accswapinB", sdk.NewInt64Coin("underlying2vx", 2_500_000)), "seed swap-in")
				return s.accs[2], vault.GetAddress(), sdk.NewInt64Coin("underlying2vx", 1_000_000)
			},
		},
		{
			name: "underlying deposit with accrued interest",
			setup: func() (simtypes.Account, sdk.AccAddress, sdk.Coin) {
				s.SetupTest()
				vault := s.newAccuracyVault(s.accs[0], "underlying2vx", "accswapinD")
				s.Require().NoError(simulation.SwapIn(s.ctx, s.app.VaultKeeper, s.accs[1], "accswapinD", sdk.NewInt64Coin("underlying2vx", 3_000_000)), "seed swap-in")
				s.accrueInterest(vault.GetAddress(), "0.10", 30*24*time.Hour, sdk.NewInt64Coin("underlying2vx", 1_000_000))
				return s.accs[2], vault.GetAddress(), sdk.NewInt64Coin("underlying2vx", 1_000_000)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			depositor, vaultAddr, deposit := tc.setup()

			qs := keeper.NewQueryServer(s.app.VaultKeeper)
			estResp, err := qs.EstimateSwapIn(s.ctx, &types.QueryEstimateSwapInRequest{
				VaultAddress: vaultAddr.String(),
				Assets:       deposit,
			})
			s.Require().NoError(err, "EstimateSwapIn for vault %s deposit %s", vaultAddr, deposit)

			sharesBefore := s.app.BankKeeper.GetBalance(s.ctx, depositor.Address, estResp.Assets.Denom)

			msgServer := keeper.NewMsgServer(s.app.VaultKeeper)
			swapResp, err := msgServer.SwapIn(s.ctx, &types.MsgSwapInRequest{
				Owner:        depositor.Address.String(),
				VaultAddress: vaultAddr.String(),
				Assets:       deposit,
			})
			s.Require().NoError(err, "SwapIn for vault %s deposit %s", vaultAddr, deposit)

			s.Require().Equal(estResp.Assets.Denom, swapResp.SharesReceived.Denom, "estimate vs actual share denom mismatch for vault %s", vaultAddr)

			sharesAfter := s.app.BankKeeper.GetBalance(s.ctx, depositor.Address, swapResp.SharesReceived.Denom)
			credited := sharesAfter.Sub(sharesBefore)
			s.Require().Equal(swapResp.SharesReceived, credited, "minted share balance delta should equal reported SharesReceived for vault %s", vaultAddr)

			delta := estResp.Assets.Amount.Sub(swapResp.SharesReceived.Amount).Abs()
			s.T().Logf("EstimateSwapIn accuracy [%s]: estimate=%s actual=%s delta=%s", tc.name, estResp.Assets, swapResp.SharesReceived, delta)
			s.Require().True(delta.IsZero(), "estimate %s and actual %s diverge by %s for vault %s", estResp.Assets, swapResp.SharesReceived, delta, vaultAddr)
		})
	}
}

// TestEstimateSwapOutAccuracy validates that the EstimateSwapOut query reports the same payout an
// owner actually receives once a swap-out settles. SwapOut is asynchronous: the message only
// escrows shares, and the payout is realized by the EndBlocker. With a zero withdrawal delay the
// payout settles in the same block as the estimate, so an accurate estimate must match the realized
// payout exactly. The payout is always the vault's underlying asset.
func (s *VaultSimTestSuite) TestEstimateSwapOutAccuracy() {
	tests := []struct {
		name string
		// setup configures the vault and returns the owner, vault address, shares to redeem, and redeem denom.
		setup func() (owner simtypes.Account, vaultAddr sdk.AccAddress, shares sdk.Coin, redeemDenom string)
	}{
		{
			name: "full underlying redeem, no interest",
			setup: func() (simtypes.Account, sdk.AccAddress, sdk.Coin, string) {
				s.SetupTest()
				vault := s.newAccuracyVault(s.accs[0], "underlying2vx", "accswapoutA")
				owner := s.accs[1]
				s.Require().NoError(simulation.SwapIn(s.ctx, s.app.VaultKeeper, owner, "accswapoutA", sdk.NewInt64Coin("underlying2vx", 2_000_000)), "owner swap-in")
				shares := s.app.BankKeeper.GetBalance(s.ctx, owner.Address, "accswapoutA")
				return owner, vault.GetAddress(), shares, "underlying2vx"
			},
		},
		{
			name: "partial underlying redeem, no interest",
			setup: func() (simtypes.Account, sdk.AccAddress, sdk.Coin, string) {
				s.SetupTest()
				vault := s.newAccuracyVault(s.accs[0], "underlying2vx", "accswapoutB")
				owner := s.accs[1]
				s.Require().NoError(simulation.SwapIn(s.ctx, s.app.VaultKeeper, owner, "accswapoutB", sdk.NewInt64Coin("underlying2vx", 2_000_000)), "owner swap-in")
				bal := s.app.BankKeeper.GetBalance(s.ctx, owner.Address, "accswapoutB")
				return owner, vault.GetAddress(), sdk.NewCoin(bal.Denom, bal.Amount.Quo(math.NewInt(2))), "underlying2vx"
			},
		},
		{
			name: "underlying redeem with accrued interest",
			setup: func() (simtypes.Account, sdk.AccAddress, sdk.Coin, string) {
				s.SetupTest()
				vault := s.newAccuracyVault(s.accs[0], "underlying2vx", "accswapoutD")
				owner := s.accs[1]
				s.Require().NoError(simulation.SwapIn(s.ctx, s.app.VaultKeeper, owner, "accswapoutD", sdk.NewInt64Coin("underlying2vx", 3_000_000)), "owner swap-in")
				s.accrueInterest(vault.GetAddress(), "0.10", 30*24*time.Hour, sdk.NewInt64Coin("underlying2vx", 1_000_000))
				bal := s.app.BankKeeper.GetBalance(s.ctx, owner.Address, "accswapoutD")
				return owner, vault.GetAddress(), sdk.NewCoin(bal.Denom, bal.Amount.Quo(math.NewInt(2))), "underlying2vx"
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			owner, vaultAddr, shares, redeemDenom := tc.setup()

			qs := keeper.NewQueryServer(s.app.VaultKeeper)
			estResp, err := qs.EstimateSwapOut(s.ctx, &types.QueryEstimateSwapOutRequest{
				VaultAddress: vaultAddr.String(),
				Shares:       shares.Amount.String(),
			})
			s.Require().NoError(err, "EstimateSwapOut for vault %s shares %s redeem %s", vaultAddr, shares, redeemDenom)

			payoutBefore := s.app.BankKeeper.GetBalance(s.ctx, owner.Address, redeemDenom)

			msgServer := keeper.NewMsgServer(s.app.VaultKeeper)
			_, err = msgServer.SwapOut(s.ctx, &types.MsgSwapOutRequest{
				Owner:        owner.Address.String(),
				VaultAddress: vaultAddr.String(),
				Assets:       shares,
			})
			s.Require().NoError(err, "SwapOut for vault %s shares %s redeem %s", vaultAddr, shares, redeemDenom)

			s.Require().NoError(s.app.VaultKeeper.EndBlocker(s.ctx), "EndBlocker should settle the pending swap-out for vault %s", vaultAddr)

			payoutAfter := s.app.BankKeeper.GetBalance(s.ctx, owner.Address, redeemDenom)
			actualPayout := payoutAfter.Sub(payoutBefore)

			s.Require().Equal(estResp.Assets.Denom, actualPayout.Denom, "estimate vs actual payout denom mismatch for vault %s", vaultAddr)
			s.Require().True(actualPayout.Amount.IsPositive(), "owner should have received a payout for vault %s (estimate %s); got %s, indicating a refund or unprocessed queue", vaultAddr, estResp.Assets, actualPayout)

			delta := estResp.Assets.Amount.Sub(actualPayout.Amount).Abs()
			s.T().Logf("EstimateSwapOut accuracy [%s]: estimate=%s actual=%s delta=%s", tc.name, estResp.Assets, actualPayout, delta)
			s.Require().True(delta.IsZero(), "estimate %s and actual payout %s diverge by %s for vault %s", estResp.Assets, actualPayout, delta, vaultAddr)
		})
	}
}
