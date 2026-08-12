package types

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// DefaultParams returns default vault module parameters.
func DefaultParams() Params {
	return Params{
		DefaultAumFeeBips:    DefaultAumFeeBips,
		TechFeeAddress:       DefaultTechFeeAddress.String(),
		GovOnlyVaultCreation: DefaultGovOnlyVaultCreation,
	}
}

// GetDefaultTechFeeAddress returns the default tech fee address based on the chain ID.
func GetDefaultTechFeeAddress(chainID string) sdk.AccAddress {
	switch chainID {
	case MainnetChainID:
		return MainnetTechFeeAddress
	case TestnetChainID:
		return TestnetTechFeeAddress
	default:
		return DefaultTechFeeAddress
	}
}

// GetDefaultGovOnlyVaultCreation reports whether a chain gates vault creation on governance
// by default. Only mainnet does.
func GetDefaultGovOnlyVaultCreation(chainID string) bool {
	return chainID == MainnetChainID
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
