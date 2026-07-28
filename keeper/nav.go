package keeper

import (
	"errors"
	"fmt"

	"github.com/provlabs/vault/types"

	"cosmossdk.io/collections"

	sdk "github.com/cosmos/cosmos-sdk/types"

	metadatatypes "github.com/provenance-io/provenance/x/metadata/types"
)

// metadataDenomAddress returns the metadata address a value-owner denom
// (nft/<bech32-metadata-addr>) refers to, and whether denom has that form at all.
// These denoms are legitimate vault assets but are barred from being markers, so the
// marker requirement is skipped for them. A malformed nft/... string is not a
// metadata denom and remains subject to the marker check.
func metadataDenomAddress(denom string) (metadatatypes.MetadataAddress, bool) {
	addr, err := metadatatypes.MetadataAddressFromDenom(denom)
	if err != nil {
		return nil, false
	}
	return addr, true
}

// requireNAVDenomRegistered enforces the on-chain denom requirement for an internal
// NAV entry: a metadata value-owner denom (nft/<scope-id>) must name an existing scope,
// and every other denom must be a registered marker.
//
// This gates the first pricing of a denom only. The marker or scope belongs to someone
// else and can disappear afterwards, so re-checking an entry already in the table would
// strand it: SetVaultNAV skips it when repricing and InitGenesis only warns.
func (k Keeper) requireNAVDenomRegistered(ctx sdk.Context, denom string) error {
	if metadataAddr, ok := metadataDenomAddress(denom); ok {
		if !metadataAddr.IsScopeAddress() {
			return fmt.Errorf("NAV denom %q is not a scope metadata address", denom)
		}
		if _, found := k.MetadataKeeper.GetScope(ctx, metadataAddr); !found {
			return fmt.Errorf("NAV denom %q does not refer to an existing scope", denom)
		}
		return nil
	}
	if _, err := k.MarkerKeeper.GetMarkerByDenom(ctx, denom); err != nil {
		return fmt.Errorf("NAV denom %q is not a registered marker: %w", denom, err)
	}
	return nil
}

// validateVaultNAVFields checks all stateless constraints on a NAV entry
// against its vault. It does not verify chain state (e.g. registered markers).
func validateVaultNAVFields(vault *types.VaultAccount, nav types.VaultNAV) error {
	if nav.Denom == vault.TotalShares.Denom {
		return fmt.Errorf("cannot set NAV for vault share denom %q", nav.Denom)
	}
	if nav.Denom == nav.Price.Denom {
		return fmt.Errorf("NAV denom %q and price denom must differ", nav.Denom)
	}
	if err := nav.Price.Validate(); err != nil {
		return fmt.Errorf("invalid NAV price: %w", err)
	}
	if nav.Price.Denom != vault.UnderlyingAsset {
		return fmt.Errorf("NAV price denom %q must be the vault underlying asset %q", nav.Price.Denom, vault.UnderlyingAsset)
	}
	if nav.Volume.IsNil() || !nav.Volume.IsPositive() {
		return fmt.Errorf("NAV volume must be positive")
	}
	if len(nav.Source) > types.MaxNAVSourceLength {
		return fmt.Errorf("NAV source too long (expected <= %d, actual: %d)", types.MaxNAVSourceLength, len(nav.Source))
	}
	return nil
}

// requireNAVEntryCapacity reports whether the vault has room to start pricing denom,
// enforcing the max_vault_nav_entries cap that bounds GetTVV's walk over the table.
//
// Callers must only invoke this for a denom the vault does not already price, since
// repricing does not grow the table.
func (k Keeper) requireNAVEntryCapacity(ctx sdk.Context, vault *types.VaultAccount, denom string) error {
	maxEntries, err := k.GetMaxVaultNAVEntries(ctx)
	if err != nil {
		return fmt.Errorf("failed to get max vault NAV entries: %w", err)
	}

	entries, err := k.countVaultNAVEntries(ctx, vault.GetAddress(), maxEntries)
	if err != nil {
		return err
	}
	if entries >= maxEntries {
		return fmt.Errorf("vault %s already holds the maximum of %d internal NAV entries: remove an entry before pricing denom %q", vault.Address, maxEntries, denom)
	}
	return nil
}

