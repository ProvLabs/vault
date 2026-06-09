package simapp

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	abci "github.com/cometbft/cometbft/abci/types"
	cmttypes "github.com/cometbft/cometbft/types"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/testutil/mock"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	"github.com/provenance-io/provenance/x/exchange"
)

// TestExchangePaymentTxWiring proves the SimApp processes exchange payment transactions
// end-to-end through BaseApp: signed MsgCreatePaymentRequest / MsgAcceptPaymentRequest
// transactions are delivered, which exercises ProvideExchangeCustomSigners and the app's
// signer-extraction path (the signer is nested inside the Payment field rather than being a
// top-level address). MsgCreatePaymentRequest is signed by the payment source and
// MsgAcceptPaymentRequest by the payment target; signing with any other key must be rejected
// by the ante handler. A broken nested-signer registration would fail these scenarios.
func TestExchangePaymentTxWiring(t *testing.T) {
	privVal := mock.NewPV()
	pubKey, err := privVal.GetPubKey()
	require.NoError(t, err, "GetPubKey")
	valSet := cmttypes.NewValidatorSet([]*cmttypes.Validator{cmttypes.NewValidator(pubKey, 1)})

	// funder is the genesis delegator that holds the bond denom.
	funderPriv := secp256k1.GenPrivKey()
	funder := authtypes.NewBaseAccount(funderPriv.PubKey().Address().Bytes(), funderPriv.PubKey(), 0, 0)
	funderBalance := banktypes.Balance{
		Address: funder.GetAddress().String(),
		Coins:   sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 100_000_000_000_000)),
	}

	// source and target are key-backed so their transactions can be signed and routed
	// through BaseApp, exercising the nested-signer extraction wired by ProvideExchangeCustomSigners.
	sourcePriv := secp256k1.GenPrivKey()
	sourceAcc := authtypes.NewBaseAccount(sourcePriv.PubKey().Address().Bytes(), sourcePriv.PubKey(), 0, 0)
	targetPriv := secp256k1.GenPrivKey()
	targetAcc := authtypes.NewBaseAccount(targetPriv.PubKey().Address().Bytes(), targetPriv.PubKey(), 0, 0)

	source := sourceAcc.GetAddress()
	target := targetAcc.GetAddress()

	sourceBalance := banktypes.Balance{Address: source.String(), Coins: sdk.NewCoins(sdk.NewInt64Coin("acorn", 100))}
	targetBalance := banktypes.Balance{Address: target.String(), Coins: sdk.NewCoins(sdk.NewInt64Coin("banana", 100))}

	app := SetupWithGenesisValSet(t, "", valSet,
		[]authtypes.GenesisAccount{funder, sourceAcc, targetAcc},
		funderBalance, sourceBalance, targetBalance,
	)
	// Commit genesis so signed transactions can be delivered in subsequent blocks.
	_, err = app.Commit()
	require.NoError(t, err, "commit genesis")

	payment := exchange.Payment{
		Source:       source.String(),
		SourceAmount: sdk.NewCoins(sdk.NewInt64Coin("acorn", 10)),
		Target:       target.String(),
		TargetAmount: sdk.NewCoins(sdk.NewInt64Coin("banana", 5)),
		ExternalId:   "wiring-test",
	}

	tests := []struct {
		name       string
		msg        sdk.Msg
		signer     sdk.AccAddress      // account whose number/sequence the tx is built against
		signKey    cryptotypes.PrivKey // key the tx is actually signed with
		expectPass bool
	}{
		{
			name:       "create payment rejected when not signed by the payment source",
			msg:        &exchange.MsgCreatePaymentRequest{Payment: payment},
			signer:     source,
			signKey:    targetPriv,
			expectPass: false,
		},
		{
			name:       "create payment accepted when signed by the payment source",
			msg:        &exchange.MsgCreatePaymentRequest{Payment: payment},
			signer:     source,
			signKey:    sourcePriv,
			expectPass: true,
		},
		{
			name:       "accept payment rejected when not signed by the payment target",
			msg:        &exchange.MsgAcceptPaymentRequest{Payment: payment},
			signer:     target,
			signKey:    sourcePriv,
			expectPass: false,
		},
		{
			name:       "accept payment accepted when signed by the payment target",
			msg:        &exchange.MsgAcceptPaymentRequest{Payment: payment},
			signer:     target,
			signKey:    targetPriv,
			expectPass: true,
		},
	}

	txCfg := app.TxConfig()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			signerAcc := app.AccountKeeper.GetAccount(app.NewContext(true), tc.signer)
			require.NotNilf(t, signerAcc, "signer account %s should exist for case %q", tc.signer, tc.name)

			tx, err := simtestutil.GenSignedMockTx(
				rand.New(rand.NewSource(1)),
				txCfg,
				[]sdk.Msg{tc.msg},
				sdk.NewCoins(),
				simtestutil.DefaultGenTxGas,
				"",
				[]uint64{signerAcc.GetAccountNumber()},
				[]uint64{signerAcc.GetSequence()},
				tc.signKey,
			)
			require.NoErrorf(t, err, "GenSignedMockTx for case %q", tc.name)

			bz, err := txCfg.TxEncoder()(tx)
			require.NoErrorf(t, err, "encode tx for case %q", tc.name)

			res, err := app.FinalizeBlock(&abci.RequestFinalizeBlock{
				Height: app.LastBlockHeight() + 1,
				Time:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				Txs:    [][]byte{bz},
			})
			require.NoErrorf(t, err, "FinalizeBlock for case %q", tc.name)
			require.Lenf(t, res.TxResults, 1, "expected one tx result for case %q", tc.name)

			txResult := res.TxResults[0]
			if tc.expectPass {
				require.Equalf(t, uint32(0), txResult.Code, "tx for case %q should succeed: %s", tc.name, txResult.Log)
			} else {
				require.NotEqualf(t, uint32(0), txResult.Code, "tx for case %q should be rejected by signer extraction", tc.name)
			}

			_, err = app.Commit()
			require.NoErrorf(t, err, "commit for case %q", tc.name)
		})
	}

	// After the accepted accept-payment transaction the funds have settled both ways.
	ctx := app.NewContext(true)
	require.Equal(t, int64(90), app.BankKeeper.GetBalance(ctx, source, "acorn").Amount.Int64(), "source acorn after settle")
	require.Equal(t, int64(5), app.BankKeeper.GetBalance(ctx, source, "banana").Amount.Int64(), "source banana after settle")
	require.Equal(t, int64(10), app.BankKeeper.GetBalance(ctx, target, "acorn").Amount.Int64(), "target acorn after settle")
	require.Equal(t, int64(95), app.BankKeeper.GetBalance(ctx, target, "banana").Amount.Int64(), "target banana after settle")
}
