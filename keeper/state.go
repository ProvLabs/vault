package keeper

import (
	"context"
	"errors"

	"github.com/provlabs/vault/types"
)

// GetVaults is a helper function for retrieving all vaults from state.
func (k *Keeper) GetVaults(ctx context.Context) (map[uint32]types.Vault, error) {
	vaults := make(map[uint32]types.Vault)

	err := k.Vaults.Walk(ctx, nil, func(index uint32, guardianSet types.Vault) (stop bool, err error) {
		vaults[index] = guardianSet
		return false, nil
	})

	return vaults, err
}

// GetVaults is a helper function for retrieving all vaults from state.
func (k *Keeper) GetParams(ctx context.Context) (types.Params, error) {
	params, err := k.Params.Get(ctx)
	if err != nil {
		return types.Params{}, errors.New("unable to get params from state")
	}

	return params, nil
}
