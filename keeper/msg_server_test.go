package keeper_test

import (
	"context"
	"strings"
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	markertypes "github.com/provenance-io/provenance/x/marker/types"
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

	adminAddr sdk.AccAddress
}

func (s *TestSuite) SetupTest() {
	s.simApp = simapp.Setup(s.T())
	s.ctx = s.simApp.NewContext(false)
	s.k = *s.simApp.VaultKeeper

	s.adminAddr = sdk.AccAddress("adminAddr___________")
}

func TestKeeperTestSuite(t *testing.T) {
	suite.Run(t, new(TestSuite))
}

func (s *TestSuite) TestMsgServer_CreateVault() {
	type postCheckArgs struct {
		UnderlyingAsset string
		ShareDenom      string
		Admin           string
		VaultAddr       sdk.AccAddress
	}

	testDef := msgServerTestDef[types.MsgCreateVaultRequest, types.MsgCreateVaultResponse, postCheckArgs]{
		endpointName: "CreateVault",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).CreateVault,
		postCheck: func(msg *types.MsgCreateVaultRequest, postCheckArgs postCheckArgs) {
			markerAddr := markertypes.MustGetMarkerAddress(postCheckArgs.ShareDenom)

			marker, err := s.simApp.MarkerKeeper.GetMarker(s.ctx, markerAddr)
			s.Require().NoError(err, "marker should exist")

			s.EqualValues(0, marker.GetSupply().Amount.Int64(), "vault marker supply should be zero")
			s.False(marker.AllowsForcedTransfer(), "vault marker should not have forced transfer")
			s.False(marker.HasGovernanceEnabled(), "vault marker should not have governance")
			s.True(marker.GetMarkerType() == markertypes.MarkerType_RestrictedCoin, "vault marker should be restricted")
			s.False(marker.HasGovernanceEnabled(), "vault marker should not allow governance control")

			access := marker.GetAccessList()
			s.Len(access, 1)
			s.Equal(types.GetVaultAddress(postCheckArgs.ShareDenom).String(), access[0].Address, "vault marker access should be granted to vault account")
			s.ElementsMatch(
				[]markertypes.Access{
					markertypes.Access_Admin,
					markertypes.Access_Mint,
					markertypes.Access_Burn,
					markertypes.Access_Withdraw,
					markertypes.Access_Transfer,
				},
				access[0].Permissions,
			)

			// Check vault record exists
			account := s.simApp.AccountKeeper.GetAccount(s.ctx, postCheckArgs.VaultAddr)
			s.Require().NotNil(account, "expected vault account to exist in state")
			vaultAcc, ok := account.(types.VaultAccountI)
			s.Require().True(ok, "expected account to be of type VaultAccountI")
			s.Equal(postCheckArgs.Admin, vaultAcc.GetAdmin(), "expected vault admin to match requested admin address")
			s.Equal(
				types.GetVaultAddress(postCheckArgs.ShareDenom),
				vaultAcc.GetAddress(),
				"expected vault address to match derived address from share denom",
			)
			s.Len(vaultAcc.GetUnderlyingAssets(), 1, "expected vault to contain exactly one underlying asset")
			s.Equal(
				postCheckArgs.UnderlyingAsset,
				vaultAcc.GetUnderlyingAssets()[0],
				"expected vault underlying asset denom to match request",
			)
		},
	}

	underlying := "undercoin"
	sharedenom := "jackthecat"
	admin := s.adminAddr.String()
	vaultAddr := types.GetVaultAddress(sharedenom)

	vaultReq := types.MsgCreateVaultRequest{
		Admin:           admin,
		ShareDenom:      sharedenom,
		UnderlyingAsset: underlying,
	}

	tc := msgServerTestCase[types.MsgCreateVaultRequest, postCheckArgs]{
		name: "happy path",
		setup: func() {
			s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlying, math.NewInt(100)), s.adminAddr)
		},
		msg:                vaultReq,
		expectedErrSubstrs: nil,
		postCheckArgs: postCheckArgs{
			UnderlyingAsset: underlying,
			ShareDenom:      sharedenom,
			Admin:           admin,
			VaultAddr:       vaultAddr,
		},
		expectedEvents: sdk.Events{
			sdk.NewEvent("provenance.marker.v1.EventMarkerAdd",
				sdk.NewAttribute("address", "provlabs157rf76qwxlttnjyncsaxvelc96m9e5eedpymea"),
				sdk.NewAttribute("amount", "0"),
				sdk.NewAttribute("denom", sharedenom),
				sdk.NewAttribute("manager", vaultAddr.String()),
				sdk.NewAttribute("marker_type", "MARKER_TYPE_RESTRICTED"),
				sdk.NewAttribute("status", "proposed"),
			),
			sdk.NewEvent("provenance.marker.v1.EventMarkerFinalize",
				sdk.NewAttribute("administrator", vaultAddr.String()),
				sdk.NewAttribute("denom", sharedenom),
			),
			sdk.NewEvent("provenance.marker.v1.EventMarkerActivate",
				sdk.NewAttribute("administrator", vaultAddr.String()),
				sdk.NewAttribute("denom", sharedenom),
			),
			sdk.NewEvent("vault.v1.EventVaultCreated",
				sdk.NewAttribute("admin", admin),
				sdk.NewAttribute("share_denom", sharedenom),
				sdk.NewAttribute("underlying_asset", underlying),
				sdk.NewAttribute("vault_address", vaultAddr.String()),
			),
		},
	}

	testDef.expectedResponse = &types.MsgCreateVaultResponse{
		VaultAddress: types.GetVaultAddress(sharedenom).String(),
	}

	runMsgServerTestCase(s, testDef, tc)
}

