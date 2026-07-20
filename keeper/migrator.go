package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Migrator wraps the Keeper to expose versioned migration handlers registered
// with the module Configurator via cfg.RegisterMigration. Each Migrate*to*
// method corresponds to a single ConsensusVersion bump and is invoked by the
// SDK module manager during RunMigrations when the stored module version is
// behind module.ConsensusVersion.
type Migrator struct {
	keeper *Keeper
}

// NewMigrator constructs a Migrator that delegates to the supplied Keeper.
func NewMigrator(k *Keeper) Migrator {
	return Migrator{keeper: k}
}

// Migrate1to2 advances the vault module from ConsensusVersion 1 to 2 by
// flattening every vault to single-denom and enabling deposit protection on
// every vault's share marker. Both steps are idempotent across retries.
func (m Migrator) Migrate1to2(ctx sdk.Context) error {
	if err := m.keeper.migrateFlattenMixedDenomVaults(ctx); err != nil {
		return fmt.Errorf("failed to flatten mixed-denom vaults: %w", err)
	}
	if err := m.keeper.migrateEnableMarkerDepositProtection(ctx); err != nil {
		return fmt.Errorf("failed to enable marker deposit protection: %w", err)
	}
	return nil
}
