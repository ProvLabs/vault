package queue

import (
	"fmt"

	"github.com/provlabs/vault/types"

	"cosmossdk.io/collections"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type FeeTimeoutQueue struct {
	keyset collections.KeySet[collections.Pair[uint64, sdk.AccAddress]]
}

func NewFeeTimeoutQueue(builder *collections.SchemaBuilder) *FeeTimeoutQueue {
	endKeyCodec := collections.PairKeyCodec(
		collections.Uint64Key,
		sdk.AccAddressKey,
	)
	return &FeeTimeoutQueue{
		keyset: collections.NewKeySet(builder, types.VaultFeeTimeoutQueuePrefix, types.VaultFeeTimeoutQueueName, endKeyCodec),
	}
}

// Enqueue schedules a vault for fee collection processing by inserting an
// entry into the FeeTimeoutQueue keyed by (feeTimeout, vault address).
func (p *FeeTimeoutQueue) Enqueue(ctx sdk.Context, feeTimeout int64, vaultAddr sdk.AccAddress) error {
	if feeTimeout < 0 {
		return fmt.Errorf("feeTimeout cannot be negative")
	}
	return p.keyset.Set(ctx, collections.Join(uint64(feeTimeout), vaultAddr))
}

// Dequeue removes a specific timeout entry (feeTimeout, vault)
// from the FeeTimeoutQueue.
func (p *FeeTimeoutQueue) Dequeue(ctx sdk.Context, feeTimeout int64, vaultAddr sdk.AccAddress) error {
	if feeTimeout < 0 {
		return fmt.Errorf("feeTimeout cannot be negative")
	}
	return p.keyset.Remove(ctx, collections.Join(uint64(feeTimeout), vaultAddr))
}


// WalkDue iterates over all entries in the FeeTimeoutQueue with
// a timeout timestamp <= nowSec. For each due entry, the callback is invoked.
func (p *FeeTimeoutQueue) WalkDue(ctx sdk.Context, nowSec int64, fn func(feeTimeout uint64, vaultAddr sdk.AccAddress) (stop bool, err error)) error {
	return p.keyset.Walk(ctx, nil, func(key collections.Pair[uint64, sdk.AccAddress]) (stop bool, err error) {
		if key.K1() > uint64(nowSec) {
			return true, nil
		}
		return fn(key.K1(), key.K2())
	})
}

// Walk iterates over all entries in the FeeTimeoutQueue.
func (p *FeeTimeoutQueue) Walk(ctx sdk.Context, fn func(feeTimeout uint64, vaultAddr sdk.AccAddress) (stop bool, err error)) error {
	return p.keyset.Walk(ctx, nil, func(key collections.Pair[uint64, sdk.AccAddress]) (stop bool, err error) {
		return fn(key.K1(), key.K2())
	})
}

// RemoveAllForVault deletes all fee timeout entries in the
// FeeTimeoutQueue for the given vault address.
func (p *FeeTimeoutQueue) RemoveAllForVault(ctx sdk.Context, vaultAddr sdk.AccAddress) error {
	var keys []collections.Pair[uint64, sdk.AccAddress]

	err := p.keyset.Walk(ctx, nil, func(kv collections.Pair[uint64, sdk.AccAddress]) (bool, error) {
		if kv.K2().Equals(vaultAddr) {
			keys = append(keys, kv)
		}
		return false, nil
	})
	if err != nil {
		return err
	}

	for _, key := range keys {
		if err := p.keyset.Remove(ctx, key); err != nil {
			return err
		}
	}
	return nil
}
