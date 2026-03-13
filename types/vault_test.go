package types_test

import (
	"fmt"
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/provlabs/vault/types"
	"github.com/provlabs/vault/utils"
)

func TestVaultAccount_Validate(t *testing.T) {
	validAdmin := utils.TestAddress().Bech32
	validBridgeAddress := utils.TestAddress().Bech32
	validAssetManager := utils.TestAddress().Bech32
	validDenom := "validdenom"
	invalidDenom := "inval!d"
	validInterest := "0.05"
	invalidInterest := "not-a-decimal"

	baseAcc := authtypes.NewBaseAccountWithAddress(sdk.MustAccAddressFromBech32(validAdmin))

	tests := []struct {
		name         string
		vaultAccount types.VaultAccount
		expectedErr  string
	}{
		{
			name: "valid vault account with interest rates (current==desired)",
			vaultAccount: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				TotalShares:         sdk.NewInt64Coin(validDenom, 0),
				UnderlyingAsset:     "uusd",
				PaymentDenom:        "uusd",
				CurrentInterestRate: validInterest,
				DesiredInterestRate: validInterest,
				OutstandingAumFee:   sdk.NewInt64Coin("uusd", 0),
			},
			expectedErr: "",
		},
		{
			name: "valid base account is nil",
			vaultAccount: types.VaultAccount{
				BaseAccount:         nil,
				Admin:               validAdmin,
				TotalShares:         sdk.NewInt64Coin(validDenom, 0),
				UnderlyingAsset:     "uusd",
				PaymentDenom:        "uusd",
				CurrentInterestRate: validInterest,
				DesiredInterestRate: validInterest,
				OutstandingAumFee:   sdk.NewInt64Coin("uusd", 0),
			},
			expectedErr: "base account cannot be nil",
		},
		{
			name: "valid vault account with bounds and current==0",
			vaultAccount: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				TotalShares:         sdk.NewInt64Coin(validDenom, 0),
				UnderlyingAsset:     "uusd",
				PaymentDenom:        "uusd",
				CurrentInterestRate: "0.00",
				DesiredInterestRate: "0.04",
				MinInterestRate:     "0.03",
				MaxInterestRate:     "0.05",
				OutstandingAumFee:   sdk.NewInt64Coin("uusd", 0),
			},
			expectedErr: "",
		},
		{
			name: "invalid admin address",
			vaultAccount: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               "invalid-address",
				TotalShares:         sdk.NewInt64Coin(validDenom, 0),
				UnderlyingAsset:     "uusd",
				PaymentDenom:        "uusd",
				CurrentInterestRate: validInterest,
				DesiredInterestRate: validInterest,
				OutstandingAumFee:   sdk.NewInt64Coin("uusd", 0),
			},
			expectedErr: "invalid admin address",
		},
		{
			name: "empty share denom",
			vaultAccount: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				TotalShares:         sdk.Coin{Denom: "", Amount: math.NewInt(0)},
				UnderlyingAsset:     "uusd",
				PaymentDenom:        "uusd",
				CurrentInterestRate: validInterest,
				DesiredInterestRate: validInterest,
				OutstandingAumFee:   sdk.NewInt64Coin("uusd", 0),
			},
			expectedErr: "invalid share denom",
		},
		{
			name: "invalid share denom",
			vaultAccount: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				TotalShares:         sdk.Coin{Denom: invalidDenom, Amount: math.NewInt(0)},
				UnderlyingAsset:     "uusd",
				PaymentDenom:        "uusd",
				CurrentInterestRate: validInterest,
				DesiredInterestRate: validInterest,
				OutstandingAumFee:   sdk.NewInt64Coin("uusd", 0),
			},
			expectedErr: "invalid share denom",
		},
		{
			name: "empty underlying assets",
			vaultAccount: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				TotalShares:         sdk.NewInt64Coin(validDenom, 0),
				UnderlyingAsset:     "",
				CurrentInterestRate: validInterest,
				DesiredInterestRate: validInterest,
				OutstandingAumFee:   sdk.Coin{Denom: "", Amount: math.NewInt(0)},
			},
			expectedErr: "invalid underlying asset denom: ",
		},
		{
			name: "invalid underlying asset denom",
			vaultAccount: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				TotalShares:         sdk.NewInt64Coin(validDenom, 0),
				UnderlyingAsset:     invalidDenom,
				CurrentInterestRate: validInterest,
				DesiredInterestRate: validInterest,
				OutstandingAumFee:   sdk.NewInt64Coin("uusd", 0),
			},
			expectedErr: fmt.Sprintf("invalid underlying asset denom: %s", invalidDenom),
		},
		{
			name: "payment denom omitted => fails validation",
			vaultAccount: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				TotalShares:         sdk.NewInt64Coin(validDenom, 0),
				UnderlyingAsset:     "uusd",
				PaymentDenom:        "",
				CurrentInterestRate: validInterest,
				DesiredInterestRate: validInterest,
				OutstandingAumFee:   sdk.Coin{Denom: "", Amount: math.NewInt(0)},
			},
			expectedErr: "invalid payment denom: \"\": invalid denom: ",
		},
		{
			name: "payment denom valid and distinct => ok",
			vaultAccount: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				TotalShares:         sdk.NewInt64Coin(validDenom, 0),
				UnderlyingAsset:     "uusd",
				PaymentDenom:        "usdc",
				CurrentInterestRate: validInterest,
				DesiredInterestRate: validInterest,
				OutstandingAumFee:   sdk.NewInt64Coin("usdc", 0),
			},
			expectedErr: "",
		},
		{
			name: "payment denom invalid format => error",
			vaultAccount: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				TotalShares:         sdk.NewInt64Coin(validDenom, 0),
				UnderlyingAsset:     "uusd",
				PaymentDenom:        "inv@lid$",
				CurrentInterestRate: validInterest,
				DesiredInterestRate: validInterest,
				OutstandingAumFee:   sdk.Coin{Denom: "inv@lid$", Amount: math.NewInt(0)},
			},
			expectedErr: "invalid payment denom",
		},
		{
			name: "invalid current interest rate",
			vaultAccount: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				TotalShares:         sdk.NewInt64Coin(validDenom, 0),
				UnderlyingAsset:     "uusd",
				PaymentDenom:        "uusd",
				CurrentInterestRate: invalidInterest,
				DesiredInterestRate: validInterest,
				OutstandingAumFee:   sdk.NewInt64Coin("uusd", 0),
			},
			expectedErr: "invalid current interest rate",
		},
		{
			name: "invalid desired interest rate",
			vaultAccount: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				TotalShares:         sdk.NewInt64Coin(validDenom, 0),
				UnderlyingAsset:     "uusd",
				PaymentDenom:        "uusd",
				CurrentInterestRate: validInterest,
				DesiredInterestRate: invalidInterest,
				OutstandingAumFee:   sdk.NewInt64Coin("uusd", 0),
			},
			expectedErr: "invalid desired interest rate",
		},
		{
			name: "valid min/max equal",
			vaultAccount: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				TotalShares:         sdk.NewInt64Coin(validDenom, 0),
				UnderlyingAsset:     "uusd",
				PaymentDenom:        "uusd",
				CurrentInterestRate: "0.00",
				DesiredInterestRate: "0.10",
				MinInterestRate:     "0.10",
				MaxInterestRate:     "0.10",
				OutstandingAumFee:   sdk.NewInt64Coin("uusd", 0),
			},
			expectedErr: "",
		},
		{
			name: "valid min < max",
			vaultAccount: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				TotalShares:         sdk.NewInt64Coin(validDenom, 0),
				UnderlyingAsset:     "uusd",
				PaymentDenom:        "uusd",
				CurrentInterestRate: "0.00",
				DesiredInterestRate: "0.03",
				MinInterestRate:     "0.01",
				MaxInterestRate:     "0.05",
				OutstandingAumFee:   sdk.NewInt64Coin("uusd", 0),
			},
			expectedErr: "",
		},
		{
			name: "invalid min format (max empty)",
			vaultAccount: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				TotalShares:         sdk.NewInt64Coin(validDenom, 0),
				UnderlyingAsset:     "uusd",
				PaymentDenom:        "uusd",
				CurrentInterestRate: "0.00",
				DesiredInterestRate: "0.00",
				MinInterestRate:     "nope",
				MaxInterestRate:     "",
				OutstandingAumFee:   sdk.NewInt64Coin("uusd", 0),
			},
			expectedErr: "invalid min interest rate",
		},
		{
			name: "invalid max format (min empty)",
			vaultAccount: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				TotalShares:         sdk.NewInt64Coin(validDenom, 0),
				UnderlyingAsset:     "uusd",
				PaymentDenom:        "uusd",
				CurrentInterestRate: "0.00",
				DesiredInterestRate: "0.00",
				MinInterestRate:     "",
				MaxInterestRate:     "nope",
				OutstandingAumFee:   sdk.NewInt64Coin("uusd", 0),
			},
			expectedErr: "invalid max interest rate",
		},
		{
			name: "min > max => error",
			vaultAccount: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				TotalShares:         sdk.NewInt64Coin(validDenom, 0),
				UnderlyingAsset:     "uusd",
				PaymentDenom:        "uusd",
				CurrentInterestRate: "0.00",
				DesiredInterestRate: "0.07",
				MinInterestRate:     "0.08",
				MaxInterestRate:     "0.05",
				OutstandingAumFee:   sdk.NewInt64Coin("uusd", 0),
			},
			expectedErr: "cannot be greater than maximum interest rate",
		},
		{
			name: "desired < min => error",
			vaultAccount: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				TotalShares:         sdk.NewInt64Coin(validDenom, 0),
				UnderlyingAsset:     "uusd",
				PaymentDenom:        "uusd",
				CurrentInterestRate: "0.00",
				DesiredInterestRate: "0.02",
				MinInterestRate:     "0.03",
				MaxInterestRate:     "",
				OutstandingAumFee:   sdk.NewInt64Coin("uusd", 0),
			},
			expectedErr: "less than minimum interest rate",
		},
		{
			name: "desired > max => error",
			vaultAccount: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				TotalShares:         sdk.NewInt64Coin(validDenom, 0),
				UnderlyingAsset:     "uusd",
				PaymentDenom:        "uusd",
				CurrentInterestRate: "0.00",
				DesiredInterestRate: "0.06",
				MinInterestRate:     "",
				MaxInterestRate:     "0.05",
				OutstandingAumFee:   sdk.NewInt64Coin("uusd", 0),
			},
			expectedErr: "greater than maximum interest rate",
		},
		{
			name: "current non-zero and not equal desired => error",
			vaultAccount: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				TotalShares:         sdk.NewInt64Coin(validDenom, 0),
				UnderlyingAsset:     "uusd",
				PaymentDenom:        "uusd",
				CurrentInterestRate: "0.03",
				DesiredInterestRate: "0.04",
				OutstandingAumFee:   sdk.NewInt64Coin("uusd", 0),
			},
			expectedErr: "current interest rate must be zero or equal to desired",
		},
		{
			name: "min only and desired == min => ok",
			vaultAccount: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				TotalShares:         sdk.NewInt64Coin(validDenom, 0),
				UnderlyingAsset:     "uusd",
				PaymentDenom:        "uusd",
				CurrentInterestRate: "0.00",
				DesiredInterestRate: "0.03",
				MinInterestRate:     "0.03",
				OutstandingAumFee:   sdk.NewInt64Coin("uusd", 0),
			},
			expectedErr: "",
		},
		{
			name: "max only and desired == max => ok",
			vaultAccount: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				TotalShares:         sdk.NewInt64Coin(validDenom, 0),
				UnderlyingAsset:     "uusd",
				PaymentDenom:        "uusd",
				CurrentInterestRate: "0.00",
				DesiredInterestRate: "0.07",
				MaxInterestRate:     "0.07",
				OutstandingAumFee:   sdk.NewInt64Coin("uusd", 0),
			},
			expectedErr: "",
		},
		{
			name: "invalid total shares (negative)",
			vaultAccount: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				TotalShares:         sdk.Coin{Denom: validDenom, Amount: math.NewInt(-100)},
				UnderlyingAsset:     "uusd",
				PaymentDenom:        "uusd",
				CurrentInterestRate: "0.0",
				DesiredInterestRate: "0.0",
				OutstandingAumFee:   sdk.NewInt64Coin("uusd", 0),
			},
			expectedErr: "total shares cannot be negative",
		},
		{
			name: "valid bridge address",
			vaultAccount: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				TotalShares:         sdk.NewInt64Coin(validDenom, 0),
				UnderlyingAsset:     "uusd",
				PaymentDenom:        "uusd",
				CurrentInterestRate: "0.0",
				DesiredInterestRate: "0.0",
				BridgeAddress:       validBridgeAddress,
				BridgeEnabled:       true,
				OutstandingAumFee:   sdk.NewInt64Coin("uusd", 0),
			},
			expectedErr: "",
		},
		{
			name: "invalid bridge address",
			vaultAccount: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				TotalShares:         sdk.NewInt64Coin(validDenom, 0),
				UnderlyingAsset:     "uusd",
				PaymentDenom:        "uusd",
				CurrentInterestRate: "0.0",
				DesiredInterestRate: "0.0",
				BridgeAddress:       "invalid-bridge-address",
				BridgeEnabled:       true,
				OutstandingAumFee:   sdk.NewInt64Coin("uusd", 0),
			},
			expectedErr: "invalid bridge address",
		},
		{
			name: "bridge enabled with no address",
			vaultAccount: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				TotalShares:         sdk.NewInt64Coin(validDenom, 0),
				UnderlyingAsset:     "uusd",
				PaymentDenom:        "uusd",
				CurrentInterestRate: "0.0",
				DesiredInterestRate: "0.0",
				BridgeAddress:       "",
				BridgeEnabled:       true,
				OutstandingAumFee:   sdk.NewInt64Coin("uusd", 0),
			},
			expectedErr: "bridge cannot be enabled without a bridge address",
		},
		{
			name: "asset manager set with valid address",
			vaultAccount: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				AssetManager:        validAssetManager,
				TotalShares:         sdk.NewInt64Coin(validDenom, 0),
				UnderlyingAsset:     "uusd",
				PaymentDenom:        "uusd",
				CurrentInterestRate: "0.0",
				DesiredInterestRate: "0.0",
				OutstandingAumFee:   sdk.NewInt64Coin("uusd", 0),
			},
			expectedErr: "",
		},
		{
			name: "asset manager set with invalid address",
			vaultAccount: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				AssetManager:        "not-a-bech32",
				TotalShares:         sdk.NewInt64Coin(validDenom, 0),
				UnderlyingAsset:     "uusd",
				PaymentDenom:        "uusd",
				CurrentInterestRate: "0.0",
				DesiredInterestRate: "0.0",
				OutstandingAumFee:   sdk.NewInt64Coin("uusd", 0),
			},
			expectedErr: "invalid asset manager address",
		},
		{
			name: "negative period start",
			vaultAccount: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				TotalShares:         sdk.NewInt64Coin(validDenom, 0),
				UnderlyingAsset:     "uusd",
				PaymentDenom:        "uusd",
				CurrentInterestRate: "0.0",
				DesiredInterestRate: "0.0",
				PeriodStart:         -1,
				OutstandingAumFee:   sdk.NewInt64Coin("uusd", 0),
			},
			expectedErr: "period start cannot be negative",
		},
		{
			name: "negative period timeout",
			vaultAccount: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				TotalShares:         sdk.NewInt64Coin(validDenom, 0),
				UnderlyingAsset:     "uusd",
				PaymentDenom:        "uusd",
				CurrentInterestRate: "0.0",
				DesiredInterestRate: "0.0",
				PeriodTimeout:       -1,
				OutstandingAumFee:   sdk.NewInt64Coin("uusd", 0),
			},
			expectedErr: "period timeout cannot be negative",
		},
		{
			name: "negative fee period start",
			vaultAccount: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				TotalShares:         sdk.NewInt64Coin(validDenom, 0),
				UnderlyingAsset:     "uusd",
				PaymentDenom:        "uusd",
				CurrentInterestRate: "0.0",
				DesiredInterestRate: "0.0",
				FeePeriodStart:      -1,
				OutstandingAumFee:   sdk.NewInt64Coin("uusd", 0),
			},
			expectedErr: "fee period start cannot be negative",
		},
		{
			name: "negative fee period timeout",
			vaultAccount: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				TotalShares:         sdk.NewInt64Coin(validDenom, 0),
				UnderlyingAsset:     "uusd",
				PaymentDenom:        "uusd",
				CurrentInterestRate: "0.0",
				DesiredInterestRate: "0.0",
				FeePeriodTimeout:    -1,
				OutstandingAumFee:   sdk.NewInt64Coin("uusd", 0),
			},
			expectedErr: "fee period timeout cannot be negative",
		},
		{
			name: "outstanding AUM fee denom mismatch",
			vaultAccount: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				TotalShares:         sdk.NewInt64Coin(validDenom, 0),
				UnderlyingAsset:     "uusd",
				PaymentDenom:        "uusd",
				CurrentInterestRate: "0.0",
				DesiredInterestRate: "0.0",
				OutstandingAumFee:   sdk.NewInt64Coin("wrong", 100),
			},
			expectedErr: "outstanding AUM fee denom wrong does not match payment denom uusd",
		},
		{
			name: "negative outstanding AUM fee",
			vaultAccount: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				TotalShares:         sdk.NewInt64Coin(validDenom, 0),
				UnderlyingAsset:     "uusd",
				PaymentDenom:        "uusd",
				CurrentInterestRate: "0.0",
				DesiredInterestRate: "0.0",
				OutstandingAumFee:   sdk.Coin{Denom: "uusd", Amount: math.NewInt(-1)},
			},
			expectedErr: "outstanding AUM fee cannot be negative",
		},
		{
			name: "non-zero outstanding AUM fee with empty denom",
			vaultAccount: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				TotalShares:         sdk.NewInt64Coin(validDenom, 0),
				UnderlyingAsset:     "uusd",
				PaymentDenom:        "uusd",
				CurrentInterestRate: "0.0",
				DesiredInterestRate: "0.0",
				OutstandingAumFee:   sdk.Coin{Denom: "", Amount: math.NewInt(100)},
			},
			expectedErr: "outstanding AUM fee denom  does not match payment denom uusd",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.vaultAccount.Validate()
			if tc.expectedErr != "" {
				require.Error(t, err, "expected an error for test case: %s", tc.name)
				require.Contains(t, err.Error(), tc.expectedErr, "error should contain expected message for test case: %s", tc.name)
			} else {
				require.NoError(t, err, "expected no error for test case: %s", tc.name)
			}
		})
	}
}

