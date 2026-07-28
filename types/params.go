package types

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// DefaultParams returns default vault module parameters.
func DefaultParams() Params {
	return Params{
		DefaultAumFeeBips:  DefaultAumFeeBips,
		TechFeeAddress:     DefaultTechFeeAddress.String(),
		MaxVaultNavEntries: DefaultMaxVaultNAVEntries,
	}
}

// MaxVaultNAVEntriesOrDefault returns the configured per-vault NAV entry cap.
//
// Zero means unset, and yields DefaultMaxVaultNAVEntries rather than a cap of zero or
// no cap at all, so a chain that upgrades into this field without a governance vote gets
// the module default.
func (p Params) MaxVaultNAVEntriesOrDefault() uint32 {
	if p.MaxVaultNavEntries == 0 {
		return DefaultMaxVaultNAVEntries
	}
	return p.MaxVaultNavEntries
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

	if p.MaxVaultNavEntries > MaxVaultNAVEntriesLimit {
		return fmt.Errorf("invalid MaxVaultNavEntries: %d (max %d)", p.MaxVaultNavEntries, MaxVaultNAVEntriesLimit)
	}

	return nil
}
