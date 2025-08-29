package keeper

import (
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/provlabs/vault/types"
	"github.com/provlabs/vault/utils"
)

// UnitPriceFraction returns the unit price of srcDenom expressed in vault.UnderlyingAsset
// as an integer fraction (priceNumerator, priceDenominator), derived from the NAV.
//
// Semantics:
//   - NAV.Price is the total value (in underlyingAsset units) for NAV.Volume units of srcDenom.
//   - Therefore, 1 srcDenom = (NAV.Price.Amount / NAV.Volume) underlying units.
//   - This function returns (NAV.Price.Amount, NAV.Volume) so callers can apply floor arithmetic.
//
// Special cases and errors:
//   - If srcDenom == underlyingAsset, returns (1, 1).
//   - Errors if no NAV exists for (srcDenom → underlyingAsset).
//   - Errors if NAV.Volume == 0.
func (k Keeper) UnitPriceFraction(ctx sdk.Context, srcDenom, underlyingAsset string) (num, den math.Int, err error) {
	if srcDenom == underlyingAsset {
		return math.NewInt(1), math.NewInt(1), nil
	}
	nav, err := k.MarkerKeeper.GetNetAssetValue(ctx, srcDenom, underlyingAsset)
	if err != nil {
		return math.Int{}, math.Int{}, err
	}
	if nav == nil {
		return math.Int{}, math.Int{}, fmt.Errorf("nav not found for %s/%s", srcDenom, underlyingAsset)
	}
	priceAmt := nav.Price.Amount
	volAmt := math.NewInt(int64(nav.Volume))
	if volAmt.IsZero() {
		return math.Int{}, math.Int{}, fmt.Errorf("nav volume is zero for %s/%s", srcDenom, underlyingAsset)
	}
	return priceAmt, volAmt, nil
}

// ToUnderlyingAssetAmount converts an input coin into its value expressed in
// vault.UnderlyingAsset using integer floor arithmetic.
//
// Formula:
//
//	value_in_underlying = in.Amount * priceNumerator / priceDenominator
//
// where (priceNumerator, priceDenominator) are from UnitPriceFraction(in.Denom → underlying).
// This performs a pure conversion based on NAV (or identity if denom==underlying). It does
// not enforce whether the denom is accepted by the vault; such policy checks are handled elsewhere.
func (k Keeper) ToUnderlyingAssetAmount(ctx sdk.Context, vault types.VaultAccount, in sdk.Coin) (math.Int, error) {
	priceAmount, volume, err := k.UnitPriceFraction(ctx, in.Denom, vault.UnderlyingAsset)
	if err != nil {
		return math.Int{}, err
	}
	return in.Amount.Mul(priceAmount).Quo(volume), nil
}

// FromUnderlyingAssetAmount converts an amount in vault.UnderlyingAsset into a payout coin
// in outDenom using integer floor arithmetic.
//
// Rules:
//   - If outDenom == underlyingAsset, returns that amount directly.
//   - Otherwise uses the reciprocal of UnitPriceFraction(outDenom → underlying):
//     out = underlyingAmount * denominator / numerator
//
// This is a pure denomination conversion (with floor). Liquidity/policy enforcement, if any,
// must be handled by callers.
func (k Keeper) FromUnderlyingAssetAmount(ctx sdk.Context, vault types.VaultAccount, underlyingAmount math.Int, outDenom string) (sdk.Coin, error) {
	if !underlyingAmount.IsPositive() {
		return sdk.NewCoin(outDenom, math.ZeroInt()), nil
	}
	if outDenom == vault.UnderlyingAsset {
		return sdk.NewCoin(outDenom, underlyingAmount), nil
	}

	priceAmount, volume, err := k.UnitPriceFraction(ctx, outDenom, vault.UnderlyingAsset)
	if err != nil {
		return sdk.Coin{}, err
	}
	if priceAmount.IsZero() {
		return sdk.Coin{}, fmt.Errorf("zero price for %s/%s", outDenom, vault.UnderlyingAsset)
	}
	out := underlyingAmount.Mul(volume).Quo(priceAmount)
	return sdk.NewCoin(outDenom, out), nil
}

