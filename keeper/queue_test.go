package keeper_test

import (
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"

	kpr "github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/types"
	"github.com/provlabs/vault/utils"
	"github.com/provlabs/vault/utils/mocks"
)

// newQueueTestVault builds a minimal valid vault with zeroed accrual periods for each test to set.
func newQueueTestVault(shareDenom string) *types.VaultAccount {
	vaultAddr := types.GetVaultAddress(shareDenom)
	return &types.VaultAccount{
		BaseAccount:         authtypes.NewBaseAccountWithAddress(vaultAddr),
		Admin:               utils.TestProvlabsAddress().Bech32,
		TotalShares:         sdk.NewInt64Coin(shareDenom, 0),
		UnderlyingAsset:     "under",
		PaymentDenom:        "under",
		CurrentInterestRate: types.ZeroInterestRate,
		DesiredInterestRate: types.ZeroInterestRate,
		OutstandingAumFee:   sdk.NewInt64Coin("under", 0),
	}
}

// timeoutQueueVariant adapts the payout and fee timeout queues to a common shape for shared tests.
type timeoutQueueVariant struct {
	name       string
	shareDenom string
	setTimeout func(vault *types.VaultAccount, timeout int64)
	timeoutOf  func(vault *types.VaultAccount) int64
	enqueue    func(k *kpr.Keeper, ctx sdk.Context, timeout int64, vaultAddr sdk.AccAddress) error
	reschedule func(k *kpr.Keeper, ctx sdk.Context, vault *types.VaultAccount, oldTimeout int64) error
	timeoutsOf func(k *kpr.Keeper, ctx sdk.Context, vaultAddr sdk.AccAddress) ([]uint64, error)
}

// timeoutQueueVariants covers both timeout queues, whose reschedule paths must stay identical.
var timeoutQueueVariants = []timeoutQueueVariant{
	{
		name:       "payout timeout",
		shareDenom: "reschedulepayoutshares",
		setTimeout: func(vault *types.VaultAccount, timeout int64) { vault.PeriodTimeout = timeout },
		timeoutOf:  func(vault *types.VaultAccount) int64 { return vault.PeriodTimeout },
		enqueue: func(k *kpr.Keeper, ctx sdk.Context, timeout int64, vaultAddr sdk.AccAddress) error {
			return k.PayoutTimeoutQueue.Enqueue(ctx, timeout, vaultAddr)
		},
		reschedule: func(k *kpr.Keeper, ctx sdk.Context, vault *types.VaultAccount, oldTimeout int64) error {
			return k.ReschedulePayoutTimeout(ctx, vault, oldTimeout)
		},
		timeoutsOf: func(k *kpr.Keeper, ctx sdk.Context, vaultAddr sdk.AccAddress) ([]uint64, error) {
			var timeouts []uint64
			err := k.PayoutTimeoutQueue.Walk(ctx, func(timeout uint64, walked sdk.AccAddress) (bool, error) {
				if walked.Equals(vaultAddr) {
					timeouts = append(timeouts, timeout)
				}
				return false, nil
			})
			return timeouts, err
		},
	},
	{
		name:       "fee timeout",
		shareDenom: "reschedulefeeshares",
		setTimeout: func(vault *types.VaultAccount, timeout int64) { vault.FeePeriodTimeout = timeout },
		timeoutOf:  func(vault *types.VaultAccount) int64 { return vault.FeePeriodTimeout },
		enqueue: func(k *kpr.Keeper, ctx sdk.Context, timeout int64, vaultAddr sdk.AccAddress) error {
			return k.FeeTimeoutQueue.Enqueue(ctx, timeout, vaultAddr)
		},
		reschedule: func(k *kpr.Keeper, ctx sdk.Context, vault *types.VaultAccount, oldTimeout int64) error {
			return k.RescheduleFeeTimeout(ctx, vault, oldTimeout)
		},
		timeoutsOf: func(k *kpr.Keeper, ctx sdk.Context, vaultAddr sdk.AccAddress) ([]uint64, error) {
			var timeouts []uint64
			err := k.FeeTimeoutQueue.Walk(ctx, func(timeout uint64, walked sdk.AccAddress) (bool, error) {
				if walked.Equals(vaultAddr) {
					timeouts = append(timeouts, timeout)
				}
				return false, nil
			})
			return timeouts, err
		},
	},
}

