package keeper_test

import (
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	"github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/types"
	"github.com/provlabs/vault/utils"
	querytest "github.com/provlabs/vault/utils/query"

	markertypes "github.com/provenance-io/provenance/x/marker/types"
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
			s.Assert().Equal(expected.Vault.UnderlyingAsset, actual.Vault.UnderlyingAsset, "vault underlying asset")

			s.Assert().Equal(expected.Principal.Address, actual.Principal.Address, "principal address")
			s.Assert().Equal(expected.Principal.Coins, actual.Principal.Coins, "principal coins")
			s.Assert().Equal(expected.Reserves.Address, actual.Reserves.Address, "reserves address")
			s.Assert().Equal(expected.Reserves.Coins, actual.Reserves.Coins, "reserves coins")
		},
	}

	shareDenom1 := "vault1"
	addr1 := types.GetVaultAddress(shareDenom1)
	shareDenom2 := "vault2"
	addr2 := types.GetVaultAddress(shareDenom2)
	markerAddr1 := markertypes.MustGetMarkerAddress(shareDenom1)
	markerAddr2 := markertypes.MustGetMarkerAddress(shareDenom2)
	nonExistentAddr := sdk.AccAddress("nonExistentAddr_____")
	admin := s.adminAddr.String()

	// Common setup for tests that need existing vaults.
	setupVaults := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin("stake1", 1), s.adminAddr)
		s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin("stake2", 1), s.adminAddr)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{Admin: admin, ShareDenom: shareDenom1, UnderlyingAsset: "stake1"})
		s.Require().NoError(err)
		_, err = s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{Admin: admin, ShareDenom: shareDenom2, UnderlyingAsset: "stake2"})
		s.Require().NoError(err)

		// Fund vault 1 reserves and principal
		s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, addr1, sdk.NewCoins(sdk.NewInt64Coin("stake1", 100))))
		s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, markerAddr1, sdk.NewCoins(sdk.NewInt64Coin("stake1", 1000))))

		// Fund vault 2 reserves and principal
		s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, addr2, sdk.NewCoins(sdk.NewInt64Coin("stake2", 200))))
		s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, markerAddr2, sdk.NewCoins(sdk.NewInt64Coin("stake2", 2000))))
	}

	tests := []querytest.TestCase[types.QueryVaultRequest, types.QueryVaultResponse]{
		{
			Name:  "vault found by address",
			Setup: setupVaults,
			Req:   &types.QueryVaultRequest{Id: addr1.String()},
			ExpectedResp: &types.QueryVaultResponse{
				Vault: *types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr1), admin, shareDenom1, "stake1", "usdc", 0),
				Principal: types.AccountBalance{
					Address: markerAddr1.String(),
					Coins:   sdk.NewCoins(sdk.NewInt64Coin("stake1", 1000)),
				},
				Reserves: types.AccountBalance{
					Address: addr1.String(),
					Coins:   sdk.NewCoins(sdk.NewInt64Coin("stake1", 100)),
				},
			},
		},
		{
			Name:  "vault found by share denom",
			Setup: setupVaults,
			Req:   &types.QueryVaultRequest{Id: shareDenom2},
			ExpectedResp: &types.QueryVaultResponse{
				Vault: *types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr2), admin, shareDenom2, "stake2", "usdc", 0),
				Principal: types.AccountBalance{
					Address: markerAddr2.String(),
					Coins:   sdk.NewCoins(sdk.NewInt64Coin("stake2", 2000)),
				},
				Reserves: types.AccountBalance{
					Address: addr2.String(),
					Coins:   sdk.NewCoins(sdk.NewInt64Coin("stake2", 200)),
				},
			},
		},
		{
			Name:               "nil request",
			Req:                nil,
			ExpectedErrSubstrs: []string{"id must be provided"},
		},
		{
			Name:               "empty vault id",
			Req:                &types.QueryVaultRequest{Id: ""},
			ExpectedErrSubstrs: []string{"id must be provided"},
		},
		{
			Name:               "vault not found by address",
			Setup:              setupVaults,
			Req:                &types.QueryVaultRequest{Id: nonExistentAddr.String()},
			ExpectedErrSubstrs: []string{"not found"},
		},
		{
			Name:               "vault not found by share denom",
			Setup:              setupVaults,
			Req:                &types.QueryVaultRequest{Id: "nonexistent-share"},
			ExpectedErrSubstrs: []string{"not found"},
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
				_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
					Admin:           admin,
					ShareDenom:      shareDenom1,
					UnderlyingAsset: "stake2",
					PaymentDenom:    "usdc",
				})
				s.Require().NoError(err)
			},
			Req: &types.QueryVaultsRequest{},
			ExpectedResp: &types.QueryVaultsResponse{
				Vaults: []types.VaultAccount{
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr1), admin, shareDenom1, "stake2", "usdc", 0),
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
				_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
					Admin:           admin,
					ShareDenom:      shareDenom1,
					UnderlyingAsset: "stake2",
					PaymentDenom:    "usdc",
				})
				s.Require().NoError(err)
				_, err = s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
					Admin:           admin,
					ShareDenom:      shareDenom2,
					UnderlyingAsset: "nhash",
					PaymentDenom:    "usdc",
				})
				s.Require().NoError(err)
				_, err = s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
					Admin:           admin,
					ShareDenom:      shareDenom3,
					UnderlyingAsset: "usdf",
					PaymentDenom:    "usdc",
				})
				s.Require().NoError(err)
			},
			Req: &types.QueryVaultsRequest{},
			ExpectedResp: &types.QueryVaultsResponse{
				Vaults: []types.VaultAccount{
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr1), admin, shareDenom1, "stake2", "usdc", 0),
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr2), admin, shareDenom2, "nhash", "usdc", 0),
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr3), admin, shareDenom3, "usdf", "usdc", 0),
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
				_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
					Admin:           admin,
					ShareDenom:      shareDenom1,
					UnderlyingAsset: "stake2",
					PaymentDenom:    "usdc",
				})
				s.Require().NoError(err)
				_, err = s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
					Admin:           admin,
					ShareDenom:      shareDenom2,
					UnderlyingAsset: "nhash",
					PaymentDenom:    "usdc",
				})
				s.Require().NoError(err)
				_, err = s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
					Admin:           admin,
					ShareDenom:      shareDenom3,
					UnderlyingAsset: "usdf",
					PaymentDenom:    "usdc",
				})
				s.Require().NoError(err)
			},
			Req: &types.QueryVaultsRequest{
				Pagination: &query.PageRequest{Limit: 2},
			},
			ExpectedResp: &types.QueryVaultsResponse{
				Vaults: []types.VaultAccount{
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr3), admin, shareDenom3, "usdf", "usdc", 0),
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr1), admin, shareDenom1, "stake2", "usdc", 0),
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
				_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
					Admin:           admin,
					ShareDenom:      shareDenom1,
					UnderlyingAsset: "stake2",
					PaymentDenom:    "usdc",
				})
				s.Require().NoError(err)
				_, err = s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
					Admin:           admin,
					ShareDenom:      shareDenom2,
					UnderlyingAsset: "nhash",
					PaymentDenom:    "usdc",
				})
				s.Require().NoError(err)
				_, err = s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
					Admin:           admin,
					ShareDenom:      shareDenom3,
					UnderlyingAsset: "usdf",
					PaymentDenom:    "usdc",
				})
				s.Require().NoError(err)
			},
			Req: &types.QueryVaultsRequest{
				Pagination: &query.PageRequest{Offset: 1},
			},
			ExpectedResp: &types.QueryVaultsResponse{
				Vaults: []types.VaultAccount{
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr1), admin, shareDenom1, "stake2", "usdc", 0),
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr2), admin, shareDenom2, "nhash", "usdc", 0),
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
				_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
					Admin:           admin,
					ShareDenom:      shareDenom1,
					UnderlyingAsset: "stake2",
					PaymentDenom:    "usdc",
				})
				s.Require().NoError(err)
				_, err = s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
					Admin:           admin,
					ShareDenom:      shareDenom2,
					UnderlyingAsset: "nhash",
					PaymentDenom:    "usdc",
				})
				s.Require().NoError(err)
				_, err = s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
					Admin:           admin,
					ShareDenom:      shareDenom3,
					UnderlyingAsset: "usdf",
					PaymentDenom:    "usdc",
				})
				s.Require().NoError(err)
			},
			Req: &types.QueryVaultsRequest{
				Pagination: &query.PageRequest{Offset: 2, Limit: 1},
			},
			ExpectedResp: &types.QueryVaultsResponse{
				Vaults: []types.VaultAccount{
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr2), admin, shareDenom2, "nhash", "usdc", 0),
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
				_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
					Admin:           admin,
					ShareDenom:      shareDenom1,
					UnderlyingAsset: "stake2",
					PaymentDenom:    "usdc",
				})
				s.Require().NoError(err)
				_, err = s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
					Admin:           admin,
					ShareDenom:      shareDenom2,
					UnderlyingAsset: "nhash",
					PaymentDenom:    "usdc",
				})
				s.Require().NoError(err)
				_, err = s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
					Admin:           admin,
					ShareDenom:      shareDenom3,
					UnderlyingAsset: "usdf",
					PaymentDenom:    "usdc",
				})
				s.Require().NoError(err)
			},
			Req: &types.QueryVaultsRequest{
				Pagination: &query.PageRequest{CountTotal: true},
			},
			ExpectedResp: &types.QueryVaultsResponse{
				Vaults: []types.VaultAccount{
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr1), admin, shareDenom1, "stake2", "usdc", 0),
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr2), admin, shareDenom2, "nhash", "usdc", 0),
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr3), admin, shareDenom3, "usdf", "usdc", 0),
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
				_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
					Admin:           admin,
					ShareDenom:      shareDenom1,
					UnderlyingAsset: "stake2",
					PaymentDenom:    "usdc",
				})
				s.Require().NoError(err)
				_, err = s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
					Admin:           admin,
					ShareDenom:      shareDenom2,
					UnderlyingAsset: "nhash",
					PaymentDenom:    "usdc",
				})
				s.Require().NoError(err)
				_, err = s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
					Admin:           admin,
					ShareDenom:      shareDenom3,
					UnderlyingAsset: "usdf",
					PaymentDenom:    "usdc",
				})
				s.Require().NoError(err)
			},
			Req: &types.QueryVaultsRequest{
				Pagination: &query.PageRequest{Reverse: true, Limit: 2},
			},
			ExpectedResp: &types.QueryVaultsResponse{
				Vaults: []types.VaultAccount{
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr2), admin, shareDenom2, "nhash", "usdc", 0),
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr1), admin, shareDenom1, "stake2", "usdc", 0),
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
				s.Require().NoError(err, "vault creation should succeed")
			},
			Req: &types.QueryEstimateSwapInRequest{
				VaultAddress: vaultAddr.String(),
				Assets:       assets,
			},
			ExpectedResp: &types.QueryEstimateSwapInResponse{
				Assets: sdk.NewCoin(shareDenom, assets.Amount.Mul(utils.ShareScalar)),
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
				s.Require().NoError(err, "vault creation should succeed")
			},
			Req: &types.QueryEstimateSwapInRequest{
				VaultAddress: vaultAddr.String(),
				Assets:       sdk.NewInt64Coin("wrongdenom", 100),
			},
			ExpectedErrSubstrs: []string{"denom not supported for vault must be of type"},
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
	sharesToSwap := sdk.NewCoin(shareDenom, math.NewInt(100).Mul(utils.ShareScalar)) // 100 * ShareScalar

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
				s.Require().NoError(err, "vault creation should succeed")

				err = FundAccount(s.ctx, s.simApp.BankKeeper, markertypes.MustGetMarkerAddress(shareDenom), sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, 100)))
				s.Require().NoError(err, "funding marker with underlying should succeed")

				err = FundAccount(s.ctx, s.simApp.BankKeeper, s.adminAddr, sdk.NewCoins(sharesToSwap))
				s.Require().NoError(err, "funding owner with scaled shares should succeed")
			},
			Req: &types.QueryEstimateSwapOutRequest{
				VaultAddress: vaultAddr.String(),
				Assets:       sharesToSwap,
			},
			ExpectedResp: &types.QueryEstimateSwapOutResponse{
				Assets: sdk.NewInt64Coin(underlyingDenom, 100), // ~exact with current offsets
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
				s.Require().NoError(err, "vault creation should succeed")
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
