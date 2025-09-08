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
	originalReq := &vtypes.PendingWithdrawal{
		Owner:        addr.Bech32,
		VaultAddress: addr.Bech32,
		Assets:       assets,
	}

	_, err := q.Enqueue(ctx, time.Now().UnixNano(), originalReq)
	require.NoError(t, err)

	var retrievedReq vtypes.PendingWithdrawal
	retrieved := false
	err = q.Walk(ctx, func(timestamp int64, id uint64, vault sdk.AccAddress, req vtypes.PendingWithdrawal) (stop bool, err error) {
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

func TestPendingWithdrawalQueueWalk(t *testing.T) {
	addr1 := utils.TestProvlabsAddress()
	addr2 := utils.TestProvlabsAddress()
	if addr1.Bech32 > addr2.Bech32 {
		addr1, addr2 = addr2, addr1
	}
	req1 := &vtypes.PendingWithdrawal{VaultAddress: addr1.Bech32, Owner: addr1.Bech32, Assets: sdk.NewInt64Coin("ylds", 25)}
	req2 := &vtypes.PendingWithdrawal{VaultAddress: addr2.Bech32, Owner: addr2.Bech32, Assets: sdk.NewInt64Coin("ylds", 50)}
	req3 := &vtypes.PendingWithdrawal{VaultAddress: addr1.Bech32, Owner: addr2.Bech32, Assets: sdk.NewInt64Coin("ylds", 100)}
	boom := errors.New("boom")

	tests := []struct {
		name         string
		setup        func(t *testing.T, ctx sdk.Context, q *queue.PendingWithdrawalQueue)
		expectedErr  error
		expectedSeen []vtypes.PendingWithdrawal
	}{
		{
			name: "success",
			setup: func(t *testing.T, ctx sdk.Context, q *queue.PendingWithdrawalQueue) {
				_, err := q.Enqueue(ctx, 1, req1)
				require.NoError(t, err)
				_, err = q.Enqueue(ctx, 2, req2)
				require.NoError(t, err)
			},
			expectedSeen: []vtypes.PendingWithdrawal{*req1, *req2},
		},
		{
			name: "success with order preservation",
			setup: func(t *testing.T, ctx sdk.Context, q *queue.PendingWithdrawalQueue) {
				_, err := q.Enqueue(ctx, 1, req1)
				require.NoError(t, err)
				_, err = q.Enqueue(ctx, 2, req2)
				require.NoError(t, err)
				_, err = q.Enqueue(ctx, 3, req3)
				require.NoError(t, err)
			},
			expectedSeen: []vtypes.PendingWithdrawal{*req1, *req2, *req3},
		},
		{
			name:         "empty",
			setup:        func(t *testing.T, ctx sdk.Context, q *queue.PendingWithdrawalQueue) {},
			expectedSeen: []vtypes.PendingWithdrawal{},
		},
		{
			name: "error",
			setup: func(t *testing.T, ctx sdk.Context, q *queue.PendingWithdrawalQueue) {
				_, err := q.Enqueue(ctx, 1, req1)
				require.NoError(t, err)
				_, err = q.Enqueue(ctx, 2, req2)
				require.NoError(t, err)
			},
			expectedErr:  boom,
			expectedSeen: []vtypes.PendingWithdrawal{*req1, *req2},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, q := newTestPendingWithdrawalQueue(t)
			if tc.setup != nil {
				tc.setup(t, ctx, q)
			}

			seen := []vtypes.PendingWithdrawal{}
			calls := 0
			err := q.Walk(ctx, func(timestamp int64, id uint64, vault sdk.AccAddress, req vtypes.PendingWithdrawal) (stop bool, err error) {
				calls++
				seen = append(seen, req)
				if tc.expectedErr != nil && calls == len(tc.expectedSeen) {
					return false, tc.expectedErr
				}
				return false, nil
			})

			if tc.expectedErr != nil {
				require.ErrorIs(t, err, tc.expectedErr)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.expectedSeen, seen)
		})
	}
}

func TestPendingWithdrawalQueueEnqueueAndDequeue(t *testing.T) {
	addr1 := utils.TestProvlabsAddress()
	addr2 := utils.TestProvlabsAddress()

	tests := []struct {
		name        string
		setup       func(t *testing.T, ctx sdk.Context, q *queue.PendingWithdrawalQueue) (int64, sdk.AccAddress, uint64)
		timestamp   int64
		req         *vtypes.PendingWithdrawal
		dequeue     bool
		expectError bool
		errorMsg    string
	}{
		{
			name: "successful enqueue and dequeue",
			setup: func(t *testing.T, ctx sdk.Context, q *queue.PendingWithdrawalQueue) (int64, sdk.AccAddress, uint64) {
				req := &vtypes.PendingWithdrawal{VaultAddress: addr1.Bech32, Owner: addr1.Bech32}
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
			req: &vtypes.PendingWithdrawal{
				VaultAddress: "invalid",
			},
			timestamp:   1,
			dequeue:     false,
			expectError: true,
		},
		{
			name: "enqueue with negative timestamp",
			req: &vtypes.PendingWithdrawal{
				VaultAddress: addr2.Bech32,
			},
			timestamp:   -1,
			dequeue:     false,
			expectError: true,
			errorMsg:    "pending time cannot be negative",
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
					_, err = q.IndexedMap.Get(ctx, collections.Join3(timestamp, id, addr))
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

func TestPendingWithdrawalQueue_GetByID(t *testing.T) {
	addr1 := utils.TestProvlabsAddress()
	req1 := &vtypes.PendingWithdrawal{VaultAddress: addr1.Bech32, Owner: addr1.Bech32}

	tests := map[string]struct {
		setup    func(t *testing.T, ctx sdk.Context, q *queue.PendingWithdrawalQueue) uint64
		id       uint64
		errorMsg string
	}{
		"success": {
			setup: func(t *testing.T, ctx sdk.Context, q *queue.PendingWithdrawalQueue) uint64 {
				id, err := q.Enqueue(ctx, 123, req1)
				require.NoError(t, err)
				return id
			},
		},
		"failure - invalid id": {
			setup: func(t *testing.T, ctx sdk.Context, q *queue.PendingWithdrawalQueue) uint64 {
				return 999
			},
			errorMsg: "not found",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctx, q := newTestPendingWithdrawalQueue(t)
			id := tc.setup(t, ctx, q)

			pendingTime, req, err := q.GetByID(ctx, id)

			if len(tc.errorMsg) > 0 {
				require.Error(t, err)
				require.ErrorContains(t, err, tc.errorMsg)
				require.Equal(t, int64(0), pendingTime)
			} else {
				require.NoError(t, err)
				require.Equal(t, req1.VaultAddress, req.VaultAddress)
				require.Equal(t, req1.Owner, req.Owner)
				require.Equal(t, int64(123), pendingTime)
			}
		})
	}
}

func TestPendingWithdrawalQueue_ExpediteWithdrawal(t *testing.T) {
	addr1 := utils.TestProvlabsAddress()
	addr2 := utils.TestProvlabsAddress()
	req1 := &vtypes.PendingWithdrawal{VaultAddress: addr1.Bech32, Owner: addr1.Bech32}
	req2 := &vtypes.PendingWithdrawal{VaultAddress: addr2.Bech32, Owner: addr2.Bech32}

	tests := map[string]struct {
		setup      func(t *testing.T, ctx sdk.Context, q *queue.PendingWithdrawalQueue) (uint64, int64, sdk.AccAddress)
		expediteId uint64
		errorMsg   string
	}{
		"success with one entry": {
			setup: func(t *testing.T, ctx sdk.Context, q *queue.PendingWithdrawalQueue) (uint64, int64, sdk.AccAddress) {
				id, err := q.Enqueue(ctx, 123, req1)
				require.NoError(t, err)
				return id, 123, sdk.MustAccAddressFromBech32(addr1.Bech32)
			},
		},
		"success with two entries, second is expedited": {
			setup: func(t *testing.T, ctx sdk.Context, q *queue.PendingWithdrawalQueue) (uint64, int64, sdk.AccAddress) {
				_, err := q.Enqueue(ctx, 123, req1)
				require.NoError(t, err)
				id2, err := q.Enqueue(ctx, 456, req2)
				require.NoError(t, err)
				return id2, 456, sdk.MustAccAddressFromBech32(addr2.Bech32)
			},
		},
		"success on entry with timestamp 0": {
			setup: func(t *testing.T, ctx sdk.Context, q *queue.PendingWithdrawalQueue) (uint64, int64, sdk.AccAddress) {
				id, err := q.Enqueue(ctx, 0, req1)
				require.NoError(t, err)
				return id, 0, sdk.MustAccAddressFromBech32(addr1.Bech32)
			},
		},
		"failure if entry does not exist": {
			setup: func(t *testing.T, ctx sdk.Context, q *queue.PendingWithdrawalQueue) (uint64, int64, sdk.AccAddress) {
				return 999, 0, nil
			},
			expediteId: 999,
			errorMsg:   "not found",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctx, q := newTestPendingWithdrawalQueue(t)
			id, oldTimestamp, addr := tc.setup(t, ctx, q)
			if tc.expediteId != 0 {
				id = tc.expediteId
			}

			err := q.ExpediteWithdrawal(ctx, id)

			if len(tc.errorMsg) > 0 {
				require.Error(t, err)
				require.ErrorContains(t, err, tc.errorMsg)
			} else {
				require.NoError(t, err)

				// Verify old entry is removed
				if oldTimestamp != 0 {
					_, err = q.IndexedMap.Get(ctx, collections.Join3(oldTimestamp, id, addr))
					require.ErrorContains(t, err, "not found")
				}

				// Verify new entry exists with timestamp 0
				_, err = q.IndexedMap.Get(ctx, collections.Join3(int64(0), id, addr))
				require.NoError(t, err)
			}
		})
	}
}

func TestPendingWithdrawalQueue_Export(t *testing.T) {
	addr1 := utils.TestProvlabsAddress()
	addr2 := utils.TestProvlabsAddress()
	if addr1.Bech32 > addr2.Bech32 {
		addr1, addr2 = addr2, addr1
	}
	req1 := &vtypes.PendingWithdrawal{VaultAddress: addr1.Bech32, Owner: addr1.Bech32, Assets: sdk.NewInt64Coin("usd", 100)}
	req2 := &vtypes.PendingWithdrawal{VaultAddress: addr2.Bech32, Owner: addr2.Bech32, Assets: sdk.NewInt64Coin("usd", 200)}
	req3 := &vtypes.PendingWithdrawal{VaultAddress: addr1.Bech32, Owner: addr1.Bech32, Assets: sdk.NewInt64Coin("usd", 300)}

	tests := []struct {
		name          string
		setup         func(t *testing.T, ctx sdk.Context, q *queue.PendingWithdrawalQueue)
		expectedQueue vtypes.PendingWithdrawalQueue
		expectError   bool
	}{
		{
			name: "multiple elements",
			setup: func(t *testing.T, ctx sdk.Context, q *queue.PendingWithdrawalQueue) {
				_, err := q.Enqueue(ctx, 1, req1)
				require.NoError(t, err)
				_, err = q.Enqueue(ctx, 2, req2)
				require.NoError(t, err)
				_, err = q.Enqueue(ctx, 1, req3)
				require.NoError(t, err)
			},
			expectedQueue: vtypes.PendingWithdrawalQueue{
				LatestSequenceNumber: 3,
				Entries: []vtypes.PendingWithdrawalQueueEntry{
					{Time: 1, Id: 0, Withdrawal: *req1},
					{Time: 1, Id: 2, Withdrawal: *req3},
					{Time: 2, Id: 1, Withdrawal: *req2},
				},
			},
			expectError: false,
		},
		{
			name: "no elements",
			setup: func(t *testing.T, ctx sdk.Context, q *queue.PendingWithdrawalQueue) {
				// No elements enqueued
			},
			expectedQueue: vtypes.PendingWithdrawalQueue{
				LatestSequenceNumber: 0,
				Entries:              []vtypes.PendingWithdrawalQueueEntry{},
			},
			expectError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, q := newTestPendingWithdrawalQueue(t)
			tc.setup(t, ctx, q)

			exportedQueue, err := q.Export(ctx)

			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedQueue.LatestSequenceNumber, exportedQueue.LatestSequenceNumber)
				require.Equal(t, len(tc.expectedQueue.Entries), len(exportedQueue.Entries))
				for i, entry := range tc.expectedQueue.Entries {
					require.Equal(t, entry.Time, exportedQueue.Entries[i].Time)
					require.Equal(t, entry.Id, exportedQueue.Entries[i].Id)
					require.Equal(t, entry.Withdrawal.Owner, exportedQueue.Entries[i].Withdrawal.Owner)
					require.Equal(t, entry.Withdrawal.VaultAddress, exportedQueue.Entries[i].Withdrawal.VaultAddress)
					require.Equal(t, entry.Withdrawal.Assets.String(), exportedQueue.Entries[i].Withdrawal.Assets.String())
				}
			}
		})
	}
}
