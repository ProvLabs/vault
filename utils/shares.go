package utils

import (
	"fmt"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Fixed precision / virtual-offset parameters.
//
// ShareScalar sets the "neutral" precision: 1 asset unit -> ShareScalar shares.
// VirtualAssets and VirtualShares are added to totals in all conversions to
// harden against first-depositor inflation/rounding attacks.
//
// IMPORTANT:
// - VirtualAssets is ONE base unit of the underlying asset.
// - VirtualShares equals ShareScalar.
// - Do NOT divide by ShareScalar in redemption: the share supply is already scaled.
var (
	ShareScalar   = math.NewInt(1_000_000) // 1 asset = 1e6 shares (neutral precision target)
	VirtualAssets = math.NewInt(1)         // 1 base unit of underlying
	VirtualShares = ShareScalar
)

// CalculateSharesFromAssets returns the number of shares that correspond
// to a given amount of deposited assets, using fixed virtual offsets.
//
// Formula (integer, floor):
//
//	let ta' = totalAssets + VirtualAssets
//	let ts' = totalShares + VirtualShares
//	if totalAssets == 0:
//	    shares = assets * ShareScalar
//	else:
//	    shares = floor( assets * ts' / ta' )
//
// Rationale:
//   - First deposit mints assets * ShareScalar (neutral start).
//   - For subsequent deposits, using ts'/ta' embeds the virtual offsets
//     (security) and preserves precision without double-scaling.
//
// Returns sdk.Coin(shareDenom, shares). Error if any input is negative.
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
	ts := totalShares.Add(VirtualShares)

	// Neutral first deposit: mint assets * ShareScalar
	if totalAssets.IsZero() {
		return sdk.NewCoin(shareDenom, assets.Mul(ShareScalar)), nil
	}

	sharesOut := assets.Mul(ts).Quo(ta)
	return sdk.NewCoin(shareDenom, sharesOut), nil
}

// CalculateAssetsFromShares returns the amount of assets that correspond
// to a given number of shares being redeemed, using fixed virtual offsets.
//
// Formula (integer, floor):
//
//	let ta' = totalAssets + VirtualAssets
//	let ts' = totalShares + VirtualShares
//	if totalShares == 0:
//	    assets = 0
//	else:
//	    assets = floor( shares * ta' / ts' )
//
// Rationale:
//   - Do NOT divide by ShareScalar here. The share supply (ts') is already in
//     ShareScalar units, so the ratio ta'/ts' converts shares directly to assets.
//   - This keeps an immediate deposit->redeem round-trip ~neutral (minus floor).
//
// Returns sdk.Coin(assetDenom, assets). Error if any input is negative.
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

	ta := totalAssets.Add(VirtualAssets)
	assetsOut := shares.Mul(ta).Quo(ts)
	return sdk.NewCoin(assetDenom, assetsOut), nil
}
