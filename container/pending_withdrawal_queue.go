package container

import (
	"context"

	"github.com/provlabs/vault/types"

	"cosmossdk.io/collections"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type PendingWithdrawalQueue struct {
	collections.Map[collections.Pair[int64, sdk.AccAddress], types.PendingWithdrawal]
}

func NewPendingWithdrawalQueue(builder *collections.SchemaBuilder, cdc codec.BinaryCodec) *PendingWithdrawalQueue {
	keyCodec := collections.PairKeyCodec(
		collections.Int64Key,
		sdk.AccAddressKey,
	)
	valueCodec := codec.CollValue[types.PendingWithdrawal](cdc)
	return &PendingWithdrawalQueue{
		Map: collections.NewMap(builder, types.VaultPendingWithdrawalQueuePrefix, types.VaultPendingWithdrawalQueueName, keyCodec, valueCodec),
	}
}

// Enqueue adds a pending withdrawal to the queue.
func (p *PendingWithdrawalQueue) Enqueue(ctx context.Context, timestamp int64, req types.PendingWithdrawal) error {
	vault, err := sdk.AccAddressFromBech32(req.VaultAddress)
	if err != nil {
		return err
	}
	return p.Set(ctx, collections.Join(timestamp, vault), req)
}

// Dequeue removes a pending withdrawal from the queue.
func (p *PendingWithdrawalQueue) Dequeue(ctx context.Context, timestamp int64, vault sdk.AccAddress) error {
	return p.Remove(ctx, collections.Join(timestamp, vault))
}

// WalkDue iterates over all entries in the PendingWithdrawalQueue with
// a timestamp <= now. For each due entry, the callback is invoked.
// Iteration stops when a key with time > now is encountered (since keys are
// ordered) or when the callback returns stop=true or an error.
func (p *PendingWithdrawalQueue) WalkDue(ctx context.Context, now int64, fn func(timestamp int64, vault sdk.AccAddress, req types.PendingWithdrawal) (stop bool, err error)) error {
	return p.Walk(ctx, nil, func(key collections.Pair[int64, sdk.AccAddress], value types.PendingWithdrawal) (stop bool, err error) {
		if key.K1() > now {
			return true, nil
		}
		return fn(key.K1(), key.K2(), value)
	})
}
