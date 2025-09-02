package keeper_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/cometbft/cometbft/crypto/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	markertypes "github.com/provenance-io/provenance/x/marker/types"
	suite "github.com/stretchr/testify/suite"

	"github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/simapp"
	"github.com/provlabs/vault/types"
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

// CreateAndFundAccount creates a new account in the app and funds it with the provided coin.
func (s *TestSuite) CreateAndFundAccount(coin sdk.Coin) sdk.AccAddress {
	key2 := secp256k1.GenPrivKey()
	pub2 := key2.PubKey()
	addr2 := sdk.AccAddress(pub2.Address())
	FundAccount(s.ctx, s.simApp.BankKeeper, addr2, sdk.Coins{coin})
	return addr2
}

func FundAccount(ctx context.Context, bankKeeper bankkeeper.Keeper, addr sdk.AccAddress, amounts sdk.Coins) error {
	if err := bankKeeper.MintCoins(ctx, minttypes.ModuleName, amounts); err != nil {
		return err
	}

	return bankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, addr, amounts)
}

func TestKeeperTestSuite(t *testing.T) {
	suite.Run(t, new(TestSuite))
}

func (s *TestSuite) assertInPayoutVerificationQueue(vaultAddr sdk.AccAddress, shouldContain bool) {
	isInQueue, err := s.k.PayoutVerificationQueue.Has(s.ctx, vaultAddr)
	s.Require().NoError(err, "should not error checking queue")
	s.Assert().Equal(shouldContain, isInQueue, "vault should be enqueued in payout verification queue at expected period start")
}

func (s *TestSuite) assertBalance(addr sdk.AccAddress, denom string, expectedAmt sdkmath.Int) {
	balance := s.simApp.BankKeeper.GetBalance(s.ctx, addr, denom)
	s.Assert().Equal(expectedAmt.String(), balance.Amount.String(), "unexpected balance for %s", addr.String())
}

func (s *TestSuite) assertVaultAndMarkerBalances(vaultAddr sdk.AccAddress, shareDenom string, denom string, expectedVaultAmt, expectedMarkerAmt sdkmath.Int) {
	markerAddr := markertypes.MustGetMarkerAddress(shareDenom)

	s.assertBalance(vaultAddr, denom, expectedVaultAmt)
	s.assertBalance(markerAddr, denom, expectedMarkerAmt)
}

func normalizeEvents(events sdk.Events) sdk.Events {
	for i := range events {
		for j := range events[i].Attributes {
			events[i].Attributes[j].Value = strings.Trim(events[i].Attributes[j].Value, `"`)
		}
	}
	return events
}

// requireAddFinalizeAndActivateMarker creates a restricted marker, requiring it to not error.
func (s *TestSuite) requireAddFinalizeAndActivateMarker(coin sdk.Coin, manager sdk.AccAddress, reqAttrs ...string) {
	markerAddr, err := markertypes.MarkerAddress(coin.Denom)
	s.Require().NoError(err, "MarkerAddress(%q)", coin.Denom)
	marker := &markertypes.MarkerAccount{
		BaseAccount: &authtypes.BaseAccount{Address: markerAddr.String()},
		Manager:     manager.String(),
		AccessControl: []markertypes.AccessGrant{
			{
				Address: manager.String(),
				Permissions: markertypes.AccessList{
					markertypes.Access_Mint, markertypes.Access_Burn,
					markertypes.Access_Deposit, markertypes.Access_Withdraw, markertypes.Access_Delete,
				},
			},
		},
		Status:                 markertypes.StatusProposed,
		Denom:                  coin.Denom,
		Supply:                 coin.Amount,
		MarkerType:             markertypes.MarkerType_Coin,
		SupplyFixed:            true,
		AllowGovernanceControl: false,
		RequiredAttributes:     reqAttrs,
	}
	err = s.simApp.MarkerKeeper.AddFinalizeAndActivateMarker(s.ctx, marker)
	s.Require().NoError(err, "AddFinalizeAndActivateMarker(%s)", coin.Denom)
}

func CoinToJSON(coin sdk.Coin) string {
	return fmt.Sprintf("{\"denom\":\"%s\",\"amount\":\"%s\"}", coin.Denom, coin.Amount.String())
}

