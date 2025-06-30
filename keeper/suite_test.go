package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/simapp"
	suite "github.com/stretchr/testify/suite"
)

type TestSuite struct {
	suite.Suite
	simApp *simapp.SimApp
	ctx    sdk.Context

	k keeper.Keeper

	adminAddr sdk.AccAddress
}

func (s *TestSuite) SetupTest() {
	s.simApp = simapp.Setup(s.T())
	s.ctx = s.simApp.NewContext(false)
	s.k = *s.simApp.VaultKeeper

	s.adminAddr = sdk.AccAddress("adminAddr___________")
}

func (s *TestSuite) Context() sdk.Context {
	return s.ctx
}

func (s *TestSuite) SetContext(ctx sdk.Context) {
	s.ctx = ctx
}

func TestKeeperTestSuite(t *testing.T) {
	suite.Run(t, new(TestSuite))
}
