package types

import (
	"fmt"
	"math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// DefaultGenesisState returns the default genesis state
func DefaultGenesisState() *GenesisState {
	return &GenesisState{
		Params: DefaultParams(),
	}
}

// Validate performs basic genesis state validation returning an error upon any
// failure.
func (gs GenesisState) Validate() error {
	if err := gs.Params.Validate(); err != nil {
		return fmt.Errorf("invalid params: %w", err)
	}

	vaults := make(map[string]VaultAccount)
	for i := range gs.Vaults {
		if err := gs.Vaults[i].Validate(); err != nil {
			return fmt.Errorf("invalid vault at index %d: %w", i, err)
		}
		if _, exists := vaults[gs.Vaults[i].Address]; exists {
			return fmt.Errorf("duplicate vault address in genesis: %s", gs.Vaults[i].Address)
		}
		vaults[gs.Vaults[i].Address] = gs.Vaults[i]
	}

	payoutQueueAddrs := make(map[string]bool)
	for i, entry := range gs.PayoutTimeoutQueue {
		if _, err := sdk.AccAddressFromBech32(entry.Addr); err != nil {
			return fmt.Errorf("invalid payout timeout queue address at index %d: %w", i, err)
		}
		v, exists := vaults[entry.Addr]
		if !exists {
			return fmt.Errorf("payout timeout queue address at index %d is not an imported vault: %s", i, entry.Addr)
		}
		if payoutQueueAddrs[entry.Addr] {
			return fmt.Errorf("duplicate payout timeout queue entry for vault: %s", entry.Addr)
		}
		payoutQueueAddrs[entry.Addr] = true

		expectedPeriodTimeout := uint64(v.PeriodTimeout) //nolint:gosec // G115: PeriodTimeout is validated non-negative.
		if entry.Time != expectedPeriodTimeout {
			return fmt.Errorf("payout timeout queue time mismatch for vault %s: expected %d, got %d", entry.Addr, expectedPeriodTimeout, entry.Time)
		}

		if entry.Time > math.MaxInt64 {
			return fmt.Errorf("payout timeout queue entry at index %d has time %d which exceeds max int64", i, entry.Time)
		}
	}

	feeQueueAddrs := make(map[string]bool)
	for i, entry := range gs.FeeTimeoutQueue {
		if _, err := sdk.AccAddressFromBech32(entry.Addr); err != nil {
			return fmt.Errorf("invalid fee timeout queue address at index %d: %w", i, err)
		}
		v, exists := vaults[entry.Addr]
		if !exists {
			return fmt.Errorf("fee timeout queue address at index %d is not an imported vault: %s", i, entry.Addr)
		}
		if feeQueueAddrs[entry.Addr] {
			return fmt.Errorf("duplicate fee timeout queue entry for vault: %s", entry.Addr)
		}
		feeQueueAddrs[entry.Addr] = true

		expectedFeePeriodTimeout := uint64(v.FeePeriodTimeout) //nolint:gosec // G115: FeePeriodTimeout is validated non-negative.
		if entry.Time != expectedFeePeriodTimeout {
			return fmt.Errorf("fee timeout queue time mismatch for vault %s: expected %d, got %d", entry.Addr, expectedFeePeriodTimeout, entry.Time)
		}

		if entry.Time > math.MaxInt64 {
			return fmt.Errorf("fee timeout queue entry at index %d has time %d which exceeds max int64", i, entry.Time)
		}
	}

	if gs.PendingSwapOutQueue.Entries != nil {
		for i, entry := range gs.PendingSwapOutQueue.Entries {
			if entry.SwapOut.VaultAddress == "" {
				return fmt.Errorf("nil SwapOut in pending swap out queue at index %d", i)
			}
			if _, err := sdk.AccAddressFromBech32(entry.SwapOut.VaultAddress); err != nil {
				return fmt.Errorf("invalid vault address in pending swap out queue at index %d: %w", i, err)
			}
			if _, exists := vaults[entry.SwapOut.VaultAddress]; !exists {
				return fmt.Errorf("pending swap out queue vault address at index %d is not an imported vault: %s", i, entry.SwapOut.VaultAddress)
			}
		}
	}

	navKeys := make(map[string]bool)
	for i, entry := range gs.Navs {
		if _, err := sdk.AccAddressFromBech32(entry.VaultAddress); err != nil {
			return fmt.Errorf("invalid nav vault address at index %d: %w", i, err)
		}
		v, exists := vaults[entry.VaultAddress]
		if !exists {
			return fmt.Errorf("nav entry at index %d is not for an imported vault: %s", i, entry.VaultAddress)
		}
		if err := sdk.ValidateDenom(entry.Nav.Denom); err != nil {
			return fmt.Errorf("invalid nav denom at index %d: %w", i, err)
		}
		if entry.Nav.Denom == v.TotalShares.Denom {
			return fmt.Errorf("nav entry at index %d prices the vault share denom %s", i, entry.Nav.Denom)
		}
		if entry.Nav.Denom == entry.Nav.Price.Denom {
			return fmt.Errorf("nav entry at index %d has matching denom and price denom %q", i, entry.Nav.Denom)
		}
		if err := entry.Nav.Price.Validate(); err != nil {
			return fmt.Errorf("invalid nav price at index %d: %w", i, err)
		}
		if entry.Nav.Price.Denom != v.UnderlyingAsset {
			return fmt.Errorf("nav price denom at index %d %q is not the underlying asset for vault %s", i, entry.Nav.Price.Denom, entry.VaultAddress)
		}
		if entry.Nav.Volume.IsNil() || !entry.Nav.Volume.IsPositive() {
			return fmt.Errorf("nav volume at index %d must be positive", i)
		}
		if len(entry.Nav.Source) > MaxNAVSourceLength {
			return fmt.Errorf("nav source at index %d too long (expected <= %d, actual: %d)", i, MaxNAVSourceLength, len(entry.Nav.Source))
		}
		key := entry.VaultAddress + "/" + entry.Nav.Denom
		if navKeys[key] {
			return fmt.Errorf("duplicate nav entry for vault %s denom %s", entry.VaultAddress, entry.Nav.Denom)
		}
		navKeys[key] = true
	}

	return nil
}
