package keeper

import (
	"context"
	"errors"
	"fmt"

	"github.com/provlabs/vault/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// GetVaults is a helper function for retrieving all vaults from state.
func (k *Keeper) GetVaults(ctx context.Context) ([]sdk.AccAddress, error) {
	vaults := []sdk.AccAddress{}

	err := k.Vaults.Walk(ctx, nil, func(key sdk.AccAddress, val []byte) (stop bool, err error) {
		vaults = append(vaults, key)
		return false, nil
	})

	return vaults, err
}

// SetVaultLookup stores a vault in the Vaults collection, keyed by its bech32 address.
// NOTE: should only be called by genesis and at vault creation.
// Returns an error if the vault is nil or the address cannot be parsed.
func (k *Keeper) SetVaultLookup(ctx context.Context, vault *types.VaultAccount) error {
	if vault == nil {
		return errors.New("vault cannot be nil")
	}

	addr, err := sdk.AccAddressFromBech32(vault.Address)
	if err != nil {
		return err
	}

	return k.Vaults.Set(ctx, addr, []byte{})
}

// SetVaultAccount validates and persists a VaultAccount using the auth keeper.
// Returns an error if validation fails.
func (k *Keeper) SetVaultAccount(ctx sdk.Context, vault *types.VaultAccount) error {
	if err := vault.Validate(); err != nil {
		return err
	}
	k.AuthKeeper.SetAccount(ctx, vault)
	return nil
}

// FindVaultAccount retrieves a vault by its address or share denomination.
func (k *Keeper) FindVaultAccount(ctx sdk.Context, id string) (*types.VaultAccount, error) {
	if addr, err := sdk.AccAddressFromBech32(id); err == nil {
		vault, err := k.GetVault(ctx, addr)
		if err != nil {
			return nil, err
		}
		if vault != nil {
			return vault, nil
		}
	}

	addr := types.GetVaultAddress(id)
	vault, err := k.GetVault(ctx, addr)
	if err != nil {
		return nil, err
	}
	if vault != nil {
		return vault, nil
	}

	return nil, fmt.Errorf("vault with id '%s' not found", id)
}

