package vault

import (
	"context"

	"cosmossdk.io/core/address"
	"cosmossdk.io/errors"

	"github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/types"
)

// InitGenesis initializes the module's state from a provided genesis state.
func InitGenesis(ctx context.Context, k *keeper.Keeper, cdc address.Codec, genState types.GenesisState) error {
	return k.Params.Set(ctx, genState.Params)
}

// ExportGenesis returns the module's exported genesis.
func ExportGenesis(ctx context.Context, k *keeper.Keeper) *types.GenesisState {
	var err error
	genesis := types.DefaultGenesisState()
	genesis.Params, err = k.Params.Get(ctx)
	if err != nil {
		panic(errors.Wrap(err, "failed to read the params"))
	}

	return genesis
}
