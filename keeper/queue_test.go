package keeper_test

import (
	"errors"
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	kpr "github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/types"
	"github.com/provlabs/vault/utils"
	"github.com/provlabs/vault/utils/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnqueueDequeue_Start(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)

	addr := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)

	require.NoError(t, k.EnqueuePayoutVerification(ctx, addr), "enqueue payout verification for %s should succeed", addr.String())

	it, err := k.PayoutVerificationQueue.Iterate(ctx, nil)
	require.NoError(t, err, "iterate payout verification queue should not error")
	defer it.Close()

	found := false
	for ; it.Valid(); it.Next() {
		kv, err := it.KeyValue()
		require.NoError(t, err, "reading key/value from payout verification iterator should not error")
		if kv.Key.Equals(addr) {
			found = true
			break
		}
	}
	require.True(t, found, "expected to find %s in payout verification queue after enqueue", addr.String())

	require.NoError(t, k.DequeuePayoutVerification(ctx, addr), "dequeue payout verification for %s should succeed", addr.String())

	it2, err := k.PayoutVerificationQueue.Iterate(ctx, nil)
	require.NoError(t, err, "iterate payout verification queue after dequeue should not error")
	defer it2.Close()
	require.False(t, it2.Valid(), "payout verification queue should be empty after dequeue of %s", addr.String())
}

func TestEnqueueDequeue_Timeout(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)

	addr := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)
	ts := int64(200)

	require.NoError(t, k.EnqueuePayoutTimeout(ctx, ts, addr), "enqueue payout timeout (%d) for %s should succeed", ts, addr.String())

	it, err := k.PayoutTimeoutQueue.Iterate(ctx, nil)
	require.NoError(t, err, "iterate payout timeout queue should not error")
	defer it.Close()

	found := false
	for ; it.Valid(); it.Next() {
		kv, err := it.KeyValue()
		require.NoError(t, err, "reading key/value from payout timeout iterator should not error")
		if kv.Key.K1() == uint64(ts) && kv.Key.K2().Equals(addr) {
			found = true
			break
		}
	}
	require.True(t, found, "expected to find timeout (%d) for %s in payout timeout queue after enqueue", ts, addr.String())

	require.NoError(t, k.DequeuePayoutTimeout(ctx, ts, addr), "dequeue payout timeout (%d) for %s should succeed", ts, addr.String())

	it2, err := k.PayoutTimeoutQueue.Iterate(ctx, nil)
	require.NoError(t, err, "iterate payout timeout queue after dequeue should not error")
	defer it2.Close()
	require.False(t, it2.Valid(), "payout timeout queue should be empty after dequeue of (%d, %s)", ts, addr.String())
}

func TestWalkDueStarts(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)

	a1 := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)
	a2 := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)

	require.NoError(t, k.EnqueuePayoutVerification(ctx, a1), "enqueue payout verification for a1 should succeed")
	require.NoError(t, k.EnqueuePayoutVerification(ctx, a2), "enqueue payout verification for a2 should succeed")
	require.NoError(t, k.EnqueuePayoutVerification(ctx, a1), "enqueue payout verification duplicate for a1 should succeed (set semantics)")

	var seen []string
	require.NoError(t, k.WalkPayoutVerifications(ctx, func(addr sdk.AccAddress) (bool, error) {
		seen = append(seen, addr.String())
		return false, nil
	}), "walking payout verifications should not error")
	assert.ElementsMatch(t, []string{a1.String(), a2.String()}, seen, "walk should visit exactly the queued payout verification addresses")
}

func TestWalkDueStarts_StopEarly(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)

	a := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)
	b := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)

	require.NoError(t, k.EnqueuePayoutVerification(ctx, a), "enqueue payout verification for a should succeed")
	require.NoError(t, k.EnqueuePayoutVerification(ctx, b), "enqueue payout verification for b should succeed")

	calls := 0
	require.NoError(t, k.WalkPayoutVerifications(ctx, func(_ sdk.AccAddress) (bool, error) {
		calls++
		return true, nil
	}), "walking payout verifications (stop early) should not error")
	assert.Equal(t, 1, calls, "walk should stop after first callback; got %d calls", calls)
}

func TestWalkDueStarts_ErrorPropagates(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)

	a := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)
	require.NoError(t, k.EnqueuePayoutVerification(ctx, a), "enqueue payout verification for a should succeed")

	errBoom := errors.New("boom")
	err := k.WalkPayoutVerifications(ctx, func(_ sdk.AccAddress) (bool, error) {
		return false, errBoom
	})
	require.ErrorIs(t, err, errBoom, "walk should propagate callback error")
}

