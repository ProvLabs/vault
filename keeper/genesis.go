package keeper

import (
	"fmt"

	"github.com/provlabs/vault/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// InitGenesis initializes the vault module state from genesis.
func (k Keeper) InitGenesis(ctx sdk.Context, genState *types.GenesisState) {
	if genState == nil {
		return
	}

	if err := genState.Validate(); err != nil {
		panic(fmt.Errorf("invalid vault genesis state: %w", err))
	}

	accounts := k.AuthKeeper.GetAllAccounts(ctx)
	for _, acc := range accounts {
		if v, ok := acc.(types.VaultAccountI); ok {
			if err := v.Validate(); err == nil {
				panic(err)
			}
			k.SetVaultLookup(ctx, v.Clone())
		}
	}

	for i := range genState.Vaults {
		v := &genState.Vaults[i]

		existing := k.AuthKeeper.GetAccount(ctx, v.GetAddress())
		if existing != nil {
			if err := v.SetAccountNumber(existing.GetAccountNumber()); err != nil {
				panic(fmt.Errorf("failed to set account number for vault %s: %w", v.Address, err))
			}
			if err := k.SetVaultAccount(ctx, v); err != nil {
				panic(fmt.Errorf("unable to set vault account %s: %w", v.Address, err))
			}
		} else {
			vaultAcc := k.AuthKeeper.NewAccount(ctx, v).(types.VaultAccountI)
			k.AuthKeeper.SetAccount(ctx, vaultAcc)
		}

		if err := v.Validate(); err != nil {
			panic(fmt.Errorf("invalid vault at index %d: %w", i, err))
		}

		if err := k.SetVaultLookup(ctx, v); err != nil {
			panic(fmt.Errorf("failed to store vault %s: %w", v.Address, err))
		}
	}
	for _, entry := range genState.PayoutTimeoutQueue {
		addr, err := sdk.AccAddressFromBech32(entry.Addr)
		if err != nil {
			panic(fmt.Errorf("invalid address in timeout queue: %w", err))
		}
		if err := k.EnqueuePayoutTimeout(ctx, int64(entry.Time), addr); err != nil {
			panic(fmt.Errorf("failed to enqueue vault timeout for %s: %w", entry.Addr, err))
		}
	}
}

// ExportGenesis exports the current state of the vault module.
func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	allAccounts := k.AuthKeeper.GetAllAccounts(ctx)

	var vaults []types.VaultAccount
	for _, acc := range allAccounts {
		if v, ok := acc.(*types.VaultAccount); ok {
			vaults = append(vaults, *v)
		}
	}

	paymentTimeoutQueue := make([]types.QueueEntry, 0)

	it, err := k.PayoutTimeoutQueue.Iterate(ctx, nil)
	if err != nil {
		panic(fmt.Errorf("failed to iterate payout timeout queue: %w", err))
	}
	defer it.Close()

	for ; it.Valid(); it.Next() {
		kv, err := it.Key()
		if err != nil {
			panic(fmt.Errorf("failed to read payout timeout queue entry: %w", err))
		}
		paymentTimeoutQueue = append(paymentTimeoutQueue, types.QueueEntry{
			Time: kv.K1(),
			Addr: kv.K2().String(),
		})
	}

	return &types.GenesisState{
		Vaults:             vaults,
		PayoutTimeoutQueue: paymentTimeoutQueue,
	}
}
