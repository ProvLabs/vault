package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (k *Keeper) BeginBlocker(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	blockTime := sdkCtx.BlockTime().Unix()

	iter, err := k.VaultInterestDetails.Iterate(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to iterate VaultInterestDetails: %w", err)
	}
	defer iter.Close()

	var toDelete []sdk.AccAddress

	for ; iter.Valid(); iter.Next() {
		vaultAddr, err := iter.Key()
		if err != nil {
			sdkCtx.Logger().Error("failed to decode vault address", "err", err)
			continue
		}

		details, err := iter.Value()
		if err != nil {
			sdkCtx.Logger().Error("failed to decode vault interest details", "vault", vaultAddr.String(), "err", err)
			toDelete = append(toDelete, vaultAddr)
			continue
		}

		if details.ExpireTime <= blockTime {
			vault, err := k.GetVault(sdkCtx, vaultAddr)
			if err != nil || vault == nil {
				toDelete = append(toDelete, vaultAddr)
				continue
			}

			if err := k.ReconcileVaultInterest(sdkCtx, vault); err != nil {
				sdkCtx.Logger().Error("failed to reconcile interest", "vault", vaultAddr.String(), "err", err)
				continue
			}
		}
	}

	for _, addr := range toDelete {
		if err := k.VaultInterestDetails.Remove(ctx, addr); err != nil {
			sdkCtx.Logger().Error("failed to remove VaultInterestDetails for missing vault", "vault", addr.String(), "err", err)
		}
	}

	return nil
}

func (k *Keeper) EndBlocker(ctx context.Context) error {
	return nil
}
