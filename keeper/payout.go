package keeper

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/provlabs/vault/types"

	"cosmossdk.io/collections"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	markertypes "github.com/provenance-io/provenance/x/marker/types"
)

// ProcessPendingSwapOuts processes the queue of pending swap-out requests. Called from the EndBlocker,
// it iterates through requests due for payout at the current block time.
// For each request, it attempts a payout. If the payout fails due to a recoverable error (e.g., insufficient liquidity),
// it refunds the user's escrowed shares. All processed requests are dequeued, regardless of success or failure.
// This function returns an error only if the queue walk itself fails; it does not return errors from individual
// payout/refund operations.
func (k *Keeper) ProcessPendingSwapOuts(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	now := sdkCtx.BlockTime().Unix()

	var processedKeys []collections.Triple[int64, sdk.AccAddress, uint64]

	err := k.PendingSwapOutQueue.WalkDue(ctx, now, func(timestamp int64, id uint64, vaultAddr sdk.AccAddress, req types.PendingSwapOut) (stop bool, err error) {
		processedKeys = append(processedKeys, collections.Join3(timestamp, vaultAddr, id))

		_, ok := k.tryGetVault(sdkCtx, vaultAddr)
		if !ok {
			sdkCtx.Logger().Error("skipping pending withdrawal for non-existent vault", "request_id", id, "vault_address", vaultAddr.String())
			return false, nil
		}

		err = k.processSingleWithdrawal(sdkCtx, id, req)
		if err != nil {
			reason := k.getRefundReason(err)
			sdkCtx.Logger().Error("Failed to process withdrawal",
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

	for _, key := range processedKeys {
		if err := k.PendingSwapOutQueue.Dequeue(ctx, key.K1(), key.K2(), key.K3()); err != nil {
			sdkCtx.Logger().Error("CRITICAL: failed to dequeue processed withdrawal", "key", key, "error", err)
		}
	}
	return nil
}

// processSingleWithdrawal executes a pending swap-out, paying out assets to the owner and burning their escrowed shares.
// It returns a non-nil error only for recoverable failures (e.g., insufficient liquidity for payout), which signals
// the caller to issue a refund. It panics for any critical, unrecoverable state inconsistencies that occur *after* the
// user has been paid, such as failing to burn the escrowed shares. An EventSwapOutCompleted is emitted on success.
func (k *Keeper) processSingleWithdrawal(ctx sdk.Context, id uint64, req types.PendingSwapOut) error {
	vaultAddr := sdk.MustAccAddressFromBech32(req.VaultAddress)
	ownerAddr := sdk.MustAccAddressFromBech32(req.Owner)
	principalAddress := markertypes.MustGetMarkerAddress(req.Shares.Denom)

	vault, ok := k.tryGetVault(ctx, vaultAddr)
	if !ok {
		// TODO Is this an error or a panic
		return fmt.Errorf("vault for single withdrawal not found at address: %s", vaultAddr.String())
	}

	assets, err := k.ConvertSharesToRedeemCoin(ctx, *vault, req.Shares.Amount, req.RedeemDenom)
	if err != nil {
		// TODO Is this an error or a panic.
		return err
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
	case strings.Contains(errMsg, "does not contain the required attribute"):
		return types.RefundReasonRecipientMissingAttributes
	case strings.Contains(errMsg, "is on deny list"),
		strings.Contains(errMsg, "does not have transfer permissions"),
		strings.Contains(errMsg, "does not have access"):
		return types.RefundReasonPermissionDenied
	case strings.Contains(errMsg, "cannot be sent to the fee collector"):
		return types.RefundReasonRecipientInvalid
	}

	return types.RefundReasonUnknown
}
