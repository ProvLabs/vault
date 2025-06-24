package keeper

import (
	"context"
	"errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/provlabs/vault/types"
)

// GetVaults is a helper function for retrieving all vaults from state.
func (k *Keeper) GetVaults(ctx context.Context) (map[string]types.Vault, error) {
	vaults := map[string]types.Vault{}

	err := k.Vaults.Walk(ctx, nil, func(key sdk.AccAddress, vault types.Vault) (stop bool, err error) {
		vaults[key.String()] = vault
		return false, nil
	})

	return vaults, err
}

func (k *Keeper) SetVault(ctx context.Context, vault *types.Vault) error {
	if vault == nil {
		return errors.New("vault cannot be nil")
	}

	addr, err := sdk.AccAddressFromBech32(vault.VaultAddress)
	if err != nil {
		return err
	}

	return k.Vaults.Set(ctx, addr, *vault)
}

// GetVaults is a helper function for retrieving all vaults from state.
func (k *Keeper) GetParams(ctx context.Context) (types.Params, error) {
	params, err := k.Params.Get(ctx)
	if err != nil {
		return types.Params{}, errors.New("unable to get params from state")
	}

	return params, nil
}
