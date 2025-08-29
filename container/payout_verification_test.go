package container_test

import (
	"errors"
	"testing"

	"cosmossdk.io/collections"
	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/provlabs/vault/container"
	"github.com/provlabs/vault/types"
	"github.com/provlabs/vault/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestPayoutVerificationQueue(t *testing.T) (sdk.Context, *container.PayoutVerificationQueue) {
	t.Helper()
	storeKey := storetypes.NewKVStoreKey(types.ModuleName)
	testCtx := testutil.DefaultContextWithDB(t, storeKey, storetypes.NewTransientStoreKey("transient_test"))
	kvStoreService := runtime.NewKVStoreService(storeKey)
	sb := collections.NewSchemaBuilder(kvStoreService)
	q := container.NewPayoutVerificationQueue(sb)
	_, err := sb.Build()
	require.NoError(t, err)
	return testCtx.Ctx.WithLogger(log.NewNopLogger()), q
}

func TestEnqueueDequeue(t *testing.T) {
	ctx, q := newTestPayoutVerificationQueue(t)

	addr := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)

	require.NoError(t, q.Enqueue(ctx, addr), "enqueue payout verification for %s should succeed", addr.String())

	found := false
	err := q.Walk(ctx, func(address sdk.AccAddress) (bool, error) {
		if address.Equals(addr) {
			found = true
			return true, nil // stop walking
		}
		return false, nil
	})
	require.NoError(t, err)
	require.True(t, found, "expected to find %s in payout verification queue after enqueue", addr.String())

	require.NoError(t, q.Dequeue(ctx, addr), "dequeue payout verification for %s should succeed", addr.String())

	found = false
	err = q.Walk(ctx, func(address sdk.AccAddress) (bool, error) {
		found = true
		return true, nil // stop walking
	})
	require.NoError(t, err)
	require.False(t, found, "payout verification queue should be empty after dequeue of %s", addr.String())
}

func TestWalk(t *testing.T) {
	ctx, q := newTestPayoutVerificationQueue(t)

	a1 := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)
	a2 := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)

	require.NoError(t, q.Enqueue(ctx, a1), "enqueue payout verification for a1 should succeed")
	require.NoError(t, q.Enqueue(ctx, a2), "enqueue payout verification for a2 should succeed")
	require.NoError(t, q.Enqueue(ctx, a1), "enqueue payout verification duplicate for a1 should succeed (set semantics)")

	var seen []string
	require.NoError(t, q.Walk(ctx, func(addr sdk.AccAddress) (bool, error) {
		seen = append(seen, addr.String())
		return false, nil
	}), "walking payout verifications should not error")
	assert.ElementsMatch(t, []string{a1.String(), a2.String()}, seen, "walk should visit exactly the queued payout verification addresses")
}

func TestWalk_StopEarly(t *testing.T) {
	ctx, q := newTestPayoutVerificationQueue(t)

	a := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)
	b := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)

	require.NoError(t, q.Enqueue(ctx, a), "enqueue payout verification for a should succeed")
	require.NoError(t, q.Enqueue(ctx, b), "enqueue payout verification for b should succeed")

	calls := 0
	require.NoError(t, q.Walk(ctx, func(_ sdk.AccAddress) (bool, error) {
		calls++
		return true, nil
	}), "walking payout verifications (stop early) should not error")
	assert.Equal(t, 1, calls, "walk should stop after first callback; got %d calls", calls)
}

func TestWalk_ErrorPropagates(t *testing.T) {
	ctx, q := newTestPayoutVerificationQueue(t)

	a := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)
	require.NoError(t, q.Enqueue(ctx, a), "enqueue payout verification for a should succeed")

	errBoom := errors.New("boom")
	err := q.Walk(ctx, func(_ sdk.AccAddress) (bool, error) {
		return false, errBoom
	})
	require.ErrorIs(t, err, errBoom, "walk should propagate callback error")
}

func TestRemove(t *testing.T) {
	ctx, q := newTestPayoutVerificationQueue(t)

	a1 := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)
	a2 := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)

	require.NoError(t, q.Enqueue(ctx, a1), "enqueue payout verification for a1 should succeed")
	require.NoError(t, q.Enqueue(ctx, a2), "enqueue payout verification for a2 should succeed")

	require.NoError(t, q.Dequeue(ctx, a1), "dequeue payout verification for a1 should succeed")

	var seen []string
	err := q.Walk(ctx, func(address sdk.AccAddress) (bool, error) {
		seen = append(seen, address.String())
		return false, nil
	})
	require.NoError(t, err)
	assert.Len(t, seen, 1)
	assert.Equal(t, a2.String(), seen[0])
}
