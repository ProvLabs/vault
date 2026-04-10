package keeper

import (
	"fmt"
	"sort"

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
			if err := v.Validate(); err != nil {
				panic(err)
			}
			if err := k.SetVaultLookup(ctx, v.Clone()); err != nil {
				panic(fmt.Errorf("failed to set vault lookup for existing vault %s: %w", v.GetAddress(), err))
			}
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
		if err := k.PayoutTimeoutQueue.Enqueue(ctx, int64(entry.Time), addr); err != nil {
			panic(fmt.Errorf("failed to enqueue vault timeout for %s: %w", entry.Addr, err))
		}
	}

	for _, entry := range genState.PendingSwapOutQueue.Entries {
		vaultAddr, err := sdk.AccAddressFromBech32(entry.SwapOut.VaultAddress)
		if err != nil {
			panic(fmt.Errorf("invalid vault address in pending swap out queue: %w", err))
		}
		if _, ok := k.tryGetVault(ctx, vaultAddr); !ok {
			panic(fmt.Errorf("pending queue entry for unknown vault %s", entry.SwapOut.VaultAddress))
		}
	}

	if err := k.PendingSwapOutQueue.Import(ctx, &genState.PendingSwapOutQueue); err != nil {
		panic(fmt.Errorf("failed to import pending swap out queue: %w", err))
	}

	for _, addrStr := range genState.PayoutVerificationSet {
		addr, err := sdk.AccAddressFromBech32(addrStr)
		if err != nil {
			panic(fmt.Errorf("invalid address in payout verification set: %w", err))
		}
		if _, ok := k.tryGetVault(ctx, addr); !ok {
			panic(fmt.Errorf("payout verification set entry for unknown vault %s", addrStr))
		}
		if err := k.PayoutVerificationSet.Set(ctx, addr); err != nil {
			panic(fmt.Errorf("failed to restore payout verification entry for %s: %w", addrStr, err))
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

	err := k.PayoutTimeoutQueue.Walk(ctx, func(periodTimeout uint64, vaultAddr sdk.AccAddress) (stop bool, err error) {
		paymentTimeoutQueue = append(paymentTimeoutQueue, types.QueueEntry{
			Time: periodTimeout,
			Addr: vaultAddr.String(),
		})
		return false, nil
	})
	if err != nil {
		panic(fmt.Errorf("failed to walk payout timeout queue: %w", err))
	}

	pendingSwapOutQueue, err := k.PendingSwapOutQueue.Export(ctx)
	if err != nil {
		panic(fmt.Errorf("failed to export pending swap out queue: %w", err))
	}

	var payoutVerificationAddrs []string
	err = k.PayoutVerificationSet.Walk(ctx, nil, func(addr sdk.AccAddress) (bool, error) {
		payoutVerificationAddrs = append(payoutVerificationAddrs, addr.String())
		return false, nil
	})
	if err != nil {
		panic(fmt.Errorf("failed to walk payout verification set: %w", err))
	}
	sort.Strings(payoutVerificationAddrs)

	return &types.GenesisState{
		Vaults:                  vaults,
		PayoutTimeoutQueue:      paymentTimeoutQueue,
		PendingSwapOutQueue:     *pendingSwapOutQueue,
		PayoutVerificationSet:   payoutVerificationAddrs,
	}
}

func vaultExists(ctx sdk.Context, k Keeper, addr sdk.AccAddress) bool {
	_, err := k.GetVault(ctx, addr)
	return err == nil
}
