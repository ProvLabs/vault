package keeper

import (
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/provlabs/vault/types"
	"github.com/provlabs/vault/utils"
)

// UnitPriceFraction returns a ratio (num, den) = underlyingAsset_units per 1 srcDenom,
// derived from NAV where NAV.Price is the value of NAV.Volume srcDenom in underlyingAsset.
// Special-case identity: srcDenom == underlyingAsset => 1/1.
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

// ToAssetAmount converts an arbitrary coin into its value expressed in underlyingAsset,
// using integer math with floor: amt * (price/volume).
func (k Keeper) ToAssetAmount(ctx sdk.Context, vault types.VaultAccount, in sdk.Coin) (math.Int, error) {
	priceAmount, volume, err := k.UnitPriceFraction(ctx, in.Denom, vault.UnderlyingAsset)
	if err != nil {
		return math.Int{}, err
	}
	return in.Amount.Mul(priceAmount).Quo(volume), nil
}

// GetTVVInAsset sums all pool balances (excluding the share denom) into underlyingAsset.
func (k Keeper) GetTVVInAsset(ctx sdk.Context, vault types.VaultAccount) (math.Int, error) {
	balances := k.BankKeeper.GetAllBalances(ctx, vault.GetAddress())
	total := math.ZeroInt()
	for _, balance := range balances {
		if balance.Denom == vault.ShareDenom {
			continue
		}
		val, err := k.ToAssetAmount(ctx, vault, balance)
		if err != nil {
			return math.Int{}, err
		}
		total = total.Add(val)
	}
	return total, nil
}

// GetNAVPerShareInAsset = TVV(asset) / totalShares (floored).
func (k Keeper) GetNAVPerShareInAsset(ctx sdk.Context, v types.VaultAccount) (math.Int, error) {
	tvv, err := k.GetTVVInAsset(ctx, v)
	if err != nil {
		return math.Int{}, err
	}
	totalShares := k.BankKeeper.GetSupply(ctx, v.ShareDenom).Amount
	if totalShares.IsZero() {
		return math.ZeroInt(), nil
	}
	return tvv.Quo(totalShares), nil
}

// ConvertDepositToSharesInAsset: value deposit in underlyingAsset, then apply your share math.
func (k Keeper) ConvertDepositToSharesInAsset(ctx sdk.Context, v types.VaultAccount, in sdk.Coin) (sdk.Coin, error) {
	valInAsset, err := k.ToAssetAmount(ctx, v, in)
	if err != nil {
		return sdk.Coin{}, err
	}
	tvv, err := k.GetTVVInAsset(ctx, v)
	if err != nil {
		return sdk.Coin{}, err
	}
	totalShares := k.BankKeeper.GetSupply(ctx, v.ShareDenom).Amount
	return utils.CalculateSharesFromAssets(valInAsset, tvv, totalShares, v.ShareDenom)
}

// ConvertSharesToRedeemCoinInAsset: shares -> asset amount -> redeem denom amount.
// If redeemDenom == underlyingAsset, return asset amount directly.
// Otherwise use reciprocal of unitPriceFraction: out = assetAmt * den / num.
func (k Keeper) ConvertSharesToRedeemCoinInAsset(ctx sdk.Context, v types.VaultAccount, shares math.Int, redeemDenom string) (sdk.Coin, error) {
	if !shares.IsPositive() {
		return sdk.NewCoin(redeemDenom, math.ZeroInt()), nil
	}

	tvv, err := k.GetTVVInAsset(ctx, v)
	if err != nil {
		return sdk.Coin{}, err
	}
	totalShares := k.BankKeeper.GetSupply(ctx, v.ShareDenom).Amount

	assetCoin, err := utils.CalculateAssetsFromShares(shares, totalShares, tvv, v.UnderlyingAsset)
	if err != nil {
		return sdk.Coin{}, err
	}

	if redeemDenom == v.UnderlyingAsset {
		return sdk.NewCoin(redeemDenom, assetCoin.Amount), nil
	}

	priceAmount, volume, err := k.UnitPriceFraction(ctx, redeemDenom, v.UnderlyingAsset)
	if err != nil {
		return sdk.Coin{}, err
	}
	if priceAmount.IsZero() {
		return sdk.Coin{}, fmt.Errorf("zero price for %s/%s", redeemDenom, v.UnderlyingAsset)
	}
	out := assetCoin.Amount.Mul(volume).Quo(priceAmount)
	return sdk.NewCoin(redeemDenom, out), nil
}
