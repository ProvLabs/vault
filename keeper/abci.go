package keeper

import (
	"context"
)

// BeginBlocker is a hook that is called at the beginning of every block.
func (k *Keeper) BeginBlocker(ctx context.Context) error {
	return k.handleVaultInterestTimeouts(ctx)
}

// EndBlocker is a hook that is called at the end of every block.
func (k *Keeper) EndBlocker(ctx context.Context) error {
	return k.handleReconciledVaults(ctx)
}
