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
	for i, entry := range gs.PayoutTimeoutQueue {
		if _, err := sdk.AccAddressFromBech32(entry.Addr); err != nil {
			return fmt.Errorf("invalid payout timeout queue address at index %d: %w", i, err)
		}
		if entry.Time > math.MaxInt64 {
			return fmt.Errorf("payout timeout queue entry at index %d has time %d which exceeds max int64", i, entry.Time)
		}
	}

	for i, entry := range gs.FeeTimeoutQueue {
		if _, err := sdk.AccAddressFromBech32(entry.Addr); err != nil {
			return fmt.Errorf("invalid fee timeout queue address at index %d: %w", i, err)
		}
		if entry.Time > math.MaxInt64 {
			return fmt.Errorf("fee timeout queue entry at index %d has time %d which exceeds max int64", i, entry.Time)
		}
	}

	return nil
}
