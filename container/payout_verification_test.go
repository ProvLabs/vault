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

func TestEnqueueDequeue(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)

	addr := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)

	require.NoError(t, k.NewPayoutVerificationQueue.Enqueue(ctx, addr), "enqueue payout verification for %s should succeed", addr.String())

	it, err := k.NewPayoutVerificationQueue.Iterate(ctx, nil)
	require.NoError(t, err, "iterate payout verification queue should not error")
	defer it.Close()

	found := false
	for ; it.Valid(); it.Next() {
		kv, err := it.Key()
		require.NoError(t, err, "reading key/value from payout verification iterator should not error")
		if kv.Equals(addr) {
			found = true
			break
		}
	}
	require.True(t, found, "expected to find %s in payout verification queue after enqueue", addr.String())

	require.NoError(t, k.NewPayoutVerificationQueue.Dequeue(ctx, addr), "dequeue payout verification for %s should succeed", addr.String())

	it2, err := k.NewPayoutVerificationQueue.Iterate(ctx, nil)
	require.NoError(t, err, "iterate payout verification queue after dequeue should not error")
	defer it2.Close()
	require.False(t, it2.Valid(), "payout verification queue should be empty after dequeue of %s", addr.String())
}

func TestWalkDue(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)

	a1 := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)
	a2 := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)

	require.NoError(t, k.NewPayoutVerificationQueue.Enqueue(ctx, a1), "enqueue payout verification for a1 should succeed")
	require.NoError(t, k.NewPayoutVerificationQueue.Enqueue(ctx, a2), "enqueue payout verification for a2 should succeed")
	require.NoError(t, k.NewPayoutVerificationQueue.Enqueue(ctx, a1), "enqueue payout verification duplicate for a1 should succeed (set semantics)")

	var seen []string
	require.NoError(t, k.NewPayoutVerificationQueue.Walk(ctx, func(addr sdk.AccAddress) (bool, error) {
		seen = append(seen, addr.String())
		return false, nil
	}), "walking payout verifications should not error")
	assert.ElementsMatch(t, []string{a1.String(), a2.String()}, seen, "walk should visit exactly the queued payout verification addresses")
}

func TestWalkDue_StopEarly(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)

	a := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)
	b := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)

	require.NoError(t, k.NewPayoutVerificationQueue.Enqueue(ctx, a), "enqueue payout verification for a should succeed")
	require.NoError(t, k.NewPayoutVerificationQueue.Enqueue(ctx, b), "enqueue payout verification for b should succeed")

	calls := 0
	require.NoError(t, k.NewPayoutVerificationQueue.Walk(ctx, func(_ sdk.AccAddress) (bool, error) {
		calls++
		return true, nil
	}), "walking payout verifications (stop early) should not error")
	assert.Equal(t, 1, calls, "walk should stop after first callback; got %d calls", calls)
}

func TestWalkDue_ErrorPropagates(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)

	a := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)
	require.NoError(t, k.NewPayoutVerificationQueue.Enqueue(ctx, a), "enqueue payout verification for a should succeed")

	errBoom := errors.New("boom")
	err := k.NewPayoutVerificationQueue.Walk(ctx, func(_ sdk.AccAddress) (bool, error) {
		return false, errBoom
	})
	require.ErrorIs(t, err, errBoom, "walk should propagate callback error")
}

func TestRemoveAllStartsForVault(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)

	a1 := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)
	a2 := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)

	require.NoError(t, k.NewPayoutVerificationQueue.Enqueue(ctx, a1), "enqueue payout verification for a1 should succeed")
	require.NoError(t, k.NewPayoutVerificationQueue.Enqueue(ctx, a2), "enqueue payout verification for a2 should succeed")

	require.NoError(t, k.NewPayoutVerificationQueue.Enqueue(ctx, a1), "dequeue payout verification for a1 should succeed")

	it, err := k.NewPayoutVerificationQueue.Iterate(ctx, nil)
	require.NoError(t, err, "iterate payout verification queue should not error")
	defer it.Close()

	for ; it.Valid(); it.Next() {
		kv, err := it.Key()
		require.NoError(t, err, "reading key/value from payout verification iterator should not error")
		require.False(t, kv.Equals(a1), "payout verification queue should not include a1 after dequeue")
	}
}
