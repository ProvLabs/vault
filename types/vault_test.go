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

	baseAcc := authtypes.NewBaseAccountWithAddress(sdk.MustAccAddressFromBech32(validAdmin))

	tests := []struct {
		name        string
		account     types.VaultAccount
		expectedErr string
	}{
		{
			name: "valid vault account",
			account: types.VaultAccount{
				BaseAccount:      baseAcc,
				Admin:            validAdmin,
				ShareDenom:       validDenom,
				UnderlyingAssets: []string{"uusd"},
			},
			expectedErr: "",
		},
		{
			name: "invalid admin address",
			account: types.VaultAccount{
				BaseAccount:      baseAcc,
				Admin:            "invalid-address",
				ShareDenom:       validDenom,
				UnderlyingAssets: []string{"uusd"},
			},
			expectedErr: "invalid admin address",
		},
		{
			name: "empty share denom",
			account: types.VaultAccount{
				BaseAccount:      baseAcc,
				Admin:            validAdmin,
				ShareDenom:       "",
				UnderlyingAssets: []string{"uusd"},
			},
			expectedErr: "invalid share denom",
		},
		{
			name: "invalid share denom",
			account: types.VaultAccount{
				BaseAccount:      baseAcc,
				Admin:            validAdmin,
				ShareDenom:       invalidDenom,
				UnderlyingAssets: []string{"uusd"},
			},
			expectedErr: "invalid share denom",
		},
		{
			name: "empty underlying assets",
			account: types.VaultAccount{
				BaseAccount:      baseAcc,
				Admin:            validAdmin,
				ShareDenom:       validDenom,
				UnderlyingAssets: []string{},
			},
			expectedErr: "at least one underlying asset is required",
		},
		{
			name: "invalid underlying asset denom",
			account: types.VaultAccount{
				BaseAccount:      baseAcc,
				Admin:            validAdmin,
				ShareDenom:       validDenom,
				UnderlyingAssets: []string{invalidDenom},
			},
			expectedErr: fmt.Sprintf("invalid underlying asset denom: %s", invalidDenom),
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

func TestVaultAccount_ValidateUnderlyingAssets(t *testing.T) {
	vault := types.VaultAccount{
		UnderlyingAssets: []string{"jackthecat", "georgethedog"},
	}

	tests := []struct {
		name        string
		asset       sdk.Coin
		expectedErr string
	}{
		{
			name:        "valid asset denom match (jackthecat)",
			asset:       sdk.NewInt64Coin("jackthecat", 100),
			expectedErr: "",
		},
		{
			name:        "valid asset denom match (georgethedog)",
			asset:       sdk.NewInt64Coin("georgethedog", 50),
			expectedErr: "",
		},
		{
			name:        "unsupported asset denom",
			asset:       sdk.NewInt64Coin("btc", 10),
			expectedErr: "btc asset denom not supported for vault",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := vault.ValidateUnderlyingAssets(tc.asset)
			if tc.expectedErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
