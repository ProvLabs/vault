package types

import (
	"fmt"
	"math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// DefaultGenesisState returns the default genesis state
func DefaultGenesisState() *GenesisState {
	return &GenesisState{}
}

// Validate performs basic genesis state validation returning an error upon any
// failure.
func (gs GenesisState) Validate() error {
	if len(gs.AumFeeAddress) > 0 {
		if len(gs.AumFeeAddress) != 20 { // Standard address length check, or use sdk.VerifyAddressFormat
			return fmt.Errorf("invalid aum fee address length: expected 20 bytes, got %d", len(gs.AumFeeAddress))
		}
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

		if entry.Time != uint64(v.PeriodTimeout) {
			return fmt.Errorf("payout timeout queue time mismatch for vault %s: expected %d, got %d", entry.Addr, uint64(v.PeriodTimeout), entry.Time)
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

		if entry.Time != uint64(v.FeePeriodTimeout) {
			return fmt.Errorf("fee timeout queue time mismatch for vault %s: expected %d, got %d", entry.Addr, uint64(v.FeePeriodTimeout), entry.Time)
		}

		if entry.Time > math.MaxInt64 {
			return fmt.Errorf("fee timeout queue entry at index %d has time %d which exceeds max int64", i, entry.Time)
		}
	}

	return nil
}
