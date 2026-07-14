package keeper_test

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/crypto/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/cosmos/gogoproto/proto"
	attrtypes "github.com/provenance-io/provenance/x/attribute/types"
	"github.com/provenance-io/provenance/x/exchange"
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
	// assetManagerAddr is the asset manager assigned by setupAssetSettlementVault; settlement
	// messages (AcceptAsset/RejectAsset) must be signed by it, never by the admin.
	assetManagerAddr sdk.AccAddress
}

// SetupTest initializes a new SimApp and context for each test and seeds
// commonly used test fixtures such as the vault keeper and an admin address.
func (s *TestSuite) SetupTest() {
	s.simApp = simapp.Setup(s.T())
	s.ctx = s.simApp.NewContext(false).WithBlockTime(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	s.k = *s.simApp.VaultKeeper

	s.adminAddr = sdk.AccAddress("adminAddr___________")
	if !s.simApp.AccountKeeper.HasAccount(s.ctx, s.adminAddr) {
		s.simApp.AccountKeeper.SetAccount(s.ctx, s.simApp.AccountKeeper.NewAccountWithAddress(s.ctx, s.adminAddr))
	}

	s.assetManagerAddr = sdk.AccAddress("assetManagerAddr____")
	if !s.simApp.AccountKeeper.HasAccount(s.ctx, s.assetManagerAddr) {
		s.simApp.AccountKeeper.SetAccount(s.ctx, s.simApp.AccountKeeper.NewAccountWithAddress(s.ctx, s.assetManagerAddr))
	}
}

// EnsureTechFeeAccount ensures that the AUM fee address account exists in the account keeper.
func (s *TestSuite) EnsureTechFeeAccount() sdk.AccAddress {
	provlabsAddr, err := s.k.GetAUMFeeAddress(s.ctx)
	s.Require().NoError(err, "failed to get AUM fee address")
	if !s.simApp.AccountKeeper.HasAccount(s.ctx, provlabsAddr) {
		s.simApp.AccountKeeper.SetAccount(s.ctx, s.simApp.AccountKeeper.NewAccountWithAddress(s.ctx, provlabsAddr))
	}
	return provlabsAddr
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

// countingBankKeeper wraps a types.BankKeeper and records how often the
// balance-lookup methods are called. It lets a test assert that a keeper path
// stays bounded to targeted GetBalance lookups over a known denom set and never
// falls back to the unbounded GetAllBalances walk. This is the regression guard
// for GetTVVInUnderlyingAsset, whose cost must scale with the denoms the vault
// actually values rather than everything parked at the principal marker.
//
// Only the two lookup methods are overridden; every other BankKeeper method is
// promoted from the embedded interface and delegates to the real keeper.
type countingBankKeeper struct {
	types.BankKeeper
	getAllBalancesCalls int
	getBalanceCalls     int
}

// GetAllBalances records the call before delegating to the wrapped keeper.
func (c *countingBankKeeper) GetAllBalances(ctx context.Context, addr sdk.AccAddress) sdk.Coins {
	c.getAllBalancesCalls++
	return c.BankKeeper.GetAllBalances(ctx, addr)
}

// GetBalance records the call before delegating to the wrapped keeper.
func (c *countingBankKeeper) GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin {
	c.getBalanceCalls++
	return c.BankKeeper.GetBalance(ctx, addr, denom)
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
		events[i] = normalizeEvent(events[i])
	}
	return events
}

// normalizeEvent trims surrounding quotes from event attribute values.
func normalizeEvent(event sdk.Event) sdk.Event {
	for i := range event.Attributes {
		event.Attributes[i].Value = strings.Trim(event.Attributes[i].Value, `"`)
	}
	return event
}

// SetupTechFeeAccount ensures the AUM fee collector account exists and has the required
// attributes to receive the specified restricted asset. It returns the fee collector address.
func (s *TestSuite) SetupTechFeeAccount(restrictedUnderlyingDenom string) sdk.AccAddress {
	provlabsAddr, err := s.k.GetAUMFeeAddress(s.ctx)
	s.Require().NoError(err, "failed to get AUM fee address")
	if !s.simApp.AccountKeeper.HasAccount(s.ctx, provlabsAddr) {
		s.simApp.AccountKeeper.SetAccount(s.ctx, s.simApp.AccountKeeper.NewAccountWithAddress(s.ctx, provlabsAddr))
	}

	if !s.simApp.NameKeeper.NameExists(s.ctx, restrictedUnderlyingDenom) {
		s.Require().NoError(s.simApp.NameKeeper.SetNameRecord(s.ctx, restrictedUnderlyingDenom, s.adminAddr, false), "should successfully bind name for %s", restrictedUnderlyingDenom)
	}
	expireTime := time.Now().Add(24 * time.Hour)
	attr := attrtypes.NewAttribute(restrictedUnderlyingDenom, provlabsAddr.String(), attrtypes.AttributeType_String, []byte("true"), &expireTime, "")
	s.Require().NoError(s.simApp.AttributeKeeper.SetAttribute(s.ctx, attr, s.adminAddr), "should successfully set attribute for tech fee account")

	return provlabsAddr
}

// requireAddFinalizeAndActivateMarker creates a restricted marker with the
// provided denom and supply, then finalizes and activates it. It fails the
// test immediately on any error.
func (s *TestSuite) requireAddFinalizeAndActivateMarker(coin sdk.Coin, manager sdk.AccAddress, reqAttrs ...string) {
	if m, _ := s.simApp.MarkerKeeper.GetMarkerByDenom(s.ctx, coin.Denom); m != nil {
		return
	}
	markerAddr, err := markertypes.MarkerAddress(coin.Denom)
	markerType := markertypes.MarkerType_Coin
	if len(reqAttrs) > 0 {
		markerType = markertypes.MarkerType_RestrictedCoin
	}
	s.Require().NoError(err, "MarkerAddress(%q)", coin.Denom)

	// Ensure the tech fee and admin accounts exist
	if !s.simApp.AccountKeeper.HasAccount(s.ctx, s.adminAddr) {
		s.simApp.AccountKeeper.SetAccount(s.ctx, s.simApp.AccountKeeper.NewAccountWithAddress(s.ctx, s.adminAddr))
	}
	provlabsAddr, err := s.k.GetAUMFeeAddress(s.ctx)
	s.Require().NoError(err, "failed to get AUM fee address")
	if !s.simApp.AccountKeeper.HasAccount(s.ctx, provlabsAddr) {
		s.simApp.AccountKeeper.SetAccount(s.ctx, s.simApp.AccountKeeper.NewAccountWithAddress(s.ctx, provlabsAddr))
	}

	// Ensure the tech fee account has the required attributes for restricted markers
	for _, attrName := range reqAttrs {
		// Only set the name if it's not already bound
		if !s.simApp.NameKeeper.NameExists(s.ctx, attrName) {
			s.Require().NoError(s.simApp.NameKeeper.SetNameRecord(s.ctx, attrName, s.adminAddr, false), "should successfully bind the name")
		}
		expireTime := time.Now().Add(365 * 24 * time.Hour)
		attribute := attrtypes.NewAttribute(attrName, provlabsAddr.String(), attrtypes.AttributeType_String, []byte("true"), &expireTime, "")
		s.Require().NoError(s.simApp.AttributeKeeper.SetAttribute(s.ctx, attribute, s.adminAddr), "should successfully set the required attribute on the tech fee account")
	}

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

// requireSimpleMarker registers denom as an unrestricted Coin marker so it passes the
// marker-existence check in SetVaultNAV. Use this in tests that set NAVs for external
// asset denoms (e.g. "rwa", "bond") that are not otherwise created by setupBaseVault.
func (s *TestSuite) requireSimpleMarker(denom string) {
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(denom, 0), s.adminAddr)
}

// requireRestrictedMarker creates, finalizes, and activates a restricted-coin marker for the
// denom with the suite admin holding full access. Callers seed holders via WithdrawCoins.
func (s *TestSuite) requireRestrictedMarker(denom string) {
	grants := []markertypes.AccessGrant{
		{Address: s.adminAddr.String(), Permissions: markertypes.AccessList{
			markertypes.Access_Mint, markertypes.Access_Burn, markertypes.Access_Withdraw,
			markertypes.Access_Admin, markertypes.Access_Transfer, markertypes.Access_Deposit,
		}},
	}
	marker := markertypes.NewMarkerAccount(
		authtypes.NewBaseAccountWithAddress(markertypes.MustGetMarkerAddress(denom)),
		sdk.NewInt64Coin(denom, 1_000_000),
		s.adminAddr,
		grants,
		markertypes.StatusProposed,
		markertypes.MarkerType_RestrictedCoin,
		false, true, false, []string{},
	)
	s.Require().NoError(s.simApp.MarkerKeeper.AddFinalizeAndActivateMarker(s.ctx, marker), "failed to create restricted marker %s", denom)
}

// setupAssetSettlementVault creates a single-denom vault (underlying asset only) and assigns
// the suite's assetManagerAddr as its asset manager, since settlement messages
// (AcceptAsset/RejectAsset) are asset-manager-only and must never be signed by the admin.
// The external asset a settlement trades against the underlying is a separate marker denom
// the caller registers and prices via the vault's internal NAV. It returns the vault and its
// principal marker address.
func (s *TestSuite) setupAssetSettlementVault(underlying, share string) (*types.VaultAccount, sdk.AccAddress) {
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlying, 1_000_000), s.adminAddr)

	vault, err := s.k.CreateVault(s.ctx, vaultAttrs{
		admin:      s.adminAddr.String(),
		share:      share,
		underlying: underlying,
	})
	s.Require().NoError(err, "failed to create settlement vault for share denom %s", share)

	_, err = keeper.NewMsgServer(s.simApp.VaultKeeper).SetAssetManager(s.ctx, &types.MsgSetAssetManagerRequest{
		Admin:        s.adminAddr.String(),
		VaultAddress: vault.GetAddress().String(),
		AssetManager: s.assetManagerAddr.String(),
	})
	s.Require().NoError(err, "failed to set asset manager %s on settlement vault %s", s.assetManagerAddr, vault.GetAddress())

	vault, err = s.k.GetVault(s.ctx, vault.GetAddress())
	s.Require().NoError(err, "failed to reload settlement vault %s after setting the asset manager", vault.GetAddress())

	return vault, vault.PrincipalMarkerAddress()
}

