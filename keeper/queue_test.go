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
	addr := sdk.MustAccAddressFromBech32(utils.TestAddress().Bech32)

	require.NoError(t, k.EnqueueVaultStart(ctx, addr))

	it, err := k.VaultPayoutVerificationQueue.Iterate(ctx, nil)
	require.NoError(t, err)
	defer it.Close()

	found := false
	for ; it.Valid(); it.Next() {
		kv, err := it.KeyValue()
		require.NoError(t, err)
		if kv.Key.Equals(addr) {
			found = true
			break
		}
	}
	require.True(t, found)

	require.NoError(t, k.DequeueVaultStart(ctx, addr))

	it2, err := k.VaultPayoutVerificationQueue.Iterate(ctx, nil)
	require.NoError(t, err)
	defer it2.Close()
	require.False(t, it2.Valid())
}

func TestEnqueueDequeue_Timeout(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)
	addr := sdk.MustAccAddressFromBech32(utils.TestAddress().Bech32)
	ts := int64(200)

	require.NoError(t, k.EnqueueVaultTimeout(ctx, ts, addr))

	it, err := k.VaultTimeoutQueue.Iterate(ctx, nil)
	require.NoError(t, err)
	defer it.Close()

	found := false
	for ; it.Valid(); it.Next() {
		kv, err := it.KeyValue()
		require.NoError(t, err)
		if kv.Key.K1() == uint64(ts) && kv.Key.K2().Equals(addr) {
			found = true
			break
		}
	}
	require.True(t, found)

	require.NoError(t, k.DequeueVaultTimeout(ctx, ts, addr))

	it2, err := k.VaultTimeoutQueue.Iterate(ctx, nil)
	require.NoError(t, err)
	defer it2.Close()
	require.False(t, it2.Valid())
}

func TestWalkDueStarts(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)
	a1 := sdk.MustAccAddressFromBech32(utils.TestAddress().Bech32)
	a2 := sdk.MustAccAddressFromBech32(utils.TestAddress().Bech32)

	require.NoError(t, k.EnqueueVaultStart(ctx, a1))
	require.NoError(t, k.EnqueueVaultStart(ctx, a2))
	require.NoError(t, k.EnqueueVaultStart(ctx, a1))

	var seen []string
	require.NoError(t, k.WalkDueStarts(ctx, 0, func(addr sdk.AccAddress) (bool, error) {
		seen = append(seen, addr.String())
		return false, nil
	}))
	assert.ElementsMatch(t, []string{a1.String(), a2.String()}, seen)
}

func TestWalkDueStarts_StopEarly(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)
	a := sdk.MustAccAddressFromBech32(utils.TestAddress().Bech32)
	b := sdk.MustAccAddressFromBech32(utils.TestAddress().Bech32)
	require.NoError(t, k.EnqueueVaultStart(ctx, a))
	require.NoError(t, k.EnqueueVaultStart(ctx, b))

	calls := 0
	require.NoError(t, k.WalkDueStarts(ctx, 0, func(_ sdk.AccAddress) (bool, error) {
		calls++
		return true, nil
	}))
	assert.Equal(t, 1, calls)
}

func TestWalkDueStarts_ErrorPropagates(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)
	a := sdk.MustAccAddressFromBech32(utils.TestAddress().Bech32)
	require.NoError(t, k.EnqueueVaultStart(ctx, a))

	errBoom := errors.New("boom")
	err := k.WalkDueStarts(ctx, 0, func(_ sdk.AccAddress) (bool, error) {
		return false, errBoom
	})
	require.ErrorIs(t, err, errBoom)
}

func TestWalkDueTimeouts(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)
	a1 := sdk.MustAccAddressFromBech32(utils.TestAddress().Bech32)
	a2 := sdk.MustAccAddressFromBech32(utils.TestAddress().Bech32)

	require.NoError(t, k.EnqueueVaultTimeout(ctx, 50, a1))
	require.NoError(t, k.EnqueueVaultTimeout(ctx, 75, a2))
	require.NoError(t, k.EnqueueVaultTimeout(ctx, 500, a1))

	var seen []uint64
	require.NoError(t, k.WalkDueTimeouts(ctx, 100, func(ts uint64, _ sdk.AccAddress) (bool, error) {
		seen = append(seen, ts)
		return false, nil
	}))
	assert.ElementsMatch(t, []uint64{50, 75}, seen)
}

func TestWalkDueTimeouts_StopEarly(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)
	a := sdk.MustAccAddressFromBech32(utils.TestAddress().Bech32)
	require.NoError(t, k.EnqueueVaultTimeout(ctx, 10, a))
	require.NoError(t, k.EnqueueVaultTimeout(ctx, 20, a))

	calls := 0
	require.NoError(t, k.WalkDueTimeouts(ctx, 25, func(_ uint64, _ sdk.AccAddress) (bool, error) {
		calls++
		return true, nil
	}))
	assert.Equal(t, 1, calls)
}

func TestWalkDueTimeouts_ErrorPropagates(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)
	a := sdk.MustAccAddressFromBech32(utils.TestAddress().Bech32)
	require.NoError(t, k.EnqueueVaultTimeout(ctx, 10, a))

	errBoom := errors.New("boom")
	err := k.WalkDueTimeouts(ctx, 25, func(_ uint64, _ sdk.AccAddress) (bool, error) {
		return false, errBoom
	})
	require.ErrorIs(t, err, errBoom)
}

