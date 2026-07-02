package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// MaxSwapOutBatchSize is the maximum number of pending swap-out queue entries
	// to visit in a single EndBlocker invocation. Every visited entry counts against
	// the budget, including entries for paused vaults (which are dequeued and refunded
	// rather than processed), so block time stays bounded regardless of queue depth or
	// composition. This is a temporary value and we will need to do more analysis on a
	// proper batch size. See https://github.com/ProvLabs/vault/issues/75.
	MaxSwapOutBatchSize = 100

	// MaxInterestTimeoutsPerBlock is the maximum number of PayoutTimeoutQueue entries
	// to visit in a single BeginBlocker invocation. Entries beyond the budget remain
	// due and are picked up in subsequent blocks, keeping interest reconciliation cost
	// per block bounded regardless of how many vaults time out at once.
	MaxInterestTimeoutsPerBlock = 100

	// MaxFeeTimeoutsPerBlock is the maximum number of FeeTimeoutQueue entries to visit
	// in a single BeginBlocker invocation. Entries beyond the budget remain due and are
	// picked up in subsequent blocks, keeping AUM fee collection cost per block bounded.
	MaxFeeTimeoutsPerBlock = 100

	// MaxPayoutVerificationsPerBlock is the maximum number of PayoutVerificationSet
	// entries to visit in a single EndBlocker invocation. Entries beyond the budget
	// stay in the set and are verified in subsequent blocks.
	MaxPayoutVerificationsPerBlock = 100
)

// BeginBlocker is a hook that is called at the beginning of every block.
func (k *Keeper) BeginBlocker(ctx sdk.Context) error {
	if err := k.handleVaultInterestTimeouts(ctx, MaxInterestTimeoutsPerBlock); err != nil {
		return fmt.Errorf("failed to handle vault interest timeouts: %w", err)
	}
	if err := k.handleVaultFeeTimeouts(ctx, MaxFeeTimeoutsPerBlock); err != nil {
		return fmt.Errorf("failed to handle vault fee timeouts: %w", err)
	}
	return nil
}

// EndBlocker is a hook that is called at the end of every block.
func (k *Keeper) EndBlocker(ctx sdk.Context) error {
	if err := k.processPendingSwapOuts(ctx, MaxSwapOutBatchSize); err != nil {
		return fmt.Errorf("failed to process pending swap outs: %w", err)
	}

	if err := k.handleReconciledVaults(ctx, MaxPayoutVerificationsPerBlock); err != nil {
		return fmt.Errorf("failed to handle reconciled vaults: %w", err)
	}

	return nil
}