// createPayment creates an exchange payment from source to target and places the escrow hold.
func (s *TestSuite) createPayment(source, target sdk.AccAddress, sourceAmount, targetAmount sdk.Coins, externalID string) {
	err := s.simApp.ExchangeKeeper.CreatePayment(s.ctx, &exchange.Payment{
		Source:       source.String(),
		SourceAmount: sourceAmount,
		Target:       target.String(),
		TargetAmount: targetAmount,
		ExternalId:   externalID,
	})
	s.Require().NoError(err, "failed to create payment %q", externalID)
}

// acceptAssetScenario describes the shared fixture for an AcceptAsset settlement test:
// the vault denoms, an optional simple marker for the external asset, an optional seeded
// internal NAV, funding for the payment source and the vault principal, and the staged
// payment legs.
type acceptAssetScenario struct {
	underlying    string
	share         string
	assetMarker   string          // external asset denom to register as a simple marker; empty skips registration
	seedNav       *types.VaultNAV // internal NAV to seed before settlement; nil skips seeding
	fundSource    sdk.Coins       // coins minted to the payment source; zero skips funding
	fundPrincipal sdk.Coins       // coins minted to the vault principal; zero skips funding
	sourceAmount  sdk.Coins       // payment source leg
	targetAmount  sdk.Coins       // payment target leg
	externalID    string
	omitPayment   bool // skip staging the payment; the test stages its own or exercises a missing payment
}

