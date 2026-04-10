package types

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// DefaultGenesisState returns the default genesis state
func DefaultGenesisState() *GenesisState {
	return &GenesisState{}
}

// Validate performs basic genesis state validation returning an error upon any
// failure.
func (gs GenesisState) Validate() error {
	seen := make(map[string]struct{}, len(gs.PayoutVerificationSet))
	for _, addr := range gs.PayoutVerificationSet {
		if _, dup := seen[addr]; dup {
			return fmt.Errorf("duplicate payout verification address %s", addr)
		}
		seen[addr] = struct{}{}
		if _, err := sdk.AccAddressFromBech32(addr); err != nil {
			return fmt.Errorf("invalid payout verification address %q: %w", addr, err)
		}
	}
	return nil
}
