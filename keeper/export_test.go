package keeper

import (
	"context"
	"testing"

	"github.com/provlabs/vault/types"
)

// TestAccessor_handleReconciledVaults exposes this keeper's handleReconciledVaults function for unit tests.
func (k Keeper) TestAccessor_handleReconciledVaults(t *testing.T, ctx context.Context) error {
	t.Helper()
	return k.handleReconciledVaults(ctx)
}

// TestAccessor_handlePayableVaults exposes this keeper's handlePayableVaults function for unit tests.
func (k Keeper) TestAccessor_handlePayableVaults(t *testing.T, ctx context.Context, payouts []*types.VaultAccount) {
	t.Helper()
	k.handlePayableVaults(ctx, payouts)
}

// TestAccessor_handleDepletedVaults exposes this keeper's handleDepletedVaults function for unit tests.
func (k Keeper) TestAccessor_handleDepletedVaults(t *testing.T, ctx context.Context, failedPayouts []*types.VaultAccount) {
	t.Helper()
	k.handleDepletedVaults(ctx, failedPayouts)
}

// TestAccessor_handleDepletedVaults exposes this keeper's handleDepletedVaults function for unit tests.
func (k Keeper) TestAccessor_handleVaultInterestTimeouts(t *testing.T, ctx context.Context) error {
	t.Helper()
	return k.handleVaultInterestTimeouts(ctx)
}

// TestAccessor_processSwapOutJobs exposes this keeper's processSwapOutJobs function for unit tests.
func (k Keeper) TestAccessor_processSwapOutJobs(t *testing.T, ctx context.Context, jobsToProcess []types.PayoutJob) {
	t.Helper()
	k.processSwapOutJobs(ctx, jobsToProcess)
}
