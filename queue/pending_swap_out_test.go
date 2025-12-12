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
	require.NoError(t, err, "schema build should not error")
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
	require.NoError(t, err, "enqueue should succeed for valid request")

	var retrievedReq vtypes.PendingSwapOut
	retrieved := false
	err = q.Walk(ctx, func(timestamp int64, id uint64, vault sdk.AccAddress, req vtypes.PendingSwapOut) (stop bool, err error) {
		retrievedReq = req
		retrieved = true
		return true, nil
	})

	require.NoError(t, err, "walk should not error")
	require.True(t, retrieved, "did not retrieve any request from the queue")

	require.Equal(t, originalReq.Owner, retrievedReq.Owner, "retrieved owner must match original owner")
	require.Equal(t, originalReq.VaultAddress, retrievedReq.VaultAddress, "retrieved vault address must match original vault address")
	require.Equal(t, originalReq.RedeemDenom, retrievedReq.RedeemDenom, "retrieved redeem denom must match original redeem denom")
	require.Equal(t, originalReq.Shares, retrievedReq.Shares, "retrieved shares must match original shares")
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
				require.NoError(t, err, "enqueue req1 should succeed")
				_, err = q.Enqueue(ctx, 2, req2)
				require.NoError(t, err, "enqueue req2 should succeed")
			},
			expectedSeen: []vtypes.PendingSwapOut{*req1, *req2},
		},
		{
			name: "success with order preservation",
			setup: func(t *testing.T, ctx sdk.Context, q *queue.PendingSwapOutQueue) {
				_, err := q.Enqueue(ctx, 1, req1)
				require.NoError(t, err, "enqueue req1 should succeed")
				_, err = q.Enqueue(ctx, 2, req2)
				require.NoError(t, err, "enqueue req2 should succeed")
				_, err = q.Enqueue(ctx, 3, req3)
				require.NoError(t, err, "enqueue req3 should succeed")
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
				require.NoError(t, err, "enqueue req1 should succeed")
				_, err = q.Enqueue(ctx, 2, req2)
				require.NoError(t, err, "enqueue req2 should succeed")
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
				require.ErrorIs(t, err, tc.expectedErr, "walk should return the expected error")
			} else {
				require.NoError(t, err, "walk should not return an error")
			}
			require.Equal(t, tc.expectedSeen, seen, "walk should iterate over all expected entries")
		})
	}
}

func TestPendingSwapOutQueueEnqueueAndDequeue(t *testing.T) {
	addr1 := utils.TestProvlabsAddress()
	addr2 := utils.TestProvlabsAddress()

	addr1Acc := sdk.MustAccAddressFromBech32(addr1.Bech32)
	validShares := sdk.NewInt64Coin("vshare", 10)
	validRedeem := "uusd"

	validReq := vtypes.NewPendingSwapOut(
		addr1Acc,
		addr1Acc,
		validShares,
		validRedeem,
	)

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
				req := vtypes.NewPendingSwapOut(addr1Acc, addr1Acc, sdk.NewInt64Coin("vshares", 100), "usd")
				id, err := q.Enqueue(ctx, 1, &req)
				require.NoError(t, err, "enqueue must succeed")
				return 1, addr1Acc, id
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
			expectError: false,
		},
		{
			name: "enqueue with invalid vault address (pre-validation, now fails validation)",
			req: &vtypes.PendingSwapOut{
				Owner:        validReq.Owner,
				VaultAddress: "invalid",
				Shares:       validShares,
				RedeemDenom:  validRedeem,
			},
			timestamp:   1,
			dequeue:     false,
			expectError: true,
			errorMsg:    "invalid pending swap out request: invalid vault address invalid:",
		},
		{
			name: "enqueue fails validation (invalid owner address)",
			req: &vtypes.PendingSwapOut{
				Owner:        "invalidowner",
				VaultAddress: validReq.VaultAddress,
				Shares:       validShares,
				RedeemDenom:  validRedeem,
			},
			timestamp:   1,
			dequeue:     false,
			expectError: true,
			errorMsg:    "invalid pending swap out request: invalid owner address invalidowner:",
		},
		{
			name: "enqueue fails validation (zero shares)",
			req: &vtypes.PendingSwapOut{
				Owner:        validReq.Owner,
				VaultAddress: validReq.VaultAddress,
				Shares:       sdk.NewInt64Coin("vshare", 0),
				RedeemDenom:  validRedeem,
			},
			timestamp:   1,
			dequeue:     false,
			expectError: true,
			errorMsg:    "shares cannot be zero",
		},
		{
			name: "enqueue fails validation (empty redeem denom)",
			req: &vtypes.PendingSwapOut{
				Owner:        validReq.Owner,
				VaultAddress: validReq.VaultAddress,
				Shares:       validShares,
				RedeemDenom:  "",
			},
			timestamp:   1,
			dequeue:     false,
			expectError: true,
			errorMsg:    "invalid pending swap out request: redeem denom cannot be empty",
		},
		{
			name: "enqueue with negative timestamp",
			req: &vtypes.PendingSwapOut{
				VaultAddress: addr2.Bech32,
				Owner:        validReq.Owner,
				Shares:       validReq.Shares,
				RedeemDenom:  validReq.RedeemDenom,
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
					require.Error(t, err, "dequeue should return an error")
					if tc.errorMsg != "" {
						require.Contains(t, err.Error(), tc.errorMsg, "dequeue error message should contain expected text")
					}
				} else {
					require.NoError(t, err, "dequeue should not return an error")
					_, err = q.IndexedMap.Get(ctx, collections.Join3(timestamp, id, addr))
					require.Error(t, err, "get should fail after successful dequeue")
					require.ErrorIs(t, err, collections.ErrNotFound, "error should be collections.ErrNotFound")
				}
			} else {
				_, err := q.Enqueue(ctx, tc.timestamp, tc.req)
				if tc.expectError {
					require.Error(t, err, "enqueue should return an error")
					if tc.errorMsg != "" {
						require.Contains(t, err.Error(), tc.errorMsg, "enqueue error message should contain expected text")
					}
				} else {
					require.NoError(t, err, "enqueue should not return an error")
				}
			}
		})
	}
}

