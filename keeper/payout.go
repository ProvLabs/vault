package keeper

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/provlabs/vault/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	markertypes "github.com/provenance-io/provenance/x/marker/types"
)

// ProcessPendingSwapOuts processes the queue of pending swap-out requests. Called from the EndBlocker,
// it iterates through requests due for payout at the current block time. It uses a safe "collect-then-mutate"
// pattern to comply with the SDK iterator contract. It first collects all due requests up to the provided `batchSize`,
// then passes them to `processSwapOutJobs` for execution. Critical, unrecoverable errors during job processing
// will cause the associated vault to be automatically paused.
func (k *Keeper) ProcessPendingSwapOuts(ctx context.Context, batchSize int) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	now := sdkCtx.BlockTime().Unix()
	var jobsToProcess []types.PayoutJob

	processed := 0
	err := k.PendingSwapOutQueue.WalkDue(ctx, now, func(timestamp int64, id uint64, vaultAddr sdk.AccAddress, req types.PendingSwapOut) (stop bool, err error) {
		vault, ok := k.tryGetVault(sdkCtx, vaultAddr)
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
		sdkCtx.Logger().Error("error during pending withdrawal queue walk", "error", err)
		return err
	}

	k.processSwapOutJobs(ctx, jobsToProcess)

	return nil
}

// processSwapOutJobs iterates pending swap-out jobs collected for this block and executes them.
// Behavior by case:
//   - Missing vault: the job is dequeued and skipped.
//   - Paused vault: the job remains queued (skipped for now) so it can be retried when unpaused.
//   - Active vault: the job is dequeued and processed.
//   - On recoverable failure, a refund is attempted. If the refund itself returns a critical error,
//     the vault is auto-paused with a stable reason string.
//   - On critical failure during processing, the vault is auto-paused with a stable reason string.
func (k *Keeper) processSwapOutJobs(ctx context.Context, jobsToProcess []types.PayoutJob) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	for _, j := range jobsToProcess {
		vault, ok := k.tryGetVault(sdkCtx, j.VaultAddr)
		if !ok {
			if err := k.PendingSwapOutQueue.Dequeue(ctx, j.Timestamp, j.VaultAddr, j.ID); err != nil {
				sdkCtx.Logger().Error(
					fmt.Sprintf("CRITICAL: failed to dequeue withdrawal request %d for non-existent vault %s", j.ID, j.VaultAddr),
					"error", err,
				)
			} else {
				sdkCtx.Logger().Error(
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

		if err := k.PendingSwapOutQueue.Dequeue(ctx, j.Timestamp, j.VaultAddr, j.ID); err != nil {
			sdkCtx.Logger().Error(
				"failed to dequeue withdrawal request",
				"request_id", j.ID,
				"vault_address", j.VaultAddr.String(),
				"error", err,
			)
			continue
		}

		if err := k.processSingleWithdrawal(sdkCtx, j.ID, j.Req, *vault); err != nil {
			var cErr *types.CriticalError
			if errors.As(err, &cErr) {
				k.autoPauseVault(sdkCtx, vault, cErr.Reason)
				continue
			}

			reason := k.getRefundReason(err)
			sdkCtx.Logger().Error(
				"failed to process withdrawal, issuing refund",
				"withdrawal_id", j.ID,
				"reason", reason,
				"error", err,
			)

			if rerr := k.refundWithdrawal(sdkCtx, j.ID, j.Req, reason); rerr != nil {
				if errors.As(rerr, &cErr) {
					k.autoPauseVault(sdkCtx, vault, cErr.Reason)
				}
			}
		}
	}
}

// processSingleWithdrawal executes a pending swap-out. It first reconciles vault interest, then converts the user's
// shares to the redeemable asset amount. It then pays out those assets to the owner and burns their escrowed shares.
// It returns nil on success and emits an EventSwapOutCompleted. If a failure occurs before payout (e.g., insufficient
// liquidity), a normal error is returned and can be refunded. If a failure occurs after payout (e.g., transfer to
// principal or burn failure), the error is wrapped in a *types.CriticalError with a stable reason so the vault
// can be automatically paused.
func (k *Keeper) processSingleWithdrawal(ctx sdk.Context, id uint64, req types.PendingSwapOut, vault types.VaultAccount) error {
	vaultAddr := sdk.MustAccAddressFromBech32(req.VaultAddress)
	ownerAddr := sdk.MustAccAddressFromBech32(req.Owner)
	principalAddress := markertypes.MustGetMarkerAddress(req.Shares.Denom)

	if err := k.ReconcileVaultInterest(ctx, &vault); err != nil {
		return fmt.Errorf("failed to reconcile vault interest: %w", err)
	}

	assets, err := k.ConvertSharesToRedeemCoin(ctx, vault, req.Shares.Amount, req.RedeemDenom)
	if err != nil {
		return fmt.Errorf("failed to convert shares to redeem coin for single withdrawal: %w", err)
	}

	if err := k.BankKeeper.SendCoins(markertypes.WithTransferAgents(ctx, vaultAddr), principalAddress, ownerAddr, sdk.NewCoins(assets)); err != nil {
		return err
	}

	if err := k.BankKeeper.SendCoins(ctx, vaultAddr, principalAddress, sdk.NewCoins(req.Shares)); err != nil {
		errMsg := fmt.Sprintf(
			"failed to transfer %s shares from %s to principal %s for burning",
			req.Shares, vaultAddr, principalAddress,
		)
		ctx.Logger().Error("CRITICAL: "+errMsg, "error", err)
		return types.CriticalErr(errMsg, fmt.Errorf("%s: %w", errMsg, err))
	}

	if err := k.MarkerKeeper.BurnCoin(ctx, vaultAddr, req.Shares); err != nil {
		errMsg := fmt.Sprintf(
			"failed to burn %s shares from account %s after successful payout",
			req.Shares, vaultAddr,
		)
		ctx.Logger().Error("CRITICAL: "+errMsg, "error", err)
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
		ctx.Logger().Error("CRITICAL: "+errMsg, "error", err)
		return types.CriticalErr(errMsg, fmt.Errorf("%s: %w", errMsg, err))
	}
	k.AuthKeeper.SetAccount(ctx, &vault)

	k.emitEvent(ctx, types.NewEventSwapOutCompleted(req.VaultAddress, req.Owner, assets, id))
	return nil
}

// refundWithdrawal handles the failure case for a pending swap out. It returns the user's
// escrowed shares from the vault's own account back to the owner and emits an EventSwapOutRefunded.
// It returns nil on success. If the refund transfer itself fails, the error is wrapped in a
// *types.CriticalError with a stable reason so the vault can be automatically paused.
func (k *Keeper) refundWithdrawal(ctx sdk.Context, id uint64, req types.PendingSwapOut, reason string) error {
	vaultAddr := sdk.MustAccAddressFromBech32(req.VaultAddress)
	ownerAddr := sdk.MustAccAddressFromBech32(req.Owner)

	if err := k.BankKeeper.SendCoins(ctx, vaultAddr, ownerAddr, sdk.NewCoins(req.Shares)); err != nil {
		errMsg := fmt.Sprintf(
			"failed to refund %s shares from vault %s to owner %s",
			req.Shares, vaultAddr, ownerAddr,
		)
		ctx.Logger().Error("CRITICAL: "+errMsg, "error", err)
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
	case strings.Contains(errMsg, "nav not found"):
		return types.RefundReasonNavNotFound
	case strings.Contains(errMsg, "failed to reconcile"):
		return types.RefundReasonReconcileFailure
	}

	return types.RefundReasonUnknown
}
