package queue

import (
	"context"
	"fmt"

	"github.com/provlabs/vault/types"

	"cosmossdk.io/collections"
	"cosmossdk.io/collections/indexes"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// PendingSwapOutIndexes defines the indexes for the pending swap out queue.
type PendingSwapOutIndexes struct {
	ByVault *indexes.Multi[sdk.AccAddress, collections.Triple[int64, uint64, sdk.AccAddress], types.PendingSwapOut]
	ByID    *indexes.Unique[uint64, collections.Triple[int64, uint64, sdk.AccAddress], types.PendingSwapOut]
}

// IndexesList returns the list of indexes for the pending swap out queue.
func (i PendingSwapOutIndexes) IndexesList() []collections.Index[collections.Triple[int64, uint64, sdk.AccAddress], types.PendingSwapOut] {
	return []collections.Index[
		collections.Triple[int64, uint64, sdk.AccAddress], types.PendingSwapOut]{i.ByVault, i.ByID}
}

// NewPendingSwapOutIndexes creates a new PendingSwapOutIndexes object.
func NewPendingSwapOutIndexes(sb *collections.SchemaBuilder) PendingSwapOutIndexes {
	return PendingSwapOutIndexes{
		ByVault: indexes.NewMulti(
			sb,
			types.VaultPendingSwapOutByVaultIndexPrefix,
			types.VaultPendingSwapOutByVaultIndexName,
			sdk.AccAddressKey,
			collections.TripleKeyCodec(collections.Int64Key, collections.Uint64Key, sdk.AccAddressKey),
			func(pk collections.Triple[int64, uint64, sdk.AccAddress], _ types.PendingSwapOut) (sdk.AccAddress, error) {
				return pk.K3(), nil
			},
		),
		ByID: indexes.NewUnique(
			sb,
			types.VaultPendingSwapOutByIdIndexPrefix,
			types.VaultPendingSwapOutByIdIndexName,
			collections.Uint64Key,
			collections.TripleKeyCodec(collections.Int64Key, collections.Uint64Key, sdk.AccAddressKey),
			func(pk collections.Triple[int64, uint64, sdk.AccAddress], _ types.PendingSwapOut) (uint64, error) {
				return pk.K2(), nil
			},
		),
	}
}

// PendingSwapOutQueue is a queue for pending swap outs.
type PendingSwapOutQueue struct {
	// IndexedMap is the indexed map of pending swap outs. The key is a triple of (timestamp, id, vault).
	IndexedMap *collections.IndexedMap[collections.Triple[int64, uint64, sdk.AccAddress], types.PendingSwapOut, PendingSwapOutIndexes]
	// Sequence is the sequence for generating unique swap out IDs.
	Sequence collections.Sequence
}

// NewPendingSwapOutQueue creates a new PendingSwapOutQueue.
func NewPendingSwapOutQueue(builder *collections.SchemaBuilder, cdc codec.BinaryCodec) *PendingSwapOutQueue {
	keyCodec := collections.TripleKeyCodec(
		collections.Int64Key,
		collections.Uint64Key,
		sdk.AccAddressKey,
	)
	valueCodec := codec.CollValue[types.PendingSwapOut](cdc)
	return &PendingSwapOutQueue{
		IndexedMap: collections.NewIndexedMap(
			builder,
			types.VaultPendingSwapOutQueuePrefix,
			types.VaultPendingSwapOutQueueName,
			keyCodec,
			valueCodec,
			NewPendingSwapOutIndexes(builder),
		),
		Sequence: collections.NewSequence(builder, types.VaultPendingSwapOutQueueSeqPrefix, types.VaultPendingSwapOutQueueSeqName),
	}
}

// Enqueue adds a pending swap out to the queue.
func (p *PendingSwapOutQueue) Enqueue(ctx context.Context, pendingTime int64, req *types.PendingSwapOut) (uint64, error) {
	if pendingTime < 0 {
		return 0, fmt.Errorf("pending time cannot be negative")
	}
	if err := req.Validate(); err != nil {
		return 0, fmt.Errorf("invalid pending swap out request: %w", err)
	}
	vault, err := sdk.AccAddressFromBech32(req.VaultAddress)
	if err != nil {
		return 0, err
	}
	id, err := p.Sequence.Next(ctx)
	if err != nil {
		return 0, err
	}
	return id, p.IndexedMap.Set(ctx, collections.Join3(pendingTime, id, vault), *req)
}

// Dequeue removes a pending swap out from the queue.
func (p *PendingSwapOutQueue) Dequeue(ctx context.Context, timestamp int64, vault sdk.AccAddress, id uint64) error {
	if timestamp < 0 {
		return fmt.Errorf("timestamp cannot be negative")
	}
	key := collections.Join3(timestamp, id, vault)
	if ok, _ := p.IndexedMap.Has(ctx, key); !ok {
		return nil
	}
	return p.IndexedMap.Remove(ctx, key)
}

// GetByID gets the pending swap out by ID.
func (p *PendingSwapOutQueue) GetByID(ctx context.Context, id uint64) (int64, *types.PendingSwapOut, error) {
	pk, err := p.IndexedMap.Indexes.ByID.MatchExact(ctx, id)
	if err != nil {
		return 0, nil, err
	}

	req, err := p.IndexedMap.Get(ctx, pk)
	if err != nil {
		return 0, nil, err
	}

	return pk.K1(), &req, nil
}

// ExpediteSwapOut sets the timestamp of a pending swap out to 0.
func (p *PendingSwapOutQueue) ExpediteSwapOut(ctx context.Context, id uint64) error {
	pk, err := p.IndexedMap.Indexes.ByID.MatchExact(ctx, id)
	if err != nil {
		return err
	}

	req, err := p.IndexedMap.Get(ctx, pk)
	if err != nil {
		return err
	}

	if err := p.IndexedMap.Remove(ctx, pk); err != nil {
		return err
	}

	return p.IndexedMap.Set(ctx, collections.Join3(int64(0), pk.K2(), pk.K3()), req)
}

// WalkDue iterates over all entries in the PendingSwapOutQueue with
// a timestamp <= now. For each due entry, the callback is invoked.
// Iteration stops when a key with time > now is encountered (since keys are
// ordered) or when the callback returns stop=true or an error.
func (p *PendingSwapOutQueue) WalkDue(ctx context.Context, now int64, fn func(timestamp int64, id uint64, vault sdk.AccAddress, req types.PendingSwapOut) (stop bool, err error)) error {
	return p.IndexedMap.Walk(ctx, nil, func(key collections.Triple[int64, uint64, sdk.AccAddress], value types.PendingSwapOut) (stop bool, err error) {
		if key.K1() > now {
			return true, nil
		}
		return fn(key.K1(), key.K2(), key.K3(), value)
	})
}

// Walk iterates over all entries in the PendingSwapOutQueue.
// Iteration stops when the callback returns stop=true or an error.
func (p *PendingSwapOutQueue) Walk(ctx context.Context, fn func(timestamp int64, id uint64, vault sdk.AccAddress, req types.PendingSwapOut) (stop bool, err error)) error {
	return p.IndexedMap.Walk(ctx, nil, func(key collections.Triple[int64, uint64, sdk.AccAddress], value types.PendingSwapOut) (stop bool, err error) {
		return fn(key.K1(), key.K2(), key.K3(), value)
	})
}

// WalkByVault iterates over all entries in the PendingSwapOutQueue for a specific vault.
// Iteration stops when the callback returns stop=true or an error.
func (p *PendingSwapOutQueue) WalkByVault(ctx context.Context, vaultAddr sdk.AccAddress, fn func(timestamp int64, id uint64, req types.PendingSwapOut) (stop bool, err error)) error {
	iter, err := p.IndexedMap.Indexes.ByVault.MatchExact(ctx, vaultAddr)
	if err != nil {
		return err
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		pk, err := iter.PrimaryKey()
		if err != nil {
			return err
		}
		req, err := p.IndexedMap.Get(ctx, pk)
		if err != nil {
			return err
		}
		if stop, err := fn(pk.K1(), pk.K2(), req); stop || err != nil {
			return err
		}
	}
	return nil
}

// Import imports the pending swap out queue from genesis.
func (p *PendingSwapOutQueue) Import(ctx context.Context, genQueue *types.PendingSwapOutQueue) error {
	if genQueue == nil {
		return fmt.Errorf("genesis queue is nil")
	}
	for _, entry := range genQueue.Entries {
		vaultAddr, err := sdk.AccAddressFromBech32(entry.SwapOut.VaultAddress)
		if err != nil {
			return fmt.Errorf("invalid vault address in pending swap out queue: %w", err)
		}
		if _, err := sdk.AccAddressFromBech32(entry.SwapOut.Owner); err != nil {
			return fmt.Errorf("invalid owner address in pending swap out queue: %w", err)
		}
		swapOut := types.PendingSwapOut{
			Owner:        entry.SwapOut.Owner,
			RedeemDenom:  entry.SwapOut.RedeemDenom,
			Shares:       entry.SwapOut.Shares,
			VaultAddress: entry.SwapOut.VaultAddress,
		}

		if err := p.IndexedMap.Set(ctx, collections.Join3(entry.Time, entry.Id, vaultAddr), swapOut); err != nil {
			return fmt.Errorf("failed to enqueue pending swap out: %w", err)
		}
	}
	if err := p.Sequence.Set(ctx, genQueue.LatestSequenceNumber); err != nil {
		return fmt.Errorf("failed to set latest sequence number for pending swap out queue: %w", err)
	}
	return nil
}

// Export exports the pending swap out queue to genesis.
func (p *PendingSwapOutQueue) Export(ctx context.Context) (*types.PendingSwapOutQueue, error) {
	pendingSwapOutQueue := make([]types.PendingSwapOutQueueEntry, 0)
	err := p.Walk(ctx, func(timestamp int64, id uint64, _ sdk.AccAddress, req types.PendingSwapOut) (stop bool, err error) {
		pendingSwapOutQueue = append(pendingSwapOutQueue, types.PendingSwapOutQueueEntry{
			Time:    timestamp,
			Id:      id,
			SwapOut: req,
		})
		return false, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk pending swap out queue: %w", err)
	}

	latestSequenceNumber, err := p.Sequence.Peek(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest sequence number for pending swap out queue: %w", err)
	}

	return &types.PendingSwapOutQueue{
			LatestSequenceNumber: latestSequenceNumber,
			Entries:              pendingSwapOutQueue,
		},
		nil
}
