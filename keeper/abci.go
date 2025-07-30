package keeper

import (
	"context"
)

func (k *Keeper) BeginBlocker(ctx context.Context) error {
	return k.handleVaultInterestTimeouts(ctx)
}

func (k *Keeper) EndBlocker(ctx context.Context) error {
	return k.handleReconciledVaults(ctx)
}
