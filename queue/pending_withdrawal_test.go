package queue_test

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

	"github.com/provlabs/vault/queue"
	vtypes "github.com/provlabs/vault/types"
	"github.com/provlabs/vault/utils"
	"github.com/provlabs/vault/utils/mocks"
)

func newTestPendingWithdrawalQueue(t *testing.T) (sdk.Context, *queue.PendingWithdrawalQueue) {
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

	q := queue.NewPendingWithdrawalQueue(sb, cfg.Codec)
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

func TestPendingWithdrawalQueueWalk_Success(t *testing.T) {
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

func TestPendingWithdrawalQueueWalk_Empty(t *testing.T) {
	ctx, q := newTestPendingWithdrawalQueue(t)

	calls := 0
	err := q.Walk(ctx, func(timestamp int64, vault sdk.AccAddress, id uint64, req vtypes.PendingWithdrawal) (stop bool, err error) {
		calls++
		return false, nil
	})
	require.NoError(t, err)
	require.Equal(t, 0, calls)
}

func TestPendingWithdrawalQueueWalk_Error(t *testing.T) {
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

func TestPendingWithdrawalQueueEnqueueAndDequeue(t *testing.T) {
	addr1 := utils.TestProvlabsAddress()
	addr2 := utils.TestProvlabsAddress()

	tests := []struct {
		name        string
		setup       func(t *testing.T, ctx sdk.Context, q *queue.PendingWithdrawalQueue) (int64, sdk.AccAddress, uint64)
		timestamp   int64
		req         vtypes.PendingWithdrawal
		dequeue     bool
		expectError bool
		errorMsg    string
	}{
		{
			name: "successful enqueue and dequeue",
			setup: func(t *testing.T, ctx sdk.Context, q *queue.PendingWithdrawalQueue) (int64, sdk.AccAddress, uint64) {
				req := vtypes.PendingWithdrawal{VaultAddress: addr1.Bech32, Owner: addr1.Bech32}
				id, err := q.Enqueue(ctx, 1, req)
				require.NoError(t, err)
				return 1, sdk.MustAccAddressFromBech32(addr1.Bech32), id
			},
			dequeue:     true,
			expectError: false,
		},
		{
			name: "dequeue non-existent key",
			setup: func(t *testing.T, ctx sdk.Context, q *queue.PendingWithdrawalQueue) (int64, sdk.AccAddress, uint64) {
				return 1, sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32), 999
			},
			dequeue:     true,
			expectError: false, // Dequeue with a non-existent key should not error.
		},
		{
			name: "enqueue with invalid vault address",
			req: vtypes.PendingWithdrawal{
				VaultAddress: "invalid",
			},
			timestamp:   1,
			dequeue:     false,
			expectError: true,
		},
		{
			name: "enqueue with negative timestamp",
			req: vtypes.PendingWithdrawal{
				VaultAddress: addr2.Bech32,
			},
			timestamp:   -1,
			dequeue:     false,
			expectError: true,
			errorMsg:    "timestamp cannot be negative",
		},
		{
			name: "dequeue with negative timestamp",
			setup: func(t *testing.T, ctx sdk.Context, q *queue.PendingWithdrawalQueue) (int64, sdk.AccAddress, uint64) {
				return -1, sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32), 1
			},
			dequeue:     true,
			expectError: true,
			errorMsg:    "timestamp cannot be negative",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, q := newTestPendingWithdrawalQueue(t)

			if tc.dequeue {
				timestamp, addr, id := tc.setup(t, ctx, q)
				err := q.Dequeue(ctx, timestamp, addr, id)
				if tc.expectError {
					require.Error(t, err)
					if tc.errorMsg != "" {
						require.Contains(t, err.Error(), tc.errorMsg)
					}
				} else {
					require.NoError(t, err)
					// Verify item is removed
					_, err = q.IndexedMap.Get(ctx, collections.Join3(timestamp, addr, id))
					require.Error(t, err)
					require.ErrorIs(t, err, collections.ErrNotFound)
				}
			} else {
				_, err := q.Enqueue(ctx, tc.timestamp, tc.req)
				if tc.expectError {
					require.Error(t, err)
					if tc.errorMsg != "" {
						require.Contains(t, err.Error(), tc.errorMsg)
					}
				} else {
					require.NoError(t, err)
				}
			}
		})
	}
}
