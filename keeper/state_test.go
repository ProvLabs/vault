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

func (s *TestSuite) TestFindVaultAccount() {
	// Vaults to be created during setup
	shareDenom1 := "share1"
	underlying1 := "underlying1"
	var vault1 *types.VaultAccount

	// An address that doesn't correspond to any created vault
	nonExistentAddr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())

	testCases := []struct {
		name          string
		setup         func()
		idToFind      func() string
		expectedVault func() *types.VaultAccount
		expectErr     bool
		errContains   string
	}{
		{
			name: "success - find by address",
			setup: func() {
				s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlying1, 1000), s.adminAddr)
				created, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{Admin: s.adminAddr.String(), ShareDenom: shareDenom1, UnderlyingAsset: underlying1})
				s.Require().NoError(err)
				// Re-fetch to get full account details
				vault1, err = s.k.GetVault(s.ctx, created.GetAddress())
				s.Require().NoError(err)
			},
			idToFind:      func() string { return vault1.GetAddress().String() },
			expectedVault: func() *types.VaultAccount { return vault1 },
			expectErr:     false,
		},
		{
			name: "success - find by share denom",
			setup: func() {
				s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlying1, 1000), s.adminAddr)
				created, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{Admin: s.adminAddr.String(), ShareDenom: shareDenom1, UnderlyingAsset: underlying1})
				s.Require().NoError(err)
				vault1, err = s.k.GetVault(s.ctx, created.GetAddress())
				s.Require().NoError(err)
			},
			idToFind:      func() string { return shareDenom1 },
			expectedVault: func() *types.VaultAccount { return vault1 },
			expectErr:     false,
		},
		{
			name: "failure - not found by address",
			setup: func() {
				s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlying1, 1000), s.adminAddr)
				_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{Admin: s.adminAddr.String(), ShareDenom: shareDenom1, UnderlyingAsset: underlying1})
				s.Require().NoError(err)
			},
			idToFind:      func() string { return nonExistentAddr.String() },
			expectedVault: func() *types.VaultAccount { return nil },
			expectErr:     true,
			errContains:   "not found",
		},
		{
			name:          "failure - not found by share denom",
			idToFind:      func() string { return "nonexistentshare" },
			expectedVault: func() *types.VaultAccount { return nil },
			expectErr:     true,
			errContains:   "not found",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			if tc.setup != nil {
				tc.setup()
			}

			actualVault, err := s.k.FindVaultAccount(s.ctx, tc.idToFind())

			if tc.expectErr {
				s.Require().Error(err)
				s.Require().ErrorContains(err, tc.errContains)
				s.Require().Nil(actualVault)
			} else {
				s.Require().NoError(err)
				s.Require().NotNil(actualVault)
				s.Require().Equal(tc.expectedVault(), actualVault)
			}
		})
	}
}
