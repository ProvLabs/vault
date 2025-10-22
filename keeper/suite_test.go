package keeper_test

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"cosmossdk.io/math"
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
	"github.com/provlabs/vault/simulation"
	"github.com/provlabs/vault/types"
)

// TestSuite wires up a full SimApp and exposes helpers for keeper tests.
type TestSuite struct {
	suite.Suite
	simApp *simapp.SimApp
	ctx    sdk.Context

	k keeper.Keeper

	adminAddr sdk.AccAddress
}

// SetupTest initializes a new SimApp and context for each test and seeds
// commonly used test fixtures such as the vault keeper and an admin address.
func (s *TestSuite) SetupTest() {
	s.simApp = simapp.Setup(s.T())
	s.ctx = s.simApp.NewContext(false)
	s.k = *s.simApp.VaultKeeper

	s.adminAddr = sdk.AccAddress("adminAddr___________")
}

// Context returns the current sdk.Context associated with the suite.
func (s *TestSuite) Context() sdk.Context {
	return s.ctx
}

// SetContext replaces the current sdk.Context associated with the suite.
// Useful when a test needs to wrap the context with a new EventManager or
// modify block metadata mid-test.
func (s *TestSuite) SetContext(ctx sdk.Context) {
	s.ctx = ctx
}

// CreateAndFundAccount creates a fresh random account and funds it with the
// provided coin using the suite's bank keeper. It returns the new address.
func (s *TestSuite) CreateAndFundAccount(coin sdk.Coin) sdk.AccAddress {
	key2 := secp256k1.GenPrivKey()
	pub2 := key2.PubKey()
	addr2 := sdk.AccAddress(pub2.Address())
	FundAccount(s.ctx, s.simApp.BankKeeper, addr2, sdk.Coins{coin})
	return addr2
}

// FundAccount mints the provided coins to the mint module account and then
// sends them to the given address. This is a convenient way to seed balances
// in tests without requiring faucet-style logic.
func FundAccount(ctx context.Context, bankKeeper bankkeeper.Keeper, addr sdk.AccAddress, amounts sdk.Coins) error {
	if err := bankKeeper.MintCoins(ctx, minttypes.ModuleName, amounts); err != nil {
		return err
	}
	return bankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, addr, amounts)
}

// TestKeeperTestSuite is the entrypoint that runs the keeper TestSuite with testify.
func TestKeeperTestSuite(t *testing.T) {
	suite.Run(t, new(TestSuite))
}

// assertInPayoutVerificationQueue asserts whether a vault address is present in
// the payout verification set, matching the expectation flag.
func (s *TestSuite) assertInPayoutVerificationQueue(vaultAddr sdk.AccAddress, shouldContain bool) {
	isInQueue, err := s.k.PayoutVerificationSet.Has(s.ctx, vaultAddr)
	s.Require().NoError(err, "should not error checking queue")
	s.Assert().Equal(shouldContain, isInQueue, "vault should be enqueued in payout verification queue at expected period start")
}

// assertBalance asserts the balance for the provided address and denom equals
// the expected amount.
func (s *TestSuite) assertBalance(addr sdk.AccAddress, denom string, expectedAmt sdkmath.Int) {
	balance := s.simApp.BankKeeper.GetBalance(s.ctx, addr, denom)
	s.Assert().Equal(expectedAmt.String(), balance.Amount.String(), "unexpected balance for %s", addr.String())
}

// assertVaultAndMarkerBalances asserts both the vault account and its marker
// account have the expected balances for the provided denom.
func (s *TestSuite) assertVaultAndMarkerBalances(vaultAddr sdk.AccAddress, shareDenom string, denom string, expectedVaultAmt, expectedMarkerAmt sdkmath.Int) {
	markerAddr := markertypes.MustGetMarkerAddress(shareDenom)

	s.assertBalance(vaultAddr, denom, expectedVaultAmt)
	s.assertBalance(markerAddr, denom, expectedMarkerAmt)
}

// normalizeEvents trims surrounding quotes from event attribute values to make
// event comparison in tests resilient to JSON/string formatting differences.
func normalizeEvents(events sdk.Events) sdk.Events {
	for i := range events {
		for j := range events[i].Attributes {
			events[i].Attributes[j].Value = strings.Trim(events[i].Attributes[j].Value, `"`)
		}
	}
	return events
}