// setupAcceptAssetScenario builds the common AcceptAsset test fixture: an asset-settlement
// vault, the optional asset marker and seeded NAV, a funded source account (which always
// carries a stake coin) and principal, and a staged payment from the source to the vault
// (unless omitPayment is set).
// It returns the vault, its principal marker address, and the payment source address.
func (s *TestSuite) setupAcceptAssetScenario(sc acceptAssetScenario) (*types.VaultAccount, sdk.AccAddress, sdk.AccAddress) {
	vault, principalAddr := s.setupAssetSettlementVault(sc.underlying, sc.share)
	if sc.assetMarker != "" {
		s.requireSimpleMarker(sc.assetMarker)
	}

	if sc.seedNav != nil {
		s.Require().NoError(
			s.k.SetVaultNAV(s.ctx, vault, *sc.seedNav, s.adminAddr.String()),
			"failed to seed internal NAV for denom %s", sc.seedNav.Denom,
		)
	}

	source := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1_000))
	if !sc.fundSource.IsZero() {
		s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, source, sc.fundSource), "failed to fund source with %s", sc.fundSource)
	}
	if !sc.fundPrincipal.IsZero() {
		s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, principalAddr, sc.fundPrincipal), "failed to fund principal with %s", sc.fundPrincipal)
	}

	if !sc.omitPayment {
		s.createPayment(source, vault.GetAddress(), sc.sourceAmount, sc.targetAmount, sc.externalID)
	}
	return vault, principalAddr, source
}

// requireTypedEventEmitted asserts that the given typed event was emitted on the current context.
func (s *TestSuite) requireTypedEventEmitted(want proto.Message) {
	s.T().Helper()
	expected, err := sdk.TypedEventToEvent(want)
	s.Require().NoError(err, "failed to convert expected typed event")

	found := false
	for _, ev := range s.ctx.EventManager().Events() {
		if ev.Type != expected.Type {
			continue
		}
		found = true
		s.Assert().Equal(normalizeEvent(expected), normalizeEvent(ev), "%s attributes mismatch", expected.Type)
	}
	s.Assert().Truef(found, "expected typed event %s to be emitted", expected.Type)
}

