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

// CalculateSharesProRata computes minted shares for a deposit of the vault's
// underlying asset using a single-floor, pro-rata formula with virtual offsets.
// Vaults are single-denom, so the deposit amount is already in underlying units
// and no price conversion applies.
//
// Arguments:
//   - amount is the deposit amount in underlying units.
//   - totalAssets / totalShares are the vault-wide totals prior to the deposit.
//   - shareDenom is the vault share denom for the resulting coin.
//
// Behavior:
//   - Applies virtual offsets: ta' = totalAssets + VirtualAssets, ts' = totalShares + VirtualShares.
//   - First deposit mints amount * ShareScalar.
//   - Otherwise: shares = floor( amount * ts' / ta' ).
//
// Errors if any input is negative.
func CalculateSharesProRata(
	amount math.Int,
	totalAssets math.Int,
	totalShares math.Int,
	shareDenom string,
) (sdk.Coin, error) {
	if amount.IsNegative() || totalAssets.IsNegative() || totalShares.IsNegative() {
		return sdk.Coin{}, fmt.Errorf("invalid input: negative values not allowed")
	}
	if amount.IsZero() {
		return sdk.NewCoin(shareDenom, math.ZeroInt()), nil
	}
	if totalAssets.IsZero() && totalShares.IsZero() {
		scaled, err := amount.SafeMul(ShareScalar)
		if err != nil {
			return sdk.Coin{}, fmt.Errorf("failed to multiply amount %s by share scalar %s: %w", amount, ShareScalar, err)
		}
		return sdk.NewCoin(shareDenom, scaled), nil
	}
	ta := totalAssets.Add(VirtualAssets)
	ts := totalShares.Add(VirtualShares)
	numerator, err := amount.SafeMul(ts)
	if err != nil {
		return sdk.Coin{}, fmt.Errorf("failed to multiply amount %s by total shares %s: %w", amount, ts, err)
	}
	return sdk.NewCoin(shareDenom, numerator.Quo(ta)), nil
}

// CalculateRedeemProRata computes the payout amount for redeeming shares into
// the vault's underlying asset using a single-floor, pro-rata formula with
// virtual offsets. Vaults are single-denom, so the payout is always in
// underlying units and no price conversion applies.
//
// Arguments:
//   - shares is the number of vault shares being redeemed.
//   - totalShares / totalAssets are the vault-wide totals prior to redemption.
//   - payoutDenom is the underlying asset denom for the resulting coin.
//
// Behavior:
//   - Applies virtual offsets: ta' = totalAssets + VirtualAssets, ts' = totalShares + VirtualShares.
//   - Computes: payout = floor( shares * ta' / ts' ).
//   - Returns sdk.Coin(payoutDenom, payout).
//
// Errors if any input is negative.
func CalculateRedeemProRata(shares math.Int, totalShares math.Int, totalAssets math.Int, payoutDenom string) (sdk.Coin, error) {
	if shares.IsNegative() || totalShares.IsNegative() || totalAssets.IsNegative() {
		return sdk.Coin{}, fmt.Errorf("invalid input: negative values not allowed")
	}
	if shares.IsZero() {
		return sdk.NewCoin(payoutDenom, math.ZeroInt()), nil
	}
	ts := totalShares.Add(VirtualShares)
	ta := totalAssets.Add(VirtualAssets)
	sharesTa, err := shares.SafeMul(ta)
	if err != nil {
		return sdk.Coin{}, fmt.Errorf("failed to multiply shares %s by total assets %s: %w", shares, ta, err)
	}
	return sdk.NewCoin(payoutDenom, sharesTa.Quo(ts)), nil
}
