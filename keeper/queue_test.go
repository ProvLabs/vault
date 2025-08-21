package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/provlabs/vault/utils"
	"github.com/provlabs/vault/utils/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnqueueDequeue_Start(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)

	addr := sdk.MustAccAddressFromBech32(utils.TestAddress().Bech32)
	ts := int64(100)

	require.NoError(t, k.EnqueueVaultStart(ctx, ts, addr), "expected enqueue start to succeed")

	it, err := k.VaultStartQueue.Iterate(ctx, nil)
	require.NoError(t, err, "expected no error iterating start queue")
	defer it.Close()

	found := false
	for ; it.Valid(); it.Next() {
		kv, err := it.KeyValue()
		require.NoError(t, err, "expected no error getting start queue key")
		if kv.Key.K1() == uint64(ts) && kv.Key.K2().Equals(addr) {
			found = true
			break
		}
	}
	require.True(t, found, "expected to find enqueued start entry in queue")

	require.NoError(t, k.DequeueVaultStart(ctx, ts, addr), "expected dequeue start to succeed")

	it2, err := k.VaultStartQueue.Iterate(ctx, nil)
	require.NoError(t, err, "expected no error iterating start queue after dequeue")
	defer it2.Close()
	require.False(t, it2.Valid(), "expected start queue to be empty after dequeue")
}

func TestEnqueueDequeue_Timeout(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)

	addr := sdk.MustAccAddressFromBech32(utils.TestAddress().Bech32)
	ts := int64(200)

	require.NoError(t, k.EnqueueVaultTimeout(ctx, ts, addr), "expected enqueue timeout to succeed")

	it, err := k.VaultTimeoutQueue.Iterate(ctx, nil)
	require.NoError(t, err, "expected no error iterating timeout queue")
	defer it.Close()

	found := false
	for ; it.Valid(); it.Next() {
		kv, err := it.KeyValue()
		require.NoError(t, err, "expected no error getting timeout queue key")
		if kv.Key.K1() == uint64(ts) && kv.Key.K2().Equals(addr) {
			found = true
			break
		}
	}
	require.True(t, found, "expected to find enqueued timeout entry in queue")

	require.NoError(t, k.DequeueVaultTimeout(ctx, ts, addr), "expected dequeue timeout to succeed")

	it2, err := k.VaultTimeoutQueue.Iterate(ctx, nil)
	require.NoError(t, err, "expected no error iterating timeout queue after dequeue")
	defer it2.Close()
	require.False(t, it2.Valid(), "expected timeout queue to be empty after dequeue")
}

func TestWalkDueStarts(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)

	a1 := sdk.MustAccAddressFromBech32(utils.TestAddress().Bech32)
	a2 := sdk.MustAccAddressFromBech32(utils.TestAddress().Bech32)

	require.NoError(t, k.EnqueueVaultStart(ctx, 100, a1), "expected enqueue start to succeed")
	require.NoError(t, k.EnqueueVaultStart(ctx, 150, a2), "expected enqueue start to succeed")
	require.NoError(t, k.EnqueueVaultStart(ctx, 300, a1), "expected enqueue start to succeed")

	var seen []uint64
	err := k.WalkDueStarts(ctx, 200, func(ts uint64, addr sdk.AccAddress) (bool, error) {
		seen = append(seen, ts)
		return false, nil
	})
	require.NoError(t, err, "expected walk due starts to complete without error")
	assert.ElementsMatch(t, []uint64{100, 150}, seen, "expected to see only entries <= 200")
}

func TestWalkDueStarts_StopEarly(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)

	a := sdk.MustAccAddressFromBech32(utils.TestAddress().Bech32)
	require.NoError(t, k.EnqueueVaultStart(ctx, 100, a), "expected enqueue start to succeed")
	require.NoError(t, k.EnqueueVaultStart(ctx, 150, a), "expected enqueue start to succeed")

	calls := 0
	err := k.WalkDueStarts(ctx, 200, func(ts uint64, addr sdk.AccAddress) (bool, error) {
		calls++
		return true, nil
	})
	require.NoError(t, err, "expected walk due starts to complete without error")
	assert.Equal(t, 1, calls, "expected walk to stop early after first callback")
}

