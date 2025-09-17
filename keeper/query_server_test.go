package keeper_test

import (
	"time"

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

			type vaultView struct {
				Address         string
				Admin           string
				ShareDenom      string
				UnderlyingAsset string
				PaymentDenom    string
				IsPaused        bool
			}

			toViews := func(vs []types.VaultAccount) []vaultView {
				out := make([]vaultView, 0, len(vs))
				for _, v := range vs {
					out = append(out, vaultView{
						Address:         v.GetAddress().String(),
						Admin:           v.GetAdmin(),
						ShareDenom:      v.GetShareDenom(),
						UnderlyingAsset: v.GetUnderlyingAsset(),
						PaymentDenom:    v.GetPaymentDenom(),
						IsPaused:        v.GetPaused(),
					})
				}
				return out
			}

			s.Assert().ElementsMatch(toViews(expected.Vaults), toViews(actual.Vaults), "vaults do not match")

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
			ExpectedErrSubstrs: []string{"unsupported deposit denom:"},
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
				Shares:       sharesToSwap.Amount,
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
				Shares:       math.NewInt(100),
				RedeemDenom:  "wrongdenom",
			},
			ExpectedErrSubstrs: []string{"unsupported redeem denom"},
		},
	}

	for _, tc := range tests {
		s.Run(tc.Name, func() {
			querytest.RunTestCase(s, testDef, tc)
		})
	}
}

