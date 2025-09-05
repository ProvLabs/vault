package keeper_test

import (
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	kpr "github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/types"
	"github.com/provlabs/vault/utils"
	"github.com/provlabs/vault/utils/mocks"
	"github.com/stretchr/testify/require"
)

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

	require.NoError(t, k.PayoutTimeoutQueue.Enqueue(ctx, 50, vaultAddr), "precondition: enqueue payout timeout (50) for vault should succeed")

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockTime(time.Unix(1000, 0))

	require.NoError(t, k.SafeAddVerification(ctx, v), "SafeEnqueueVerification should clear timeouts, set start, and enqueue verification")

	itS, err := k.PayoutVerificationSet.Iterate(ctx, nil)
	require.NoError(t, err, "iterate payout verification queue should not error after SafeEnqueueVerification")
	defer itS.Close()

	foundStart := false
	for ; itS.Valid(); itS.Next() {
		kv, err := itS.Key()
		require.NoError(t, err, "reading key/value from payout verification iterator should not error")
		if kv.Equals(vaultAddr) {
			foundStart = true
		}
	}
	require.True(t, foundStart, "payout verification queue should contain vault %s after SafeEnqueueVerification", vaultAddr.String())

	err = k.PayoutTimeoutQueue.Walk(ctx, func(timestamp uint64, address sdk.AccAddress) (bool, error) {
		require.False(t, address.Equals(vaultAddr), "payout timeout queue should not contain vault %s after SafeEnqueueVerification", vaultAddr.String())
		return false, nil
	})
	require.NoError(t, err, "walk payout timeout queue should not error after SafeEnqueueVerification")

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

	require.NoError(t, k.PayoutTimeoutQueue.Enqueue(ctx, 30, vaultAddr), "precondition: enqueue payout timeout (30) for vault should succeed")

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	now := time.Unix(2000, 0)
	ctx = sdkCtx.WithBlockTime(now)

	require.NoError(t, k.SafeEnqueueTimeout(ctx, v), "SafeEnqueueTimeout should clear previous timeouts, set new timeout, and enqueue timeout entry")

	expectTimeout := uint64(now.Unix() + kpr.AutoReconcileTimeout)

	var times []uint64
	err := k.PayoutTimeoutQueue.Walk(ctx, func(timestamp uint64, address sdk.AccAddress) (bool, error) {
		if address.Equals(vaultAddr) {
			times = append(times, timestamp)
		}
		return false, nil
	})
	require.NoError(t, err, "walk timeout queue should not error after SafeEnqueueTimeout")
	require.ElementsMatch(t, []uint64{expectTimeout}, times, "timeout queue should include exactly one entry for vault at %d; got %v", expectTimeout, times)

	acc := k.AuthKeeper.GetAccount(ctx, vaultAddr)
	require.NotNil(t, acc, "vault account should exist in state after SafeEnqueueTimeout")
	va, ok := acc.(*types.VaultAccount)
	require.True(t, ok, "retrieved account should be *types.VaultAccount; got %T", acc)
	require.Equal(t, now.Unix(), va.PeriodStart, "vault PeriodStart should equal block time; expected %d, got %d", now.Unix(), va.PeriodStart)
	require.Equal(t, int64(expectTimeout), va.PeriodTimeout, "vault PeriodTimeout should equal expected timeout; expected %d, got %d", expectTimeout, va.PeriodTimeout)
}
