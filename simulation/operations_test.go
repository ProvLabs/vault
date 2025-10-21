package simulation_test

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/provlabs/vault/simapp"
	"github.com/provlabs/vault/simulation"
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

	s.setupAccounts()

	err := simulation.CreateGlobalMarker(s.ctx, s.app.AccountKeeper, s.app.BankKeeper, s.app.MarkerKeeper, sdk.NewInt64Coin("underlying2vx", 100_000_000), s.accs, true)
	s.Require().NoError(err, "CreateGlobalMarker underlying")
	err = simulation.CreateGlobalMarker(s.ctx, s.app.AccountKeeper, s.app.BankKeeper, s.app.MarkerKeeper, sdk.NewInt64Coin("payment2vx", 100_000_000), s.accs, true)
	s.Require().NoError(err, "CreateGlobalMarker payment")
}

func (s *VaultSimTestSuite) setupAccounts() {
	_ = s.getTestingAccounts(s.random, 5)
	s.accs = s.getTestingAccounts(s.random, 5)
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

	err := simulation.CreateVault(s.ctx, s.app.VaultKeeper, s.app.AccountKeeper, s.app.BankKeeper, s.app.MarkerKeeper, "underlying2vx", "", "underlyingshare", selected, s.accs)
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

	err := simulation.CreateVault(s.ctx, s.app.VaultKeeper, s.app.AccountKeeper, s.app.BankKeeper, s.app.MarkerKeeper, "underlying2vx", "", "underlyingshare", selected, s.accs)
	s.Require().NoError(err, "CreateVault")
	err = simulation.AddAttribute(s.ctx, selected.Address, simulation.RequiredMarkerAttribute, s.app.NameKeeper, s.app.AttributeKeeper)
	s.Require().NoError(err, "AddAttribute")
	err = simulation.SwapIn(s.ctx, s.app.VaultKeeper, selected, "underlyingshare", sdk.NewInt64Coin("underlying2vx", 100))
	s.Require().NoError(err, "SwapIn")

	op := simulation.SimulateMsgSwapOut(*s.app.VaultKeeper)
	opMsg, futureOps, err := op(s.random, s.app.BaseApp, s.ctx, s.accs, "")
	s.Require().NoError(err, "SimulateMsgSwapOut")
	s.Require().True(opMsg.OK, "operationMsg.OK")
	s.Require().NotEmpty(opMsg.Name, "operationMsg.Name")
	s.Require().NotEmpty(opMsg.Route, "operationMsg.Route")
	s.Require().Len(futureOps, 0, "futureOperations")
}

func (s *VaultSimTestSuite) TestSimulateMsgToggleSwapIn() {
	selected := s.accs[0]

	err := simulation.CreateVault(s.ctx, s.app.VaultKeeper, s.app.AccountKeeper, s.app.BankKeeper, s.app.MarkerKeeper, "underlying2vx", "", "underlyingshare", selected, s.accs)
	s.Require().NoError(err, "CreateVault")

	op := simulation.SimulateMsgToggleSwapIn(*s.app.VaultKeeper)
	opMsg, futureOps, err := op(s.random, s.app.BaseApp, s.ctx, s.accs, "")
	s.Require().NoError(err, "SimulateMsgToggleSwapIn")
	s.Require().True(opMsg.OK, "operationMsg.OK")
	s.Require().NotEmpty(opMsg.Name, "operationMsg.Name")
	s.Require().NotEmpty(opMsg.Route, "operationMsg.Route")
	s.Require().Len(futureOps, 0, "futureOperations")
}

func (s *VaultSimTestSuite) TestSimulateMsgToggleSwapOut() {
	selected := s.accs[0]

	err := simulation.CreateVault(s.ctx, s.app.VaultKeeper, s.app.AccountKeeper, s.app.BankKeeper, s.app.MarkerKeeper, "underlying2vx", "", "underlyingshare", selected, s.accs)
	s.Require().NoError(err, "CreateVault")

	op := simulation.SimulateMsgToggleSwapOut(*s.app.VaultKeeper)
	opMsg, futureOps, err := op(s.random, s.app.BaseApp, s.ctx, s.accs, "")
	s.Require().NoError(err, "SimulateMsgToggleSwapOut")
	s.Require().True(opMsg.OK, "operationMsg.OK")
	s.Require().NotEmpty(opMsg.Name, "operationMsg.Name")
	s.Require().NotEmpty(opMsg.Route, "operationMsg.Route")
	s.Require().Len(futureOps, 0, "futureOperations")
}

