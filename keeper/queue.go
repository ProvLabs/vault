package keeper

import (
	"fmt"

	"github.com/provlabs/vault/interest"
	"github.com/provlabs/vault/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// AutoReconcileTimeout is the duration (in seconds) that a vault is considered
	// recently reconciled and is exempt from automatic interest checks.
	AutoReconcileTimeout = 20 * interest.SecondsPerHour
)

// SafeAddVerification clears any existing timeout entry for the given vault (if any),
// sets the vault's period start to the current block time, clears the period timeout,
// persists the vault, and stores the vault in the PayoutVerificationSet.
//
// This ensures a vault is not present in both the verification set and timeout queues
// at the same time. Typically called after enabling interest or completing a
// reconciliation so the next accrual cycle begins cleanly.
func (k Keeper) SafeAddVerification(ctx sdk.Context, vault *types.VaultAccount) error {
	cacheCtx, write := ctx.CacheContext()
	currentBlockTime := cacheCtx.BlockTime().Unix()
	v := vault.Clone()

	if err := k.PayoutTimeoutQueue.Dequeue(cacheCtx, v.PeriodTimeout, v.GetAddress()); err != nil {
		return fmt.Errorf("failed to dequeue existing payout timeout for vault %s: %w", v.GetAddress(), err)
	}

	v.PeriodStart = currentBlockTime
	v.PeriodTimeout = 0
	if err := k.SetVaultAccount(cacheCtx, v); err != nil {
		k.getLogger(cacheCtx).Error("failed to set vault while adding to verification set", "vault", v.GetAddress().String(), "err", err)
		return fmt.Errorf("failed to persist vault period for vault %s: %w", v.GetAddress(), err)
	}

	if err := k.PayoutVerificationSet.Set(cacheCtx, v.GetAddress()); err != nil {
		return fmt.Errorf("failed to set payout verification for vault %s: %w", v.GetAddress(), err)
	}

	write()
	*vault = *v
	return nil
}

// SafeEnqueueTimeout clears any existing timeout entry for the given vault (if any),
// sets the vault's period start to the current block time, sets a new period timeout
// at (now + AutoReconcileTimeout), persists the vault, and enqueues the timeout entry
// in the PayoutTimeoutQueue.
//
// This ensures a vault is not present in both the timeout and verification queues
// at the same time. Typically called after a vault has been marked as payable so it
// will be revisited after the auto-reconcile window.
func (k Keeper) SafeEnqueueTimeout(ctx sdk.Context, vault *types.VaultAccount) error {
	cacheCtx, write := ctx.CacheContext()
	v := vault.Clone()

	if err := k.PayoutTimeoutQueue.Dequeue(cacheCtx, v.PeriodTimeout, v.GetAddress()); err != nil {
		return fmt.Errorf("failed to dequeue existing payout timeout for vault %s: %w", v.GetAddress(), err)
	}

	currentBlockTime := cacheCtx.BlockTime().Unix()
	v.PeriodStart = currentBlockTime
	v.PeriodTimeout = currentBlockTime + AutoReconcileTimeout
	if err := k.SetVaultAccount(cacheCtx, v); err != nil {
		k.getLogger(cacheCtx).Error("failed to set vault while enqueueing payout timeout", "vault", v.GetAddress().String(), "err", err)
		return fmt.Errorf("failed to persist vault period for vault %s: %w", v.GetAddress(), err)
	}

	if err := k.PayoutTimeoutQueue.Enqueue(cacheCtx, v.PeriodTimeout, v.GetAddress()); err != nil {
		return fmt.Errorf("failed to enqueue payout timeout for vault %s at %d: %w", v.GetAddress(), v.PeriodTimeout, err)
	}

	write()
	*vault = *v
	return nil
}

// SafeEnqueueFeeTimeout clears any existing fee timeout entry for the given vault (if any),
// sets the vault's fee period start to the current block time, sets a new fee period timeout
// at (now + AutoReconcileTimeout), persists the vault, and enqueues the timeout entry
// in the FeeTimeoutQueue.
func (k Keeper) SafeEnqueueFeeTimeout(ctx sdk.Context, vault *types.VaultAccount) error {
	cacheCtx, write := ctx.CacheContext()
	v := vault.Clone()

	if err := k.FeeTimeoutQueue.Dequeue(cacheCtx, v.FeePeriodTimeout, v.GetAddress()); err != nil {
		return fmt.Errorf("failed to dequeue existing fee timeout for vault %s: %w", v.GetAddress(), err)
	}

	currentBlockTime := cacheCtx.BlockTime().Unix()
	v.FeePeriodStart = currentBlockTime
	v.FeePeriodTimeout = currentBlockTime + AutoReconcileTimeout
	if err := k.SetVaultAccount(cacheCtx, v); err != nil {
		k.getLogger(cacheCtx).Error("failed to set vault while enqueueing fee timeout", "vault", v.GetAddress().String(), "err", err)
		return fmt.Errorf("failed to persist vault fee period for vault %s: %w", v.GetAddress(), err)
	}

	if err := k.FeeTimeoutQueue.Enqueue(cacheCtx, v.FeePeriodTimeout, v.GetAddress()); err != nil {
		return fmt.Errorf("failed to enqueue fee timeout for vault %s at %d: %w", v.GetAddress(), v.FeePeriodTimeout, err)
	}

	write()
	*vault = *v
	return nil
}
