package keeper_test

import (
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
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
				s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin("stake2", 1), s.adminAddr)
				s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin("stake3", 1), s.adminAddr)
				_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{Admin: admin, ShareDenom: shareDenom1, UnderlyingAsset: "stake2"})
				s.Require().NoError(err)
				_, err = s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{Admin: admin, ShareDenom: shareDenom2, UnderlyingAsset: "stake3"})
				s.Require().NoError(err)
			},
			Req: &types.QueryVaultRequest{VaultAddress: addr2.String()},
			ExpectedResp: &types.QueryVaultResponse{
				Vault: *types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr2), admin, shareDenom2, []string{"stake3"}),
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
				s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin("stake2", 1), s.adminAddr)
				_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{Admin: admin, ShareDenom: shareDenom1, UnderlyingAsset: "stake2"})
				s.Require().NoError(err)
			},
			Req: &types.QueryVaultsRequest{},
			ExpectedResp: &types.QueryVaultsResponse{
				Vaults: []types.VaultAccount{
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr1), admin, shareDenom1, []string{"stake2"}),
				},
				Pagination: &query.PageResponse{Total: 1},
			},
		},
		{
			Name: "happy path - multiple vaults",
			Setup: func() {
				s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin("stake2", 1), s.adminAddr)
				s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin("nhash", 1), s.adminAddr)
				s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin("usdf", 1), s.adminAddr)
				_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{Admin: admin, ShareDenom: shareDenom1, UnderlyingAsset: "stake2"})
				s.Require().NoError(err)
				_, err = s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{Admin: admin, ShareDenom: shareDenom2, UnderlyingAsset: "nhash"})
				s.Require().NoError(err)
				_, err = s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{Admin: admin, ShareDenom: shareDenom3, UnderlyingAsset: "usdf"})
				s.Require().NoError(err)
			},
			Req: &types.QueryVaultsRequest{},
			ExpectedResp: &types.QueryVaultsResponse{
				Vaults: []types.VaultAccount{
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr1), admin, shareDenom1, []string{"stake2"}),
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr2), admin, shareDenom2, []string{"nhash"}),
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr3), admin, shareDenom3, []string{"usdf"}),
				},
				Pagination: &query.PageResponse{Total: 3},
			},
		},
		{
			Name: "pagination - limits the number of outputs",
			Setup: func() {
				s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin("stake2", 1), s.adminAddr)
				s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin("nhash", 1), s.adminAddr)
				s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin("usdf", 1), s.adminAddr)
				_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{Admin: admin, ShareDenom: shareDenom1, UnderlyingAsset: "stake2"})
				s.Require().NoError(err)
				_, err = s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{Admin: admin, ShareDenom: shareDenom2, UnderlyingAsset: "nhash"})
				s.Require().NoError(err)
				_, err = s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{Admin: admin, ShareDenom: shareDenom3, UnderlyingAsset: "usdf"})
				s.Require().NoError(err)
			},
			Req: &types.QueryVaultsRequest{
				Pagination: &query.PageRequest{Limit: 2},
			},
			ExpectedResp: &types.QueryVaultsResponse{
				Vaults: []types.VaultAccount{
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr3), admin, shareDenom3, []string{"usdf"}),
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr1), admin, shareDenom1, []string{"stake2"}),
				},
				Pagination: &query.PageResponse{
					NextKey: []byte("not nil"),
				},
			},
		},
		{
			Name: "pagination - offset starts at correct location",
			Setup: func() {
				s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin("stake2", 1), s.adminAddr)
				s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin("nhash", 1), s.adminAddr)
				s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin("usdf", 1), s.adminAddr)
				_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{Admin: admin, ShareDenom: shareDenom1, UnderlyingAsset: "stake2"})
				s.Require().NoError(err)
				_, err = s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{Admin: admin, ShareDenom: shareDenom2, UnderlyingAsset: "nhash"})
				s.Require().NoError(err)
				_, err = s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{Admin: admin, ShareDenom: shareDenom3, UnderlyingAsset: "usdf"})
				s.Require().NoError(err)
			},
			Req: &types.QueryVaultsRequest{
				Pagination: &query.PageRequest{Offset: 1},
			},
			ExpectedResp: &types.QueryVaultsResponse{
				Vaults: []types.VaultAccount{
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr1), admin, shareDenom1, []string{"stake2"}),
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr2), admin, shareDenom2, []string{"nhash"}),
				},
				Pagination: &query.PageResponse{Total: 3},
			},
		},
		{
			Name: "pagination - offset starts at correct location and enforces limit",
			Setup: func() {
				s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin("stake2", 1), s.adminAddr)
				s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin("nhash", 1), s.adminAddr)
				s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin("usdf", 1), s.adminAddr)
				_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{Admin: admin, ShareDenom: shareDenom1, UnderlyingAsset: "stake2"})
				s.Require().NoError(err)
				_, err = s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{Admin: admin, ShareDenom: shareDenom2, UnderlyingAsset: "nhash"})
				s.Require().NoError(err)
				_, err = s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{Admin: admin, ShareDenom: shareDenom3, UnderlyingAsset: "usdf"})
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
				s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin("stake2", 1), s.adminAddr)
				s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin("nhash", 1), s.adminAddr)
				s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin("usdf", 1), s.adminAddr)
				_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{Admin: admin, ShareDenom: shareDenom1, UnderlyingAsset: "stake2"})
				s.Require().NoError(err)
				_, err = s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{Admin: admin, ShareDenom: shareDenom2, UnderlyingAsset: "nhash"})
				s.Require().NoError(err)
				_, err = s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{Admin: admin, ShareDenom: shareDenom3, UnderlyingAsset: "usdf"})
				s.Require().NoError(err)
			},
			Req: &types.QueryVaultsRequest{
				Pagination: &query.PageRequest{CountTotal: true},
			},
			ExpectedResp: &types.QueryVaultsResponse{
				Vaults: []types.VaultAccount{
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr1), admin, shareDenom1, []string{"stake2"}),
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr2), admin, shareDenom2, []string{"nhash"}),
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr3), admin, shareDenom3, []string{"usdf"}),
				},
				Pagination: &query.PageResponse{Total: 3},
			},
		},
		{
			Name: "pagination - reverse provides the results in reverse order",
			Setup: func() {
				s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin("stake2", 1), s.adminAddr)
				s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin("nhash", 1), s.adminAddr)
				s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin("usdf", 1), s.adminAddr)
				_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{Admin: admin, ShareDenom: shareDenom1, UnderlyingAsset: "stake2"})
				s.Require().NoError(err)
				_, err = s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{Admin: admin, ShareDenom: shareDenom2, UnderlyingAsset: "nhash"})
				s.Require().NoError(err)
				_, err = s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{Admin: admin, ShareDenom: shareDenom3, UnderlyingAsset: "usdf"})
				s.Require().NoError(err)
			},
			Req: &types.QueryVaultsRequest{
				Pagination: &query.PageRequest{Reverse: true, Limit: 2},
			},
			ExpectedResp: &types.QueryVaultsResponse{
				Vaults: []types.VaultAccount{
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr2), admin, shareDenom2, []string{"nhash"}),
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr1), admin, shareDenom1, []string{"stake2"}),
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

