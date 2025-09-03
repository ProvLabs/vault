package queue_test

import (
	"errors"
	"testing"

	"cosmossdk.io/collections"
	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/provlabs/vault/queue"
	"github.com/provlabs/vault/types"
	"github.com/provlabs/vault/utils"
	"github.com/provlabs/vault/utils/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestPayoutTimeoutQueue(t *testing.T) (sdk.Context, *queue.PayoutTimeoutQueue) {
	t.Helper()
	storeKey := storetypes.NewKVStoreKey(types.ModuleName)
	testCtx := testutil.DefaultContextWithDB(t, storeKey, storetypes.NewTransientStoreKey("transient_test"))

	cfg := mocks.MakeTestEncodingConfig("provlabs")
	sdkCfg := sdk.GetConfig()
	sdkCfg.SetBech32PrefixForAccount("provlabs", "provlabspub")
	sdkCfg.SetBech32PrefixForValidator("provlabsvaloper", "provlabsvaloperpub")
	sdkCfg.SetBech32PrefixForConsensusNode("provlabsvalcons", "provlabsvalconspub")
	types.RegisterInterfaces(cfg.InterfaceRegistry)

	kvStoreService := runtime.NewKVStoreService(storeKey)
	sb := collections.NewSchemaBuilder(kvStoreService)
	q := queue.NewPayoutTimeoutQueue(sb)
	_, err := sb.Build()
	require.NoError(t, err)
	return testCtx.Ctx.WithLogger(log.NewNopLogger()), q
}

func TestPayoutTimeoutQueueEnqueueDequeue(t *testing.T) {
	ctx, q := newTestPayoutTimeoutQueue(t)

	addr := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)
	ts := int64(200)

	require.EqualError(t, q.Enqueue(ctx, -1, addr), "periodTimeout cannot be negative", addr.String())
	require.NoError(t, q.Enqueue(ctx, ts, addr), "enqueue payout timeout (%d) for %s should succeed", ts, addr.String())

	found := false
	err := q.Walk(ctx, func(timestamp uint64, address sdk.AccAddress) (bool, error) {
		if timestamp == uint64(ts) && address.Equals(addr) {
			found = true
			return true, nil // stop walking
		}
		return false, nil
	})
	require.NoError(t, err)
	require.True(t, found, "expected to find timeout (%d) for %s in payout timeout queue after enqueue", ts, addr.String())

	require.EqualError(t, q.Dequeue(ctx, -1, addr), "periodTimeout cannot be negative", ts, addr.String())
	require.NoError(t, q.Dequeue(ctx, ts, addr), "dequeue payout timeout (%d) for %s should succeed", ts, addr.String())

	found = false
	err = q.Walk(ctx, func(timestamp uint64, address sdk.AccAddress) (bool, error) {
		found = true
		return true, nil // stop walking
	})
	require.NoError(t, err)
	require.False(t, found, "payout timeout queue should be empty after dequeue")
}

func TestPayoutTimeoutQueueEnqueueDequeue_Timeout(t *testing.T) {
	ctx, q := newTestPayoutTimeoutQueue(t)

	addr := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)
	ts := int64(200)

	require.NoError(t, q.Enqueue(ctx, ts, addr), "enqueue payout timeout (%d) for %s should succeed", ts, addr.String())

	found := false
	err := q.Walk(ctx, func(timestamp uint64, address sdk.AccAddress) (bool, error) {
		if timestamp == uint64(ts) && address.Equals(addr) {
			found = true
			return true, nil // stop walking
		}
		return false, nil
	})
	require.NoError(t, err)
	require.True(t, found, "expected to find timeout (%d) for %s in payout timeout queue after enqueue", ts, addr.String())

	require.NoError(t, q.Dequeue(ctx, ts, addr), "dequeue payout timeout (%d) for %s should succeed", ts, addr.String())

	found = false
	err = q.Walk(ctx, func(timestamp uint64, address sdk.AccAddress) (bool, error) {
		found = true
		return true, nil // stop walking
	})
	require.NoError(t, err)
	require.False(t, found, "payout timeout queue should be empty after dequeue")
}

func TestPayoutTimeoutQueueWalkDueTimeouts(t *testing.T) {
	ctx, q := newTestPayoutTimeoutQueue(t)

	a1 := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)
	a2 := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)

	require.NoError(t, q.Enqueue(ctx, 50, a1), "enqueue payout timeout (50) for a1 should succeed")
	require.NoError(t, q.Enqueue(ctx, 75, a2), "enqueue payout timeout (75) for a2 should succeed")
	require.NoError(t, q.Enqueue(ctx, 500, a1), "enqueue payout timeout (500) for a1 should succeed")

	var seen []uint64
	require.NoError(t, q.WalkDue(ctx, 100, func(ts uint64, _ sdk.AccAddress) (bool, error) {
		seen = append(seen, ts)
		return false, nil
	}), "walking due payout timeouts <= 100 should not error")
	assert.ElementsMatch(t, []uint64{50, 75}, seen, "walk should visit exactly timeouts <= 100; got %v", seen)
}

func TestPayoutTimeoutQueueWalkDueTimeouts_ErrorPropagates(t *testing.T) {
	ctx, q := newTestPayoutTimeoutQueue(t)

	a := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)
	require.NoError(t, q.Enqueue(ctx, 10, a), "enqueue payout timeout (10) should succeed")

	errBoom := errors.New("boom")
	err := q.WalkDue(ctx, 25, func(_ uint64, _ sdk.AccAddress) (bool, error) {
		return false, errBoom
	})
	require.ErrorIs(t, err, errBoom, "walk should propagate callback error")
}

func TestPayoutTimeoutQueueWalkDueTimeouts_StopEarly(t *testing.T) {
	ctx, q := newTestPayoutTimeoutQueue(t)

	a := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)
	require.NoError(t, q.Enqueue(ctx, 10, a), "enqueue payout timeout (10) should succeed")
	require.NoError(t, q.Enqueue(ctx, 20, a), "enqueue payout timeout (20) should succeed")

	calls := 0
	require.NoError(t, q.WalkDue(ctx, 25, func(_ uint64, _ sdk.AccAddress) (bool, error) {
		calls++
		return true, nil
	}), "walking due payout timeouts (stop early) should not error")
	assert.Equal(t, 1, calls, "walk should stop after first callback; got %d calls", calls)
}

func TestPayoutTimeoutQueueRemoveAllTimeoutsForVault(t *testing.T) {
	ctx, q := newTestPayoutTimeoutQueue(t)

	a1 := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)
	a2 := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)

	require.NoError(t, q.Enqueue(ctx, 100, a1), "enqueue payout timeout (100) for a1 should succeed")
	require.NoError(t, q.Enqueue(ctx, 150, a1), "enqueue payout timeout (150) for a1 should succeed")
	require.NoError(t, q.Enqueue(ctx, 200, a2), "enqueue payout timeout (200) for a2 should succeed")

	require.NoError(t, q.RemoveAllForVault(ctx, a1), "remove all timeouts for a1 should succeed")

	err := q.Walk(ctx, func(timestamp uint64, address sdk.AccAddress) (bool, error) {
		require.False(t, address.Equals(a1), "payout timeout queue should not include any entries for a1 after removal")
		return false, nil
	})
	require.NoError(t, err)
}
