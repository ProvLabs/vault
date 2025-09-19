package simulation_test

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/simapp"
	"github.com/provlabs/vault/simulation"
	"github.com/provlabs/vault/types"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"

	markertypes "github.com/provenance-io/provenance/x/marker/types"
)

type VaultSimTestSuite struct {
	suite.Suite

	ctx    sdk.Context
	app    *simapp.SimApp
	cdc    codec.Codec
	accs   []simtypes.Account
	random *rand.Rand
}

func TestVaultSimTestSuite(t *testing.T) {
	suite.Run(t, new(VaultSimTestSuite))
}

func (s *VaultSimTestSuite) SetupTest() {
	s.app = simapp.Setup(s.T())
	s.ctx = s.app.BaseApp.NewContext(false)
	s.cdc = s.app.AppCodec()
	s.random = rand.New(rand.NewSource(1))
	s.setupEnv()
}

func (s *VaultSimTestSuite) setupEnv() {
	s.setupAccounts()
	s.setupMarkers()
	s.setupNavs()
	s.setupBalances()
}

func (s *VaultSimTestSuite) setupMarkers() {

}

func (s *VaultSimTestSuite) setupNavs() {

}

func (s *VaultSimTestSuite) setupBalances() {

}

func (s *VaultSimTestSuite) setupAccounts() {
	_ = s.getTestingAccounts(s.random, 3)
	s.accs = s.getTestingAccounts(s.random, 3)
}

func (s *VaultSimTestSuite) getTestingAccounts(r *rand.Rand, n int) []simtypes.Account {
	return GenerateTestingAccounts(s.T(), s.ctx, s.app, r, n)
}

func (s *VaultSimTestSuite) TestWeightedOperations() {
	sum := 0
	for _, w := range simulation.DefaultWeights {
		sum += w
	}

	s.Require().Equal(100, sum, "sum of simulation weights must be 100")
}

func (s *VaultSimTestSuite) TestSimulateMsgCreateVault() {
	op := simulation.SimulateMsgCreateVault(*s.app.VaultKeeper)

	opMsg, futureOps, err := op(s.random, s.app.BaseApp, s.ctx, s.accs, "")
	s.Require().NoError(err, "SimulateMsgCreateVault")
	s.Require().True(opMsg.OK, "operationMsg.OK")
	s.Require().NotEmpty(opMsg.Name, "operationMsg.Name")
	s.Require().NotEmpty(opMsg.Route, "operationMsg.Route")
	s.Require().Len(futureOps, 0, "futureOperations")
}

func (s *VaultSimTestSuite) TestSimulateMsgSwapIn() {
	selected := s.accs[0]

	err := simulation.CreateGlobalMarker(s.ctx, s.app, sdk.NewInt64Coin("underlying", 1000), s.accs)
	s.Require().NoError(err, "CreateGlobalMarker")
	err = simulation.CreateVault(s.ctx, s.app, "underlying", "underlyingshare", selected, s.accs)
	s.Require().NoError(err, "CreateVault")

	op := simulation.SimulateMsgSwapIn(*s.app.VaultKeeper)
	opMsg, futureOps, err := op(s.random, s.app.BaseApp, s.ctx, s.accs, "")
	s.Require().NoError(err, "SimulateMsgSwapIn")
	s.Require().True(opMsg.OK, "operationMsg.OK")
	s.Require().NotEmpty(opMsg.Name, "operationMsg.Name")
	s.Require().NotEmpty(opMsg.Route, "operationMsg.Route")
	s.Require().Len(futureOps, 0, "futureOperations")
}

func (s *VaultSimTestSuite) TestSimulateMsgSwapOut() {
	s.ctx = s.ctx.WithBlockTime(time.Now())
	selected := s.accs[0]

	err := simulation.CreateGlobalMarker(s.ctx, s.app, sdk.NewInt64Coin("underlying", 1000), s.accs)
	s.Require().NoError(err, "CreateGlobalMarker")
	err = simulation.CreateVault(s.ctx, s.app, "underlying", "underlyingshare", selected, s.accs)
	s.Require().NoError(err, "CreateVault")

	err = simulation.AddAttribute(s.ctx, selected.Address, simulation.RequiredMarkerAttribute, s.app.NameKeeper, s.app.AttributeKeeper)
	s.Require().NoError(err, "AddAttribute")

	// Swap in for shares
	swapIn := &types.MsgSwapInRequest{
		Owner:        selected.Address.String(),
		VaultAddress: types.GetVaultAddress("underlyingshare").String(),
		Assets:       sdk.NewInt64Coin("underlying", 100),
	}

	msgServer := keeper.NewMsgServer(s.app.VaultKeeper)
	resp, err := msgServer.SwapIn(s.ctx, swapIn)
	s.Require().NoError(err, "SwapOut")
	s.Require().NotNil(resp, "SwapOut Response not nil")

	op := simulation.SimulateMsgSwapOut(*s.app.VaultKeeper)
	opMsg, futureOps, err := op(s.random, s.app.BaseApp, s.ctx, s.accs, "")
	s.Require().NoError(err, "SimulateMsgSwapOut")
	s.Require().True(opMsg.OK, "operationMsg.OK")
	s.Require().NotEmpty(opMsg.Name, "operationMsg.Name")
	s.Require().NotEmpty(opMsg.Route, "operationMsg.Route")
	s.Require().Len(futureOps, 0, "futureOperations")
}

// GenerateTestingAccounts generates n new accounts, creates them (in state) and gives each 1 million power worth of bond tokens.
func GenerateTestingAccounts(t *testing.T, ctx sdk.Context, app *simapp.SimApp, r *rand.Rand, n int) []simtypes.Account {
	return GenerateTestingAccountsWithPower(t, ctx, app, r, n, 1_000_000)
}

// GenerateTestingAccountsWithPower generates n new accounts, creates them (in state) and gives each the provided power worth of bond tokens.
func GenerateTestingAccountsWithPower(t *testing.T, ctx sdk.Context, app *simapp.SimApp, r *rand.Rand, n int, power int64) []simtypes.Account {
	if n <= 0 {
		return nil
	}
	t.Helper()

	initAmt := sdk.TokensFromConsensusPower(power, sdk.DefaultPowerReduction)
	initCoins := sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, initAmt))

	accs := simtypes.RandomAccounts(r, n)
	// add coins to the accounts
	for i, account := range accs {
		acc := app.AccountKeeper.NewAccountWithAddress(ctx, account.Address)
		app.AccountKeeper.SetAccount(ctx, acc)
		err := FundAccount(ctx, app.BankKeeper, account.Address, initCoins)
		require.NoError(t, err, "[dd%d]: FundAccount", i)
	}
	return accs
}

func FundAccount(ctx context.Context, bankKeeper bankkeeper.Keeper, addr sdk.AccAddress, amounts sdk.Coins) error {
	if err := bankKeeper.MintCoins(ctx, minttypes.ModuleName, amounts); err != nil {
		return err
	}
	ctx = markertypes.WithBypass(ctx) // Bypass marker checks for this operation.
	return bankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, addr, amounts)
}
