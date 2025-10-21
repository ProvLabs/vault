package keeper_test

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/provlabs/vault/types"
	"github.com/provlabs/vault/utils"
	"github.com/provlabs/vault/utils/mocks"
)

func TestSetVaultLookup_ErrorsAndSuccess(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)

	err := k.SetVaultLookup(ctx, nil)
	require.Error(t, err, "expected error when vault is nil")

	vBad := &types.VaultAccount{BaseAccount: &authtypes.BaseAccount{Address: "not-bech32"}}
	err = k.SetVaultLookup(ctx, vBad)
	require.Error(t, err, "expected error when vault address is not bech32")

	addr := utils.TestProvlabsAddress().Bech32
	v := &types.VaultAccount{BaseAccount: authtypes.NewBaseAccountWithAddress(sdk.MustAccAddressFromBech32(addr))}
	err = k.SetVaultLookup(ctx, v)
	require.NoError(t, err, "expected no error when vault is valid")

	got, err := k.GetVaults(ctx)
	require.NoError(t, err, "expected no error retrieving vaults after setting")
	require.Len(t, got, 1, "expected exactly one vault stored")
	assert.True(t, got[0].Equals(sdk.MustAccAddressFromBech32(addr)), "expected stored vault address to match input")
}

func TestSetVaultAccount_ValidateError(t *testing.T) {
	_, k := mocks.NewVaultKeeper(t)

	err := k.SetVaultAccount(sdk.Context{}, &types.VaultAccount{})
	require.Error(t, err, "expected error when vault validation fails")
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

