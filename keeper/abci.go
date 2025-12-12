package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// MaxSwapOutBatchSize is the maximum number of pending swap-out requests
	// to process in a single EndBlocker invocation. This prevents a large queue
	// from consuming excessive block time and memory. This is a temporary value
	// and we will need to do more analysis on a proper batch size.
	// See https://github.com/ProvLabs/vault/issues/75.
	MaxSwapOutBatchSize = 100
)

// BeginBlocker is a hook that is called at the beginning of every block.
func (k *Keeper) BeginBlocker(ctx sdk.Context) error {
	return k.handleVaultInterestTimeouts(ctx)
}

// EndBlocker is a hook that is called at the end of every block.
func (k *Keeper) EndBlocker(ctx sdk.Context) error {
	if err := k.processPendingSwapOuts(ctx, MaxSwapOutBatchSize); err != nil {
		return err
	}

	if err := k.handleReconciledVaults(ctx); err != nil {
		return err
	}

	return nil
}
