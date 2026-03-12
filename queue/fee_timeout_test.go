package queue_test

import (
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

func newTestFeeTimeoutQueue(t *testing.T) (sdk.Context, *queue.FeeTimeoutQueue) {
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
	q := queue.NewFeeTimeoutQueue(sb)
	_, err := sb.Build()
	require.NoError(t, err, "schema build should not error")
	return testCtx.Ctx.WithLogger(log.NewNopLogger()), q
}

func TestFeeTimeoutQueueEnqueueDequeue(t *testing.T) {
	ctx, q := newTestFeeTimeoutQueue(t)

	addr := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)
	ts := int64(200)

	require.EqualError(t, q.Enqueue(ctx, -1, addr), "feeTimeout cannot be negative", "enqueue with negative timestamp should return specific error")
	require.NoError(t, q.Enqueue(ctx, ts, addr), "enqueue fee timeout (%d) for %s should succeed", ts, addr.String())

	found := false
	err := q.Walk(ctx, func(timestamp uint64, address sdk.AccAddress) (bool, error) {
		if timestamp == uint64(ts) && address.Equals(addr) {
			found = true
			return true, nil
		}
		return false, nil
	})
	require.NoError(t, err, "walking the fee timeout queue should not error")
	require.True(t, found, "expected to find timeout (%d) for %s in fee timeout queue after enqueue", ts, addr.String())

	require.EqualError(t, q.Dequeue(ctx, -1, addr), "feeTimeout cannot be negative", "dequeue with negative timestamp should return specific error")
	require.NoError(t, q.Dequeue(ctx, ts, addr), "dequeue fee timeout (%d) for %s should succeed", ts, addr.String())

	found = false
	err = q.Walk(ctx, func(timestamp uint64, address sdk.AccAddress) (bool, error) {
		found = true
		return true, nil
	})
	require.NoError(t, err, "walking the fee timeout queue after dequeue should not error")
	require.False(t, found, "fee timeout queue should be empty after dequeue")
}

func TestFeeTimeoutQueueWalkDueTimeouts(t *testing.T) {
	ctx, q := newTestFeeTimeoutQueue(t)

	a1 := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)
	a2 := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)

	require.NoError(t, q.Enqueue(ctx, 50, a1), "enqueue fee timeout (50) for a1 should succeed")
	require.NoError(t, q.Enqueue(ctx, 75, a2), "enqueue fee timeout (75) for a2 should succeed")
	require.NoError(t, q.Enqueue(ctx, 500, a1), "enqueue fee timeout (500) for a1 should succeed")

	var seen []uint64
	require.NoError(t, q.WalkDue(ctx, 100, func(ts uint64, _ sdk.AccAddress) (bool, error) {
		seen = append(seen, ts)
		return false, nil
	}), "walking due fee timeouts <= 100 should not error")
	assert.ElementsMatch(t, []uint64{50, 75}, seen, "walk should visit exactly timeouts <= 100; got %v", seen)
}

func TestFeeTimeoutQueueRemoveAllTimeoutsForVault(t *testing.T) {
	ctx, q := newTestFeeTimeoutQueue(t)

	a1 := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)
	a2 := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)

	require.NoError(t, q.Enqueue(ctx, 100, a1), "enqueue fee timeout (100) for a1 should succeed")
	require.NoError(t, q.Enqueue(ctx, 150, a1), "enqueue fee timeout (150) for a1 should succeed")
	require.NoError(t, q.Enqueue(ctx, 200, a2), "enqueue fee timeout (200) for a2 should succeed")

	require.NoError(t, q.RemoveAllForVault(ctx, a1), "remove all fee timeouts for a1 should succeed")

	err := q.Walk(ctx, func(timestamp uint64, address sdk.AccAddress) (bool, error) {
		require.False(t, address.Equals(a1), "fee timeout queue should not include any entries for a1 after removal")
		return false, nil
	})
	require.NoError(t, err, "walking the fee timeout queue after removal should not error")
}
