package types_test

import (
	"math"
	"testing"

	sdkmath "cosmossdk.io/math"
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
				Params: types.DefaultParams(),
				Vaults: []types.VaultAccount{
					func() types.VaultAccount {
						v := validVault
						v.PeriodTimeout = 100
						return v
					}(),
				},
				PayoutTimeoutQueue: []types.QueueEntry{
					{Time: 100, Addr: validAddr},
				},
			},
		},
		{
			name: "invalid address in payout timeout queue",
			genState: types.GenesisState{
				Params: types.DefaultParams(),
				PayoutTimeoutQueue: []types.QueueEntry{
					{Time: 100, Addr: invalidAddr},
				},
			},
			expectedErr: "invalid payout timeout queue address at index 0",
		},
		{
			name: "time exceeds max int64 in payout timeout queue",
			genState: types.GenesisState{
				Params: types.DefaultParams(),
				Vaults: []types.VaultAccount{
					func() types.VaultAccount {
						v := validVault
						v.PeriodTimeout = math.MinInt64
						return v
					}(),
				},
				PayoutTimeoutQueue: []types.QueueEntry{
					{Time: uint64(math.MaxInt64) + 1, Addr: validAddr},
				},
			},
			expectedErr: "payout timeout queue entry at index 0 has time 9223372036854775808 which exceeds max int64",
		},
		{
			name: "valid fee timeout queue",
			genState: types.GenesisState{
				Params: types.DefaultParams(),
				Vaults: []types.VaultAccount{
					func() types.VaultAccount {
						v := validVault
						v.FeePeriodTimeout = 100
						return v
					}(),
				},
				FeeTimeoutQueue: []types.QueueEntry{
					{Time: 100, Addr: validAddr},
				},
			},
		},
		{
			name: "invalid address in fee timeout queue",
			genState: types.GenesisState{
				Params: types.DefaultParams(),
				FeeTimeoutQueue: []types.QueueEntry{
					{Time: 100, Addr: invalidAddr},
				},
			},
			expectedErr: "invalid fee timeout queue address at index 0",
		},
		{
			name: "time exceeds max int64 in fee timeout queue",
			genState: types.GenesisState{
				Params: types.DefaultParams(),
				Vaults: []types.VaultAccount{
					func() types.VaultAccount {
						v := validVault
						v.FeePeriodTimeout = math.MinInt64
						return v
					}(),
				},
				FeeTimeoutQueue: []types.QueueEntry{
					{Time: uint64(math.MaxInt64) + 1, Addr: validAddr},
				},
			},
			expectedErr: "fee timeout queue entry at index 0 has time 9223372036854775808 which exceeds max int64",
		},
		{
			name: "invalid vault address in pending swap out queue",
			genState: types.GenesisState{
				Params: types.DefaultParams(),
				PendingSwapOutQueue: types.PendingSwapOutQueue{
					Entries: []types.PendingSwapOutQueueEntry{
						{
							SwapOut: types.PendingSwapOut{
								VaultAddress: "invalid-address",
							},
						},
					},
				},
			},
			expectedErr: "invalid vault address in pending swap out queue at index 0",
		},
		{
			name: "vault address not an imported vault in pending swap out queue",
			genState: types.GenesisState{
				Params: types.DefaultParams(),
				PendingSwapOutQueue: types.PendingSwapOutQueue{
					Entries: []types.PendingSwapOutQueueEntry{
						{
							SwapOut: types.PendingSwapOut{
								VaultAddress: validAddr,
							},
						},
					},
				},
			},
			expectedErr: "pending swap out queue vault address at index 0 is not an imported vault",
		},
		{
			name: "invalid tech fee address in params",
			genState: types.GenesisState{
				Params: types.Params{
					TechFeeAddress: "invalid",
				},
			},
			expectedErr: "invalid params: invalid TechFeeAddress",
		},
		{
			name: "invalid default aum fee bips in params",
			genState: types.GenesisState{
				Params: types.Params{
					TechFeeAddress:    validAddr,
					DefaultAumFeeBips: 10_001,
				},
			},
			expectedErr: "invalid params: invalid DefaultAumFeeBips",
		},
		{
			name: "valid nav entry for an imported vault",
			genState: types.GenesisState{
				Params: types.DefaultParams(),
				Vaults: []types.VaultAccount{validVault},
				Navs: []types.VaultNAVEntry{
					{
						VaultAddress: validAddr,
						Nav: types.VaultNAV{
							Denom:  "rwa",
							Price:  sdk.NewInt64Coin("under", 100),
							Volume: sdkmath.NewInt(1),
						},
					},
				},
			},
		},
		{
			name: "nav entry with invalid vault address",
			genState: types.GenesisState{
				Params: types.DefaultParams(),
				Vaults: []types.VaultAccount{validVault},
				Navs: []types.VaultNAVEntry{
					{
						VaultAddress: "not-a-bech32",
						Nav: types.VaultNAV{
							Denom:  "rwa",
							Price:  sdk.NewInt64Coin("under", 100),
							Volume: sdkmath.NewInt(1),
						},
					},
				},
			},
			expectedErr: "invalid nav vault address at index 0",
		},
		{
			name: "nav entry references a vault not in the imported set",
			genState: types.GenesisState{
				Params: types.DefaultParams(),
				Navs: []types.VaultNAVEntry{
					{
						VaultAddress: validAddr,
						Nav: types.VaultNAV{
							Denom:  "rwa",
							Price:  sdk.NewInt64Coin("under", 100),
							Volume: sdkmath.NewInt(1),
						},
					},
				},
			},
			expectedErr: "nav entry at index 0 is not for an imported vault",
		},
		{
			name: "nav entry has invalid denom",
			genState: types.GenesisState{
				Params: types.DefaultParams(),
				Vaults: []types.VaultAccount{validVault},
				Navs: []types.VaultNAVEntry{
					{
						VaultAddress: validAddr,
						Nav: types.VaultNAV{
							Denom:  "inv@lid!",
							Price:  sdk.NewInt64Coin("under", 100),
							Volume: sdkmath.NewInt(1),
						},
					},
				},
			},
			expectedErr: "invalid nav denom at index 0",
		},
		{
			name: "nav entry prices the vault share denom",
			genState: types.GenesisState{
				Params: types.DefaultParams(),
				Vaults: []types.VaultAccount{validVault},
				Navs: []types.VaultNAVEntry{
					{
						VaultAddress: validAddr,
						Nav: types.VaultNAV{
							Denom:  "share",
							Price:  sdk.NewInt64Coin("under", 100),
							Volume: sdkmath.NewInt(1),
						},
					},
				},
			},
			expectedErr: "nav entry at index 0 prices the vault share denom share",
		},
		{
			name: "nav entry has self-referential price denom",
			genState: types.GenesisState{
				Params: types.DefaultParams(),
				Vaults: []types.VaultAccount{validVault},
				Navs: []types.VaultNAVEntry{
					{
						VaultAddress: validAddr,
						Nav: types.VaultNAV{
							Denom:  "rwa",
							Price:  sdk.NewInt64Coin("rwa", 100),
							Volume: sdkmath.NewInt(1),
						},
					},
				},
			},
			expectedErr: "nav entry at index 0 has matching denom and price denom",
		},
		{
			name: "nav entry has an invalid price coin",
			genState: types.GenesisState{
				Params: types.DefaultParams(),
				Vaults: []types.VaultAccount{validVault},
				Navs: []types.VaultNAVEntry{
					{
						VaultAddress: validAddr,
						Nav: types.VaultNAV{
							Denom:  "rwa",
							Price:  sdk.Coin{Denom: "under", Amount: sdkmath.NewInt(-1)},
							Volume: sdkmath.NewInt(1),
						},
					},
				},
			},
			expectedErr: "invalid nav price at index 0",
		},
		{
			name: "nav entry for a held denom may have a zero price amount",
			genState: types.GenesisState{
				Params: types.DefaultParams(),
				Vaults: []types.VaultAccount{validVault},
				Navs: []types.VaultNAVEntry{
					{
						VaultAddress: validAddr,
						Nav: types.VaultNAV{
							Denom:  "rwa",
							Price:  sdk.NewInt64Coin("under", 0),
							Volume: sdkmath.NewInt(1),
						},
					},
				},
			},
		},
		{
			name: "nav entry for an accepted denom has zero price amount",
			genState: types.GenesisState{
				Params: types.DefaultParams(),
				Vaults: []types.VaultAccount{func() types.VaultAccount {
					v := validVault
					v.PaymentDenom = "pay"
					return v
				}()},
				Navs: []types.VaultNAVEntry{
					{
						VaultAddress: validAddr,
						Nav: types.VaultNAV{
							Denom:  "pay",
							Price:  sdk.NewInt64Coin("under", 0),
							Volume: sdkmath.NewInt(1),
						},
					},
				},
			},
			expectedErr: `nav price at index 0 for accepted denom "pay" must be positive`,
		},
		{
			name: "nav entry has zero volume",
			genState: types.GenesisState{
				Params: types.DefaultParams(),
				Vaults: []types.VaultAccount{validVault},
				Navs: []types.VaultNAVEntry{
					{
						VaultAddress: validAddr,
						Nav: types.VaultNAV{
							Denom:  "rwa",
							Price:  sdk.NewInt64Coin("under", 100),
							Volume: sdkmath.ZeroInt(),
						},
					},
				},
			},
			expectedErr: "nav volume at index 0 must be positive",
		},
		{
			name: "nav entry has negative volume",
			genState: types.GenesisState{
				Params: types.DefaultParams(),
				Vaults: []types.VaultAccount{validVault},
				Navs: []types.VaultNAVEntry{
					{
						VaultAddress: validAddr,
						Nav: types.VaultNAV{
							Denom:  "rwa",
							Price:  sdk.NewInt64Coin("under", 100),
							Volume: sdkmath.NewInt(-1),
						},
					},
				},
			},
			expectedErr: "nav volume at index 0 must be positive",
		},
		{
			name: "nav price denom is not an accepted vault denom",
			genState: types.GenesisState{
				Params: types.DefaultParams(),
				Vaults: []types.VaultAccount{validVault},
				Navs: []types.VaultNAVEntry{
					{
						VaultAddress: validAddr,
						Nav: types.VaultNAV{
							Denom:  "rwa",
							Price:  sdk.NewInt64Coin("wrongdenom", 100),
							Volume: sdkmath.NewInt(1),
						},
					},
				},
			},
			expectedErr: `nav price denom at index 0 "wrongdenom" is not an accepted denom`,
		},
		{
			name: "duplicate nav entry for the same vault and denom",
			genState: types.GenesisState{
				Params: types.DefaultParams(),
				Vaults: []types.VaultAccount{validVault},
				Navs: []types.VaultNAVEntry{
					{
						VaultAddress: validAddr,
						Nav: types.VaultNAV{
							Denom:  "rwa",
							Price:  sdk.NewInt64Coin("under", 100),
							Volume: sdkmath.NewInt(1),
						},
					},
					{
						VaultAddress: validAddr,
						Nav: types.VaultNAV{
							Denom:  "rwa",
							Price:  sdk.NewInt64Coin("under", 200),
							Volume: sdkmath.NewInt(2),
						},
					},
				},
			},
			expectedErr: "duplicate nav entry for vault",
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
