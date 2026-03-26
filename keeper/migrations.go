package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/provlabs/vault/types"
)
// Migrator is a struct for handling in-place store migrations.
type Migrator struct {
	keeper Keeper
}

// NewMigrator returns a new Migrator.
func NewMigrator(keeper Keeper) Migrator {
	return Migrator{keeper: keeper}
}

// Migrate1to2 migrates from version 1 to 2.
func (m Migrator) Migrate1to2(ctx sdk.Context) error {
	if err := m.keeper.MigrateVaultAccountPaymentDenomDefaults(ctx); err != nil {
		return err
	}
	return m.keeper.MigrateAUMFeeParams(ctx)
}

// MigrateVaultAccountPaymentDenomDefaults updates legacy VaultAccount state created
...
// prior to v1.0.13 by normalizing empty payment denom fields.
//
// In versions <= v1.0.13, VaultAccount instances could be persisted with an empty
// PaymentDenom. Newer versions require a valid payment denom, which by default
// should be the vault’s underlying asset denom.
//
// This migration iterates all accounts in the auth store, identifies VaultAccount
// instances, and for any vault with an empty PaymentDenom sets it to UnderlyingAsset.
// Updated vault accounts are validated and re-persisted using the auth keeper.
//
// This function is intended to be executed once from an upgrade handler and is
// idempotent; running it multiple times will not modify already-migrated state.
func (k Keeper) MigrateVaultAccountPaymentDenomDefaults(ctx sdk.Context) error {
	allAccounts := k.AuthKeeper.GetAllAccounts(ctx)

	for _, acc := range allAccounts {
		v, ok := acc.(*types.VaultAccount)
		if !ok {
			continue
		}

		if v.PaymentDenom == "" {
			v.PaymentDenom = v.UnderlyingAsset
			err := k.SetVaultAccount(ctx, v)
			if err != nil {
				return fmt.Errorf("failed to update vault account %s during migration: %w", v.Address, err)
			}
		}
	}

	return nil
}

// MigrateAUMFeeParams initializes the module parameters if they are not already set,
// and updates existing vaults to use the default AUM fee bips.
func (k Keeper) MigrateAUMFeeParams(ctx sdk.Context) error {
	params, err := k.Params.Get(ctx)
	if err != nil {
		params = types.DefaultParams()

		// Attempt to read legacy AUM fee recipient from legacy AUMFeeAddressKeyPrefix (prefix 8).
		kvStore := k.storeService.OpenKVStore(ctx)
		bz, err := kvStore.Get(types.AUMFeeAddressKeyPrefix)
		if err == nil && len(bz) > 0 {
			params.TechFeeAddress = sdk.AccAddress(bz).String()
		} else {
			params.TechFeeAddress = types.GetDefaultTechFeeAddress(ctx.ChainID()).String()
		}

		if err := k.Params.Set(ctx, params); err != nil {
			return fmt.Errorf("failed to initialize params during migration: %w", err)
		}
	}

	allAccounts := k.AuthKeeper.GetAllAccounts(ctx)
	for _, acc := range allAccounts {
		v, ok := acc.(*types.VaultAccount)
		if !ok {
			continue
		}

		if v.AumFeeBips == 0 {
			v.AumFeeBips = params.DefaultAumFeeBips
			if err := k.SetVaultAccount(ctx, v); err != nil {
				return fmt.Errorf("failed to update vault account %s during migration: %w", v.Address, err)
			}
		}
	}

	return nil
}