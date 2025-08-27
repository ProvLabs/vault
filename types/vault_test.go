package types_test

import (
	"fmt"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"

	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/provlabs/vault/types"
	"github.com/provlabs/vault/utils"
)

func TestVaultAccount_Validate(t *testing.T) {
	validAdmin := utils.TestAddress().Bech32
	validDenom := "validdenom"
	invalidDenom := "inval!d"
	validInterest := "0.05"
	invalidInterest := "not-a-decimal"

	baseAcc := authtypes.NewBaseAccountWithAddress(sdk.MustAccAddressFromBech32(validAdmin))

	tests := []struct {
		name        string
		account     types.VaultAccount
		expectedErr string
	}{
		{
			name: "valid vault account with interest rates (current==desired)",
			account: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				ShareDenom:          validDenom,
				UnderlyingAsset:     "uusd",
				CurrentInterestRate: validInterest,
				DesiredInterestRate: validInterest,
			},
			expectedErr: "",
		},
		{
			name: "valid vault account with bounds and current==0",
			account: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				ShareDenom:          validDenom,
				UnderlyingAsset:     "uusd",
				CurrentInterestRate: "0.00",
				DesiredInterestRate: "0.04",
				MinInterestRate:     "0.03",
				MaxInterestRate:     "0.05",
			},
			expectedErr: "",
		},
		{
			name: "invalid admin address",
			account: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               "invalid-address",
				ShareDenom:          validDenom,
				UnderlyingAsset:     "uusd",
				CurrentInterestRate: validInterest,
				DesiredInterestRate: validInterest,
			},
			expectedErr: "invalid admin address",
		},
		{
			name: "empty share denom",
			account: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				ShareDenom:          "",
				UnderlyingAsset:     "uusd",
				CurrentInterestRate: validInterest,
				DesiredInterestRate: validInterest,
			},
			expectedErr: "invalid share denom",
		},
		{
			name: "invalid share denom",
			account: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				ShareDenom:          invalidDenom,
				UnderlyingAsset:     "uusd",
				CurrentInterestRate: validInterest,
				DesiredInterestRate: validInterest,
			},
			expectedErr: "invalid share denom",
		},
		{
			name: "empty underlying assets",
			account: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				ShareDenom:          validDenom,
				UnderlyingAsset:     "",
				CurrentInterestRate: validInterest,
				DesiredInterestRate: validInterest,
			},
			expectedErr: "at least one underlying asset is required",
		},
		{
			name: "invalid underlying asset denom",
			account: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				ShareDenom:          validDenom,
				UnderlyingAsset:     invalidDenom,
				CurrentInterestRate: validInterest,
				DesiredInterestRate: validInterest,
			},
			expectedErr: fmt.Sprintf("invalid underlying asset denom: %s", invalidDenom),
		},
		{
			name: "invalid current interest rate",
			account: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				ShareDenom:          validDenom,
				UnderlyingAsset:     "uusd",
				CurrentInterestRate: invalidInterest,
				DesiredInterestRate: validInterest,
			},
			expectedErr: "invalid current interest rate",
		},
		{
			name: "invalid desired interest rate",
			account: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				ShareDenom:          validDenom,
				UnderlyingAsset:     "uusd",
				CurrentInterestRate: validInterest,
				DesiredInterestRate: invalidInterest,
			},
			expectedErr: "invalid desired interest rate",
		},
		{
			name: "valid min/max equal",
			account: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				ShareDenom:          validDenom,
				UnderlyingAsset:     "uusd",
				CurrentInterestRate: "0.00",
				DesiredInterestRate: "0.10",
				MinInterestRate:     "0.10",
				MaxInterestRate:     "0.10",
			},
			expectedErr: "",
		},
		{
			name: "valid min < max",
			account: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				ShareDenom:          validDenom,
				UnderlyingAsset:     "uusd",
				CurrentInterestRate: "0.00",
				DesiredInterestRate: "0.03",
				MinInterestRate:     "0.01",
				MaxInterestRate:     "0.05",
			},
			expectedErr: "",
		},
		{
			name: "invalid min format (max empty)",
			account: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				ShareDenom:          validDenom,
				UnderlyingAsset:     "uusd",
				CurrentInterestRate: "0.00",
				DesiredInterestRate: "0.00",
				MinInterestRate:     "nope",
				MaxInterestRate:     "",
			},
			expectedErr: "invalid min interest rate",
		},
		{
			name: "invalid max format (min empty)",
			account: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				ShareDenom:          validDenom,
				UnderlyingAsset:     "uusd",
				CurrentInterestRate: "0.00",
				DesiredInterestRate: "0.00",
				MinInterestRate:     "",
				MaxInterestRate:     "nope",
			},
			expectedErr: "invalid max interest rate",
		},
		{
			name: "min > max => error",
			account: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				ShareDenom:          validDenom,
				UnderlyingAsset:     "uusd",
				CurrentInterestRate: "0.00",
				DesiredInterestRate: "0.07",
				MinInterestRate:     "0.08",
				MaxInterestRate:     "0.05",
			},
			expectedErr: "cannot be greater than maximum interest rate",
		},
		{
			name: "desired < min => error",
			account: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				ShareDenom:          validDenom,
				UnderlyingAsset:     "uusd",
				CurrentInterestRate: "0.00",
				DesiredInterestRate: "0.02",
				MinInterestRate:     "0.03",
				MaxInterestRate:     "",
			},
			expectedErr: "less than minimum interest rate",
		},
		{
			name: "desired > max => error",
			account: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				ShareDenom:          validDenom,
				UnderlyingAsset:     "uusd",
				CurrentInterestRate: "0.00",
				DesiredInterestRate: "0.06",
				MinInterestRate:     "",
				MaxInterestRate:     "0.05",
			},
			expectedErr: "greater than maximum interest rate",
		},
		{
			name: "current non-zero and not equal desired => error",
			account: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				ShareDenom:          validDenom,
				UnderlyingAsset:     "uusd",
				CurrentInterestRate: "0.03",
				DesiredInterestRate: "0.04",
			},
			expectedErr: "current interest rate must be zero or equal to desired",
		},
		{
			name: "min only and desired == min => ok",
			account: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				ShareDenom:          validDenom,
				UnderlyingAsset:     "uusd",
				CurrentInterestRate: "0.00",
				DesiredInterestRate: "0.03",
				MinInterestRate:     "0.03",
			},
			expectedErr: "",
		},
		{
			name: "max only and desired == max => ok",
			account: types.VaultAccount{
				BaseAccount:         baseAcc,
				Admin:               validAdmin,
				ShareDenom:          validDenom,
				UnderlyingAsset:     "uusd",
				CurrentInterestRate: "0.00",
				DesiredInterestRate: "0.07",
				MaxInterestRate:     "0.07",
			},
			expectedErr: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.account.Validate()
			if tc.expectedErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