func TestWalkDueTimeouts(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)

	a1 := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)
	a2 := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)

	require.NoError(t, k.EnqueuePayoutTimeout(ctx, 50, a1), "enqueue payout timeout (50) for a1 should succeed")
	require.NoError(t, k.EnqueuePayoutTimeout(ctx, 75, a2), "enqueue payout timeout (75) for a2 should succeed")
	require.NoError(t, k.EnqueuePayoutTimeout(ctx, 500, a1), "enqueue payout timeout (500) for a1 should succeed")

	var seen []uint64
	require.NoError(t, k.WalkDuePayoutTimeouts(ctx, 100, func(ts uint64, _ sdk.AccAddress) (bool, error) {
		seen = append(seen, ts)
		return false, nil
	}), "walking due payout timeouts <= 100 should not error")
	assert.ElementsMatch(t, []uint64{50, 75}, seen, "walk should visit exactly timeouts <= 100; got %v", seen)
}

func TestWalkDueTimeouts_StopEarly(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)

	a := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)
	require.NoError(t, k.EnqueuePayoutTimeout(ctx, 10, a), "enqueue payout timeout (10) should succeed")
	require.NoError(t, k.EnqueuePayoutTimeout(ctx, 20, a), "enqueue payout timeout (20) should succeed")

	calls := 0
	require.NoError(t, k.WalkDuePayoutTimeouts(ctx, 25, func(_ uint64, _ sdk.AccAddress) (bool, error) {
		calls++
		return true, nil
	}), "walking due payout timeouts (stop early) should not error")
	assert.Equal(t, 1, calls, "walk should stop after first callback; got %d calls", calls)
}

func TestWalkDueTimeouts_ErrorPropagates(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)

	a := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)
	require.NoError(t, k.EnqueuePayoutTimeout(ctx, 10, a), "enqueue payout timeout (10) should succeed")

	errBoom := errors.New("boom")
	err := k.WalkDuePayoutTimeouts(ctx, 25, func(_ uint64, _ sdk.AccAddress) (bool, error) {
		return false, errBoom
	})
	require.ErrorIs(t, err, errBoom, "walk should propagate callback error")
}

func TestRemoveAllStartsForVault(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)

	a1 := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)
	a2 := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)

	require.NoError(t, k.EnqueuePayoutVerification(ctx, a1), "enqueue payout verification for a1 should succeed")
	require.NoError(t, k.EnqueuePayoutVerification(ctx, a2), "enqueue payout verification for a2 should succeed")

	require.NoError(t, k.DequeuePayoutVerification(ctx, a1), "dequeue payout verification for a1 should succeed")

	it, err := k.PayoutVerificationQueue.Iterate(ctx, nil)
	require.NoError(t, err, "iterate payout verification queue should not error")
	defer it.Close()

	for ; it.Valid(); it.Next() {
		kv, err := it.KeyValue()
		require.NoError(t, err, "reading key/value from payout verification iterator should not error")
		require.False(t, kv.Key.Equals(a1), "payout verification queue should not include a1 after dequeue")
	}
}

func TestRemoveAllTimeoutsForVault(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)
	stdCtx := sdk.WrapSDKContext(ctx)

	a1 := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)
	a2 := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)

	require.NoError(t, k.EnqueuePayoutTimeout(stdCtx, 100, a1), "enqueue payout timeout (100) for a1 should succeed")
	require.NoError(t, k.EnqueuePayoutTimeout(stdCtx, 150, a1), "enqueue payout timeout (150) for a1 should succeed")
	require.NoError(t, k.EnqueuePayoutTimeout(stdCtx, 200, a2), "enqueue payout timeout (200) for a2 should succeed")

	require.NoError(t, k.RemoveAllPayoutTimeoutsForVault(stdCtx, a1), "remove all timeouts for a1 should succeed")

	it, err := k.PayoutTimeoutQueue.Iterate(stdCtx, nil)
	require.NoError(t, err, "iterate payout timeout queue after removal should not error")
	defer it.Close()

	for ; it.Valid(); it.Next() {
		kv, err := it.KeyValue()
		require.NoError(t, err, "reading key/value from payout timeout iterator should not error")
		require.False(t, kv.Key.K2().Equals(a1), "payout timeout queue should not include any entries for a1 after removal")
	}
}

