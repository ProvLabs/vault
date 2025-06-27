package keeper_test

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/provlabs/vault/types"
)

func (s *TestSuite) TestCreateVaultAccount_And_GetVault() {
	shareDenom := "vaultshare"
	underlying := "vaultbase"

	vault, err := s.k.CreateVaultAccount(s.ctx, s.adminAddr.String(), shareDenom, underlying)
	s.Require().NoError(err, "CreateVaultAccount should not error")
	s.Require().Equal(types.GetVaultAddress(shareDenom).String(), vault.Address, "vault address mismatch")
	s.Require().Equal(s.adminAddr.String(), vault.Admin, "vault admin mismatch")

	foundVault, err := s.k.GetVault(s.ctx, types.GetVaultAddress(shareDenom))
	s.Require().NoError(err, "GetVault should not error")
	s.Require().Equal(vault.Address, foundVault.Address, "retrieved vault address mismatch")
}

func (s *TestSuite) TestCreateVaultAccount_AlreadyExistsError() {
	shareDenom := "dupecoin"
	underlying := "basecoin"

	addr := types.GetVaultAddress(shareDenom)
	existing := s.simApp.AccountKeeper.NewAccountWithAddress(s.ctx, addr)
	existing.SetSequence(1000)
	s.simApp.AccountKeeper.SetAccount(s.ctx, existing)

	_, err := s.k.CreateVaultAccount(s.ctx, s.adminAddr.String(), shareDenom, underlying)
	s.Require().ErrorContains(err, "account at provlabs1etm9rr8jc5ej4clfhwhynh0vxgm6v26znv73u0 is not a vault account")
}

func (s *TestSuite) TestCreateVaultMarker_Success() {
	denom := "sharetoken"
	underlying := "coinbase"

	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlying, 1000), s.adminAddr)

	vault, err := s.k.CreateVaultAccount(s.ctx, s.adminAddr.String(), denom, underlying)
	s.Require().NoError(err)

	marker, err := s.k.CreateVaultMarker(s.ctx, vault.GetAddress(), denom, underlying)
	s.Require().NoError(err, "CreateVaultMarker should not error")
	s.Require().Equal(denom, marker.Denom, "denom mismatch")
	s.Require().Equal(types.GetVaultAddress(denom).String(), marker.Manager, "manager mismatch")
}

func (s *TestSuite) TestCreateVaultMarker_DuplicateDenomFails() {
	denom := "existingmarker"

	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(denom, 100), s.adminAddr)

	addr := types.GetVaultAddress(denom)
	_, err := s.k.CreateVaultMarker(s.ctx, addr, denom, "under")
	s.Require().ErrorContains(err, "a marker with the share denomination \"existingmarker\" already exists")
}
