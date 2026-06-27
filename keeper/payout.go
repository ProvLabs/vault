package keeper

import (
	"errors"
	"fmt"
	"strings"

	"github.com/provlabs/vault/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	markertypes "github.com/provenance-io/provenance/x/marker/types"
)

// processPendingSwapOuts processes the queue of pending swap-out requests. Called from the EndBlocker,
// it iterates through requests due for payout at the current block time. It uses a safe "collect-then-mutate"
// pattern to comply with the SDK iterator contract. It first collects all due requests up to the provided `batchSize`,
// then passes them to `processSwapOutJobs` for execution. Critical, unrecoverable errors during job processing
// will cause the associated vault to be automatically paused.
func (k *Keeper) processPendingSwapOuts(ctx sdk.Context, batchSize int) error {
	now := ctx.BlockTime().Unix()
	var jobsToProcess []types.PayoutJob

	processed := 0
	err := k.PendingSwapOutQueue.WalkDue(ctx, now, func(timestamp int64, id uint64, vaultAddr sdk.AccAddress, req types.PendingSwapOut) (stop bool, err error) {
		vault, ok := k.tryGetVault(ctx, vaultAddr)
		if ok && vault.Paused {
			return false, nil
		}
		if processed == batchSize {
			return true, nil
		}
		processed++
		jobsToProcess = append(jobsToProcess, types.NewPayoutJob(timestamp, id, vaultAddr, req))
		return false, nil
	})
	if err != nil {
		k.getLogger(ctx).Error("error during pending withdrawal queue walk", "error", err)
		return fmt.Errorf("failed to walk pending swap out queue: %w", err)
	}

	k.processSwapOutJobs(ctx, jobsToProcess)

	return nil
}

// processSwapOutJobs iterates pending swap-out jobs collected for this block and executes them.
//
// A pending request is removed from the queue only when a durable outcome is committed in the same
// atomic unit as the deletion: a completed payout or a successful refund. The dequeue therefore shares
// the cache context with its outcome, so on a critical failure both the work and the deletion roll back
// together and the request, along with the shares escrowed at request time, is preserved for later
// settlement. This keeps the payout/burn atomicity intact while ensuring escrow can never be stranded
// by a partially committed outcome.
//
// The processing strategy involves:
//  1. Verifying the vault exists and is not paused.
//  2. Executing the withdrawal logic (reconciliation, payout, and burning) within a cache context, then
//     dequeuing in that same cache context and committing only on success.
//  3. Handling recoverable failures by dequeuing and refunding atomically in a fresh cache context.
//  4. Handling critical or unrecoverable failures by auto-pausing the vault and discarding the cache so
//     the request is preserved. Because paused vaults are skipped, the preserved request waits for an
//     unpause rather than re-executing, so it is never double-processed.
func (k *Keeper) processSwapOutJobs(ctx sdk.Context, jobsToProcess []types.PayoutJob) {
	for _, j := range jobsToProcess {
		vault, ok := k.tryGetVault(ctx, j.VaultAddr)
		if !ok {
			if err := k.PendingSwapOutQueue.Dequeue(ctx, j.Timestamp, j.VaultAddr, j.ID); err != nil {
				k.getLogger(ctx).Error(
					fmt.Sprintf("CRITICAL: failed to dequeue withdrawal request %d for non-existent vault %s", j.ID, j.VaultAddr),
					"error", err,
				)
			} else {
				k.getLogger(ctx).Error(
					"dequeued and skipped pending withdrawal for non-existent vault",
					"request_id", j.ID,
					"vault_address", j.VaultAddr.String(),
				)
			}
			continue
		}

		if vault.Paused {
			continue
		}

		var cErr *types.CriticalError
		cacheCtx, write := ctx.CacheContext()
		err := k.processSingleWithdrawal(cacheCtx, j.ID, j.Req, *vault)
		if err == nil {
			if derr := k.PendingSwapOutQueue.Dequeue(cacheCtx, j.Timestamp, j.VaultAddr, j.ID); derr != nil {
				k.getLogger(ctx).Error(
					"failed to dequeue withdrawal request after successful payout",
					"request_id", j.ID,
					"vault_address", j.VaultAddr.String(),
					"error", derr,
				)
				continue
			}
			write()
			continue
		}

		if errors.As(err, &cErr) {
			k.autoPauseVault(ctx, vault, cErr.Reason)
			continue
		}

		reason := k.getRefundReason(err)
		k.getLogger(ctx).Error(
			"failed to process withdrawal, issuing refund",
			"withdrawal_id", j.ID,
			"reason", reason,
			"error", err,
		)

		refundCtx, refundWrite := ctx.CacheContext()
		if derr := k.PendingSwapOutQueue.Dequeue(refundCtx, j.Timestamp, j.VaultAddr, j.ID); derr != nil {
			k.getLogger(ctx).Error(
				"failed to dequeue withdrawal request before refund",
				"request_id", j.ID,
				"vault_address", j.VaultAddr.String(),
				"error", derr,
			)
			continue
		}
		if rerr := k.refundWithdrawal(refundCtx, j.ID, j.Req, reason); rerr != nil {
			if errors.As(rerr, &cErr) {
				k.autoPauseVault(ctx, vault, cErr.Reason)
			}
			continue
		}
		refundWrite()
	}
}

