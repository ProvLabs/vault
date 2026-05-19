package types_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/assert"

	"github.com/provlabs/vault/utils"
)

// NewTestAddress returns a random valid bech32 address for use in tests.
func NewTestAddress() string {
	return utils.TestAddress().Bech32
}

// validateBasicCase is a table test case for ValidateBasic tests.
type validateBasicCase struct {
	name        string
	msg         interface{ ValidateBasic() error }
	expectedErr string
}

// RunValidateBasicTable runs a table-driven ValidateBasic test over the given cases.
func RunValidateBasicTable(t *testing.T, cases []validateBasicCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if tc.expectedErr != "" {
				assert.Error(t, err, "expected error for case %q", tc.name)
				assert.Contains(t, err.Error(), tc.expectedErr, "error should contain expected substring for case %q", tc.name)
			} else {
				assert.NoError(t, err, "expected no error for case %q", tc.name)
			}
		})
	}
}

// makeNAVAuthorityFixtures returns three distinct test addresses and a base account for use
// in NAV authority tests. admin is used as the vault admin; oracle is a separate authority;
// other is an unrelated address; baseAcc is a BaseAccount keyed to admin.
func makeNAVAuthorityFixtures() (admin, oracle, other string, baseAcc *authtypes.BaseAccount) {
	admin = utils.TestAddress().Bech32
	oracle = utils.TestAddress().Bech32
	other = utils.TestAddress().Bech32
	baseAcc = authtypes.NewBaseAccountWithAddress(sdk.MustAccAddressFromBech32(admin))
	return
}