// requireAddFinalizeAndActivateMarker creates a restricted marker with the
// provided denom and supply, then finalizes and activates it. It fails the
// test immediately on any error.
func (s *TestSuite) requireAddFinalizeAndActivateMarker(coin sdk.Coin, manager sdk.AccAddress, reqAttrs ...string) {
	markerAddr, err := markertypes.MarkerAddress(coin.Denom)
	markerType := markertypes.MarkerType_Coin
	if len(reqAttrs) > 0 {
		markerType = markertypes.MarkerType_RestrictedCoin
	}
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
		MarkerType:             markerType,
		SupplyFixed:            true,
		AllowGovernanceControl: false,
		RequiredAttributes:     reqAttrs,
	}
	err = s.simApp.MarkerKeeper.AddFinalizeAndActivateMarker(s.ctx, marker)
	s.Require().NoError(err, "AddFinalizeAndActivateMarker(%s)", coin.Denom)
}

// CoinToJSON returns a stable JSON string representation of an sdk.Coin suitable
// for inclusion in event attribute comparisons.
func CoinToJSON(coin sdk.Coin) string {
	return fmt.Sprintf("{\"denom\":\"%s\",\"amount\":\"%s\"}", coin.Denom, coin.Amount.String())
}

// createReceiveCoinsEvents constructs the standard bank events emitted when a
// module/account receives and mints the specified amount.
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

// createSendCoinEvents constructs the standard bank events emitted for a transfer
// of the given amount from one address to another.
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

// setupBaseVaultRestricted creates a vault with a restricted underlying asset.
// It establishes a marker for the underlying asset, requiring a specific attribute for transfers.
// An optional paymentDenom can be provided for the vault's configuration.
// It returns the newly created vault account.
func (s *TestSuite) setupBaseVaultRestricted(underlyingDenom, shareDenom string, paymentDenom ...string) *types.VaultAccount {
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlyingDenom, 2_000_000), s.adminAddr, simulation.RequiredMarkerAttribute)

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

// createMarkerMintCoinEvents builds the expected event sequence for minting
// marker coins and sending them to a recipient.
func createMarkerMintCoinEvents(markerModule, admin, recipient sdk.AccAddress, coin sdk.Coin) []sdk.Event {
	events := createReceiveCoinsEvents(markerModule.String(), sdk.NewCoins(coin).String())

	sendEvents := createSendCoinEvents(markerModule.String(), recipient.String(), sdk.NewCoins(coin).String())
	events = append(events, sendEvents...)

	// The specific marker mint event
	markerMintEvent := sdk.NewEvent("provenance.marker.v1.EventMarkerMint",
		sdk.NewAttribute("administrator", admin.String()),
		sdk.NewAttribute("amount", coin.Amount.String()),
		sdk.NewAttribute("denom", coin.Denom),
	)
	events = append(events, markerMintEvent)

	return events
}

// createBurnCoinEvents builds the expected bank events for burning coins from a
// module account.
func createBurnCoinEvents(burner, amount string) []sdk.Event {
	events := sdk.NewEventManager().Events()

	events = events.AppendEvent(sdk.NewEvent(
		banktypes.EventTypeCoinSpent,
		sdk.NewAttribute(banktypes.AttributeKeySpender, burner),
		sdk.NewAttribute(sdk.AttributeKeyAmount, amount),
	))

	events = events.AppendEvent(sdk.NewEvent(
		banktypes.EventTypeCoinBurn,
		sdk.NewAttribute(banktypes.AttributeKeyBurner, burner),
		sdk.NewAttribute(sdk.AttributeKeyAmount, amount),
	))

	return events
}

// createMarkerWithdraw builds the expected event sequence for withdrawing shares
// from a marker to a recipient.
func createMarkerWithdraw(administrator, sender sdk.AccAddress, recipient sdk.AccAddress, shares sdk.Coin) []sdk.Event {
	events := createSendCoinEvents(sender.String(), recipient.String(), sdk.NewCoins(shares).String())

	// The specific marker withdraw event
	withdrawEvent := sdk.NewEvent("provenance.marker.v1.EventMarkerWithdraw",
		sdk.NewAttribute("administrator", administrator.String()),
		sdk.NewAttribute("coins", sdk.NewCoins(shares).String()),
		sdk.NewAttribute("denom", shares.Denom),
		sdk.NewAttribute("to_address", recipient.String()),
	)

	events = append(events, withdrawEvent)

	return events
}

