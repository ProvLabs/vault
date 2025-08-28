package container

import (
	"context"

	"cosmossdk.io/collections"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// PriorityQueueKey is the key for the PriorityQueue.
// It's a pair of (priority, item_address). In the vault module, priority is a timestamp.
var PriorityQueueKey = collections.PairKeyCodec(
	collections.Int64Key,
	sdk.AccAddressKey,
)

// PriorityQueue is a time-ordered queue of items.
// It is a KeySet ordered by priority (e.g., time), then by address.
type PriorityQueue struct {
	collections.KeySet[collections.Pair[int64, sdk.AccAddress]]
}

// NewPriorityQueue creates a new PriorityQueue.
func NewPriorityQueue(schema *collections.SchemaBuilder, prefix, name string) PriorityQueue {
	return PriorityQueue{
		KeySet: collections.NewKeySet(schema, collections.NewPrefix(prefix), name, PriorityQueueKey),
	}
}

// Enqueue adds an item to the queue with a specific priority (e.g., a timestamp).
func (q PriorityQueue) Enqueue(ctx context.Context, itemAddr sdk.AccAddress, priority int64) error {
	return q.Set(ctx, collections.Join(priority, itemAddr))
}

// Dequeue removes an item from the queue.
func (q PriorityQueue) Dequeue(ctx context.Context, itemAddr sdk.AccAddress, priority int64) error {
	return q.Remove(ctx, collections.Join(priority, itemAddr))
}

// WalkDue iterates over all entries in the queue with a priority <= maxPriority.
// For each due entry, the callback is invoked with the key components.
// Iteration stops when a key with priority > maxPriority is encountered or when the callback
// returns stop=true or an error.
func (q PriorityQueue) WalkDue(ctx context.Context, maxPriority int64, fn func(priority int64, itemAddr sdk.AccAddress) (stop bool, err error)) error {
	it, err := q.Iterate(ctx, nil)
	if err != nil {
		return err
	}
	defer it.Close()

	for ; it.Valid(); it.Next() {
		key, err := it.Key()
		if err != nil {
			return err
		}
		if key.K1() > maxPriority {
			break
		}
		stop, err := fn(key.K1(), key.K2())
		if err != nil || stop {
			return err
		}
	}
	return nil
}

// RemoveAllForItem deletes all entries in the queue for the given item address.
// Note: This is an O(N) operation where N is the total number of items in the queue.
func (q PriorityQueue) RemoveAllForItem(ctx context.Context, itemAddr sdk.AccAddress) error {
	var keys []collections.Pair[int64, sdk.AccAddress]

	it, err := q.Iterate(ctx, nil)
	if err != nil {
		return err
	}
	defer it.Close()

	for ; it.Valid(); it.Next() {
		key, err := it.Key()
		if err != nil {
			return err
		}
		if key.K2().Equals(itemAddr) {
			keys = append(keys, key)
		}
	}

	for _, key := range keys {
		if err := q.Remove(ctx, key); err != nil {
			return err
		}
	}
	return nil
}
