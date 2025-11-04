package keeper

import (
	"fmt"

	"github.com/provlabs/vault/types"
	"github.com/provlabs/vault/utils"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// UnitPriceFraction returns the unit price of srcDenom expressed in underlyingAsset
// as an integer fraction (numerator, denominator) using marker Net Asset Value (NAV).
//
// Semantics
//   - Forward NAV (srcDenom → underlyingAsset):
//     NAV.Price is total underlying units for NAV.Volume units of srcDenom.
//     1 srcDenom = NAV.Price.Amount / NAV.Volume underlyingAsset → (num, den) = (NAV.Price.Amount, NAV.Volume).
//   - Reverse NAV (underlyingAsset → srcDenom):
//     NAV.Price is total srcDenom units for NAV.Volume units of underlyingAsset.
//     1 srcDenom = NAV.Volume / NAV.Price.Amount underlyingAsset → (num, den) = (NAV.Volume, NAV.Price.Amount).
//
// The result is integer (floor-safe) arithmetic.
//
// Source selection
// - Identity/peg fast-paths return (1, 1):
//   - If srcDenom == underlyingAsset
//   - If underlyingAsset == "uylds.fcc" (temporary 1:1 peg; see https://github.com/ProvLabs/vault/issues/73)
//   - If vault.PaymentDenom == "uylds.fcc" (temporary 1:1 peg; see https://github.com/ProvLabs/vault/issues/73)
//
// - Otherwise read both forward and reverse NAVs:
//   - If only one exists, use it.
//   - If both exist, choose the one with the greater UpdatedBlockHeight (newest).
//
// Errors
// - If neither NAV exists, return the lookup error (if any) or "nav not found for src/underlying".
// - For the selected NAV direction:
//   - Forward: error if NAV.Volume == 0.
//   - Reverse: error if NAV.Price.Amount == 0.
//
// Returns
// - (num, den) as math.Int, suitable for floor(x * num / den).
func (k Keeper) UnitPriceFraction(ctx sdk.Context, srcDenom string, vault types.VaultAccount) (num, den math.Int, err error) {
	underlyingAsset := vault.UnderlyingAsset
	if srcDenom == underlyingAsset {
		return math.NewInt(1), math.NewInt(1), nil
	}

	// For now, if either the vault’s underlying asset or payment denom is "uylds.fcc",
	// we assume a 1:1 equivalence between the payment denom and the underlying denom.
	// See https://github.com/ProvLabs/vault/issues/73 for details.
	const uyldsFccDenom = "uylds.fcc"
	if vault.PaymentDenom == uyldsFccDenom || underlyingAsset == uyldsFccDenom {
		return math.NewInt(1), math.NewInt(1), nil
	}

	fwd, errF := k.MarkerKeeper.GetNetAssetValue(ctx, srcDenom, underlyingAsset)
	rev, errR := k.MarkerKeeper.GetNetAssetValue(ctx, underlyingAsset, srcDenom)

	if fwd == nil && rev == nil {
		if errF != nil {
			return math.Int{}, math.Int{}, errF
		}
		if errR != nil {
			return math.Int{}, math.Int{}, errR
		}
		return math.Int{}, math.Int{}, fmt.Errorf("nav not found for %s/%s", srcDenom, underlyingAsset)
	}

	useForward := false
	switch {
	case fwd != nil && rev == nil:
		useForward = true
	case fwd == nil && rev != nil:
		useForward = false
	default:
		useForward = fwd.UpdatedBlockHeight >= rev.UpdatedBlockHeight
	}

	if useForward {
		if fwd.Volume == 0 {
			return math.Int{}, math.Int{}, fmt.Errorf("nav volume is zero for %s/%s", srcDenom, underlyingAsset)
		}
		priceAmt := fwd.Price.Amount
		volAmt := math.NewIntFromUint64(fwd.Volume)
		return priceAmt, volAmt, nil
	}

	if rev.Price.Amount.IsZero() {
		return math.Int{}, math.Int{}, fmt.Errorf("nav price is zero for %s/%s", underlyingAsset, srcDenom)
	}
	priceAmt := math.NewIntFromUint64(rev.Volume)
	volAmt := rev.Price.Amount
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
	priceAmount, volume, err := k.UnitPriceFraction(ctx, in.Denom, vault)
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
	priceNum, priceDen, err := k.UnitPriceFraction(ctx, in.Denom, vault)
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
	priceNum, priceDen, err := k.UnitPriceFraction(ctx, redeemDenom, vault)
	if err != nil {
		return sdk.Coin{}, err
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
		return sdk.Coin{}, fmt.Errorf("get tvv: %w", err)
	}
	estAmt, err := k.CalculateVaultTotalAssets(ctx, vault, sdk.Coin{Denom: vault.UnderlyingAsset, Amount: baseAmt})
	if err != nil {
		return sdk.Coin{}, fmt.Errorf("estimate tvv: %w", err)
	}
	return sdk.Coin{Denom: vault.UnderlyingAsset, Amount: estAmt}, nil
}
