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

func newTestPendingSwapOutQueue(t *testing.T) (sdk.Context, *queue.PendingSwapOutQueue) {
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

	q := queue.NewPendingSwapOutQueue(sb, cfg.Codec)
	_, err := sb.Build()
	require.NoError(t, err)
	return testCtx.Ctx.WithLogger(log.NewNopLogger()), q
}

func TestPendingSwapOutQueue_Codec(t *testing.T) {
	ctx, q := newTestPendingSwapOutQueue(t)

	addr := utils.TestProvlabsAddress()
	redeemDenom := "usd"
	shares := sdk.NewInt64Coin("vshares", 100)
	originalReq := &vtypes.PendingSwapOut{
		Owner:        addr.Bech32,
		VaultAddress: addr.Bech32,
		RedeemDenom:  redeemDenom,
		Shares:       shares,
	}

	_, err := q.Enqueue(ctx, time.Now().Unix(), originalReq)
	require.NoError(t, err)

	var retrievedReq vtypes.PendingSwapOut
	retrieved := false
	err = q.Walk(ctx, func(timestamp int64, id uint64, vault sdk.AccAddress, req vtypes.PendingSwapOut) (stop bool, err error) {
		retrievedReq = req
		retrieved = true
		return true, nil // stop after first item
	})

	require.NoError(t, err)
	require.True(t, retrieved, "did not retrieve any request from the queue")

	require.Equal(t, originalReq.Owner, retrievedReq.Owner)
	require.Equal(t, originalReq.VaultAddress, retrievedReq.VaultAddress)
	require.Equal(t, originalReq.RedeemDenom, retrievedReq.RedeemDenom)
}