func TestRemoveAllStartsForVault(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)
	a1 := sdk.MustAccAddressFromBech32(utils.TestAddress().Bech32)
	a2 := sdk.MustAccAddressFromBech32(utils.TestAddress().Bech32)

	require.NoError(t, k.EnqueueVaultStart(ctx, a1))
	require.NoError(t, k.EnqueueVaultStart(ctx, a2))

	require.NoError(t, k.RemoveAllStartsForVault(ctx, a1))

	it, err := k.VaultPayoutVerificationQueue.Iterate(ctx, nil)
	require.NoError(t, err)
	defer it.Close()

	for ; it.Valid(); it.Next() {
		kv, err := it.KeyValue()
		require.NoError(t, err)
		require.False(t, kv.Key.Equals(a1))
	}
}

func TestRemoveAllTimeoutsForVault(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)
	a1 := sdk.MustAccAddressFromBech32(utils.TestAddress().Bech32)
	a2 := sdk.MustAccAddressFromBech32(utils.TestAddress().Bech32)

	require.NoError(t, k.EnqueueVaultTimeout(ctx, 100, a1))
	require.NoError(t, k.EnqueueVaultTimeout(ctx, 150, a1))
	require.NoError(t, k.EnqueueVaultTimeout(ctx, 200, a2))

	require.NoError(t, k.RemoveAllTimeoutsForVault(ctx, a1))

	it, err := k.VaultTimeoutQueue.Iterate(ctx, nil)
	require.NoError(t, err)
	defer it.Close()

	for ; it.Valid(); it.Next() {
		kv, err := it.KeyValue()
		require.NoError(t, err)
		require.False(t, kv.Key.K2().Equals(a1))
	}
}

func TestSafeEnqueueStart_UpdatesVaultAndQueues(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)
	admin := sdk.MustAccAddressFromBech32(utils.TestAddress().Bech32)
	share := "vaultshares"
	vaultAddr := types.GetVaultAddress(share)

	v := &types.VaultAccount{
		BaseAccount:      authtypes.NewBaseAccountWithAddress(vaultAddr),
		Admin:            admin.String(),
		ShareDenom:       share,
		UnderlyingAssets: []string{"under"},
		PeriodStart:      0,
		PeriodTimeout:    50,
	}

	require.NoError(t, k.EnqueueVaultTimeout(ctx, 50, vaultAddr))

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx = sdkCtx.WithBlockTime(time.Unix(1000, 0))

	require.NoError(t, k.SafeEnqueueStart(ctx, v))

	itS, err := k.VaultPayoutVerificationQueue.Iterate(ctx, nil)
	require.NoError(t, err)
	defer itS.Close()

	foundStart := false
	for ; itS.Valid(); itS.Next() {
		kv, err := itS.KeyValue()
		require.NoError(t, err)
		if kv.Key.Equals(vaultAddr) {
			foundStart = true
		}
	}
	require.True(t, foundStart)

	itT, err := k.VaultTimeoutQueue.Iterate(ctx, nil)
	require.NoError(t, err)
	defer itT.Close()

	for ; itT.Valid(); itT.Next() {
		kv, err := itT.KeyValue()
		require.NoError(t, err)
		require.False(t, kv.Key.K2().Equals(vaultAddr))
	}

	acc := k.AuthKeeper.GetAccount(sdkCtx, vaultAddr)
	require.NotNil(t, acc)
	va, ok := acc.(*types.VaultAccount)
	require.True(t, ok)
	require.Equal(t, int64(1000), va.PeriodStart)
	require.Equal(t, int64(0), va.PeriodTimeout)
}

func TestSafeEnqueueTimeout_UpdatesVaultAndQueues(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)
	admin := sdk.MustAccAddressFromBech32(utils.TestAddress().Bech32)
	share := "vaultshares2"
	vaultAddr := types.GetVaultAddress(share)

	v := &types.VaultAccount{
		BaseAccount:      authtypes.NewBaseAccountWithAddress(vaultAddr),
		Admin:            admin.String(),
		ShareDenom:       share,
		UnderlyingAssets: []string{"under"},
		PeriodStart:      0,
		PeriodTimeout:    30,
	}

	require.NoError(t, k.EnqueueVaultTimeout(ctx, 30, vaultAddr))

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	now := time.Unix(2000, 0)
	sdkCtx = sdkCtx.WithBlockTime(now)

	require.NoError(t, k.SafeEnqueueTimeout(sdkCtx, v))

	expectTimeout := uint64(now.Unix() + kpr.AutoReconcileTimeout)

	var times []uint64
	itT, err := k.VaultTimeoutQueue.Iterate(ctx, nil)
	require.NoError(t, err)
	defer itT.Close()
	for ; itT.Valid(); itT.Next() {
		kv, err := itT.KeyValue()
		require.NoError(t, err)
		if kv.Key.K2().Equals(vaultAddr) {
			times = append(times, kv.Key.K1())
		}
	}
	require.ElementsMatch(t, []uint64{expectTimeout}, times)

	acc := k.AuthKeeper.GetAccount(sdkCtx, vaultAddr)
	require.NotNil(t, acc)
	va, ok := acc.(*types.VaultAccount)
	require.True(t, ok)
	require.Equal(t, now.Unix(), va.PeriodStart)
	require.Equal(t, int64(expectTimeout), va.PeriodTimeout)
}
