package keeper

import (
	"context"
	"errors"

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

// Sets a vault in the store, using its address as the key.
func (k *Keeper) SetVault(ctx context.Context, vault *types.VaultAccount) error {
	if vault == nil {
		return errors.New("vault cannot be nil")
	}

	addr, err := sdk.AccAddressFromBech32(vault.Address)
	if err != nil {
		return err
	}

	return k.Vaults.Set(ctx, addr, []byte{})
}

// SetVaultAccount persists a vault account after validating it.
func (k *Keeper) SetVaultAccount(ctx sdk.Context, vault *types.VaultAccount) error {
	if err := vault.Validate(); err != nil {
		return err
	}
	k.AuthKeeper.SetAccount(ctx, vault)
	return nil
}

// GetVaults is a helper function for retrieving all vaults from state.
func (k *Keeper) GetParams(ctx context.Context) (types.Params, error) {
	params, err := k.Params.Get(ctx)
	if err != nil {
		return types.Params{}, errors.New("unable to get params from state")
	}

	return params, nil
}
