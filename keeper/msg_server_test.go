package keeper_test

import (
	"context"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
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

// msgServerTestDef is the definition of a MsgServer endpoint to be tested.
// R is the request Msg type. S is the response message type.
// F is a type that holds arguments to provide to the followup function.
type msgServerTestDef[R any, S any, F any] struct {
	// endpointName is the name of the endpoint being tested.
	endpointName string
	// endpoint is the endpoint function to invoke.
	endpoint func(goCtx context.Context, msg *R) (*S, error)
	// expResp is the expected response from the endpoint. It's only used if an error is not expected.
	expResp *S
	// followup is a function that runs any needed followup checks.
	// This is only executed if an error is neither expected, nor received.
	// The TestSuite's ctx will be the cached context with the results of the setup and endpoint applied.
	followup func(msg *R, fArgs F)
}

// msgServerTestCase is a test case for a MsgServer endpoint
// R is the request Msg type.
// F is a type that holds arguments to provide to the followup function.
type msgServerTestCase[R any, F any] struct {
	// name is the name of the test case.
	name string
	// setup is a function that does any needed app/state setup.
	// A cached context is used for tests, so this setup will not carry over between test cases.
	setup func()
	// msg is the sdk.Msg to provide to the endpoint.
	msg R
	// expInErr is the strings that are expected to be in the error returned by the endpoint.
	// If empty, that error is expected to be nil.
	expInErr []string
	// fArgs are any args to provide to the followup function.
	fArgs F
	// expEvents are the typed events that should be emitted.
	// These are only checked if an error is neither expected, nor received.
	expEvents sdk.Events
}

// runMsgServerTestCase runs a unit test on a MsgServer endpoint.
// A cached context is used so each test case won't affect the others.
// R is the request Msg type. S is the response Msg type.
// F is a type that holds arguments to provide to the td.followup function.
func runMsgServerTestCase[R any, S any, F any](s *TestSuite, td msgServerTestDef[R, S, F], tc msgServerTestCase[R, F]) {
	s.T().Helper()
	origCtx := s.ctx
	defer func() {
		s.ctx = origCtx
	}()
	s.ctx, _ = s.ctx.CacheContext()

	var expResp *S
	if len(tc.expInErr) == 0 {
		expResp = td.expResp
	}

	if tc.setup != nil {
		tc.setup()
	}

	em := sdk.NewEventManager()
	var resp *S
	var err error
	testFunc := func() {
		resp, err = td.endpoint(s.ctx, &tc.msg)
	}
	s.Require().NotPanicsf(testFunc, td.endpointName)
	s.assertErrorContentsf(err, tc.expInErr, "%s error", td.endpointName)
	s.Assert().Equalf(expResp, resp, "%s response", td.endpointName)

	if len(tc.expInErr) > 0 || err != nil {
		return
	}

	actEvents := em.Events()
	s.assertEqualEvents(tc.expEvents, actEvents, "%s events", td.endpointName)

	td.followup(&tc.msg, tc.fArgs)
}

func (s *TestSuite) TestMsgServer_CreateVault() {
	type followupArgs struct {
		UnderlyingAsset string
		ShareDenom      string
		Admin           string
	}

	testDef := msgServerTestDef[types.MsgCreateVaultRequest, types.MsgCreateVaultResponse, followupArgs]{
		endpointName: "CreateVault",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).CreateVault,
		followup: func(msg *types.MsgCreateVaultRequest, fargs followupArgs) {
			vaultDenom := fargs.ShareDenom
			vaultAddr := markertypes.MustGetMarkerAddress(vaultDenom)

			marker, err := s.simApp.MarkerKeeper.GetMarker(s.ctx, vaultAddr)
			s.Require().NoError(err, "marker should exist")

			s.EqualValues(0, marker.GetSupply().Amount.Int64(), "vault marker supply should be zero")
			s.False(marker.AllowsForcedTransfer(), "vault marker should not have forced transfer")
			s.False(marker.HasGovernanceEnabled(), "vault marker should not have governance")
			s.True(marker.GetMarkerType() == markertypes.MarkerType_RestrictedCoin, "vault marker should be restricted")
			s.False(marker.HasGovernanceEnabled(), "vault marker should not allow governance control")

			access := marker.GetAccessList()
			s.Len(access, 1)
			s.Equal(fargs.Admin, access[0].Address, "vault marker access should be granted to admin")
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
			vault, found := s.k.GetVault(s.ctx, vaultAddr)
			s.Require().True(found, "vault should exist in state")
			s.Equal(fargs.Admin, vault.Admin)
			s.Equal(fargs.ShareDenom, vault.ShareDenom)
			s.Equal(fargs.UnderlyingAsset, vault.UnderlyingAsset)
			s.Equal(vaultAddr.String(), vault.VaultAddress)

			// Check event emitted
			// s.assertTypedEvent(&types.EventVaultCreated{
			// 	Admin:           fargs.Admin,
			// 	ShareDenom:      vault.ShareDenom,
			// 	UnderlyingAsset: vault.UnderlyingAsset,
			// })
		},
	}

	underlying := "undercoin"
	sharedenom := "jackthecat"
	admin := s.adminAddr.String()

	vaultReq := types.MsgCreateVaultRequest{
		Admin:           admin,
		ShareDenom:      sharedenom,
		UnderlyingAsset: underlying,
	}

	tc := msgServerTestCase[types.MsgCreateVaultRequest, followupArgs]{
		name:     "happy path",
		setup:    nil,
		msg:      vaultReq,
		expInErr: nil,
		fArgs: followupArgs{
			UnderlyingAsset: underlying,
			ShareDenom:      sharedenom,
			Admin:           admin,
		},
		expEvents: sdk.Events{},
	}

	testDef.expResp = &types.MsgCreateVaultResponse{
		VaultAddress: markertypes.MustGetMarkerAddress(sharedenom).String(),
	}

	runMsgServerTestCase(s, testDef, tc)
}
