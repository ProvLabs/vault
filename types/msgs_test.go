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
		{
			name: "share denom equals underlying (not allowed)",
			msg: types.MsgCreateVaultRequest{
				Admin:           admin,
				ShareDenom:      "uusd",
				UnderlyingAsset: "uusd",
				PaymentDenom:    "usdc",
			},
			expectedErr: fmt.Errorf("share denom (%q) cannot equal underlying asset denom (%q)", "uusd", "uusd"),
		},
		{
			name: "share denom equals payment denom (not allowed)",
			msg: types.MsgCreateVaultRequest{
				Admin:           admin,
				ShareDenom:      "usdc",
				UnderlyingAsset: "uusd",
				PaymentDenom:    "usdc",
			},
			expectedErr: fmt.Errorf("share denom (%q) cannot equal payment denom (%q)", "usdc", "usdc"),
		},
		{
			name: "swap out delay over two years (not allowed)",
			msg: types.MsgCreateVaultRequest{
				Admin:                  admin,
				ShareDenom:             "vaultshare",
				UnderlyingAsset:        "uusd",
				WithdrawalDelaySeconds: types.MaxWithdrawalDelay + 1,
			},
			expectedErr: fmt.Errorf("withdrawal delay cannot exceed %d seconds", types.MaxWithdrawalDelay),
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
			name: "valid with redeem denom",
			msg: types.MsgSwapOutRequest{
				Owner:        owner,
				VaultAddress: vault,
				Assets:       sdk.NewInt64Coin("uusd", 100),
				RedeemDenom:  "usdc",
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
		{
			name: "invalid redeem denom",
			msg: types.MsgSwapOutRequest{
				Owner:        owner,
				VaultAddress: vault,
				Assets:       sdk.NewInt64Coin("uusd", 100),
				RedeemDenom:  "inv@lid$",
			},
			expectedErr: fmt.Errorf("invalid redeem_denom: %w", fmt.Errorf("invalid denom: %s", "inv@lid$")),
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
				Authority:    addr,
				VaultAddress: addr,
				NewRate:      "1.5",
			},
			expectedErr: nil,
		},
		{
			name: "invalid admin",
			msg: types.MsgUpdateInterestRateRequest{
				Authority:    "bad",
				VaultAddress: addr,
				NewRate:      "1.5",
			},
			expectedErr: fmt.Errorf("invalid authority address: %q", "bad"),
		},
		{
			name: "invalid vault address",
			msg: types.MsgUpdateInterestRateRequest{
				Authority:    addr,
				VaultAddress: "bad",
				NewRate:      "1.5",
			},
			expectedErr: fmt.Errorf("invalid vault address: %q", "bad"),
		},
		{
			name: "invalid new rate",
			msg: types.MsgUpdateInterestRateRequest{
				Authority:    addr,
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
				Authority:    addr,
				VaultAddress: addr,
				Amount:       sdk.NewInt64Coin("uusd", 1000),
			},
			expectedErr: nil,
		},
		{
			name: "zero amount",
			msg: types.MsgDepositInterestFundsRequest{
				Authority:    addr,
				VaultAddress: addr,
				Amount:       sdk.NewInt64Coin("uusd", 0),
			},
			expectedErr: fmt.Errorf("deposit amount must be greater than zero"),
		},
		{
			name: "invalid authority",
			msg: types.MsgDepositInterestFundsRequest{
				Authority:    "bad",
				VaultAddress: addr,
				Amount:       sdk.NewInt64Coin("uusd", 1000),
			},
			expectedErr: fmt.Errorf("invalid authority address: %q", "bad"),
		},
		{
			name: "invalid vault",
			msg: types.MsgDepositInterestFundsRequest{
				Authority:    addr,
				VaultAddress: "bad",
				Amount:       sdk.NewInt64Coin("uusd", 1000),
			},
			expectedErr: fmt.Errorf("invalid vault address: %q", "bad"),
		},
		{
			name: "invalid denom",
			msg: types.MsgDepositInterestFundsRequest{
				Authority:    addr,
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
				Authority:    addr,
				VaultAddress: addr,
				Amount:       sdk.NewInt64Coin("uusd", 1000),
			},
			expectedErr: nil,
		},
		{
			name: "zero amount",
			msg: types.MsgWithdrawInterestFundsRequest{
				Authority:    addr,
				VaultAddress: addr,
				Amount:       sdk.NewInt64Coin("uusd", 0),
			},
			expectedErr: fmt.Errorf("withdrawal amount must be greater than zero"),
		},
		{
			name: "invalid authority",
			msg: types.MsgWithdrawInterestFundsRequest{
				Authority:    "bad",
				VaultAddress: addr,
				Amount:       sdk.NewInt64Coin("uusd", 1000),
			},
			expectedErr: fmt.Errorf("invalid authority address: %q", "bad"),
		},
		{
			name: "invalid vault",
			msg: types.MsgWithdrawInterestFundsRequest{
				Authority:    addr,
				VaultAddress: "bad",
				Amount:       sdk.NewInt64Coin("uusd", 1000),
			},
			expectedErr: fmt.Errorf("invalid vault address: %q", "bad"),
		},
		{
			name: "invalid denom",
			msg: types.MsgWithdrawInterestFundsRequest{
				Authority:    addr,
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
				Authority:    addr,
				VaultAddress: addr,
				Amount:       sdk.NewInt64Coin("uusd", 1000),
			},
			expectedErr: nil,
		},
		{
			name: "zero amount",
			msg: types.MsgDepositPrincipalFundsRequest{
				Authority:    addr,
				VaultAddress: addr,
				Amount:       sdk.NewInt64Coin("uusd", 0),
			},
			expectedErr: fmt.Errorf("deposit amount must be greater than zero"),
		},
		{
			name: "invalid authority",
			msg: types.MsgDepositPrincipalFundsRequest{
				Authority:    "bad",
				VaultAddress: addr,
				Amount:       sdk.NewInt64Coin("uusd", 1000),
			},
			expectedErr: fmt.Errorf("invalid authority address: %q", "bad"),
		},
		{
			name: "invalid vault",
			msg: types.MsgDepositPrincipalFundsRequest{
				Authority:    addr,
				VaultAddress: "bad",
				Amount:       sdk.NewInt64Coin("uusd", 1000),
			},
			expectedErr: fmt.Errorf("invalid vault address: %q", "bad"),
		},
		{
			name: "invalid denom",
			msg: types.MsgDepositPrincipalFundsRequest{
				Authority:    addr,
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
				Authority:    addr,
				VaultAddress: addr,
				Amount:       sdk.NewInt64Coin("uusd", 1000),
			},
			expectedErr: nil,
		},
		{
			name: "zero amount",
			msg: types.MsgWithdrawPrincipalFundsRequest{
				Authority:    addr,
				VaultAddress: addr,
				Amount:       sdk.NewInt64Coin("uusd", 0),
			},
			expectedErr: fmt.Errorf("withdrawal amount must be greater than zero"),
		},
		{
			name: "invalid authority",
			msg: types.MsgWithdrawPrincipalFundsRequest{
				Authority:    "bad",
				VaultAddress: addr,
				Amount:       sdk.NewInt64Coin("uusd", 1000),
			},
			expectedErr: fmt.Errorf("invalid authority address: %q", "bad"),
		},
		{
			name: "invalid vault",
			msg: types.MsgWithdrawPrincipalFundsRequest{
				Authority:    addr,
				VaultAddress: "bad",
				Amount:       sdk.NewInt64Coin("uusd", 1000),
			},
			expectedErr: fmt.Errorf("invalid vault address: %q", "bad"),
		},
		{
			name: "invalid denom",
			msg: types.MsgWithdrawPrincipalFundsRequest{
				Authority:    addr,
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

func TestMsgExpeditePendingSwapOutRequest_ValidateBasic(t *testing.T) {
	addr := utils.TestAddress().Bech32

	tests := []struct {
		name        string
		msg         types.MsgExpeditePendingSwapOutRequest
		expectedErr error
	}{
		{
			name: "valid",
			msg: types.MsgExpeditePendingSwapOutRequest{
				Authority: addr,
				RequestId: 1,
			},
			expectedErr: nil,
		},
		{
			name: "invalid admin",
			msg: types.MsgExpeditePendingSwapOutRequest{
				Authority: "bad",
				RequestId: 1,
			},
			expectedErr: fmt.Errorf("invalid authority address: %q", "bad"),
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

func TestMsgPauseVaultRequest_ValidateBasic(t *testing.T) {
	addr := utils.TestAddress().Bech32

	tests := []struct {
		name        string
		msg         types.MsgPauseVaultRequest
		expectedErr error
	}{
		{
			name: "valid",
			msg: types.MsgPauseVaultRequest{
				Authority:    addr,
				VaultAddress: addr,
				Reason:       "rebalancing",
			},
			expectedErr: nil,
		},
		{
			name: "invalid authority",
			msg: types.MsgPauseVaultRequest{
				Authority:    "bad",
				VaultAddress: addr,
				Reason:       "rebalancing",
			},
			expectedErr: fmt.Errorf("invalid authority address: %q", "bad"),
		},
		{
			name: "invalid vault address",
			msg: types.MsgPauseVaultRequest{
				Authority:    addr,
				VaultAddress: "bad",
				Reason:       "rebalancing",
			},
			expectedErr: fmt.Errorf("invalid vault address: %q", "bad"),
		},
		{
			name: "empty reason",
			msg: types.MsgPauseVaultRequest{
				Authority:    addr,
				VaultAddress: addr,
				Reason:       "",
			},
			expectedErr: fmt.Errorf("reason cannot be empty"),
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

func TestMsgUnpauseVaultRequest_ValidateBasic(t *testing.T) {
	addr := utils.TestAddress().Bech32

	tests := []struct {
		name        string
		msg         types.MsgUnpauseVaultRequest
		expectedErr error
	}{
		{
			name: "valid",
			msg: types.MsgUnpauseVaultRequest{
				Authority:    addr,
				VaultAddress: addr,
			},
			expectedErr: nil,
		},
		{
			name: "invalid authority",
			msg: types.MsgUnpauseVaultRequest{
				Authority:    "bad",
				VaultAddress: addr,
			},
			expectedErr: fmt.Errorf("invalid authority address: %q", "bad"),
		},
		{
			name: "invalid vault address",
			msg: types.MsgUnpauseVaultRequest{
				Authority:    addr,
				VaultAddress: "bad",
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

func TestMsgSetBridgeAddressRequest_ValidateBasic(t *testing.T) {
	admin := utils.TestAddress().Bech32
	vault := utils.TestAddress().Bech32
	bridge := utils.TestAddress().Bech32

	tests := []struct {
		name        string
		msg         types.MsgSetBridgeAddressRequest
		expectedErr error
	}{
		{
			name: "valid",
			msg: types.MsgSetBridgeAddressRequest{
				Admin:         admin,
				VaultAddress:  vault,
				BridgeAddress: bridge,
			},
			expectedErr: nil,
		},
		{
			name: "invalid admin",
			msg: types.MsgSetBridgeAddressRequest{
				Admin:         "bad",
				VaultAddress:  vault,
				BridgeAddress: bridge,
			},
			expectedErr: fmt.Errorf("invalid admin address: %q", "bad"),
		},
		{
			name: "invalid vault",
			msg: types.MsgSetBridgeAddressRequest{
				Admin:         admin,
				VaultAddress:  "bad",
				BridgeAddress: bridge,
			},
			expectedErr: fmt.Errorf("invalid vault address: %q", "bad"),
		},
		{
			name: "invalid bridge",
			msg: types.MsgSetBridgeAddressRequest{
				Admin:         admin,
				VaultAddress:  vault,
				BridgeAddress: "bad",
			},
			expectedErr: fmt.Errorf("invalid bridge address: %q", "bad"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if tc.expectedErr != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMsgToggleBridgeRequest_ValidateBasic(t *testing.T) {
	admin := utils.TestAddress().Bech32
	vault := utils.TestAddress().Bech32

	tests := []struct {
		name        string
		msg         types.MsgToggleBridgeRequest
		expectedErr error
	}{
		{
			name: "valid",
			msg: types.MsgToggleBridgeRequest{
				Admin:        admin,
				VaultAddress: vault,
				Enabled:      true,
			},
			expectedErr: nil,
		},
		{
			name: "invalid admin",
			msg: types.MsgToggleBridgeRequest{
				Admin:        "bad",
				VaultAddress: vault,
				Enabled:      false,
			},
			expectedErr: fmt.Errorf("invalid admin address: %q", "bad"),
		},
		{
			name: "invalid vault",
			msg: types.MsgToggleBridgeRequest{
				Admin:        admin,
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
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMsgBridgeMintSharesRequest_ValidateBasic(t *testing.T) {
	bridge := utils.TestAddress().Bech32
	vault := utils.TestAddress().Bech32

	tests := []struct {
		name        string
		msg         types.MsgBridgeMintSharesRequest
		expectedErr error
	}{
		{
			name: "valid",
			msg: types.MsgBridgeMintSharesRequest{
				Bridge:       bridge,
				VaultAddress: vault,
				Shares:       sdk.NewInt64Coin("vaultshare", 100),
			},
			expectedErr: nil,
		},
		{
			name: "invalid bridge",
			msg: types.MsgBridgeMintSharesRequest{
				Bridge:       "bad",
				VaultAddress: vault,
				Shares:       sdk.NewInt64Coin("vaultshare", 100),
			},
			expectedErr: fmt.Errorf("invalid bridge address: %q", "bad"),
		},
		{
			name: "invalid vault",
			msg: types.MsgBridgeMintSharesRequest{
				Bridge:       bridge,
				VaultAddress: "bad",
				Shares:       sdk.NewInt64Coin("vaultshare", 100),
			},
			expectedErr: fmt.Errorf("invalid vault address: %q", "bad"),
		},
		{
			name: "invalid shares denom",
			msg: types.MsgBridgeMintSharesRequest{
				Bridge:       bridge,
				VaultAddress: vault,
				Shares:       sdk.Coin{Denom: "inv@lid$", Amount: sdkmath.NewInt(100)},
			},
			expectedErr: fmt.Errorf("invalid shares coin %v: %w", sdk.Coin{Denom: "inv@lid$", Amount: sdkmath.NewInt(100)}, fmt.Errorf("invalid denom: %s", "inv@lid$")),
		},
		{
			name: "zero shares amount",
			msg: types.MsgBridgeMintSharesRequest{
				Bridge:       bridge,
				VaultAddress: vault,
				Shares:       sdk.NewInt64Coin("vaultshare", 0),
			},
			expectedErr: fmt.Errorf("shares amount must be greater than zero"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if tc.expectedErr != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMsgBridgeBurnSharesRequest_ValidateBasic(t *testing.T) {
	bridge := utils.TestAddress().Bech32
	vault := utils.TestAddress().Bech32

	tests := []struct {
		name        string
		msg         types.MsgBridgeBurnSharesRequest
		expectedErr error
	}{
		{
			name: "valid",
			msg: types.MsgBridgeBurnSharesRequest{
				Bridge:       bridge,
				VaultAddress: vault,
				Shares:       sdk.NewInt64Coin("vaultshare", 100),
			},
			expectedErr: nil,
		},
		{
			name: "invalid bridge",
			msg: types.MsgBridgeBurnSharesRequest{
				Bridge:       "bad",
				VaultAddress: vault,
				Shares:       sdk.NewInt64Coin("vaultshare", 100),
			},
			expectedErr: fmt.Errorf("invalid bridge address: %q", "bad"),
		},
		{
			name: "invalid vault",
			msg: types.MsgBridgeBurnSharesRequest{
				Bridge:       bridge,
				VaultAddress: "bad",
				Shares:       sdk.NewInt64Coin("vaultshare", 100),
			},
			expectedErr: fmt.Errorf("invalid vault address: %q", "bad"),
		},
		{
			name: "invalid shares denom",
			msg: types.MsgBridgeBurnSharesRequest{
				Bridge:       bridge,
				VaultAddress: vault,
				Shares:       sdk.Coin{Denom: "inv@lid$", Amount: sdkmath.NewInt(100)},
			},
			expectedErr: fmt.Errorf("invalid shares coin %v: %w", sdk.Coin{Denom: "inv@lid$", Amount: sdkmath.NewInt(100)}, fmt.Errorf("invalid denom: %s", "inv@lid$")),
		},
		{
			name: "zero shares amount",
			msg: types.MsgBridgeBurnSharesRequest{
				Bridge:       bridge,
				VaultAddress: vault,
				Shares:       sdk.NewInt64Coin("vaultshare", 0),
			},
			expectedErr: fmt.Errorf("shares amount must be greater than zero"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if tc.expectedErr != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMsgSetAssetManagerRequest_ValidateBasic(t *testing.T) {
	admin := utils.TestAddress().Bech32
	vault := utils.TestAddress().Bech32
	manager := utils.TestAddress().Bech32

	tests := []struct {
		name        string
		msg         types.MsgSetAssetManagerRequest
		expectedErr error
	}{
		{
			name: "valid - set asset manager",
			msg: types.MsgSetAssetManagerRequest{
				Admin:        admin,
				VaultAddress: vault,
				AssetManager: manager,
			},
			expectedErr: nil,
		},
		{
			name: "valid - clear asset manager with empty string",
			msg: types.MsgSetAssetManagerRequest{
				Admin:        admin,
				VaultAddress: vault,
				AssetManager: "",
			},
			expectedErr: nil,
		},
		{
			name: "invalid admin",
			msg: types.MsgSetAssetManagerRequest{
				Admin:        "bad",
				VaultAddress: vault,
				AssetManager: manager,
			},
			expectedErr: fmt.Errorf("invalid admin address: %q", "bad"),
		},
		{
			name: "invalid vault address",
			msg: types.MsgSetAssetManagerRequest{
				Admin:        admin,
				VaultAddress: "bad",
				AssetManager: manager,
			},
			expectedErr: fmt.Errorf("invalid vault address: %q", "bad"),
		},
		{
			name: "invalid asset manager address string",
			msg: types.MsgSetAssetManagerRequest{
				Admin:        admin,
				VaultAddress: vault,
				AssetManager: "bad",
			},
			expectedErr: fmt.Errorf("invalid asset manager address: %q", "bad"),
		},
		{
			name: "valid - asset manager equals admin (allowed)",
			msg: types.MsgSetAssetManagerRequest{
				Admin:        admin,
				VaultAddress: vault,
				AssetManager: admin,
			},
			expectedErr: nil,
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
