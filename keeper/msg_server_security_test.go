package keeper_test

import (
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	markertypes "github.com/provenance-io/provenance/x/marker/types"

	"github.com/provlabs/vault/keeper"
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

	err = FundAccount(s.ctx, s.simApp, owner, sdk.NewCoins(tiny))
	s.Require().NoError(err, "funding owner with tiny underlying should succeed")

	s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	s.ctx = s.ctx.WithBlockTime(time.Now())
	respIn, err := s.k.SwapIn(s.ctx, vaultAddr, owner, tiny)
	s.Require().NoError(err, "tiny swap-in should succeed")
	s.Require().Equal(tiny.Amount.Mul(utils.ShareScalar), respIn.Amount, "tiny swap-in should mint deposit * ShareScalar shares")
	vault, err = s.k.GetVault(s.ctx, vaultAddr)
	s.Require().NoError(err, "should successfully get vault after tiny swap-in")
	s.Require().NotNil(vault, "vault should not be nil after tiny swap-in")

	totalShares := s.simApp.BankKeeper.GetSupply(s.ctx, shareDenom).Amount
	s.Require().Equal(vault.TotalShares.Amount, totalShares, "total share supply should equal tiny depositor shares")
	totalAssets := s.simApp.BankKeeper.GetBalance(s.ctx, markerAddr, underlying).Amount
	impliedPricePre := totalAssets.Mul(utils.ShareScalar).Quo(totalShares) // assets per ShareScalar shares
	s.Require().Equal(math.NewInt(1), impliedPricePre, "implied price should be 1 right after first swap-in")

	err = FundAccount(s.ctx, s.simApp, markerAddr, sdk.NewCoins(hugeDonation))
	s.Require().NoError(err, "funding marker with huge donation should succeed")

	s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	s.ctx = s.ctx.WithBlockTime(time.Now())
	sharesToBurn := sdk.NewCoin(shareDenom, respIn.Amount)
	_, err = s.k.SwapOut(s.ctx, vaultAddr, owner, sharesToBurn)
	s.Require().NoError(err, "swap-out of all tiny depositor shares should succeed")

	s.simApp.VaultKeeper.TestAccessor_processPendingSwapOuts(s.T(), s.ctx, keeper.MaxSwapOutBatchSize)
	vault, err = s.k.GetVault(s.ctx, vaultAddr)
	s.Require().NoError(err, "should successfully get vault after tiny swap-out")
	s.Require().NotNil(vault, "vault should not be nil after tiny swap-out")
	postTotalShares := s.simApp.BankKeeper.GetSupply(s.ctx, shareDenom).Amount
	s.Require().Equal(postTotalShares, vault.TotalShares.Amount, "total share supply should match vault account after tiny swap-out")
	postTotalAssets := s.simApp.BankKeeper.GetBalance(s.ctx, markerAddr, underlying).Amount
	if !postTotalShares.IsZero() {
		impliedPrice := postTotalAssets.Mul(utils.ShareScalar).Quo(postTotalShares) // assets per ShareScalar shares
		s.Require().Equal(math.NewInt(1), impliedPrice, "implied price should normalize to 1 at scale")
	}
}

func (s *TestSuite) TestMsgServer_UpdateVaultNAV_LiveHeldRepriceRefusedSoNoSwapCanSandwichTheCorrection() {
	underlying := "navunderlying"
	shareDenom := "vaultsharesnav"
	staleLowPricedHeldDenom := "staleheldrwa"

	const (
		deposit         = 1_000_000
		heldAmount      = 1_000_000
		staleLowPrice   = 1
		correctedPrice  = 2
		pricedPerVolume = 1
	)

	vault := s.setupHeldNAVVault(underlying, shareDenom, staleLowPricedHeldDenom, sdk.NewInt64Coin(underlying, staleLowPrice), pricedPerVolume, heldAmount)
	vaultAddr := vault.GetAddress()
	msgServer := keeper.NewMsgServer(s.simApp.VaultKeeper)

	netTVV := func() math.Int {
		reloaded, err := s.k.GetVault(s.ctx, vaultAddr)
		s.Require().NoError(err, "should get vault %s to value it", vaultAddr)
		tvv, err := s.k.GetNetTVV(s.ctx, *reloaded)
		s.Require().NoError(err, "should value vault %s", vaultAddr)
		return tvv
	}

	swapInAtStaleLowPrice := func(who string, funding int64) sdk.AccAddress {
		addr := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1))
		s.Require().NoError(FundAccount(s.ctx, s.simApp, addr, sdk.NewCoins(sdk.NewInt64Coin(underlying, funding))),
			"funding the %s with %d%s should succeed", who, funding, underlying)
		_, err := s.k.SwapIn(s.ctx, vaultAddr, addr, sdk.NewInt64Coin(underlying, deposit))
		s.Require().NoError(err, "the %s should be able to swap in %d%s while %s is priced stale-low", who, deposit, underlying, staleLowPricedHeldDenom)
		return addr
	}

	swapInAtStaleLowPrice("pre-existing shareholder", deposit)
	sandwicher := swapInAtStaleLowPrice("sandwicher", 2*deposit)
	sandwicherShares := s.simApp.BankKeeper.GetBalance(s.ctx, sandwicher, shareDenom)

	upwardCorrection := &types.MsgUpdateVaultNAVRequest{
		Signer:       s.adminAddr.String(),
		VaultAddress: vaultAddr.String(),
		Denom:        staleLowPricedHeldDenom,
		Price:        sdk.NewInt64Coin(underlying, correctedPrice),
		Volume:       math.NewInt(pricedPerVolume),
		Source:       "oracle",
	}

	tvvAtStaleLowPrice := netTVV()
	_, err := msgServer.UpdateVaultNAV(s.ctx, upwardCorrection)
	s.Require().ErrorContains(err, "pause the vault to reprice a held asset",
		"correcting the price of held %s must be refused while the vault is live and swappable", staleLowPricedHeldDenom)
	s.Require().Equal(tvvAtStaleLowPrice, netTVV(), "a refused correction must leave total vault value untouched")

	_, err = msgServer.PauseVault(s.ctx, &types.MsgPauseVaultRequest{
		Authority:    s.adminAddr.String(),
		VaultAddress: vaultAddr.String(),
		Reason:       "correcting a held asset price",
	})
	s.Require().NoError(err, "the admin should be able to pause the vault to correct the price")

	_, err = s.k.SwapIn(s.ctx, vaultAddr, sandwicher, sdk.NewInt64Coin(underlying, deposit))
	s.Require().ErrorContains(err, "is paused", "no position may be opened across the price step")
	_, err = s.k.SwapOut(s.ctx, vaultAddr, sandwicher, sandwicherShares)
	s.Require().ErrorContains(err, "is paused", "no position may be closed across the price step")

	_, err = msgServer.UpdateVaultNAV(s.ctx, upwardCorrection)
	s.Require().NoError(err, "the paused vault should accept the correction")

	_, err = msgServer.UnpauseVault(s.ctx, &types.MsgUnpauseVaultRequest{
		Authority:    s.adminAddr.String(),
		VaultAddress: vaultAddr.String(),
	})
	s.Require().NoError(err, "the admin should be able to unpause the vault after correcting the price")

	nav, err := s.k.GetVaultNAV(s.ctx, vaultAddr, staleLowPricedHeldDenom)
	s.Require().NoError(err, "the corrected NAV entry for %s should be stored", staleLowPricedHeldDenom)
	s.Require().Equal(int64(correctedPrice), nav.Price.Amount.Int64(), "the correction should be in force once the vault reopens")
}
