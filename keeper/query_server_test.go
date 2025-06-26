package keeper_test

// import (
// 	"fmt"

// 	"github.com/cosmos/cosmos-sdk/types/query"
// 	markertypes "github.com/provenance-io/provenance/x/marker/types"
// 	"github.com/provlabs/vault/keeper"
// 	"github.com/provlabs/vault/types"
// 	querytest "github.com/provlabs/vault/utils/query"
// )

// func (s *TestSuite) TestQueryServer_Vault() {
// 	testDef := querytest.TestDef[types.QueryVaultRequest, types.QueryVaultResponse]{
// 		QueryName: "Vault",
// 		Query:     keeper.NewQueryServer(s.simApp.VaultKeeper).Vault,
// 	}

// 	addr1 := markertypes.MustGetMarkerAddress("vault1").String()
// 	addr2 := markertypes.MustGetMarkerAddress("vault2").String()
// 	addr3 := markertypes.MustGetMarkerAddress("vault3").String()
// 	admin := s.adminAddr.String()

// 	tests := []querytest.TestCase[types.QueryVaultRequest, types.QueryVaultResponse]{
// 		{
// 			Name: "vault found",
// 			Setup: func() {
// 				testVault := types.NewVault(admin, addr2, "stake")
// 				s.Require().NoError(s.k.SetVault(s.ctx, testVault))
// 			},
// 			Req: &types.QueryVaultRequest{VaultAddress: addr2},
// 			ExpectedResp: &types.QueryVaultResponse{
// 				Vault: *types.NewVault(admin, addr2, "stake"),
// 			},
// 		},
// 		{
// 			Name: "multiple vaults exist but query for specific one",
// 			Setup: func() {
// 				testVault1 := types.NewVault(admin, addr2, "stake")
// 				s.Require().NoError(s.k.SetVault(s.ctx, testVault1))
// 				testVault2 := types.NewVault(admin, addr1, "stake2")
// 				s.Require().NoError(s.k.SetVault(s.ctx, testVault2))
// 				testVault3 := types.NewVault(admin, addr3, "stake3")
// 				s.Require().NoError(s.k.SetVault(s.ctx, testVault3))
// 			},
// 			Req: &types.QueryVaultRequest{VaultAddress: addr2},
// 			ExpectedResp: &types.QueryVaultResponse{
// 				Vault: *types.NewVault(admin, addr2, "stake"),
// 			},
// 		},
// 		{
// 			Name:               "nil request",
// 			Req:                nil,
// 			ExpectedErrSubstrs: []string{"vault_address must be provided"},
// 		},
// 		{
// 			Name:               "empty vault address",
// 			Req:                &types.QueryVaultRequest{VaultAddress: ""},
// 			ExpectedErrSubstrs: []string{"vault_address must be provided"},
// 		},
// 		{
// 			Name:               "invalid vault address",
// 			Req:                &types.QueryVaultRequest{VaultAddress: "invalid-bech32-address"},
// 			ExpectedErrSubstrs: []string{"invalid vault_address", "decoding bech32 failed"},
// 		},
// 		{
// 			Name:               "vault not found",
// 			Req:                &types.QueryVaultRequest{VaultAddress: addr1},
// 			ExpectedErrSubstrs: []string{fmt.Sprintf("vault with address %q not found", addr1)},
// 		},
// 	}

// 	for _, tc := range tests {
// 		s.Run(tc.Name, func() {
// 			querytest.RunTestCase(s, testDef, tc)
// 		})
// 	}
// }

// // TestQueryServer_Vaults tests the Vaults query endpoint.
// func (s *TestSuite) TestQueryServer_Vaults() {
// 	testDef := querytest.TestDef[types.QueryVaultsRequest, types.QueryVaultsResponse]{
// 		QueryName: "Vaults",
// 		Query:     keeper.NewQueryServer(s.simApp.VaultKeeper).Vaults,
// 	}

// 	admin := s.adminAddr.String()

