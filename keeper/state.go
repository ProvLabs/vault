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

// SetVaultLookup stores a vault in the Vaults collection, keyed by its bech32 address.
// NOTE: should only be called by genesis and at vault creation.
// Returns an error if the vault is nil or the address cannot be parsed.
func (k *Keeper) SetVaultLookup(ctx context.Context, vault *types.VaultAccount) error {
	if vault == nil {
		return errors.New("vault cannot be nil")
	}

	addr, err := sdk.AccAddressFromBech32(vault.Address)
	if err != nil {
		return err
	}

	return k.Vaults.Set(ctx, addr, []byte{})
}

// SetVaultAccount validates and persists a VaultAccount using the auth keeper.
// Returns an error if validation fails.
func (k *Keeper) SetVaultAccount(ctx sdk.Context, vault *types.VaultAccount) error {
	if err := vault.Validate(); err != nil {
		return err
	}
	k.AuthKeeper.SetAccount(ctx, vault)
	return nil
}

// EnqueueVaultStart schedules a vault for reconciliation by adding an entry
// to the VaultStartQueue keyed by the given period start and vault address.
func (k Keeper) EnqueueVaultStart(ctx context.Context, periodStart int64, vaultAddr sdk.AccAddress) error {
	return k.VaultStartQueue.Set(ctx, collections.Join(uint64(periodStart), vaultAddr), collections.NoValue{})
}

// EnqueueVaultTimeout schedules a vault for timeout processing by adding an entry
// to the VaultTimeoutQueue keyed by the given timeout and vault address.
func (k Keeper) EnqueueVaultTimeout(ctx context.Context, periodTimeout int64, vaultAddr sdk.AccAddress) error {
	return k.VaultTimeoutQueue.Set(ctx, collections.Join(uint64(periodTimeout), vaultAddr), collections.NoValue{})
}

// DequeueVaultStart removes a vault entry from the VaultStartQueue for the given
// period start and vault address.
func (k Keeper) DequeueVaultStart(ctx context.Context, periodStart int64, vaultAddr sdk.AccAddress) error {
	return k.VaultStartQueue.Remove(ctx, collections.Join(uint64(periodStart), vaultAddr))
}

// DequeueVaultTimeout removes a vault entry from the VaultTimeoutQueue for the given
// period timeout and vault address.
func (k Keeper) DequeueVaultTimeout(ctx context.Context, periodTimeout int64, vaultAddr sdk.AccAddress) error {
	return k.VaultTimeoutQueue.Remove(ctx, collections.Join(uint64(periodTimeout), vaultAddr))
}

// WalkDueStarts iterates over all entries in the VaultStartQueue with periodStart <= nowSec.
// For each due entry, it calls the provided callback function. Iteration stops if the
// callback returns stop=true or an error.
func (k Keeper) WalkDueStarts(ctx context.Context, nowSec int64, fn func(periodStart uint64, vaultAddr sdk.AccAddress) (stop bool, err error)) error {
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

// WalkDueTimeouts iterates over all entries in the VaultTimeoutQueue with periodTimeout <= nowSec.
// For each due entry, it calls the provided callback function. Iteration stops if the
// callback returns stop=true or an error.
func (k Keeper) WalkDueTimeouts(ctx context.Context, nowSec int64, fn func(periodTimeout uint64, vaultAddr sdk.AccAddress) (stop bool, err error)) error {
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

// RemoveAllTimeoutsForVault deletes all timeout entries in the VaultTimeoutQueue
// for the given vault address.
func (k Keeper) RemoveAllTimeoutsForVault(ctx context.Context, vaultAddr sdk.AccAddress) error {
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
		if kv.Key.K2().Equals(vaultAddr) {
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
