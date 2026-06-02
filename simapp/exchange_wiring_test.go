package simapp

import (
	"testing"

	"github.com/stretchr/testify/require"

	cmttypes "github.com/cometbft/cometbft/types"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/cosmos-sdk/testutil/mock"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	"github.com/provenance-io/provenance/x/exchange"
	exchangekeeper "github.com/provenance-io/provenance/x/exchange/keeper"
	markertypes "github.com/provenance-io/provenance/x/marker/types"
)

// TestExchangePaymentTxWiring proves the SimApp can process exchange payment
// transactions end-to-end: the exchange keeper is wired with its store, the
// hold keeper (escrow), and the bank keeper.
func TestExchangePaymentTxWiring(t *testing.T) {
	privVal := mock.NewPV()
	pubKey, err := privVal.GetPubKey()
	require.NoError(t, err, "GetPubKey")
	valSet := cmttypes.NewValidatorSet([]*cmttypes.Validator{cmttypes.NewValidator(pubKey, 1)})

	senderPrivKey := secp256k1.GenPrivKey()
	acc := authtypes.NewBaseAccount(senderPrivKey.PubKey().Address().Bytes(), senderPrivKey.PubKey(), 0, 0)
	balance := banktypes.Balance{
		Address: acc.GetAddress().String(),
		Coins:   sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 100_000_000_000_000)),
	}
	app := SetupWithGenesisValSet(t, "", valSet, []authtypes.GenesisAccount{acc}, balance)

	ctx := app.NewContext(false)

	source := sdk.AccAddress("source______________")
	target := sdk.AccAddress("target______________")

	sourceFunds := sdk.NewCoins(sdk.NewInt64Coin("acorn", 100))
	targetFunds := sdk.NewCoins(sdk.NewInt64Coin("banana", 100))
	require.NoError(t, app.BankKeeper.MintCoins(ctx, markertypes.ModuleName, sourceFunds.Add(targetFunds...)), "MintCoins")
	require.NoError(t, app.BankKeeper.SendCoinsFromModuleToAccount(ctx, markertypes.ModuleName, source, sourceFunds), "fund source")
	require.NoError(t, app.BankKeeper.SendCoinsFromModuleToAccount(ctx, markertypes.ModuleName, target, targetFunds), "fund target")

	msgServer := exchangekeeper.NewMsgServer(app.ExchangeKeeper)

	payment := exchange.Payment{
		Source:       source.String(),
		SourceAmount: sdk.NewCoins(sdk.NewInt64Coin("acorn", 10)),
		Target:       target.String(),
		TargetAmount: sdk.NewCoins(sdk.NewInt64Coin("banana", 5)),
		ExternalId:   "wiring-test",
	}

	_, err = msgServer.CreatePayment(ctx, &exchange.MsgCreatePaymentRequest{Payment: payment})
	require.NoError(t, err, "CreatePayment")

	stored, err := app.ExchangeKeeper.GetPayment(ctx, source, "wiring-test")
	require.NoError(t, err, "GetPayment")
	require.NotNil(t, stored, "stored payment")

	_, err = msgServer.AcceptPayment(ctx, &exchange.MsgAcceptPaymentRequest{Payment: payment})
	require.NoError(t, err, "AcceptPayment")

	require.Equal(t, int64(90), app.BankKeeper.GetBalance(ctx, source, "acorn").Amount.Int64(), "source acorn after settle")
	require.Equal(t, int64(5), app.BankKeeper.GetBalance(ctx, source, "banana").Amount.Int64(), "source banana after settle")
	require.Equal(t, int64(10), app.BankKeeper.GetBalance(ctx, target, "acorn").Amount.Int64(), "target acorn after settle")
	require.Equal(t, int64(95), app.BankKeeper.GetBalance(ctx, target, "banana").Amount.Int64(), "target banana after settle")
}