// 	// Define some vault addresses for consistent testing
// 	vault1Addr := markertypes.MustGetMarkerAddress("vault1").String()
// 	vault2Addr := markertypes.MustGetMarkerAddress("vault2").String()
// 	vault3Addr := markertypes.MustGetMarkerAddress("vault3").String()

// 	tests := []querytest.TestCase[types.QueryVaultsRequest, types.QueryVaultsResponse]{
// 		{
// 			Name: "happy path - single vault",
// 			Setup: func() {
// 				testVault1 := types.NewVault(admin, vault1Addr, "stake")
// 				s.Require().NoError(s.k.SetVault(s.ctx, testVault1))
// 			},
// 			Req: &types.QueryVaultsRequest{},
// 			ExpectedResp: &types.QueryVaultsResponse{
// 				Vaults: []types.Vault{
// 					*types.NewVault(admin, vault1Addr, "stake"),
// 				},
// 				Pagination: &query.PageResponse{Total: 1},
// 			},
// 		},
// 		{
// 			Name: "happy path - multiple vaults",
// 			Setup: func() {
// 				testVault1 := types.NewVault(admin, vault1Addr, "stake")
// 				s.Require().NoError(s.k.SetVault(s.ctx, testVault1))
// 				testVault2 := types.NewVault(admin, vault2Addr, "nhash")
// 				s.Require().NoError(s.k.SetVault(s.ctx, testVault2))
// 				testVault3 := types.NewVault(admin, vault3Addr, "usdf")
// 				s.Require().NoError(s.k.SetVault(s.ctx, testVault3))
// 			},
// 			Req: &types.QueryVaultsRequest{},
// 			ExpectedResp: &types.QueryVaultsResponse{
// 				Vaults: []types.Vault{
// 					*types.NewVault(admin, vault1Addr, "stake"),
// 					*types.NewVault(admin, vault3Addr, "usdf"),
// 					*types.NewVault(admin, vault2Addr, "nhash"),
// 				},
// 				Pagination: &query.PageResponse{Total: 3},
// 			},
// 		},
// 		{
// 			Name: "pagination - limits the number of outputs",
// 			Setup: func() {
// 				s.Require().NoError(s.k.SetVault(s.ctx, types.NewVault(admin, vault1Addr, "stake")))
// 				s.Require().NoError(s.k.SetVault(s.ctx, types.NewVault(admin, vault2Addr, "nhash")))
// 				s.Require().NoError(s.k.SetVault(s.ctx, types.NewVault(admin, vault3Addr, "usdf")))
// 			},
// 			Req: &types.QueryVaultsRequest{
// 				Pagination: &query.PageRequest{Limit: 2},
// 			},
// 			ExpectedResp: &types.QueryVaultsResponse{
// 				Vaults: []types.Vault{
// 					*types.NewVault(admin, vault1Addr, "stake"),
// 					*types.NewVault(admin, vault3Addr, "usdf"),
// 				},
// 				Pagination: &query.PageResponse{
// 					NextKey: []byte{0xfd, 0xbb, 0xbd, 0xab, 0x27, 0x50, 0xf4, 0x3c, 0x2b, 0x2d, 0xb2, 0xe1, 0xa9, 0x3d, 0xb6, 0x64, 0xf4, 0xb0, 0xe7, 0xd2},
// 				},
// 			},
// 		},
// 		{
// 			Name: "pagination - offset starts at correct location",
// 			Setup: func() {
// 				s.Require().NoError(s.k.SetVault(s.ctx, types.NewVault(admin, vault1Addr, "stake")))
// 				s.Require().NoError(s.k.SetVault(s.ctx, types.NewVault(admin, vault2Addr, "nhash")))
// 				s.Require().NoError(s.k.SetVault(s.ctx, types.NewVault(admin, vault3Addr, "usdf")))
// 			},
// 			Req: &types.QueryVaultsRequest{
// 				Pagination: &query.PageRequest{Offset: 1},
// 			},
// 			ExpectedResp: &types.QueryVaultsResponse{
// 				Vaults: []types.Vault{
// 					*types.NewVault(admin, vault3Addr, "usdf"),
// 					*types.NewVault(admin, vault2Addr, "nhash"),
// 				},
// 				Pagination: &query.PageResponse{Total: 3},
// 			},
// 		},
// 		{
// 			Name: "pagination - offset starts at correct location and enforces limit",
// 			Setup: func() {
// 				s.Require().NoError(s.k.SetVault(s.ctx, types.NewVault(admin, vault1Addr, "stake")))
// 				s.Require().NoError(s.k.SetVault(s.ctx, types.NewVault(admin, vault2Addr, "nhash")))
// 				s.Require().NoError(s.k.SetVault(s.ctx, types.NewVault(admin, vault3Addr, "usdf")))
// 			},
// 			Req: &types.QueryVaultsRequest{
// 				Pagination: &query.PageRequest{Offset: 2, Limit: 1},
// 			},
// 			ExpectedResp: &types.QueryVaultsResponse{
// 				Vaults: []types.Vault{
// 					*types.NewVault(admin, vault2Addr, "nhash"),
// 				},
// 				Pagination: &query.PageResponse{},
// 			},
// 		},
// 		{
// 			Name: "pagination - enabled count total",
// 			Setup: func() {
// 				s.Require().NoError(s.k.SetVault(s.ctx, types.NewVault(admin, vault1Addr, "stake")))
// 				s.Require().NoError(s.k.SetVault(s.ctx, types.NewVault(admin, vault2Addr, "nhash")))
// 				s.Require().NoError(s.k.SetVault(s.ctx, types.NewVault(admin, vault3Addr, "usdf")))
// 			},
// 			Req: &types.QueryVaultsRequest{
// 				Pagination: &query.PageRequest{CountTotal: true},
// 			},
// 			ExpectedResp: &types.QueryVaultsResponse{
// 				Vaults: []types.Vault{
// 					*types.NewVault(admin, vault1Addr, "stake"),
// 					*types.NewVault(admin, vault3Addr, "usdf"),
// 					*types.NewVault(admin, vault2Addr, "nhash"),
// 				},
// 				Pagination: &query.PageResponse{Total: 3},
// 			},
// 		},
// 		{
// 			Name: "pagination - reverse provides the results in reverse order",
// 			Setup: func() {
// 				s.Require().NoError(s.k.SetVault(s.ctx, types.NewVault(admin, vault1Addr, "stake")))
// 				s.Require().NoError(s.k.SetVault(s.ctx, types.NewVault(admin, vault2Addr, "nhash")))
// 				s.Require().NoError(s.k.SetVault(s.ctx, types.NewVault(admin, vault3Addr, "usdf")))
// 			},
// 			Req: &types.QueryVaultsRequest{
// 				Pagination: &query.PageRequest{Reverse: true, Limit: 2},
// 			},
// 			ExpectedResp: &types.QueryVaultsResponse{
// 				Vaults: []types.Vault{
// 					*types.NewVault(admin, vault2Addr, "nhash"),
// 					*types.NewVault(admin, vault3Addr, "usdf"),
// 				},
// 				Pagination: &query.PageResponse{
// 					NextKey: []byte{0x6b, 0xd4, 0x73, 0x07, 0xd6, 0x2f, 0xda, 0x7b, 0x2e, 0x33, 0x60, 0xae, 0xba, 0x1c, 0x2d, 0x4d, 0x49, 0x97, 0x68, 0x84},
// 				},
// 			},
// 		},
// 		{
// 			Name: "empty state",
// 			Setup: func() {
// 			},
// 			Req: &types.QueryVaultsRequest{},
// 			ExpectedResp: &types.QueryVaultsResponse{
// 				Vaults:     []types.Vault{},
// 				Pagination: &query.PageResponse{},
// 			},
// 		},
// 		{
// 			Name:               "nil request",
// 			Req:                nil,
// 			ExpectedErrSubstrs: []string{"invalid request"},
// 		},
// 	}

// 	for _, tc := range tests {
// 		s.Run(tc.Name, func() {
// 			querytest.RunTestCase(s, testDef, tc)
// 		})
// 	}
// }