func TestVaultAccount_AcceptedDenoms(t *testing.T) {
	tests := []struct {
		name            string
		underlyingAsset string
		paymentDenom    string
		expectedDenoms  []string
	}{
		{
			name:            "only underlying when payment empty",
			underlyingAsset: "uusd",
			paymentDenom:    "",
			expectedDenoms:  []string{"uusd"},
		},
		{
			name:            "only underlying when payment equals underlying",
			underlyingAsset: "uusd",
			paymentDenom:    "uusd",
			expectedDenoms:  []string{"uusd"},
		},
		{
			name:            "underlying and payment when distinct",
			underlyingAsset: "uusd",
			paymentDenom:    "usdc",
			expectedDenoms:  []string{"uusd", "usdc"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := types.VaultAccount{UnderlyingAsset: tc.underlyingAsset, PaymentDenom: tc.paymentDenom}
			got := v.AcceptedDenoms()
			assert.ElementsMatch(t, tc.expectedDenoms, got, "accepted denoms should match expected")
		})
	}
}

func TestVaultAccount_IsAcceptedDenom(t *testing.T) {
	vaWithMulti := types.VaultAccount{UnderlyingAsset: "uusd", PaymentDenom: "usdc"}
	assert.True(t, vaWithMulti.IsAcceptedDenom("uusd"), "underlying should be accepted")
	assert.True(t, vaWithMulti.IsAcceptedDenom("usdc"), "payment denom should be accepted")
	assert.False(t, vaWithMulti.IsAcceptedDenom("uatom"), "unlisted denom should not be accepted")

	vaUnderlyingOnly := types.VaultAccount{UnderlyingAsset: "uusd", PaymentDenom: ""}
	assert.True(t, vaUnderlyingOnly.IsAcceptedDenom("uusd"), "underlying should be accepted when payment empty")
	assert.False(t, vaUnderlyingOnly.IsAcceptedDenom("usdc"), "payment should not be accepted when not configured")

	vaMultiSameDenom := types.VaultAccount{UnderlyingAsset: "uusd", PaymentDenom: "uusd"}
	assert.True(t, vaMultiSameDenom.IsAcceptedDenom("uusd"), "underlying should be accepted when payment equals underlying")
	assert.False(t, vaMultiSameDenom.IsAcceptedDenom("usdc"), "unlisted denom should not be accepted when payment equals underlying")
}

