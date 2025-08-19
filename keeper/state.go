package keeper

import (
	"context"
	"errors"

	"cosmossdk.io/collections"

	"github.com/provlabs/vault/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// GetVaults is a helper function for retrieving all vaults from state.
func (k *Keeper) GetVaults(ctx context.Context) ([]sdk.AccAddress, error) {
	vaults := []sdk.AccAddress{}

	err := k.Vaults.Walk(ctx, nil, func(key sdk.AccAddress, val []byte) (stop bool, err error) {
		vaults = append(vaults, key)
		return false, nil
	})

	return vaults, err
}

// Sets a vault in the store, using its address as the key.
func (k *Keeper) SetVault(ctx context.Context, vault *types.VaultAccount) error {
	if vault == nil {
		return errors.New("vault cannot be nil")
	}

	addr, err := sdk.AccAddressFromBech32(vault.Address)
	if err != nil {
		return err
	}

	return k.Vaults.Set(ctx, addr, []byte{})
}

// SetVaultAccount persists a vault account after validating it.
func (k *Keeper) SetVaultAccount(ctx sdk.Context, vault *types.VaultAccount) error {
	if err := vault.Validate(); err != nil {
		return err
	}
	k.AuthKeeper.SetAccount(ctx, vault)
	return nil
}

func (k Keeper) EnqueueVaultStart(ctx context.Context, tSec int64, addr sdk.AccAddress) error {
	return k.VaultStartQueue.Set(ctx, collections.Join(uint64(tSec), addr), collections.NoValue{})
}

func (k Keeper) EnqueueVaultTimeout(ctx context.Context, tSec int64, addr sdk.AccAddress) error {
	return k.VaultTimeoutQueue.Set(ctx, collections.Join(uint64(tSec), addr), collections.NoValue{})
}

func (k Keeper) DequeueVaultStart(ctx context.Context, tSec int64, addr sdk.AccAddress) error {
	return k.VaultStartQueue.Remove(ctx, collections.Join(uint64(tSec), addr))
}

func (k Keeper) DequeueVaultEnd(ctx context.Context, tSec int64, addr sdk.AccAddress) error {
	return k.VaultTimeoutQueue.Remove(ctx, collections.Join(uint64(tSec), addr))
}

func (k Keeper) WalkDueStarts(ctx context.Context, nowSec int64, fn func(t uint64, addr sdk.AccAddress) (stop bool, err error)) error {
	it, err := k.VaultStartQueue.Iterate(ctx, nil)
	if err != nil {
		return err
	}
	defer it.Close()

	for ; it.Valid(); it.Next() {
		kv, err := it.KeyValue()
		if err != nil {
			return err
		}
		if kv.Key.K1() > uint64(nowSec) {
			break
		}
		stop, err := fn(kv.Key.K1(), kv.Key.K2())
		if err != nil || stop {
			return err
		}
	}
	return nil
}

func (k Keeper) WalkDueTimeouts(ctx context.Context, nowSec int64, fn func(t uint64, addr sdk.AccAddress) (stop bool, err error)) error {
	it, err := k.VaultTimeoutQueue.Iterate(ctx, nil)
	if err != nil {
		return err
	}
	defer it.Close()

	for ; it.Valid(); it.Next() {
		kv, err := it.KeyValue()
		if err != nil {
			return err
		}
		if kv.Key.K1() > uint64(nowSec) {
			break
		}
		stop, err := fn(kv.Key.K1(), kv.Key.K2())
		if err != nil || stop {
			return err
		}
	}
	return nil
}

func (k Keeper) RemoveAllStartsForVault(ctx context.Context, addr sdk.AccAddress) error {
	var keys []collections.Pair[uint64, sdk.AccAddress]

	it, err := k.VaultStartQueue.Iterate(ctx, nil)
	if err != nil {
		return err
	}
	defer it.Close()

	for ; it.Valid(); it.Next() {
		kv, err := it.KeyValue()
		if err != nil {
			return err
		}
		if kv.Key.K2().Equals(addr) {
			keys = append(keys, kv.Key)
		}
	}
	for _, key := range keys {
		if err := k.VaultStartQueue.Remove(ctx, key); err != nil {
			return err
		}
	}
	return nil
}

func (k Keeper) RemoveAllTimeoutsForVault(ctx context.Context, addr sdk.AccAddress) error {
	var keys []collections.Pair[uint64, sdk.AccAddress]

	it, err := k.VaultTimeoutQueue.Iterate(ctx, nil)
	if err != nil {
		return err
	}
	defer it.Close()

	for ; it.Valid(); it.Next() {
		kv, err := it.KeyValue()
		if err != nil {
			return err
		}
		if kv.Key.K2().Equals(addr) {
			keys = append(keys, kv.Key)
		}
	}
	for _, key := range keys {
		if err := k.VaultTimeoutQueue.Remove(ctx, key); err != nil {
			return err
		}
	}
	return nil
}
