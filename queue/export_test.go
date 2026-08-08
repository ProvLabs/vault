package queue

import (
	"context"
	"testing"

	"cosmossdk.io/collections"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// TestAccessor_requireQueued exposes this queue's requireQueued guard for unit tests
func (p *PendingSwapOutQueue) TestAccessor_requireQueued(t *testing.T, ctx context.Context, timestamp int64, vault sdk.AccAddress, id uint64) error {
	t.Helper()
	return p.requireQueued(ctx, collections.Join3(timestamp, id, vault))
}
