package keeper

import (
	"context"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/provlabs/vault/types"
)

// TestAccessor_handleReconciledVaults exposes this keeper's handleReconciledVaults function for unit tests.
func (k Keeper) TestAccessor_handleReconciledVaults(t *testing.T, ctx context.Context) error {
	t.Helper()
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return k.handleReconciledVaults(sdkCtx)
}

// TestAccessor_handlePayableVaults exposes this keeper's handlePayableVaults function for unit tests.
func (k Keeper) TestAccessor_handlePayableVaults(t *testing.T, ctx context.Context, payouts []*types.VaultAccount) {
	t.Helper()
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	k.handlePayableVaults(sdkCtx, payouts)
}

// TestAccessor_handleDepletedVaults exposes this keeper's handleDepletedVaults function for unit tests.
func (k Keeper) TestAccessor_handleDepletedVaults(t *testing.T, ctx context.Context, failedPayouts []*types.VaultAccount) {
	t.Helper()
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	k.handleDepletedVaults(sdkCtx, failedPayouts)
}

// TestAccessor_handleDepletedVaults exposes this keeper's handleDepletedVaults function for unit tests.
func (k Keeper) TestAccessor_handleVaultInterestTimeouts(t *testing.T, ctx context.Context) error {
	t.Helper()
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return k.handleVaultInterestTimeouts(sdkCtx)
}

// TestAccessor_processSwapOutJobs exposes this keeper's processSwapOutJobs function for unit tests.
func (k Keeper) TestAccessor_processSwapOutJobs(t *testing.T, ctx context.Context, jobsToProcess []types.PayoutJob) {
	t.Helper()
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	k.processSwapOutJobs(sdkCtx, jobsToProcess)
}

// TestAccessor_autoPauseVault exposes this keeper's autoPauseVault function for unit tests.
func (k Keeper) TestAccessor_autoPauseVault(t *testing.T, ctx context.Context, vault *types.VaultAccount, reason string) {
	t.Helper()
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	k.autoPauseVault(sdkCtx, vault, reason)
}

// TestAccessor_reconcileVaultInterest exposes this keeper's reconcileVaultInterest function for unit tests.
func (k Keeper) TestAccessor_reconcileVaultInterest(t *testing.T, ctx context.Context, vault *types.VaultAccount) error {
	t.Helper()
	return k.reconcileVaultInterest(sdk.UnwrapSDKContext(ctx), vault)
}

// TestAccessor_processPendingSwapOuts exposes this keeper's processPendingSwapOuts function for unit tests.
func (k Keeper) TestAccessor_processPendingSwapOuts(t *testing.T, ctx context.Context, size int) error {
	t.Helper()
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return k.processPendingSwapOuts(sdkCtx, size)
}
