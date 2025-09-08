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

// PendingWithdrawalIndexes defines the indexes for the pending withdrawal queue.
type PendingWithdrawalIndexes struct {
	ByVault *indexes.Multi[sdk.AccAddress, collections.Triple[int64, uint64, sdk.AccAddress], types.PendingWithdrawal]
	ByID    *indexes.Unique[uint64, collections.Triple[int64, uint64, sdk.AccAddress], types.PendingWithdrawal]
}

// IndexesList returns the list of indexes for the pending withdrawal queue.
func (i PendingWithdrawalIndexes) IndexesList() []collections.Index[collections.Triple[int64, uint64, sdk.AccAddress], types.PendingWithdrawal] {
	return []collections.Index[
		collections.Triple[int64, uint64, sdk.AccAddress], types.PendingWithdrawal]{i.ByVault, i.ByID}
}

// NewPendingWithdrawalIndexes creates a new PendingWithdrawalIndexes object.
func NewPendingWithdrawalIndexes(sb *collections.SchemaBuilder) PendingWithdrawalIndexes {
	return PendingWithdrawalIndexes{
		ByVault: indexes.NewMulti(
			sb,
			types.VaultPendingWithdrawalByVaultIndexPrefix,
			types.VaultPendingWithdrawalByVaultIndexName,
			sdk.AccAddressKey,
			collections.TripleKeyCodec(collections.Int64Key, collections.Uint64Key, sdk.AccAddressKey),
			func(pk collections.Triple[int64, uint64, sdk.AccAddress], _ types.PendingWithdrawal) (sdk.AccAddress, error) {
				return pk.K3(), nil
			},
		),
		ByID: indexes.NewUnique(
			sb,
			types.VaultPendingWithdrawalByIdIndexPrefix,
			types.VaultPendingWithdrawalByIdIndexName,
			collections.Uint64Key,
			collections.TripleKeyCodec(collections.Int64Key, collections.Uint64Key, sdk.AccAddressKey),
			func(pk collections.Triple[int64, uint64, sdk.AccAddress], _ types.PendingWithdrawal) (uint64, error) {
				return pk.K2(), nil
			},
		),
	}
}

// PendingWithdrawalQueue is a queue for pending withdrawals.
type PendingWithdrawalQueue struct {
	// IndexedMap is the indexed map of pending withdrawals. The key is a triple of (timestamp, id, vault).
	IndexedMap *collections.IndexedMap[collections.Triple[int64, uint64, sdk.AccAddress], types.PendingWithdrawal, PendingWithdrawalIndexes]
	// Sequence is the sequence for generating unique withdrawal IDs.
	Sequence collections.Sequence
}

// NewPendingWithdrawalQueue creates a new PendingWithdrawalQueue.
func NewPendingWithdrawalQueue(builder *collections.SchemaBuilder, cdc codec.BinaryCodec) *PendingWithdrawalQueue {
	keyCodec := collections.TripleKeyCodec(
		collections.Int64Key,
		collections.Uint64Key,
		sdk.AccAddressKey,
	)
	valueCodec := codec.CollValue[types.PendingWithdrawal](cdc)
	return &PendingWithdrawalQueue{
		IndexedMap: collections.NewIndexedMap(
			builder,
			types.VaultPendingWithdrawalQueuePrefix,
			types.VaultPendingWithdrawalQueueName,
			keyCodec,
			valueCodec,
			NewPendingWithdrawalIndexes(builder),
		),
		Sequence: collections.NewSequence(builder, types.VaultPendingWithdrawalQueueSeqPrefix, types.VaultPendingWithdrawalQueueSeqName),
	}
}

