package keeper_test

import (
	"github.com/cosmos/cosmos-sdk/types/query"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/types"
	querytest "github.com/provlabs/vault/utils/query"
)

func (s *TestSuite) TestQueryServer_Vault() {
	testDef := querytest.TestDef[types.QueryVaultRequest, types.QueryVaultResponse]{
		QueryName: "Vault",
		Query:     keeper.NewQueryServer(s.simApp.VaultKeeper).Vault,
		ManualEquality: func(s querytest.TestSuiter, expected, actual *types.QueryVaultResponse) {
			s.Require().NotNil(actual, "actual response should not be nil")
			s.Require().NotNil(expected, "expected response should not be nil")

			// Can't do a direct compare because of account numbers.
			s.Assert().Equal(expected.Vault.Address, actual.Vault.Address, "vault address")
			s.Assert().Equal(expected.Vault.Admin, actual.Vault.Admin, "vault admin")
			s.Assert().Equal(expected.Vault.ShareDenom, actual.Vault.ShareDenom, "vault share denom")
			s.Assert().Equal(expected.Vault.UnderlyingAssets, actual.Vault.UnderlyingAssets, "vault underlying assets")
		},
	}

	shareDenom1 := "vault1"
	shareDenom2 := "vault2"
	shareDenom3 := "vault3"
	addr2 := types.GetVaultAddress(shareDenom2)
	addr3 := types.GetVaultAddress(shareDenom3)
	admin := s.adminAddr.String()

	tests := []querytest.TestCase[types.QueryVaultRequest, types.QueryVaultResponse]{
		{
			Name: "vault found",
			Setup: func() {
				_, err := s.k.CreateVaultAccount(s.ctx, admin, shareDenom1, "stake2")
				s.Require().NoError(err)
				_, err = s.k.CreateVaultAccount(s.ctx, admin, shareDenom2, "stake")
				s.Require().NoError(err)
			},
			Req: &types.QueryVaultRequest{VaultAddress: addr2.String()},
			ExpectedResp: &types.QueryVaultResponse{
				Vault: *types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr2), admin, shareDenom2, []string{"stake"}),
			},
		},
		{
			Name:               "nil request",
			Req:                nil,
			ExpectedErrSubstrs: []string{"vault_address must be provided"},
		},
		{
			Name:               "empty vault address",
			Req:                &types.QueryVaultRequest{VaultAddress: ""},
			ExpectedErrSubstrs: []string{"vault_address must be provided"},
		},
		{
			Name:               "invalid vault address",
			Req:                &types.QueryVaultRequest{VaultAddress: "invalid-bech32-address"},
			ExpectedErrSubstrs: []string{"invalid vault_address", "decoding bech32 failed"},
		},
		{
			Name:               "vault not found",
			Req:                &types.QueryVaultRequest{VaultAddress: addr3.String()},
			ExpectedErrSubstrs: []string{"vault with address", "not found"},
		},
	}

	for _, tc := range tests {
		s.Run(tc.Name, func() {
			querytest.RunTestCase(s, testDef, tc)
		})
	}
}