func TestPendingSwapOutQueue_GetByID(t *testing.T) {
	addr1 := utils.TestProvlabsAddress()
	req1 := &vtypes.PendingSwapOut{VaultAddress: addr1.Bech32, Owner: addr1.Bech32, RedeemDenom: "usd", Shares: sdk.NewInt64Coin("vshares", 100)}

	tests := map[string]struct {
		setup    func(t *testing.T, ctx sdk.Context, q *queue.PendingSwapOutQueue) uint64
		id       uint64
		errorMsg string
	}{
		"success": {
			setup: func(t *testing.T, ctx sdk.Context, q *queue.PendingSwapOutQueue) uint64 {
				id, err := q.Enqueue(ctx, 123, req1)
				require.NoError(t, err, "enqueue should succeed")
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
				require.Error(t, err, "GetByID should return an error")
				require.ErrorContains(t, err, tc.errorMsg, "error message should contain expected text")
				require.Equal(t, int64(0), pendingTime, "pending time should be zero on error")
			} else {
				require.NoError(t, err, "GetByID should not return an error")
				require.Equal(t, req1.VaultAddress, req.VaultAddress, "retrieved vault address should match")
				require.Equal(t, req1.Owner, req.Owner, "retrieved owner should match")
				require.Equal(t, int64(123), pendingTime, "retrieved timestamp should match")
			}
		})
	}
}