// GetTVVInUnderlyingAsset returns the Total Vault Value (TVV) expressed in
// vault.UnderlyingAsset using floor arithmetic.
//
// Source of truth:
//   - TVV sums the balances held at the vault’s *principal* account, i.e. the marker
//     address for vault.PrincipalMarkerAddress().
//   - The vault account’s own balances are treated as *reserves* and are not included here.
//
// Computation:
//   - Iterate all non-share-denom balances at the marker (principal) account.
//   - Convert each balance to underlying units via ToUnderlyingAssetAmount.
//   - Sum the converted amounts (floor at each multiplication/division step).
func (k Keeper) GetTVVInUnderlyingAsset(ctx sdk.Context, vault types.VaultAccount) (math.Int, error) {
	balances := k.BankKeeper.GetAllBalances(ctx, vault.PrincipalMarkerAddress())
	total := math.ZeroInt()
	for _, balance := range balances {
		if balance.Denom == vault.ShareDenom {
			continue
		}
		val, err := k.ToUnderlyingAssetAmount(ctx, vault, balance)
		if err != nil {
			return math.Int{}, err
		}
		total = total.Add(val)
	}
	return total, nil
}

// GetNAVPerShareInUnderlyingAsset returns the floor NAV-per-share in units of
// vault.UnderlyingAsset.
//
// Computation:
//   - TVV(underlying) is obtained from GetTVVInUnderlyingAsset (principal/marker balances only).
//   - totalShareSupply is fetched from BankKeeper.GetSupply(vault.ShareDenom) (the live supply).
//   - If total shares == 0, returns 0. Otherwise returns TVV / totalShareSupply (floor).
func (k Keeper) GetNAVPerShareInUnderlyingAsset(ctx sdk.Context, vault types.VaultAccount) (math.Int, error) {
	tvv, err := k.GetTVVInUnderlyingAsset(ctx, vault)
	if err != nil {
		return math.Int{}, err
	}
	totalShares := k.BankKeeper.GetSupply(ctx, vault.ShareDenom).Amount
	if totalShares.IsZero() {
		return math.ZeroInt(), nil
	}
	return tvv.Quo(totalShares), nil
}

// ConvertDepositToSharesInUnderlyingAsset converts a deposit coin into vault shares
// using the current TVV and total share supply (both pro-rata, floor arithmetic).
//
// Steps:
//  1. Convert the deposit into underlying-asset value via ToUnderlyingAssetAmount.
//  2. Compute the share amount using
//     CalculateSharesFromAssets(value_in_underlying, TVV, totalShares)
//     where TVV is from principal (marker) balances.
//
// Returns a coin in vault.ShareDenom representing the minted shares. This function
// performs calculation only; callers mint/burn/transfer as needed. Very small deposits
// may floor to zero shares.
func (k Keeper) ConvertDepositToSharesInUnderlyingAsset(ctx sdk.Context, vault types.VaultAccount, in sdk.Coin) (sdk.Coin, error) {
	valInAsset, err := k.ToUnderlyingAssetAmount(ctx, vault, in)
	if err != nil {
		return sdk.Coin{}, err
	}
	tvv, err := k.GetTVVInUnderlyingAsset(ctx, vault)
	if err != nil {
		return sdk.Coin{}, err
	}
	totalShares := k.BankKeeper.GetSupply(ctx, vault.ShareDenom).Amount
	return utils.CalculateSharesFromAssets(valInAsset, tvv, totalShares, vault.ShareDenom)
}

// ConvertSharesToRedeemCoin converts a share amount into a payout coin in redeemDenom
// using the current TVV and total share supply (both pro-rata, floor arithmetic).
//
// Steps:
//  1. Convert shares → underlying via
//     CalculateAssetsFromShares(shares, totalShares, TVV)
//     where TVV is from principal (marker) balances.
//  2. Convert the resulting underlying amount to redeemDenom via FromUnderlyingAssetAmount
//     (identity fast-path if redeemDenom == vault.UnderlyingAsset).
//
// Returns a coin in redeemDenom. This function performs calculation only; callers
// must enforce liquidity/policy. If shares <= 0, returns a zero-amount coin.
func (k Keeper) ConvertSharesToRedeemCoin(ctx sdk.Context, vault types.VaultAccount, shares math.Int, redeemDenom string) (sdk.Coin, error) {
	if !shares.IsPositive() {
		return sdk.NewCoin(redeemDenom, math.ZeroInt()), nil
	}

	tvv, err := k.GetTVVInUnderlyingAsset(ctx, vault)
	if err != nil {
		return sdk.Coin{}, err
	}
	totalShares := k.BankKeeper.GetSupply(ctx, vault.ShareDenom).Amount

	assetCoin, err := utils.CalculateAssetsFromShares(shares, totalShares, tvv, vault.UnderlyingAsset)
	if err != nil {
		return sdk.Coin{}, err
	}

	return k.FromUnderlyingAssetAmount(ctx, vault, assetCoin.Amount, redeemDenom)
}
