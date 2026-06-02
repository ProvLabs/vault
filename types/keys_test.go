package types_test

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/types/bech32"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/provlabs/vault/types"
)

// TestMainnetTechFeeAddress verifies that the hardcoded MainnetTechFeeAddress bytes
// round-trip to the bech32 string documented in its godoc.
func TestMainnetTechFeeAddress(t *testing.T) {
	const expected = "pb144z5tr4lk85xvhrplsrh6gmclr8pgpv76fqjt23gzz9awrmh4laqclqdun"

	hrp, bz, err := bech32.DecodeAndConvert(expected)
	require.NoError(t, err, "DecodeAndConvert(%q)", expected)
	assert.Equal(t, "pb", hrp, "human-readable prefix of MainnetTechFeeAddress")
	assert.Equal(t, []byte(types.MainnetTechFeeAddress), bz, "MainnetTechFeeAddress bytes should match the documented bech32 string")

	encoded, err := bech32.ConvertAndEncode("pb", types.MainnetTechFeeAddress)
	require.NoError(t, err, "ConvertAndEncode(MainnetTechFeeAddress)")
	assert.Equal(t, expected, encoded, "MainnetTechFeeAddress should encode back to the documented bech32 string")
}
