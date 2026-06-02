package keeper

import (
	"errors"
	"fmt"

	"github.com/provlabs/vault/types"
	"github.com/provlabs/vault/utils"

	"cosmossdk.io/collections"
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ErrInternalNAVNotFound is returned by UnitPriceFraction (and any helper that
// composes it) when no Internal NAV entry exists for the requested denom on the
// target vault. Callers should match this with errors.Is to classify the
// failure (e.g. swap refund classification in getRefundReason) without
// relying on the formatted error string.
var ErrInternalNAVNotFound = errors.New("internal NAV entry not found")

// UnitPriceFraction returns the unit price of srcDenom expressed in the vault's
// underlying asset as an integer fraction (numerator, denominator), sourced
// exclusively from the per-vault Internal NAV table.
//
// # Semantics
//
// The Internal NAV entry for a denom records the price of `volume` units of the
// denom denominated in the vault's underlying asset:
//
//	1 srcDenom = nav.Price.Amount / nav.Volume underlying
//	(num, den) = (nav.Price.Amount, nav.Volume)
//
// Suitable for floor(x * num / den) integer arithmetic.
//
// Identity fast-path
//   - If srcDenom == vault.UnderlyingAsset, returns (1, 1) without a lookup.
//     This also covers single-denom vaults where payment_denom == underlying.
//
// Errors
//   - Wraps ErrInternalNAVNotFound when no entry exists for srcDenom on this
//     vault. Callers should classify with errors.Is(err, ErrInternalNAVNotFound)
//     rather than matching on the formatted error string.
//   - Returns wrapped errors for any other Internal NAV lookup failure.
//   - Defensive: rejects nav.Volume <= 0 or nav.Price.Amount <= 0 (these are
//     already enforced at NAV-write time by validateVaultNAVFields).
func (k Keeper) UnitPriceFraction(ctx sdk.Context, srcDenom string, vault types.VaultAccount) (num, den math.Int, err error) {
	if srcDenom == vault.UnderlyingAsset {
		return math.NewInt(1), math.NewInt(1), nil
	}

	nav, err := k.GetVaultNAV(ctx, vault.GetAddress(), srcDenom)
	if err != nil {
		if errors.Is(err, collections.ErrNotFound) {
			return math.Int{}, math.Int{}, fmt.Errorf("no internal NAV entry for denom %q on vault %s: %w", srcDenom, vault.GetAddress(), ErrInternalNAVNotFound)
		}
		return math.Int{}, math.Int{}, fmt.Errorf("failed to get internal NAV for denom %q on vault %s: %w", srcDenom, vault.GetAddress(), err)
	}

	if nav.Volume.IsNil() || !nav.Volume.IsPositive() {
		volumeForLog := "<nil>"
		if !nav.Volume.IsNil() {
			volumeForLog = nav.Volume.String()
		}
		k.getLogger(ctx).Error("internal NAV invariant violated: non-positive volume",
			"vault", vault.GetAddress().String(),
			"denom", srcDenom,
			"volume", volumeForLog,
		)
		return math.Int{}, math.Int{}, fmt.Errorf("internal NAV volume must be positive for denom %q on vault %s", srcDenom, vault.GetAddress())
	}
	if nav.Price.Amount.IsNil() || !nav.Price.Amount.IsPositive() {
		priceForLog := "<nil>"
		if !nav.Price.Amount.IsNil() {
			priceForLog = nav.Price.String()
		}
		k.getLogger(ctx).Error("internal NAV invariant violated: non-positive price",
			"vault", vault.GetAddress().String(),
			"denom", srcDenom,
			"price", priceForLog,
		)
		return math.Int{}, math.Int{}, fmt.Errorf("internal NAV price must be positive for denom %q on vault %s", srcDenom, vault.GetAddress())
	}

	return nav.Price.Amount, nav.Volume, nil
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
	priceAmount, volume, err := k.UnitPriceFraction(ctx, in.Denom, vault)
	if err != nil {
		return math.Int{}, fmt.Errorf("failed to get unit price fraction: %w", err)
	}
	product, err := in.Amount.SafeMul(priceAmount)
	if err != nil {
		return math.Int{}, fmt.Errorf("failed to multiply amount %s by price %s: %w", in.Amount, priceAmount, err)
	}
	return product.Quo(volume), nil
}

// FromUnderlyingAssetAmount converts an amount of vault.UnderlyingAsset into
// the equivalent value in targetDenom using integer floor arithmetic.
//
// Formula:
//
//	value_in_target = inAmount * priceDenominator / priceNumerator
//
// where (priceNumerator, priceDenominator) are from UnitPriceFraction(targetDenom → underlying).
func (k Keeper) FromUnderlyingAssetAmount(ctx sdk.Context, vault types.VaultAccount, inAmount math.Int, targetDenom string) (math.Int, error) {
	priceNum, priceDen, err := k.UnitPriceFraction(ctx, targetDenom, vault)
	if err != nil {
		return math.Int{}, fmt.Errorf("failed to get unit price fraction: %w", err)
	}
	if priceNum.IsZero() {
		return math.Int{}, fmt.Errorf("zero price for %s/%s", targetDenom, vault.UnderlyingAsset)
	}
	product, err := inAmount.SafeMul(priceDen)
	if err != nil {
		return math.Int{}, fmt.Errorf("failed to multiply amount %s by price denominator %s: %w", inAmount, priceDen, err)
	}
	return product.Quo(priceNum), nil
}

// GetTVVInUnderlyingAsset returns the Total Vault Value (TVV) expressed in
// vault.UnderlyingAsset using floor arithmetic.
//
// Paused fast-path:
//   - If vault.Paused is true, this function short-circuits and returns
//     vault.PausedBalance.Amount (no balance iteration or NAV conversion).
//
// Source of truth (when not paused):
//   - TVV sums the balances held at the vault’s *principal* account, i.e. the marker
//     address for vault.PrincipalMarkerAddress().
//   - The vault account’s own balances are treated as *reserves* and are not included here.
//
// Computation (when not paused):
//   - Iterate all non-share-denom balances at the marker (principal) account.
//   - Convert each balance to underlying units via ToUnderlyingAssetAmount.
//   - Sum the converted amounts (floor at each multiplication/division step).
func (k Keeper) GetTVVInUnderlyingAsset(ctx sdk.Context, vault types.VaultAccount) (math.Int, error) {
	if vault.Paused {
		return vault.PausedBalance.Amount, nil
	}
	balances := k.BankKeeper.GetAllBalances(ctx, vault.PrincipalMarkerAddress())
	total := math.ZeroInt()
	for _, balance := range balances {
		if balance.Denom == vault.TotalShares.Denom || !vault.IsAcceptedDenom(balance.Denom) {
			continue
		}
		val, err := k.ToUnderlyingAssetAmount(ctx, vault, balance)
		if err != nil {
			return math.Int{}, fmt.Errorf("failed to convert balance to underlying: %w", err)
		}
		total, err = total.SafeAdd(val)
		if err != nil {
			return math.Int{}, fmt.Errorf("failed to add balance %s to total vault value: %w", val, err)
		}
	}
	return total, nil
}

// GetNAVPerShareInUnderlyingAsset returns the floor NAV per share in units of
// vault.UnderlyingAsset.
//
// Paused fast-path:
//   - If vault.Paused is true, this function short-circuits and returns
//     vault.PausedBalance.Amount (ignores live TVV and share supply).
//
// Computation (when not paused):
//   - TVV(underlying) is obtained from GetTVVInUnderlyingAsset (includes current vault holdings in underlying units).
//   - totalShareSupply is taken from vault.TotalShares.Amount (the recorded share supply).
//   - If total shares == 0, returns 0. Otherwise returns TVV / totalShareSupply (floor).
func (k Keeper) GetNAVPerShareInUnderlyingAsset(ctx sdk.Context, vault types.VaultAccount) (math.Int, error) {
	tvv, err := k.GetTVVInUnderlyingAsset(ctx, vault)
	if err != nil {
		return math.Int{}, fmt.Errorf("failed to get TVV: %w", err)
	}

	if vault.TotalShares.IsZero() {
		return math.ZeroInt(), nil
	}
	return tvv.Quo(vault.TotalShares.Amount), nil
}

// ConvertSharesToRedeemCoin converts a share amount into a payout coin in redeemDenom
// using the current TVV and total share supply (pro-rata, single-floor arithmetic).
//
// Steps:
//  1. Look up the unit price fraction for redeemDenom → underlying via UnitPriceFraction.
//  2. Compute the payout in one step using
//     CalculateRedeemProRataFraction(shares, totalShares, TVV, priceNum, priceDen)
//     where TVV is from principal (marker) balances.
//
// Returns a coin in redeemDenom. This function performs calculation only; callers
// must enforce liquidity/policy. If shares <= 0, returns a zero-amount coin.
func (k Keeper) ConvertDepositToSharesInUnderlyingAsset(ctx sdk.Context, vault types.VaultAccount, in sdk.Coin) (sdk.Coin, error) {
	priceNum, priceDen, err := k.UnitPriceFraction(ctx, in.Denom, vault)
	if err != nil {
		return sdk.Coin{}, fmt.Errorf("failed to get unit price fraction: %w", err)
	}
	tvv, err := k.GetTVVInUnderlyingAsset(ctx, vault)
	if err != nil {
		return sdk.Coin{}, fmt.Errorf("failed to get TVV: %w", err)
	}
	amountNumerator, err := in.Amount.SafeMul(priceNum)
	if err != nil {
		return sdk.Coin{}, fmt.Errorf("failed to multiply amount %s by price numerator %s: %w", in.Amount, priceNum, err)
	}
	return utils.CalculateSharesProRataFraction(amountNumerator, priceDen, tvv, vault.TotalShares.Amount, vault.TotalShares.Denom)
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
		return sdk.Coin{}, fmt.Errorf("failed to get TVV: %w", err)
	}
	priceNum, priceDen, err := k.UnitPriceFraction(ctx, redeemDenom, vault)
	if err != nil {
		return sdk.Coin{}, fmt.Errorf("failed to get unit price fraction: %w", err)
	}
	if priceNum.IsZero() {
		return sdk.Coin{}, fmt.Errorf("zero price for %s/%s", redeemDenom, vault.UnderlyingAsset)
	}
	return utils.CalculateRedeemProRataFraction(shares, vault.TotalShares.Amount, tvv, priceNum, priceDen, redeemDenom)
}

// EstimateTotalVaultValue returns an estimated Total Vault Value (TVV) as a Coin
// denominated in the vault's underlying asset. It composes two steps without
// mutating state:
//  1. Reads the current principal-only TVV from on-chain balances at the
//     principal (marker) account (excludes reserves and unpaid interest).
//  2. Applies the vault's interest model to estimate unpaid interest through
//     CalculateVaultTotalAssets, producing a best-effort TVV as of the query
//     block. The result is floor-rounded and suitable for pro-rata calculations.
//
// If the vault is paused, the estimation honors the keeper’s paused logic
// inside GetTVVInUnderlyingAsset.
//
// Returns an sdk.Coin { Denom: vault.UnderlyingAsset, Amount: ... }.
func (k Keeper) EstimateTotalVaultValue(ctx sdk.Context, vault *types.VaultAccount) (sdk.Coin, error) {
	baseAmt, err := k.GetTVVInUnderlyingAsset(ctx, *vault)
	if err != nil {
		return sdk.Coin{}, fmt.Errorf("failed to get tvv: %w", err)
	}
	estAmt, err := k.CalculateVaultTotalAssets(ctx, vault, sdk.Coin{Denom: vault.UnderlyingAsset, Amount: baseAmt})
	if err != nil {
		return sdk.Coin{}, fmt.Errorf("failed to estimate tvv: %w", err)
	}
	return sdk.Coin{Denom: vault.UnderlyingAsset, Amount: estAmt}, nil
}
