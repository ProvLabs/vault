package keeper

import (
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/provlabs/vault/types"
	"github.com/provlabs/vault/utils"
)

// UnitPriceFraction returns a price ratio (priceNumerator, priceDenominator) that expresses
// how many units of underlyingAsset are worth one unit of srcDenom, derived from the NAV.
//
// Semantics:
//   - NAV.Price is the total value (in underlyingAsset) for NAV.Volume units of srcDenom.
//   - Therefore, unit price = NAV.Price.Amount / NAV.Volume (underlying per 1 src).
//   - This function returns that unit price as an integer fraction (num, den).
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

// ToUnderlyingAssetAmount converts an input coin into its value expressed in vault.UnderlyingAsset,
// using integer floor arithmetic:
//
//	value_in_underlying = in.Amount * priceNumerator / priceDenominator
//
// where (priceNumerator, priceDenominator) come from UnitPriceFraction(in.Denom → underlying).
func (k Keeper) ToUnderlyingAssetAmount(ctx sdk.Context, vault types.VaultAccount, in sdk.Coin) (math.Int, error) {
	priceAmount, volume, err := k.UnitPriceFraction(ctx, in.Denom, vault.UnderlyingAsset)
	if err != nil {
		return math.Int{}, err
	}
	return in.Amount.Mul(priceAmount).Quo(volume), nil
}

// GetTVVInUnderlyingAsset returns the Total Vault Value (TVV) expressed in vault.UnderlyingAsset.
// It sums all balances at the vault address (excluding share_denom) after converting each
// balance into underlying units via ToAssetAmount. Result is floored integer units.
func (k Keeper) GetTVVInUnderlyingAsset(ctx sdk.Context, vault types.VaultAccount) (math.Int, error) {
	balances := k.BankKeeper.GetAllBalances(ctx, vault.GetAddress())
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

// GetNAVPerShareInUnderlyingAsset returns the floor NAV-per-share in units of vault.UnderlyingAsset.
// Computation: NAV_per_share = TVV(underlying) / totalShareSupply. If total shares == 0,
// returns 0. All values are integer and use floor division.
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

// ConvertDepositToSharesInUnderlyingAsset converts a deposit coin into vault shares.
// Steps:
//  1. Convert the deposit into underlying-asset value via ToAssetAmount.
//  2. Compute shares using CalculateSharesFromAssets(value_in_underlying, TVV, totalShares).
//
// The returned coin is the minted shares in vault.ShareDenom.
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

// ConvertSharesToRedeemCoin converts a share amount into a payout coin in redeemDenom.
// Steps:
//  1. Convert shares → underlying amount via CalculateAssetsFromShares(shares, totalShares, TVV).
//  2. If redeemDenom == underlying, return that underlying amount.
//  3. Otherwise convert underlying → redeemDenom using the reciprocal of UnitPriceFraction:
//     amount_redeem = amount_underlying * denominator / numerator
//
// All arithmetic is integer with floor. If shares <= 0, returns a zero-amount coin.
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

	if redeemDenom == vault.UnderlyingAsset {
		return sdk.NewCoin(redeemDenom, assetCoin.Amount), nil
	}

	priceAmount, volume, err := k.UnitPriceFraction(ctx, redeemDenom, vault.UnderlyingAsset)
	if err != nil {
		return sdk.Coin{}, err
	}
	if priceAmount.IsZero() {
		return sdk.Coin{}, fmt.Errorf("zero price for %s/%s", redeemDenom, vault.UnderlyingAsset)
	}
	redeemAmount := assetCoin.Amount.Mul(volume).Quo(priceAmount)
	return sdk.NewCoin(redeemDenom, redeemAmount), nil
}
