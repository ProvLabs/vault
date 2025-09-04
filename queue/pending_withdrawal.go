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

type PendingWithdrawalIndexes struct {
	ByVault *indexes.Multi[sdk.AccAddress, collections.Triple[int64, sdk.AccAddress, uint64], types.PendingWithdrawal]
}

func (i PendingWithdrawalIndexes) IndexesList() []collections.Index[collections.Triple[int64, sdk.AccAddress, uint64], types.PendingWithdrawal] {
	return []collections.Index[collections.Triple[int64, sdk.AccAddress, uint64], types.PendingWithdrawal]{i.ByVault}
}

func NewPendingWithdrawalIndexes(sb *collections.SchemaBuilder) PendingWithdrawalIndexes {
	return PendingWithdrawalIndexes{
		ByVault: indexes.NewMulti(
			sb,
			types.VaultPendingWithdrawalByVaultIndexPrefix,
			types.VaultPendingWithdrawalByVaultIndexName,
			sdk.AccAddressKey,
			collections.TripleKeyCodec(collections.Int64Key, sdk.AccAddressKey, collections.Uint64Key),
			func(pk collections.Triple[int64, sdk.AccAddress, uint64], _ types.PendingWithdrawal) (sdk.AccAddress, error) {
				return pk.K2(), nil
			},
		),
	}
}

type PendingWithdrawalQueue struct {
	IndexedMap *collections.IndexedMap[collections.Triple[int64, sdk.AccAddress, uint64], types.PendingWithdrawal, PendingWithdrawalIndexes]
	Sequence   collections.Sequence
}

func NewPendingWithdrawalQueue(builder *collections.SchemaBuilder, cdc codec.BinaryCodec) *PendingWithdrawalQueue {
	keyCodec := collections.TripleKeyCodec(
		collections.Int64Key,
		sdk.AccAddressKey,
		collections.Uint64Key,
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
func (p *PendingWithdrawalQueue) Enqueue(ctx context.Context, timestamp int64, req types.PendingWithdrawal) (uint64, error) {
	if timestamp < 0 {
		return 0, fmt.Errorf("timestamp cannot be negative")
	}
	vault, err := sdk.AccAddressFromBech32(req.VaultAddress)
	if err != nil {
		return 0, err
	}
	id, err := p.Sequence.Next(ctx)
	if err != nil {
		return 0, err
	}
	return id, p.IndexedMap.Set(ctx, collections.Join3(timestamp, vault, id), req)
}

// Dequeue removes a pending withdrawal from the queue.
func (p *PendingWithdrawalQueue) Dequeue(ctx context.Context, timestamp int64, vault sdk.AccAddress, id uint64) error {
	if timestamp < 0 {
		return fmt.Errorf("timestamp cannot be negative")
	}
	if ok, _ := p.IndexedMap.Has(ctx, collections.Join3(timestamp, vault, id)); !ok {
		return nil
	}
	return p.IndexedMap.Remove(ctx, collections.Join3(timestamp, vault, id))
}

// WalkDue iterates over all entries in the PendingWithdrawalQueue with
// a timestamp <= now. For each due entry, the callback is invoked.
// Iteration stops when a key with time > now is encountered (since keys are
// ordered) or when the callback returns stop=true or an error.
func (p *PendingWithdrawalQueue) WalkDue(ctx context.Context, now int64, fn func(timestamp int64, vault sdk.AccAddress, id uint64, req types.PendingWithdrawal) (stop bool, err error)) error {
	return p.IndexedMap.Walk(ctx, nil, func(key collections.Triple[int64, sdk.AccAddress, uint64], value types.PendingWithdrawal) (stop bool, err error) {
		if key.K1() > now {
			return true, nil
		}
		return fn(key.K1(), key.K2(), key.K3(), value)
	})
}

// Walk iterates over all entries in the PendingWithdrawalQueue.
// Iteration stops when the callback returns stop=true or an error.
func (p *PendingWithdrawalQueue) Walk(ctx context.Context, fn func(timestamp int64, vault sdk.AccAddress, id uint64, req types.PendingWithdrawal) (stop bool, err error)) error {
	return p.IndexedMap.Walk(ctx, nil, func(key collections.Triple[int64, sdk.AccAddress, uint64], value types.PendingWithdrawal) (stop bool, err error) {
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
		if stop, err := fn(pk.K1(), pk.K3(), req); stop || err != nil {
			return err
		}
	}
	return nil
}
