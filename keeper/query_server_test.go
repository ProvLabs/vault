package keeper_test

import (
	"strings"
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/provenance-io/provenance/x/exchange"
	markertypes "github.com/provenance-io/provenance/x/marker/types"

	"github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/types"
	"github.com/provlabs/vault/utils"
	querytest "github.com/provlabs/vault/utils/query"
)

func (s *TestSuite) TestQueryServer_Vault() {
	testDef := querytest.TestDef[types.QueryVaultRequest, types.QueryVaultResponse]{
		QueryName: "Vault",
		Query:     keeper.NewQueryServer(s.simApp.VaultKeeper).Vault,
		ManualEquality: func(s querytest.TestSuiter, expected, actual *types.QueryVaultResponse) {
			s.Require().NotNil(actual, "actual response should not be nil")
			s.Require().NotNil(expected, "expected response should not be nil")
			s.Assert().Equal(expected.Vault.Address, actual.Vault.Address, "vault address")
			s.Assert().Equal(expected.Vault.Admin, actual.Vault.Admin, "vault admin")
			s.Assert().Equal(expected.Vault.TotalShares, actual.Vault.TotalShares, "vault total shares")
			s.Assert().Equal(expected.Vault.UnderlyingAsset, actual.Vault.UnderlyingAsset, "vault underlying asset")
			s.Assert().Equal(expected.Vault.AumFeeBips, actual.Vault.AumFeeBips, "vault AUM fee bips")
			s.Assert().Equal(expected.Vault.MinSwapInValue, actual.Vault.MinSwapInValue, "vault MinSwapInValue")
			s.Assert().Equal(expected.Vault.MinSwapOutValue, actual.Vault.MinSwapOutValue, "vault MinSwapOutValue")
			s.Assert().Equal(expected.Vault.MaxSwapInValue, actual.Vault.MaxSwapInValue, "vault MaxSwapInValue")
			s.Assert().Equal(expected.Vault.MaxSwapOutValue, actual.Vault.MaxSwapOutValue, "vault MaxSwapOutValue")
			s.Assert().Equal(expected.Principal.Address, actual.Principal.Address, "principal address")
			s.Assert().Equal(expected.Principal.Coins, actual.Principal.Coins, "principal coins")
			s.Assert().Equal(expected.Reserves.Address, actual.Reserves.Address, "reserves address")
			s.Assert().Equal(expected.Reserves.Coins, actual.Reserves.Coins, "reserves coins")
			s.Assert().Equal(expected.TotalVaultValue, actual.TotalVaultValue, "total vault value")
		},
	}

	underlying := "uylds.fcc"
	heldAsset := "usdc"

	shareDenom1 := "vault1"
	addr1 := types.GetVaultAddress(shareDenom1)
	shareDenom2 := "vault2"
	addr2 := types.GetVaultAddress(shareDenom2)
	markerAddr1 := markertypes.MustGetMarkerAddress(shareDenom1)
	markerAddr2 := markertypes.MustGetMarkerAddress(shareDenom2)
	nonExistentAddr := sdk.AccAddress("nonExistentAddr_____")
	admin := s.adminAddr.String()

	setupVaults := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlying, 1), s.adminAddr)
		s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(heldAsset, 1), s.adminAddr)
		vault1 := s.createSingleDenomVault(vaultAttrs{admin: admin, share: shareDenom1, underlying: underlying})
		vault2 := s.createSingleDenomVault(vaultAttrs{admin: admin, share: shareDenom2, underlying: underlying})
		s.setVaultNAV(vault1, heldAsset, sdk.NewInt64Coin(underlying, 1), 1)
		s.setVaultNAV(vault2, heldAsset, sdk.NewInt64Coin(underlying, 1), 1)
		s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, addr1, sdk.NewCoins(sdk.NewInt64Coin(underlying, 40))), "fund reserves for vault1 should succeed")
		s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, markerAddr1, sdk.NewCoins(sdk.NewInt64Coin(underlying, 100), sdk.NewInt64Coin(heldAsset, 250))), "fund principal (marker) for vault1 should succeed")
		s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, addr2, sdk.NewCoins(sdk.NewInt64Coin(underlying, 20))), "fund reserves for vault2 should succeed")
		s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, markerAddr2, sdk.NewCoins(sdk.NewInt64Coin(underlying, 200), sdk.NewInt64Coin(heldAsset, 100))), "fund principal (marker) for vault2 should succeed")
	}

	tests := []querytest.TestCase[types.QueryVaultRequest, types.QueryVaultResponse]{
		{
			Name:  "vault found by address (held asset priced via internal NAV)",
			Setup: setupVaults,
			Req:   &types.QueryVaultRequest{Id: addr1.String()},
			ExpectedResp: &types.QueryVaultResponse{
				Vault: *types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr1), admin, shareDenom1, underlying, 0, 15, "", "", "", ""),
				Principal: types.AccountBalance{
					Address: markerAddr1.String(),
					Coins:   sdk.NewCoins(sdk.NewInt64Coin(underlying, 100), sdk.NewInt64Coin(heldAsset, 250)),
				},
				Reserves: types.AccountBalance{
					Address: addr1.String(),
					Coins:   sdk.NewCoins(sdk.NewInt64Coin(underlying, 40)),
				},
				TotalVaultValue: sdk.NewInt64Coin(underlying, 350),
			},
		},
		{
			Name:  "vault found by share denom (held asset priced via internal NAV)",
			Setup: setupVaults,
			Req:   &types.QueryVaultRequest{Id: shareDenom2},
			ExpectedResp: &types.QueryVaultResponse{
				Vault: *types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr2), admin, shareDenom2, underlying, 0, 15, "", "", "", ""),
				Principal: types.AccountBalance{
					Address: markerAddr2.String(),
					Coins:   sdk.NewCoins(sdk.NewInt64Coin(underlying, 200), sdk.NewInt64Coin(heldAsset, 100)),
				},
				Reserves: types.AccountBalance{
					Address: addr2.String(),
					Coins:   sdk.NewCoins(sdk.NewInt64Coin(underlying, 20)),
				},
				TotalVaultValue: sdk.NewInt64Coin(underlying, 300),
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
				AumFeeBips      uint32
				IsPaused        bool
				MinSwapInValue  string
				MinSwapOutValue string
				MaxSwapInValue  string
				MaxSwapOutValue string
			}

			toViews := func(vs []types.VaultAccount) []vaultView {
				out := make([]vaultView, 0, len(vs))
				for _, v := range vs {
					out = append(out, vaultView{
						Address:         v.GetAddress().String(),
						Admin:           v.GetAdmin(),
						ShareDenom:      v.GetTotalShares().Denom,
						UnderlyingAsset: v.GetUnderlyingAsset(),
						PaymentDenom:    v.GetPaymentDenom(),
						AumFeeBips:      v.AumFeeBips,
						IsPaused:        v.GetPaused(),
						MinSwapInValue:  v.MinSwapInValue,
						MinSwapOutValue: v.MinSwapOutValue,
						MaxSwapInValue:  v.MaxSwapInValue,
						MaxSwapOutValue: v.MaxSwapOutValue,
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

	setupThreeVaults := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin("stake2", 1), s.adminAddr)
		s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin("nhash", 1), s.adminAddr)
		s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin("usdf", 1), s.adminAddr)
		s.CreateVaultWithParams(shareDenom1, "stake2")
		s.CreateVaultWithParams(shareDenom2, "nhash")
		s.CreateVaultWithParams(shareDenom3, "usdf")
	}

	tests := []querytest.TestCase[types.QueryVaultsRequest, types.QueryVaultsResponse]{
		{
			Name: "happy path - single vault",
			Setup: func() {
				s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin("stake2", 1), s.adminAddr)
				s.CreateVaultWithParams(shareDenom1, "stake2")
			},
			Req: &types.QueryVaultsRequest{},
			ExpectedResp: &types.QueryVaultsResponse{
				Vaults: []types.VaultAccount{
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr1), admin, shareDenom1, "stake2", 0, 15, "", "", "", ""),
				},
				Pagination: &query.PageResponse{Total: 1},
			},
		},
		{
			Name:  "happy path - multiple vaults",
			Setup: setupThreeVaults,
			Req:   &types.QueryVaultsRequest{},
			ExpectedResp: &types.QueryVaultsResponse{
				Vaults: []types.VaultAccount{
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr1), admin, shareDenom1, "stake2", 0, 15, "", "", "", ""),
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr2), admin, shareDenom2, "nhash", 0, 15, "", "", "", ""),
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr3), admin, shareDenom3, "usdf", 0, 15, "", "", "", ""),
				},
				Pagination: &query.PageResponse{Total: 3},
			},
		},
		{
			Name:  "pagination - limits the number of outputs",
			Setup: setupThreeVaults,
			Req: &types.QueryVaultsRequest{
				Pagination: &query.PageRequest{Limit: 2},
			},
			ExpectedResp: &types.QueryVaultsResponse{
				Vaults: []types.VaultAccount{
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr3), admin, shareDenom3, "usdf", 0, 15, "", "", "", ""),
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr1), admin, shareDenom1, "stake2", 0, 15, "", "", "", ""),
				},
				Pagination: &query.PageResponse{
					NextKey: []byte("not nil"),
				},
			},
		},
		{
			Name:  "pagination - offset starts at correct location",
			Setup: setupThreeVaults,
			Req: &types.QueryVaultsRequest{
				Pagination: &query.PageRequest{Offset: 1},
			},
			ExpectedResp: &types.QueryVaultsResponse{
				Vaults: []types.VaultAccount{
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr1), admin, shareDenom1, "stake2", 0, 15, "", "", "", ""),
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr2), admin, shareDenom2, "nhash", 0, 15, "", "", "", ""),
				},
				Pagination: &query.PageResponse{Total: 3},
			},
		},
		{
			Name:  "pagination - offset starts at correct location and enforces limit",
			Setup: setupThreeVaults,
			Req: &types.QueryVaultsRequest{
				Pagination: &query.PageRequest{Offset: 2, Limit: 1},
			},
			ExpectedResp: &types.QueryVaultsResponse{
				Vaults: []types.VaultAccount{
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr2), admin, shareDenom2, "nhash", 0, 15, "", "", "", ""),
				},
				Pagination: &query.PageResponse{},
			},
		},
		{
			Name:  "pagination - enabled count total",
			Setup: setupThreeVaults,
			Req: &types.QueryVaultsRequest{
				Pagination: &query.PageRequest{CountTotal: true},
			},
			ExpectedResp: &types.QueryVaultsResponse{
				Vaults: []types.VaultAccount{
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr1), admin, shareDenom1, "stake2", 0, 15, "", "", "", ""),
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr2), admin, shareDenom2, "nhash", 0, 15, "", "", "", ""),
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr3), admin, shareDenom3, "usdf", 0, 15, "", "", "", ""),
				},
				Pagination: &query.PageResponse{Total: 3},
			},
		},
		{
			Name:  "pagination - reverse provides the results in reverse order",
			Setup: setupThreeVaults,
			Req: &types.QueryVaultsRequest{
				Pagination: &query.PageRequest{Reverse: true, Limit: 2},
			},
			ExpectedResp: &types.QueryVaultsResponse{
				Vaults: []types.VaultAccount{
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr2), admin, shareDenom2, "nhash", 0, 15, "", "", "", ""),
					*types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(addr1), admin, shareDenom1, "stake2", 0, 15, "", "", "", ""),
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
		ManualEquality: func(s querytest.TestSuiter, expected, actual *types.QueryEstimateSwapInResponse) {
			s.Require().NotNil(actual, "actual response should not be nil")
			s.Require().NotNil(expected, "expected response should not be nil")
			s.Assert().Equal(expected.Assets, actual.Assets, "assets mismatch")
		},
	}

	underlyingDenom := "uylds.fcc"
	shareDenom := "vaultshares"
	vaultAddr := types.GetVaultAddress(shareDenom)
	assetsUnderlying := sdk.NewInt64Coin(underlyingDenom, 100)

	setupVault := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlyingDenom, math.NewInt(1000)), s.adminAddr)
		s.CreateVaultWithParams(shareDenom, underlyingDenom)
	}

	tests := []querytest.TestCase[types.QueryEstimateSwapInRequest, types.QueryEstimateSwapInResponse]{
		{
			Name:  "happy path underlying deposit (peg)",
			Setup: setupVault,
			Req: &types.QueryEstimateSwapInRequest{
				VaultAddress: vaultAddr.String(),
				Assets:       assetsUnderlying,
			},
			ExpectedResp: &types.QueryEstimateSwapInResponse{
				Assets: sdk.NewCoin(shareDenom, assetsUnderlying.Amount.Mul(utils.ShareScalar)),
			},
		},
		{
			Name: "fails if vault is paused",
			Setup: func() {
				setupVault()
				vault, err := s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err, "getting vault should succeed")
				vault.Paused = true
				s.k.AuthKeeper.SetAccount(s.ctx, vault)
			},
			Req: &types.QueryEstimateSwapInRequest{
				VaultAddress: vaultAddr.String(),
				Assets:       assetsUnderlying,
			},
			ExpectedErrSubstrs: []string{"swap-in disabled or vault paused", "FailedPrecondition"},
		},
		{
			Name: "fails if swap-in is disabled",
			Setup: func() {
				setupVault()
				vault, err := s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err, "getting vault should succeed")
				vault.SwapInEnabled = false
				s.k.AuthKeeper.SetAccount(s.ctx, vault)
			},
			Req: &types.QueryEstimateSwapInRequest{
				VaultAddress: vaultAddr.String(),
				Assets:       assetsUnderlying,
			},
			ExpectedErrSubstrs: []string{"swap-in disabled or vault paused", "FailedPrecondition"},
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
			Name:  "unsupported denom",
			Setup: setupVault,
			Req: &types.QueryEstimateSwapInRequest{
				VaultAddress: vaultAddr.String(),
				Assets:       sdk.NewInt64Coin("otherdenom", 100),
			},
			ExpectedErrSubstrs: []string{"unsupported deposit denom"},
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
		ManualEquality: func(s querytest.TestSuiter, expected, actual *types.QueryEstimateSwapOutResponse) {
			s.Require().NotNil(actual, "actual response should not be nil")
			s.Require().NotNil(expected, "expected response should not be nil")
			s.Assert().Equal(expected.Assets, actual.Assets, "assets mismatch")
		},
	}

	underlyingDenom := "uylds.fcc"
	shareDenom := "vaultshares"
	vaultAddr := types.GetVaultAddress(shareDenom)
	sharesToSwap := sdk.NewCoin(shareDenom, math.NewInt(100).Mul(utils.ShareScalar))

	setupVault := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlyingDenom, math.NewInt(1000)), s.adminAddr)
		s.CreateVaultWithParams(shareDenom, underlyingDenom)
	}

	setupFundedVault := func() {
		setupVault()
		err := FundAccount(s.ctx, s.simApp.BankKeeper, markertypes.MustGetMarkerAddress(shareDenom), sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, 100)))
		s.Require().NoError(err, "fund marker with underlying should succeed")
		err = FundAccount(s.ctx, s.simApp.BankKeeper, s.adminAddr, sdk.NewCoins(sharesToSwap))
		s.Require().NoError(err, "fund owner with shares should succeed")
		vault, err := s.k.GetVault(s.ctx, vaultAddr)
		s.Require().NoError(err, "get vault should succeed")
		s.Require().NotNil(vault, "vault should not be nil")
		vault.TotalShares = sharesToSwap
		s.k.AuthKeeper.SetAccount(s.ctx, vault)
	}

	tests := []querytest.TestCase[types.QueryEstimateSwapOutRequest, types.QueryEstimateSwapOutResponse]{
		{
			Name:  "happy path redeem to underlying (peg)",
			Setup: setupFundedVault,
			Req: &types.QueryEstimateSwapOutRequest{
				VaultAddress: vaultAddr.String(),
				Shares:       sharesToSwap.Amount.String(),
			},
			ExpectedResp: &types.QueryEstimateSwapOutResponse{
				Assets: sdk.NewInt64Coin(underlyingDenom, 100),
			},
		},
		{
			Name: "fails if vault is paused",
			Setup: func() {
				setupVault()
				vault, err := s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err, "getting vault should succeed")
				vault.Paused = true
				s.k.AuthKeeper.SetAccount(s.ctx, vault)
			},
			Req: &types.QueryEstimateSwapOutRequest{
				VaultAddress: vaultAddr.String(),
				Shares:       sharesToSwap.Amount.String(),
			},
			ExpectedErrSubstrs: []string{"swap-out disabled or vault paused", "FailedPrecondition"},
		},
		{
			Name: "fails if swap-out is disabled",
			Setup: func() {
				setupVault()
				vault, err := s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err, "getting vault should succeed")
				vault.SwapOutEnabled = false
				s.k.AuthKeeper.SetAccount(s.ctx, vault)
			},
			Req: &types.QueryEstimateSwapOutRequest{
				VaultAddress: vaultAddr.String(),
				Shares:       sharesToSwap.Amount.String(),
			},
			ExpectedErrSubstrs: []string{"swap-out disabled or vault paused", "FailedPrecondition"},
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
			Name:  "unsupported redeem denom",
			Setup: setupVault,
			Req: &types.QueryEstimateSwapOutRequest{
				VaultAddress: vaultAddr.String(),
				Shares:       math.NewInt(100).String(),
				RedeemDenom:  "wrongdenom",
			},
			ExpectedErrSubstrs: []string{"unsupported redeem denom"},
		},
		{
			Name:  "fails with incorrect shares string",
			Setup: setupVault,
			Req: &types.QueryEstimateSwapOutRequest{
				VaultAddress: vaultAddr.String(),
				Shares:       "bogus",
			},
			ExpectedErrSubstrs: []string{"invalid shares amount \"bogus\" : must be a valid integer"},
		},
	}

	for _, tc := range tests {
		s.Run(tc.Name, func() {
			querytest.RunTestCase(s, testDef, tc)
		})
	}
}