func (s *VaultSimTestSuite) TestSimulateMsgPauseVault() {
	selected := s.accs[0]

	err := simulation.CreateVault(s.ctx, s.app.VaultKeeper, s.app.AccountKeeper, s.app.BankKeeper, s.app.MarkerKeeper, "underlying2vx", "", "underlyingshare", selected, s.accs)
	s.Require().NoError(err, "CreateVault")

	op := simulation.SimulateMsgPauseVault(*s.app.VaultKeeper)
	opMsg, futureOps, err := op(s.random, s.app.BaseApp, s.ctx, s.accs, "")
	s.Require().NoError(err, "SimulateMsgPauseVault")
	s.Require().True(opMsg.OK, "operationMsg.OK")
	s.Require().NotEmpty(opMsg.Name, "operationMsg.Name")
	s.Require().NotEmpty(opMsg.Route, "operationMsg.Route")
	s.Require().Len(futureOps, 0, "futureOperations")
}

func (s *VaultSimTestSuite) TestSimulateMsgUnpauseVault() {
	selected := s.accs[0]

	err := simulation.CreateVault(s.ctx, s.app.VaultKeeper, s.app.AccountKeeper, s.app.BankKeeper, s.app.MarkerKeeper, "underlying2vx", "", "underlyingshare", selected, s.accs)
	s.Require().NoError(err, "CreateVault")
	err = simulation.PauseVault(s.ctx, s.app.VaultKeeper, "underlyingshare")
	s.Require().NoError(err, "PauseVault")

	op := simulation.SimulateMsgUnpauseVault(*s.app.VaultKeeper)
	opMsg, futureOps, err := op(s.random, s.app.BaseApp, s.ctx, s.accs, "")
	s.Require().NoError(err, "SimulateMsgUnpauseVault")
	s.Require().True(opMsg.OK, "operationMsg.OK")
	s.Require().NotEmpty(opMsg.Name, "operationMsg.Name")
	s.Require().NotEmpty(opMsg.Route, "operationMsg.Route")
	s.Require().Len(futureOps, 0, "futureOperations")
}

func (s *VaultSimTestSuite) TestSimulateMsgExpeditePendingSwapOut() {
	s.ctx = s.ctx.WithBlockTime(time.Now())
	user := s.accs[0]
	admin := s.accs[1]

	err := simulation.CreateVault(s.ctx, s.app.VaultKeeper, s.app.AccountKeeper, s.app.BankKeeper, s.app.MarkerKeeper, "underlying2vx", "", "underlyingshare", admin, s.accs)
	s.Require().NoError(err, "CreateVault")
	err = simulation.AddAttribute(s.ctx, user.Address, simulation.RequiredMarkerAttribute, s.app.NameKeeper, s.app.AttributeKeeper)
	s.Require().NoError(err, "AddAttribute")
	err = simulation.SwapIn(s.ctx, s.app.VaultKeeper, user, "underlyingshare", sdk.NewInt64Coin("underlying2vx", 100))
	s.Require().NoError(err, "SwapIn")
	shares := s.app.BankKeeper.GetBalance(s.ctx, user.Address, "underlyingshare")
	err = simulation.SwapOut(s.ctx, s.app.VaultKeeper, user, shares, "underlying2vx")
	s.Require().NoError(err, "SwapOut")

	// Now run the simulation for Expedite
	op := simulation.SimulateMsgExpeditePendingSwapOut(*s.app.VaultKeeper)
	opMsg, futureOps, err := op(s.random, s.app.BaseApp, s.ctx, s.accs, "")
	s.Require().NoError(err, "SimulateMsgExpediteSwapOut")
	s.Require().True(opMsg.OK, "operationMsg.OK")
	s.Require().NotEmpty(opMsg.Name, "operationMsg.Name")
	s.Require().NotEmpty(opMsg.Route, "operationMsg.Route")
	s.Require().Len(futureOps, 0, "futureOperations")
}