func TestWalkDueTimeouts(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)

	a1 := sdk.MustAccAddressFromBech32(utils.TestAddress().Bech32)
	a2 := sdk.MustAccAddressFromBech32(utils.TestAddress().Bech32)

	require.NoError(t, k.EnqueueVaultTimeout(ctx, 50, a1), "expected enqueue timeout to succeed")
	require.NoError(t, k.EnqueueVaultTimeout(ctx, 75, a2), "expected enqueue timeout to succeed")
	require.NoError(t, k.EnqueueVaultTimeout(ctx, 500, a1), "expected enqueue timeout to succeed")

	var seen []uint64
	err := k.WalkDueTimeouts(ctx, 100, func(ts uint64, addr sdk.AccAddress) (bool, error) {
		seen = append(seen, ts)
		return false, nil
	})
	require.NoError(t, err, "expected walk due timeouts to complete without error")
	assert.ElementsMatch(t, []uint64{50, 75}, seen, "expected to see only timeout entries <= 100")
}

func TestWalkDueTimeouts_StopEarly(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)

	a := sdk.MustAccAddressFromBech32(utils.TestAddress().Bech32)
	require.NoError(t, k.EnqueueVaultTimeout(ctx, 10, a), "expected enqueue timeout to succeed")
	require.NoError(t, k.EnqueueVaultTimeout(ctx, 20, a), "expected enqueue timeout to succeed")

	calls := 0
	err := k.WalkDueTimeouts(ctx, 25, func(ts uint64, addr sdk.AccAddress) (bool, error) {
		calls++
		return true, nil
	})
	require.NoError(t, err, "expected walk due timeouts to complete without error")
	assert.Equal(t, 1, calls, "expected walk to stop early after first callback")
}

func TestRemoveAllStartsForVault(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)

	a1 := sdk.MustAccAddressFromBech32(utils.TestAddress().Bech32)
	a2 := sdk.MustAccAddressFromBech32(utils.TestAddress().Bech32)

	require.NoError(t, k.EnqueueVaultStart(ctx, 100, a1), "expected enqueue start to succeed")
	require.NoError(t, k.EnqueueVaultStart(ctx, 150, a1), "expected enqueue start to succeed")
	require.NoError(t, k.EnqueueVaultStart(ctx, 200, a2), "expected enqueue start to succeed")

	require.NoError(t, k.RemoveAllStartsForVault(ctx, a1), "expected remove all starts to succeed")

	it, err := k.VaultStartQueue.Iterate(ctx, nil)
	require.NoError(t, err, "expected no error iterating start queue after removal")
	defer it.Close()

	for ; it.Valid(); it.Next() {
		kv, err := it.KeyValue()
		require.NoError(t, err, "expected no error reading key")
		require.False(t, kv.Key.K2().Equals(a1), "expected no start queue entries for a1")
	}
}

func TestRemoveAllTimeoutsForVault(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)

	a1 := sdk.MustAccAddressFromBech32(utils.TestAddress().Bech32)
	a2 := sdk.MustAccAddressFromBech32(utils.TestAddress().Bech32)

	require.NoError(t, k.EnqueueVaultTimeout(ctx, 100, a1), "expected enqueue timeout to succeed")
	require.NoError(t, k.EnqueueVaultTimeout(ctx, 150, a1), "expected enqueue timeout to succeed")
	require.NoError(t, k.EnqueueVaultTimeout(ctx, 200, a2), "expected enqueue timeout to succeed")

	require.NoError(t, k.RemoveAllTimeoutsForVault(ctx, a1), "expected remove all timeouts to succeed")

	it, err := k.VaultTimeoutQueue.Iterate(ctx, nil)
	require.NoError(t, err, "expected no error iterating timeout queue after removal")
	defer it.Close()

	for ; it.Valid(); it.Next() {
		kv, err := it.KeyValue()
		require.NoError(t, err, "expected no error reading key")
		require.False(t, kv.Key.K2().Equals(a1), "expected no timeout queue entries for a1")
	}
}