// createMarkerBurn builds the expected event sequence for sending shares to the
// marker module and subsequently burning them.
func createMarkerBurn(admin, markerAddr sdk.AccAddress, shares sdk.Coin) []sdk.Event {
	markerModule := authtypes.NewModuleAddress(markertypes.ModuleName)
	events := createSendCoinEvents(markerAddr.String(), markerModule.String(), sdk.NewCoins(shares).String())

	burnEvents := createBurnCoinEvents(markerModule.String(), shares.String())
	events = append(events, burnEvents...)

	// The specific marker burn event
	markerBurnEvent := sdk.NewEvent("provenance.marker.v1.EventMarkerBurn",
		sdk.NewAttribute("administrator", admin.String()),
		sdk.NewAttribute("amount", shares.Amount.String()),
		sdk.NewAttribute("denom", shares.Denom),
	)
	events = append(events, markerBurnEvent)

	return events
}

// createSwapOutEvents builds the expected event sequence for a successful
// SwapOut request: escrow of shares followed by the module event.
func createSwapOutEvents(owner, vaultAddr sdk.AccAddress, assets, shares sdk.Coin) []sdk.Event {
	var allEvents []sdk.Event

	// 1. owner sends shares to vault address for escrow
	sendToMarkerEvents := createSendCoinEvents(owner.String(), vaultAddr.String(), shares.String())
	allEvents = append(allEvents, sendToMarkerEvents...)

	// 2. The vault's own SwapOut event
	swapOutEvent := sdk.NewEvent("provlabs.vault.v1.EventSwapOutRequested",
		sdk.NewAttribute("owner", owner.String()),
		sdk.NewAttribute("redeem_denom", assets.Denom),
		sdk.NewAttribute("request_id", "0"),
		sdk.NewAttribute("shares", shares.String()),
		sdk.NewAttribute("vault_address", vaultAddr.String()),
	)
	allEvents = append(allEvents, swapOutEvent)

	return allEvents
}

// createSwapInEvents builds the expected event sequence for a successful SwapIn:
// marker mint, withdraw to owner, underlying transfer in, and the module event.
func createSwapInEvents(owner, vaultAddr, markerAddr sdk.AccAddress, asset, shares sdk.Coin) []sdk.Event {
	var allEvents []sdk.Event

	markerModule := authtypes.NewModuleAddress(markertypes.ModuleName)
	mintEvents := createMarkerMintCoinEvents(markerModule, vaultAddr, markerAddr, shares)
	allEvents = append(allEvents, mintEvents...)

	withdrawEvents := createMarkerWithdraw(vaultAddr, markerAddr, owner, shares)
	allEvents = append(allEvents, withdrawEvents...)

	sendAssetEvents := createSendCoinEvents(owner.String(), markerAddr.String(), sdk.NewCoins(asset).String())
	allEvents = append(allEvents, sendAssetEvents...)

	swapInEvent := sdk.NewEvent("provlabs.vault.v1.EventSwapIn",
		sdk.NewAttribute("amount_in", asset.String()),
		sdk.NewAttribute("owner", owner.String()),
		sdk.NewAttribute("shares_received", shares.String()),
		sdk.NewAttribute("vault_address", vaultAddr.String()),
	)
	allEvents = append(allEvents, swapInEvent)

	return allEvents
}

// setupSinglePaymentDenomVault is a comprehensive helper that creates a vault with
// an underlying asset, a share denom, and a single payment denom. It creates all markers,
// withdraws funds to the admin, creates the vault with the paymentDenom configured,
// and sets a custom NAV for the payment denom to the underlying denom.
func (s *TestSuite) setupSinglePaymentDenomVault(underlyingDenom, shareDenom, paymentDenom string, price, volume int64) *types.VaultAccount {
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(paymentDenom, 2_000_000), s.adminAddr)
	s.k.MarkerKeeper.WithdrawCoins(s.ctx, s.adminAddr, s.adminAddr, paymentDenom, sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 100_000)))
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

// setReverseNAV sets a reverse net asset value on the underlying denom marker,
// allowing the vault to value the underlying in terms of the payment denom.
func (s *TestSuite) setReverseNAV(underlyingDenom, paymentDenom string, price, volume int64) {
	underlyingMarkerAddr := markertypes.MustGetMarkerAddress(underlyingDenom)
	underlyingMarkerAccount, err := s.k.MarkerKeeper.GetMarker(s.ctx, underlyingMarkerAddr)
	s.Require().NoError(err, "should fetch underlying marker for reverse NAV setup")
	s.Require().NoError(s.k.MarkerKeeper.SetNetAssetValue(s.ctx, underlyingMarkerAccount, markertypes.NetAssetValue{
		Price:  sdk.NewInt64Coin(paymentDenom, price),
		Volume: uint64(volume),
	}, "test-reverse"), "should set reverse NAV %s->%s=%d/%d", underlyingDenom, paymentDenom, price, volume)
}

