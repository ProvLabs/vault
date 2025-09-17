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
// it iterates through requests due for payout at the current block time. It skips any requests for
// vaults that are currently paused, leaving them in the queue to be re-evaluated in the next block.
// For each due request, it first attempts to dequeue it. If the dequeue fails, an error is logged, and the
// item is skipped to prevent duplicate processing. If the dequeue is successful, the payout is attempted.
// If the payout fails due to a recoverable error (e.g., insufficient liquidity), it refunds the
// user's escrowed shares. This function returns an error only if the queue walk itself fails; it does not
// return errors from individual payout/refund operations.
func (k *Keeper) ProcessPendingSwapOuts(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	now := sdkCtx.BlockTime().Unix()

	err := k.PendingSwapOutQueue.WalkDue(ctx, now, func(timestamp int64, id uint64, vaultAddr sdk.AccAddress, req types.PendingSwapOut) (stop bool, err error) {
		vault, ok := k.tryGetVault(sdkCtx, vaultAddr)
		if ok && vault.Paused {
			return false, nil
		}

		if err := k.PendingSwapOutQueue.Dequeue(ctx, timestamp, vaultAddr, id); err != nil {
			sdkCtx.Logger().Error("CRITICAL: failed to dequeue pending withdrawal, skipping processing to prevent re-execution", "id", id, "error", err)
			return false, nil
		}

		if !ok {
			sdkCtx.Logger().Error("dequeued and skipped pending withdrawal for non-existent vault", "request_id", id, "vault_address", vaultAddr.String())
			return false, nil
		}

		err = k.processSingleWithdrawal(sdkCtx, id, req, *vault)
		if err != nil {
			reason := k.getRefundReason(err)
			sdkCtx.Logger().Error("Failed to process withdrawal, issuing refund",
				"withdrawal_id", id,
				"reason", reason,
				"error", err,
			)
			k.refundWithdrawal(sdkCtx, id, req, reason)
		}
		return false, nil
	})

	if err != nil {
		sdkCtx.Logger().Error("error during pending withdrawal queue walk", "error", err)
		return err
	}

	return nil
}

// processSingleWithdrawal executes a pending swap-out. It first reconciles vault interest, then converts the user's
// shares to the redeemable asset amount. It then pays out those assets to the owner and burns their escrowed shares.
// It returns a non-nil error for recoverable failures (e.g., insufficient liquidity), which signals
// the caller to issue a refund. It panics for critical, unrecoverable state inconsistencies that occur *after* the
// user has been paid, such as failing to burn the escrowed shares. An EventSwapOutCompleted is emitted on success.
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
		panic(fmt.Errorf("CRITICAL: failed to transfer escrowed shares to principal for burning %w", err))
	}

	if err := k.MarkerKeeper.BurnCoin(ctx, vaultAddr, req.Shares); err != nil {
		panic(fmt.Errorf("CRITICAL: failed to burn shares after successful swap out payout %w", err))
	}

	k.emitEvent(ctx, types.NewEventSwapOutCompleted(req.VaultAddress, req.Owner, assets, id))
	return nil
}

// refundWithdrawal handles the failure case for a pending swap out. It returns the user's
// escrowed shares from the vault's own account back to the owner and emits an EventSwapOutRefunded.
// This function panics if the refund transfer fails, as this represents a critical state inconsistency
// where user funds would otherwise be lost.
func (k *Keeper) refundWithdrawal(ctx sdk.Context, id uint64, req types.PendingSwapOut, reason string) {
	vaultAddr := sdk.MustAccAddressFromBech32(req.VaultAddress)
	ownerAddr := sdk.MustAccAddressFromBech32(req.Owner)

	if err := k.BankKeeper.SendCoins(ctx, vaultAddr, ownerAddr, sdk.NewCoins(req.Shares)); err != nil {
		panic(fmt.Errorf("CRITICAL: failed to refund shares for failed withdrawal %w", err))
	}

	k.emitEvent(ctx, types.NewEventSwapOutRefunded(req.VaultAddress, req.Owner, req.Shares, id, reason))
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
