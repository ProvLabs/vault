package keeper

import (
	"fmt"

	"github.com/provlabs/vault/types"

	"cosmossdk.io/collections"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// SetVaultNAV creates or updates the internal net asset value entry for a denom
// held by the given vault. The nav argument supplies the denom, price, volume,
// and source; the updated block height and time are stamped from ctx before the
// entry is stored.
//
// The denom may not be the vault's share denom, whose value is derived from
// the vault's total holdings rather than set externally. The denom must also
// be a registered marker on-chain. The price must be a valid positive coin
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
	if _, err := k.MarkerKeeper.GetMarkerByDenom(ctx, nav.Denom); err != nil {
		return fmt.Errorf("NAV denom %q is not a registered marker: %w", nav.Denom, err)
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