// requireAddFinalizeAndActivateReceiptMarker creates and activates a restricted marker
// for the given coin and grants the grantee full permissions (mint, burn, transfer,
// withdraw, deposit). This is used to replicate a receipt token. It fails the test if any step errors.
func (s *TestSuite) requireAddFinalizeAndActivateReceiptMarker(coin sdk.Coin, grantees ...sdk.AccAddress) {
	s.Require().NotEmpty(grantees, "requireAddFinalizeAndActivateReceiptMarker: grantees must not be empty")
	markerAddr, err := markertypes.MarkerAddress(coin.Denom)
	s.Require().NoError(err, "MarkerAddress(%q)", coin.Denom)

	// Ensure the tech fee account has any required attributes if this was a restricted marker.
	// For receipt markers, we usually don't have required attributes but we give the tech fee account transfer permission
	// so it can receive the tokens if the marker is restricted.
	provlabsAddr, err := s.k.GetAUMFeeAddress(s.ctx)
	s.Require().NoError(err, "failed to get AUM fee address")

	accessControl := make([]markertypes.AccessGrant, len(grantees)+1)
	for i, grantee := range grantees {
		accessControl[i] = markertypes.AccessGrant{
			Address: grantee.String(),
			Permissions: markertypes.AccessList{
				markertypes.Access_Mint,
				markertypes.Access_Burn,
				markertypes.Access_Transfer,
				markertypes.Access_Withdraw,
				markertypes.Access_Deposit,
			},
		}
	}
	accessControl[len(grantees)] = markertypes.AccessGrant{
		Address: provlabsAddr.String(),
		Permissions: markertypes.AccessList{
			markertypes.Access_Transfer,
			markertypes.Access_Deposit,
		},
	}

	marker := &markertypes.MarkerAccount{
		BaseAccount:            &authtypes.BaseAccount{Address: markerAddr.String()},
		Manager:                grantees[0].String(),
		AccessControl:          accessControl,
		Status:                 markertypes.StatusProposed,
		Denom:                  coin.Denom,
		Supply:                 coin.Amount,
		MarkerType:             markertypes.MarkerType_RestrictedCoin,
		SupplyFixed:            true,
		AllowGovernanceControl: false,
		RequiredAttributes:     nil,
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

// vaultAttrs is a simple implementation of the types.VaultCreator interface
// for use in testing.
type vaultAttrs struct {
	admin                  string
	share                  string
	underlying             string
	withdrawalDelaySeconds uint64
	minSwapIn              string
	minSwapOut             string
	maxSwapIn              string
	maxSwapOut             string
	expected               types.VaultAccount
}

func (v vaultAttrs) GetAdmin() string                  { return v.admin }
func (v vaultAttrs) GetShareDenom() string             { return v.share }
func (v vaultAttrs) GetUnderlyingAsset() string        { return v.underlying }
func (v vaultAttrs) GetWithdrawalDelaySeconds() uint64 { return v.withdrawalDelaySeconds }
func (v vaultAttrs) GetMinSwapInValue() string         { return v.minSwapIn }
func (v vaultAttrs) GetMinSwapOutValue() string        { return v.minSwapOut }
func (v vaultAttrs) GetMaxSwapInValue() string         { return v.maxSwapIn }
func (v vaultAttrs) GetMaxSwapOutValue() string        { return v.maxSwapOut }

// createSingleDenomVault creates a single-denom vault via keeper.CreateVault. It is the
// shared workhorse behind the base-vault helpers now that vaults are single-denom.
func (s *TestSuite) createSingleDenomVault(cfg vaultAttrs) *types.VaultAccount {
	vault, err := s.k.CreateVault(s.ctx, cfg)
	s.Require().NoError(err, "vault creation should succeed for share denom %s", cfg.share)
	return vault
}

// setupBaseVaultRestricted creates a single-denom vault with a restricted underlying asset.
// It establishes a marker for the underlying asset, requiring a specific attribute for
// transfers, and returns the newly created vault account.
func (s *TestSuite) setupBaseVaultRestricted(underlyingDenom, shareDenom string) *types.VaultAccount {
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlyingDenom, 2_000_000), s.adminAddr, simulation.RequiredMarkerAttribute)

	return s.createSingleDenomVault(vaultAttrs{
		admin:      s.adminAddr.String(),
		share:      shareDenom,
		underlying: underlyingDenom,
	})
}

// setupBaseVault creates and activates the marker for the underlying denom, withdraws some
// underlying coins to the admin, and creates a single-denom vault. It returns the newly
// created vault account.
func (s *TestSuite) setupBaseVault(underlyingDenom, shareDenom string) *types.VaultAccount {
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlyingDenom, 2_000_000), s.adminAddr)
	s.k.MarkerKeeper.WithdrawCoins(s.ctx, s.adminAddr, s.adminAddr, underlyingDenom, sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, 100_000)))

	return s.createSingleDenomVault(vaultAttrs{
		admin:      s.adminAddr.String(),
		share:      shareDenom,
		underlying: underlyingDenom,
	})
}

