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
	for _, entry := range genState.TimeoutQueue {
		addr, err := sdk.AccAddressFromBech32(entry.Addr)
		if err != nil {
			panic(fmt.Errorf("invalid address in timeout queue: %w", err))
		}
		if err := k.EnqueueVaultTimeout(ctx, int64(entry.Time), addr); err != nil {
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

	startQueue := make([]types.QueueEntry, 0)
	err := k.WalkDueStarts(ctx, ctx.BlockTime().Unix(), func(addr sdk.AccAddress) (stop bool, err error) {
		startQueue = append(startQueue, types.QueueEntry{
			Addr: addr.String(),
		})
		return false, nil
	})
	if err != nil {
		panic(fmt.Errorf("failed to walk vault start queue: %w", err))
	}

	timeoutQueue := make([]types.QueueEntry, 0)
	err = k.WalkDueTimeouts(ctx, ctx.BlockTime().Unix(), func(t uint64, addr sdk.AccAddress) (stop bool, err error) {
		timeoutQueue = append(timeoutQueue, types.QueueEntry{
			Time: t,
			Addr: addr.String(),
		})
		return false, nil
	})
	if err != nil {
		panic(fmt.Errorf("failed to walk vault timeout queue: %w", err))
	}

	return &types.GenesisState{
		Vaults:       vaults,
		TimeoutQueue: timeoutQueue,
	}
}