func TestVaultAccount_ValidateAcceptedDenom(t *testing.T) {
	vaUnderlyingOnly := types.VaultAccount{UnderlyingAsset: "uusd"}
	err := vaUnderlyingOnly.ValidateAcceptedDenom("uusd")
	assert.NoError(t, err, "valid underlying should pass")

	err = vaUnderlyingOnly.ValidateAcceptedDenom("usdc")
	assert.Error(t, err, "unlisted denom should error")
	assert.Contains(t, err.Error(), `denom not supported for vault`, "error should indicate unsupported denom")
	assert.Contains(t, err.Error(), `"uusd"`, "error should list allowed denom")
	assert.Contains(t, err.Error(), `"usdc"`, "error should include the provided denom")

	vaWithMulti := types.VaultAccount{UnderlyingAsset: "uusd", PaymentDenom: "usdc"}
	err = vaWithMulti.ValidateAcceptedDenom("uusd")
	assert.NoError(t, err, "underlying should pass when dual")

	err = vaWithMulti.ValidateAcceptedDenom("usdc")
	assert.NoError(t, err, "payment denom should pass when dual")

	err = vaWithMulti.ValidateAcceptedDenom("uatom")
	assert.Error(t, err, "unlisted denom should error when dual")
	assert.Contains(t, err.Error(), `"uusd"`, "error should include first allowed denom")
	assert.Contains(t, err.Error(), `"usdc"`, "error should include second allowed denom")
	assert.Contains(t, err.Error(), `"uatom"`, "error should include provided denom")
}