func (s *VaultSimTestSuite) TestSimulateMsgDepositInterestFunds() {
	admin := s.accs[0]

	err := simulation.CreateVault(s.ctx, s.app.VaultKeeper, s.app.AccountKeeper, s.app.BankKeeper, s.app.MarkerKeeper, "underlying2vx", "", "underlyingshare", admin, s.accs)
	s.Require().NoError(err, "CreateVault")

	op := simulation.SimulateMsgDepositInterestFunds(*s.app.VaultKeeper)
	opMsg, futureOps, err := op(s.random, s.app.BaseApp, s.ctx, s.accs, "")
	s.Require().NoError(err, "SimulateMsgDepositInterest")
	s.Require().True(opMsg.OK, "operationMsg.OK")
	s.Require().NotEmpty(opMsg.Name, "operationMsg.Name")
	s.Require().NotEmpty(opMsg.Route, "operationMsg.Route")
	s.Require().Len(futureOps, 0, "futureOperations")
}

func (s *VaultSimTestSuite) TestSimulateMsgDepositPrincipalFunds() {
	admin := s.accs[0]

	err := simulation.CreateVault(s.ctx, s.app.VaultKeeper, s.app.AccountKeeper, s.app.BankKeeper, s.app.MarkerKeeper, "underlying2vx", "", "underlyingshare", admin, s.accs)
	s.Require().NoError(err, "CreateVault")
	err = simulation.PauseVault(s.ctx, s.app.VaultKeeper, "underlyingshare")
	s.Require().NoError(err, "PauseVault")

	op := simulation.SimulateMsgDepositPrincipalFunds(*s.app.VaultKeeper)
	opMsg, futureOps, err := op(s.random, s.app.BaseApp, s.ctx, s.accs, "")
	s.Require().NoError(err, "SimulateMsgDepositPrincipal")
	s.Require().True(opMsg.OK, "operationMsg.OK")
	s.Require().NotEmpty(opMsg.Name, "operationMsg.Name")
	s.Require().NotEmpty(opMsg.Route, "operationMsg.Route")
	s.Require().Len(futureOps, 0, "futureOperations")
}

func (s *VaultSimTestSuite) TestSimulateMsgWithdrawInterestFunds() {
	admin := s.accs[0]

	err := simulation.CreateVault(s.ctx, s.app.VaultKeeper, s.app.AccountKeeper, s.app.BankKeeper, s.app.MarkerKeeper, "underlying2vx", "", "underlyingshare", admin, s.accs)
	s.Require().NoError(err, "CreateVault")
	_, err = simulation.DepositInterestFunds(s.ctx, s.app.VaultKeeper, "underlyingshare", sdk.NewInt64Coin("underlying2vx", 100))
	s.Require().NoError(err, "DepositInterest")
	err = simulation.AddAttribute(s.ctx, admin.Address, simulation.RequiredMarkerAttribute, s.app.NameKeeper, s.app.AttributeKeeper)
	s.Require().NoError(err, "AddAttribute")

	op := simulation.SimulateMsgWithdrawInterestFunds(*s.app.VaultKeeper)
	opMsg, futureOps, err := op(s.random, s.app.BaseApp, s.ctx, s.accs, "")
	s.Require().NoError(err, "SimulateMsgWithdrawInterest")
	s.Require().True(opMsg.OK, "operationMsg.OK")
	s.Require().NotEmpty(opMsg.Name, "operationMsg.Name")
	s.Require().NotEmpty(opMsg.Route, "operationMsg.Route")
	s.Require().Len(futureOps, 0, "futureOperations")
}

func (s *VaultSimTestSuite) TestSimulateMsgWithdrawPrincipalFunds() {
	admin := s.accs[0]

	err := simulation.CreateVault(s.ctx, s.app.VaultKeeper, s.app.AccountKeeper, s.app.BankKeeper, s.app.MarkerKeeper, "underlying2vx", "", "underlyingshare", admin, s.accs)
	s.Require().NoError(err, "CreateVault")
	err = simulation.PauseVault(s.ctx, s.app.VaultKeeper, "underlyingshare")
	s.Require().NoError(err, "PauseVault")
	_, err = simulation.DepositPrincipalFunds(s.ctx, s.app.VaultKeeper, "underlyingshare", sdk.NewInt64Coin("underlying2vx", 100))
	s.Require().NoError(err, "DepositPrincipal")
	err = simulation.AddAttribute(s.ctx, admin.Address, simulation.RequiredMarkerAttribute, s.app.NameKeeper, s.app.AttributeKeeper)
	s.Require().NoError(err, "AddAttribute")

	op := simulation.SimulateMsgWithdrawPrincipalFunds(*s.app.VaultKeeper)
	opMsg, futureOps, err := op(s.random, s.app.BaseApp, s.ctx, s.accs, "")
	s.Require().NoError(err, "SimulateMsgWithdrawPrincipal")
	s.Require().True(opMsg.OK, "operationMsg.OK")
	s.Require().NotEmpty(opMsg.Name, "operationMsg.Name")
	s.Require().NotEmpty(opMsg.Route, "operationMsg.Route")
	s.Require().Len(futureOps, 0, "futureOperations")
}