// Enqueue adds a pending withdrawal to the queue.
func (p *PendingWithdrawalQueue) Enqueue(ctx context.Context, pendingTime int64, req *types.PendingWithdrawal) (uint64, error) {
	if pendingTime < 0 {
		return 0, fmt.Errorf("pending time cannot be negative")
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

// Dequeue removes a pending withdrawal from the queue.
func (p *PendingWithdrawalQueue) Dequeue(ctx context.Context, timestamp int64, vault sdk.AccAddress, id uint64) error {
	if timestamp < 0 {
		return fmt.Errorf("timestamp cannot be negative")
	}
	key := collections.Join3(timestamp, id, vault)
	if ok, _ := p.IndexedMap.Has(ctx, key); !ok {
		return nil
	}
	return p.IndexedMap.Remove(ctx, key)
}

// GetByID gets the pending withdrawal by ID.
func (p *PendingWithdrawalQueue) GetByID(ctx context.Context, id uint64) (int64, *types.PendingWithdrawal, error) {
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

// ExpediteWithdrawal sets the timestamp of a pending withdrawal to 0.
func (p *PendingWithdrawalQueue) ExpediteWithdrawal(ctx context.Context, id uint64) error {
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

// WalkDue iterates over all entries in the PendingWithdrawalQueue with
// a timestamp <= now. For each due entry, the callback is invoked.
// Iteration stops when a key with time > now is encountered (since keys are
// ordered) or when the callback returns stop=true or an error.
func (p *PendingWithdrawalQueue) WalkDue(ctx context.Context, now int64, fn func(timestamp int64, id uint64, vault sdk.AccAddress, req types.PendingWithdrawal) (stop bool, err error)) error {
	return p.IndexedMap.Walk(ctx, nil, func(key collections.Triple[int64, uint64, sdk.AccAddress], value types.PendingWithdrawal) (stop bool, err error) {
		if key.K1() > now {
			return true, nil
		}
		return fn(key.K1(), key.K2(), key.K3(), value)
	})
}

// Walk iterates over all entries in the PendingWithdrawalQueue.
// Iteration stops when the callback returns stop=true or an error.
func (p *PendingWithdrawalQueue) Walk(ctx context.Context, fn func(timestamp int64, id uint64, vault sdk.AccAddress, req types.PendingWithdrawal) (stop bool, err error)) error {
	return p.IndexedMap.Walk(ctx, nil, func(key collections.Triple[int64, uint64, sdk.AccAddress], value types.PendingWithdrawal) (stop bool, err error) {
		return fn(key.K1(), key.K2(), key.K3(), value)
	})
}

// WalkByVault iterates over all entries in the PendingWithdrawalQueue for a specific vault.
// Iteration stops when the callback returns stop=true or an error.
func (p *PendingWithdrawalQueue) WalkByVault(ctx context.Context, vaultAddr sdk.AccAddress, fn func(timestamp int64, id uint64, req types.PendingWithdrawal) (stop bool, err error)) error {
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

// Import imports the pending withdrawal queue from genesis.
func (p *PendingWithdrawalQueue) Import(ctx context.Context, genQueue *types.PendingWithdrawalQueue) error {
	for _, entry := range genQueue.Entries {
		vaultAddr, err := sdk.AccAddressFromBech32(entry.Withdrawal.VaultAddress)
		if err != nil {
			return fmt.Errorf("invalid vault address in pending withdrawal queue: %w", err)
		}
		withdrawal := types.PendingWithdrawal{
			Owner:        entry.Withdrawal.Owner,
			Assets:       entry.Withdrawal.Assets,
			VaultAddress: entry.Withdrawal.VaultAddress,
		}

		if err := p.IndexedMap.Set(ctx, collections.Join3(entry.Time, entry.Id, vaultAddr), withdrawal); err != nil {
			return fmt.Errorf("failed to enqueue pending withdrawal: %w", err)
		}
	}
	if err := p.Sequence.Set(ctx, genQueue.LatestSequenceNumber); err != nil {
		return fmt.Errorf("failed to set latest sequence number for pending withdrawal queue: %w", err)
	}
	return nil
}

// Export exports the pending withdrawal queue to genesis.
func (p *PendingWithdrawalQueue) Export(ctx context.Context) (*types.PendingWithdrawalQueue, error) {
	pendingWithdrawalQueue := make([]types.PendingWithdrawalQueueEntry, 0)
	err := p.Walk(ctx, func(timestamp int64, id uint64, _ sdk.AccAddress, req types.PendingWithdrawal) (stop bool, err error) {
		pendingWithdrawalQueue = append(pendingWithdrawalQueue, types.PendingWithdrawalQueueEntry{
			Time:       timestamp,
			Id:         id,
			Withdrawal: req,
		})
		return false, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk pending withdrawal queue: %w", err)
	}

	latestSequenceNumber, err := p.Sequence.Peek(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest sequence number for pending withdrawal queue: %w", err)
	}

	return &types.PendingWithdrawalQueue{
			LatestSequenceNumber: latestSequenceNumber,
			Entries:              pendingWithdrawalQueue,
		},
		nil
}