// countVaultNAVEntries returns the number of internal NAV entries stored for a vault,
// stopping the walk once limit entries have been seen. The result is the exact count below
// limit and exactly limit at or above it, which is all a capacity check needs and keeps
// the cost bounded by limit rather than by the size of the table.
func (k Keeper) countVaultNAVEntries(ctx sdk.Context, vaultAddr sdk.AccAddress, limit uint32) (uint32, error) {
	var count uint32
	navRange := collections.NewPrefixedPairRange[sdk.AccAddress, string](vaultAddr)
	err := k.NAVs.Walk(ctx, navRange, func(_ collections.Pair[sdk.AccAddress, string], _ types.VaultNAV) (bool, error) {
		count++
		return count >= limit, nil
	})
	if err != nil {
		return 0, fmt.Errorf("failed to count internal NAV entries for vault %s: %w", vaultAddr, err)
	}
	return count, nil
}

// SetVaultNAV creates or updates the internal net asset value entry for a denom
// on the given vault. The denom need not be one the vault already holds: pricing an
// unheld denom is what authorizes the asset manager to acquire it, and the entry
// contributes nothing to total vault value until the asset arrives at the principal
// marker (see GetTVV). The nav argument supplies the denom, price, volume, and
// source; the updated block height and time are stamped from ctx before the entry
// is stored.
//
// The denom may not be the vault's share denom, whose value is derived from
// the vault's total holdings rather than set externally. The price must be a valid coin
// denominated in the vault's underlying asset. Its amount may be zero so the authority
// can write a worthless held asset down to zero. The volume must be positive.
//
// A denom the vault does not already price must be a registered marker (or, for an
// nft/<scope-id> denom, name an existing scope), and is rejected once the vault holds
// max_vault_nav_entries entries. Repricing an existing entry skips both checks, so a
// lowered cap or a vanished marker or scope can never strand a held asset at a stale price.
//
// This method does NOT verify that signer is authorized to mutate the vault's
// NAV table; signer is recorded for event attribution only. Callers must run
// vault.ValidateNAVAuthority (or an equivalent check) before invoking it.
//
// An EventNAVUpdated event is emitted with signer recorded as the NAV authority
// that performed the update.
func (k *Keeper) SetVaultNAV(ctx sdk.Context, vault *types.VaultAccount, nav types.VaultNAV, signer string) error {
	if err := validateVaultNAVFields(vault, nav); err != nil {
		return err
	}

	alreadyPriced, err := k.NAVs.Has(ctx, collections.Join(vault.GetAddress(), nav.Denom))
	if err != nil {
		return fmt.Errorf("failed to check for existing internal NAV for denom %q on vault %s: %w", nav.Denom, vault.Address, err)
	}
	if !alreadyPriced {
		if err := k.requireNAVDenomRegistered(ctx, nav.Denom); err != nil {
			return err
		}
		if err := k.requireNAVEntryCapacity(ctx, vault, nav.Denom); err != nil {
			return err
		}
	}

	nav.UpdatedBlockHeight = ctx.BlockHeight()
	nav.UpdatedTime = ctx.BlockTime().UTC()
	if err := k.NAVs.Set(ctx, collections.Join(vault.GetAddress(), nav.Denom), nav); err != nil {
		return fmt.Errorf("failed to store vault NAV: %w", err)
	}

	k.emitEvent(ctx, types.NewEventNAVUpdated(vault.Address, nav, signer))

	return nil
}

// GetVaultNAV returns the internal NAV entry for the given vault address and
// denom. It returns collections.ErrNotFound when no entry exists.
func (k *Keeper) GetVaultNAV(ctx sdk.Context, vaultAddr sdk.AccAddress, denom string) (types.VaultNAV, error) {
	return k.NAVs.Get(ctx, collections.Join(vaultAddr, denom))
}

