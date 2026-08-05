package types_test

import (
	"math/big"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"

	"github.com/provlabs/vault/types"
)

func TestValidateIntStringLength(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		expErr    string
		expEchoed bool
	}{
		{
			name:  "empty string is within the bound",
			value: "",
		},
		{
			name:  "ordinary amount is within the bound",
			value: "100000000000",
		},
		{
			name:  "largest representable math.Int is within the bound",
			value: maxRepresentableIntString(t),
		},
		{
			name:  "string exactly at the bound is accepted",
			value: strings.Repeat("9", types.MaxIntStringLength),
		},
		{
			name:   "string one character over the bound is rejected",
			value:  strings.Repeat("9", types.MaxIntStringLength+1),
			expErr: "invalid shares amount: must be at most 80 characters",
		},
		{
			name:      "oversized value is not echoed into the error",
			value:     strings.Repeat("7", 4096),
			expErr:    "invalid shares amount: must be at most 80 characters",
			expEchoed: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := types.ValidateIntStringLength("shares amount", tc.value)
			if tc.expErr == "" {
				require.NoError(t, err, "ValidateIntStringLength should accept a %d character value", len(tc.value))
				return
			}
			require.EqualError(t, err, tc.expErr, "ValidateIntStringLength error for a %d character value", len(tc.value))
			if tc.expEchoed {
				assert.NotContains(t, err.Error(), tc.value, "ValidateIntStringLength must not echo the caller-supplied value")
			}
		})
	}
}

func TestValidateDecStringLength(t *testing.T) {
	tests := []struct {
		name   string
		value  string
		expErr string
	}{
		{
			name:  "empty string is within the bound",
			value: "",
		},
		{
			name:  "ordinary rate is within the bound",
			value: "0.055",
		},
		{
			name:  "fully precise rate at maximum magnitude is within the bound",
			value: maxRepresentableIntString(t) + "." + strings.Repeat("9", sdkmath.LegacyPrecision),
		},
		{
			name:  "string exactly at the bound is accepted",
			value: strings.Repeat("9", types.MaxDecStringLength),
		},
		{
			name:   "string one character over the bound is rejected",
			value:  strings.Repeat("9", types.MaxDecStringLength+1),
			expErr: "invalid interest rate: must be at most 99 characters",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := types.ValidateDecStringLength("interest rate", tc.value)
			if tc.expErr == "" {
				require.NoError(t, err, "ValidateDecStringLength should accept a %d character value", len(tc.value))
				return
			}
			require.EqualError(t, err, tc.expErr, "ValidateDecStringLength error for a %d character value", len(tc.value))
		})
	}
}

// maxRepresentableIntString returns the largest value a math.Int can hold, asserting it fits
// inside MaxIntStringLength so the bound can never reject a valid amount.
func maxRepresentableIntString(t *testing.T) string {
	t.Helper()
	maxBig := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), sdkmath.MaxBitLen), big.NewInt(1))
	maxInt, ok := sdkmath.NewIntFromString(maxBig.String())
	require.True(t, ok, "the largest %d-bit value must be a representable math.Int", sdkmath.MaxBitLen)
	s := maxInt.String()
	require.LessOrEqual(t, len(s), types.MaxIntStringLength, "MaxIntStringLength must admit the largest representable math.Int %s", s)
	return s
}