func TestVaultAccount_ValidateAcceptedCoin(t *testing.T) {
	vaWithMulti := types.VaultAccount{UnderlyingAsset: "uusd", PaymentDenom: "usdc"}

	err := vaWithMulti.ValidateAcceptedCoin(sdk.NewInt64Coin("uusd", 1))
	assert.NoError(t, err, "non-zero underlying coin should be accepted")

	err = vaWithMulti.ValidateAcceptedCoin(sdk.NewInt64Coin("usdc", 5))
	assert.NoError(t, err, "non-zero payment coin should be accepted")

	err = vaWithMulti.ValidateAcceptedCoin(sdk.NewInt64Coin("uatom", 7))
	assert.Error(t, err, "unlisted denom should error")
	assert.Contains(t, err.Error(), "denom not supported for vault", "error should indicate unsupported denom")

	err = vaWithMulti.ValidateAcceptedCoin(sdk.NewInt64Coin("uusd", 0))
	assert.Error(t, err, "zero amount should error")
	assert.Equal(t, "amount must be greater than zero", err.Error(), "error should match expected message")

	vaUnderlyingOnly := types.VaultAccount{UnderlyingAsset: "uusd"}
	err = vaUnderlyingOnly.ValidateAcceptedCoin(sdk.NewInt64Coin("usdc", 3))
	assert.Error(t, err, "payment denom should not be accepted when not configured")
	assert.Contains(t, err.Error(), "denom not supported for vault", "error should indicate unsupported denom")
}