func (s *TestSuite) TestQueryServer_VaultPendingSwapOuts() {
	testDef := querytest.TestDef[types.QueryVaultPendingSwapOutsRequest, types.QueryVaultPendingSwapOutsResponse]{
		QueryName: "VaultPendingSwapOuts",
		Query:     keeper.NewQueryServer(s.simApp.VaultKeeper).VaultPendingSwapOuts,
		ManualEquality: func(s querytest.TestSuiter, expected, actual *types.QueryVaultPendingSwapOutsResponse) {
			s.Require().NotNil(actual, "actual response should not be nil")
			s.Require().NotNil(expected, "expected response should not be nil")

			s.Require().Len(actual.PendingSwapOuts, len(expected.PendingSwapOuts), "unexpected number of pending swap outs returned")

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

	underlyingAsset := "stake_vpso"
	shareDenomA, shareDenomB := "vshare_a", "vshare_b"
	vaultAddrA := types.GetVaultAddress(shareDenomA)
	vaultAddrB := types.GetVaultAddress(shareDenomB)

	owner1, owner2, owner3, owner4 := sdk.AccAddress("owner1______________"), sdk.AccAddress("owner2______________"), sdk.AccAddress("owner3______________"), sdk.AccAddress("owner4______________")

	reqA1 := &types.PendingSwapOut{Owner: owner1.String(), VaultAddress: vaultAddrA.String(), Shares: sdk.NewInt64Coin(shareDenomA, 100), RedeemDenom: underlyingAsset}
	reqA2 := &types.PendingSwapOut{Owner: owner2.String(), VaultAddress: vaultAddrA.String(), Shares: sdk.NewInt64Coin(shareDenomA, 200), RedeemDenom: underlyingAsset}

	reqB1 := &types.PendingSwapOut{Owner: owner3.String(), VaultAddress: vaultAddrB.String(), Shares: sdk.NewInt64Coin(shareDenomB, 300), RedeemDenom: underlyingAsset}
	reqB2 := &types.PendingSwapOut{Owner: owner4.String(), VaultAddress: vaultAddrB.String(), Shares: sdk.NewInt64Coin(shareDenomB, 400), RedeemDenom: underlyingAsset}

	timeA1, timeA2 := time.Now().Add(1*time.Hour).Truncate(time.Second), time.Now().Add(2*time.Hour).Truncate(time.Second)
	timeB1, timeB2 := time.Now().Add(3*time.Hour).Truncate(time.Second), time.Now().Add(4*time.Hour).Truncate(time.Second)

	baseSetup := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlyingAsset, 1_000_000), s.adminAddr)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{Admin: s.adminAddr.String(), ShareDenom: shareDenomA, UnderlyingAsset: underlyingAsset})
		s.Require().NoError(err, "creating vault A")
		_, err = s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{Admin: s.adminAddr.String(), ShareDenom: shareDenomB, UnderlyingAsset: underlyingAsset})
		s.Require().NoError(err, "creating vault B")

		_, err = s.k.PendingSwapOutQueue.Enqueue(s.ctx, timeB1.Unix(), reqB1)
		s.Require().NoError(err, "populating pending swap out queue for vault B1")
		_, err = s.k.PendingSwapOutQueue.Enqueue(s.ctx, timeB2.Unix(), reqB2)
		s.Require().NoError(err, "populating pending swap out queue for vault B2")
	}

	tests := []querytest.TestCase[types.QueryVaultPendingSwapOutsRequest, types.QueryVaultPendingSwapOutsResponse]{
		{
			Name: "success - vault A has 1 entry",
			Setup: func() {
				baseSetup()
				_, err := s.k.PendingSwapOutQueue.Enqueue(s.ctx, timeA1.Unix(), reqA1)
				s.Require().NoError(err, "populating pending swap out queue for vault A1")
			},
			Req: &types.QueryVaultPendingSwapOutsRequest{Id: vaultAddrA.String()},
			ExpectedResp: &types.QueryVaultPendingSwapOutsResponse{
				PendingSwapOuts: []types.PendingSwapOutWithTimeout{
					{RequestId: 2, PendingSwapOut: *reqA1, Timeout: timeA1},
				},
				Pagination: &query.PageResponse{Total: 1},
			},
		},
		{
			Name: "success - vault A has 2 entries",
			Setup: func() {
				baseSetup()
				_, err := s.k.PendingSwapOutQueue.Enqueue(s.ctx, timeA1.Unix(), reqA1)
				s.Require().NoError(err, "populating pending swap out queue for vault A1")
				_, err = s.k.PendingSwapOutQueue.Enqueue(s.ctx, timeA2.Unix(), reqA2)
				s.Require().NoError(err, "populating pending swap out queue for vault A2")
			},
			Req: &types.QueryVaultPendingSwapOutsRequest{Id: vaultAddrA.String()},
			ExpectedResp: &types.QueryVaultPendingSwapOutsResponse{
				PendingSwapOuts: []types.PendingSwapOutWithTimeout{
					{RequestId: 2, PendingSwapOut: *reqA1, Timeout: timeA1},
					{RequestId: 3, PendingSwapOut: *reqA2, Timeout: timeA2},
				},
				Pagination: &query.PageResponse{Total: 2},
			},
		},
		{
			Name: "success - vault A has 2 entries with pagination",
			Setup: func() {
				baseSetup()
				_, err := s.k.PendingSwapOutQueue.Enqueue(s.ctx, timeA1.Unix(), reqA1)
				s.Require().NoError(err, "populating pending swap out queue for vault A1")
				_, err = s.k.PendingSwapOutQueue.Enqueue(s.ctx, timeA2.Unix(), reqA2)
				s.Require().NoError(err, "populating pending swap out queue for vault A2")
			},
			Req: &types.QueryVaultPendingSwapOutsRequest{
				Id:         vaultAddrA.String(),
				Pagination: &query.PageRequest{Limit: 1, CountTotal: true},
			},
			ExpectedResp: &types.QueryVaultPendingSwapOutsResponse{
				PendingSwapOuts: []types.PendingSwapOutWithTimeout{
					{RequestId: 2, PendingSwapOut: *reqA1, Timeout: timeA1},
				},
				Pagination: &query.PageResponse{NextKey: []byte("not nil"), Total: 2},
			},
		},
		{
			Name:  "success - vault A has 0 entries",
			Setup: baseSetup,
			Req:   &types.QueryVaultPendingSwapOutsRequest{Id: vaultAddrA.String()},
			ExpectedResp: &types.QueryVaultPendingSwapOutsResponse{
				PendingSwapOuts: []types.PendingSwapOutWithTimeout{},
				Pagination:      &query.PageResponse{Total: 0},
			},
		},
		{
			Name:               "failure - invalid vault id",
			Setup:              baseSetup,
			Req:                &types.QueryVaultPendingSwapOutsRequest{Id: "invalid-id"},
			ExpectedErrSubstrs: []string{"not found"},
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
				s.Require().NoError(err, "populating pending swap out queue")
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
				s.Require().NoError(err, "populating pending swap out queue for vault A1")
				_, err = s.k.PendingSwapOutQueue.Enqueue(s.ctx, payoutTime2.Unix(), swapOut2)
				s.Require().NoError(err, "populating pending swap out queue for vault A2")
				_, err = s.k.PendingSwapOutQueue.Enqueue(s.ctx, payoutTime3.Unix(), swapOut3)
				s.Require().NoError(err, "populating pending swap out queue for vault A3")
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
				s.Require().NoError(err, "populating pending swap out queue for vault A1")
				_, err = s.k.PendingSwapOutQueue.Enqueue(s.ctx, payoutTime2.Unix(), swapOut2)
				s.Require().NoError(err, "populating pending swap out queue for vault A2")
				_, err = s.k.PendingSwapOutQueue.Enqueue(s.ctx, payoutTime3.Unix(), swapOut3)
				s.Require().NoError(err, "populating pending swap out queue for vault A3")
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
				s.Require().NoError(err, "populating pending swap out queue for vault A1")
				_, err = s.k.PendingSwapOutQueue.Enqueue(s.ctx, payoutTime2.Unix(), swapOut2)
				s.Require().NoError(err, "populating pending swap out queue for vault A2")
				_, err = s.k.PendingSwapOutQueue.Enqueue(s.ctx, payoutTime3.Unix(), swapOut3)
				s.Require().NoError(err, "populating pending swap out queue for vault A3")
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
				s.Require().NoError(err, "populating pending swap out queue for vault A1")
				_, err = s.k.PendingSwapOutQueue.Enqueue(s.ctx, payoutTime2.Unix(), swapOut2)
				s.Require().NoError(err, "populating pending swap out queue for vault A2")
				_, err = s.k.PendingSwapOutQueue.Enqueue(s.ctx, payoutTime3.Unix(), swapOut3)
				s.Require().NoError(err, "populating pending swap out queue for vault A3")
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
				s.Require().NoError(err, "populating pending swap out queue for vault A1")
				_, err = s.k.PendingSwapOutQueue.Enqueue(s.ctx, payoutTime2.Unix(), swapOut2)
				s.Require().NoError(err, "populating pending swap out queue for vault A2")
				_, err = s.k.PendingSwapOutQueue.Enqueue(s.ctx, payoutTime3.Unix(), swapOut3)
				s.Require().NoError(err, "populating pending swap out queue for vault A3")
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
				s.Require().NoError(err, "populating pending swap out queue for vault A1")
				_, err = s.k.PendingSwapOutQueue.Enqueue(s.ctx, payoutTime2.Unix(), swapOut2)
				s.Require().NoError(err, "populating pending swap out queue for vault A2")
				_, err = s.k.PendingSwapOutQueue.Enqueue(s.ctx, payoutTime3.Unix(), swapOut3)
				s.Require().NoError(err, "populating pending swap out queue for vault A3")
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

// TestQueryServer_VaultNavs verifies the VaultNavs query returns every internal
// NAV entry for a vault, rejects malformed requests, and paginates the results.
// The vault fixture is shared across cases because every case is a read-only query.
func (s *TestSuite) TestQueryServer_VaultNavs() {
	underlying := "under"
	share := "vaultshares"
	vaultAddr := types.GetVaultAddress(share)

	vault := s.setupBaseVault(underlying, share)
	otherVault := s.setupBaseVault("under2", "vaultshares2")

	navDenoms := []string{"rwaa", "rwab", "rwac", "rwad", "rwae"}
	for _, denom := range navDenoms {
		s.requireSimpleMarker(denom)
	}
	s.requireSimpleMarker("otherrwa")
	for i, denom := range navDenoms {
		nav := types.VaultNAV{Denom: denom, Price: sdk.NewInt64Coin(underlying, int64(100+i)), Volume: math.NewInt(int64(i + 1))}
		s.Require().NoError(s.k.SetVaultNAV(s.ctx, vault, nav, s.adminAddr.String()), "failed to seed NAV for %s", denom)
	}
	otherNav := types.VaultNAV{Denom: "otherrwa", Price: sdk.NewInt64Coin("under2", 999), Volume: math.NewInt(1)}
	s.Require().NoError(s.k.SetVaultNAV(s.ctx, otherVault, otherNav, s.adminAddr.String()), "failed to seed NAV for other vault")

	queryServer := keeper.NewQueryServer(s.simApp.VaultKeeper)

	tests := []struct {
		name        string
		req         *types.QueryVaultNavsRequest
		expectErr   bool
		errContains string
		validate    func(resp *types.QueryVaultNavsResponse)
	}{
		{
			name:        "rejects empty id",
			req:         &types.QueryVaultNavsRequest{},
			expectErr:   true,
			errContains: "id must be provided",
		},
		{
			name:        "nil request returns InvalidArgument",
			req:         nil,
			expectErr:   true,
			errContains: "id must be provided",
		},
		{
			name:        "missing vault returns NotFound",
			req:         &types.QueryVaultNavsRequest{Id: types.GetVaultAddress("nope").String()},
			expectErr:   true,
			errContains: "not found",
		},
		{
			name: "page request that sets both key and offset is rejected",
			req: &types.QueryVaultNavsRequest{
				Id:         vaultAddr.String(),
				Pagination: &query.PageRequest{Key: []byte{0x01}, Offset: 1},
			},
			expectErr:   true,
			errContains: "failed to paginate vault navs",
		},
		{
			name: "returns every NAV for the vault",
			req:  &types.QueryVaultNavsRequest{Id: vaultAddr.String()},
			validate: func(resp *types.QueryVaultNavsResponse) {
				s.Require().Len(resp.Navs, len(navDenoms), "VaultNavs should return every NAV for the vault")
				s.Assert().ElementsMatch(navDenoms, navDenomsOf(resp.Navs), "VaultNavs returned unexpected denoms")
			},
		},
		{
			name: "first page respects the requested limit and returns a next key",
			req: &types.QueryVaultNavsRequest{
				Id:         vaultAddr.String(),
				Pagination: &query.PageRequest{Limit: 2},
			},
			validate: func(resp *types.QueryVaultNavsResponse) {
				s.Require().Len(resp.Navs, 2, "limit of 2 should return two entries")
				s.Assert().ElementsMatch([]string{"rwaa", "rwab"}, navDenomsOf(resp.Navs), "first page should return the two lowest denoms in key order")
				s.Require().NotNil(resp.Pagination, "paginated response should include pagination")
				s.Assert().NotEmpty(resp.Pagination.NextKey, "a partial page should report a next key")
			},
		},
		{
			name: "offset pagination skips entries and reports the total",
			req: &types.QueryVaultNavsRequest{
				Id:         vaultAddr.String(),
				Pagination: &query.PageRequest{Offset: 4, Limit: 10, CountTotal: true},
			},
			validate: func(resp *types.QueryVaultNavsResponse) {
				s.Require().Len(resp.Navs, 1, "offset of 4 over 5 entries should return one entry")
				s.Assert().ElementsMatch([]string{"rwae"}, navDenomsOf(resp.Navs), "offset of 4 should return the highest denom")
				s.Assert().Equal(uint64(len(navDenoms)), resp.Pagination.Total, "CountTotal should report every NAV for the vault")
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			resp, err := queryServer.VaultNavs(s.ctx, tc.req)
			if tc.expectErr {
				s.Require().Error(err, "VaultNavs should return an error for case %q", tc.name)
				s.Assert().Contains(err.Error(), tc.errContains, "VaultNavs error mismatch for case %q", tc.name)
				return
			}
			s.Require().NoError(err, "VaultNavs should succeed for case %q", tc.name)
			tc.validate(resp)
		})
	}
}

// TestQueryServer_NavValue verifies the NavValue query returns a single entry
// and reports NotFound for unknown vaults or denoms.
func (s *TestSuite) TestQueryServer_NavValue() {
	underlying := "under"
	share := "vaultshares"
	navDenom := "rwa"
	vaultAddr := types.GetVaultAddress(share)

	vault := s.setupBaseVault(underlying, share)
	s.requireSimpleMarker(navDenom)
	s.ctx = s.ctx.WithBlockHeight(42)
	price := sdk.NewInt64Coin(underlying, 1234)
	seeded := types.VaultNAV{Denom: navDenom, Price: price, Volume: math.NewInt(7), Source: "oracle-x"}
	s.Require().NoError(s.k.SetVaultNAV(s.ctx, vault, seeded, s.adminAddr.String()), "failed to seed NAV")

	queryServer := keeper.NewQueryServer(s.simApp.VaultKeeper)

	tests := []struct {
		name        string
		req         *types.QueryNavValueRequest
		expectErr   bool
		errContains string
		validate    func(resp *types.QueryNavValueResponse)
	}{
		{
			name: "returns the stored entry",
			req:  &types.QueryNavValueRequest{Id: vaultAddr.String(), Denom: navDenom},
			validate: func(resp *types.QueryNavValueResponse) {
				s.Assert().Equal(navDenom, resp.Nav.Denom, "NavValue denom mismatch")
				s.Assert().Equal(price, resp.Nav.Price, "NavValue price mismatch")
				s.Assert().Equal(math.NewInt(7), resp.Nav.Volume, "NavValue volume mismatch")
				s.Assert().Equal("oracle-x", resp.Nav.Source, "NavValue source mismatch")
				s.Assert().Equal(int64(42), resp.Nav.UpdatedBlockHeight, "NavValue block height mismatch")
			},
		},
		{
			name: "resolves the vault by its share denom",
			req:  &types.QueryNavValueRequest{Id: share, Denom: navDenom},
			validate: func(resp *types.QueryNavValueResponse) {
				s.Assert().Equal(price, resp.Nav.Price, "NavValue price mismatch")
			},
		},
		{
			name:        "unknown denom returns NotFound",
			req:         &types.QueryNavValueRequest{Id: vaultAddr.String(), Denom: "missing"},
			expectErr:   true,
			errContains: "no NAV entry",
		},
		{
			name:        "missing vault returns NotFound",
			req:         &types.QueryNavValueRequest{Id: types.GetVaultAddress("nope").String(), Denom: navDenom},
			expectErr:   true,
			errContains: "not found",
		},
		{
			name:        "rejects empty denom",
			req:         &types.QueryNavValueRequest{Id: vaultAddr.String()},
			expectErr:   true,
			errContains: "denom must be provided",
		},
		{
			name:        "rejects empty id",
			req:         &types.QueryNavValueRequest{Denom: navDenom},
			expectErr:   true,
			errContains: "id must be provided",
		},
		{
			name:        "nil request returns InvalidArgument",
			req:         nil,
			expectErr:   true,
			errContains: "id must be provided",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			resp, err := queryServer.NavValue(s.ctx, tc.req)
			if tc.expectErr {
				s.Require().Error(err, "NavValue should return an error for case %q", tc.name)
				s.Assert().Contains(err.Error(), tc.errContains, "NavValue error mismatch for case %q", tc.name)
				return
			}
			s.Require().NoError(err, "NavValue should succeed for case %q", tc.name)
			tc.validate(resp)
		})
	}
}

// TestQueryServer_VaultPayment verifies the VaultPayment query returns a single
// payment targeting the vault and reports NotFound for unknown or mistargeted payments.
func (s *TestSuite) TestQueryServer_VaultPayment() {
	underlying, share, asset := "under", "vshare", "rwacoin"

	vault, _ := s.setupAssetSettlementVault(underlying, share)
	vaultAddr := vault.GetAddress()

	source := s.CreateAndFundAccount(sdk.NewInt64Coin(asset, 10))
	sourceAmount := sdk.NewCoins(sdk.NewInt64Coin(asset, 10))
	targetAmount := sdk.NewCoins(sdk.NewInt64Coin(underlying, 5))
	s.createPayment(source, vaultAddr, sourceAmount, targetAmount, "invoice-1")

	otherSource := s.CreateAndFundAccount(sdk.NewInt64Coin(asset, 10))
	otherTarget := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1))
	s.createPayment(otherSource, otherTarget, sdk.NewCoins(sdk.NewInt64Coin(asset, 10)), nil, "elsewhere")

	queryServer := keeper.NewQueryServer(s.simApp.VaultKeeper)

	tests := []struct {
		name        string
		req         *types.QueryVaultPaymentRequest
		expectErr   bool
		errContains string
		validate    func(resp *types.QueryVaultPaymentResponse)
	}{
		{
			name: "returns the payment",
			req: &types.QueryVaultPaymentRequest{
				Id:         vaultAddr.String(),
				Source:     source.String(),
				ExternalId: "invoice-1",
			},
			validate: func(resp *types.QueryVaultPaymentResponse) {
				s.Assert().Equal(source.String(), resp.Payment.Source, "VaultPayment source mismatch")
				s.Assert().Equal(vaultAddr.String(), resp.Payment.Target, "VaultPayment target mismatch")
				s.Assert().Equal(sourceAmount, resp.Payment.SourceAmount, "VaultPayment source amount mismatch")
				s.Assert().Equal(targetAmount, resp.Payment.TargetAmount, "VaultPayment target amount mismatch")
				s.Assert().Equal("invoice-1", resp.Payment.ExternalId, "VaultPayment external id mismatch")
			},
		},
		{
			name: "resolves the vault by its share denom",
			req: &types.QueryVaultPaymentRequest{
				Id:         share,
				Source:     source.String(),
				ExternalId: "invoice-1",
			},
			validate: func(resp *types.QueryVaultPaymentResponse) {
				s.Assert().Equal("invoice-1", resp.Payment.ExternalId, "VaultPayment external id mismatch")
			},
		},
		{
			name: "unknown external id returns NotFound",
			req: &types.QueryVaultPaymentRequest{
				Id:         vaultAddr.String(),
				Source:     source.String(),
				ExternalId: "missing",
			},
			expectErr:   true,
			errContains: "no payment",
		},
		{
			name: "payment targeting another account returns NotFound",
			req: &types.QueryVaultPaymentRequest{
				Id:         vaultAddr.String(),
				Source:     otherSource.String(),
				ExternalId: "elsewhere",
			},
			expectErr:   true,
			errContains: "no payment",
		},
		{
			name: "missing vault returns NotFound",
			req: &types.QueryVaultPaymentRequest{
				Id:         types.GetVaultAddress("nope").String(),
				Source:     source.String(),
				ExternalId: "invoice-1",
			},
			expectErr:   true,
			errContains: "not found",
		},
		{
			name: "invalid source returns InvalidArgument",
			req: &types.QueryVaultPaymentRequest{
				Id:         vaultAddr.String(),
				Source:     "not-an-address",
				ExternalId: "invoice-1",
			},
			expectErr:   true,
			errContains: "invalid source",
		},
		{
			name:        "rejects empty source",
			req:         &types.QueryVaultPaymentRequest{Id: vaultAddr.String()},
			expectErr:   true,
			errContains: "source must be provided",
		},
		{
			name: "external id over the exchange length limit returns InvalidArgument",
			req: &types.QueryVaultPaymentRequest{
				Id:         vaultAddr.String(),
				Source:     source.String(),
				ExternalId: strings.Repeat("x", exchange.MaxExternalIDLength+1),
			},
			expectErr:   true,
			errContains: "invalid external id",
		},
		{
			name: "empty external id is a valid payment key, returns NotFound when no such payment exists",
			req: &types.QueryVaultPaymentRequest{
				Id:     vaultAddr.String(),
				Source: source.String(),
			},
			expectErr:   true,
			errContains: "no payment",
		},
		{
			name:        "rejects empty id",
			req:         &types.QueryVaultPaymentRequest{Source: source.String()},
			expectErr:   true,
			errContains: "id must be provided",
		},
		{
			name:        "nil request returns InvalidArgument",
			req:         nil,
			expectErr:   true,
			errContains: "id must be provided",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			resp, err := queryServer.VaultPayment(s.ctx, tc.req)
			if tc.expectErr {
				s.Require().Error(err, "VaultPayment should return an error for case %q", tc.name)
				s.Assert().Contains(err.Error(), tc.errContains, "VaultPayment error mismatch for case %q", tc.name)
				return
			}
			s.Require().NoError(err, "VaultPayment should succeed for case %q", tc.name)
			tc.validate(resp)
		})
	}
}

