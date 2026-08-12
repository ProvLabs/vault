package keeper

import (
	"context"
	"errors"
	"fmt"

	"github.com/provlabs/vault/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// GetVaults is a helper function for retrieving all vaults from state.
func (k *Keeper) GetVaults(ctx context.Context) ([]sdk.AccAddress, error) {
	vaults := []sdk.AccAddress{}

	err := k.Vaults.Walk(ctx, nil, func(key sdk.AccAddress, _ []byte) (stop bool, err error) {
		vaults = append(vaults, key)
		return false, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk vaults: %w", err)
	}

	return vaults, nil
}

// resolveVaults returns the vault account behind every entry in the vault lookup, plus the number of
// entries it could not resolve. Every consumer of the lookup goes through here, so none of them can
// disagree about which entries count as vaults.
//
// An entry with no account, or one whose address holds a non-vault account, is inert: nothing else
// reads it and the vault it names owns nothing. It is logged and skipped rather than failing an
// upgrade or halting a chain. purpose names the caller in those logs.
func (k Keeper) resolveVaults(ctx sdk.Context, purpose string) ([]types.VaultAccount, int, error) {
	addrs, err := k.GetVaults(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list vaults: %w", err)
	}

	vaults := make([]types.VaultAccount, 0, len(addrs))
	skipped := 0
	for _, addr := range addrs {
		vault, err := k.GetVault(ctx, addr)
		switch {
		case err != nil:
			skipped++
			k.getLogger(ctx).Error("skipping unloadable vault lookup entry",
				"purpose", purpose,
				"vault", addr.String(),
				"err", err,
			)
		case vault == nil:
			skipped++
			k.getLogger(ctx).Error("skipping vault lookup entry with no vault account",
				"purpose", purpose,
				"vault", addr.String(),
			)
		default:
			vaults = append(vaults, *vault)
		}
	}

	return vaults, skipped, nil
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
		return fmt.Errorf("failed to parse vault address %s: %w", vault.Address, err)
	}

	return k.Vaults.Set(ctx, addr, []byte{})
}

// SetVaultAccount validates and persists a VaultAccount using the auth keeper.
// Returns an error if validation fails.
func (k *Keeper) SetVaultAccount(ctx sdk.Context, vault *types.VaultAccount) error {
	if err := vault.Validate(); err != nil {
		return fmt.Errorf("failed to validate vault: %w", err)
	}
	k.AuthKeeper.SetAccount(ctx, vault)
	return nil
}

// FindVaultAccount retrieves a vault by its address or share denomination.
func (k *Keeper) FindVaultAccount(ctx sdk.Context, id string) (*types.VaultAccount, error) {
	if addr, err := sdk.AccAddressFromBech32(id); err == nil {
		vault, err := k.GetVault(ctx, addr)
		if err != nil {
			return nil, fmt.Errorf("failed to get vault: %w", err)
		}
		if vault != nil {
			return vault, nil
		}
	}

	addr := types.GetVaultAddress(id)
	vault, err := k.GetVault(ctx, addr)
	if err != nil {
		return nil, fmt.Errorf("failed to get vault: %w", err)
	}
	if vault != nil {
		return vault, nil
	}

	return nil, fmt.Errorf("vault with id '%s' not found: %w", id, types.ErrVaultNotFound)
}