// CreateVaultWithParams creates a single-denom vault with the given share and underlying
// denoms and returns the vault account. The underlying marker must already exist.
func (s *TestSuite) CreateVaultWithParams(shareDenom, underlyingDenom string) *types.VaultAccount {
	return s.createSingleDenomVault(vaultAttrs{
		admin:      s.adminAddr.String(),
		share:      shareDenom,
		underlying: underlyingDenom,
	})
}

// CreateAndActivateVault creates a marker for the underlying asset and then creates the vault itself.
// It returns the newly created vault address.
func (s *TestSuite) CreateAndActivateVault(admin sdk.AccAddress, share, underlying string) sdk.AccAddress {
	s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlying, sdkmath.NewInt(1000)), admin)
	_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
		Admin:           admin.String(),
		ShareDenom:      share,
		UnderlyingAsset: underlying,
	})
	s.Require().NoError(err, "failed to create vault for share denom %s and underlying %s", share, underlying)
	return types.GetVaultAddress(share)
}

// FundMarker mints and sends the provided coins to the marker account associated with the share denom.
func (s *TestSuite) FundMarker(shareDenom string, coins sdk.Coins) {
	markerAddr := markertypes.MustGetMarkerAddress(shareDenom)
	s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, markerAddr, coins), "funding marker %s should not error", shareDenom)
}

// SetVaultRatesAndPeriod updates a vault's interest rates and fee period settings.
func (s *TestSuite) SetVaultRatesAndPeriod(vault *types.VaultAccount, currentRate, desiredRate string, feeStart, feeTimeout int64) {
	vault.CurrentInterestRate = currentRate
	vault.DesiredInterestRate = desiredRate
	vault.FeePeriodStart = feeStart
	vault.FeePeriodTimeout = feeTimeout
	s.k.AuthKeeper.SetAccount(s.ctx, vault)
}

// SetCtxBlockTime updates the suite's context block time and resets the event manager.
func (s *TestSuite) SetCtxBlockTime(t time.Time) {
	s.ctx = s.ctx.WithBlockTime(t).WithEventManager(sdk.NewEventManager())
}

// enqueueDueSwapOut creates a funded owner and base vault, swaps assets in, escrows the
// resulting shares into the vault account, and enqueues a due pending swap-out redeeming
// the underlying asset. It returns the owner, the escrowed shares, the request id, and the
// queued request. It consolidates the swap-out setup repeated across the payout tests.
func (s *TestSuite) enqueueDueSwapOut(underlyingDenom, shareDenom string, assets sdk.Coin, duePayoutTime int64) (sdk.AccAddress, sdk.Coin, uint64, types.PendingSwapOut) {
	ownerAddr := s.CreateAndFundAccount(assets)
	vault := s.setupBaseVault(underlyingDenom, shareDenom)

	minted, err := s.k.SwapIn(s.ctx, vault.GetAddress(), ownerAddr, assets)
	s.Require().NoError(err, "should successfully swap in assets for share denom %s", shareDenom)
	s.Require().NoError(
		s.k.BankKeeper.SendCoins(s.ctx, ownerAddr, vault.GetAddress(), sdk.NewCoins(*minted)),
		"should escrow %s shares into vault account %s", minted, vault.GetAddress(),
	)

	req := types.PendingSwapOut{
		Owner:        ownerAddr.String(),
		VaultAddress: vault.GetAddress().String(),
		RedeemDenom:  underlyingDenom,
		Shares:       *minted,
	}
	id, err := s.k.PendingSwapOutQueue.Enqueue(s.ctx, duePayoutTime, &req)
	s.Require().NoError(err, "should successfully enqueue due swap-out for share denom %s", shareDenom)

	return ownerAddr, *minted, id, req
}

