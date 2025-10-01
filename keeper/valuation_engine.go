package keeper

import (
	"fmt"

	"github.com/provlabs/vault/types"
	"github.com/provlabs/vault/utils"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
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
	// Currently, we are treating "uylds.fcc" as a universal stablecoin equivalent to the underlying asset.
	// This is a temporary measure until we have a more robust multi-currency support and stablecoin handling.
	// The assumption is that "uylds.fcc" is always pegged 1:1 with the underlying asset for vault valuation purposes.
	// For more information, see https://github.com/ProvLabs/vault/issues/73
	if srcDenom == underlyingAsset || underlyingAsset == "uylds.fcc" {
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
		if balance.Denom == vault.TotalShares.Denom {
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
// Paused fast-path:
//   - If vault.Paused is true, this function short-circuits and returns
//     vault.PausedBalance.Amount (ignores live TVV and share supply).
//
// Computation (when not paused):
//   - TVV(underlying) is obtained from GetTVVInUnderlyingAsset (principal/marker balances only).
//   - totalShareSupply is fetched from BankKeeper.GetSupply(vault.ShareDenom) (the live supply).
//   - If total shares == 0, returns 0. Otherwise returns TVV / totalShareSupply (floor).
func (k Keeper) GetNAVPerShareInUnderlyingAsset(ctx sdk.Context, vault types.VaultAccount) (math.Int, error) {
	if vault.Paused {
		return vault.PausedBalance.Amount, nil
	}
	tvv, err := k.GetTVVInUnderlyingAsset(ctx, vault)
	if err != nil {
		return math.Int{}, err
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
	priceNum, priceDen, err := k.UnitPriceFraction(ctx, in.Denom, vault.UnderlyingAsset)
	if err != nil {
		return sdk.Coin{}, err
	}
	tvv, err := k.GetTVVInUnderlyingAsset(ctx, vault)
	if err != nil {
		return sdk.Coin{}, err
	}
	amountNumerator := in.Amount.Mul(priceNum)
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
		return sdk.Coin{}, err
	}
	priceNum, priceDen, err := k.UnitPriceFraction(ctx, redeemDenom, vault.UnderlyingAsset)
	if err != nil {
		return sdk.Coin{}, err
	}
	if priceNum.IsZero() {
		return sdk.Coin{}, fmt.Errorf("zero price for %s/%s", redeemDenom, vault.UnderlyingAsset)
	}
	return utils.CalculateRedeemProRataFraction(shares, vault.TotalShares.Amount, tvv, priceNum, priceDen, redeemDenom)
}