func TestSafeAddPayoutVerification_UpdatesVaultAndQueues(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)

	share := "vaultshares"
	vaultAddr := types.GetVaultAddress(share)

	v := newQueueTestVault(share)
	v.PeriodTimeout = 50

	require.NoError(t, k.PayoutTimeoutQueue.Enqueue(ctx, 50, vaultAddr), "precondition: enqueue payout timeout (50) for vault should succeed")

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockTime(time.Unix(1000, 0))

	require.NoError(t, k.SafeAddPayoutVerification(ctx, v), "SafeAddPayoutVerification should clear timeouts, set start, and enqueue verification")

	itS, err := k.PayoutVerificationSet.Iterate(ctx, nil)
	require.NoError(t, err, "iterate payout verification queue should not error after SafeAddPayoutVerification")
	defer itS.Close()

	foundStart := false
	for ; itS.Valid(); itS.Next() {
		kv, err := itS.Key()
		require.NoError(t, err, "reading key/value from payout verification iterator should not error")
		if kv.Equals(vaultAddr) {
			foundStart = true
		}
	}
	require.True(t, foundStart, "payout verification queue should contain vault %s after SafeAddPayoutVerification", vaultAddr.String())

	err = k.PayoutTimeoutQueue.Walk(ctx, func(timestamp uint64, address sdk.AccAddress) (bool, error) {
		require.False(t, address.Equals(vaultAddr), "payout timeout queue should not contain vault %s after SafeAddPayoutVerification", vaultAddr.String())
		return false, nil
	})
	require.NoError(t, err, "walk payout timeout queue should not error after SafeAddPayoutVerification")

	acc := k.AuthKeeper.GetAccount(ctx, vaultAddr)
	require.NotNil(t, acc, "vault account should exist in state after SafeAddPayoutVerification")
	va, ok := acc.(*types.VaultAccount)
	require.True(t, ok, "retrieved account should be *types.VaultAccount; got %T", acc)
	require.Equal(t, int64(1000), va.PeriodStart, "vault PeriodStart should be set to block time; expected 1000, got %d", va.PeriodStart)
	require.Equal(t, int64(0), va.PeriodTimeout, "vault PeriodTimeout should be cleared; expected 0, got %d", va.PeriodTimeout)
}

func TestSafeEnqueuePayoutTimeout_UpdatesVaultAndQueues(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)

	share := "vaultshares2"
	vaultAddr := types.GetVaultAddress(share)

	v := newQueueTestVault(share)
	v.PeriodTimeout = 30

	require.NoError(t, k.PayoutTimeoutQueue.Enqueue(ctx, 30, vaultAddr), "precondition: enqueue payout timeout (30) for vault should succeed")

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	now := time.Unix(2000, 0)
	ctx = sdkCtx.WithBlockTime(now)

	require.NoError(t, k.SafeEnqueuePayoutTimeout(ctx, v), "SafeEnqueuePayoutTimeout should clear previous timeouts, set new timeout, and enqueue timeout entry")

	expectTimeout := uint64(now.Unix() + kpr.AutoReconcileTimeout)

	var times []uint64
	err := k.PayoutTimeoutQueue.Walk(ctx, func(timestamp uint64, address sdk.AccAddress) (bool, error) {
		if address.Equals(vaultAddr) {
			times = append(times, timestamp)
		}
		return false, nil
	})
	require.NoError(t, err, "walk timeout queue should not error after SafeEnqueuePayoutTimeout")
	require.ElementsMatch(t, []uint64{expectTimeout}, times, "timeout queue should include exactly one entry for vault at %d; got %v", expectTimeout, times)

	acc := k.AuthKeeper.GetAccount(ctx, vaultAddr)
	require.NotNil(t, acc, "vault account should exist in state after SafeEnqueuePayoutTimeout")
	va, ok := acc.(*types.VaultAccount)
	require.True(t, ok, "retrieved account should be *types.VaultAccount; got %T", acc)
	require.Equal(t, now.Unix(), va.PeriodStart, "vault PeriodStart should equal block time; expected %d, got %d", now.Unix(), va.PeriodStart)
	require.Equal(t, int64(expectTimeout), va.PeriodTimeout, "vault PeriodTimeout should equal expected timeout; expected %d, got %d", expectTimeout, va.PeriodTimeout)
}

func TestSafeEnqueueFeeTimeout_UpdatesVaultAndQueues(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)

	share := "vaultshares3"
	vaultAddr := types.GetVaultAddress(share)

	v := newQueueTestVault(share)
	v.FeePeriodTimeout = 30

	require.NoError(t, k.FeeTimeoutQueue.Enqueue(ctx, 30, vaultAddr), "precondition: enqueue fee timeout (30) should succeed")

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	now := time.Unix(3000, 0)
	ctx = sdkCtx.WithBlockTime(now)

	require.NoError(t, k.SafeEnqueueFeeTimeout(ctx, v), "SafeEnqueueFeeTimeout should clear timeouts and enqueue new entry")

	expectTimeout := uint64(now.Unix() + kpr.AutoReconcileTimeout)

	var times []uint64
	err := k.FeeTimeoutQueue.Walk(ctx, func(timestamp uint64, address sdk.AccAddress) (bool, error) {
		if address.Equals(vaultAddr) {
			times = append(times, timestamp)
		}
		return false, nil
	})
	require.NoError(t, err, "walk fee timeout queue should not error")
	require.ElementsMatch(t, []uint64{expectTimeout}, times, "fee timeout queue should contain exactly the expected timeout; got %v", times)

	acc := k.AuthKeeper.GetAccount(ctx, vaultAddr)
	require.NotNil(t, acc, "vault account should exist after SafeEnqueueFeeTimeout")
	va, ok := acc.(*types.VaultAccount)
	require.True(t, ok, "retrieved account should be *types.VaultAccount; got %T", acc)
	require.Equal(t, now.Unix(), va.FeePeriodStart, "fee period start should be set to now")
	require.Equal(t, int64(expectTimeout), va.FeePeriodTimeout, "fee period timeout mismatch; expected %d, got %d", expectTimeout, va.FeePeriodTimeout)
}

