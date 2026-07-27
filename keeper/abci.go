package keeper

import (
	"fmt"

	"github.com/provlabs/vault/interest"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// MaxSwapOutBatchSize is the maximum number of pending swap-out queue entries
	// visited per EndBlocker. Entries for paused vaults count against the budget
	// and are dequeued and refunded. This is a temporary value and we will need to
	// do more analysis on a proper batch size.
	// See https://github.com/ProvLabs/vault/issues/75.
	MaxSwapOutBatchSize = 100

	// MaxInterestTimeoutsPerBlock is the maximum number of PayoutTimeoutQueue
	// entries visited per BeginBlocker.
	MaxInterestTimeoutsPerBlock = 100

	// MaxFeeTimeoutsPerBlock is the maximum number of FeeTimeoutQueue entries
	// visited per BeginBlocker.
	MaxFeeTimeoutsPerBlock = 100

	// MaxPayoutVerificationsPerBlock is the maximum number of PayoutVerificationSet
	// entries visited per EndBlocker.
	MaxPayoutVerificationsPerBlock = 100

	// SwapOutImmediateRetries is the number of failures a pending swap out may accumulate
	// before its retries start being delayed by SwapOutRetryBackoffBase.
	SwapOutImmediateRetries = 1

	// SwapOutRetryBackoffBase is the first delay (in seconds) applied to a repeatedly
	// failing swap out. The delay doubles with each further failure.
	SwapOutRetryBackoffBase = 600

	// SwapOutRetryBackoffMax caps the retry delay (in seconds) so a permanently failing
	// swap out is still revisited, at a cost the batch budget can absorb.
	SwapOutRetryBackoffMax = 6 * interest.SecondsPerHour
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
