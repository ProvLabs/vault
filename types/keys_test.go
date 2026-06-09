package types_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/provlabs/vault/types"
)

// TestTechFeeAddresses verifies that each hardcoded tech fee address constant
// round-trips to the bech32 string documented in its godoc.
func TestTechFeeAddresses(t *testing.T) {
	tests := []struct {
		name     string
		addr     sdk.AccAddress
		hrp      string
		expected string
	}{
		{
			name:     "default",
			addr:     types.DefaultTechFeeAddress,
			hrp:      "pb",
			expected: "pb1evyv7neax9qtxxzuexnhylxyz4guvsyjjqke4h",
		},
		{
			name:     "testnet",
			addr:     types.TestnetTechFeeAddress,
			hrp:      "tp",
			expected: "tp1u7j87uj7e6xrkhgwmq9pqr47kdjdw6mhgv7dm4",
		},
		{
			name:     "mainnet",
			addr:     types.MainnetTechFeeAddress,
			hrp:      "pb",
			expected: "pb144z5tr4lk85xvhrplsrh6gmclr8pgpv76fqjt23gzz9awrmh4laqclqdun",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hrp, bz, err := bech32.DecodeAndConvert(tc.expected)
			require.NoError(t, err, "DecodeAndConvert(%q)", tc.expected)
			assert.Equal(t, tc.hrp, hrp, "human-readable prefix of %s tech fee address", tc.name)
			assert.Equal(t, []byte(tc.addr), bz, "%s tech fee address bytes should match the documented bech32 string", tc.name)

			encoded, err := bech32.ConvertAndEncode(tc.hrp, tc.addr)
			require.NoError(t, err, "ConvertAndEncode(%s tech fee address)", tc.name)
			assert.Equal(t, tc.expected, encoded, "%s tech fee address should encode back to the documented bech32 string", tc.name)
		})
	}
}
