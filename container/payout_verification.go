package container

import (
	"context"

	"github.com/provlabs/vault/types"

	"cosmossdk.io/collections"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// PayoutVerificationQueue is a queue for vaults that need payout verification.
type PayoutVerificationQueue struct {
	collections.KeySet[sdk.AccAddress]
}

// NewPayoutVerificationQueue creates a new PayoutVerificationQueue.
func NewPayoutVerificationQueue(schemaBuilder *collections.SchemaBuilder) *PayoutVerificationQueue {
	return &PayoutVerificationQueue{
		KeySet: collections.NewKeySet(schemaBuilder, types.VaultPayoutVerificationQueuePrefix, types.VaultPayoutVerificationQueueName, sdk.AccAddressKey),
	}
}

// Enqueue schedules a vault for payout verification by inserting its address into the queue.
func (q *PayoutVerificationQueue) Enqueue(ctx context.Context, vaultAddr sdk.AccAddress) error {
	return q.Set(ctx, vaultAddr)
}

// Dequeue removes a vault from the queue.
func (q *PayoutVerificationQueue) Dequeue(ctx context.Context, vaultAddr sdk.AccAddress) error {
	return q.Remove(ctx, vaultAddr)
}

// Walk iterates over all entries in the queue.
func (q *PayoutVerificationQueue) Walk(ctx context.Context, fn func(vaultAddr sdk.AccAddress) (stop bool, err error)) error {
	return q.KeySet.Walk(ctx, nil, fn)
}
