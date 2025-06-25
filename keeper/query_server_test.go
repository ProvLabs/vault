package keeper_test

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	markertypes "github.com/provenance-io/provenance/x/marker/types"
	"github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/types"
)

// queryTestDef is the definition of a QueryServer endpoint to be tested.
// R is the request message type. S is the response message type.
type queryTestDef[R any, S any] struct {
	// queryName is the name of the query being tested.
	queryName string
	// query is the query function to invoke.
	query func(goCtx context.Context, req *R) (*S, error)
	// followup is a function that runs any desired followup assertions to help pinpoint
	// differences between the expected and actual. It's only called if they're not equal and neither are nil.
	followup func(expected, actual *S)
}

// queryTestCase is a test case for a QueryServer endpoint.
// R is the request message type. S is the response message type.
type queryTestCase[R any, S any] struct {
	// name is the name of the test case.
	name string
	// setup is a function that does any needed app/state setup.
	// A cached context is used for tests, so this setup will not carry over between test cases.
	setup func()
	// req is the request message to provide to the query.
	req *R
	// expResp is the expected response from the query
	expResp *S
	// expInErr is the strings that are expected to be in the error returned by the endpoint.
	// If empty, that error is expected to be nil.
	expInErr []string
}

// runQueryTestCase runs a unit test on a QueryServer endpoint.
// A cached context is used so each test case won't affect the others.
// R is the request message type. S is the response message type.
func runQueryTestCase[R any, S any](s *TestSuite, td queryTestDef[R, S], tc queryTestCase[R, S]) {
	origCtx := s.ctx
	defer func() {
		s.ctx = origCtx
	}()
	s.ctx, _ = s.ctx.CacheContext()

	if tc.setup != nil {
		tc.setup()
	}

	goCtx := sdk.WrapSDKContext(s.ctx)
	var resp *S
	var err error
	testFunc := func() {
		resp, err = td.query(goCtx, tc.req)
	}
	s.Require().NotPanics(testFunc, td.queryName)

	if len(tc.expInErr) == 0 {
		s.Assert().NoErrorf(err, "%s error", td.queryName)
		s.Assert().Equalf(tc.expResp, resp, "%s response", td.queryName)
	} else {
		s.Assert().Errorf(err, "%s error", td.queryName)
		for _, substr := range tc.expInErr {
			s.Assert().Containsf(err.Error(), substr, "%s error missing expected substring", td.queryName)
		}
		return
	}

	if td.followup != nil && tc.expResp != nil && resp != nil {
		td.followup(tc.expResp, resp)
	}
}

func (s *TestSuite) TestQueryServer_Vault() {
	testDef := queryTestDef[types.QueryVaultRequest, types.QueryVaultResponse]{
		queryName: "Vault",
		query:     keeper.NewQueryServer(s.simApp.VaultKeeper).Vault,
	}

	addr1 := markertypes.MustGetMarkerAddress("vault1").String()
	addr2 := markertypes.MustGetMarkerAddress("vault2").String()
	addr3 := markertypes.MustGetMarkerAddress("vault3").String()
	admin := s.adminAddr.String()

	tests := []queryTestCase[types.QueryVaultRequest, types.QueryVaultResponse]{
		{
			name:     "nil request",
			req:      nil,
			expInErr: []string{"vault_address must be provided"},
		},
		{
			name:     "empty vault address",
			req:      &types.QueryVaultRequest{VaultAddress: ""},
			expInErr: []string{"vault_address must be provided"},
		},
		{
			name:     "invalid vault address",
			req:      &types.QueryVaultRequest{VaultAddress: "invalid-bech32-address"},
			expInErr: []string{"invalid vault_address", "decoding bech32 failed"},
		},
		{
			name:     "vault not found",
			req:      &types.QueryVaultRequest{VaultAddress: addr1},
			expInErr: []string{fmt.Sprintf("vault with address %q not found", addr1)},
		},
		{
			name: "vault found",
			setup: func() {
				testVault := types.NewVault(admin, addr2, "stake")
				s.Require().NoError(s.k.SetVault(s.ctx, testVault))
			},
			req: &types.QueryVaultRequest{VaultAddress: addr2},
			expResp: &types.QueryVaultResponse{
				Vault: *types.NewVault(admin, addr2, "stake"),
			},
		},
		{
			name: "multiple vaults exist but query for specific one",
			setup: func() {
				testVault1 := types.NewVault(admin, addr2, "stake")
				s.Require().NoError(s.k.SetVault(s.ctx, testVault1))
				testVault2 := types.NewVault(admin, addr1, "stake2")
				s.Require().NoError(s.k.SetVault(s.ctx, testVault2))
				testVault3 := types.NewVault(admin, addr3, "stake3")
				s.Require().NoError(s.k.SetVault(s.ctx, testVault3))
			},
			req: &types.QueryVaultRequest{VaultAddress: addr2},
			expResp: &types.QueryVaultResponse{
				Vault: *types.NewVault(admin, addr2, "stake"),
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			runQueryTestCase(s, testDef, tc)
		})
	}
}
