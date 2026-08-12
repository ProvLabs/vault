package types

import (
	fmt "fmt"

	sdkmath "cosmossdk.io/math"
)

const (
	// MaxIntStringLength bounds a caller-supplied integer string, since a math.Int caps at
	// MaxBitLen = 256 and so is never more than 78 decimal digits.
	MaxIntStringLength = 80

	// MaxDecStringLength bounds a caller-supplied decimal string, which adds a decimal point
	// and LegacyPrecision fractional digits to the integer bound.
	MaxDecStringLength = MaxIntStringLength + 1 + sdkmath.LegacyPrecision
)

// ValidateIntStringLength rejects an over-length integer string before math/big scans it, naming
// the field rather than echoing the value so an unbounded input is never reflected back.
func ValidateIntStringLength(field, value string) error {
	if len(value) > MaxIntStringLength {
		return fmt.Errorf("invalid %s: must be at most %d characters", field, MaxIntStringLength)
	}
	return nil
}

// ValidateDecStringLength rejects an over-length decimal string before math/big scans it, naming
// the field rather than echoing the value so an unbounded input is never reflected back.
func ValidateDecStringLength(field, value string) error {
	if len(value) > MaxDecStringLength {
		return fmt.Errorf("invalid %s: must be at most %d characters", field, MaxDecStringLength)
	}
	return nil
}