// assertSwapOutEntryPreservedAndPaused verifies the invariant that protects escrowed funds when a
// swap-out hits a critical, unrecoverable failure: the pending request is still in the queue, the
// vault is paused, the owner was not paid, and the escrowed shares remain on the vault account. This
// is the state a fixed processSwapOutJobs must leave behind so the request can be settled after unpause.
func (s *TestSuite) assertSwapOutEntryPreservedAndPaused(reqID uint64, vaultAddr, ownerAddr sdk.AccAddress, escrowedShares sdk.Coin, underlyingDenom string) {
	var entries []uint64
	err := s.k.PendingSwapOutQueue.Walk(s.ctx, func(_ int64, id uint64, _ sdk.AccAddress, _ types.PendingSwapOut) (bool, error) {
		entries = append(entries, id)
		return false, nil
	})
	s.Require().NoError(err, "walking the queue should not error")
	s.Require().Equal([]uint64{reqID}, entries, "request %d must remain queued after a critical failure", reqID)

	vault, err := s.k.GetVault(s.ctx, vaultAddr)
	s.Require().NoError(err, "should successfully get vault %s", vaultAddr)
	s.Require().NotNil(vault, "vault %s should not be nil", vaultAddr)
	s.Require().True(vault.Paused, "vault %s must be paused after a critical failure", vaultAddr)

	s.assertBalance(ownerAddr, underlyingDenom, sdkmath.ZeroInt())
	s.assertBalance(vaultAddr, escrowedShares.Denom, escrowedShares.Amount)
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

// setupHeldAssetVault creates a single-denom vault (underlying + share) and gives it an
// external held asset: it registers the heldDenom marker, withdraws some to the admin, and
// seeds the vault's internal NAV pricing volume units of heldDenom at price units of the
// underlying asset. This models a vault that has acquired a non-underlying asset via
// settlement and prices it through the internal NAV chain, which is how the valuation engine
// values held assets under the single-denom design. heldDenom must differ from the underlying.
func (s *TestSuite) setupHeldAssetVault(underlyingDenom, shareDenom, heldDenom string, price, volume int64) *types.VaultAccount {
	vault := s.setupBaseVault(underlyingDenom, shareDenom)
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(heldDenom, 2_000_000), s.adminAddr)
	s.k.MarkerKeeper.WithdrawCoins(s.ctx, s.adminAddr, s.adminAddr, heldDenom, sdk.NewCoins(sdk.NewInt64Coin(heldDenom, 100_000)))
	s.setVaultNAV(vault, heldDenom, sdk.NewInt64Coin(underlyingDenom, price), volume)
	return vault
}

// createLegacyVaultAccount stores a *types.VaultAccount directly via the
// AccountKeeper, bypassing keeper.CreateVault entirely. Use this in migration
// tests that need to simulate a pre-v2 vault in state: one that may still
// carry a payment denom that differs from its underlying asset. The caller is
// responsible for registering any markers the vault depends on (e.g. payment
// + underlying).
func (s *TestSuite) createLegacyVaultAccount(shareDenom, underlyingDenom, paymentDenom string) *types.VaultAccount {
	addr := types.GetVaultAddress(shareDenom)
	if paymentDenom == "" {
		paymentDenom = underlyingDenom
	}
	vault := &types.VaultAccount{
		BaseAccount:         authtypes.NewBaseAccountWithAddress(addr),
		Admin:               s.adminAddr.String(),
		NavAuthority:        s.adminAddr.String(),
		TotalShares:         sdk.NewCoin(shareDenom, sdkmath.ZeroInt()),
		UnderlyingAsset:     underlyingDenom,
		PaymentDenom:        paymentDenom,
		CurrentInterestRate: types.ZeroInterestRate,
		DesiredInterestRate: types.ZeroInterestRate,
	}
	acct := s.simApp.AccountKeeper.NewAccount(s.ctx, vault)
	vaultAcct, ok := acct.(*types.VaultAccount)
	s.Require().True(ok, "new account should return a *types.VaultAccount for a VaultAccount prototype")
	s.simApp.AccountKeeper.SetAccount(s.ctx, vaultAcct)
	return vaultAcct
}

// setupOversizedNAVVault builds a single-denom vault holding an external asset primed for
// the NAV overflow guard tests. It returns the vault, the suite keeper (fully wired, including
// the internal NAV table the valuation engine reads), the underlying denom, and the held-asset
// denom whose internal NAV the overflow tests overwrite with oversized price/volume via
// seedOversizedNAV.
func (s *TestSuite) setupOversizedNAVVault() (*types.VaultAccount, keeper.Keeper, string, string) {
	underlyingDenom := "ylds"
	heldDenom := "usdc"
	shareDenom := "vshare"
	vault := s.setupHeldAssetVault(underlyingDenom, shareDenom, heldDenom, 1, 2)
	return vault, s.k, underlyingDenom, heldDenom
}

// setVaultNAV seeds an Internal NAV entry on the given vault for denom, pricing
// volume units at price (which must be denominated in the vault's underlying
// asset). The entry is recorded under the admin signer used by the suite.
func (s *TestSuite) setVaultNAV(vault *types.VaultAccount, denom string, price sdk.Coin, volume int64) {
	nav := types.NewVaultNAV(denom, price, sdkmath.NewInt(volume), "test")
	s.Require().NoError(s.k.SetVaultNAV(s.ctx, vault, nav, s.adminAddr.String()),
		"should set internal NAV %s -> %s=%s/%d", denom, price.Denom, price.Amount, volume)
}

// bumpHeight increments the suite's context block height by 1.
func (s *TestSuite) bumpHeight() {
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
}