func createReceiveCoinsEvents(fromAddress, amount string) sdk.Events {
	events := sdk.NewEventManager().Events()
	events = events.AppendEvent(sdk.NewEvent(
		banktypes.EventTypeCoinReceived,
		sdk.NewAttribute(banktypes.AttributeKeyReceiver, fromAddress),
		sdk.NewAttribute(sdk.AttributeKeyAmount, amount),
	))
	events = events.AppendEvent(sdk.NewEvent(
		banktypes.EventTypeCoinMint,
		sdk.NewAttribute(banktypes.AttributeKeyMinter, fromAddress),
		sdk.NewAttribute(sdk.AttributeKeyAmount, amount),
	))
	return events
}

func createSendCoinEvents(fromAddress, toAddress string, amount string) []sdk.Event {
	events := sdk.NewEventManager().Events()
	events = events.AppendEvent(sdk.NewEvent(
		banktypes.EventTypeCoinSpent,
		sdk.NewAttribute(banktypes.AttributeKeySpender, fromAddress),
		sdk.NewAttribute(sdk.AttributeKeyAmount, amount),
	))
	events = events.AppendEvent(sdk.NewEvent(
		banktypes.EventTypeCoinReceived,
		sdk.NewAttribute(banktypes.AttributeKeyReceiver, toAddress),
		sdk.NewAttribute(sdk.AttributeKeyAmount, amount),
	))
	events = events.AppendEvent(sdk.NewEvent(
		banktypes.EventTypeTransfer,
		sdk.NewAttribute(banktypes.AttributeKeyRecipient, toAddress),
		sdk.NewAttribute(banktypes.AttributeKeySender, fromAddress),
		sdk.NewAttribute(sdk.AttributeKeyAmount, amount),
	))
	events = events.AppendEvent(sdk.NewEvent(
		"message",
		sdk.NewAttribute(banktypes.AttributeKeySender, fromAddress),
	))

	return events
}

// setupBaseVault creates and activates markers for the underlying and share denoms,
// withdraws some underlying coins to the admin, and creates the vault.
// It can optionally accept a paymentDenom for the vault's configuration.
// It returns the newly created vault account.
func (s *TestSuite) setupBaseVault(underlyingDenom, shareDenom string, paymentDenom ...string) *types.VaultAccount {
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlyingDenom, 2_000_000), s.adminAddr)
	s.k.MarkerKeeper.WithdrawCoins(s.ctx, s.adminAddr, s.adminAddr, underlyingDenom, sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, 100_000)))

	var pDenom string
	if len(paymentDenom) > 0 {
		pDenom = paymentDenom[0]
	}

	vaultCfg := vaultAttrs{
		admin:      s.adminAddr.String(),
		share:      shareDenom,
		underlying: underlyingDenom,
		payment:    pDenom,
	}
	vault, err := s.k.CreateVault(s.ctx, vaultCfg)
	s.Require().NoError(err, "vault creation should succeed")
	return vault
}

// setupSinglePaymentDenomVault is a comprehensive helper that creates a vault with
// an underlying asset, a share denom, and a single payment denom. It creates all markers,
// withdraws funds to the admin, creates the vault with the paymentDenom configured,
// and sets a custom NAV for the payment denom to the underlying denom.
func (s *TestSuite) setupSinglePaymentDenomVault(underlyingDenom, shareDenom, paymentDenom string, price, volume int64) *types.VaultAccount {
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(paymentDenom, 2_000_000), s.adminAddr)
	s.k.MarkerKeeper.WithdrawCoins(s.ctx, s.adminAddr, s.adminAddr, paymentDenom, sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 100_000)))
	// Pass the paymentDenom here so the vault is created with it.
	vault := s.setupBaseVault(underlyingDenom, shareDenom, paymentDenom)

	paymentMarkerAddr := markertypes.MustGetMarkerAddress(paymentDenom)
	paymentMarkerAccount, err := s.k.MarkerKeeper.GetMarker(s.ctx, paymentMarkerAddr)
	s.Require().NoError(err, "should fetch payment marker for NAV setup")
	s.Require().NoError(s.k.MarkerKeeper.SetNetAssetValue(s.ctx, paymentMarkerAccount, markertypes.NetAssetValue{
		Price:  sdk.NewInt64Coin(underlyingDenom, price),
		Volume: uint64(volume),
	}, "test"), "should set NAV %s->%s=%d/%d", paymentDenom, underlyingDenom, price, volume)

	return vault
}
