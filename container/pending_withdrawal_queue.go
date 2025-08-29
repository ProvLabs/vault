package container

import (
	"context"

	"github.com/provlabs/vault/types"

	"cosmossdk.io/collections"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type PendingWithdrawalQueue struct {
	collections.Map[collections.Triple[int64, sdk.AccAddress, uint64], types.PendingWithdrawal]
	Sequence collections.Sequence
}

func NewPendingWithdrawalQueue(builder *collections.SchemaBuilder, cdc codec.BinaryCodec) *PendingWithdrawalQueue {
	keyCodec := collections.TripleKeyCodec(
		collections.Int64Key,
		sdk.AccAddressKey,
		collections.Uint64Key,
	)
	valueCodec := codec.CollValue[types.PendingWithdrawal](cdc)
	return &PendingWithdrawalQueue{
		Map:      collections.NewMap(builder, types.VaultPendingWithdrawalQueuePrefix, types.VaultPendingWithdrawalQueueName, keyCodec, valueCodec),
		Sequence: collections.NewSequence(builder, types.VaultPendingWithdrawalQueueSeqPrefix, types.VaultPendingWithdrawalQueueSeqName),
	}
}

// Enqueue adds a pending withdrawal to the queue.
func (p *PendingWithdrawalQueue) Enqueue(ctx context.Context, timestamp int64, req types.PendingWithdrawal) (uint64, error) {
	vault, err := sdk.AccAddressFromBech32(req.VaultAddress)
	if err != nil {
		return 0, err
	}
	id, err := p.Sequence.Next(ctx)
	if err != nil {
		return 0, err
	}
	return id, p.Set(ctx, collections.Join3(timestamp, vault, id), req)
}

// Dequeue removes a pending withdrawal from the queue.
func (p *PendingWithdrawalQueue) Dequeue(ctx context.Context, timestamp int64, vault sdk.AccAddress, id uint64) error {
	return p.Remove(ctx, collections.Join3(timestamp, vault, id))
}

// WalkDue iterates over all entries in the PendingWithdrawalQueue with
// a timestamp <= now. For each due entry, the callback is invoked.
// Iteration stops when a key with time > now is encountered (since keys are
// ordered) or when the callback returns stop=true or an error.
func (p *PendingWithdrawalQueue) WalkDue(ctx context.Context, now int64, fn func(timestamp int64, vault sdk.AccAddress, id uint64, req types.PendingWithdrawal) (stop bool, err error)) error {
	return p.Map.Walk(ctx, nil, func(key collections.Triple[int64, sdk.AccAddress, uint64], value types.PendingWithdrawal) (stop bool, err error) {
		if key.K1() > now {
			return true, nil
		}
		return fn(key.K1(), key.K2(), key.K3(), value)
	})
}

func (p *PendingWithdrawalQueue) Walk(ctx context.Context, fn func(timestamp int64, vault sdk.AccAddress, id uint64, req types.PendingWithdrawal) (stop bool, err error)) error {
	return p.Map.Walk(ctx, nil, func(key collections.Triple[int64, sdk.AccAddress, uint64], value types.PendingWithdrawal) (stop bool, err error) {
		return fn(key.K1(), key.K2(), key.K3(), value)
	})
}
