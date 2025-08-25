package keeper

import (
	"context"

	"github.com/provlabs/vault/interest"
	"github.com/provlabs/vault/types"

	"cosmossdk.io/collections"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// AutoReconcileTimeout is the duration (in seconds) that a vault is considered
	// recently reconciled and is exempt from automatic interest checks.
	AutoReconcileTimeout = 20 * interest.SecondsPerHour
)

// EnqueuePayoutVerification schedules a vault for payout verification by inserting
// its address into the PayoutVerificationQueue (a set keyed only by vault address).
func (k Keeper) EnqueuePayoutVerification(ctx context.Context, vaultAddr sdk.AccAddress) error {
	return k.PayoutVerificationQueue.Set(ctx, vaultAddr, collections.NoValue{})
}

// EnqueuePayoutTimeout schedules a vault for timeout processing by inserting an
// entry into the PayoutTimeoutQueue keyed by (periodTimeout, vault address).
func (k Keeper) EnqueuePayoutTimeout(ctx context.Context, periodTimeout int64, vaultAddr sdk.AccAddress) error {
	return k.PayoutTimeoutQueue.Set(ctx, collections.Join(uint64(periodTimeout), vaultAddr), collections.NoValue{})
}

// DequeuePayoutVerification removes a vault from the PayoutVerificationQueue.
func (k Keeper) DequeuePayoutVerification(ctx context.Context, vaultAddr sdk.AccAddress) error {
	return k.PayoutVerificationQueue.Remove(ctx, vaultAddr)
}

// DequeuePayoutTimeout removes a specific timeout entry (periodTimeout, vault)
// from the PayoutTimeoutQueue.
func (k Keeper) DequeuePayoutTimeout(ctx context.Context, periodTimeout int64, vaultAddr sdk.AccAddress) error {
	return k.PayoutTimeoutQueue.Remove(ctx, collections.Join(uint64(periodTimeout), vaultAddr))
}

// WalkPayoutVerifications iterates over all entries in the PayoutVerificationQueue.
// For each entry, the provided callback is invoked. Iteration stops if the callback
// returns stop=true or an error.
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

// WalkDuePayoutTimeouts iterates over all entries in the PayoutTimeoutQueue with
// a timeout timestamp <= nowSec. For each due entry, the callback is invoked.
// Iteration stops when a key with time > nowSec is encountered (since keys are
// ordered) or when the callback returns stop=true or an error.
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

// RemoveAllPayoutTimeoutsForVault deletes all timeout entries in the
// PayoutTimeoutQueue for the given vault address.
func (k Keeper) RemoveAllPayoutTimeoutsForVault(ctx context.Context, vaultAddr sdk.AccAddress) error {
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
