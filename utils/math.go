package utils

import (
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Fixed precision parameters for vault share calculations.
//
// ShareScalar specifies how many shares correspond to 1 unit of the underlying asset.
// VirtualAssets and VirtualShares are added to the total asset/share counts
// during conversion calculations to prevent edge cases such as the initial
// deposit having outsized influence on the share price.
//
// These values are fixed and not configurable per-vault.
var (
	ShareScalar   = math.NewInt(1_000_000) // 1 asset unit = 1e6 shares
	VirtualAssets = math.NewInt(1_000_000) // offset added to total assets in calculations
	VirtualShares = math.NewInt(1_000_000) // offset added to total shares in calculations
)

// CalculateSharesFromAssets returns the number of shares that correspond
// to a given amount of deposited assets, using fixed precision and virtual offsets.
//
// The formula used is:
//
//	shares = floor( assets * ShareScalar * (totalShares + VirtualShares) / (totalAssets + VirtualAssets) )
//
// This improves on the simple (assets * totalShares / totalAssets)
// calculation by:
//   - Increasing precision via ShareScalar
//   - Adding virtual offsets to stabilize the rate when the vault is empty
//     or has very low liquidity, reducing the risk of inflation attacks
//   - Minimizing rounding loss impact for small deposits
//
// If totalAssets is zero (i.e., first deposit), it mints assets * ShareScalar
// shares directly, maintaining a neutral starting price.
//
// Returns an sdk.Coin with the share denomination and calculated amount.
// An error is returned if any provided value is negative.
func CalculateSharesFromAssets(
	assets math.Int,
	totalAssets math.Int,
	totalShares math.Int,
	shareDenom string,
) (sdk.Coin, error) {
	if assets.IsNegative() || totalAssets.IsNegative() || totalShares.IsNegative() {
		return sdk.Coin{}, fmt.Errorf("invalid input: negative values not allowed")
	}
	if assets.IsZero() {
		return sdk.NewCoin(shareDenom, math.ZeroInt()), nil
	}

	ta := totalAssets.Add(VirtualAssets)
	if ta.IsZero() {
		return sdk.NewCoin(shareDenom, assets.Mul(ShareScalar)), nil
	}

	num := assets.Mul(ShareScalar).Mul(totalShares.Add(VirtualShares))
	sharesOut := num.Quo(ta)
	return sdk.NewCoin(shareDenom, sharesOut), nil
}

// CalculateAssetsFromShares returns the amount of assets that correspond
// to a given number of shares being redeemed, using fixed precision and virtual offsets.
//
// The formula used is:
//
//	assets = floor( shares * (totalAssets + VirtualAssets) / ( (totalShares + VirtualShares) * ShareScalar ) )
//
// This improves on the simple (shares * totalAssets / totalShares)
// calculation by:
//   - Increasing precision via ShareScalar
//   - Adding virtual offsets to stabilize the rate in low-liquidity vaults
//   - Making manipulation of the share price via donations economically costly
//
// If totalShares is zero or shares is zero, it returns zero assets.
// An error is returned if any provided value is negative.
func CalculateAssetsFromShares(
	shares math.Int,
	totalShares math.Int,
	totalAssets math.Int,
	assetDenom string,
) (sdk.Coin, error) {
	if shares.IsNegative() || totalShares.IsNegative() || totalAssets.IsNegative() {
		return sdk.Coin{}, fmt.Errorf("invalid input: negative values not allowed")
	}
	if shares.IsZero() {
		return sdk.NewCoin(assetDenom, math.ZeroInt()), nil
	}

	ts := totalShares.Add(VirtualShares)
	if ts.IsZero() {
		return sdk.NewCoin(assetDenom, math.ZeroInt()), nil
	}

	num := shares.Mul(totalAssets.Add(VirtualAssets))
	assetsOut := num.Quo(ts).Quo(ShareScalar)
	return sdk.NewCoin(assetDenom, assetsOut), nil
}

// ExpDec calculates e^x using the Maclaurin series expansion up to `terms` terms.
// x must be a cosmosmath.LegacyDec. The more terms, the more accurate (and slower).
// Safe for on-chain use (fully deterministic).
//
// The expansion used is:
//
//	e^x = 1 + x + x^2/2! + x^3/3! + ... + x^n/n!
//
// Where n = terms.
//
// This function is intended for deterministic exponentiation in financial
// calculations where floating-point math is unsafe or unavailable.
func ExpDec(x math.LegacyDec, terms int) math.LegacyDec {
	result := math.LegacyOneDec()
	power := math.LegacyOneDec()
	factorial := math.LegacyOneDec()

	for i := 1; i <= terms; i++ {
		power = power.Mul(x)
		factorial = factorial.MulInt64(int64(i))
		term := power.Quo(factorial)
		result = result.Add(term)
	}

	return result
}