// checkSettlementNAVGuardrail requires an asset settlement to trade exactly at the
// vault's internal NAV entry for the asset denom, so the manager cannot settle at
// off-NAV prices. Equality is checked by cross-multiplication
// (assetAmount * navPrice == paymentAmount * navVolume) to avoid rounding.
//
// A denom with no entry is rejected rather than waved through. The NAV authority must
// price a denom before the asset manager can trade it, so the manager cannot mint a
// price of their choosing into the pricing table by being the first to acquire the
// denom. The internal NAV table is a price list rather than a held-asset inventory:
// the authority may price a denom the vault does not hold yet, and until the asset
// arrives at the principal marker that entry contributes nothing to the vault's value
// (see GetTVV). Pricing first and settling second is therefore the acquisition path,
// and both messages can ride in a single transaction so the authority's price and the
// manager's settlement commit atomically.
func (k *Keeper) checkSettlementNAVGuardrail(ctx sdk.Context, vault *types.VaultAccount, assetCoin, paymentCoin sdk.Coin) error {
	nav, err := k.GetVaultNAV(ctx, vault.GetAddress(), assetCoin.Denom)
	if err != nil {
		if errors.Is(err, collections.ErrNotFound) {
			return fmt.Errorf("denom %q has no internal NAV entry on vault %s: the NAV authority must price it before it can be settled", assetCoin.Denom, vault.Address)
		}
		return fmt.Errorf("failed to get internal NAV for denom %q on vault %s: %w", assetCoin.Denom, vault.Address, err)
	}

	if nav.Price.Denom != paymentCoin.Denom {
		return fmt.Errorf("settlement of %s for %s is priced in %q but internal NAV for %q on vault %s is priced in %q",
			assetCoin, paymentCoin, paymentCoin.Denom, assetCoin.Denom, vault.Address, nav.Price.Denom)
	}

	assetValue, err := assetCoin.Amount.SafeMul(nav.Price.Amount)
	if err != nil {
		return fmt.Errorf("failed to multiply settlement asset amount %s by NAV price %s: %w", assetCoin.Amount, nav.Price.Amount, err)
	}
	paymentValue, err := paymentCoin.Amount.SafeMul(nav.Volume)
	if err != nil {
		return fmt.Errorf("failed to multiply settlement payment amount %s by NAV volume %s: %w", paymentCoin.Amount, nav.Volume, err)
	}
	if !assetValue.Equal(paymentValue) {
		return fmt.Errorf("settlement of %s for %s does not match internal NAV of %s per %s%s on vault %s",
			assetCoin, paymentCoin, nav.Price, nav.Volume, assetCoin.Denom, vault.Address)
	}

	return nil
}

// RemoveVaultNAV deletes the internal net asset value entry for a denom on the
// given vault and emits an EventNAVRemoved carrying the last recorded price and
// volume, so a price the vault no longer stands behind stops driving valuation
// while its final value is still surfaced to downstream consumers.
//
// It serves two callers: the outbound settlement path, which drops the entry
// once it has drained the vault's last unit of a denom, and the NAV authority,
// which revokes a price it set for a denom the vault never acquired. The
// signer is recorded on the event for attribution and is empty for the
// protocol-initiated settlement removal.
//
// This method does NOT verify that signer is authorized to mutate the vault's
// NAV table, nor that the vault has stopped holding the denom; callers own both
// checks. It returns an error when no entry exists for the denom.
func (k *Keeper) RemoveVaultNAV(ctx sdk.Context, vault *types.VaultAccount, denom, signer string) error {
	nav, err := k.GetVaultNAV(ctx, vault.GetAddress(), denom)
	if err != nil {
		return fmt.Errorf("failed to get internal NAV for denom %q on vault %s: %w", denom, vault.Address, err)
	}
	if err := k.NAVs.Remove(ctx, collections.Join(vault.GetAddress(), denom)); err != nil {
		return fmt.Errorf("failed to remove internal NAV for denom %q on vault %s: %w", denom, vault.Address, err)
	}
	k.emitEvent(ctx, types.NewEventNAVRemoved(vault.Address, nav, signer))
	return nil
}

// SetNAVAuthority rotates the address authorized to mutate the vault's internal
// NAV table. The caller is responsible for verifying that signer is authorized
// to perform this rotation (typically via vault.ValidateAdmin); signer is
// recorded on the emitted EventNAVAuthorityUpdated for attribution only.
//
// When newAuthority equals the current vault.NavAuthority this is a no-op: the
// vault is left unchanged and no event is emitted.
func (k *Keeper) SetNAVAuthority(ctx sdk.Context, vault *types.VaultAccount, newAuthority, signer string) error {
	if vault.NavAuthority == newAuthority {
		return nil
	}
	vault.NavAuthority = newAuthority
	if err := k.SetVaultAccount(ctx, vault); err != nil {
		return fmt.Errorf("failed to set vault account: %w", err)
	}
	k.emitEvent(ctx, types.NewEventNAVAuthorityUpdated(vault.Address, signer, newAuthority))
	return nil
}
