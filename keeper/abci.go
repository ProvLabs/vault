package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (k *Keeper) BeginBlocker(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	blockTime := sdkCtx.BlockTime().Unix()

	iter, err := k.VaultInterestDetails.Iterate(ctx, nil)
	if err != nil {
		return err
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		vaultAddr, err := iter.Key()
		if err != nil {
			continue
		}

		details, err := iter.Value()
		if err != nil {
			continue
		}

		if details.ExpireTime <= blockTime {
			vault, err := k.GetVault(sdkCtx, vaultAddr)
			if err != nil {
				continue
			}
			if vault == nil {
				continue
			}
			if err := k.ReconcileVaultInterest(sdkCtx, vault); err != nil {
				continue
			}
		}
	}

	return nil
}

func (k *Keeper) EndBlocker(ctx context.Context) error {
	return nil
}
