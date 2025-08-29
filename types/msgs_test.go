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
			name: "success without payment denom",
			msg: types.MsgCreateVaultRequest{
				Admin:           admin,
				ShareDenom:      "vaultshare",
				UnderlyingAsset: "uusd",
			},
			expectedErr: nil,
		},
		{
			name: "success with distinct payment denom",
			msg: types.MsgCreateVaultRequest{
				Admin:           admin,
				ShareDenom:      "vaultshare",
				UnderlyingAsset: "uusd",
				PaymentDenom:    "usdc",
			},
			expectedErr: nil,
		},
		{
			name: "admin empty",
			msg: types.MsgCreateVaultRequest{
				Admin:           "",
				ShareDenom:      "vaultshare",
				UnderlyingAsset: "uusd",
			},
			expectedErr: fmt.Errorf("invalid admin address: %q", ""),
		},
		{
			name: "admin invalid",
			msg: types.MsgCreateVaultRequest{
				Admin:           "bad",
				ShareDenom:      "vaultshare",
				UnderlyingAsset: "uusd",
			},
			expectedErr: fmt.Errorf("invalid admin address: %q", "bad"),
		},
		{
			name: "share denom empty",
			msg: types.MsgCreateVaultRequest{
				Admin:           admin,
				ShareDenom:      "",
				UnderlyingAsset: "uusd",
			},
			expectedErr: fmt.Errorf("invalid share denom: %q", ""),
		},
		{
			name: "share denom invalid",
			msg: types.MsgCreateVaultRequest{
				Admin:           admin,
				ShareDenom:      "inv@lid$",
				UnderlyingAsset: "uusd",
			},
			expectedErr: fmt.Errorf("invalid share denom: %q", "inv@lid$"),
		},
		{
			name: "underlying empty",
			msg: types.MsgCreateVaultRequest{
				Admin:           admin,
				ShareDenom:      "vaultshare",
				UnderlyingAsset: "",
			},
			expectedErr: fmt.Errorf("invalid underlying asset: %q", ""),
		},
		{
			name: "underlying invalid",
			msg: types.MsgCreateVaultRequest{
				Admin:           admin,
				ShareDenom:      "vaultshare",
				UnderlyingAsset: "inv@lid$",
			},
			expectedErr: fmt.Errorf("invalid underlying asset: %q: %w", "inv@lid$", fmt.Errorf("invalid denom: %s", "inv@lid$")),
		},
		{
			name: "payment denom invalid",
			msg: types.MsgCreateVaultRequest{
				Admin:           admin,
				ShareDenom:      "vaultshare",
				UnderlyingAsset: "uusd",
				PaymentDenom:    "inv@lid$",
			},
			expectedErr: fmt.Errorf("invalid payment denom: %q: %w", "inv@lid$", fmt.Errorf("invalid denom: %s", "inv@lid$")),
		},
		{
			name: "payment denom equals underlying (not allowed)",
			msg: types.MsgCreateVaultRequest{
				Admin:           admin,
				ShareDenom:      "vaultshare",
				UnderlyingAsset: "uusd",
				PaymentDenom:    "uusd",
			},
			expectedErr: fmt.Errorf("payment (%q) denom cannot equal underlying asset denom (%q)", "uusd", "uusd"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if tc.expectedErr != nil {
				assert.Error(t, err, "expected error for case %q", tc.name)
				assert.Contains(t, err.Error(), tc.expectedErr.Error(), "error should contain expected substring for case %q", tc.name)
			} else {
				assert.NoError(t, err, "expected no error for case %q", tc.name)
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
		expectedErr error
	}{
		{
			name: "valid",
			msg: types.MsgSwapInRequest{
				Owner:        owner,
				VaultAddress: vault,
				Assets:       sdk.NewInt64Coin("uusd", 100),
			},
			expectedErr: nil,
		},
		{
			name: "invalid vault address",
			msg: types.MsgSwapInRequest{
				Owner:        owner,
				VaultAddress: "bad",
				Assets:       sdk.NewInt64Coin("uusd", 100),
			},
			expectedErr: fmt.Errorf("invalid vault address: %q", "bad"),
		},
		{
			name: "invalid owner address",
			msg: types.MsgSwapInRequest{
				Owner:        "bad",
				VaultAddress: vault,
				Assets:       sdk.NewInt64Coin("uusd", 100),
			},
			expectedErr: fmt.Errorf("invalid owner address: %q", "bad"),
		},
		{
			name: "invalid asset denom",
			msg: types.MsgSwapInRequest{
				Owner:        owner,
				VaultAddress: vault,
				Assets:       sdk.Coin{Denom: "inv@lid$", Amount: sdkmath.NewInt(100)},
			},
			expectedErr: fmt.Errorf("invalid assets coin %v: %w", sdk.Coin{Denom: "inv@lid$", Amount: sdkmath.NewInt(100)}, fmt.Errorf("invalid denom: %s", "inv@lid$")),
		},
		{
			name: "zero amount",
			msg: types.MsgSwapInRequest{
				Owner:        owner,
				VaultAddress: vault,
				Assets:       sdk.NewInt64Coin("uusd", 0),
			},
			expectedErr: fmt.Errorf("invalid amount: assets %s must be greater than zero", "uusd"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if tc.expectedErr != nil {
				assert.Error(t, err, "expected error for case %q", tc.name)
				assert.Contains(t, err.Error(), tc.expectedErr.Error(), "error should contain expected substring for case %q", tc.name)
			} else {
				assert.NoError(t, err, "expected no error for case %q", tc.name)
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
		expectedErr error
	}{
		{
			name: "valid",
			msg: types.MsgSwapOutRequest{
				Owner:        owner,
				VaultAddress: vault,
				Assets:       sdk.NewInt64Coin("uusd", 100),
			},
			expectedErr: nil,
		},
		{
			name: "invalid vault address",
			msg: types.MsgSwapOutRequest{
				Owner:        owner,
				VaultAddress: "bad",
				Assets:       sdk.NewInt64Coin("uusd", 100),
			},
			expectedErr: fmt.Errorf("invalid vault address: %q", "bad"),
		},
		{
			name: "invalid owner address",
			msg: types.MsgSwapOutRequest{
				Owner:        "bad",
				VaultAddress: vault,
				Assets:       sdk.NewInt64Coin("uusd", 100),
			},
			expectedErr: fmt.Errorf("invalid owner address: %q", "bad"),
		},
		{
			name: "invalid asset denom",
			msg: types.MsgSwapOutRequest{
				Owner:        owner,
				VaultAddress: vault,
				Assets:       sdk.Coin{Denom: "inv@lid$", Amount: sdkmath.NewInt(100)},
			},
			expectedErr: fmt.Errorf("invalid assets coin %v: %w", sdk.Coin{Denom: "inv@lid$", Amount: sdkmath.NewInt(100)}, fmt.Errorf("invalid denom: %s", "inv@lid$")),
		},
		{
			name: "zero amount",
			msg: types.MsgSwapOutRequest{
				Owner:        owner,
				VaultAddress: vault,
				Assets:       sdk.NewInt64Coin("uusd", 0),
			},
			expectedErr: fmt.Errorf("invalid amount: assets %s must be greater than zero", "uusd"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if tc.expectedErr != nil {
				assert.Error(t, err, "expected error for case %q", tc.name)
				assert.Contains(t, err.Error(), tc.expectedErr.Error(), "error should contain expected substring for case %q", tc.name)
			} else {
				assert.NoError(t, err, "expected no error for case %q", tc.name)
			}
		})
	}
}

func TestMsgUpdateMinInterestRateRequest_ValidateBasic(t *testing.T) {
	addr := utils.TestAddress().Bech32

	tests := []struct {
		name        string
		msg         types.MsgUpdateMinInterestRateRequest
		expectedErr error
	}{
		{
			name: "valid with negative min rate",
			msg: types.MsgUpdateMinInterestRateRequest{
				Admin:        addr,
				VaultAddress: addr,
				MinRate:      "-0.75",
			},
			expectedErr: nil,
		},
		{
			name: "empty min rate clears",
			msg: types.MsgUpdateMinInterestRateRequest{
				Admin:        addr,
				VaultAddress: addr,
				MinRate:      "",
			},
			expectedErr: nil,
		},
		{
			name: "invalid admin",
			msg: types.MsgUpdateMinInterestRateRequest{
				Admin:        "bad",
				VaultAddress: addr,
				MinRate:      "0.10",
			},
			expectedErr: fmt.Errorf("invalid admin address: %q", "bad"),
		},
		{
			name: "invalid vault",
			msg: types.MsgUpdateMinInterestRateRequest{
				Admin:        addr,
				VaultAddress: "bad",
				MinRate:      "0.10",
			},
			expectedErr: fmt.Errorf("invalid vault address: %q", "bad"),
		},
		{
			name: "invalid min rate",
			msg: types.MsgUpdateMinInterestRateRequest{
				Admin:        addr,
				VaultAddress: addr,
				MinRate:      "abc",
			},
			expectedErr: fmt.Errorf("invalid min rate: %q", "abc"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if tc.expectedErr != nil {
				assert.Error(t, err, "expected error for case %q", tc.name)
				assert.Contains(t, err.Error(), tc.expectedErr.Error(), "error should contain expected substring for case %q", tc.name)
			} else {
				assert.NoError(t, err, "expected no error for case %q", tc.name)
			}
		})
	}
}

func TestMsgUpdateMaxInterestRateRequest_ValidateBasic(t *testing.T) {
	addr := utils.TestAddress().Bech32

	tests := []struct {
		name        string
		msg         types.MsgUpdateMaxInterestRateRequest
		expectedErr error
	}{
		{
			name: "valid with positive max rate",
			msg: types.MsgUpdateMaxInterestRateRequest{
				Admin:        addr,
				VaultAddress: addr,
				MaxRate:      "3.25",
			},
			expectedErr: nil,
		},
		{
			name: "empty max rate clears",
			msg: types.MsgUpdateMaxInterestRateRequest{
				Admin:        addr,
				VaultAddress: addr,
				MaxRate:      "",
			},
			expectedErr: nil,
		},
		{
			name: "invalid admin",
			msg: types.MsgUpdateMaxInterestRateRequest{
				Admin:        "bad",
				VaultAddress: addr,
				MaxRate:      "2.0",
			},
			expectedErr: fmt.Errorf("invalid admin address: %q", "bad"),
		},
		{
			name: "invalid vault",
			msg: types.MsgUpdateMaxInterestRateRequest{
				Admin:        addr,
				VaultAddress: "bad",
				MaxRate:      "2.0",
			},
			expectedErr: fmt.Errorf("invalid vault address: %q", "bad"),
		},
		{
			name: "invalid max rate",
			msg: types.MsgUpdateMaxInterestRateRequest{
				Admin:        addr,
				VaultAddress: addr,
				MaxRate:      "notanumber",
			},
			expectedErr: fmt.Errorf("invalid max rate: %q", "notanumber"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if tc.expectedErr != nil {
				assert.Error(t, err, "expected error for case %q", tc.name)
				assert.Contains(t, err.Error(), tc.expectedErr.Error(), "error should contain expected substring for case %q", tc.name)
			} else {
				assert.NoError(t, err, "expected no error for case %q", tc.name)
			}
		})
	}
}

func TestMsgUpdateInterestRateRequest_ValidateBasic(t *testing.T) {
	addr := utils.TestAddress().Bech32

	tests := []struct {
		name        string
		msg         types.MsgUpdateInterestRateRequest
		expectedErr error
	}{
		{
			name: "valid",
			msg: types.MsgUpdateInterestRateRequest{
				Admin:        addr,
				VaultAddress: addr,
				NewRate:      "1.5",
			},
			expectedErr: nil,
		},
		{
			name: "invalid admin",
			msg: types.MsgUpdateInterestRateRequest{
				Admin:        "bad",
				VaultAddress: addr,
				NewRate:      "1.5",
			},
			expectedErr: fmt.Errorf("invalid admin address: %q", "bad"),
		},
		{
			name: "invalid vault address",
			msg: types.MsgUpdateInterestRateRequest{
				Admin:        addr,
				VaultAddress: "bad",
				NewRate:      "1.5",
			},
			expectedErr: fmt.Errorf("invalid vault address: %q", "bad"),
		},
		{
			name: "invalid new rate",
			msg: types.MsgUpdateInterestRateRequest{
				Admin:        addr,
				VaultAddress: addr,
				NewRate:      "bad",
			},
			expectedErr: fmt.Errorf("invalid interest rate: %q", "bad"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if tc.expectedErr != nil {
				assert.Error(t, err, "expected error for case %q", tc.name)
				assert.Contains(t, err.Error(), tc.expectedErr.Error(), "error should contain expected substring for case %q", tc.name)
			} else {
				assert.NoError(t, err, "expected no error for case %q", tc.name)
			}
		})
	}
}

func TestMsgToggleSwapInRequest_ValidateBasic(t *testing.T) {
	addr := utils.TestAddress().Bech32

	tests := []struct {
		name        string
		msg         types.MsgToggleSwapInRequest
		expectedErr error
	}{
		{
			name: "valid",
			msg: types.MsgToggleSwapInRequest{
				Admin:        addr,
				VaultAddress: addr,
				Enabled:      true,
			},
			expectedErr: nil,
		},
		{
			name: "invalid admin",
			msg: types.MsgToggleSwapInRequest{
				Admin:        "bad",
				VaultAddress: addr,
				Enabled:      false,
			},
			expectedErr: fmt.Errorf("invalid admin address: %q", "bad"),
		},
		{
			name: "invalid vault address",
			msg: types.MsgToggleSwapInRequest{
				Admin:        addr,
				VaultAddress: "bad",
				Enabled:      true,
			},
			expectedErr: fmt.Errorf("invalid vault address: %q", "bad"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if tc.expectedErr != nil {
				assert.Error(t, err, "expected error for case %q", tc.name)
				assert.Contains(t, err.Error(), tc.expectedErr.Error(), "error should contain expected substring for case %q", tc.name)
			} else {
				assert.NoError(t, err, "expected no error for case %q", tc.name)
			}
		})
	}
}

func TestMsgToggleSwapOutRequest_ValidateBasic(t *testing.T) {
	addr := utils.TestAddress().Bech32

	tests := []struct {
		name        string
		msg         types.MsgToggleSwapOutRequest
		expectedErr error
	}{
		{
			name: "valid",
			msg: types.MsgToggleSwapOutRequest{
				Admin:        addr,
				VaultAddress: addr,
				Enabled:      true,
			},
			expectedErr: nil,
		},
		{
			name: "invalid admin",
			msg: types.MsgToggleSwapOutRequest{
				Admin:        "bad",
				VaultAddress: addr,
				Enabled:      false,
			},
			expectedErr: fmt.Errorf("invalid admin address: %q", "bad"),
		},
		{
			name: "invalid vault address",
			msg: types.MsgToggleSwapOutRequest{
				Admin:        addr,
				VaultAddress: "bad",
				Enabled:      true,
			},
			expectedErr: fmt.Errorf("invalid vault address: %q", "bad"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if tc.expectedErr != nil {
				assert.Error(t, err, "expected error for case %q", tc.name)
				assert.Contains(t, err.Error(), tc.expectedErr.Error(), "error should contain expected substring for case %q", tc.name)
			} else {
				assert.NoError(t, err, "expected no error for case %q", tc.name)
			}
		})
	}
}

func TestMsgDepositInterestFundsRequest_ValidateBasic(t *testing.T) {
	addr := utils.TestAddress().Bech32

	tests := []struct {
		name        string
		msg         types.MsgDepositInterestFundsRequest
		expectedErr error
	}{
		{
			name: "valid",
			msg: types.MsgDepositInterestFundsRequest{
				Admin:        addr,
				VaultAddress: addr,
				Amount:       sdk.NewInt64Coin("uusd", 1000),
			},
			expectedErr: nil,
		},
		{
			name: "zero amount",
			msg: types.MsgDepositInterestFundsRequest{
				Admin:        addr,
				VaultAddress: addr,
				Amount:       sdk.NewInt64Coin("uusd", 0),
			},
			expectedErr: fmt.Errorf("deposit amount must be greater than zero"),
		},
		{
			name: "invalid admin",
			msg: types.MsgDepositInterestFundsRequest{
				Admin:        "bad",
				VaultAddress: addr,
				Amount:       sdk.NewInt64Coin("uusd", 1000),
			},
			expectedErr: fmt.Errorf("invalid admin address: %q", "bad"),
		},
		{
			name: "invalid vault",
			msg: types.MsgDepositInterestFundsRequest{
				Admin:        addr,
				VaultAddress: "bad",
				Amount:       sdk.NewInt64Coin("uusd", 1000),
			},
			expectedErr: fmt.Errorf("invalid vault address: %q", "bad"),
		},
		{
			name: "invalid denom",
			msg: types.MsgDepositInterestFundsRequest{
				Admin:        addr,
				VaultAddress: addr,
				Amount:       sdk.Coin{Denom: "inv@lid$", Amount: sdkmath.NewInt(1000)},
			},
			expectedErr: fmt.Errorf("invalid deposit amount: %w", fmt.Errorf("invalid denom: %s", "inv@lid$")),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if tc.expectedErr != nil {
				assert.Error(t, err, "expected error for case %q", tc.name)
				assert.Contains(t, err.Error(), tc.expectedErr.Error(), "error should contain expected substring for case %q", tc.name)
			} else {
				assert.NoError(t, err, "expected no error for case %q", tc.name)
			}
		})
	}
}

func TestMsgWithdrawInterestFundsRequest_ValidateBasic(t *testing.T) {
	addr := utils.TestAddress().Bech32

	tests := []struct {
		name        string
		msg         types.MsgWithdrawInterestFundsRequest
		expectedErr error
	}{
		{
			name: "valid",
			msg: types.MsgWithdrawInterestFundsRequest{
				Admin:        addr,
				VaultAddress: addr,
				Amount:       sdk.NewInt64Coin("uusd", 1000),
			},
			expectedErr: nil,
		},
		{
			name: "zero amount",
			msg: types.MsgWithdrawInterestFundsRequest{
				Admin:        addr,
				VaultAddress: addr,
				Amount:       sdk.NewInt64Coin("uusd", 0),
			},
			expectedErr: fmt.Errorf("withdrawal amount must be greater than zero"),
		},
		{
			name: "invalid interest admin",
			msg: types.MsgWithdrawInterestFundsRequest{
				Admin:        "bad",
				VaultAddress: addr,
				Amount:       sdk.NewInt64Coin("uusd", 1000),
			},
			expectedErr: fmt.Errorf("invalid interest admin address: %q", "bad"),
		},
		{
			name: "invalid vault",
			msg: types.MsgWithdrawInterestFundsRequest{
				Admin:        addr,
				VaultAddress: "bad",
				Amount:       sdk.NewInt64Coin("uusd", 1000),
			},
			expectedErr: fmt.Errorf("invalid vault address: %q", "bad"),
		},
		{
			name: "invalid denom",
			msg: types.MsgWithdrawInterestFundsRequest{
				Admin:        addr,
				VaultAddress: addr,
				Amount:       sdk.Coin{Denom: "inv@lid$", Amount: sdkmath.NewInt(1000)},
			},
			expectedErr: fmt.Errorf("invalid withdrawal amount: %w", fmt.Errorf("invalid denom: %s", "inv@lid$")),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if tc.expectedErr != nil {
				assert.Error(t, err, "expected error for case %q", tc.name)
				assert.Contains(t, err.Error(), tc.expectedErr.Error(), "error should contain expected substring for case %q", tc.name)
			} else {
				assert.NoError(t, err, "expected no error for case %q", tc.name)
			}
		})
	}
}

func TestMsgDepositPrincipalFundsRequest_ValidateBasic(t *testing.T) {
	addr := utils.TestAddress().Bech32

	tests := []struct {
		name        string
		msg         types.MsgDepositPrincipalFundsRequest
		expectedErr error
	}{
		{
			name: "valid",
			msg: types.MsgDepositPrincipalFundsRequest{
				Admin:        addr,
				VaultAddress: addr,
				Amount:       sdk.NewInt64Coin("uusd", 1000),
			},
			expectedErr: nil,
		},
		{
			name: "zero amount",
			msg: types.MsgDepositPrincipalFundsRequest{
				Admin:        addr,
				VaultAddress: addr,
				Amount:       sdk.NewInt64Coin("uusd", 0),
			},
			expectedErr: fmt.Errorf("deposit amount must be greater than zero"),
		},
		{
			name: "invalid admin",
			msg: types.MsgDepositPrincipalFundsRequest{
				Admin:        "bad",
				VaultAddress: addr,
				Amount:       sdk.NewInt64Coin("uusd", 1000),
			},
			expectedErr: fmt.Errorf("invalid admin address: %q", "bad"),
		},
		{
			name: "invalid vault",
			msg: types.MsgDepositPrincipalFundsRequest{
				Admin:        addr,
				VaultAddress: "bad",
				Amount:       sdk.NewInt64Coin("uusd", 1000),
			},
			expectedErr: fmt.Errorf("invalid vault address: %q", "bad"),
		},
		{
			name: "invalid denom",
			msg: types.MsgDepositPrincipalFundsRequest{
				Admin:        addr,
				VaultAddress: addr,
				Amount:       sdk.Coin{Denom: "inv@lid$", Amount: sdkmath.NewInt(1000)},
			},
			expectedErr: fmt.Errorf("invalid deposit amount: %w", fmt.Errorf("invalid denom: %s", "inv@lid$")),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if tc.expectedErr != nil {
				assert.Error(t, err, "expected error for case %q", tc.name)
				assert.Contains(t, err.Error(), tc.expectedErr.Error(), "error should contain expected substring for case %q", tc.name)
			} else {
				assert.NoError(t, err, "expected no error for case %q", tc.name)
			}
		})
	}
}

func TestMsgWithdrawPrincipalFundsRequest_ValidateBasic(t *testing.T) {
	addr := utils.TestAddress().Bech32

	tests := []struct {
		name        string
		msg         types.MsgWithdrawPrincipalFundsRequest
		expectedErr error
	}{
		{
			name: "valid",
			msg: types.MsgWithdrawPrincipalFundsRequest{
				Admin:        addr,
				VaultAddress: addr,
				Amount:       sdk.NewInt64Coin("uusd", 1000),
			},
			expectedErr: nil,
		},
		{
			name: "zero amount",
			msg: types.MsgWithdrawPrincipalFundsRequest{
				Admin:        addr,
				VaultAddress: addr,
				Amount:       sdk.NewInt64Coin("uusd", 0),
			},
			expectedErr: fmt.Errorf("withdrawal amount must be greater than zero"),
		},
		{
			name: "invalid admin",
			msg: types.MsgWithdrawPrincipalFundsRequest{
				Admin:        "bad",
				VaultAddress: addr,
				Amount:       sdk.NewInt64Coin("uusd", 1000),
			},
			expectedErr: fmt.Errorf("invalid admin address: %q", "bad"),
		},
		{
			name: "invalid vault",
			msg: types.MsgWithdrawPrincipalFundsRequest{
				Admin:        addr,
				VaultAddress: "bad",
				Amount:       sdk.NewInt64Coin("uusd", 1000),
			},
			expectedErr: fmt.Errorf("invalid vault address: %q", "bad"),
		},
		{
			name: "invalid denom",
			msg: types.MsgWithdrawPrincipalFundsRequest{
				Admin:        addr,
				VaultAddress: addr,
				Amount:       sdk.Coin{Denom: "inv@lid$", Amount: sdkmath.NewInt(1000)},
			},
			expectedErr: fmt.Errorf("invalid withdrawal amount: %w", fmt.Errorf("invalid denom: %s", "inv@lid$")),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if tc.expectedErr != nil {
				assert.Error(t, err, "expected error for case %q", tc.name)
				assert.Contains(t, err.Error(), tc.expectedErr.Error(), "error should contain expected substring for case %q", tc.name)
			} else {
				assert.NoError(t, err, "expected no error for case %q", tc.name)
			}
		})
	}
}
