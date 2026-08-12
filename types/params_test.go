package types_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/provlabs/vault/types"
)

func TestDefaultParams(t *testing.T) {
	params := types.DefaultParams()

	assert.Equal(t, types.DefaultTechFeeAddress.String(), params.TechFeeAddress, "default tech fee address")
	assert.Equal(t, uint32(types.DefaultAumFeeBips), params.DefaultAumFeeBips, "default AUM fee bips")
	assert.False(t, params.GovOnlyVaultCreation, "vault creation should be open to any signer by default so a chain must opt in to the governance gate")
	require.NoError(t, params.Validate(), "default params should be valid")
}

func TestGetDefaultGovOnlyVaultCreation(t *testing.T) {
	tests := []struct {
		name     string
		chainID  string
		expected bool
	}{
		{
			name:     "mainnet gates vault creation on governance",
			chainID:  types.MainnetChainID,
			expected: true,
		},
		{
			name:     "testnet leaves vault creation open",
			chainID:  types.TestnetChainID,
			expected: false,
		},
		{
			name:     "local chain leaves vault creation open",
			chainID:  "vaulty-1",
			expected: false,
		},
		{
			name:     "empty chain id leaves vault creation open",
			chainID:  "",
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, types.GetDefaultGovOnlyVaultCreation(tc.chainID), "default gov-only vault creation for chain %q", tc.chainID)
		})
	}
}

func TestParams_Validate_GovOnlyVaultCreation(t *testing.T) {
	tests := []struct {
		name    string
		govOnly bool
	}{
		{
			name:    "gate enabled is valid",
			govOnly: true,
		},
		{
			name:    "gate disabled is valid",
			govOnly: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			params := types.DefaultParams()
			params.GovOnlyVaultCreation = tc.govOnly

			assert.NoError(t, params.Validate(), "either setting of gov_only_vault_creation should validate")
		})
	}
}
