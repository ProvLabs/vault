package types_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/provlabs/vault/types"
)

func TestParams_Validate(t *testing.T) {
	techFeeAddress := NewTestAddress()

	tests := []struct {
		name        string
		params      types.Params
		expectedErr string
	}{
		{
			name:   "default params are valid",
			params: types.DefaultParams(),
		},
		{
			name: "unset max vault NAV entries is valid and means the module default",
			params: types.Params{
				TechFeeAddress:     techFeeAddress,
				DefaultAumFeeBips:  15,
				MaxVaultNavEntries: 0,
			},
		},
		{
			name: "max vault NAV entries at the limit is valid",
			params: types.Params{
				TechFeeAddress:     techFeeAddress,
				DefaultAumFeeBips:  15,
				MaxVaultNavEntries: types.MaxVaultNAVEntriesLimit,
			},
		},
		{
			name: "max vault NAV entries above the limit is rejected",
			params: types.Params{
				TechFeeAddress:     techFeeAddress,
				DefaultAumFeeBips:  15,
				MaxVaultNavEntries: types.MaxVaultNAVEntriesLimit + 1,
			},
			expectedErr: "invalid MaxVaultNavEntries",
		},
		{
			name: "invalid tech fee address is rejected",
			params: types.Params{
				TechFeeAddress:     "notanaddress",
				DefaultAumFeeBips:  15,
				MaxVaultNavEntries: 100,
			},
			expectedErr: "invalid TechFeeAddress",
		},
		{
			name: "AUM fee bips above 10000 is rejected",
			params: types.Params{
				TechFeeAddress:     techFeeAddress,
				DefaultAumFeeBips:  10_001,
				MaxVaultNavEntries: 100,
			},
			expectedErr: "invalid DefaultAumFeeBips",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.params.Validate()
			if tc.expectedErr != "" {
				require.Error(t, err, "Validate should reject params for case %q", tc.name)
				assert.Contains(t, err.Error(), tc.expectedErr, "Validate error for case %q should mention %q", tc.name, tc.expectedErr)
				return
			}
			assert.NoError(t, err, "Validate should accept params for case %q", tc.name)
		})
	}
}

func TestParams_MaxVaultNAVEntriesOrDefault(t *testing.T) {
	tests := []struct {
		name     string
		params   types.Params
		expected uint32
	}{
		{
			name:     "unset cap yields the module default",
			params:   types.Params{MaxVaultNavEntries: 0},
			expected: types.DefaultMaxVaultNAVEntries,
		},
		{
			name:     "configured cap is returned unchanged",
			params:   types.Params{MaxVaultNavEntries: 7},
			expected: 7,
		},
		{
			name:     "default params yield the module default",
			params:   types.DefaultParams(),
			expected: types.DefaultMaxVaultNAVEntries,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.params.MaxVaultNAVEntriesOrDefault(), "MaxVaultNAVEntriesOrDefault for case %q", tc.name)
		})
	}
}
