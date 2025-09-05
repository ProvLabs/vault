package keeper

import (
	"context"

	"cosmossdk.io/collections"
	sdk "github.com/cosmos/cosmos-sdk/types"
	markertypes "github.com/provenance-io/provenance/x/marker/types"
	"github.com/provlabs/vault/types"
)

// ProcessPendingWithdrawals iterates through the queue of withdrawal requests that are due to be
// processed at the current block time. For each due request, it attempts to pay out the assets.
// If a payout fails (e.g., due to insufficient liquidity), it refunds the user's escrowed shares.
// All processed requests (both successful and failed) are dequeued. Errors during the queue walk
// are returned, but individual payout/refund failures are logged and do not halt the process.
func (k *Keeper) ProcessPendingWithdrawals(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	now := sdkCtx.BlockTime().Unix()

	var processedKeys []collections.Triple[int64, sdk.AccAddress, uint64]

	err := k.PendingWithdrawalQueue.WalkDue(ctx, now, func(timestamp int64, vaultAddr sdk.AccAddress, id uint64, req types.PendingWithdrawal) (stop bool, err error) {
		processedKeys = append(processedKeys, collections.Join3(timestamp, vaultAddr, id))

		err = k.processSingleWithdrawal(sdkCtx, id, req)
		if err != nil {
			if refundErr := k.refundWithdrawal(sdkCtx, id, req, err.Error()); refundErr != nil {
				sdkCtx.Logger().Error("CRITICAL: failed to refund shares for failed withdrawal", "request_id", id, "error", refundErr)
			}
		}
		return false, nil
	})

	if err != nil {
		sdkCtx.Logger().Error("error during pending withdrawal queue walk", "error", err)
		return err
	}

	for _, key := range processedKeys {
		if err := k.PendingWithdrawalQueue.Remove(ctx, key); err != nil {
			sdkCtx.Logger().Error("CRITICAL: failed to dequeue processed withdrawal", "key", key, "error", err)
		}
	}
	return nil
}

// processSingleWithdrawal handles the successful state changes for a pending withdrawal.
// It sends the pre-calculated assets from the vault's principal to the owner and then
// burns the owner's escrowed shares from the vault's own account. An event is emitted on success.
// A critical error is logged if the share burning fails after the payout has already succeeded.
func (k *Keeper) processSingleWithdrawal(ctx sdk.Context, id uint64, req types.PendingWithdrawal) error {
	vaultAddr := sdk.MustAccAddressFromBech32(req.VaultAddress)
	ownerAddr := sdk.MustAccAddressFromBech32(req.Owner)
	principalAddress := markertypes.MustGetMarkerAddress(req.Shares.Denom)

	if err := k.BankKeeper.SendCoins(markertypes.WithBypass(ctx), principalAddress, ownerAddr, sdk.NewCoins(req.Assets)); err != nil {
		return err
	}

	if err := k.MarkerKeeper.BurnCoin(ctx, vaultAddr, req.Shares); err != nil {
		ctx.Logger().Error("CRITICAL: failed to burn shares after successful withdrawal payout", "error", err)
		return err
	}

	k.emitEvent(ctx, types.NewEventWithdrawalCompleted(req.VaultAddress, req.Owner, req.Assets, id))
	return nil
}

// refundWithdrawal handles the failure case for a pending withdrawal. It returns the user's
// escrowed shares from the vault's own account back to the owner and emits an event detailing
// the refund and the reason for the failure.
func (k *Keeper) refundWithdrawal(ctx sdk.Context, id uint64, req types.PendingWithdrawal, reason string) error {
	vaultAddr := sdk.MustAccAddressFromBech32(req.VaultAddress)
	ownerAddr := sdk.MustAccAddressFromBech32(req.Owner)

	if err := k.BankKeeper.SendCoins(ctx, vaultAddr, ownerAddr, sdk.NewCoins(req.Shares)); err != nil {
		return err
	}

	k.emitEvent(ctx, types.NewEventWithdrawalRefunded(req.VaultAddress, req.Owner, req.Shares, id, reason))
	return nil
}
