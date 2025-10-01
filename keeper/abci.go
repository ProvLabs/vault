package keeper

import (
	"context"
)

const (
	// MaxSwapOutBatchSize is the maximum number of pending swap-out requests
	// to process in a single EndBlocker invocation. This prevents a large queue
	// from consuming excessive block time and memory.
	MaxSwapOutBatchSize = 100
)

// BeginBlocker is a hook that is called at the beginning of every block.
func (k *Keeper) BeginBlocker(ctx context.Context) error {
	return k.handleVaultInterestTimeouts(ctx)
}

// EndBlocker is a hook that is called at the end of every block.
func (k *Keeper) EndBlocker(ctx context.Context) error {
	if err := k.ProcessPendingSwapOuts(ctx, MaxSwapOutBatchSize); err != nil {
		return err
	}

	if err := k.handleReconciledVaults(ctx); err != nil {
		return err
	}

	return nil
}
