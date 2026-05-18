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
// The denom may not be the vault's share denom or its underlying asset, both of
// which derive their value elsewhere (the share denom from valuation and the
// underlying asset from its implicit 1:1 self price). The price must be a valid
// positive coin and the volume must be positive.
//
// An EventNAVUpdated event is emitted with signer recorded as the NAV authority
// that performed the update.
func (k *Keeper) SetVaultNAV(ctx sdk.Context, vault *types.VaultAccount, nav types.VaultNAV, signer string) error {
	if nav.Denom == vault.TotalShares.Denom {
		return fmt.Errorf("cannot set NAV for vault share denom %q", nav.Denom)
	}
	if nav.Denom == vault.UnderlyingAsset {
		return fmt.Errorf("cannot set NAV for vault underlying asset %q; it is always priced 1:1", nav.Denom)
	}
	if err := nav.Price.Validate(); err != nil {
		return fmt.Errorf("invalid NAV price: %w", err)
	}
	if !nav.Price.Amount.IsPositive() {
		return fmt.Errorf("NAV price amount must be positive, got %s", nav.Price.Amount)
	}
	if nav.Volume.IsNil() || !nav.Volume.IsPositive() {
		return fmt.Errorf("NAV volume must be positive")
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
