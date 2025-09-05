package keeper

import (
	"context"

	"cosmossdk.io/collections"
	sdk "github.com/cosmos/cosmos-sdk/types"
	markertypes "github.com/provenance-io/provenance/x/marker/types"
	"github.com/provlabs/vault/types"
)

// ProcessPendingWithdrawals iterates through the queue and processes any due withdrawals.
func (k *Keeper) ProcessPendingWithdrawals(ctx context.Context) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	now := sdkCtx.BlockTime().Unix()

	var processedKeys []collections.Triple[int64, sdk.AccAddress, uint64]

	k.PendingWithdrawalQueue.WalkDue(ctx, now, func(timestamp int64, vaultAddr sdk.AccAddress, id uint64, req types.PendingWithdrawal) (stop bool, err error) {
		processedKeys = append(processedKeys, collections.Join3(timestamp, vaultAddr, id))

		err = k.processSingleWithdrawal(sdkCtx, req)
		if err != nil {
			if refundErr := k.refundWithdrawal(sdkCtx, req); refundErr != nil {
				sdkCtx.Logger().Error("CRITICAL: failed to refund shares for failed withdrawal", "request_id", id, "error", refundErr)
			}
		}
		return false, nil
	})

	for _, key := range processedKeys {
		if err := k.PendingWithdrawalQueue.Remove(ctx, key); err != nil {
			sdkCtx.Logger().Error("CRITICAL: failed to dequeue processed withdrawal", "key", key, "error", err)
		}
	}
}

func (k *Keeper) processSingleWithdrawal(ctx sdk.Context, req types.PendingWithdrawal) error {
	vaultAddr := sdk.MustAccAddressFromBech32(req.VaultAddress)
	ownerAddr := sdk.MustAccAddressFromBech32(req.Owner)
	principalAddress := markertypes.MustGetMarkerAddress(req.Shares.Denom)
	if err := k.BankKeeper.SendCoins(markertypes.WithBypass(ctx), principalAddress, ownerAddr, sdk.NewCoins(req.Assets)); err != nil {
		// k.emitEvent(ctx, types.NewEventWithdrawalFailed(req.VaultAddress, req.Owner, req.Shares, "insufficient liquidity"))
		return err
	}

	if err := k.MarkerKeeper.BurnCoin(ctx, vaultAddr, req.Shares); err != nil {
		ctx.Logger().Error("CRITICAL: failed to burn shares after successful withdrawal payout", "error", err)
		return err
	}

	// k.emitEvent(ctx, types.NewEventWithdrawalCompleted(req.VaultAddress, req.Owner, req.Assets))
	return nil
}

func (k *Keeper) refundWithdrawal(ctx sdk.Context, req types.PendingWithdrawal) error {
	vaultAddr := sdk.MustAccAddressFromBech32(req.VaultAddress)
	ownerAddr := sdk.MustAccAddressFromBech32(req.Owner)

	if err := k.BankKeeper.SendCoins(ctx, vaultAddr, ownerAddr, sdk.NewCoins(req.Shares)); err != nil {
		return err
	}

	// k.emitEvent(ctx, types.NewEventWithdrawalRefunded(req.VaultAddress, req.Owner, req.Shares))
	return nil
}
