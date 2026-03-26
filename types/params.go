package types

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// DefaultParams returns default vault module parameters.
func DefaultParams() Params {
	return Params{
		DefaultAumFeeBips: DefaultAumFeeBips,
		TechFeeAddress:    DefaultTechFeeAddress.String(),
	}
}

// GetDefaultTechFeeAddress returns the default tech fee address based on the chain ID.
func GetDefaultTechFeeAddress(chainID string) sdk.AccAddress {
	switch chainID {
	case "pio-mainnet-1":
		return MainnetTechFeeAddress
	case "pio-testnet-1":
		return TestnetTechFeeAddress
	default:
		return DefaultTechFeeAddress
	}
}

// Validate checks that the parameters have valid values.
func (p Params) Validate() error {
	if _, err := sdk.AccAddressFromBech32(p.TechFeeAddress); err != nil {
		return fmt.Errorf("invalid TechFeeAddress: %w", err)
	}

	if p.DefaultAumFeeBips > 10_000 {
		return fmt.Errorf("invalid DefaultAumFeeBips: %d (max 10000)", p.DefaultAumFeeBips)
	}

	return nil
}
