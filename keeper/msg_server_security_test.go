package keeper_test

import (
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	markertypes "github.com/provenance-io/provenance/x/marker/types"

	"github.com/provlabs/vault/types"
	"github.com/provlabs/vault/utils"
)

func (s *TestSuite) TestMsgServer_SmallFirstSwapIn_HugeDonation_SwapOut() {
	underlying := "underlying"
	shareDenom := "vaultshares"
	owner := s.adminAddr
	vaultAddr := types.GetVaultAddress(shareDenom)
	markerAddr := markertypes.MustGetMarkerAddress(shareDenom)

	tiny := sdk.NewInt64Coin(underlying, 1)
	hugeDonation := sdk.NewInt64Coin(underlying, 1_000_000_000)

	s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlying, math.NewInt(10_000_000_000)), owner)
	vault, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
		Admin:           owner.String(),
		ShareDenom:      shareDenom,
		UnderlyingAsset: underlying,
	})
	s.Require().NoError(err, "vault creation should succeed")
	vault.SwapInEnabled = true
	vault.SwapOutEnabled = true
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	err = FundAccount(s.ctx, s.simApp.BankKeeper, owner, sdk.NewCoins(tiny))
	s.Require().NoError(err, "funding owner with tiny underlying should succeed")

	s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	respIn, err := s.k.SwapIn(s.ctx, vaultAddr, owner, tiny)
	s.Require().NoError(err, "tiny swap-in should succeed")
	s.Require().Equal(tiny.Amount.Mul(utils.ShareScalar), respIn.Amount, "tiny swap-in should mint deposit * ShareScalar shares")

	totalShares := s.simApp.BankKeeper.GetSupply(s.ctx, shareDenom).Amount
	totalAssets := s.simApp.BankKeeper.GetBalance(s.ctx, markerAddr, underlying).Amount
	impliedPricePre := totalAssets.Mul(utils.ShareScalar).Quo(totalShares) // assets per ShareScalar shares
	s.Require().Equal(math.NewInt(1), impliedPricePre, "implied price should be 1 right after first swap-in")

	err = FundAccount(s.ctx, s.simApp.BankKeeper, markerAddr, sdk.NewCoins(hugeDonation))
	s.Require().NoError(err, "funding marker with huge donation should succeed")

	s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	sharesToBurn := sdk.NewCoin(shareDenom, respIn.Amount)
	respOut, err := s.k.SwapOut(s.ctx, vaultAddr, owner, sharesToBurn)
	s.Require().NoError(err, "swap-out of all tiny depositor shares should succeed")

	s.Require().True(
		respOut.Amount.GTE(tiny.Amount),
		"payout should be >= original tiny swap-in (got=%s want>=%s)",
		respOut.Amount, tiny.Amount,
	)

	currentVaultAssets := s.simApp.BankKeeper.GetBalance(s.ctx, markerAddr, underlying).Amount
	s.Require().True(
		respOut.Amount.LTE(currentVaultAssets),
		"payout should be <= current vault assets (got=%s vault=%s)",
		respOut.Amount, currentVaultAssets,
	)

	postTotalShares := s.simApp.BankKeeper.GetSupply(s.ctx, shareDenom).Amount
	postTotalAssets := s.simApp.BankKeeper.GetBalance(s.ctx, markerAddr, underlying).Amount
	if !postTotalShares.IsZero() {
		impliedPrice := postTotalAssets.Mul(utils.ShareScalar).Quo(postTotalShares) // assets per ShareScalar shares
		s.Require().Equal(math.NewInt(1), impliedPrice, "implied price should normalize to 1 at scale")
	}
}