func TestSafeEnqueueVerification_UpdatesVaultAndQueues(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)

	admin := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)
	share := "vaultshares"
	vaultAddr := types.GetVaultAddress(share)

	v := &types.VaultAccount{
		BaseAccount:         authtypes.NewBaseAccountWithAddress(vaultAddr),
		Admin:               admin.String(),
		ShareDenom:          share,
		UnderlyingAsset:     "under",
		CurrentInterestRate: types.ZeroInterestRate,
		DesiredInterestRate: types.ZeroInterestRate,
		PeriodStart:         0,
		PeriodTimeout:       50,
	}

	require.NoError(t, k.EnqueuePayoutTimeout(ctx, 50, vaultAddr), "precondition: enqueue payout timeout (50) for vault should succeed")

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockTime(time.Unix(1000, 0))

	require.NoError(t, k.SafeEnqueueVerification(ctx, v), "SafeEnqueueVerification should clear timeouts, set start, and enqueue verification")

	itS, err := k.PayoutVerificationQueue.Iterate(ctx, nil)
	require.NoError(t, err, "iterate payout verification queue should not error after SafeEnqueueVerification")
	defer itS.Close()

	foundStart := false
	for ; itS.Valid(); itS.Next() {
		kv, err := itS.KeyValue()
		require.NoError(t, err, "reading key/value from payout verification iterator should not error")
		if kv.Key.Equals(vaultAddr) {
			foundStart = true
		}
	}
	require.True(t, foundStart, "payout verification queue should contain vault %s after SafeEnqueueVerification", vaultAddr.String())

	itT, err := k.PayoutTimeoutQueue.Iterate(ctx, nil)
	require.NoError(t, err, "iterate payout timeout queue should not error after SafeEnqueueVerification")
	defer itT.Close()

	for ; itT.Valid(); itT.Next() {
		kv, err := itT.KeyValue()
		require.NoError(t, err, "reading key/value from payout timeout iterator should not error")
		require.False(t, kv.Key.K2().Equals(vaultAddr), "payout timeout queue should not contain vault %s after SafeEnqueueVerification", vaultAddr.String())
	}

	acc := k.AuthKeeper.GetAccount(ctx, vaultAddr)
	require.NotNil(t, acc, "vault account should exist in state after SafeEnqueueVerification")
	va, ok := acc.(*types.VaultAccount)
	require.True(t, ok, "retrieved account should be *types.VaultAccount; got %T", acc)
	require.Equal(t, int64(1000), va.PeriodStart, "vault PeriodStart should be set to block time; expected 1000, got %d", va.PeriodStart)
	require.Equal(t, int64(0), va.PeriodTimeout, "vault PeriodTimeout should be cleared; expected 0, got %d", va.PeriodTimeout)
}

func TestSafeEnqueueTimeout_UpdatesVaultAndQueues(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)

	admin := sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32)
	share := "vaultshares2"
	vaultAddr := types.GetVaultAddress(share)

	v := &types.VaultAccount{
		BaseAccount:         authtypes.NewBaseAccountWithAddress(vaultAddr),
		Admin:               admin.String(),
		ShareDenom:          share,
		UnderlyingAsset:     "under",
		CurrentInterestRate: types.ZeroInterestRate,
		DesiredInterestRate: types.ZeroInterestRate,
		PeriodStart:         0,
		PeriodTimeout:       30,
	}

	require.NoError(t, k.EnqueuePayoutTimeout(ctx, 30, vaultAddr), "precondition: enqueue payout timeout (30) for vault should succeed")

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	now := time.Unix(2000, 0)
	ctx = sdkCtx.WithBlockTime(now)

	require.NoError(t, k.SafeEnqueueTimeout(ctx, v), "SafeEnqueueTimeout should clear previous timeouts, set new timeout, and enqueue timeout entry")

	expectTimeout := uint64(now.Unix() + kpr.AutoReconcileTimeout)

	var times []uint64
	itT, err := k.PayoutTimeoutQueue.Iterate(ctx, nil)
	require.NoError(t, err, "iterate payout timeout queue should not error after SafeEnqueueTimeout")
	defer itT.Close()
	for ; itT.Valid(); itT.Next() {
		kv, err := itT.KeyValue()
		require.NoError(t, err, "reading key/value from payout timeout iterator should not error")
		if kv.Key.K2().Equals(vaultAddr) {
			times = append(times, kv.Key.K1())
		}
	}
	require.ElementsMatch(t, []uint64{expectTimeout}, times, "timeout queue should include exactly one entry for vault at %d; got %v", expectTimeout, times)

	acc := k.AuthKeeper.GetAccount(ctx, vaultAddr)
	require.NotNil(t, acc, "vault account should exist in state after SafeEnqueueTimeout")
	va, ok := acc.(*types.VaultAccount)
	require.True(t, ok, "retrieved account should be *types.VaultAccount; got %T", acc)
	require.Equal(t, now.Unix(), va.PeriodStart, "vault PeriodStart should equal block time; expected %d, got %d", now.Unix(), va.PeriodStart)
	require.Equal(t, int64(expectTimeout), va.PeriodTimeout, "vault PeriodTimeout should equal expected timeout; expected %d, got %d", expectTimeout, va.PeriodTimeout)
}
