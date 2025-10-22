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

// CalculateSharesProRataFraction computes minted shares for a deposit using a
// single-floor, pro-rata formula with virtual offsets and an explicit price fraction.
//
// Arguments:
//   - amountNumerator / amountDenominator represent the deposit value expressed
//     in underlying units as a rational (e.g., amount * priceNum / priceDen).
//     For underlying deposits, pass (amount, 1).
//   - totalAssets / totalShares are the vault-wide totals prior to the deposit.
//   - shareDenom is the vault share denom for the resulting coin.
//
// Behavior:
//   - Applies virtual offsets: ta' = totalAssets + VirtualAssets, ts' = totalShares + VirtualShares.
//   - First deposit mints amount * ShareScalar / amountDenominator.
//   - Otherwise: shares = floor( amountNumerator * ts' / (amountDenominator * ta') ).
//
// Errors if any input is negative or if amountDenominator == 0.
func CalculateSharesProRataFraction(
	amountNumerator math.Int,
	amountDenominator math.Int,
	totalAssets math.Int,
	totalShares math.Int,
	shareDenom string,
) (sdk.Coin, error) {
	if amountNumerator.IsNegative() || amountDenominator.IsNegative() || totalAssets.IsNegative() || totalShares.IsNegative() {
		return sdk.Coin{}, fmt.Errorf("invalid input: negative values not allowed")
	}
	if amountNumerator.IsZero() {
		return sdk.NewCoin(shareDenom, math.ZeroInt()), nil
	}
	if amountDenominator.IsZero() {
		return sdk.Coin{}, fmt.Errorf("invalid input: zero denominator")
	}
	if totalAssets.IsZero() {
		shares := amountNumerator.Mul(ShareScalar).Quo(amountDenominator)
		return sdk.NewCoin(shareDenom, shares), nil
	}
	ta := totalAssets.Add(VirtualAssets)
	ts := totalShares.Add(VirtualShares)
	den := amountDenominator.Mul(ta)
	shares := amountNumerator.Mul(ts).Quo(den)
	return sdk.NewCoin(shareDenom, shares), nil
}

// CalculateRedeemProRataFraction computes the payout amount for redeeming shares
// into an arbitrary payout denom using a single-floor, pro-rata formula with
// virtual offsets and an explicit price fraction.
//
// Arguments:
//   - shares is the number of vault shares being redeemed.
//   - totalShares / totalAssets are the vault-wide totals prior to redemption.
//   - priceNumerator / priceDenominator encode the payout denom’s price in
//     underlying units (1 payout = priceNum / priceDen underlying). For
//     underlying payouts, pass (1, 1).
//   - payoutDenom is the target payout denom for the resulting coin.
//
// Behavior:
//   - Applies virtual offsets: ta' = totalAssets + VirtualAssets, ts' = totalShares + VirtualShares.
//   - Computes: payout = floor( shares * ta' * priceDen / (ts' * priceNum) ).
//   - Returns sdk.Coin(payoutDenom, payout).
//
// Errors if any input is negative or if priceNumerator == 0.
func CalculateRedeemProRataFraction(shares math.Int, totalShares math.Int, totalAssets math.Int, priceNumerator math.Int, priceDenominator math.Int, payoutDenom string) (sdk.Coin, error) {
	if shares.IsNegative() || totalShares.IsNegative() || totalAssets.IsNegative() || priceNumerator.IsNegative() || priceDenominator.IsNegative() {
		return sdk.Coin{}, fmt.Errorf("invalid input: negative values not allowed")
	}
	if shares.IsZero() {
		return sdk.NewCoin(payoutDenom, math.ZeroInt()), nil
	}
	ts := totalShares.Add(VirtualShares)
	if ts.IsZero() {
		return sdk.NewCoin(payoutDenom, math.ZeroInt()), nil
	}
	if priceNumerator.IsZero() {
		return sdk.Coin{}, fmt.Errorf("invalid input: zero price numerator")
	}
	ta := totalAssets.Add(VirtualAssets)
	num := shares.Mul(ta).Mul(priceDenominator)
	den := ts.Mul(priceNumerator)
	out := num.Quo(den)
	return sdk.NewCoin(payoutDenom, out), nil
}