// oversizedNAVPrice returns a NAV price amount (2^200) large enough that
// multiplying it by a comparable balance exceeds the 256-bit sdkmath.Int
// ceiling. It models the attacker-controlled price the marker module does not
// bound, and exercises the SafeMul overflow guards on the valuation paths.
func oversizedNAVPrice() sdkmath.Int {
	return sdkmath.NewIntFromBigInt(new(big.Int).Lsh(big.NewInt(1), 200))
}

// maxValidNAVPrice returns the largest value an sdkmath.Int can hold (2^256-1).
// As a NAV price it converts a single unit of a denom to the maximum
// representable underlying value, leaving no headroom in the TVV accumulator so
// that any additional balance trips the SafeAdd overflow guard in
// GetTVVInUnderlyingAsset without the per-balance SafeMul overflowing first.
func maxValidNAVPrice() sdkmath.Int {
	return sdkmath.NewIntFromBigInt(new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1)))
}

// seedOversizedNAV overwrites the vault's internal NAV entry for navDenom,
// pricing volume units of it at priceAmount of underlyingDenom. The internal
// NAV write path bounds neither price nor volume magnitude, so this lets the
// overflow guard tests stage values far beyond any realistic NAV and drive the
// SafeMul/SafeAdd guards on the valuation paths into overflow. Seed an oversized
// priceAmount to trip the forward (denom->underlying) multiply, or an oversized
// volume to trip the reverse (underlying->denom) multiply.
func (s *TestSuite) seedOversizedNAV(vault *types.VaultAccount, navDenom, underlyingDenom string, priceAmount, volume sdkmath.Int) {
	nav := types.NewVaultNAV(navDenom, sdk.NewCoin(underlyingDenom, priceAmount), volume, "test-oversized")
	s.Require().NoError(s.k.SetVaultNAV(s.ctx, vault, nav, s.adminAddr.String()),
		"should seed oversized internal NAV for %s priced %s/%s", navDenom, priceAmount, volume)
}

// setupReconcileVault initializes a vault with the provided parameters, including markers and funding.
func (s *TestSuite) setupReconcileVault(interestRate string, periodStartSeconds int64, paused bool, underlying sdk.Coin, shareDenom string, totalShares sdk.Coin, testBlockTime time.Time) (sdk.AccAddress, *types.VaultAccount) {
	s.requireAddFinalizeAndActivateMarker(underlying, s.adminAddr)
	vaultAddr := types.GetVaultAddress(shareDenom)
	_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
		Admin:           s.adminAddr.String(),
		ShareDenom:      shareDenom,
		UnderlyingAsset: underlying.Denom,
	})
	s.Require().NoError(err, "failed to create vault for share denom %s", shareDenom)

	vault, err := s.k.GetVault(s.ctx, vaultAddr)
	s.Require().NoError(err, "failed to get vault for address %s", vaultAddr.String())
	vault.CurrentInterestRate = interestRate
	vault.DesiredInterestRate = interestRate
	vault.PeriodStart = periodStartSeconds
	vault.FeePeriodStart = periodStartSeconds
	vault.Paused = paused
	vault.TotalShares = totalShares
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	err = FundAccount(s.ctx, s.simApp.BankKeeper, vaultAddr, sdk.NewCoins(underlying))
	s.Require().NoError(err, "failed to fund vault account %s with %s", vaultAddr.String(), underlying.String())
	err = FundAccount(s.ctx, s.simApp.BankKeeper, markertypes.MustGetMarkerAddress(shareDenom), sdk.NewCoins(underlying))
	s.Require().NoError(err, "failed to fund share marker account for denom %s with %s", shareDenom, underlying.String())

	s.ctx = s.ctx.WithBlockTime(testBlockTime)
	s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())

	return vaultAddr, vault
}

// setupBridgeVault creates and activates the underlying marker, creates a vault for
// the given share/underlying denoms, enables bridging to bridgeAddr, records the
// share supply-of-record via TotalShares, and persists the vault. It returns the
// stored vault account. Callers create and fund bridgeAddr themselves because the
// message under test usually needs the bridge address before this setup runs.
func (s *TestSuite) setupBridgeVault(underlying, share string, bridgeAddr sdk.AccAddress, totalShares sdkmath.Int) *types.VaultAccount {
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlying, 2_000_000), s.adminAddr)
	v, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
		Admin:           s.adminAddr.String(),
		ShareDenom:      share,
		UnderlyingAsset: underlying,
	})
	s.Require().NoError(err, "setup: expected vault creation to succeed for share %s", share)
	v.BridgeEnabled = true
	v.BridgeAddress = bridgeAddr.String()
	v.TotalShares = sdk.NewCoin(share, totalShares)
	s.k.AuthKeeper.SetAccount(s.ctx, v)
	return v
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

