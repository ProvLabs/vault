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
	if totalAssets.IsZero() {
		return sdk.NewCoin(shareDenom, assets), nil // First deposit: 1:1 mapping
	}

	sharesOut := assets.Mul(totalShares).Quo(totalAssets)
	return sdk.NewCoin(shareDenom, sharesOut), nil
}

// CalculateAssetsFromShares returns the amount of assets that correspond
// to a given number of shares being redeemed.
//
// If totalShares is zero, it returns an error to avoid division by zero.
//
//	assets = (shares * totalAssets) / totalShares
func CalculateAssetsFromShares(
	shares math.Int,
	totalShares math.Int,
	totalAssets math.Int,
	assetDenom string,
) (sdk.Coin, error) {
	if totalShares.IsZero() {
		return sdk.Coin{}, fmt.Errorf("cannot calculate assets: totalShares is zero")
	}

	assetsOut := shares.Mul(totalAssets).Quo(totalShares)
	return sdk.NewCoin(assetDenom, assetsOut), nil
}

// ExpDec calculates e^x using Maclaurin series expansion up to `terms` terms.
// x must be an cosmosmath.LegacyDec. The more terms, the more accurate (and slower).
// Safe for on-chain use (fully deterministic).
func ExpDec(x math.LegacyDec, terms int) math.LegacyDec {
	result := math.LegacyOneDec()    // starts at 1
	power := math.LegacyOneDec()     // x^0
	factorial := math.LegacyOneDec() // 0! = 1

	for i := 1; i <= terms; i++ {
		power = power.Mul(x)                     // x^i
		factorial = factorial.MulInt64(int64(i)) // i!
		term := power.Quo(factorial)             // x^i / i!
		result = result.Add(term)
	}

	return result
}
