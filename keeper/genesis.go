package keeper

import (
	"fmt"
	"math"

	"github.com/provlabs/vault/types"

	"cosmossdk.io/collections"

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

	params := types.DefaultParams()
	if len(genState.Params.TechFeeAddress) > 0 {
		params.TechFeeAddress = genState.Params.TechFeeAddress
	} else {
		// Fallback to chain-specific default if TechFeeAddress is not provided.
		params.TechFeeAddress = types.GetDefaultTechFeeAddress(ctx.ChainID()).String()
	}
	if genState.Params.DefaultAumFeeBips > 0 {
		params.DefaultAumFeeBips = genState.Params.DefaultAumFeeBips
	}

	if err := k.Params.Set(ctx, params); err != nil {
		panic(fmt.Errorf("failed to set params: %w", err))
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

		if err := k.SetVaultLookup(ctx, v); err != nil {
			panic(fmt.Errorf("failed to store vault %s: %w", v.Address, err))
		}
	}

	for _, entry := range genState.PayoutTimeoutQueue {
		addr, err := sdk.AccAddressFromBech32(entry.Addr)
		if err != nil {
			panic(fmt.Errorf("invalid address in payout timeout queue: %w", err))
		}
		if _, ok := k.tryGetVault(ctx, addr); !ok {
			panic(fmt.Errorf("payout timeout queue entry for non-existent vault %s", entry.Addr))
		}
		if entry.Time > math.MaxInt64 {
			panic(fmt.Errorf("payout timeout queue entry for %s has time %d which exceeds max int64", entry.Addr, entry.Time))
		}
		if err := k.PayoutTimeoutQueue.Enqueue(ctx, int64(entry.Time), addr); err != nil {
			panic(fmt.Errorf("failed to enqueue vault payout timeout for %s: %w", entry.Addr, err))
		}
	}

	for _, entry := range genState.FeeTimeoutQueue {
		addr, err := sdk.AccAddressFromBech32(entry.Addr)
		if err != nil {
			panic(fmt.Errorf("invalid address in fee timeout queue: %w", err))
		}
		if _, ok := k.tryGetVault(ctx, addr); !ok {
			panic(fmt.Errorf("fee timeout queue entry for non-existent vault %s", entry.Addr))
		}
		if entry.Time > math.MaxInt64 {
			panic(fmt.Errorf("fee timeout queue entry for %s has time %d which exceeds max int64", entry.Addr, entry.Time))
		}
		if err := k.FeeTimeoutQueue.Enqueue(ctx, int64(entry.Time), addr); err != nil {
			panic(fmt.Errorf("failed to enqueue vault fee timeout for %s: %w", entry.Addr, err))
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

	for _, entry := range genState.Navs {
		addr, err := sdk.AccAddressFromBech32(entry.VaultAddress)
		if err != nil {
			panic(fmt.Errorf("invalid vault address in nav entry: %w", err))
		}
		if _, err := k.MarkerKeeper.GetMarkerByDenom(ctx, entry.Nav.Denom); err != nil {
			panic(fmt.Errorf("nav denom %q for vault %s is not a registered marker: %w", entry.Nav.Denom, entry.VaultAddress, err))
		}
		if err := k.NAVs.Set(ctx, collections.Join(addr, entry.Nav.Denom), entry.Nav); err != nil {
			panic(fmt.Errorf("failed to import vault nav for %s/%s: %w", entry.VaultAddress, entry.Nav.Denom, err))
		}
	}
}

// ExportGenesis exports the current state of the vault module.
func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	params, err := k.Params.Get(ctx)
	if err != nil {
		params = types.DefaultParams()
	}

	allAccounts := k.AuthKeeper.GetAllAccounts(ctx)

	var vaults []types.VaultAccount
	for _, acc := range allAccounts {
		if v, ok := acc.(*types.VaultAccount); ok {
			vaults = append(vaults, *v)
		}
	}

	paymentTimeoutQueue := make([]types.QueueEntry, 0)

	err = k.PayoutTimeoutQueue.Walk(ctx, func(periodTimeout uint64, vaultAddr sdk.AccAddress) (stop bool, err error) {
		paymentTimeoutQueue = append(paymentTimeoutQueue, types.QueueEntry{
			Time: periodTimeout,
			Addr: vaultAddr.String(),
		})
		return false, nil
	})
	if err != nil {
		panic(fmt.Errorf("failed to walk payout timeout queue: %w", err))
	}

	feeTimeoutQueue := make([]types.QueueEntry, 0)

	err = k.FeeTimeoutQueue.Walk(ctx, func(feeTimeout uint64, vaultAddr sdk.AccAddress) (stop bool, err error) {
		feeTimeoutQueue = append(feeTimeoutQueue, types.QueueEntry{
			Time: feeTimeout,
			Addr: vaultAddr.String(),
		})
		return false, nil
	})
	if err != nil {
		panic(fmt.Errorf("failed to walk fee timeout queue: %w", err))
	}

	pendingSwapOutQueue, err := k.PendingSwapOutQueue.Export(ctx)
	if err != nil {
		panic(fmt.Errorf("failed to export pending swap out queue: %w", err))
	}

	navs := make([]types.VaultNAVEntry, 0)
	err = k.NAVs.Walk(ctx, nil, func(key collections.Pair[sdk.AccAddress, string], value types.VaultNAV) (stop bool, err error) {
		navs = append(navs, types.VaultNAVEntry{
			VaultAddress: key.K1().String(),
			Nav:          value,
		})
		return false, nil
	})
	if err != nil {
		panic(fmt.Errorf("failed to walk vault navs: %w", err))
	}

	return &types.GenesisState{
		Vaults:              vaults,
		PayoutTimeoutQueue:  paymentTimeoutQueue,
		FeeTimeoutQueue:     feeTimeoutQueue,
		PendingSwapOutQueue: *pendingSwapOutQueue,
		Params:              params,
		Navs:                navs,
	}
}
