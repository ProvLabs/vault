package container_test

import (
	"errors"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/provlabs/vault/utils"
	"github.com/provlabs/vault/utils/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnqueueDequeue_Timeout(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)

	addr := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)
	ts := int64(200)

	require.NoError(t, k.NewPayoutTimeoutQueue.Enqueue(ctx, ts, addr), "enqueue payout timeout (%d) for %s should succeed", ts, addr.String())

	it, err := k.NewPayoutTimeoutQueue.Iterate(ctx, nil)
	require.NoError(t, err, "iterate payout timeout queue should not error")
	defer it.Close()

	found := false
	for ; it.Valid(); it.Next() {
		kv, err := it.Key()
		require.NoError(t, err, "reading key/value from payout timeout iterator should not error")
		if kv.K1() == uint64(ts) && kv.K2().Equals(addr) {
			found = true
			break
		}
	}
	require.True(t, found, "expected to find timeout (%d) for %s in payout timeout queue after enqueue", ts, addr.String())

	require.NoError(t, k.NewPayoutTimeoutQueue.Dequeue(ctx, ts, addr), "dequeue payout timeout (%d) for %s should succeed", ts, addr.String())

	it2, err := k.NewPayoutTimeoutQueue.Iterate(ctx, nil)
	require.NoError(t, err, "iterate payout timeout queue after dequeue should not error")
	defer it2.Close()
	require.False(t, it2.Valid(), "payout timeout queue should be empty after dequeue of (%d, %s)", ts, addr.String())
}

func TestWalkDueTimeouts(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)

	a1 := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)
	a2 := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)

	require.NoError(t, k.NewPayoutTimeoutQueue.Enqueue(ctx, 50, a1), "enqueue payout timeout (50) for a1 should succeed")
	require.NoError(t, k.NewPayoutTimeoutQueue.Enqueue(ctx, 75, a2), "enqueue payout timeout (75) for a2 should succeed")
	require.NoError(t, k.NewPayoutTimeoutQueue.Enqueue(ctx, 500, a1), "enqueue payout timeout (500) for a1 should succeed")

	var seen []uint64
	require.NoError(t, k.NewPayoutTimeoutQueue.WalkDue(ctx, 100, func(ts uint64, _ sdk.AccAddress) (bool, error) {
		seen = append(seen, ts)
		return false, nil
	}), "walking due payout timeouts <= 100 should not error")
	assert.ElementsMatch(t, []uint64{50, 75}, seen, "walk should visit exactly timeouts <= 100; got %v", seen)
}

func TestWalkDueTimeouts_ErrorPropagates(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)

	a := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)
	require.NoError(t, k.NewPayoutTimeoutQueue.Enqueue(ctx, 10, a), "enqueue payout timeout (10) should succeed")

	errBoom := errors.New("boom")
	err := k.NewPayoutTimeoutQueue.WalkDue(ctx, 25, func(_ uint64, _ sdk.AccAddress) (bool, error) {
		return false, errBoom
	})
	require.ErrorIs(t, err, errBoom, "walk should propagate callback error")
}

func TestWalkDueTimeouts_StopEarly(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)

	a := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)
	require.NoError(t, k.NewPayoutTimeoutQueue.Enqueue(ctx, 10, a), "enqueue payout timeout (10) should succeed")
	require.NoError(t, k.NewPayoutTimeoutQueue.Enqueue(ctx, 20, a), "enqueue payout timeout (20) should succeed")

	calls := 0
	require.NoError(t, k.NewPayoutTimeoutQueue.WalkDue(ctx, 25, func(_ uint64, _ sdk.AccAddress) (bool, error) {
		calls++
		return true, nil
	}), "walking due payout timeouts (stop early) should not error")
	assert.Equal(t, 1, calls, "walk should stop after first callback; got %d calls", calls)
}

func TestRemoveAllTimeoutsForVault(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)
	stdCtx := sdk.WrapSDKContext(ctx)

	a1 := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)
	a2 := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)

	require.NoError(t, k.NewPayoutTimeoutQueue.Enqueue(stdCtx, 100, a1), "enqueue payout timeout (100) for a1 should succeed")
	require.NoError(t, k.NewPayoutTimeoutQueue.Enqueue(stdCtx, 150, a1), "enqueue payout timeout (150) for a1 should succeed")
	require.NoError(t, k.NewPayoutTimeoutQueue.Enqueue(stdCtx, 200, a2), "enqueue payout timeout (200) for a2 should succeed")

	require.NoError(t, k.NewPayoutTimeoutQueue.RemoveAllForVault(stdCtx, a1), "remove all timeouts for a1 should succeed")

	it, err := k.NewPayoutTimeoutQueue.Iterate(stdCtx, nil)
	require.NoError(t, err, "iterate payout timeout queue after removal should not error")
	defer it.Close()

	for ; it.Valid(); it.Next() {
		kv, err := it.Key()
		require.NoError(t, err, "reading key/value from payout timeout iterator should not error")
		require.False(t, kv.K2().Equals(a1), "payout timeout queue should not include any entries for a1 after removal")
	}
}
