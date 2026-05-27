package keeper

import (
	"fmt"

	"github.com/provlabs/vault/types"

	"cosmossdk.io/collections"
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// MigrationNAVSeedSource is the value recorded on the source field of every
// VaultNAV entry created by migrateInternalNAVSeedFromMarker. It identifies the
// entry as a one-time seed from the Marker module's NAV table so indexers can
// distinguish migrated entries from oracle- or operator-supplied updates.
const MigrationNAVSeedSource = "x/vault internal NAV seed migration"

// uyldsFccDenom is the temporary 1:1 peg denom recognized by the pricing engine
// fast-path. Vaults whose payment_denom or underlying_asset is this denom do not
// require an Internal NAV entry; UnitPriceFraction short-circuits to (1, 1) and
// the migration intentionally skips them. See
// https://github.com/ProvLabs/vault/issues/73.
const uyldsFccDenom = "uylds.fcc"

// MigrateVaultAccountPaymentDenomDefaults updates legacy VaultAccount state created
// prior to v1.0.13 by normalizing empty payment denom fields.
//
// In versions <= v1.0.13, VaultAccount instances could be persisted with an empty
// PaymentDenom. Newer versions require a valid payment denom, which by default
// should be the vault’s underlying asset denom.
//
// This migration iterates all accounts in the auth store, identifies VaultAccount
// instances, and for any vault with an empty PaymentDenom sets it to UnderlyingAsset.
// Updated vault accounts are validated and re-persisted using the auth keeper.
//
// This function is intended to be executed once from an upgrade handler and is
// idempotent; running it multiple times will not modify already-migrated state.
func (k Keeper) MigrateVaultAccountPaymentDenomDefaults(ctx sdk.Context) error {
	allAccounts := k.AuthKeeper.GetAllAccounts(ctx)

	for _, acc := range allAccounts {
		v, ok := acc.(*types.VaultAccount)
		if !ok {
			continue
		}

		if v.PaymentDenom == "" {
			v.PaymentDenom = v.UnderlyingAsset
			err := k.SetVaultAccount(ctx, v)
			if err != nil {
				return fmt.Errorf("failed to update vault account %s during migration: %w", v.Address, err)
			}
		}
	}

	return nil
}

// migrateInternalNAVSeedFromMarker seeds the Internal NAV table for every vault
// that will need an Internal NAV entry once the pricing engine switches to
// Internal-NAV-only reads.
//
// It is unexported on purpose: the only supported entry point is the
// Migrator.Migrate1to2 handler registered with the SDK module manager via
// module.RegisterServices. Running it directly from a downstream upgrade
// handler would bypass the ConsensusVersion tracking that makes the migration
// fire exactly once per chain.
//
// For each VaultAccount in state the migration:
//  1. Defaults v.NavAuthority to v.Admin when empty.
//  2. Skips the vault if payment_denom == underlying_asset (identity fast-path
//     in UnitPriceFraction; an Internal NAV entry is impossible here because
//     denom and price denom would collide).
//  3. Skips the vault if an Internal NAV entry already exists for payment_denom
//     so a re-run preserves operator-supplied values.
//  4. For vaults where payment_denom or underlying_asset is "uylds.fcc", writes
//     a 1:1 Internal NAV (Price.Amount=1, Volume=1) so the new pricing engine
//     reads an explicit entry rather than relying on the legacy peg fast-path.
//     See https://github.com/ProvLabs/vault/issues/73.
//  5. Otherwise reads the forward (payment→underlying) and reverse
//     (underlying→payment) Marker NAVs and, mirroring UnitPriceFraction, picks
//     the entry with the greater UpdatedBlockHeight when both are present, or
//     the single available entry. The chosen NAV is translated into the
//     single-sided Internal NAV format (Denom=payment_denom, Price denominated
//     in underlying_asset).
//
// All seeded entries are persisted via SetVaultNAV, which emits
// EventNAVUpdated for indexers.
//
// If a vault has payment_denom != underlying_asset, is not on the uylds.fcc peg
// fast-path, has no pre-existing Internal NAV entry, and no forward or reverse
// Marker NAV is available, the migration returns an error naming the vault.
// Failing the migration is preferred over silently leaving the vault
// unpriceable post-upgrade.
//
// The migration is idempotent on subsequent runs because pre-existing Internal
// NAV entries are preserved in step 3.
func (k Keeper) migrateInternalNAVSeedFromMarker(ctx sdk.Context) error {
	accounts := k.AuthKeeper.GetAllAccounts(ctx)

	for _, acc := range accounts {
		vault, ok := acc.(*types.VaultAccount)
		if !ok {
			continue
		}

		if vault.NavAuthority == "" {
			vault.NavAuthority = vault.Admin
			if err := k.SetVaultAccount(ctx, vault); err != nil {
				return fmt.Errorf("failed to default nav_authority for vault %s: %w", vault.Address, err)
			}
		}

		if vault.PaymentDenom == vault.UnderlyingAsset {
			continue
		}

		exists, err := k.NAVs.Has(ctx, collections.Join(vault.GetAddress(), vault.PaymentDenom))
		if err != nil {
			return fmt.Errorf("failed to check internal NAV for vault %s: %w", vault.Address, err)
		}
		if exists {
			continue
		}

		var nav types.VaultNAV
		if vault.PaymentDenom == uyldsFccDenom || vault.UnderlyingAsset == uyldsFccDenom {
			nav = types.VaultNAV{
				Denom:  vault.PaymentDenom,
				Price:  sdk.NewCoin(vault.UnderlyingAsset, math.OneInt()),
				Volume: math.OneInt(),
				Source: MigrationNAVSeedSource,
			}
		} else {
			nav, err = k.buildInternalNAVFromMarker(ctx, vault)
			if err != nil {
				return err
			}
		}

		if err := k.SetVaultNAV(ctx, vault, nav, vault.NavAuthority); err != nil {
			return fmt.Errorf("failed to seed internal NAV for vault %s: %w", vault.Address, err)
		}
	}

	return nil
}

// buildInternalNAVFromMarker reads the forward and reverse Marker NAVs for the
// vault's payment/underlying pair and returns the VaultNAV that should be
// written to the Internal NAV table. It mirrors UnitPriceFraction's selection
// rule: when both directions exist, the one with the greater UpdatedBlockHeight
// wins; when only one exists, that entry is used.
//
// Both directions are queried unconditionally so a transient error on one
// lookup does not prevent the migration from using a valid NAV from the other
// direction. An error is returned only when no usable NAV is available — that
// is, when both lookups return nil. In that case the surfaced error names the
// failing direction (if any) so operators can diagnose the underlying Marker
// store issue; if both lookups returned (nil, nil), the migration aborts with a
// "no marker NAV available" error naming the vault.
//
// Also returns an error if the selected NAV has a zero price amount or zero
// volume.
func (k Keeper) buildInternalNAVFromMarker(ctx sdk.Context, vault *types.VaultAccount) (types.VaultNAV, error) {
	fwd, errF := k.MarkerKeeper.GetNetAssetValue(ctx, vault.PaymentDenom, vault.UnderlyingAsset)
	rev, errR := k.MarkerKeeper.GetNetAssetValue(ctx, vault.UnderlyingAsset, vault.PaymentDenom)

	if fwd == nil && rev == nil {
		if errF != nil {
			return types.VaultNAV{}, fmt.Errorf("failed to read forward marker NAV for vault %s (%s/%s): %w", vault.Address, vault.PaymentDenom, vault.UnderlyingAsset, errF)
		}
		if errR != nil {
			return types.VaultNAV{}, fmt.Errorf("failed to read reverse marker NAV for vault %s (%s/%s): %w", vault.Address, vault.UnderlyingAsset, vault.PaymentDenom, errR)
		}
		return types.VaultNAV{}, fmt.Errorf("no marker NAV available to seed internal NAV for vault %s (payment %s, underlying %s)", vault.Address, vault.PaymentDenom, vault.UnderlyingAsset)
	}

	useForward := fwd != nil && (rev == nil || fwd.UpdatedBlockHeight >= rev.UpdatedBlockHeight)

	var priceAmt, volume math.Int
	if useForward {
		if fwd.Volume == 0 || fwd.Price.Amount.IsZero() {
			return types.VaultNAV{}, fmt.Errorf("invalid forward marker NAV for vault %s (%s/%s): price=%s volume=%d", vault.Address, vault.PaymentDenom, vault.UnderlyingAsset, fwd.Price.Amount, fwd.Volume)
		}
		priceAmt = fwd.Price.Amount
		volume = math.NewIntFromUint64(fwd.Volume)
	} else {
		if rev.Volume == 0 || rev.Price.Amount.IsZero() {
			return types.VaultNAV{}, fmt.Errorf("invalid reverse marker NAV for vault %s (%s/%s): price=%s volume=%d", vault.Address, vault.UnderlyingAsset, vault.PaymentDenom, rev.Price.Amount, rev.Volume)
		}
		priceAmt = math.NewIntFromUint64(rev.Volume)
		volume = rev.Price.Amount
	}

	return types.VaultNAV{
		Denom:  vault.PaymentDenom,
		Price:  sdk.NewCoin(vault.UnderlyingAsset, priceAmt),
		Volume: volume,
		Source: MigrationNAVSeedSource,
	}, nil
}
