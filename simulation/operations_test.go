package simulation_test

import (
	"context"
	"math/rand"
	"testing"

	"github.com/provlabs/vault/simapp"
	"github.com/provlabs/vault/simulation"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
)

type VaultSimTestSuite struct {
	suite.Suite

	ctx sdk.Context
	app *simapp.SimApp
	cdc codec.Codec
}

func TestVaultSimTestSuite(t *testing.T) {
	suite.Run(t, new(VaultSimTestSuite))
}

func (s *VaultSimTestSuite) SetupTest() {
	s.app = simapp.Setup(s.T())
	s.ctx = s.app.BaseApp.NewContext(false)
	s.cdc = s.app.AppCodec()
}

func (s *VaultSimTestSuite) getTestingAccounts(r *rand.Rand, n int) []simtypes.Account {
	return GenerateTestingAccounts(s.T(), s.ctx, s.app, r, n)
}

func (s *VaultSimTestSuite) TestSimulateMsgCreateVault() {
	r := rand.New(rand.NewSource(1))
	accs := s.getTestingAccounts(r, 3)

	op := simulation.SimulateMsgCreateVault(*s.app.VaultKeeper)
	opMsg, futureOps, err := op(r, s.app.BaseApp, s.ctx, accs, "")
	s.Require().NoError(err, "SimulateMsgCreateVault")
	s.Require().True(opMsg.OK, "operationMsg.OK")
	s.Require().NotEmpty(opMsg.Name, "operationMsg.Name")
	s.Require().NotEmpty(opMsg.Route, "operationMsg.Route")
	s.Require().Len(futureOps, 0, "futureOperations")
}

func (s *VaultSimTestSuite) TestSimulateMsgSwapIn() {
	r := rand.New(rand.NewSource(1))
	accs := s.getTestingAccounts(r, 3)

	op := simulation.SimulateMsgSwapIn(*s.app.VaultKeeper)
	opMsg, futureOps, err := op(r, s.app.BaseApp, s.ctx, accs, "")
	s.Require().NoError(err, "SimulateMsgSwapIn")
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

	return bankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, addr, amounts)
}