func TestPendingSwapOutQueueWalk(t *testing.T) {
	addr1 := utils.TestProvlabsAddress()
	addr2 := utils.TestProvlabsAddress()
	if addr1.Bech32 > addr2.Bech32 {
		addr1, addr2 = addr2, addr1
	}
	req1 := &vtypes.PendingSwapOut{VaultAddress: addr1.Bech32, Owner: addr1.Bech32, RedeemDenom: "ylds", Shares: sdk.NewInt64Coin("vshares", 25)}
	req2 := &vtypes.PendingSwapOut{VaultAddress: addr2.Bech32, Owner: addr2.Bech32, RedeemDenom: "ylds", Shares: sdk.NewInt64Coin("vshares", 50)}
	req3 := &vtypes.PendingSwapOut{VaultAddress: addr1.Bech32, Owner: addr2.Bech32, RedeemDenom: "ylds", Shares: sdk.NewInt64Coin("vshares", 100)}
	boom := errors.New("boom")

	tests := []struct {
		name         string
		setup        func(t *testing.T, ctx sdk.Context, q *queue.PendingSwapOutQueue)
		expectedErr  error
		expectedSeen []vtypes.PendingSwapOut
	}{
		{
			name: "success",
			setup: func(t *testing.T, ctx sdk.Context, q *queue.PendingSwapOutQueue) {
				_, err := q.Enqueue(ctx, 1, req1)
				require.NoError(t, err)
				_, err = q.Enqueue(ctx, 2, req2)
				require.NoError(t, err)
			},
			expectedSeen: []vtypes.PendingSwapOut{*req1, *req2},
		},
		{
			name: "success with order preservation",
			setup: func(t *testing.T, ctx sdk.Context, q *queue.PendingSwapOutQueue) {
				_, err := q.Enqueue(ctx, 1, req1)
				require.NoError(t, err)
				_, err = q.Enqueue(ctx, 2, req2)
				require.NoError(t, err)
				_, err = q.Enqueue(ctx, 3, req3)
				require.NoError(t, err)
			},
			expectedSeen: []vtypes.PendingSwapOut{*req1, *req2, *req3},
		},
		{
			name:         "empty",
			setup:        func(t *testing.T, ctx sdk.Context, q *queue.PendingSwapOutQueue) {},
			expectedSeen: []vtypes.PendingSwapOut{},
		},
		{
			name: "error",
			setup: func(t *testing.T, ctx sdk.Context, q *queue.PendingSwapOutQueue) {
				_, err := q.Enqueue(ctx, 1, req1)
				require.NoError(t, err)
				_, err = q.Enqueue(ctx, 2, req2)
				require.NoError(t, err)
			},
			expectedErr:  boom,
			expectedSeen: []vtypes.PendingSwapOut{*req1, *req2},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, q := newTestPendingSwapOutQueue(t)
			if tc.setup != nil {
				tc.setup(t, ctx, q)
			}

			seen := []vtypes.PendingSwapOut{}
			calls := 0
			err := q.Walk(ctx, func(timestamp int64, id uint64, vault sdk.AccAddress, req vtypes.PendingSwapOut) (stop bool, err error) {
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

func TestPendingSwapOutQueueEnqueueAndDequeue(t *testing.T) {
	addr1 := utils.TestProvlabsAddress()
	addr2 := utils.TestProvlabsAddress()

	tests := []struct {
		name        string
		setup       func(t *testing.T, ctx sdk.Context, q *queue.PendingSwapOutQueue) (int64, sdk.AccAddress, uint64)
		timestamp   int64
		req         *vtypes.PendingSwapOut
		dequeue     bool
		expectError bool
		errorMsg    string
	}{
		{
			name: "successful enqueue and dequeue",
			setup: func(t *testing.T, ctx sdk.Context, q *queue.PendingSwapOutQueue) (int64, sdk.AccAddress, uint64) {
				req := &vtypes.PendingSwapOut{VaultAddress: addr1.Bech32, Owner: addr1.Bech32}
				id, err := q.Enqueue(ctx, 1, req)
				require.NoError(t, err)
				return 1, sdk.MustAccAddressFromBech32(addr1.Bech32), id
			},
			dequeue:     true,
			expectError: false,
		},
		{
			name: "dequeue non-existent key",
			setup: func(t *testing.T, ctx sdk.Context, q *queue.PendingSwapOutQueue) (int64, sdk.AccAddress, uint64) {
				return 1, sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32), 999
			},
			dequeue:     true,
			expectError: false, // Dequeue with a non-existent key should not error.
		},
		{
			name: "enqueue with invalid vault address",
			req: &vtypes.PendingSwapOut{
				VaultAddress: "invalid",
			},
			timestamp:   1,
			dequeue:     false,
			expectError: true,
		},
		{
			name: "enqueue with negative timestamp",
			req: &vtypes.PendingSwapOut{
				VaultAddress: addr2.Bech32,
			},
			timestamp:   -1,
			dequeue:     false,
			expectError: true,
			errorMsg:    "pending time cannot be negative",
		},
		{
			name: "dequeue with negative timestamp",
			setup: func(t *testing.T, ctx sdk.Context, q *queue.PendingSwapOutQueue) (int64, sdk.AccAddress, uint64) {
				return -1, sdk.MustAccAddressFromBech32(utils.TestProvlabsAddress().Bech32), 1
			},
			dequeue:     true,
			expectError: true,
			errorMsg:    "timestamp cannot be negative",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, q := newTestPendingSwapOutQueue(t)

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

func TestPendingSwapOutQueue_GetByID(t *testing.T) {
	addr1 := utils.TestProvlabsAddress()
	req1 := &vtypes.PendingSwapOut{VaultAddress: addr1.Bech32, Owner: addr1.Bech32}

	tests := map[string]struct {
		setup    func(t *testing.T, ctx sdk.Context, q *queue.PendingSwapOutQueue) uint64
		id       uint64
		errorMsg string
	}{
		"success": {
			setup: func(t *testing.T, ctx sdk.Context, q *queue.PendingSwapOutQueue) uint64 {
				id, err := q.Enqueue(ctx, 123, req1)
				require.NoError(t, err)
				return id
			},
		},
		"failure - invalid id": {
			setup: func(t *testing.T, ctx sdk.Context, q *queue.PendingSwapOutQueue) uint64 {
				return 999
			},
			errorMsg: "not found",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctx, q := newTestPendingSwapOutQueue(t)
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

func TestPendingSwapOutQueue_ExpediteSwapOut(t *testing.T) {
	addr1 := utils.TestProvlabsAddress()
	addr2 := utils.TestProvlabsAddress()
	req1 := &vtypes.PendingSwapOut{VaultAddress: addr1.Bech32, Owner: addr1.Bech32}
	req2 := &vtypes.PendingSwapOut{VaultAddress: addr2.Bech32, Owner: addr2.Bech32}

	tests := map[string]struct {
		setup      func(t *testing.T, ctx sdk.Context, q *queue.PendingSwapOutQueue) (uint64, int64, sdk.AccAddress)
		expediteId uint64
		errorMsg   string
	}{
		"success with one entry": {
			setup: func(t *testing.T, ctx sdk.Context, q *queue.PendingSwapOutQueue) (uint64, int64, sdk.AccAddress) {
				id, err := q.Enqueue(ctx, 123, req1)
				require.NoError(t, err)
				return id, 123, sdk.MustAccAddressFromBech32(addr1.Bech32)
			},
		},
		"success with two entries, second is expedited": {
			setup: func(t *testing.T, ctx sdk.Context, q *queue.PendingSwapOutQueue) (uint64, int64, sdk.AccAddress) {
				_, err := q.Enqueue(ctx, 123, req1)
				require.NoError(t, err)
				id2, err := q.Enqueue(ctx, 456, req2)
				require.NoError(t, err)
				return id2, 456, sdk.MustAccAddressFromBech32(addr2.Bech32)
			},
		},
		"success on entry with timestamp 0": {
			setup: func(t *testing.T, ctx sdk.Context, q *queue.PendingSwapOutQueue) (uint64, int64, sdk.AccAddress) {
				id, err := q.Enqueue(ctx, 0, req1)
				require.NoError(t, err)
				return id, 0, sdk.MustAccAddressFromBech32(addr1.Bech32)
			},
		},
		"failure if entry does not exist": {
			setup: func(t *testing.T, ctx sdk.Context, q *queue.PendingSwapOutQueue) (uint64, int64, sdk.AccAddress) {
				return 999, 0, nil
			},
			expediteId: 999,
			errorMsg:   "not found",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctx, q := newTestPendingSwapOutQueue(t)
			id, oldTimestamp, addr := tc.setup(t, ctx, q)
			if tc.expediteId != 0 {
				id = tc.expediteId
			}

			err := q.ExpediteSwapOut(ctx, id)

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

func TestPendingSwapOutQueue_Export(t *testing.T) {
	addr1 := utils.TestProvlabsAddress()
	addr2 := utils.TestProvlabsAddress()
	if addr1.Bech32 > addr2.Bech32 {
		addr1, addr2 = addr2, addr1
	}
	req1 := &vtypes.PendingSwapOut{VaultAddress: addr1.Bech32, Owner: addr1.Bech32, RedeemDenom: "usd"}
	req2 := &vtypes.PendingSwapOut{VaultAddress: addr2.Bech32, Owner: addr2.Bech32, RedeemDenom: "usd"}
	req3 := &vtypes.PendingSwapOut{VaultAddress: addr1.Bech32, Owner: addr1.Bech32, RedeemDenom: "usd"}

	tests := []struct {
		name          string
		setup         func(t *testing.T, ctx sdk.Context, q *queue.PendingSwapOutQueue)
		expectedQueue vtypes.PendingSwapOutQueue
		expectError   bool
	}{
		{
			name: "multiple elements",
			setup: func(t *testing.T, ctx sdk.Context, q *queue.PendingSwapOutQueue) {
				_, err := q.Enqueue(ctx, 1, req1)
				require.NoError(t, err)
				_, err = q.Enqueue(ctx, 2, req2)
				require.NoError(t, err)
				_, err = q.Enqueue(ctx, 1, req3)
				require.NoError(t, err)
			},
			expectedQueue: vtypes.PendingSwapOutQueue{
				LatestSequenceNumber: 3,
				Entries: []vtypes.PendingSwapOutQueueEntry{
					{Time: 1, Id: 0, SwapOut: *req1},
					{Time: 1, Id: 2, SwapOut: *req3},
					{Time: 2, Id: 1, SwapOut: *req2},
				},
			},
			expectError: false,
		},
		{
			name: "no elements",
			setup: func(t *testing.T, ctx sdk.Context, q *queue.PendingSwapOutQueue) {
				// No elements enqueued
			},
			expectedQueue: vtypes.PendingSwapOutQueue{
				LatestSequenceNumber: 0,
				Entries:              []vtypes.PendingSwapOutQueueEntry{},
			},
			expectError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, q := newTestPendingSwapOutQueue(t)
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
					require.Equal(t, entry.SwapOut.Owner, exportedQueue.Entries[i].SwapOut.Owner)
					require.Equal(t, entry.SwapOut.VaultAddress, exportedQueue.Entries[i].SwapOut.VaultAddress)
					require.Equal(t, entry.SwapOut.RedeemDenom, exportedQueue.Entries[i].SwapOut.RedeemDenom)
				}
			}
		})
	}
}

func TestPendingSwapOutQueue_Import(t *testing.T) {
	addr1 := utils.TestProvlabsAddress()
	addr2 := utils.TestProvlabsAddress()
	if addr1.Bech32 > addr2.Bech32 {
		addr1, addr2 = addr2, addr1
	}
	req1 := vtypes.PendingSwapOut{VaultAddress: addr1.Bech32, Owner: addr1.Bech32, RedeemDenom: "usd", Shares: sdk.NewInt64Coin("vshares", 100)}
	req2 := vtypes.PendingSwapOut{VaultAddress: addr2.Bech32, Owner: addr2.Bech32, RedeemDenom: "usd", Shares: sdk.NewInt64Coin("vshares", 200)}
	req3 := vtypes.PendingSwapOut{VaultAddress: addr1.Bech32, Owner: addr1.Bech32, RedeemDenom: "usd", Shares: sdk.NewInt64Coin("vshares", 300)}

	tests := []struct {
		name     string
		genQueue *vtypes.PendingSwapOutQueue
		errorMsg string
	}{
		{
			name: "multiple elements",
			genQueue: &vtypes.PendingSwapOutQueue{
				LatestSequenceNumber: 2,
				Entries: []vtypes.PendingSwapOutQueueEntry{
					{Time: 1, Id: 0, SwapOut: req1},
					{Time: 2, Id: 1, SwapOut: req2},
					{Time: 1, Id: 2, SwapOut: req3},
				},
			},
		},
		{
			name: "empty list",
			genQueue: &vtypes.PendingSwapOutQueue{
				LatestSequenceNumber: 5,
				Entries:              []vtypes.PendingSwapOutQueueEntry{},
			},
		},
		{
			name:     "nil queue",
			genQueue: nil,
			errorMsg: "genesis queue is nil",
		},
		{
			name: "bad vault address",
			genQueue: &vtypes.PendingSwapOutQueue{
				LatestSequenceNumber: 1,
				Entries: []vtypes.PendingSwapOutQueueEntry{
					{Time: 1, Id: 0, SwapOut: vtypes.PendingSwapOut{VaultAddress: "badaddress", Owner: addr1.Bech32, RedeemDenom: "usd", Shares: sdk.NewInt64Coin("vshares", 100)}},
				},
			},
			errorMsg: "invalid vault address in pending swap out queue",
		},
		{
			name: "bad owner address",
			genQueue: &vtypes.PendingSwapOutQueue{
				LatestSequenceNumber: 1,
				Entries: []vtypes.PendingSwapOutQueueEntry{
					{Time: 1, Id: 0, SwapOut: vtypes.PendingSwapOut{VaultAddress: addr1.Bech32, Owner: "badaddress", RedeemDenom: "usd", Shares: sdk.NewInt64Coin("vshares", 100)}},
				},
			},
			errorMsg: "invalid owner address in pending swap out queue",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, q := newTestPendingSwapOutQueue(t)

			err := q.Import(ctx, tc.genQueue)

			if len(tc.errorMsg) > 0 {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errorMsg)
			} else {
				require.NoError(t, err)

				seq, err := q.Sequence.Peek(ctx)
				require.NoError(t, err)
				require.Equal(t, tc.genQueue.LatestSequenceNumber, seq)

				for _, entry := range tc.genQueue.Entries {
					vaultAddr, err := sdk.AccAddressFromBech32(entry.SwapOut.VaultAddress)
					require.NoError(t, err)
					req, err := q.IndexedMap.Get(ctx, collections.Join3(entry.Time, entry.Id, vaultAddr))
					require.NoError(t, err)
					require.Equal(t, entry.SwapOut, req)
				}
			}
		})
	}
}