func TestPendingSwapOutQueue_ExpediteSwapOut(t *testing.T) {
	addr1 := utils.TestProvlabsAddress()
	addr2 := utils.TestProvlabsAddress()
	req1 := &vtypes.PendingSwapOut{VaultAddress: addr1.Bech32, Owner: addr1.Bech32, RedeemDenom: "usd", Shares: sdk.NewInt64Coin("vshares", 100)}
	req2 := &vtypes.PendingSwapOut{VaultAddress: addr2.Bech32, Owner: addr2.Bech32, RedeemDenom: "usd", Shares: sdk.NewInt64Coin("vshares", 100)}

	tests := map[string]struct {
		setup      func(t *testing.T, ctx sdk.Context, q *queue.PendingSwapOutQueue) (uint64, int64, sdk.AccAddress)
		expediteId uint64
		errorMsg   string
	}{
		"success with one entry": {
			setup: func(t *testing.T, ctx sdk.Context, q *queue.PendingSwapOutQueue) (uint64, int64, sdk.AccAddress) {
				id, err := q.Enqueue(ctx, 123, req1)
				require.NoError(t, err, "enqueue should succeed")
				return id, 123, sdk.MustAccAddressFromBech32(addr1.Bech32)
			},
		},
		"success with two entries, second is expedited": {
			setup: func(t *testing.T, ctx sdk.Context, q *queue.PendingSwapOutQueue) (uint64, int64, sdk.AccAddress) {
				_, err := q.Enqueue(ctx, 123, req1)
				require.NoError(t, err, "enqueue req1 should succeed")
				id2, err := q.Enqueue(ctx, 456, req2)
				require.NoError(t, err, "enqueue req2 should succeed")
				return id2, 456, sdk.MustAccAddressFromBech32(addr2.Bech32)
			},
		},
		"success on entry with timestamp 0": {
			setup: func(t *testing.T, ctx sdk.Context, q *queue.PendingSwapOutQueue) (uint64, int64, sdk.AccAddress) {
				id, err := q.Enqueue(ctx, 0, req1)
				require.NoError(t, err, "enqueue should succeed")
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
				require.Error(t, err, "ExpediteSwapOut should return an error")
				require.ErrorContains(t, err, tc.errorMsg, "error message should contain expected text")
			} else {
				require.NoError(t, err, "ExpediteSwapOut should not return an error")

				if oldTimestamp != 0 {
					_, err = q.IndexedMap.Get(ctx, collections.Join3(oldTimestamp, id, addr))
					require.ErrorContains(t, err, "not found", "old entry should be removed")
				}

				newKey := collections.Join3(int64(0), id, addr)
				_, err = q.IndexedMap.Get(ctx, newKey)
				require.NoError(t, err, "new entry with timestamp 0 should exist")
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
	req1 := &vtypes.PendingSwapOut{VaultAddress: addr1.Bech32, Owner: addr1.Bech32, RedeemDenom: "usd", Shares: sdk.NewInt64Coin("vshares", 100)}
	req2 := &vtypes.PendingSwapOut{VaultAddress: addr2.Bech32, Owner: addr2.Bech32, RedeemDenom: "usd", Shares: sdk.NewInt64Coin("vshares", 200)}
	req3 := &vtypes.PendingSwapOut{VaultAddress: addr1.Bech32, Owner: addr1.Bech32, RedeemDenom: "usd", Shares: sdk.NewInt64Coin("vshares", 300)}

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
				require.NoError(t, err, "enqueue req1 should succeed")
				_, err = q.Enqueue(ctx, 2, req2)
				require.NoError(t, err, "enqueue req2 should succeed")
				_, err = q.Enqueue(ctx, 1, req3)
				require.NoError(t, err, "enqueue req3 should succeed")
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
				require.Error(t, err, "export should return an error")
			} else {
				require.NoError(t, err, "export should not return an error")
				require.Equal(t, tc.expectedQueue.LatestSequenceNumber, exportedQueue.LatestSequenceNumber, "latest sequence number should match")
				require.Equal(t, len(tc.expectedQueue.Entries), len(exportedQueue.Entries), "number of exported entries should match")
				for i, entry := range tc.expectedQueue.Entries {
					require.Equal(t, entry.Time, exportedQueue.Entries[i].Time, "exported entry time should match")
					require.Equal(t, entry.Id, exportedQueue.Entries[i].Id, "exported entry ID should match")
					require.Equal(t, entry.SwapOut.Owner, exportedQueue.Entries[i].SwapOut.Owner, "exported entry owner should match")
					require.Equal(t, entry.SwapOut.VaultAddress, exportedQueue.Entries[i].SwapOut.VaultAddress, "exported entry vault address should match")
					require.Equal(t, entry.SwapOut.RedeemDenom, exportedQueue.Entries[i].SwapOut.RedeemDenom, "exported entry redeem denom should match")
					require.Equal(t, entry.SwapOut.Shares, exportedQueue.Entries[i].SwapOut.Shares, "exported entry shares should match")
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
			errorMsg: "invalid vault address in pending swap out queue:",
		},
		{
			name: "bad owner address",
			genQueue: &vtypes.PendingSwapOutQueue{
				LatestSequenceNumber: 1,
				Entries: []vtypes.PendingSwapOutQueueEntry{
					{Time: 1, Id: 0, SwapOut: vtypes.PendingSwapOut{VaultAddress: addr1.Bech32, Owner: "badaddress", RedeemDenom: "usd", Shares: sdk.NewInt64Coin("vshares", 100)}},
				},
			},
			errorMsg: "invalid owner address in pending swap out queue:",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, q := newTestPendingSwapOutQueue(t)

			err := q.Import(ctx, tc.genQueue)

			if len(tc.errorMsg) > 0 {
				require.Error(t, err, "import should return an error")
				require.Contains(t, err.Error(), tc.errorMsg, "import error message should contain expected text")
			} else {
				require.NoError(t, err, "import should not return an error")

				seq, err := q.Sequence.Peek(ctx)
				require.NoError(t, err, "sequence peek should not error")
				require.Equal(t, tc.genQueue.LatestSequenceNumber, seq, "latest sequence number should match imported value")

				for _, entry := range tc.genQueue.Entries {
					vaultAddr, err := sdk.AccAddressFromBech32(entry.SwapOut.VaultAddress)
					require.NoError(t, err, "vault address should be valid")
					req, err := q.IndexedMap.Get(ctx, collections.Join3(entry.Time, entry.Id, vaultAddr))
					require.NoError(t, err, "indexed map get should not error")
					require.Equal(t, entry.SwapOut, req, "retrieved request should match imported request")
				}
			}
		})
	}
}
