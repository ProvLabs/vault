package keeper_test

import (
	"context"
	"fmt"
	"sort"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	markertypes "github.com/provenance-io/provenance/x/marker/types"
	"github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/types"
)

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
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			runQueryTestCase(s, testDef, tc)
		})
	}
}

// TestQueryServer_Vaults tests the Vaults query endpoint.
func (s *TestSuite) TestQueryServer_Vaults() {
	testDef := queryTestDef[types.QueryVaultsRequest, types.QueryVaultsResponse]{
		queryName: "Vaults",
		query:     keeper.NewQueryServer(s.simApp.VaultKeeper).Vaults,
		postCheck: func(expected, actual *types.QueryVaultsResponse) {
			// Sort both expected and actual vaults for deterministic comparison
			sortVaults(expected.Vaults)
			sortVaults(actual.Vaults)
		},
	}

	admin := s.adminAddr.String()

	// Define some vault addresses for consistent testing
	vault1Addr := markertypes.MustGetMarkerAddress("vault1").String()
	vault2Addr := markertypes.MustGetMarkerAddress("vault2").String()
	vault3Addr := markertypes.MustGetMarkerAddress("vault3").String()

	tests := []queryTestCase[types.QueryVaultsRequest, types.QueryVaultsResponse]{
		{
			name: "happy path - single vault",
			setup: func() {
				testVault1 := types.NewVault(admin, vault1Addr, "stake")
				s.Require().NoError(s.k.SetVault(s.ctx, testVault1))
			},
			req: &types.QueryVaultsRequest{},
			expResp: &types.QueryVaultsResponse{
				Vaults: []types.Vault{
					*types.NewVault(admin, vault1Addr, "stake"),
				},
				Pagination: &query.PageResponse{Total: 1},
			},
		},
		{
			name: "happy path - multiple vaults",
			setup: func() {
				testVault1 := types.NewVault(admin, vault1Addr, "stake")
				s.Require().NoError(s.k.SetVault(s.ctx, testVault1))
				testVault2 := types.NewVault(admin, vault2Addr, "nhash")
				s.Require().NoError(s.k.SetVault(s.ctx, testVault2))
				testVault3 := types.NewVault(admin, vault3Addr, "usdf")
				s.Require().NoError(s.k.SetVault(s.ctx, testVault3))
			},
			req: &types.QueryVaultsRequest{},
			expResp: &types.QueryVaultsResponse{
				Vaults: []types.Vault{
					*types.NewVault(admin, vault1Addr, "stake"),
					*types.NewVault(admin, vault3Addr, "usdf"),
					*types.NewVault(admin, vault2Addr, "nhash"),
				},
				Pagination: &query.PageResponse{Total: 3},
			},
		},
		{
			name: "pagination - limits the number of outputs",
			setup: func() {
				s.Require().NoError(s.k.SetVault(s.ctx, types.NewVault(admin, vault1Addr, "stake")))
				s.Require().NoError(s.k.SetVault(s.ctx, types.NewVault(admin, vault2Addr, "nhash")))
				s.Require().NoError(s.k.SetVault(s.ctx, types.NewVault(admin, vault3Addr, "usdf")))
			},
			req: &types.QueryVaultsRequest{
				Pagination: &query.PageRequest{Limit: 2},
			},
			expResp: &types.QueryVaultsResponse{
				Vaults: []types.Vault{
					*types.NewVault(admin, vault1Addr, "stake"),
					*types.NewVault(admin, vault3Addr, "usdf"),
				},
				Pagination: &query.PageResponse{
					NextKey: []byte{0xfd, 0xbb, 0xbd, 0xab, 0x27, 0x50, 0xf4, 0x3c, 0x2b, 0x2d, 0xb2, 0xe1, 0xa9, 0x3d, 0xb6, 0x64, 0xf4, 0xb0, 0xe7, 0xd2},
				},
			},
		},
		{
			name: "pagination - offset starts at correct location",
			setup: func() {
				s.Require().NoError(s.k.SetVault(s.ctx, types.NewVault(admin, vault1Addr, "stake")))
				s.Require().NoError(s.k.SetVault(s.ctx, types.NewVault(admin, vault2Addr, "nhash")))
				s.Require().NoError(s.k.SetVault(s.ctx, types.NewVault(admin, vault3Addr, "usdf")))
			},
			req: &types.QueryVaultsRequest{
				Pagination: &query.PageRequest{Offset: 1},
			},
			expResp: &types.QueryVaultsResponse{
				Vaults: []types.Vault{
					*types.NewVault(admin, vault3Addr, "usdf"),
					*types.NewVault(admin, vault2Addr, "nhash"),
				},
				Pagination: &query.PageResponse{Total: 3},
			},
		},
		{
			name: "pagination - offset starts at correct location and enforces limit",
			setup: func() {
				s.Require().NoError(s.k.SetVault(s.ctx, types.NewVault(admin, vault1Addr, "stake")))
				s.Require().NoError(s.k.SetVault(s.ctx, types.NewVault(admin, vault2Addr, "nhash")))
				s.Require().NoError(s.k.SetVault(s.ctx, types.NewVault(admin, vault3Addr, "usdf")))
			},
			req: &types.QueryVaultsRequest{
				Pagination: &query.PageRequest{Offset: 2, Limit: 1},
			},
			expResp: &types.QueryVaultsResponse{
				Vaults: []types.Vault{
					*types.NewVault(admin, vault2Addr, "nhash"),
				},
				Pagination: &query.PageResponse{},
			},
		},
		{
			name: "pagination - enabled count total",
			setup: func() {
				s.Require().NoError(s.k.SetVault(s.ctx, types.NewVault(admin, vault1Addr, "stake")))
				s.Require().NoError(s.k.SetVault(s.ctx, types.NewVault(admin, vault2Addr, "nhash")))
				s.Require().NoError(s.k.SetVault(s.ctx, types.NewVault(admin, vault3Addr, "usdf")))
			},
			req: &types.QueryVaultsRequest{
				Pagination: &query.PageRequest{CountTotal: true},
			},
			expResp: &types.QueryVaultsResponse{
				Vaults: []types.Vault{
					*types.NewVault(admin, vault1Addr, "stake"),
					*types.NewVault(admin, vault3Addr, "usdf"),
					*types.NewVault(admin, vault2Addr, "nhash"),
				},
				Pagination: &query.PageResponse{Total: 3},
			},
		},
		{
			name: "pagination - reverse provides the results in reverse order",
			setup: func() {
				s.Require().NoError(s.k.SetVault(s.ctx, types.NewVault(admin, vault1Addr, "stake")))
				s.Require().NoError(s.k.SetVault(s.ctx, types.NewVault(admin, vault2Addr, "nhash")))
				s.Require().NoError(s.k.SetVault(s.ctx, types.NewVault(admin, vault3Addr, "usdf")))
			},
			req: &types.QueryVaultsRequest{
				Pagination: &query.PageRequest{Reverse: true, Limit: 2},
			},
			expResp: &types.QueryVaultsResponse{
				Vaults: []types.Vault{
					*types.NewVault(admin, vault2Addr, "nhash"),
					*types.NewVault(admin, vault3Addr, "usdf"),
				},
				Pagination: &query.PageResponse{
					NextKey: []byte{0x6b, 0xd4, 0x73, 0x07, 0xd6, 0x2f, 0xda, 0x7b, 0x2e, 0x33, 0x60, 0xae, 0xba, 0x1c, 0x2d, 0x4d, 0x49, 0x97, 0x68, 0x84},
				},
			},
		},
		{
			name: "empty state",
			setup: func() {
			},
			req: &types.QueryVaultsRequest{},
			expResp: &types.QueryVaultsResponse{
				Vaults:     []types.Vault{},
				Pagination: &query.PageResponse{},
			},
		},
		{
			name:     "nil request",
			req:      nil,
			expInErr: []string{"invalid request"},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			runQueryTestCase(s, testDef, tc)
		})
	}
}

// sortVaults sorts a slice of Vaults by their VaultAddress for deterministic testing.
func sortVaults(vaults []types.Vault) {
	sort.Slice(vaults, func(i, j int) bool {
		return vaults[i].VaultAddress < vaults[j].VaultAddress
	})
}

// queryTestDef is the definition of a QueryServer endpoint to be tested.
// R is the request message type. S is the response message type.
type queryTestDef[R any, S any] struct {
	// queryName is the name of the query being tested.
	queryName string
	// query is the query function to invoke.
	query func(goCtx context.Context, req *R) (*S, error)
	// postCheck is a function that runs any desired followup assertions to help pinpoint
	// differences between the expected and actual. It's only called if they're not equal and neither are nil.
	postCheck func(expected, actual *S)
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

	if td.postCheck != nil && tc.expResp != nil && resp != nil {
		td.postCheck(tc.expResp, resp)
	}
}