// bumpHeight increments the suite's context block height by 1.
func (s *TestSuite) bumpHeight() {
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
}

// createBridgeMintSharesEventsExact returns the exact ordered events a successful
// BridgeMintShares emits: marker mint to the share marker, withdraw to the bridge,
// then the vault EventBridgeMintShares—suitable for strict equality checks in tests.
func createBridgeMintSharesEventsExact(vaultAddr, bridgeAddr sdk.AccAddress, shareDenom string, shares sdk.Coin) sdk.Events {
	markerAddr := markertypes.MustGetMarkerAddress(shareDenom)
	markerModuleAddr := authtypes.NewModuleAddress(markertypes.ModuleName)

	events := sdk.NewEventManager().Events()
	events = append(events, createMarkerMintCoinEvents(markerModuleAddr, vaultAddr, markerAddr, shares)...)
	events = append(events, createMarkerWithdraw(vaultAddr, markerAddr, bridgeAddr, shares)...)
	events = append(events, sdk.NewEvent(
		"provlabs.vault.v1.EventBridgeMintShares",
		sdk.NewAttribute("bridge", bridgeAddr.String()),
		sdk.NewAttribute("shares", shares.String()),
		sdk.NewAttribute("vault_address", vaultAddr.String()),
	))

	return events
}

// createMarkerSetNAV constructs the expected event emitted when a marker's NAV
func createMarkerSetNAV(shareDenom string, price sdk.Coin, source string, volume uint64) sdk.Event {
	return sdk.NewEvent(
		"provenance.marker.v1.EventSetNetAssetValue",
		sdk.NewAttribute("denom", shareDenom),
		sdk.NewAttribute("price", price.String()),
		sdk.NewAttribute("source", source),
		sdk.NewAttribute("volume", strconv.FormatUint(volume, 10)),
	)
}

// createReconcileEvents constructs the expected event sequence for a vault
// interest reconciliation, including any bank send events and the vault's
// own EventVaultReconcile.
func createReconcileEvents(vaultAddr, markerAddr sdk.AccAddress, interest, principle, principleAfter sdkmath.Int, denom, rate string, durations int64) []sdk.Event {
	var allEvents []sdk.Event

	r, err := sdkmath.LegacyNewDecFromStr(rate)
	if err != nil {
		panic(fmt.Sprintf("invalid rate %s: %v", rate, err))
	}
	var fromAddress string
	var toAddress string
	if r.IsNegative() {
		fromAddress = markerAddr.String()
		toAddress = vaultAddr.String()
	} else {
		fromAddress = vaultAddr.String()
		toAddress = markerAddr.String()
	}
	sendToMarkerEvents := createSendCoinEvents(fromAddress, toAddress, sdk.NewCoin(denom, interest.Abs()).String())
	allEvents = append(allEvents, sendToMarkerEvents...)
	interestEarned := sdk.Coin{Denom: denom, Amount: interest}
	reconcileEvent := sdk.NewEvent("provlabs.vault.v1.EventVaultReconcile",
		sdk.NewAttribute("interest_earned", interestEarned.String()),
		sdk.NewAttribute("principal_after", sdk.NewCoin(denom, principleAfter).String()),
		sdk.NewAttribute("principal_before", sdk.NewCoin(denom, principle).String()),
		sdk.NewAttribute("rate", rate),
		sdk.NewAttribute("time", fmt.Sprintf("%v", durations)),
		sdk.NewAttribute("vault_address", vaultAddr.String()),
	)
	allEvents = append(allEvents, reconcileEvent)
	return allEvents
}

// expectedWithSimpleAPY calculates the total amount (principal + interest)
// using a simple APY formula.
func expectedWithSimpleAPY(baseAmt math.Int, rateStr string, seconds int64) (math.Int, error) {
	rateDec, err := math.LegacyNewDecFromStr(rateStr)
	if err != nil {
		return math.Int{}, err
	}
	durationDec := math.LegacyNewDec(seconds)
	secondsPerYearDec := math.LegacyNewDec(31536000)
	timeFraction := durationDec.Quo(secondsPerYearDec)
	interestDec := baseAmt.ToLegacyDec().Mul(rateDec).Mul(timeFraction)
	return baseAmt.Add(interestDec.TruncateInt()), nil
}
