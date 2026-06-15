package keeper

import (
	"errors"
	"fmt"

	"github.com/provlabs/vault/types"

	"cosmossdk.io/collections"

	sdk "github.com/cosmos/cosmos-sdk/types"

	markertypes "github.com/provenance-io/provenance/x/marker/types"
	metadatatypes "github.com/provenance-io/provenance/x/metadata/types"
)

// isMetadataDenom reports whether denom is a well-formed metadata value-owner
// denom (nft/<bech32-metadata-addr>), such as the coin minted for a scope. These
// denoms are legitimate vault assets but are barred from being markers, so the
// marker requirement and marker NAV mirror are skipped for them. A malformed
// nft/... string is not a metadata denom and remains subject to the marker check.
func isMetadataDenom(denom string) bool {
	_, err := metadatatypes.MetadataAddressFromDenom(denom)
	return err == nil
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
	if !nav.Price.Amount.IsPositive() {
		return fmt.Errorf("NAV price amount must be positive, got %s", nav.Price.Amount)
	}
	if !vault.IsAcceptedDenom(nav.Price.Denom) {
		return fmt.Errorf("NAV price denom %q must be an accepted vault denom %v", nav.Price.Denom, vault.AcceptedDenoms())
	}
	if nav.Volume.IsNil() || !nav.Volume.IsPositive() {
		return fmt.Errorf("NAV volume must be positive")
	}
	if len(nav.Source) > types.MaxNAVSourceLength {
		return fmt.Errorf("NAV source too long (expected <= %d, actual: %d)", types.MaxNAVSourceLength, len(nav.Source))
	}
	return nil
}

// SetVaultNAV creates or updates the internal net asset value entry for a denom
// held by the given vault. The nav argument supplies the denom, price, volume,
// and source; the updated block height and time are stamped from ctx before the
// entry is stored.
//
// The denom may not be the vault's share denom, whose value is derived from
// the vault's total holdings rather than set externally. The denom must also
// be a registered marker on-chain, except for metadata value-owner denoms
// (nft/<scope-id>), which are legitimate vault assets but cannot be markers.
// The price must be a valid positive coin
// denominated in one of the vault's accepted denoms (underlying asset or
// payment denom), and the volume must be positive.
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
	if !isMetadataDenom(nav.Denom) {
		if _, err := k.MarkerKeeper.GetMarkerByDenom(ctx, nav.Denom); err != nil {
			return fmt.Errorf("NAV denom %q is not a registered marker: %w", nav.Denom, err)
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
// (assetAmount * navPrice == paymentAmount * navVolume) to avoid rounding. When no
// entry exists (first acquisition) the guardrail is skipped.
func (k *Keeper) checkSettlementNAVGuardrail(ctx sdk.Context, vault *types.VaultAccount, assetCoin, paymentCoin sdk.Coin) error {
	nav, err := k.GetVaultNAV(ctx, vault.GetAddress(), assetCoin.Denom)
	if err != nil {
		if errors.Is(err, collections.ErrNotFound) {
			return nil
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

// publishAssetNAVToMarker mirrors an internal NAV entry into the marker module's
// NAV records for the entry's denom, attributed to the vault address so
// downstream consumers can tell vault-originated prices apart from other
// sources. This is a one-way downstream publish: the internal table stays the
// vault's pricing source of truth and is never read back from the marker.
func (k *Keeper) publishAssetNAVToMarker(ctx sdk.Context, vault *types.VaultAccount, nav types.VaultNAV) error {
	if !nav.Volume.IsUint64() {
		return fmt.Errorf("internal NAV volume %s for denom %q on vault %s overflows the marker NAV volume (uint64)", nav.Volume, nav.Denom, vault.Address)
	}
	marker, err := k.MarkerKeeper.GetMarkerByDenom(ctx, nav.Denom)
	if err != nil {
		return fmt.Errorf("failed to get marker for NAV denom %q: %w", nav.Denom, err)
	}
	markerNAV := markertypes.NetAssetValue{Price: nav.Price, Volume: nav.Volume.Uint64()}
	if err := k.MarkerKeeper.SetNetAssetValue(ctx, marker, markerNAV, vault.Address); err != nil {
		return fmt.Errorf("failed to set marker NAV for denom %q on vault %s: %w", nav.Denom, vault.Address, err)
	}
	return nil
}

// RemoveVaultNAV deletes the internal net asset value entry for a denom held
// by the given vault and emits an EventNAVRemoved carrying the last recorded
// price and volume. It exists for settlement flows that drain the vault's last
// unit of a denom: a price entry for an asset the vault no longer holds must
// not linger in the pricing table, but its final price is still worth
// surfacing to downstream consumers via the event.
//
// It returns an error when no entry exists for the denom. Callers remove
// entries they have just read or written (e.g. immediately after a settlement
// upsert), so a missing entry indicates a caller bug rather than a benign
// no-op.
func (k *Keeper) RemoveVaultNAV(ctx sdk.Context, vault *types.VaultAccount, denom string) error {
	nav, err := k.GetVaultNAV(ctx, vault.GetAddress(), denom)
	if err != nil {
		return fmt.Errorf("failed to get internal NAV for denom %q on vault %s: %w", denom, vault.Address, err)
	}
	if err := k.NAVs.Remove(ctx, collections.Join(vault.GetAddress(), denom)); err != nil {
		return fmt.Errorf("failed to remove internal NAV for denom %q on vault %s: %w", denom, vault.Address, err)
	}
	k.emitEvent(ctx, types.NewEventNAVRemoved(vault.Address, nav))
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