// processSingleWithdrawal executes a pending swap-out. It first reconciles the vault (interest and AUM fees), then converts the user's
// shares to the redeemable asset amount. It then pays out those assets to the owner and burns their escrowed shares.
// It returns nil on success and emits an EventSwapOutCompleted. If a failure occurs before payout (e.g., insufficient
// liquidity), a normal error is returned and can be refunded. If a failure occurs after payout (e.g., transfer to
// principal or burn failure), the error is wrapped in a *types.CriticalError with a stable reason so the vault
// can be automatically paused.
func (k *Keeper) processSingleWithdrawal(ctx sdk.Context, id uint64, req types.PendingSwapOut, vault types.VaultAccount) error {
	vaultAddr, err := sdk.AccAddressFromBech32(req.VaultAddress)
	if err != nil {
		return fmt.Errorf("invalid vault address %s: %w", req.VaultAddress, err)
	}
	ownerAddr, err := sdk.AccAddressFromBech32(req.Owner)
	if err != nil {
		return fmt.Errorf("invalid owner address %s: %w", req.Owner, err)
	}
	principalAddress, err := markertypes.MarkerAddress(req.Shares.Denom)
	if err != nil {
		return fmt.Errorf("invalid principal address for denom %s: %w", req.Shares.Denom, err)
	}

	if err = k.reconcileVault(ctx, &vault); err != nil {
		return fmt.Errorf("failed to reconcile vault: %w", err)
	}

	assets, err := k.ConvertSharesToRedeemCoin(ctx, vault, req.Shares.Amount, req.RedeemDenom)
	if err != nil {
		return fmt.Errorf("failed to convert shares to redeem coin for single withdrawal: %w", err)
	}

	if err = k.BankKeeper.SendCoins(markertypes.WithTransferAgents(ctx, vaultAddr), principalAddress, ownerAddr, sdk.NewCoins(assets)); err != nil {
		return fmt.Errorf("failed to payout assets to owner: %w", err)
	}

	if err = k.BankKeeper.SendCoins(ctx, vaultAddr, principalAddress, sdk.NewCoins(req.Shares)); err != nil {
		errMsg := fmt.Sprintf(
			"failed to transfer %s shares from %s to principal %s for burning",
			req.Shares, vaultAddr, principalAddress,
		)
		k.getLogger(ctx).Error("CRITICAL: "+errMsg, "error", err)
		return types.CriticalErr(errMsg, fmt.Errorf("%s: %w", errMsg, err))
	}

	if err = k.MarkerKeeper.BurnCoin(ctx, vaultAddr, req.Shares); err != nil {
		errMsg := fmt.Sprintf(
			"failed to burn %s shares from account %s after successful payout",
			req.Shares, vaultAddr,
		)
		k.getLogger(ctx).Error("CRITICAL: "+errMsg, "error", err)
		return types.CriticalErr(errMsg, fmt.Errorf("%s: %w", errMsg, err))
	}

	// This should not fail under correct accounting. Liquidity issues would cause BankKeeper.SendCoins to fail earlier.
	// If SafeSub fails here, req.Shares > TotalShares, indicating accounting drift; we pause the vault.
	vault.TotalShares, err = vault.TotalShares.SafeSub(req.Shares)
	if err != nil {
		errMsg := fmt.Sprintf(
			"failed to deduct %s shares from vault %s total shares after successful payout",
			req.Shares, vaultAddr,
		)
		k.getLogger(ctx).Error("CRITICAL: "+errMsg, "error", err)
		return types.CriticalErr(errMsg, fmt.Errorf("%s: %w", errMsg, err))
	}

	if err := k.SetVaultAccount(ctx, &vault); err != nil {
		errMsg := fmt.Sprintf(
			"failed to update vault account %s after successful payout",
			vaultAddr,
		)
		k.getLogger(ctx).Error("CRITICAL: "+errMsg, "error", err)
		return types.CriticalErr(errMsg, fmt.Errorf("%s: %w", errMsg, err))
	}

	k.emitEvent(ctx, types.NewEventSwapOutCompleted(req.VaultAddress, req.Owner, assets, id))
	return nil
}

