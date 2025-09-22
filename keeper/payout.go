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

func (k *Keeper) ProcessPendingSwapOuts(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	now := sdkCtx.BlockTime().Unix()

	var jobsToProcess []types.PayoutJob

	err := k.PendingSwapOutQueue.WalkDue(ctx, now, func(timestamp int64, id uint64, vaultAddr sdk.AccAddress, req types.PendingSwapOut) (stop bool, err error) {
		vault, ok := k.tryGetVault(sdkCtx, vaultAddr)
		if ok && vault.Paused {
			return false, nil
		}
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

func (k *Keeper) processSwapOutJobs(ctx context.Context, jobsToProcess []types.PayoutJob) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	for _, j := range jobsToProcess {
		vault, ok := k.tryGetVault(sdkCtx, j.VaultAddr)
		if !ok {
			if err := k.PendingSwapOutQueue.Dequeue(ctx, j.Timestamp, j.VaultAddr, j.ID); err != nil {
				sdkCtx.Logger().Error(
					fmt.Sprintf("failed to dequeue withdrawal request %d for non-existent vault %s", j.ID, j.VaultAddr),
					"error", err,
				)
			} else {
				sdkCtx.Logger().Error("dequeued and skipped pending withdrawal for non-existent vault", "request_id", j.ID, "vault_address", j.VaultAddr.String())
			}
			continue
		}

		if vault.Paused {
			continue
		}

		if err := k.PendingSwapOutQueue.Dequeue(ctx, j.Timestamp, j.VaultAddr, j.ID); err != nil {
			k.autoPauseVault(sdkCtx, vault, fmt.Errorf("failed to dequeue withdrawal request %d for vault %s: %w", j.ID, j.VaultAddr, err))
			continue
		}

		err, isCritical := k.processSingleWithdrawal(sdkCtx, j.ID, j.Req, *vault)
		if err != nil && !isCritical {
			reason := k.getRefundReason(err)
			sdkCtx.Logger().Error("Failed to process withdrawal, issuing refund",
				"withdrawal_id", j.ID,
				"reason", reason,
				"error", err,
			)
			err = k.refundWithdrawal(sdkCtx, j.ID, j.Req, reason)
			if err != nil {
				k.autoPauseVault(sdkCtx, vault, err)
			}
		}
		if err != nil && isCritical {
			k.autoPauseVault(sdkCtx, vault, err)
		}
	}
}

func (k *Keeper) processSingleWithdrawal(ctx sdk.Context, id uint64, req types.PendingSwapOut, vault types.VaultAccount) (error, bool) {
	vaultAddr := sdk.MustAccAddressFromBech32(req.VaultAddress)
	ownerAddr := sdk.MustAccAddressFromBech32(req.Owner)
	principalAddress := markertypes.MustGetMarkerAddress(req.Shares.Denom)

	if err := k.ReconcileVaultInterest(ctx, &vault); err != nil {
		return fmt.Errorf("failed to reconcile vault interest: %w", err), false
	}

	assets, err := k.ConvertSharesToRedeemCoin(ctx, vault, req.Shares.Amount, req.RedeemDenom)
	if err != nil {
		return fmt.Errorf("failed to convert shares to redeem coin for single withdrawal: %w", err), false
	}

	if err := k.BankKeeper.SendCoins(markertypes.WithTransferAgents(ctx, vaultAddr), principalAddress, ownerAddr, sdk.NewCoins(assets)); err != nil {
		return err, false
	}

	if err := k.BankKeeper.SendCoins(ctx, vaultAddr, principalAddress, sdk.NewCoins(req.Shares)); err != nil {
		err = fmt.Errorf(
			"failed to transfer %s shares from %s to principal %s for burning: %w",
			req.Shares, vaultAddr, principalAddress, err,
		)
		return err, true
	}

	if err := k.MarkerKeeper.BurnCoin(ctx, vaultAddr, req.Shares); err != nil {
		err = fmt.Errorf(
			"failed to burn %s shares from account %s after successful payout: %w",
			req.Shares, vaultAddr, err,
		)
		return err, true
	}

	k.emitEvent(ctx, types.NewEventSwapOutCompleted(req.VaultAddress, req.Owner, assets, id))
	return nil, false
}

func (k *Keeper) refundWithdrawal(ctx sdk.Context, id uint64, req types.PendingSwapOut, reason string) error {
	vaultAddr := sdk.MustAccAddressFromBech32(req.VaultAddress)
	ownerAddr := sdk.MustAccAddressFromBech32(req.Owner)

	if err := k.BankKeeper.SendCoins(ctx, vaultAddr, ownerAddr, sdk.NewCoins(req.Shares)); err != nil {
		return fmt.Errorf(
			"failed to refund %s shares from vault %s to owner %s: %w",
			req.Shares, vaultAddr, ownerAddr, err,
		)
	}

	k.emitEvent(ctx, types.NewEventSwapOutRefunded(req.VaultAddress, req.Owner, req.Shares, id, reason))
	return nil
}

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
