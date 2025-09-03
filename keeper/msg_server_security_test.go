package keeper_test

import (
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	markertypes "github.com/provenance-io/provenance/x/marker/types"

	"github.com/provlabs/vault/types"
	"github.com/provlabs/vault/utils"
)

// TestMsgServer_SmallFirstSwapIn_HugeDonation_SwapOut verifies our vault's protection
// against the ERC‑4626 “inflation via donation” attack.
//
// Attack model (per OpenZeppelin’s ERC‑4626 analysis https://docs.openzeppelin.com/contracts/5.x/erc4626#the_attack):
//  1. An attacker first mints a tiny amount of shares by depositing a minimal asset amount.
//  2. The attacker then “donates” a very large amount of assets directly to the vault,
//     inflating the asset/share exchange rate and aiming to make subsequent deposits lose
//     value to rounding or allow the attacker to extract outsized redemptions.
//
// What this test simulates:
//   - A tiny initial SwapIn to create shares at the canonical 1:1 price scale
//     (shares are minted at the vault’s higher share precision via utils.ShareScalar).
//   - A huge asset donation directly to the marker (vault principal) to mimic the
//     inflation step of the attack.
//   - A full SwapOut of the tiny depositor’s shares immediately after the donation.
//
// What it asserts:
//   - The tiny depositor can fully redeem without loss: payout >= original tiny deposit.
//   - The payout never exceeds the vault’s current assets.
//   - If any shares remain outstanding, the implied price re-normalizes to 1 at scale
//     (assets per ShareScalar shares), demonstrating price integrity post‑donation.
//
// Why this defeats the attack here:
//   - The vault uses higher-precision shares (ShareScalar) so conversions have negligible
//     rounding loss even at small sizes, removing the “round to zero” leverage.
//   - Donations are not granting the attacker a disproportionate claim against the pool;
//     the depositor’s redeem value is preserved at or above their original contribution.
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
	respOut, err := s.k.SwapOut(s.ctx, vaultAddr, owner, sharesToBurn, "")
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