// refundWithdrawal handles the failure case for a pending swap out. It returns the user's
// escrowed shares from the vault's own account back to the owner and emits an EventSwapOutRefunded.
// It returns nil on success. If the refund transfer itself fails, the error is wrapped in a
// *types.CriticalError with a stable reason so the vault can be automatically paused.
func (k *Keeper) refundWithdrawal(ctx sdk.Context, id uint64, req types.PendingSwapOut, reason string) error {
	vaultAddr, err := sdk.AccAddressFromBech32(req.VaultAddress)
	if err != nil {
		return fmt.Errorf("invalid vault address %s: %w", req.VaultAddress, err)
	}
	ownerAddr, err := sdk.AccAddressFromBech32(req.Owner)
	if err != nil {
		return fmt.Errorf("invalid owner address %s: %w", req.Owner, err)
	}

	if err := k.BankKeeper.SendCoins(ctx, vaultAddr, ownerAddr, sdk.NewCoins(req.Shares)); err != nil {
		errMsg := fmt.Sprintf(
			"failed to refund %s shares from vault %s to owner %s",
			req.Shares, vaultAddr, ownerAddr,
		)
		k.getLogger(ctx).Error("CRITICAL: "+errMsg, "error", err)
		return types.CriticalErr(errMsg, fmt.Errorf("%s: %w", errMsg, err))
	}

	k.emitEvent(ctx, types.NewEventSwapOutRefunded(req.VaultAddress, req.Owner, req.Shares, id, reason))
	return nil
}

// getRefundReason translates a processing error into a standardized reason string for events.
func (k Keeper) getRefundReason(err error) string {
	if errors.Is(err, sdkerrors.ErrInsufficientFunds) {
		return types.RefundReasonInsufficientFunds
	}
	if errors.Is(err, ErrInternalNAVNotFound) {
		return types.RefundReasonNavNotFound
	}

	errMsg := err.Error()

	switch {
	case strings.Contains(errMsg, "marker status is not active"):
		return types.RefundReasonMarkerNotActive
	case strings.Contains(errMsg, "does not contain") && strings.Contains(errMsg, "required attribute"):
		return types.RefundReasonRecipientMissingAttributes
	case strings.Contains(errMsg, "is on deny list"),
		strings.Contains(errMsg, "does not have transfer permissions"),
		strings.Contains(errMsg, "does not have access"):
		return types.RefundReasonPermissionDenied
	case strings.Contains(errMsg, "cannot be sent to the fee collector"):
		return types.RefundReasonRecipientInvalid
	case strings.Contains(errMsg, "failed to reconcile"):
		return types.RefundReasonReconcileFailure
	}

	return types.RefundReasonUnknown
}
