package types_test

import (
	"fmt"
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/stretchr/testify/assert"

	"github.com/provlabs/vault/types"
	"github.com/provlabs/vault/utils"
)

func TestMsgCreateVaultRequest_ValidateBasic(t *testing.T) {
	admin := utils.TestAddress().Bech32
	tests := []struct {
		name        string
		msg         types.MsgCreateVaultRequest
		expectedErr error
	}{
		{
			name: "A successful MsgCreateVault",
			msg: types.MsgCreateVaultRequest{
				Admin:           admin,
				ShareDenom:      "uatom",
				UnderlyingAsset: "uusd",
			},
			expectedErr: nil,
		},
		{
			name: "Admin is empty",
			msg: types.MsgCreateVaultRequest{
				Admin:           "",
				ShareDenom:      "uatom",
				UnderlyingAsset: "uusd",
			},
			expectedErr: fmt.Errorf("invalid admin address: %q", ""),
		},
		{
			name: "Admin with invalid address",
			msg: types.MsgCreateVaultRequest{
				Admin:           "invalid address",
				ShareDenom:      "uatom",
				UnderlyingAsset: "uusd",
			},
			expectedErr: fmt.Errorf("invalid admin address: %q", "invalid address"),
		},
		{
			name: "ShareDenom is empty",
			msg: types.MsgCreateVaultRequest{
				Admin:           admin,
				ShareDenom:      "",
				UnderlyingAsset: "uusd",
			},
			expectedErr: fmt.Errorf("invalid share denom: %q", ""),
		},
		{
			name: "ShareDenom with invalid denom",
			msg: types.MsgCreateVaultRequest{
				Admin:           admin,
				ShareDenom:      "inv@lid$",
				UnderlyingAsset: "uusd",
			},
			expectedErr: fmt.Errorf("invalid share denom: %q", "inv@lid$"),
		},
		{
			name: "UnderlyingAsset is empty",
			msg: types.MsgCreateVaultRequest{
				Admin:           admin,
				ShareDenom:      "uatom",
				UnderlyingAsset: "",
			},
			expectedErr: fmt.Errorf("invalid underlying asset: %q", ""),
		},
		{
			name: "UnderlyingAsset with invalid denom",
			msg: types.MsgCreateVaultRequest{
				Admin:           admin,
				ShareDenom:      "uatom",
				UnderlyingAsset: "inv@lid$",
			},
			expectedErr: fmt.Errorf("invalid underlying asset: %q: %w", "inv@lid$", fmt.Errorf("invalid denom: %s", "inv@lid$")),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()

			if tc.expectedErr != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErr.Error())
			} else {
				// We expect no error
				assert.NoError(t, err)
			}
		})
	}
}

func TestMsgSwapInRequest_ValidateBasic(t *testing.T) {
	owner := utils.TestAddress().Bech32
	vault := utils.TestAddress().Bech32

	tests := []struct {
		name        string
		msg         types.MsgSwapInRequest
		expectedErr string
	}{
		{
			name: "valid request",
			msg: types.MsgSwapInRequest{
				Owner:        owner,
				VaultAddress: vault,
				Assets:       sdk.NewInt64Coin("uusd", 100),
			},
			expectedErr: "",
		},
		{
			name: "invalid vault address",
			msg: types.MsgSwapInRequest{
				Owner:        owner,
				VaultAddress: "invalid",
				Assets:       sdk.NewInt64Coin("uusd", 100),
			},
			expectedErr: "invalid vault address",
		},
		{
			name: "invalid owner address",
			msg: types.MsgSwapInRequest{
				Owner:        "invalid",
				VaultAddress: vault,
				Assets:       sdk.NewInt64Coin("uusd", 100),
			},
			expectedErr: "invalid owner address",
		},
		{
			name: "invalid denom",
			msg: types.MsgSwapInRequest{
				Owner:        owner,
				VaultAddress: vault,
				Assets:       sdk.Coin{Denom: "inv@lid$", Amount: sdkmath.NewInt(100)},
			},
			expectedErr: "invalid assets coin",
		},
		{
			name: "zero amount",
			msg: types.MsgSwapInRequest{
				Owner:        owner,
				VaultAddress: vault,
				Assets:       sdk.NewInt64Coin("uusd", 0),
			},
			expectedErr: "must be greater than zero",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()

			if tc.expectedErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMsgSwapOutRequest_ValidateBasic(t *testing.T) {
	owner := utils.TestAddress().Bech32
	vault := utils.TestAddress().Bech32

	tests := []struct {
		name        string
		msg         types.MsgSwapOutRequest
		expectedErr string
	}{
		{
			name: "valid request",
			msg: types.MsgSwapOutRequest{
				Owner:        owner,
				VaultAddress: vault,
				Assets:       sdk.NewInt64Coin("uusd", 100),
			},
			expectedErr: "",
		},
		{
			name: "invalid vault address",
			msg: types.MsgSwapOutRequest{
				Owner:        owner,
				VaultAddress: "invalid",
				Assets:       sdk.NewInt64Coin("uusd", 100),
			},
			expectedErr: "invalid vault address",
		},
		{
			name: "invalid owner address",
			msg: types.MsgSwapOutRequest{
				Owner:        "invalid",
				VaultAddress: vault,
				Assets:       sdk.NewInt64Coin("uusd", 100),
			},
			expectedErr: "invalid owner address",
		},
		{
			name: "invalid denom",
			msg: types.MsgSwapOutRequest{
				Owner:        owner,
				VaultAddress: vault,
				Assets:       sdk.Coin{Denom: "inv@lid$", Amount: sdkmath.NewInt(100)},
			},
			expectedErr: "invalid assets coin",
		},
		{
			name: "zero amount",
			msg: types.MsgSwapOutRequest{
				Owner:        owner,
				VaultAddress: vault,
				Assets:       sdk.NewInt64Coin("uusd", 0),
			},
			expectedErr: "must be greater than zero",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()

			if tc.expectedErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
