package types_test

import (
	"math"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"

	"github.com/provlabs/vault/types"
	"github.com/provlabs/vault/utils"
)

func TestGenesisState_Validate(t *testing.T) {
	validAddr := utils.TestAddress().Bech32
	invalidAddr := "invalid-address"
	validVault := types.VaultAccount{
		BaseAccount:         authtypes.NewBaseAccountWithAddress(sdk.MustAccAddressFromBech32(validAddr)),
		Admin:               validAddr,
		UnderlyingAsset:     "under",
		PaymentDenom:        "under",
		TotalShares:         sdk.NewInt64Coin("share", 0),
		CurrentInterestRate: types.ZeroInterestRate,
		DesiredInterestRate: types.ZeroInterestRate,
	}

	tests := []struct {
		name        string
		genState    types.GenesisState
		expectedErr string
	}{
		{
			name:     "default genesis is valid",
			genState: *types.DefaultGenesisState(),
		},
		{
			name: "valid payout timeout queue",
			genState: types.GenesisState{
				Vaults: []types.VaultAccount{validVault},
				PayoutTimeoutQueue: []types.QueueEntry{
					{Time: 100, Addr: validAddr},
				},
			},
		},
		{
			name: "invalid address in payout timeout queue",
			genState: types.GenesisState{
				PayoutTimeoutQueue: []types.QueueEntry{
					{Time: 100, Addr: invalidAddr},
				},
			},
			expectedErr: "invalid payout timeout queue address at index 0",
		},
		{
			name: "time exceeds max int64 in payout timeout queue",
			genState: types.GenesisState{
				Vaults: []types.VaultAccount{validVault},
				PayoutTimeoutQueue: []types.QueueEntry{
					{Time: uint64(math.MaxInt64) + 1, Addr: validAddr},
				},
			},
			expectedErr: "payout timeout queue entry at index 0 has time 9223372036854775808 which exceeds max int64",
		},
		{
			name: "valid fee timeout queue",
			genState: types.GenesisState{
				Vaults: []types.VaultAccount{validVault},
				FeeTimeoutQueue: []types.QueueEntry{
					{Time: 100, Addr: validAddr},
				},
			},
		},
		{
			name: "invalid address in fee timeout queue",
			genState: types.GenesisState{
				FeeTimeoutQueue: []types.QueueEntry{
					{Time: 100, Addr: invalidAddr},
				},
			},
			expectedErr: "invalid fee timeout queue address at index 0",
		},
		{
			name: "time exceeds max int64 in fee timeout queue",
			genState: types.GenesisState{
				Vaults: []types.VaultAccount{validVault},
				FeeTimeoutQueue: []types.QueueEntry{
					{Time: uint64(math.MaxInt64) + 1, Addr: validAddr},
				},
			},
			expectedErr: "fee timeout queue entry at index 0 has time 9223372036854775808 which exceeds max int64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.genState.Validate()
			if tt.expectedErr == "" {
				require.NoError(t, err, "Validate() should not return an error for test case: %s", tt.name)
			} else {
				require.Error(t, err, "Validate() should return an error for test case: %s", tt.name)
				require.Contains(t, err.Error(), tt.expectedErr, "Validate() error message mismatch for test case: %s", tt.name)
			}
		})
	}
}