func TestRescheduleTimeout_MovesEntryToNextWindow(t *testing.T) {
	const oldTimeout = int64(40)

	for _, variant := range timeoutQueueVariants {
		t.Run(variant.name+": valid vault leaves the old key and is filed at the next window", func(t *testing.T) {
			ctx, k := mocks.NewVaultKeeper(t)
			now := time.Unix(5_000, 0)
			ctx = ctx.WithBlockTime(now)

			vault := newQueueTestVault(variant.shareDenom)
			vaultAddr := vault.GetAddress()
			variant.setTimeout(vault, oldTimeout)
			require.NoError(t, k.SetVaultAccount(ctx, vault), "precondition: persisting the valid vault should succeed")
			require.NoError(t, variant.enqueue(k, ctx, oldTimeout, vaultAddr), "precondition: enqueueing the old %s (%d) should succeed", variant.name, oldTimeout)

			require.NoError(t, variant.reschedule(k, ctx, vault, oldTimeout), "rescheduling the %s for a valid vault should succeed", variant.name)

			expectTimeout := now.Unix() + kpr.AutoReconcileTimeout
			timeouts, err := variant.timeoutsOf(k, ctx, vaultAddr)
			require.NoError(t, err, "walking the %s queue should not error after a successful reschedule", variant.name)
			require.ElementsMatch(t, []uint64{uint64(expectTimeout)}, timeouts, "%s queue should hold exactly the new key %d for vault %s; got %v", variant.name, expectTimeout, vaultAddr, timeouts)
			require.Equal(t, expectTimeout, variant.timeoutOf(vault), "caller's vault should carry the new %s; expected %d, got %d", variant.name, expectTimeout, variant.timeoutOf(vault))

			stored, err := k.GetVault(ctx, vaultAddr)
			require.NoError(t, err, "loading vault %s after a successful reschedule should not error", vaultAddr)
			require.Equal(t, expectTimeout, variant.timeoutOf(stored), "persisted vault should carry the new %s; expected %d, got %d", variant.name, expectTimeout, variant.timeoutOf(stored))
		})
	}
}

func TestRescheduleTimeout_PersistFailureLeavesEntryUnderOldKey(t *testing.T) {
	const oldTimeout = int64(40)
	const rateAboveMaxMagnitude = "200.0"

	for _, variant := range timeoutQueueVariants {
		t.Run(variant.name+": vault that fails validation stays queued under its old key", func(t *testing.T) {
			ctx, k := mocks.NewVaultKeeper(t)
			ctx = ctx.WithBlockTime(time.Unix(5_000, 0))

			vault := newQueueTestVault(variant.shareDenom)
			vaultAddr := vault.GetAddress()
			variant.setTimeout(vault, oldTimeout)
			vault.CurrentInterestRate = rateAboveMaxMagnitude
			k.AuthKeeper.SetAccount(ctx, vault)
			require.NoError(t, variant.enqueue(k, ctx, oldTimeout, vaultAddr), "precondition: enqueueing the old %s (%d) should succeed", variant.name, oldTimeout)

			err := variant.reschedule(k, ctx, vault, oldTimeout)
			require.Error(t, err, "rescheduling the %s should fail for a vault whose interest rate exceeds the maximum magnitude", variant.name)

			timeouts, err := variant.timeoutsOf(k, ctx, vaultAddr)
			require.NoError(t, err, "walking the %s queue should not error after a failed reschedule", variant.name)
			require.ElementsMatch(t, []uint64{uint64(oldTimeout)}, timeouts, "%s queue should still hold only the old key %d for vault %s so the blocker revisits it; got %v", variant.name, oldTimeout, vaultAddr, timeouts)
			require.Equal(t, oldTimeout, variant.timeoutOf(vault), "caller's vault should keep its old %s after a failed reschedule; expected %d, got %d", variant.name, oldTimeout, variant.timeoutOf(vault))

			stored := k.AuthKeeper.GetAccount(ctx, vaultAddr)
			require.NotNil(t, stored, "vault account %s should still exist after a failed reschedule", vaultAddr)
			storedVault, ok := stored.(*types.VaultAccount)
			require.True(t, ok, "stored account should be *types.VaultAccount; got %T", stored)
			require.Equal(t, oldTimeout, variant.timeoutOf(storedVault), "persisted vault should keep its old %s after a failed reschedule; expected %d, got %d", variant.name, oldTimeout, variant.timeoutOf(storedVault))
		})
	}
}