func (s *TestSuite) TestMsgServer_CreateVault_Failures() {
	testDef := msgServerTestDef[types.MsgCreateVaultRequest, types.MsgCreateVaultResponse, any]{
		endpointName: "CreateVault",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).CreateVault,
		postCheck:    nil,
	}

	tests := []msgServerTestCase[types.MsgCreateVaultRequest, any]{
		{
			name: "invalid admin address",
			setup: func() {
				s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin("assetcoin", 100), s.adminAddr)
			},
			msg: types.MsgCreateVaultRequest{
				Admin:           "invalid_bech32",
				ShareDenom:      "sharecoin",
				UnderlyingAsset: "assetcoin",
			},
			expectedErrSubstrs: []string{"failed to create vault account: failed to validate vault account: invalid admin address: decoding bech32"},
		},
		{
			name: "invalid share denom",
			setup: func() {
				s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin("assetcoin", 100), s.adminAddr)
			},
			msg: types.MsgCreateVaultRequest{
				Admin:           s.adminAddr.String(),
				ShareDenom:      "invalid denom!",
				UnderlyingAsset: "assetcoin",
			},
			expectedErrSubstrs: []string{"failed to create vault account: failed to validate vault account: invalid share denom: invalid denom: invalid denom!"},
		},
		{
			name: "underlying asset marker not found",
			msg: types.MsgCreateVaultRequest{
				Admin:           s.adminAddr.String(),
				ShareDenom:      "vaulttoken",
				UnderlyingAsset: "nonexistentasset",
			},
			expectedErrSubstrs: []string{"underlying asset marker \"nonexistentasset\" not found"},
		},
		{
			name: "share denom marker already exists",
			setup: func() {
				s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin("existingmarker", 1), s.adminAddr)
				s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin("under", 1), s.adminAddr)
			},
			msg: types.MsgCreateVaultRequest{
				Admin:           s.adminAddr.String(),
				ShareDenom:      "existingmarker",
				UnderlyingAsset: "under",
			},
			expectedErrSubstrs: []string{"already exists"},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			runMsgServerTestCase(s, testDef, tc)
		})
	}
}

// msgServerTestDef defines the configuration for testing a specific MsgServer endpoint.
// Req is the request message type.
// Resp is the expected response message type.
// CheckArgs is the argument type passed to the postCheck function.
type msgServerTestDef[Req any, Resp any, CheckArgs any] struct {
	endpointName     string
	endpoint         func(ctx context.Context, msg *Req) (*Resp, error)
	expectedResponse *Resp
	postCheck        func(msg *Req, args CheckArgs)
}

// msgServerTestCase defines a single test case for a MsgServer endpoint.
// Req is the request message type.
// CheckArgs is the argument type passed to the postCheck function.
type msgServerTestCase[Req any, CheckArgs any] struct {
	name               string
	setup              func()
	msg                Req
	expectedErrSubstrs []string
	postCheckArgs      CheckArgs
	expectedEvents     sdk.Events
}

// runMsgServerTestCase executes a unit test for a MsgServer endpoint using the given test definition and test case.
// Req is the request message type.
// Resp is the expected response message type.
// CheckArgs is the argument type passed to the postCheck function.
func runMsgServerTestCase[Req any, Resp any, CheckArgs any](
	s *TestSuite,
	td msgServerTestDef[Req, Resp, CheckArgs],
	tc msgServerTestCase[Req, CheckArgs],
) {
	s.T().Helper()

	origCtx := s.ctx
	defer func() { s.ctx = origCtx }()
	s.ctx, _ = s.ctx.CacheContext()

	if tc.setup != nil {
		tc.setup()
	}

	em := sdk.NewEventManager()
	s.ctx = s.ctx.WithEventManager(em)

	var resp *Resp
	var err error
	s.Require().NotPanicsf(func() {
		resp, err = td.endpoint(s.ctx, &tc.msg)
	}, "%s panic", td.endpointName)

	if len(tc.expectedErrSubstrs) == 0 {
		s.Assert().NoErrorf(err, "%s error", td.endpointName)
		s.Assert().Equalf(td.expectedResponse, resp, "%s response", td.endpointName)
	} else {
		s.Assert().Errorf(err, "%s error", td.endpointName)
		for _, substr := range tc.expectedErrSubstrs {
			s.Assert().Containsf(err.Error(), substr, "%s error missing expected substring", td.endpointName)
		}
		return
	}

	s.Assert().Equalf(
		normalizeEvents(tc.expectedEvents),
		normalizeEvents(em.Events()),
		"%s events", td.endpointName,
	)

	td.postCheck(&tc.msg, tc.postCheckArgs)
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
					markertypes.Access_Admin, markertypes.Access_Transfer,
				},
			},
		},
		Status:                 markertypes.StatusProposed,
		Denom:                  coin.Denom,
		Supply:                 coin.Amount,
		MarkerType:             markertypes.MarkerType_RestrictedCoin,
		SupplyFixed:            true,
		AllowGovernanceControl: true,
		AllowForcedTransfer:    true,
		RequiredAttributes:     reqAttrs,
	}
	err = s.simApp.MarkerKeeper.AddFinalizeAndActivateMarker(s.ctx, marker)
	s.Require().NoError(err, "AddFinalizeAndActivateMarker(%s)", coin.Denom)
}
