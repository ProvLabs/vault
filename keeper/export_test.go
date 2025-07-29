package keeper

import (
	"context"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Test_partitionReconciledVaults exposes this keeper's partitionReconciledVaults function for unit tests.
func (k Keeper) TestAccessor_partitionReconciledVaults(t *testing.T, ctx sdk.Context, vaults []ReconciledVault) ([]ReconciledVault, []ReconciledVault) {
	t.Helper()
	return k.partitionReconciledVaults(ctx, vaults)
}

// TestAccessor_handleReconciledVaults exposes this keeper's handleReconciledVaults function for unit tests.
func (k Keeper) TestAccessor_handleReconciledVaults(t *testing.T, ctx context.Context) error {
	t.Helper()
	return k.handleReconciledVaults(ctx)
}

// TestAccessor_handlePayableVaults exposes this keeper's handlePayableVaults function for unit tests.
func (k Keeper) TestAccessor_handlePayableVaults(t *testing.T, ctx context.Context, payouts []ReconciledVault) {
	t.Helper()
	k.handlePayableVaults(ctx, payouts)
}

// TestAccessor_handleDepletedVaults exposes this keeper's handleDepletedVaults function for unit tests.
func (k Keeper) TestAccessor_handleDepletedVaults(t *testing.T, ctx context.Context, failedPayouts []ReconciledVault) {
	t.Helper()
	k.handleDepletedVaults(ctx, failedPayouts)
}
