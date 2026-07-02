package keeper

import (
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

// Migrate1to2 advances the vault module from ConsensusVersion 1 to 2 by seeding
// the Internal NAV table from Marker NAVs, defaulting nav_authority to the vault
// admin when unset, and normalizing the FeeRemainder field on legacy VaultAccount
// state to an explicit zero. The work is delegated to unexported helpers, each of
// which is idempotent across retries.
func (m Migrator) Migrate1to2(ctx sdk.Context) error {
	if err := m.keeper.migrateInternalNAVSeedFromMarker(ctx); err != nil {
		return err
	}
	return m.keeper.migrateVaultFeeRemainderDefaults(ctx)
}