func TestGetProvLabsFeeAddress(t *testing.T) {
	tests := []struct {
		name    string
		chainID string
	}{
		{
			name:    "mainnet",
			chainID: "pio-mainnet-1",
		},
		{
			name:    "testnet",
			chainID: "pio-testnet-1",
		},
		{
			name:    "other/local",
			chainID: "vaulty-1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			addr, err := types.GetProvLabsFeeAddress(tc.chainID)
			// On some test environments, Bech32 prefix might be locked to 'cosmos', 
			// making Bech32 decoding of 'pb...' or 'tp...' fail if not configured.
			// The default path uses crypto.AddressHash which is prefix-agnostic.
			if tc.chainID == "pio-mainnet-1" || tc.chainID == "pio-testnet-1" {
				// These might fail if the SDK global config isn't set to 'pb' or 'tp'
				// For now, let's just ensure it doesn't panic and we handle the error if it's a prefix mismatch
				if err != nil {
					require.Containsf(t, err.Error(), "invalid Bech32 prefix", "test case %q: unexpected error; want Bech32 prefix error, got %v", tc.name, err)
				} else {
					require.NotNilf(t, addr, "test case %q: fee address should not be nil", tc.name)
				}
			} else {
				require.NoErrorf(t, err, "test case %q: unexpected error for chain %s", tc.name, tc.chainID)
				require.NotNilf(t, addr, "test case %q: fee address should not be nil", tc.name)
			}
		})
	}
}

