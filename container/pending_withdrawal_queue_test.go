package container_test

import (
	"errors"
	"testing"
	"time"

	"cosmossdk.io/collections"
	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/provlabs/vault/container"
	vtypes "github.com/provlabs/vault/types"
	"github.com/provlabs/vault/utils"
	"github.com/provlabs/vault/utils/mocks"
)

func newTestPendingWithdrawalQueue(t *testing.T) (sdk.Context, *container.PendingWithdrawalQueue) {
	t.Helper()
	storeKey := storetypes.NewKVStoreKey(vtypes.ModuleName)
	testCtx := testutil.DefaultContextWithDB(t, storeKey, storetypes.NewTransientStoreKey("transient_test"))

	cfg := mocks.MakeTestEncodingConfig("provlabs")
	sdkCfg := sdk.GetConfig()
	sdkCfg.SetBech32PrefixForAccount("provlabs", "provlabspub")
	sdkCfg.SetBech32PrefixForValidator("provlabsvaloper", "provlabsvaloperpub")
	sdkCfg.SetBech32PrefixForConsensusNode("provlabsvalcons", "provlabsvalconspub")
	vtypes.RegisterInterfaces(cfg.InterfaceRegistry)

	kvStoreService := runtime.NewKVStoreService(storeKey)
	sb := collections.NewSchemaBuilder(kvStoreService)

	q := container.NewPendingWithdrawalQueue(sb, cfg.Codec)
	_, err := sb.Build()
	require.NoError(t, err)
	return testCtx.Ctx.WithLogger(log.NewNopLogger()), q
}

func TestPendingWithdrawalQueue_Codec(t *testing.T) {
	ctx, q := newTestPendingWithdrawalQueue(t)

	addr := utils.TestProvlabsAddress()
	assets := sdk.NewInt64Coin("usd", 100)
	originalReq := vtypes.PendingWithdrawal{
		Owner:        addr.Bech32,
		VaultAddress: addr.Bech32,
		Assets:       assets,
	}

	_, err := q.Enqueue(ctx, time.Now().UnixNano(), originalReq)
	require.NoError(t, err)

	var retrievedReq vtypes.PendingWithdrawal
	retrieved := false
	err = q.Walk(ctx, func(timestamp int64, vault sdk.AccAddress, id uint64, req vtypes.PendingWithdrawal) (stop bool, err error) {
		retrievedReq = req
		retrieved = true
		return true, nil // stop after first item
	})

	require.NoError(t, err)
	require.True(t, retrieved, "did not retrieve any request from the queue")

	require.Equal(t, originalReq.Owner, retrievedReq.Owner)
	require.Equal(t, originalReq.VaultAddress, retrievedReq.VaultAddress)
	require.Equal(t, originalReq.Assets, retrievedReq.Assets)
}

func TestWalk_Success(t *testing.T) {
	ctx, q := newTestPendingWithdrawalQueue(t)
	addr1 := utils.TestProvlabsAddress()
	addr2 := utils.TestProvlabsAddress()
	req1 := vtypes.PendingWithdrawal{VaultAddress: addr1.Bech32}
	req2 := vtypes.PendingWithdrawal{VaultAddress: addr2.Bech32}

	_, err := q.Enqueue(ctx, time.Now().UnixNano(), req1)
	require.NoError(t, err)
	_, err = q.Enqueue(ctx, time.Now().UnixNano(), req2)
	require.NoError(t, err)

	var seen []string
	err = q.Walk(ctx, func(timestamp int64, vault sdk.AccAddress, id uint64, req vtypes.PendingWithdrawal) (stop bool, err error) {
		seen = append(seen, vault.String())
		return false, nil
	})
	require.NoError(t, err)
	require.ElementsMatch(t, []string{addr1.Bech32, addr2.Bech32}, seen)
}

func TestWalk_Empty(t *testing.T) {
	ctx, q := newTestPendingWithdrawalQueue(t)

	calls := 0
	err := q.Walk(ctx, func(timestamp int64, vault sdk.AccAddress, id uint64, req vtypes.PendingWithdrawal) (stop bool, err error) {
		calls++
		return false, nil
	})
	require.NoError(t, err)
	require.Equal(t, 0, calls)
}

func TestWalk_Error(t *testing.T) {
	ctx, q := newTestPendingWithdrawalQueue(t)
	addr1 := utils.TestProvlabsAddress()
	addr2 := utils.TestProvlabsAddress()
	req1 := vtypes.PendingWithdrawal{VaultAddress: addr1.Bech32}
	req2 := vtypes.PendingWithdrawal{VaultAddress: addr2.Bech32}

	_, err := q.Enqueue(ctx, time.Now().UnixNano(), req1)
	require.NoError(t, err)
	_, err = q.Enqueue(ctx, time.Now().UnixNano(), req2)
	require.NoError(t, err)

	boom := errors.New("boom")
	calls := 0
	err = q.Walk(ctx, func(timestamp int64, vault sdk.AccAddress, id uint64, req vtypes.PendingWithdrawal) (stop bool, err error) {
		calls++
		if calls == 2 {
			return false, boom
		}
		return false, nil
	})
	require.ErrorIs(t, err, boom)
	require.Equal(t, 2, calls)
}

func TestEnqueueAndDequeue(t *testing.T) {
	ctx, q := newTestPendingWithdrawalQueue(t)

	// Dequeue with a non-existent key should not error.
	addr := utils.TestProvlabsAddress()
	err := q.Dequeue(ctx, 1, sdk.MustAccAddressFromBech32(addr.Bech32), 1)
	require.NoError(t, err)

	addr1 := utils.TestProvlabsAddress()
	addr2 := utils.TestProvlabsAddress()
	req1 := vtypes.PendingWithdrawal{VaultAddress: addr1.Bech32, Owner: addr1.Bech32}
	req2 := vtypes.PendingWithdrawal{VaultAddress: addr2.Bech32, Owner: addr2.Bech32}

	// Enqueue two items
	id1, err := q.Enqueue(ctx, 1, req1)
	require.NoError(t, err)
	id2, err := q.Enqueue(ctx, 2, req2)
	require.NoError(t, err)

	// Dequeue first item
	err = q.Dequeue(ctx, 1, sdk.MustAccAddressFromBech32(addr1.Bech32), id1)
	require.NoError(t, err)

	// Verify first item is removed
	_, err = q.Get(ctx, collections.Join3(int64(1), sdk.MustAccAddressFromBech32(addr1.Bech32), id1))
	require.Error(t, err)
	require.ErrorIs(t, err, collections.ErrNotFound)

	// Dequeue second item
	err = q.Dequeue(ctx, 2, sdk.MustAccAddressFromBech32(addr2.Bech32), id2)
	require.NoError(t, err)

	// Verify second item is removed
	_, err = q.Get(ctx, collections.Join3(int64(2), sdk.MustAccAddressFromBech32(addr2.Bech32), id2))
	require.Error(t, err)
	require.ErrorIs(t, err, collections.ErrNotFound)
}
