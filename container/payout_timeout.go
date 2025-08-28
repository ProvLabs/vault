package container

import (
	"context"

	"github.com/provlabs/vault/types"

	"cosmossdk.io/collections"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type PayoutTimeout struct {
	queue collections.KeySet[collections.Pair[uint64, sdk.AccAddress]]
}

func NewPayoutTimeout(builder *collections.SchemaBuilder) *PayoutTimeout {
	endKeyCodec := collections.PairKeyCodec(
		collections.Uint64Key,
		sdk.AccAddressKey,
	)
	return &PayoutTimeout{
		queue: collections.NewKeySet(builder, types.VaultPayoutTimeoutQueuePrefix, types.VaultPayoutTimeoutQueueName, endKeyCodec),
	}
}

// Enqueue schedules a vault for timeout processing by inserting an
// entry into the PayoutTimeoutQueue keyed by (periodTimeout, vault address).
func (p *PayoutTimeout) Enqueue(ctx context.Context, periodTimeout int64, vaultAddr sdk.AccAddress) error {
	return p.queue.Set(ctx, collections.Join(uint64(periodTimeout), vaultAddr))
}

// Dequeue removes a specific timeout entry (periodTimeout, vault)
// from the PayoutTimeoutQueue.
func (p *PayoutTimeout) Dequeue(ctx context.Context, periodTimeout int64, vaultAddr sdk.AccAddress) error {
	return p.queue.Remove(ctx, collections.Join(uint64(periodTimeout), vaultAddr))
}

// WalkDue iterates over all entries in the PayoutTimeoutQueue with
// a timeout timestamp <= nowSec. For each due entry, the callback is invoked.
// Iteration stops when a key with time > nowSec is encountered (since keys are
// ordered) or when the callback returns stop=true or an error.
func (p *PayoutTimeout) WalkDue(ctx context.Context, nowSec int64, fn func(periodTimeout uint64, vaultAddr sdk.AccAddress) (stop bool, err error)) error {
	return p.queue.Walk(ctx, nil, func(key collections.Pair[uint64, sdk.AccAddress]) (stop bool, err error) {
		if key.K1() > uint64(nowSec) {
			return true, nil
		}
		return fn(key.K1(), key.K2())
	})
}

// Walk iterates over all entries in the PayoutTimeoutQueue.
// Iteration stops when the callback returns stop=true or an error.
func (p *PayoutTimeout) Walk(ctx context.Context, fn func(periodTimeout uint64, vaultAddr sdk.AccAddress) (stop bool, err error)) error {
	return p.queue.Walk(ctx, nil, func(key collections.Pair[uint64, sdk.AccAddress]) (stop bool, err error) {
		return fn(key.K1(), key.K2())
	})
}

// RemoveAllForVault deletes all timeout entries in the
// PayoutTimeoutQueue for the given vault address.
func (p *PayoutTimeout) RemoveAllForVault(ctx context.Context, vaultAddr sdk.AccAddress) error {
	var keys []collections.Pair[uint64, sdk.AccAddress]

	err := p.queue.Walk(ctx, nil, func(kv collections.Pair[uint64, sdk.AccAddress]) (bool, error) {
		if kv.K2().Equals(vaultAddr) {
			keys = append(keys, kv)
		}
		return false, nil
	})
	if err != nil {
		return err
	}

	for _, key := range keys {
		if err := p.queue.Remove(ctx, key); err != nil {
			return err
		}
	}
	return nil
}