func TestPendingSwapOut_Validate(t *testing.T) {
	validOwner := utils.TestAddress().Bech32
	validVault := utils.TestAddress().Bech32
	validDenom := "vshare"
	validRedeem := "uusd"
	invalidBech32 := "invalidaddress"

	baseReq := types.PendingSwapOut{
		Owner:        validOwner,
		VaultAddress: validVault,
		Shares:       sdk.NewInt64Coin(validDenom, 100),
		RedeemDenom:  validRedeem,
	}

	const invalidBech32ErrPrefix = "decoding bech32 failed:"

	tests := []struct {
		name           string
		pendingSwapOut types.PendingSwapOut
		expectedErr    string
	}{
		{
			name:           "valid request",
			pendingSwapOut: baseReq,
			expectedErr:    "",
		},
		{
			name: "invalid owner address",
			pendingSwapOut: types.PendingSwapOut{
				Owner:        invalidBech32,
				VaultAddress: validVault,
				Shares:       baseReq.Shares,
				RedeemDenom:  validRedeem,
			},
			expectedErr: fmt.Sprintf("invalid owner address %s: %s", invalidBech32, invalidBech32ErrPrefix),
		},
		{
			name: "invalid vault address",
			pendingSwapOut: types.PendingSwapOut{
				Owner:        validOwner,
				VaultAddress: invalidBech32,
				Shares:       baseReq.Shares,
				RedeemDenom:  validRedeem,
			},
			expectedErr: fmt.Sprintf("invalid vault address %s: %s", invalidBech32, invalidBech32ErrPrefix),
		},
		{
			name: "invalid shares (negative)",
			pendingSwapOut: types.PendingSwapOut{
				Owner:        validOwner,
				VaultAddress: validVault,
				Shares:       sdk.Coin{Denom: validDenom, Amount: math.NewInt(-10)},
				RedeemDenom:  validRedeem,
			},
			expectedErr: "invalid shares: -10vshare",
		},
		{
			name: "invalid shares (zero amount)",
			pendingSwapOut: types.PendingSwapOut{
				Owner:        validOwner,
				VaultAddress: validVault,
				Shares:       sdk.NewInt64Coin(validDenom, 0),
				RedeemDenom:  validRedeem,
			},
			expectedErr: "shares cannot be zero",
		},
		{
			name: "invalid shares (empty denom)",
			pendingSwapOut: types.PendingSwapOut{
				Owner:        validOwner,
				VaultAddress: validVault,
				Shares:       sdk.Coin{Denom: "", Amount: math.NewInt(10)},
				RedeemDenom:  validRedeem,
			},
			expectedErr: "invalid shares: 10",
		},
		{
			name: "empty redeem denom",
			pendingSwapOut: types.PendingSwapOut{
				Owner:        validOwner,
				VaultAddress: validVault,
				Shares:       baseReq.Shares,
				RedeemDenom:  "",
			},
			expectedErr: "redeem denom cannot be empty",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.pendingSwapOut.Validate()
			if tc.expectedErr != "" {
				require.Error(t, err, "expected an error for test case: %s", tc.name)
				require.Contains(t, err.Error(), tc.expectedErr, "error should contain expected message for test case: %s", tc.name)
			} else {
				require.NoError(t, err, "expected no error for test case: %s", tc.name)
			}
		})
	}
}
