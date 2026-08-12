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
	params.DefaultAumFeeBips = genState.Params.DefaultAumFeeBips
	params.GovOnlyVaultCreation = genState.Params.GovOnlyVaultCreation

	if err := k.Params.Set(ctx, params); err != nil {
		panic(fmt.Errorf("failed to set params: %w", err))
	}

	accounts := k.AuthKeeper.GetAllAccounts(ctx)
	for _, acc := range accounts {
		if v, ok := acc.(types.VaultAccountI); ok {
			if err := v.Validate(); err != nil {
				panic(err)
			}
			if err := k.validateShareSupplyInvariant(ctx, v.Clone()); err != nil {
				panic(fmt.Errorf("invalid existing vault %s: %w", v.GetAddress(), err))
			}
			if err := k.SetVaultLookup(ctx, v.Clone()); err != nil {
				panic(fmt.Errorf("failed to set vault lookup for existing vault %s: %w", v.GetAddress(), err))
			}
		}
	}

	for i := range genState.Vaults {
		v := &genState.Vaults[i]

		if err := k.validateShareSupplyInvariant(ctx, v); err != nil {
			panic(fmt.Errorf("invalid vault %s in genesis: %w", v.Address, err))
		}

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

	for _, addr := range genState.PayoutVerificationSet {
		vaultAddr, err := sdk.AccAddressFromBech32(addr)
		if err != nil {
			panic(fmt.Errorf("invalid address in payout verification set: %w", err))
		}
		if _, ok := k.tryGetVault(ctx, vaultAddr); !ok {
			panic(fmt.Errorf("payout verification set entry for non-existent vault %s", addr))
		}
		if err := k.PayoutVerificationSet.Set(ctx, vaultAddr); err != nil {
			panic(fmt.Errorf("failed to set payout verification for %s: %w", addr, err))
		}
	}

	if err := k.restorePayoutVerificationSet(ctx); err != nil {
		panic(fmt.Errorf("failed to restore payout verification set: %w", err))
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
		vault, ok := k.tryGetVault(ctx, addr)
		if !ok {
			panic(fmt.Errorf("nav entry for unknown vault %s", entry.VaultAddress))
		}
		if err := types.ValidateNAVComponentMagnitudes(entry.Nav.Price, entry.Nav.Volume); err != nil {
			k.getLogger(ctx).Warn("skipping oversized vault nav entry",
				"vault", entry.VaultAddress,
				"denom", entry.Nav.Denom,
				"err", err,
			)
			continue
		}
		if err := validateVaultNAVFields(vault, entry.Nav); err != nil {
			panic(fmt.Errorf("invalid nav entry for vault %s: %w", entry.VaultAddress, err))
		}
		// A priced scope can be deleted after the fact, so skip stale entries instead of
		// panicking and blocking the import.
		if err := k.requireNAVDenomRegistered(ctx, entry.Nav.Denom); err != nil {
			k.getLogger(ctx).Warn("skipping vault nav entry for unregistered denom",
				"vault", entry.VaultAddress,
				"denom", entry.Nav.Denom,
				"err", err,
			)
			continue
		}
		if err := k.NAVs.Set(ctx, collections.Join(addr, entry.Nav.Denom), entry.Nav); err != nil {
			panic(fmt.Errorf("failed to import vault nav for %s/%s: %w", entry.VaultAddress, entry.Nav.Denom, err))
		}
	}

	if err := k.rebuildNAVCounts(ctx); err != nil {
		panic(fmt.Errorf("failed to seed vault nav entry counts: %w", err))
	}

	if err := k.HydrateTotalValues(ctx); err != nil {
		panic(fmt.Errorf("failed to seed vault total values: %w", err))
	}
}

// restorePayoutVerificationSet re-derives payout verification set membership from imported vault
// state. A vault mid-interest-cycle is tracked in exactly one of two places: the payout timeout
// queue when it holds a scheduled timeout, or the verification set when it awaits the next
// affordability check. SafeAddPayoutVerification clears the timeout as it moves a vault into the
// set, so a set member carries PeriodStart != 0 and PeriodTimeout == 0 and owns no queue entry —
// enough to rebuild membership without reading the genesis field.
//
// Deriving rather than trusting the field alone repairs any genesis exported before
// payout_verification_set existed. Without it an idle vault exported from the set lands in neither
// structure, so no blocker ever visits it again: its interest keeps accruing while the
// CanPayInterestDuration check that zeroes an unaffordable rate never runs.
//
// A paused vault is skipped because pausing clears both periods and both queue entries, and
// membership is idempotent, so a genesis that carries the field derives the same entries.
func (k Keeper) restorePayoutVerificationSet(ctx sdk.Context) error {
	queued := make(map[string]bool)
	if err := k.PayoutTimeoutQueue.Walk(ctx, func(_ uint64, vaultAddr sdk.AccAddress) (stop bool, err error) {
		queued[vaultAddr.String()] = true
		return false, nil
	}); err != nil {
		return fmt.Errorf("failed to walk payout timeout queue: %w", err)
	}

	vaultAddrs, err := k.GetVaults(ctx)
	if err != nil {
		return fmt.Errorf("failed to list vaults: %w", err)
	}

	for _, vaultAddr := range vaultAddrs {
		vault, ok := k.tryGetVault(ctx, vaultAddr)
		if !ok {
			continue
		}
		if vault.Paused || vault.PeriodStart == 0 || vault.PeriodTimeout != 0 || queued[vaultAddr.String()] {
			continue
		}
		k.getLogger(ctx).Info("restoring payout verification entry derived from imported vault state",
			"vault", vaultAddr.String(),
			"period_start", vault.PeriodStart,
		)
		if err := k.PayoutVerificationSet.Set(ctx, vaultAddr); err != nil {
			return fmt.Errorf("failed to restore payout verification entry for vault %s: %w", vaultAddr, err)
		}
	}

	return nil
}

// validateShareSupplyInvariant returns an error when a vault's total_shares is below the local bank supply of its share denom
func (k Keeper) validateShareSupplyInvariant(ctx sdk.Context, vault *types.VaultAccount) error {
	_, err := k.availableBridgeMintCapacity(ctx, vault)
	return err
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

	payoutVerificationSet := make([]string, 0)

	err = k.PayoutVerificationSet.Walk(ctx, nil, func(vaultAddr sdk.AccAddress) (stop bool, err error) {
		payoutVerificationSet = append(payoutVerificationSet, vaultAddr.String())
		return false, nil
	})
	if err != nil {
		panic(fmt.Errorf("failed to walk payout verification set: %w", err))
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
		if key.K2() != value.Denom {
			return true, fmt.Errorf("nav key/value denom mismatch for vault %s: key=%q value=%q", key.K1(), key.K2(), value.Denom)
		}
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
		Vaults:                vaults,
		PayoutTimeoutQueue:    paymentTimeoutQueue,
		PayoutVerificationSet: payoutVerificationSet,
		FeeTimeoutQueue:       feeTimeoutQueue,
		PendingSwapOutQueue:   *pendingSwapOutQueue,
		Params:                params,
		Navs:                  navs,
	}
}
