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

// ErrInternalNAVPriceCycle is returned by UnitPriceFraction when a vault's
// Internal NAV table chains a denom's price back onto a denom already being
// resolved on the same path (a self-price or a longer loop). SetVaultNAV's
// accepted-denom validation makes this unreachable for normally-written state,
// but the engine detects it defensively so a NAV seeded outside that path (e.g.
// by a migration or a direct write) can never drive unbounded recursion.
var ErrInternalNAVPriceCycle = errors.New("internal NAV price chain contains a cycle")

// UnitPriceFraction returns the unit price of srcDenom expressed in the vault's
// underlying asset as an integer fraction (numerator, denominator), sourced
// exclusively from the per-vault Internal NAV table.
//
// # Semantics
//
// The Internal NAV entry for a denom records the price of `volume` units of the
// denom denominated in the vault's underlying asset (held assets acquired via
// AcceptAsset settlement are priced this way):
//
//	1 srcDenom = nav.Price.Amount / nav.Volume nav.Price.Denom
//
// When nav.Price.Denom is the underlying asset, the returned fraction is simply
// (nav.Price.Amount, nav.Volume). Should an entry's price denom chain onto
// another priced denom (possible only for state written outside SetVaultNAV's
// validation, e.g. by a migration or a direct write), the walk continues until
// the underlying is reached, and the fraction is the product of every entry's
// price over the product of every entry's volume along the chain
// srcDenom -> ... -> underlying:
//
//	1 srcDenom = (price_0 * price_1 * …) / (volume_0 * volume_1 * …) underlying
//
// Suitable for floor(x * num / den) integer arithmetic.
//
// Identity fast-path
//   - If srcDenom == vault.UnderlyingAsset, returns (1, 1) without a lookup.
//
// Errors
//   - Wraps ErrInternalNAVNotFound when no entry exists for srcDenom on this
//     vault. Callers should classify with errors.Is(err, ErrInternalNAVNotFound)
//     rather than matching on the formatted error string.
//   - Returns wrapped errors for any other Internal NAV lookup failure.
//   - Defensive: rejects nav.Volume <= 0 or a negative nav.Price.Amount (these
//     are already enforced at NAV-write time by validateVaultNAVFields). A zero
//     price is permitted (a held asset written down to zero) and yields a zero
//     unit price.
//   - Wraps ErrInternalNAVPriceCycle if the price chain ever revisits a denom
//     already seen on the walk (a self-price or a longer loop). Walking a finite,
//     non-repeating set of denoms is the hard termination guarantee.
//
// The value is the product of every entry's price over the product of every
// entry's volume along the chain srcDenom -> ... -> underlying. The loop walks
// that chain, accumulating (num, den), and stops at the underlying. The visited
// set bounds the walk to the number of distinct denoms regardless of how the NAV
// table was seeded, so it terminates even for state written outside SetVaultNAV's
// accepted-denom validation. Under that validation real chains are a single hop
// (srcDenom -> underlying).
func (k Keeper) UnitPriceFraction(ctx sdk.Context, srcDenom string, vault types.VaultAccount) (math.Int, math.Int, error) {
	num, den := math.NewInt(1), math.NewInt(1)
	visited := make(map[string]struct{})

	for denom := srcDenom; denom != vault.UnderlyingAsset; {
		if _, seen := visited[denom]; seen {
			return math.Int{}, math.Int{}, fmt.Errorf("price chain revisits denom %q on vault %s: %w", denom, vault.GetAddress(), ErrInternalNAVPriceCycle)
		}
		visited[denom] = struct{}{}

		nav, err := k.GetVaultNAV(ctx, vault.GetAddress(), denom)
		if err != nil {
			if errors.Is(err, collections.ErrNotFound) {
				return math.Int{}, math.Int{}, fmt.Errorf("no internal NAV entry for denom %q on vault %s: %w", denom, vault.GetAddress(), ErrInternalNAVNotFound)
			}
			return math.Int{}, math.Int{}, fmt.Errorf("failed to get internal NAV for denom %q on vault %s: %w", denom, vault.GetAddress(), err)
		}

		if nav.Volume.IsNil() || !nav.Volume.IsPositive() {
			volumeForLog := "<nil>"
			if !nav.Volume.IsNil() {
				volumeForLog = nav.Volume.String()
			}
			k.getLogger(ctx).Error("internal NAV invariant violated: non-positive volume",
				"vault", vault.GetAddress().String(),
				"denom", denom,
				"volume", volumeForLog,
			)
			return math.Int{}, math.Int{}, fmt.Errorf("internal NAV volume must be positive for denom %q on vault %s", denom, vault.GetAddress())
		}
		if nav.Price.Amount.IsNil() || nav.Price.Amount.IsNegative() {
			priceForLog := "<nil>"
			if !nav.Price.Amount.IsNil() {
				priceForLog = nav.Price.String()
			}
			k.getLogger(ctx).Error("internal NAV invariant violated: negative price",
				"vault", vault.GetAddress().String(),
				"denom", denom,
				"price", priceForLog,
			)
			return math.Int{}, math.Int{}, fmt.Errorf("internal NAV price must not be negative for denom %q on vault %s", denom, vault.GetAddress())
		}

		num, err = num.SafeMul(nav.Price.Amount)
		if err != nil {
			return math.Int{}, math.Int{}, fmt.Errorf("failed to scale price chain numerator %s by NAV price %s for denom %q: %w", num, nav.Price.Amount, denom, err)
		}
		den, err = den.SafeMul(nav.Volume)
		if err != nil {
			return math.Int{}, math.Int{}, fmt.Errorf("failed to scale price chain denominator %s by NAV volume %s for denom %q: %w", den, nav.Volume, denom, err)
		}

		denom = nav.Price.Denom
	}

	return num, den, nil
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

// GetTVV returns the gross Total Vault Value (TVV) expressed in vault.UnderlyingAsset — every
// asset the vault's principal marker holds, before the OutstandingAumFee liability is deducted.
//
// Gross and net are not interchangeable, and which one a caller wants is a policy decision:
//   - The AUM fee is assessed on gross, so the fee accrues on assets under management rather
//     than on equity that already has the fee netted out (see PerformVaultFeeTransfer).
//   - Share pricing, share-NAV publication and interest accrual use GetNetTVV, because those
//     must reflect equity actually owned by shareholders.
//   - Solvency and reserve checks use gross, since a liability owed does not change what is on
//     hand to pay out with.
//
// A paused vault returns vault.PausedBalance.Amount, which was captured net of the fee liability
// at pause time, so paused pricing stays frozen and NAV-independent.
//
// Otherwise this is a single store read of the materialized total rather than a walk of the
// vault's balances. WalkTotalValue defines the number, every path that moves a priced balance or
// changes a price reports its change, and the total-value invariant enforces that the two agree.
// A vault whose total has never been materialized has it derived and stored on first read.
//
// Because a held asset's internal NAV is set by the vault's NAV authority, repricing moves TVV and
// everything derived from it — a deliberate economic and trust surface.
func (k Keeper) GetTVV(ctx sdk.Context, vault types.VaultAccount) (math.Int, error) {
	if vault.Paused {
		return vault.PausedBalance.Amount, nil
	}
	return k.totalValue(ctx, vault)
}

// totalValue returns the materialized total, ignoring the paused snapshot. A missing entry is
// derived and stored rather than treated as an error.
func (k Keeper) totalValue(ctx sdk.Context, vault types.VaultAccount) (math.Int, error) {
	total, found, err := k.storedTotalValue(ctx, vault)
	if err != nil {
		return math.Int{}, err
	}
	if !found {
		return k.RecomputeTotalValue(ctx, vault)
	}
	return total, nil
}

// storedTotalValue reads a vault's materialized total, reporting whether an entry exists so callers
// can tell "no entry yet" apart from a stored zero.
func (k Keeper) storedTotalValue(ctx sdk.Context, vault types.VaultAccount) (math.Int, bool, error) {
	total, err := k.TotalValues.Get(ctx, vault.GetAddress())
	switch {
	case err == nil:
		return total, true, nil
	case errors.Is(err, collections.ErrNotFound):
		return math.ZeroInt(), false, nil
	default:
		return math.Int{}, false, fmt.Errorf("failed to read materialized total value for vault %s: %w", vault.GetAddress(), err)
	}
}

// RecomputeTotalValue derives a vault's total value from state, stores it, and returns it. This
// is the repair path, used by genesis import and whenever the stored total is missing or wrong.
func (k Keeper) RecomputeTotalValue(ctx sdk.Context, vault types.VaultAccount) (math.Int, error) {
	total, err := k.WalkTotalValue(ctx, vault)
	if err != nil {
		return math.Int{}, err
	}
	if err := k.TotalValues.Set(ctx, vault.GetAddress(), total); err != nil {
		return math.Int{}, fmt.Errorf("failed to store materialized total value for vault %s: %w", vault.GetAddress(), err)
	}
	return total, nil
}

// WalkTotalValue derives a vault's total value by walking its NAV table, and is the definition the
// materialized total must match. It counts only the principal marker's balances, valuing the
// underlying at identity and each priced denom at its NAV. Iterating the NAV table rather than every
// principal balance keeps the cost proportional to the denoms the vault actually prices, and
// per-denom value comes from denomValue so the walk and the incremental deltas cannot disagree.
//
// The share denom and the underlying are skipped up front. Neither entry should exist — the
// accumulator already holds the underlying, and validateVaultNAVFields rejects a NAV on the share
// denom — so this guards state that reached the table without passing that validation.
func (k Keeper) WalkTotalValue(ctx sdk.Context, vault types.VaultAccount) (math.Int, error) {
	principal := vault.PrincipalMarkerAddress()
	total := k.BankKeeper.GetBalance(ctx, principal, vault.UnderlyingAsset).Amount

	navRange := collections.NewPrefixedPairRange[sdk.AccAddress, string](vault.GetAddress())
	err := k.NAVs.Walk(ctx, navRange, func(key collections.Pair[sdk.AccAddress, string], _ types.VaultNAV) (bool, error) {
		denom := key.K2()
		if denom == vault.TotalShares.Denom || denom == vault.UnderlyingAsset {
			return false, nil
		}
		val, err := k.denomValue(ctx, vault, denom, k.BankKeeper.GetBalance(ctx, principal, denom).Amount)
		if err != nil {
			return true, err
		}
		total, err = total.SafeAdd(val)
		if err != nil {
			return true, fmt.Errorf("failed to add balance %s to total vault value: %w", val, err)
		}
		return false, nil
	})
	if err != nil {
		return math.Int{}, fmt.Errorf("failed to iterate vault NAV entries for %s: %w", vault.GetAddress(), err)
	}

	return total, nil
}

// InitTotalValue seeds a new vault's materialized total by deriving it rather than assuming zero
func (k Keeper) InitTotalValue(ctx sdk.Context, vault *types.VaultAccount) error {
	if _, err := k.RecomputeTotalValue(ctx, *vault); err != nil {
		return fmt.Errorf("failed to initialize materialized total value for vault %s: %w", vault.GetAddress(), err)
	}
	return nil
}

// HydrateTotalValues derives and stores the materialized total for every vault in the lookup,
// resolving that lookup exactly as the total-value invariant does so the two cannot disagree about
// which vaults must end up with an entry. A vault that cannot be valued is logged and skipped,
// because aborting an upgrade or a genesis import over one bad vault is worse; the invariant passes
// over that vault too, having no reference of its own to compare against.
func (k Keeper) HydrateTotalValues(ctx sdk.Context) error {
	vaults, _, err := k.resolveVaults(ctx, "materializing total values")
	if err != nil {
		return err
	}

	for _, vault := range vaults {
		total, err := k.RecomputeTotalValue(ctx, vault)
		if err != nil {
			k.getLogger(ctx).Error("skipping total value for vault that cannot be valued",
				"vault", vault.GetAddress().String(),
				"err", err,
			)
			continue
		}

		k.getLogger(ctx).Info("materialized vault total value",
			"vault", vault.GetAddress().String(),
			"total_value", total.String(),
			"denom", vault.UnderlyingAsset,
		)
	}

	return nil
}

// denomValue returns what a principal balance of denom contributes to total vault value. An
// unpriced denom contributes zero rather than erroring, matching what the walk does with it.
func (k Keeper) denomValue(ctx sdk.Context, vault types.VaultAccount, denom string, balance math.Int) (math.Int, error) {
	if balance.IsNil() || !balance.IsPositive() {
		return math.ZeroInt(), nil
	}
	if denom == vault.UnderlyingAsset {
		return balance, nil
	}
	if denom == vault.TotalShares.Denom {
		return math.ZeroInt(), nil
	}
	value, err := k.ToUnderlyingAssetAmount(ctx, vault, sdk.NewCoin(denom, balance))
	if err != nil {
		if errors.Is(err, ErrInternalNAVNotFound) {
			return math.ZeroInt(), nil
		}
		return math.Int{}, fmt.Errorf("failed to value held denom %q for vault %s: %w", denom, vault.GetAddress(), err)
	}
	return value, nil
}

// adjustTotalValue folds a signed change into a vault's materialized total. A delta that would
// drive the total negative means some path misreported, so the total is logged and rebuilt from
// state rather than carried into share pricing or failed inside a block hook. Callers must report
// after the balance has moved, so a missing entry is derived instead of adjusted: deriving already
// includes the move.
func (k Keeper) adjustTotalValue(ctx sdk.Context, vault types.VaultAccount, delta math.Int) error {
	if delta.IsNil() || delta.IsZero() {
		return nil
	}

	current, found, err := k.storedTotalValue(ctx, vault)
	if err != nil {
		return err
	}
	if !found {
		_, err = k.RecomputeTotalValue(ctx, vault)
		return err
	}

	updated, err := current.SafeAdd(delta)
	if err != nil {
		return fmt.Errorf("failed to apply value delta %s to total %s for vault %s: %w", delta, current, vault.GetAddress(), err)
	}

	if updated.IsNegative() {
		k.getLogger(ctx).Error("materialized total value went negative; rebuilding from state",
			"vault", vault.GetAddress().String(),
			"stored", current.String(),
			"delta", delta.String(),
		)
		_, err = k.RecomputeTotalValue(ctx, vault)
		return err
	}

	if err := k.TotalValues.Set(ctx, vault.GetAddress(), updated); err != nil {
		return fmt.Errorf("failed to store materialized total value for vault %s: %w", vault.GetAddress(), err)
	}
	return nil
}

// refreshDenomValue folds one denom's change into the total, given its principal balance either
// side of a transfer. Both balances are re-valued rather than valuing the amount moved, because
// the total sums per-denom floors: floor(a×p/v) + floor(b×p/v) != floor((a+b)×p/v).
func (k Keeper) refreshDenomValue(ctx sdk.Context, vault types.VaultAccount, denom string, before, after math.Int) error {
	if before.Equal(after) {
		return nil
	}
	oldValue, err := k.denomValue(ctx, vault, denom, before)
	if err != nil {
		return fmt.Errorf("failed to value denom %q at its balance before the transfer: %w", denom, err)
	}
	newValue, err := k.denomValue(ctx, vault, denom, after)
	if err != nil {
		return fmt.Errorf("failed to value denom %q at its balance after the transfer: %w", denom, err)
	}
	return k.adjustTotalValue(ctx, vault, newValue.Sub(oldValue))
}

// principalBalances snapshots the principal's balance for each denom of coins, to pair with
// refreshPrincipalValue around a transfer. It returns a slice, not a map, so the refresh applies its
// deltas in a fixed order: adjustTotalValue can bail out partway, making order observable.
func (k Keeper) principalBalances(ctx sdk.Context, vault types.VaultAccount, coins sdk.Coins) []sdk.Coin {
	principal := vault.PrincipalMarkerAddress()
	balances := make([]sdk.Coin, len(coins))
	for i, coin := range coins {
		balances[i] = sdk.NewCoin(coin.Denom, k.BankKeeper.GetBalance(ctx, principal, coin.Denom).Amount)
	}
	return balances
}

// refreshPrincipalValue folds every denom's change into the total, given the balances captured
// before the transfer.
func (k Keeper) refreshPrincipalValue(ctx sdk.Context, vault types.VaultAccount, before []sdk.Coin) error {
	principal := vault.PrincipalMarkerAddress()
	for _, prior := range before {
		after := k.BankKeeper.GetBalance(ctx, principal, prior.Denom).Amount
		if err := k.refreshDenomValue(ctx, vault, prior.Denom, prior.Amount, after); err != nil {
			return fmt.Errorf("failed to refresh value of denom %q for vault %s: %w", prior.Denom, vault.GetAddress(), err)
		}
	}
	return nil
}

// GetNetTVV returns the Total Vault Value (TVV) expressed in
// vault.UnderlyingAsset, net of the vault's OutstandingAumFee liability.
//
// This is the authoritative valuation basis for share pricing and the published share
// NAV: it represents the equity actually owned by shareholders, excluding the AUM fee
// already owed to the fee collector but not yet transferred out of the principal marker.
//
// Paused fast-path:
//   - If vault.Paused is true, this returns vault.PausedBalance.Amount directly. The paused
//     balance is captured net of the OutstandingAumFee liability at pause time, so paused
//     pricing stays frozen and NAV-independent.
//
// When not paused, GetTVV supplies the gross sum of principal-marker
// balances; this method subtracts the OutstandingAumFee (already denominated in the
// underlying asset) and floors the result at zero.
func (k Keeper) GetNetTVV(ctx sdk.Context, vault types.VaultAccount) (math.Int, error) {
	gross, err := k.GetTVV(ctx, vault)
	if err != nil {
		return math.Int{}, fmt.Errorf("failed to get gross TVV: %w", err)
	}
	if vault.Paused {
		return gross, nil
	}
	net := gross.Sub(vault.OutstandingAumFee.Amount)
	if net.IsNegative() {
		return math.ZeroInt(), nil
	}
	return net, nil
}

// GetNAVPerShare returns the floor NAV per share in units of
// vault.UnderlyingAsset.
//
// Computation:
//   - TVV(underlying) is obtained from GetNetTVV (net of the OutstandingAumFee liability).
//   - totalShareSupply is taken from vault.TotalShares.Amount (the recorded share supply).
//   - If total shares == 0, returns 0. Otherwise returns TVV / totalShareSupply (floor).
//
// For a paused vault, GetNetTVV supplies the frozen vault.PausedBalance.Amount,
// so the result is that frozen balance divided by the share supply.
func (k Keeper) GetNAVPerShare(ctx sdk.Context, vault types.VaultAccount) (math.Int, error) {
	tvv, err := k.GetNetTVV(ctx, vault)
	if err != nil {
		return math.Int{}, fmt.Errorf("failed to get TVV: %w", err)
	}

	if vault.TotalShares.IsZero() {
		return math.ZeroInt(), nil
	}
	return tvv.Quo(vault.TotalShares.Amount), nil
}

// ConvertDepositToShares converts a deposit in the vault's
// underlying asset into the share amount it purchases, using the current net TVV
// and total share supply (pro-rata, floor arithmetic). Callers validate the
// deposit denom via ValidateAcceptedCoin, so no price conversion is required.
//
// Returns a coin in the share denom. This function performs calculation only;
// callers must enforce liquidity/policy. Returns utils.ErrZeroAssetsWithSharesOutstanding
// when net TVV is zero while shares are outstanding; callers surface that as a rejection.
func (k Keeper) ConvertDepositToShares(ctx sdk.Context, vault types.VaultAccount, in sdk.Coin) (sdk.Coin, error) {
	tvv, err := k.GetNetTVV(ctx, vault)
	if err != nil {
		return sdk.Coin{}, fmt.Errorf("failed to get TVV: %w", err)
	}
	return utils.CalculateSharesProRata(in.Amount, tvv, vault.TotalShares.Amount, vault.TotalShares.Denom)
}

// ConvertSharesToRedeemCoin converts a share amount into a payout coin in the
// vault's underlying asset — the only denom a vault redeems — using the current
// net TVV and total share supply (pro-rata, floor arithmetic).
//
// This function performs calculation only; callers must enforce liquidity/policy.
// If shares <= 0, returns a zero-amount coin.
func (k Keeper) ConvertSharesToRedeemCoin(ctx sdk.Context, vault types.VaultAccount, shares math.Int) (sdk.Coin, error) {
	if !shares.IsPositive() {
		return sdk.NewCoin(vault.UnderlyingAsset, math.ZeroInt()), nil
	}
	tvv, err := k.GetNetTVV(ctx, vault)
	if err != nil {
		return sdk.Coin{}, fmt.Errorf("failed to get TVV: %w", err)
	}
	return utils.CalculateRedeemProRata(shares, vault.TotalShares.Amount, tvv, vault.UnderlyingAsset)
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
// inside GetTVV.
//
// Returns an sdk.Coin { Denom: vault.UnderlyingAsset, Amount: ... }.
func (k Keeper) EstimateTotalVaultValue(ctx sdk.Context, vault *types.VaultAccount) (sdk.Coin, error) {
	baseAmt, err := k.GetTVV(ctx, *vault)
	if err != nil {
		return sdk.Coin{}, fmt.Errorf("failed to get tvv: %w", err)
	}
	estAmt, err := k.CalculateVaultTotalAssets(ctx, vault, sdk.Coin{Denom: vault.UnderlyingAsset, Amount: baseAmt})
	if err != nil {
		return sdk.Coin{}, fmt.Errorf("failed to estimate tvv: %w", err)
	}
	return sdk.Coin{Denom: vault.UnderlyingAsset, Amount: estAmt}, nil
}