// TestQueryServer_VaultPayments verifies the VaultPayments query returns every payment
// targeting the vault, excludes payments for other targets, and paginates the results.
func (s *TestSuite) TestQueryServer_VaultPayments() {
	underlying, share, asset := "under", "vshare", "rwacoin"

	vault, _ := s.setupAssetSettlementVault(underlying, share)
	vaultAddr := vault.GetAddress()

	externalIDs := []string{"p-1", "p-2", "p-3", "p-4", "p-5"}
	for _, externalID := range externalIDs {
		source := s.CreateAndFundAccount(sdk.NewInt64Coin(asset, 10))
		s.createPayment(source, vaultAddr, sdk.NewCoins(sdk.NewInt64Coin(asset, 10)), sdk.NewCoins(sdk.NewInt64Coin(underlying, 5)), externalID)
	}

	otherSource := s.CreateAndFundAccount(sdk.NewInt64Coin(asset, 10))
	otherTarget := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1))
	s.createPayment(otherSource, otherTarget, sdk.NewCoins(sdk.NewInt64Coin(asset, 10)), nil, "elsewhere")

	queryServer := keeper.NewQueryServer(s.simApp.VaultKeeper)

	tests := []struct {
		name        string
		req         *types.QueryVaultPaymentsRequest
		expectErr   bool
		errContains string
		validate    func(resp *types.QueryVaultPaymentsResponse)
	}{
		{
			name:        "rejects empty id",
			req:         &types.QueryVaultPaymentsRequest{},
			expectErr:   true,
			errContains: "id must be provided",
		},
		{
			name:        "nil request returns InvalidArgument",
			req:         nil,
			expectErr:   true,
			errContains: "id must be provided",
		},
		{
			name:        "missing vault returns NotFound",
			req:         &types.QueryVaultPaymentsRequest{Id: types.GetVaultAddress("nope").String()},
			expectErr:   true,
			errContains: "not found",
		},
		{
			name: "returns every payment targeting the vault",
			req:  &types.QueryVaultPaymentsRequest{Id: vaultAddr.String()},
			validate: func(resp *types.QueryVaultPaymentsResponse) {
				s.Require().Len(resp.Payments, len(externalIDs), "VaultPayments should return every payment targeting the vault")
				for _, payment := range resp.Payments {
					s.Assert().Equal(vaultAddr.String(), payment.Target, "VaultPayments returned a payment for another target")
				}
				s.Assert().ElementsMatch(externalIDs, paymentExternalIDsOf(resp.Payments), "VaultPayments returned unexpected external ids")
			},
		},
		{
			name: "resolves the vault by its share denom",
			req:  &types.QueryVaultPaymentsRequest{Id: share},
			validate: func(resp *types.QueryVaultPaymentsResponse) {
				s.Assert().Len(resp.Payments, len(externalIDs), "VaultPayments should return every payment for the vault")
			},
		},
		{
			name: "first page respects the requested limit and returns a next key",
			req: &types.QueryVaultPaymentsRequest{
				Id:         vaultAddr.String(),
				Pagination: &query.PageRequest{Limit: 2},
			},
			validate: func(resp *types.QueryVaultPaymentsResponse) {
				s.Require().Len(resp.Payments, 2, "limit of 2 should return two entries")
				s.Require().NotNil(resp.Pagination, "paginated response should include pagination")
				s.Assert().NotEmpty(resp.Pagination.NextKey, "a partial page should report a next key")
			},
		},
		{
			name: "offset pagination skips entries and reports the total",
			req: &types.QueryVaultPaymentsRequest{
				Id:         vaultAddr.String(),
				Pagination: &query.PageRequest{Offset: 4, Limit: 10, CountTotal: true},
			},
			validate: func(resp *types.QueryVaultPaymentsResponse) {
				s.Require().Len(resp.Payments, 1, "offset of 4 over 5 entries should return one entry")
				s.Assert().Equal(uint64(len(externalIDs)), resp.Pagination.Total, "CountTotal should report every payment for the vault")
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			resp, err := queryServer.VaultPayments(s.ctx, tc.req)
			if tc.expectErr {
				s.Require().Error(err, "VaultPayments should return an error for case %q", tc.name)
				s.Assert().Contains(err.Error(), tc.errContains, "VaultPayments error mismatch for case %q", tc.name)
				return
			}
			s.Require().NoError(err, "VaultPayments should succeed for case %q", tc.name)
			tc.validate(resp)
		})
	}
}

// navDenomsOf extracts the denom of each NAV entry, preserving order.
func navDenomsOf(navs []types.VaultNAV) []string {
	denoms := make([]string, len(navs))
	for i, nav := range navs {
		denoms[i] = nav.Denom
	}
	return denoms
}

// paymentExternalIDsOf extracts the external id of each payment, preserving order.
func paymentExternalIDsOf(payments []types.Payment) []string {
	ids := make([]string, len(payments))
	for i, payment := range payments {
		ids[i] = payment.ExternalId
	}
	return ids
}