func TestSafeEnqueueStart_ClearsTimeoutsAndSetsStart(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)

	addr := sdk.MustAccAddressFromBech32(utils.TestAddress().Bech32)
	other := sdk.MustAccAddressFromBech32(utils.TestAddress().Bech32)

	require.NoError(t, k.EnqueueVaultTimeout(ctx, 50, addr), "expected initial timeout enqueue for addr to succeed")
	require.NoError(t, k.EnqueueVaultTimeout(ctx, 60, other), "expected initial timeout enqueue for other to succeed")

	startTS := int64(100)
	require.NoError(t, k.SafeEnqueueStart(ctx, startTS, addr), "expected SafeEnqueueStart to succeed")

	itS, err := k.VaultStartQueue.Iterate(ctx, nil)
	require.NoError(t, err, "expected no error iterating start queue")
	defer itS.Close()

	foundStart := false
	for ; itS.Valid(); itS.Next() {
		kv, err := itS.KeyValue()
		require.NoError(t, err, "expected no error reading start queue key")
		if kv.Key.K2().Equals(addr) && kv.Key.K1() == uint64(startTS) {
			foundStart = true
		}
	}
	require.True(t, foundStart, "expected exactly one start entry for addr at requested timestamp")

	itT, err := k.VaultTimeoutQueue.Iterate(ctx, nil)
	require.NoError(t, err, "expected no error iterating timeout queue")
	defer itT.Close()

	for ; itT.Valid(); itT.Next() {
		kv, err := itT.KeyValue()
		require.NoError(t, err, "expected no error reading timeout queue key")
		require.False(t, kv.Key.K2().Equals(addr), "expected no remaining timeout entries for addr after SafeEnqueueStart")
	}
}

func TestSafeEnqueueTimeout_ClearsStartsAndSetsTimeout(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)

	addr := sdk.MustAccAddressFromBech32(utils.TestAddress().Bech32)
	other := sdk.MustAccAddressFromBech32(utils.TestAddress().Bech32)

	require.NoError(t, k.EnqueueVaultStart(ctx, 40, addr), "expected initial start enqueue for addr to succeed")
	require.NoError(t, k.EnqueueVaultStart(ctx, 45, other), "expected initial start enqueue for other to succeed")

	timeoutTS := int64(120)
	require.NoError(t, k.SafeEnqueueTimeout(ctx, timeoutTS, addr), "expected SafeEnqueueTimeout to succeed")

	itT, err := k.VaultTimeoutQueue.Iterate(ctx, nil)
	require.NoError(t, err, "expected no error iterating timeout queue")
	defer itT.Close()

	foundTimeout := false
	for ; itT.Valid(); itT.Next() {
		kv, err := itT.KeyValue()
		require.NoError(t, err, "expected no error reading timeout queue key")
		if kv.Key.K2().Equals(addr) && kv.Key.K1() == uint64(timeoutTS) {
			foundTimeout = true
		}
	}
	require.True(t, foundTimeout, "expected exactly one timeout entry for addr at requested timestamp")

	itS, err := k.VaultStartQueue.Iterate(ctx, nil)
	require.NoError(t, err, "expected no error iterating start queue")
	defer itS.Close()

	for ; itS.Valid(); itS.Next() {
		kv, err := itS.KeyValue()
		require.NoError(t, err, "expected no error reading start queue key")
		require.False(t, kv.Key.K2().Equals(addr), "expected no remaining start entries for addr after SafeEnqueueTimeout")
	}
}
