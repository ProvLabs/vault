package keeper_test

import (
	"fmt"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/simapp"
	"github.com/provlabs/vault/types"
	suite "github.com/stretchr/testify/suite"
)

type TestSuite struct {
	suite.Suite
	simApp *simapp.SimApp
	ctx    sdk.Context

	k keeper.Keeper
}

func (s *TestSuite) SetupTest() {
	s.simApp = simapp.Setup(s.T())
	s.ctx = s.simApp.NewContext(false)
	s.k = *s.simApp.VaultKeeper

}

func TestKeeperTestSuite(t *testing.T) {
	suite.Run(t, new(TestSuite))
}

func (s *TestSuite) TestMsgServer_CreateVault() {
	s.T().Helper()
	origCtx := s.ctx
	defer func() {
		s.ctx = origCtx
	}()
	s.ctx, _ = s.ctx.CacheContext()
	em := sdk.NewEventManager()
	s.ctx = s.ctx.WithEventManager(em)

	msg_server := keeper.NewMsgServer(s.simApp.VaultKeeper)

	response, err := msg_server.CreateVault(s.ctx, &types.MsgCreateVaultRequest{})
	if err != nil {
		panic(err)
	}
	fmt.Printf("%v", response)
}