func (s *VaultSimTestSuite) TestSimulateMsgUpdateInterestRate() {
	selected := s.accs[0]

	err := simulation.CreateVault(s.ctx, s.app.VaultKeeper, s.app.AccountKeeper, s.app.BankKeeper, s.app.MarkerKeeper, "underlying2vx", "", "underlyingshare", selected, s.accs)
	s.Require().NoError(err, "CreateVault")

	op := simulation.SimulateMsgUpdateInterestRate(*s.app.VaultKeeper)
	opMsg, futureOps, err := op(s.random, s.app.BaseApp, s.ctx, s.accs, "")
	s.Require().NoError(err, "SimulateMsgUpdateInterestRate")
	s.Require().True(opMsg.OK, "operationMsg.OK")
	s.Require().NotEmpty(opMsg.Name, "operationMsg.Name")
	s.Require().NotEmpty(opMsg.Route, "operationMsg.Route")
	s.Require().Len(futureOps, 0, "futureOperations")
}

func (s *VaultSimTestSuite) TestSimulateMsgUpdateMinInterestRate() {
	selected := s.accs[0]

	err := simulation.CreateVault(s.ctx, s.app.VaultKeeper, s.app.AccountKeeper, s.app.BankKeeper, s.app.MarkerKeeper, "underlying2vx", "", "underlyingshare", selected, s.accs)
	s.Require().NoError(err, "CreateVault")

	op := simulation.SimulateMsgUpdateMinInterestRate(*s.app.VaultKeeper)
	opMsg, futureOps, err := op(s.random, s.app.BaseApp, s.ctx, s.accs, "")
	s.Require().NoError(err, "SimulateMsgUpdateMinInterestRate")
	s.Require().True(opMsg.OK, "operationMsg.OK")
	s.Require().NotEmpty(opMsg.Name, "operationMsg.Name")
	s.Require().NotEmpty(opMsg.Route, "operationMsg.Route")
	s.Require().Len(futureOps, 0, "futureOperations")
}

func (s *VaultSimTestSuite) TestSimulateMsgUpdateMaxInterestRate() {
	selected := s.accs[0]

	err := simulation.CreateVault(s.ctx, s.app.VaultKeeper, s.app.AccountKeeper, s.app.BankKeeper, s.app.MarkerKeeper, "underlying2vx", "", "underlyingshare", selected, s.accs)
	s.Require().NoError(err, "CreateVault")

	op := simulation.SimulateMsgUpdateMaxInterestRate(*s.app.VaultKeeper)
	opMsg, futureOps, err := op(s.random, s.app.BaseApp, s.ctx, s.accs, "")
	s.Require().NoError(err, "SimulateMsgUpdateMaxInterestRate")
	s.Require().True(opMsg.OK, "operationMsg.OK")
	s.Require().NotEmpty(opMsg.Name, "operationMsg.Name")
	s.Require().NotEmpty(opMsg.Route, "operationMsg.Route")
	s.Require().Len(futureOps, 0, "futureOperations")
}

func (s *VaultSimTestSuite) TestSimulateMsgToggleBridge() {
	selected := s.accs[0]

	err := simulation.CreateVault(s.ctx, s.app.VaultKeeper, s.app.AccountKeeper, s.app.BankKeeper, s.app.MarkerKeeper, "underlying2vx", "", "underlyingshare", selected, s.accs)
	s.Require().NoError(err, "CreateVault")

	err = simulation.SetVaultBridge(s.ctx, s.app.VaultKeeper, "underlyingshare", sdk.AccAddress(""), true)
	s.Require().NoError(err, "SetVaultBridge")

	op := simulation.SimulateMsgToggleBridge(*s.app.VaultKeeper)
	opMsg, futureOps, err := op(s.random, s.app.BaseApp, s.ctx, s.accs, "")
	s.Require().NoError(err, "SimulateMsgToggleBridge")
	s.Require().True(opMsg.OK, "operationMsg.OK")
	s.Require().NotEmpty(opMsg.Name, "operationMsg.Name")
	s.Require().NotEmpty(opMsg.Route, "operationMsg.Route")
	s.Require().Len(futureOps, 0, "futureOperations")
}

