package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/provlabs/vault/types"
)

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
				return err
			}
		}
	}

	return nil
}