// TestQueryServer_Vaults tests the Vaults query endpoint.
func (s *TestSuite) TestQueryServer_Vaults() {
	testDef := querytest.TestDef[types.QueryVaultsRequest, types.QueryVaultsResponse]{
		QueryName: "Vaults",
		Query:     keeper.NewQueryServer(s.simApp.VaultKeeper).Vaults,
		ManualEquality: func(s querytest.TestSuiter, expected, actual *types.QueryVaultsResponse) {
			s.Require().NotNil(actual, "actual response should not be nil")
			s.Require().NotNil(expected, "expected response should not be nil")

			s.Require().Len(actual.Vaults, len(expected.Vaults), "unexpected number of vaults returned")

			// Can't do a direct compare because of account numbers and ordering.
			// So we ignore account numbers and sequence and use ElementsMatch.
			actualCloned := make([]types.VaultAccount, len(actual.Vaults))
			for i, v := range actual.Vaults {
				cloned := v.Clone()
				cloned.BaseAccount.AccountNumber = 0
				cloned.BaseAccount.Sequence = 0
				actualCloned[i] = *cloned
			}

			expectedCloned := make([]types.VaultAccount, len(expected.Vaults))
			for i, v := range expected.Vaults {
				cloned := v.Clone()
				cloned.BaseAccount.AccountNumber = 0
				cloned.BaseAccount.Sequence = 0
				expectedCloned[i] = *cloned
			}

			s.Assert().ElementsMatch(expectedCloned, actualCloned, "vaults do not match")

			if expected.Pagination != nil {
				if expected.Pagination.Total > 0 {
					s.Assert().Equal(expected.Pagination.Total, actual.Pagination.Total, "pagination total")
				}
				if len(expected.Pagination.NextKey) > 0 {
					s.Assert().NotEmpty(actual.Pagination.NextKey, "pagination next_key should not be empty")
				} else {
					s.Assert().Empty(actual.Pagination.NextKey, "pagination next_key should be empty")
				}
			}
		},
	}

	admin := s.adminAddr.String()

	// Define some vault addresses for consistent testing
	shareDenom1 := "vault1"
	addr1 := types.GetVaultAddress(shareDenom1)
	shareDenom2 := "vault2"
	addr2 := types.GetVaultAddress(shareDenom2)
	shareDenom3 := "vault3"
	addr3 := types.GetVaultAddress(shareDenom3)

	tests := []querytest.TestCase[types.QueryVaultsRequest, types.QueryVaultsResponse]{
		{
			Name: "happy path - single vault",
			Setup: func() {
				_, err := s.k.CreateVaultAccount(s.ctx, admin, shareDenom1, "stake")
				s.Require().NoError(err)
			},
			Req: &types.QueryVaultsRequest{},
			ExpectedResp: &types.QueryVaultsResponse{
				Vaults: []types.VaultAccount{
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr1), admin, shareDenom1, []string{"stake"}),
				},
				Pagination: &query.PageResponse{Total: 1},
			},
		},
		{
			Name: "happy path - multiple vaults",
			Setup: func() {
				_, err := s.k.CreateVaultAccount(s.ctx, admin, shareDenom1, "stake")
				s.Require().NoError(err)
				_, err = s.k.CreateVaultAccount(s.ctx, admin, shareDenom2, "nhash")
				s.Require().NoError(err)
				_, err = s.k.CreateVaultAccount(s.ctx, admin, shareDenom3, "usdf")
				s.Require().NoError(err)
			},
			Req: &types.QueryVaultsRequest{},
			ExpectedResp: &types.QueryVaultsResponse{
				Vaults: []types.VaultAccount{
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr1), admin, shareDenom1, []string{"stake"}),
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr2), admin, shareDenom2, []string{"nhash"}),
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr3), admin, shareDenom3, []string{"usdf"}),
				},
				Pagination: &query.PageResponse{Total: 3},
			},
		},
		{
			Name: "pagination - limits the number of outputs",
			Setup: func() {
				_, err := s.k.CreateVaultAccount(s.ctx, admin, shareDenom1, "stake")
				s.Require().NoError(err)
				_, err = s.k.CreateVaultAccount(s.ctx, admin, shareDenom2, "nhash")
				s.Require().NoError(err)
				_, err = s.k.CreateVaultAccount(s.ctx, admin, shareDenom3, "usdf")
				s.Require().NoError(err)
			},
			Req: &types.QueryVaultsRequest{
				Pagination: &query.PageRequest{Limit: 2},
			},
			ExpectedResp: &types.QueryVaultsResponse{
				Vaults: []types.VaultAccount{
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr3), admin, shareDenom3, []string{"usdf"}),
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr1), admin, shareDenom1, []string{"stake"}),
				},
				Pagination: &query.PageResponse{
					NextKey: []byte("not nil"),
				},
			},
		},
		{
			Name: "pagination - offset starts at correct location",
			Setup: func() {
				_, err := s.k.CreateVaultAccount(s.ctx, admin, shareDenom1, "stake")
				s.Require().NoError(err)
				_, err = s.k.CreateVaultAccount(s.ctx, admin, shareDenom2, "nhash")
				s.Require().NoError(err)
				_, err = s.k.CreateVaultAccount(s.ctx, admin, shareDenom3, "usdf")
				s.Require().NoError(err)
			},
			Req: &types.QueryVaultsRequest{
				Pagination: &query.PageRequest{Offset: 1},
			},
			ExpectedResp: &types.QueryVaultsResponse{
				Vaults: []types.VaultAccount{
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr1), admin, shareDenom1, []string{"stake"}),
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr2), admin, shareDenom2, []string{"nhash"}),
				},
				Pagination: &query.PageResponse{Total: 3},
			},
		},
		{
			Name: "pagination - offset starts at correct location and enforces limit",
			Setup: func() {
				_, err := s.k.CreateVaultAccount(s.ctx, admin, shareDenom1, "stake")
				s.Require().NoError(err)
				_, err = s.k.CreateVaultAccount(s.ctx, admin, shareDenom2, "nhash")
				s.Require().NoError(err)
				_, err = s.k.CreateVaultAccount(s.ctx, admin, shareDenom3, "usdf")
				s.Require().NoError(err)
			},
			Req: &types.QueryVaultsRequest{
				Pagination: &query.PageRequest{Offset: 2, Limit: 1},
			},
			ExpectedResp: &types.QueryVaultsResponse{
				Vaults: []types.VaultAccount{
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr2), admin, shareDenom2, []string{"nhash"}),
				},
				Pagination: &query.PageResponse{},
			},
		},
		{
			Name: "pagination - enabled count total",
			Setup: func() {
				_, err := s.k.CreateVaultAccount(s.ctx, admin, shareDenom1, "stake")
				s.Require().NoError(err)
				_, err = s.k.CreateVaultAccount(s.ctx, admin, shareDenom2, "nhash")
				s.Require().NoError(err)
				_, err = s.k.CreateVaultAccount(s.ctx, admin, shareDenom3, "usdf")
				s.Require().NoError(err)
			},
			Req: &types.QueryVaultsRequest{
				Pagination: &query.PageRequest{CountTotal: true},
			},
			ExpectedResp: &types.QueryVaultsResponse{
				Vaults: []types.VaultAccount{
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr1), admin, shareDenom1, []string{"stake"}),
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr2), admin, shareDenom2, []string{"nhash"}),
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr3), admin, shareDenom3, []string{"usdf"}),
				},
				Pagination: &query.PageResponse{Total: 3},
			},
		},
		{
			Name: "pagination - reverse provides the results in reverse order",
			Setup: func() {
				_, err := s.k.CreateVaultAccount(s.ctx, admin, shareDenom1, "stake")
				s.Require().NoError(err)
				_, err = s.k.CreateVaultAccount(s.ctx, admin, shareDenom2, "nhash")
				s.Require().NoError(err)
				_, err = s.k.CreateVaultAccount(s.ctx, admin, shareDenom3, "usdf")
				s.Require().NoError(err)
			},
			Req: &types.QueryVaultsRequest{
				Pagination: &query.PageRequest{Reverse: true, Limit: 2},
			},
			ExpectedResp: &types.QueryVaultsResponse{
				Vaults: []types.VaultAccount{
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr2), admin, shareDenom2, []string{"nhash"}),
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr1), admin, shareDenom1, []string{"stake"}),
				},
				Pagination: &query.PageResponse{
					NextKey: []byte("not nil"),
				},
			},
		},
		{
			Name: "empty state",
			Setup: func() {
			},
			Req: &types.QueryVaultsRequest{},
			ExpectedResp: &types.QueryVaultsResponse{
				Vaults:     []types.VaultAccount{},
				Pagination: &query.PageResponse{},
			},
		},
		{
			Name:               "nil request",
			Req:                nil,
			ExpectedErrSubstrs: []string{"invalid request"},
		},
	}

	for _, tc := range tests {
		s.Run(tc.Name, func() {
			querytest.RunTestCase(s, testDef, tc)
		})
	}
}