func (s *TestSuite) TestQueryServer_EstimateSwapIn() {
	testDef := querytest.TestDef[types.QueryEstimateSwapInRequest, types.QueryEstimateSwapInResponse]{
		QueryName: "EstimateSwapIn",
		Query:     keeper.NewQueryServer(s.simApp.VaultKeeper).EstimateSwapIn,
	}

	underlyingDenom := "underlying"
	shareDenom := "vaultshares"
	vaultAddr := types.GetVaultAddress(shareDenom)
	admin := s.adminAddr.String()
	assets := sdk.NewInt64Coin(underlyingDenom, 100)

	tests := []querytest.TestCase[types.QueryEstimateSwapInRequest, types.QueryEstimateSwapInResponse]{
		{
			Name: "happy path",
			Setup: func() {
				s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlyingDenom, math.NewInt(1000)), s.adminAddr)
				_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
					Admin:           admin,
					ShareDenom:      shareDenom,
					UnderlyingAsset: underlyingDenom,
				})
				s.Require().NoError(err)
			},
			Req: &types.QueryEstimateSwapInRequest{
				VaultAddress: vaultAddr.String(),
				Assets:       assets,
			},
			ExpectedResp: &types.QueryEstimateSwapInResponse{
				Assets: sdk.NewCoin(shareDenom, assets.Amount),
			},
		},
		{
			Name:               "nil request",
			Req:                nil,
			ExpectedErrSubstrs: []string{"invalid request"},
		},
		{
			Name:               "empty vault address",
			Req:                &types.QueryEstimateSwapInRequest{VaultAddress: ""},
			ExpectedErrSubstrs: []string{"vault_address must be provided"},
		},
		{
			Name:               "invalid vault address",
			Req:                &types.QueryEstimateSwapInRequest{VaultAddress: "invalid-bech32-address"},
			ExpectedErrSubstrs: []string{"invalid vault_address", "decoding bech32 failed"},
		},
		{
			Name:               "vault not found",
			Req:                &types.QueryEstimateSwapInRequest{VaultAddress: vaultAddr.String()},
			ExpectedErrSubstrs: []string{"vault with address", "not found"},
		},
		{
			Name: "invalid asset denom",
			Setup: func() {
				s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlyingDenom, math.NewInt(1000)), s.adminAddr)
				_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
					Admin:           admin,
					ShareDenom:      shareDenom,
					UnderlyingAsset: underlyingDenom,
				})
				s.Require().NoError(err)
			},
			Req: &types.QueryEstimateSwapInRequest{
				VaultAddress: vaultAddr.String(),
				Assets:       sdk.NewInt64Coin("wrongdenom", 100),
			},
			ExpectedErrSubstrs: []string{"invalid asset for vault", "asset denom not supported for vault"},
		},
	}

	for _, tc := range tests {
		s.Run(tc.Name, func() {
			querytest.RunTestCase(s, testDef, tc)
		})
	}
}

