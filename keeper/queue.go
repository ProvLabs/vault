package keeper

import (
	"context"

	"github.com/provlabs/vault/interest"
	"github.com/provlabs/vault/types"

	"cosmossdk.io/collections"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// AutoReconcileTimeout is the duration (in seconds) that a vault is considered recently reconciled
	// and is exempt from automatic interest checks in the BeginBlocker.
	AutoReconcileTimeout = 20 * interest.SecondsPerHour
)

// EnqueuePayoutVerification schedules a vault for reconciliation by adding an entry
// to the VaultStartQueue keyed by the given period start and vault address.
func (k Keeper) EnqueuePayoutVerification(ctx context.Context, vaultAddr sdk.AccAddress) error {
	return k.PayoutVerificationQueue.Set(ctx, vaultAddr, collections.NoValue{})
}

// EnqueuePayoutTimeout schedules a vault for timeout processing by adding an entry
// to the VaultTimeoutQueue keyed by the given timeout and vault address.
func (k Keeper) EnqueuePayoutTimeout(ctx context.Context, periodTimeout int64, vaultAddr sdk.AccAddress) error {
	return k.PayoutTimeoutQueue.Set(ctx, collections.Join(uint64(periodTimeout), vaultAddr), collections.NoValue{})
}

// DequeuePayoutVerification removes a vault entry from the VaultStartQueue for the given
// period start and vault address.
func (k Keeper) DequeuePayoutVerification(ctx context.Context, vaultAddr sdk.AccAddress) error {
	return k.PayoutVerificationQueue.Remove(ctx, vaultAddr)
}

// DequeuePayoutTimeout removes a vault entry from the VaultTimeoutQueue for the given
// period timeout and vault address.
func (k Keeper) DequeuePayoutTimeout(ctx context.Context, periodTimeout int64, vaultAddr sdk.AccAddress) error {
	return k.PayoutTimeoutQueue.Remove(ctx, collections.Join(uint64(periodTimeout), vaultAddr))
}

// WalkPayoutVerifications iterates over all entries in the VaultStartQueue with periodStart <= nowSec.
// For each due entry, it calls the provided callback function. Iteration stops if the
// callback returns stop=true or an error.
func (k Keeper) WalkPayoutVerifications(ctx context.Context, fn func(vaultAddr sdk.AccAddress) (stop bool, err error)) error {
	it, err := k.PayoutVerificationQueue.Iterate(ctx, nil)
	if err != nil {
		return err
	}
	defer it.Close()

	for ; it.Valid(); it.Next() {
		kv, err := it.KeyValue()
		if err != nil {
			return err
		}
		stop, err := fn(kv.Key)
		if err != nil || stop {
			return err
		}
	}
	return nil
}

// WalkDuePayoutTimeouts iterates over all entries in the VaultTimeoutQueue with periodTimeout <= nowSec.
// For each due entry, it calls the provided callback function. Iteration stops if the
// callback returns stop=true or an error.
func (k Keeper) WalkDuePayoutTimeouts(ctx context.Context, nowSec int64, fn func(periodTimeout uint64, vaultAddr sdk.AccAddress) (stop bool, err error)) error {
	it, err := k.PayoutTimeoutQueue.Iterate(ctx, nil)
	if err != nil {
		return err
	}
	defer it.Close()

	for ; it.Valid(); it.Next() {
		kv, err := it.KeyValue()
		if err != nil {
			return err
		}
		if kv.Key.K1() > uint64(nowSec) {
			break
		}
		stop, err := fn(kv.Key.K1(), kv.Key.K2())
		if err != nil || stop {
			return err
		}
	}
	return nil
}

// RemoveAllTimeoutsForVault deletes all timeout entries in the VaultTimeoutQueue
// for the given vault address.
func (k Keeper) RemoveAllTimeoutsForVault(ctx context.Context, vaultAddr sdk.AccAddress) error {
	var keys []collections.Pair[uint64, sdk.AccAddress]

	it, err := k.PayoutTimeoutQueue.Iterate(ctx, nil)
	if err != nil {
		return err
	}
	defer it.Close()

	for ; it.Valid(); it.Next() {
		kv, err := it.KeyValue()
		if err != nil {
			return err
		}
		if kv.Key.K2().Equals(vaultAddr) {
			keys = append(keys, kv.Key)
		}
	}
	for _, key := range keys {
		if err := k.PayoutTimeoutQueue.Remove(ctx, key); err != nil {
			return err
		}
	}
	return nil
}

// SafeEnqueueStart clears any existing start or timeout entries for the given vault,
// then enqueues a new start entry at the provided periodStart.
//
// This ensures a vault is never present in both the start and timeout queues at once.
// Typically called after enabling interest or performing a reconciliation so that
// the next accrual cycle begins cleanly.
func (k Keeper) SafeEnqueueStart(ctx context.Context, vault *types.VaultAccount) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	currentBlockTime := sdkCtx.BlockTime().Unix()

	if err := k.DequeuePayoutTimeout(ctx, vault.PeriodTimeout, vault.GetAddress()); err != nil {
		return err
	}

	vault.PeriodStart = currentBlockTime
	vault.PeriodTimeout = 0
	if err := k.SetVaultAccount(sdkCtx, vault); err != nil {
		return err
	}

	return k.EnqueuePayoutVerification(ctx, vault.GetAddress())
}

// SafeEnqueueTimeout clears any existing start or timeout entries for the given vault,
// then enqueues a new timeout entry at the provided periodTimeout.
//
// This ensures a vault is never present in both the timeout and start queues at once.
// Typically called after marking a vault as payable, so it will be revisited only
// after the configured auto-reconcile timeout window expires.
func (k Keeper) SafeEnqueueTimeout(ctx context.Context, vault *types.VaultAccount) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	if err := k.DequeuePayoutTimeout(ctx, vault.PeriodTimeout, vault.GetAddress()); err != nil {
		return err
	}

	currentBlockTime := sdkCtx.BlockTime().Unix()
	vault.PeriodStart = currentBlockTime
	vault.PeriodTimeout = currentBlockTime + AutoReconcileTimeout
	if err := k.SetVaultAccount(sdkCtx, vault); err != nil {
		sdkCtx.Logger().Error("failed to set vault", "vault", vault.GetAddress().String(), "err", err)
		return err
	}
	return k.EnqueuePayoutTimeout(ctx, vault.PeriodTimeout, vault.GetAddress())
}