func (s *VaultSimTestSuite) TestSimulateMsgSetBridgeAddress() {
	selected := s.accs[0]

	err := simulation.CreateVault(s.ctx, s.app.VaultKeeper, s.app.AccountKeeper, s.app.BankKeeper, s.app.MarkerKeeper, "underlying2vx", "", "underlyingshare", selected, s.accs)
	s.Require().NoError(err, "CreateVault")

	op := simulation.SimulateMsgSetBridgeAddress(*s.app.VaultKeeper)
	opMsg, futureOps, err := op(s.random, s.app.BaseApp, s.ctx, s.accs, "")
	s.Require().NoError(err, "SimulateMsgSetBridgeAddress")
	s.Require().True(opMsg.OK, "operationMsg.OK")
	s.Require().NotEmpty(opMsg.Name, "operationMsg.Name")
	s.Require().NotEmpty(opMsg.Route, "operationMsg.Route")
	s.Require().Len(futureOps, 0, "futureOperations")
}

func (s *VaultSimTestSuite) TestSimulateMsgBridgeMintShares() {
	admin := s.accs[0]
	bridge := s.accs[1]

	err := simulation.CreateVault(s.ctx, s.app.VaultKeeper, s.app.AccountKeeper, s.app.BankKeeper, s.app.MarkerKeeper, "underlying2vx", "", "underlyingshare", admin, s.accs)
	s.Require().NoError(err, "CreateVault")

	err = simulation.BridgeAssets(s.ctx, s.app.VaultKeeper, "underlyingshare", sdk.NewInt64Coin("underlyingshare", 1000), sdk.NewInt64Coin("underlyingshare", 500))
	s.Require().NoError(err, "BridgeAssets")

	err = simulation.UpdateVaultTotalShares(s.ctx, s.app.VaultKeeper, sdk.NewInt64Coin("underlyingshare", 1000))
	s.Require().NoError(err, "UpdateVaultTotalShares")

	err = simulation.SetVaultBridge(s.ctx, s.app.VaultKeeper, "underlyingshare", bridge.Address, true)
	s.Require().NoError(err, "SetVaultBridge")

	op := simulation.SimulateMsgBridgeMintShares(*s.app.VaultKeeper)
	opMsg, futureOps, err := op(s.random, s.app.BaseApp, s.ctx, s.accs, "")
	s.Require().NoError(err, "SimulateMsgBridgeMintShares")
	s.Require().True(opMsg.OK, "operationMsg.OK")
	s.Require().NotEmpty(opMsg.Name, "operationMsg.Name")
	s.Require().NotEmpty(opMsg.Route, "operationMsg.Route")
	s.Require().Len(futureOps, 0, "futureOperations")
}

func (s *VaultSimTestSuite) TestSimulateMsgBridgeBurnShares() {
	admin := s.accs[0]
	bridge := s.accs[1]

	err := simulation.CreateVault(s.ctx, s.app.VaultKeeper, s.app.AccountKeeper, s.app.BankKeeper, s.app.MarkerKeeper, "underlying2vx", "", "underlyingshare", admin, s.accs)
	s.Require().NoError(err, "CreateVault")

	err = simulation.SetVaultBridge(s.ctx, s.app.VaultKeeper, "underlyingshare", bridge.Address, true)
	s.Require().NoError(err, "SetVaultBridge")

	err = simulation.UpdateVaultTotalShares(s.ctx, s.app.VaultKeeper, sdk.NewInt64Coin("underlyingshare", 1000))
	s.Require().NoError(err, "UpdateVaultTotalShares")

	// give bridge account some shares
	shares := sdk.NewInt64Coin("underlyingshare", 1000)
	err = FundAccount(s.ctx, s.app.BankKeeper, bridge.Address, sdk.NewCoins(shares))
	s.Require().NoError(err, "FundAccount for bridge")

	op := simulation.SimulateMsgBridgeBurnShares(*s.app.VaultKeeper)
	opMsg, futureOps, err := op(s.random, s.app.BaseApp, s.ctx, s.accs, "")
	s.Require().NoError(err, "SimulateMsgBridgeBurnShares")
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

