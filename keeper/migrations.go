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
	return m.keeper.MigrateVaultAccountPaymentDenomDefaults(ctx)
}

// MigrateVaultAccountPaymentDenomDefaults updates legacy VaultAccount state created
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