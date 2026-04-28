package keeper

import (
	"fmt"
	"math"

	"cosmossdk.io/collections"

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

	params := genState.Params
	// If TechFeeAddress is missing, we use the chain-specific default.
	if len(params.TechFeeAddress) == 0 {
		params.TechFeeAddress = types.GetDefaultTechFeeAddress(ctx.ChainID()).String()
	}
	// NOTE: DefaultAumFeeBips is allowed to be 0, so we don't default it if it's 0.
	// It's already validated by genState.Validate() -> Params.Validate().

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

	for _, entry := range genState.NetAssetValues {
		vaultAddr, err := sdk.AccAddressFromBech32(entry.VaultAddress)
		if err != nil {
			panic(fmt.Errorf("invalid vault address in net asset values: %w", err))
		}
		if err := k.NetAssetValues.Set(ctx, collections.Join(vaultAddr, entry.Denom), entry.Nav); err != nil {
			panic(fmt.Errorf("failed to set net asset value for vault %s and denom %s: %w", entry.VaultAddress, entry.Denom, err))
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

	var netAssetValues []types.NetAssetValueGenesisEntry
	err = k.NetAssetValues.Walk(ctx, nil, func(key collections.Pair[sdk.AccAddress, string], value types.VaultNAV) (stop bool, err error) {
		netAssetValues = append(netAssetValues, types.NetAssetValueGenesisEntry{
			VaultAddress: key.K1().String(),
			Denom:        key.K2(),
			Nav:          value,
		})
		return false, nil
	})
	if err != nil {
		panic(fmt.Errorf("failed to walk net asset values: %w", err))
	}

	return &types.GenesisState{
		Vaults:              vaults,
		PayoutTimeoutQueue:  paymentTimeoutQueue,
		FeeTimeoutQueue:     feeTimeoutQueue,
		PendingSwapOutQueue: *pendingSwapOutQueue,
		Params:              params,
		NetAssetValues:      netAssetValues,
	}
}
