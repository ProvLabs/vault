package utils

import (
	"fmt"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// CalculateSharesFromAssets returns the number of shares that correspond
// to a given amount of deposited assets.
//
// If totalAssets is zero (i.e. first deposit), it returns the same amount
// of shares as assets (1:1 ratio). Otherwise:
//
//	shares = (assets * totalShares) / totalAssets
func CalculateSharesFromAssets(
	assets math.Int,
	totalAssets math.Int,
	totalShares math.Int,
	shareDenom string,
) (sdk.Coin, error) {
	if assets.IsNegative() || totalAssets.IsNegative() || totalShares.IsNegative() {
		return sdk.Coin{}, fmt.Errorf("invalid input: negative values not allowed")
	}

	if totalAssets.IsZero() {
		return sdk.NewCoin(shareDenom, assets), nil
	}

	sharesOut := assets.Mul(totalShares).Quo(totalAssets)
	return sdk.NewCoin(shareDenom, sharesOut), nil
}

// CalculateAssetsFromShares returns the amount of assets that correspond
// to a given number of shares being redeemed.
//
// If totalShares is zero, it returns zero assets.
//
//	assets = (shares * totalAssets) / totalShares

func CalculateAssetsFromShares(
	shares math.Int,
	totalShares math.Int,
	totalAssets math.Int,
	assetDenom string,
) (sdk.Coin, error) {
	if shares.IsNegative() || totalShares.IsNegative() || totalAssets.IsNegative() {
		return sdk.Coin{}, fmt.Errorf("invalid input: negative values not allowed")
	}

	if totalShares.IsZero() || shares.IsZero() {
		return sdk.NewCoin(assetDenom, math.ZeroInt()), nil
	}

	assetsOut := shares.Mul(totalAssets).Quo(totalShares)
	return sdk.NewCoin(assetDenom, assetsOut), nil
}