func (s *TestSuite) TestQueryServer_EstimateSwapOut() {
	testDef := querytest.TestDef[types.QueryEstimateSwapOutRequest, types.QueryEstimateSwapOutResponse]{
		QueryName: "EstimateSwapOut",
		Query:     keeper.NewQueryServer(s.simApp.VaultKeeper).EstimateSwapOut,
	}

	underlyingDenom := "underlying"
	shareDenom := "vaultshares"
	vaultAddr := types.GetVaultAddress(shareDenom)
	admin := s.adminAddr.String()
	sharesToSwap := sdk.NewInt64Coin(shareDenom, 100)

	tests := []querytest.TestCase[types.QueryEstimateSwapOutRequest, types.QueryEstimateSwapOutResponse]{
		{
			Name: "happy path",
			Setup: func() {
				s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlyingDenom, math.NewInt(1000)), s.adminAddr)
				_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
					Admin:           admin,
					ShareDenom:      shareDenom,
					UnderlyingAsset: underlyingDenom,
				})
				s.Require().NoError(err)
			},
			Req: &types.QueryEstimateSwapOutRequest{
				VaultAddress: vaultAddr.String(),
				Assets:       sharesToSwap,
			},
			ExpectedResp: &types.QueryEstimateSwapOutResponse{
				Assets: sdk.NewCoin(underlyingDenom, sharesToSwap.Amount),
			},
		},
		{
			Name:               "nil request",
			Req:                nil,
			ExpectedErrSubstrs: []string{"invalid request"},
		},
		{
			Name:               "empty vault address",
			Req:                &types.QueryEstimateSwapOutRequest{VaultAddress: ""},
			ExpectedErrSubstrs: []string{"vault_address must be provided"},
		},
		{
			Name:               "invalid vault address",
			Req:                &types.QueryEstimateSwapOutRequest{VaultAddress: "invalid-bech32-address"},
			ExpectedErrSubstrs: []string{"invalid vault_address", "decoding bech32 failed"},
		},
		{
			Name:               "vault not found",
			Req:                &types.QueryEstimateSwapOutRequest{VaultAddress: vaultAddr.String()},
			ExpectedErrSubstrs: []string{"vault with address", "not found"},
		},
		{
			Name: "invalid asset denom",
			Setup: func() {
				s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlyingDenom, math.NewInt(1000)), s.adminAddr)
				_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
					Admin:           admin,
					ShareDenom:      shareDenom,
					UnderlyingAsset: underlyingDenom,
				})
				s.Require().NoError(err)
			},
			Req: &types.QueryEstimateSwapOutRequest{
				VaultAddress: vaultAddr.String(),
				Assets:       sdk.NewInt64Coin("wrongdenom", 100),
			},
			ExpectedErrSubstrs: []string{"asset denom", "does not match vault share denom"},
		},
	}

	for _, tc := range tests {
		s.Run(tc.Name, func() {
			querytest.RunTestCase(s, testDef, tc)
		})
	}
}
