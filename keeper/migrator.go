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

// Migrate1to2 advances the vault module from ConsensusVersion 1 to 2 by
// flattening every vault to single-denom: payment denoms collapse onto the
// underlying asset, outstanding AUM fees are re-denominated into the
// underlying, pending swap-outs redeem in the underlying, and nav_authority
// defaults to the vault admin when unset. The work is delegated to the
// unexported migrateFlattenMixedDenomVaults, which is idempotent across
// retries.
func (m Migrator) Migrate1to2(ctx sdk.Context) error {
	return m.keeper.migrateFlattenMixedDenomVaults(ctx)
}
