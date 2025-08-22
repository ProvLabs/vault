package keeper_test

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"

	"github.com/provlabs/vault/types"
	"github.com/provlabs/vault/utils"
	"github.com/provlabs/vault/utils/mocks"
)

func TestGetVaults(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)

	vaults, err := k.GetVaults(ctx)

	require.NoError(t, err, "expected no error when the vaults map is empty")
	require.Empty(t, vaults, "expected empty vaults map")

	// Generate unique addresses for vault keys
	vault1Addr := utils.TestAddress().Bech32
	vault2Addr := utils.TestAddress().Bech32
	// Ensure they are different for testing distinct entries
	for vault1Addr == vault2Addr {
		vault2Addr = utils.TestAddress().Bech32
	}

	vault1 := types.VaultAccount{
		BaseAccount: authtypes.NewBaseAccountWithAddress(types.GetVaultAddress("address1")),
		Admin:       utils.TestAddress().Bech32,
	}
	vault2 := types.VaultAccount{
		BaseAccount: authtypes.NewBaseAccountWithAddress(types.GetVaultAddress("address2")),
		Admin:       utils.TestAddress().Bech32,
	}

	// Set vaults using their Bech32 addresses as keys
	err = k.Vaults.Set(ctx, sdk.MustAccAddressFromBech32(vault1.Address), []byte{})
	require.NoError(t, err, "expected no error setting the first vault")
	err = k.Vaults.Set(ctx, sdk.MustAccAddressFromBech32(vault2.Address), []byte{})
	require.NoError(t, err, "expected no error setting the second vault")

	vaults, err = k.GetVaults(ctx)

	require.NoError(t, err, "expected no error when vaults are present")
	require.Len(t, vaults, 2, "expected two vaults")
	// Assert using the vault addresses as keys
	require.Contains(t, vaults, sdk.MustAccAddressFromBech32(vault1.Address), "expected the first vault")
	require.Contains(t, vaults, sdk.MustAccAddressFromBech32(vault2.Address), "expected the second vault")
}

func (s *TestSuite) TestGetVaultAccounts() {
	testCases := []struct {
		name           string
		setup          func() []*types.VaultAccount
		expectErr      bool
		errContains    string
		expectedLength int
	}{
		{
			name: "success - multiple vaults",
			setup: func() []*types.VaultAccount {
				s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin("underlying1", 1000), s.adminAddr)
				s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin("underlying2", 1000), s.adminAddr)

				vault1, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{Admin: s.adminAddr.String(), ShareDenom: "share1", UnderlyingAsset: "underlying1"})
				s.Require().NoError(err)
				vault2, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{Admin: s.adminAddr.String(), ShareDenom: "share2", UnderlyingAsset: "underlying2"})
				s.Require().NoError(err)

				// Re-fetch from state to get latest account numbers, etc.
				v1, err := s.k.GetVault(s.ctx, vault1.GetAddress())
				s.Require().NoError(err)
				v2, err := s.k.GetVault(s.ctx, vault2.GetAddress())
				s.Require().NoError(err)

				return []*types.VaultAccount{v1, v2}
			},
			expectErr:      false,
			expectedLength: 2,
		},
		{
			name: "success - single vault",
			setup: func() []*types.VaultAccount {
				s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin("underlying1", 1000), s.adminAddr)

				vault1, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{Admin: s.adminAddr.String(), ShareDenom: "share1", UnderlyingAsset: "underlying1"})
				s.Require().NoError(err)

				v1, err := s.k.GetVault(s.ctx, vault1.GetAddress())
				s.Require().NoError(err)

				return []*types.VaultAccount{v1}
			},
			expectErr:      false,
			expectedLength: 1,
		},
		{
			name: "success - no vaults",
			setup: func() []*types.VaultAccount {
				return []*types.VaultAccount{}
			},
			expectErr:      false,
			expectedLength: 0,
		},
		{
			name: "failure - inconsistent state with non-vault account",
			setup: func() []*types.VaultAccount {
				// Create a standard account, not a vault account
				senderPrivKey := secp256k1.GenPrivKey()
				acc := authtypes.NewBaseAccount(senderPrivKey.PubKey().Address().Bytes(), senderPrivKey.PubKey(), 100, 0)
				s.k.AuthKeeper.SetAccount(s.ctx, acc)

				// Manually add it to the vault index to create an inconsistent state
				err := s.k.Vaults.Set(s.ctx, acc.GetAddress(), []byte{})
				s.Require().NoError(err)

				return nil // We don't expect any vaults back, just an error
			},
			expectErr:   true,
			errContains: "is not a vault account",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			expectedVaults := tc.setup()

			// Call the function under test
			actualVaults, err := s.k.GetVaultAccounts(s.ctx)

			if tc.expectErr {
				s.Require().Error(err)
				s.Require().ErrorContains(err, tc.errContains)
				s.Require().Nil(actualVaults)
			} else {
				s.Require().NoError(err)
				s.Require().NotNil(actualVaults)
				s.Require().Len(actualVaults, tc.expectedLength)
				// Using ElementsMatch to compare slices without regard to order
				s.Require().ElementsMatch(expectedVaults, actualVaults)
			}
		})
	}
}