func getAttribute(ev sdk.Event, key string) string {
	for _, attr := range ev.Attributes {
		if string(attr.Key) == key {
			return string(attr.Value)
		}
	}
	return ""
}

// expectedWithSimpleAPY calculates the total amount (principal + interest)
// using a simple APY formula.
func expectedWithSimpleAPY(baseAmt sdkmath.Int, rateStr string, seconds int64) (sdkmath.Int, error) {
	rateDec, err := sdkmath.LegacyNewDecFromStr(rateStr)
	if err != nil {
		return sdkmath.Int{}, err
	}
	durationDec := sdkmath.LegacyNewDec(seconds)
	secondsPerYearDec := sdkmath.LegacyNewDec(31536000)
	timeFraction := durationDec.Quo(secondsPerYearDec)
	interestDec := baseAmt.ToLegacyDec().Mul(rateDec).Mul(timeFraction)
	return baseAmt.Add(interestDec.TruncateInt()), nil
}

// createVaultFeeCollectedEvent constructs the expected EventVaultFeeCollected event.
func createVaultFeeCollectedEvent(vaultAddr sdk.AccAddress, aumSnapshot, collected, requested, outstanding sdk.Coin, duration int64) sdk.Event {
	return sdk.NewEvent("provlabs.vault.v1.EventVaultFeeCollected",
		sdk.NewAttribute("aum_snapshot", aumSnapshot.String()),
		sdk.NewAttribute("collected_amount", collected.String()),
		sdk.NewAttribute("duration_seconds", fmt.Sprintf("%v", duration)),
		sdk.NewAttribute("outstanding_amount", outstanding.String()),
		sdk.NewAttribute("requested_amount", requested.String()),
		sdk.NewAttribute("vault_address", vaultAddr.String()),
	)
}

// makeGenesisVaultAccount constructs a minimal VaultAccount for genesis state tests.
// Both the underlying asset and payment denom are set to underlying, and both interest
// rates are initialised to ZeroInterestRate. admin is assigned as both Admin and
// NavAuthority.
func makeGenesisVaultAccount(shareDenom, underlying, admin string) types.VaultAccount {
	vaultAddr := types.GetVaultAddress(shareDenom)
	return types.VaultAccount{
		BaseAccount:         authtypes.NewBaseAccountWithAddress(vaultAddr),
		Admin:               admin,
		NavAuthority:        admin,
		TotalShares:         sdk.NewInt64Coin(shareDenom, 0),
		UnderlyingAsset:     underlying,
		PaymentDenom:        underlying,
		CurrentInterestRate: types.ZeroInterestRate,
		DesiredInterestRate: types.ZeroInterestRate,
		OutstandingAumFee:   sdk.NewInt64Coin(underlying, 0),
	}
}

// buildSingleVaultGenesisState constructs a GenesisState containing one vault and the
// provided NAV entries without touching chain state. Use this to prepare a genesis
// payload for panic/validation tests where InitGenesis must not be called in advance.
func buildSingleVaultGenesisState(shareDenom, underlying, admin string, navs []types.VaultNAVEntry) *types.GenesisState {
	return &types.GenesisState{
		Params: types.DefaultParams(),
		Vaults: []types.VaultAccount{makeGenesisVaultAccount(shareDenom, underlying, admin)},
		Navs:   navs,
	}
}

// setupVaultWithNavs builds a single-vault genesis state, initialises it via
// InitGenesis, and returns the genesis state for use in export assertions.
// Each NAV denom is registered as a marker first so the entries pass the
// marker-existence check enforced by InitGenesis.
func (s *TestSuite) setupVaultWithNavs(shareDenom, underlying, admin string, navs []types.VaultNAVEntry) *types.GenesisState {
	genesis := buildSingleVaultGenesisState(shareDenom, underlying, admin, navs)
	for _, entry := range navs {
		s.requireSimpleMarker(entry.Nav.Denom)
	}
	s.k.InitGenesis(s.ctx, genesis)
	return genesis
}

// findLastEventVaultPaused returns the most recent EventVaultPaused decoded from
// the context event manager, or nil when none has been emitted.
func (s *TestSuite) findLastEventVaultPaused() *types.EventVaultPaused {
	events := s.ctx.EventManager().Events()
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Type != "provlabs.vault.v1.EventVaultPaused" {
			continue
		}
		msg, err := sdk.ParseTypedEvent(abci.Event(events[i]))
		s.Require().NoError(err, "parsing the EventVaultPaused typed event should succeed")
		paused, ok := msg.(*types.EventVaultPaused)
		s.Require().True(ok, "parsed event should be *EventVaultPaused")
		return paused
	}
	return nil
}