func (s *TestSuite) TestQueryServer_PendingSwapOuts() {
	testDef := querytest.TestDef[types.QueryPendingSwapOutsRequest, types.QueryPendingSwapOutsResponse]{
		QueryName: "PendingSwapOuts",
		Query:     keeper.NewQueryServer(s.simApp.VaultKeeper).PendingSwapOuts,
		ManualEquality: func(s querytest.TestSuiter, expected, actual *types.QueryPendingSwapOutsResponse) {
			s.Require().NotNil(actual, "actual response should not be nil")
			s.Require().NotNil(expected, "expected response should not be nil")

			s.Require().Len(actual.PendingSwapOuts, len(expected.PendingSwapOuts), "unexpected number of pending swap outs returned")

			// Custom comparison to ignore time drift in tests
			for i := range expected.PendingSwapOuts {
				s.Assert().Equal(expected.PendingSwapOuts[i].RequestId, actual.PendingSwapOuts[i].RequestId, "request id mismatch for entry %d", i)
				s.Assert().Equal(expected.PendingSwapOuts[i].PendingSwapOut, actual.PendingSwapOuts[i].PendingSwapOut, "pending swap out mismatch for entry %d", i)
				s.Assert().WithinDuration(expected.PendingSwapOuts[i].Timeout, actual.PendingSwapOuts[i].Timeout, 1*time.Second, "timeout mismatch for entry %d", i)
			}

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

	// Define some swap outs for consistent testing
	addr1 := sdk.AccAddress("addr1_______________")
	addr2 := sdk.AccAddress("addr2_______________")
	addr3 := sdk.AccAddress("addr3_______________")
	vaultAddr := sdk.AccAddress("vault_address______")
	swapOut1 := &types.PendingSwapOut{Owner: addr1.String(), VaultAddress: vaultAddr.String(), RedeemDenom: "v_usdc", Shares: sdk.NewInt64Coin("v_share", 100)}
	swapOut2 := &types.PendingSwapOut{Owner: addr2.String(), VaultAddress: vaultAddr.String(), RedeemDenom: "v_usdc", Shares: sdk.NewInt64Coin("v_share", 200)}
	swapOut3 := &types.PendingSwapOut{Owner: addr3.String(), VaultAddress: vaultAddr.String(), RedeemDenom: "v_usdc", Shares: sdk.NewInt64Coin("v_share", 300)}

	payoutTime1 := time.Now().Add(1 * time.Hour)
	payoutTime2 := time.Now().Add(2 * time.Hour)
	payoutTime3 := time.Now().Add(3 * time.Hour)

	tests := []querytest.TestCase[types.QueryPendingSwapOutsRequest, types.QueryPendingSwapOutsResponse]{
		{
			Name: "happy path - single swap out",
			Setup: func() {
				_, err := s.k.PendingSwapOutQueue.Enqueue(s.ctx, payoutTime1.Unix(), swapOut1)
				s.Require().NoError(err)
			},
			Req: &types.QueryPendingSwapOutsRequest{},
			ExpectedResp: &types.QueryPendingSwapOutsResponse{
				PendingSwapOuts: []types.PendingSwapOutWithTimeout{
					{
						RequestId:      0,
						PendingSwapOut: *swapOut1,
						Timeout:        payoutTime1.Truncate(time.Second),
					},
				},
				Pagination: &query.PageResponse{Total: 1},
			},
		},
		{
			Name: "happy path - multiple swap outs",
			Setup: func() {
				_, err := s.k.PendingSwapOutQueue.Enqueue(s.ctx, payoutTime1.Unix(), swapOut1)
				s.Require().NoError(err)
				_, err = s.k.PendingSwapOutQueue.Enqueue(s.ctx, payoutTime2.Unix(), swapOut2)
				s.Require().NoError(err)
				_, err = s.k.PendingSwapOutQueue.Enqueue(s.ctx, payoutTime3.Unix(), swapOut3)
				s.Require().NoError(err)
			},
			Req: &types.QueryPendingSwapOutsRequest{},
			ExpectedResp: &types.QueryPendingSwapOutsResponse{
				PendingSwapOuts: []types.PendingSwapOutWithTimeout{
					{RequestId: 0, PendingSwapOut: *swapOut1, Timeout: payoutTime1.Truncate(time.Second)},
					{RequestId: 1, PendingSwapOut: *swapOut2, Timeout: payoutTime2.Truncate(time.Second)},
					{RequestId: 2, PendingSwapOut: *swapOut3, Timeout: payoutTime3.Truncate(time.Second)},
				},
				Pagination: &query.PageResponse{Total: 3},
			},
		},
		{
			Name: "pagination - limits the number of outputs",
			Setup: func() {
				_, err := s.k.PendingSwapOutQueue.Enqueue(s.ctx, payoutTime1.Unix(), swapOut1)
				s.Require().NoError(err)
				_, err = s.k.PendingSwapOutQueue.Enqueue(s.ctx, payoutTime2.Unix(), swapOut2)
				s.Require().NoError(err)
				_, err = s.k.PendingSwapOutQueue.Enqueue(s.ctx, payoutTime3.Unix(), swapOut3)
				s.Require().NoError(err)
			},
			Req: &types.QueryPendingSwapOutsRequest{
				Pagination: &query.PageRequest{Limit: 2, CountTotal: true},
			},
			ExpectedResp: &types.QueryPendingSwapOutsResponse{
				PendingSwapOuts: []types.PendingSwapOutWithTimeout{
					{RequestId: 0, PendingSwapOut: *swapOut1, Timeout: payoutTime1.Truncate(time.Second)},
					{RequestId: 1, PendingSwapOut: *swapOut2, Timeout: payoutTime2.Truncate(time.Second)},
				},
				Pagination: &query.PageResponse{
					NextKey: []byte("not nil"),
					Total:   3,
				},
			},
		},
		{
			Name: "pagination - offset starts at correct location",
			Setup: func() {
				_, err := s.k.PendingSwapOutQueue.Enqueue(s.ctx, payoutTime1.Unix(), swapOut1)
				s.Require().NoError(err)
				_, err = s.k.PendingSwapOutQueue.Enqueue(s.ctx, payoutTime2.Unix(), swapOut2)
				s.Require().NoError(err)
				_, err = s.k.PendingSwapOutQueue.Enqueue(s.ctx, payoutTime3.Unix(), swapOut3)
				s.Require().NoError(err)
			},
			Req: &types.QueryPendingSwapOutsRequest{
				Pagination: &query.PageRequest{Offset: 1, CountTotal: true},
			},
			ExpectedResp: &types.QueryPendingSwapOutsResponse{
				PendingSwapOuts: []types.PendingSwapOutWithTimeout{
					{RequestId: 1, PendingSwapOut: *swapOut2, Timeout: payoutTime2.Truncate(time.Second)},
					{RequestId: 2, PendingSwapOut: *swapOut3, Timeout: payoutTime3.Truncate(time.Second)},
				},
				Pagination: &query.PageResponse{Total: 3},
			},
		},
		{
			Name: "pagination - offset starts at correct location and enforces limit",
			Setup: func() {
				_, err := s.k.PendingSwapOutQueue.Enqueue(s.ctx, payoutTime1.Unix(), swapOut1)
				s.Require().NoError(err)
				_, err = s.k.PendingSwapOutQueue.Enqueue(s.ctx, payoutTime2.Unix(), swapOut2)
				s.Require().NoError(err)
				_, err = s.k.PendingSwapOutQueue.Enqueue(s.ctx, payoutTime3.Unix(), swapOut3)
				s.Require().NoError(err)
			},
			Req: &types.QueryPendingSwapOutsRequest{
				Pagination: &query.PageRequest{Offset: 2, Limit: 1, CountTotal: true},
			},
			ExpectedResp: &types.QueryPendingSwapOutsResponse{
				PendingSwapOuts: []types.PendingSwapOutWithTimeout{
					{RequestId: 2, PendingSwapOut: *swapOut3, Timeout: payoutTime3.Truncate(time.Second)},
				},
				Pagination: &query.PageResponse{Total: 3},
			},
		},
		{
			Name: "pagination - enabled count total",
			Setup: func() {
				_, err := s.k.PendingSwapOutQueue.Enqueue(s.ctx, payoutTime1.Unix(), swapOut1)
				s.Require().NoError(err)
				_, err = s.k.PendingSwapOutQueue.Enqueue(s.ctx, payoutTime2.Unix(), swapOut2)
				s.Require().NoError(err)
				_, err = s.k.PendingSwapOutQueue.Enqueue(s.ctx, payoutTime3.Unix(), swapOut3)
				s.Require().NoError(err)
			},
			Req: &types.QueryPendingSwapOutsRequest{
				Pagination: &query.PageRequest{CountTotal: true},
			},
			ExpectedResp: &types.QueryPendingSwapOutsResponse{
				PendingSwapOuts: []types.PendingSwapOutWithTimeout{
					{RequestId: 0, PendingSwapOut: *swapOut1, Timeout: payoutTime1.Truncate(time.Second)},
					{RequestId: 1, PendingSwapOut: *swapOut2, Timeout: payoutTime2.Truncate(time.Second)},
					{RequestId: 2, PendingSwapOut: *swapOut3, Timeout: payoutTime3.Truncate(time.Second)},
				},
				Pagination: &query.PageResponse{Total: 3},
			},
		},
		{
			Name: "pagination - reverse provides the results in reverse order",
			Setup: func() {
				_, err := s.k.PendingSwapOutQueue.Enqueue(s.ctx, payoutTime1.Unix(), swapOut1)
				s.Require().NoError(err)
				_, err = s.k.PendingSwapOutQueue.Enqueue(s.ctx, payoutTime2.Unix(), swapOut2)
				s.Require().NoError(err)
				_, err = s.k.PendingSwapOutQueue.Enqueue(s.ctx, payoutTime3.Unix(), swapOut3)
				s.Require().NoError(err)
			},
			Req: &types.QueryPendingSwapOutsRequest{
				Pagination: &query.PageRequest{Reverse: true, Limit: 2, CountTotal: true},
			},
			ExpectedResp: &types.QueryPendingSwapOutsResponse{
				PendingSwapOuts: []types.PendingSwapOutWithTimeout{
					{RequestId: 2, PendingSwapOut: *swapOut3, Timeout: payoutTime3.Truncate(time.Second)},
					{RequestId: 1, PendingSwapOut: *swapOut2, Timeout: payoutTime2.Truncate(time.Second)},
				},
				Pagination: &query.PageResponse{
					NextKey: []byte("not nil"),
					Total:   3,
				},
			},
		},
		{
			Name: "empty state",
			Setup: func() {
			},
			Req: &types.QueryPendingSwapOutsRequest{},
			ExpectedResp: &types.QueryPendingSwapOutsResponse{
				PendingSwapOuts: []types.PendingSwapOutWithTimeout{},
				Pagination:      &query.PageResponse{},
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
