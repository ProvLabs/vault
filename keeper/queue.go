package keeper

import (
	"context"

	"github.com/provlabs/vault/interest"
	"github.com/provlabs/vault/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// AutoReconcileTimeout is the duration (in seconds) that a vault is considered
	// recently reconciled and is exempt from automatic interest checks.
	AutoReconcileTimeout = 20 * interest.SecondsPerHour
)

// SafeEnqueueVerification clears any existing timeout entry for the given vault (if any),
// sets the vault's period start to the current block time, clears the period timeout,
// persists the vault, and enqueues the vault in the PayoutVerificationQueue.
//
// This ensures a vault is not present in both the verification and timeout queues
// at the same time. Typically called after enabling interest or completing a
// reconciliation so the next accrual cycle begins cleanly.
func (k Keeper) SafeEnqueueVerification(ctx context.Context, vault *types.VaultAccount) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	currentBlockTime := sdkCtx.BlockTime().Unix()

	if err := k.PayoutTimeoutQueue.Dequeue(ctx, vault.PeriodTimeout, vault.GetAddress()); err != nil {
		return err
	}

	vault.PeriodStart = currentBlockTime
	vault.PeriodTimeout = 0
	if err := k.SetVaultAccount(sdkCtx, vault); err != nil {
		return err
	}

	return k.PayoutVerificationQueue.Enqueue(ctx, vault.GetAddress())
}

// SafeEnqueueTimeout clears any existing timeout entry for the given vault (if any),
// sets the vault's period start to the current block time, sets a new period timeout
// at (now + AutoReconcileTimeout), persists the vault, and enqueues the timeout entry
// in the PayoutTimeoutQueue.
//
// This ensures a vault is not present in both the timeout and verification queues
// at the same time. Typically called after a vault has been marked as payable so it
// will be revisited after the auto-reconcile window.
func (k Keeper) SafeEnqueueTimeout(ctx context.Context, vault *types.VaultAccount) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	if err := k.PayoutTimeoutQueue.Dequeue(ctx, vault.PeriodTimeout, vault.GetAddress()); err != nil {
		return err
	}

	currentBlockTime := sdkCtx.BlockTime().Unix()
	vault.PeriodStart = currentBlockTime
	vault.PeriodTimeout = currentBlockTime + AutoReconcileTimeout
	if err := k.SetVaultAccount(sdkCtx, vault); err != nil {
		sdkCtx.Logger().Error("failed to set vault", "vault", vault.GetAddress().String(), "err", err)
		return err
	}
	return k.PayoutTimeoutQueue.Enqueue(ctx, vault.PeriodTimeout, vault.GetAddress())
}
