package keeper_test

import (
	"context"
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	markertypes "github.com/provenance-io/provenance/x/marker/types"

	"github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/types"
)

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
			s.True(marker.GetMarkerType() == markertypes.MarkerType_Coin, "vault marker should be coin")
			s.False(marker.HasGovernanceEnabled(), "vault marker should not allow governance control")

			access := marker.GetAccessList()
			s.Len(access, 1)
			s.Equal(types.GetVaultAddress(postCheckArgs.ShareDenom).String(), access[0].Address, "vault marker access should be granted to vault account")
			s.ElementsMatch(
				[]markertypes.Access{
					markertypes.Access_Mint,
					markertypes.Access_Burn,
					markertypes.Access_Withdraw,
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
				sdk.NewAttribute("marker_type", "MARKER_TYPE_COIN"),
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
				sdk.NewAttribute("underlying_assets", "[\"undercoin\"]"),
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

func (s *TestSuite) TestMsgServer_SwapIn() {
	type postCheckArgs struct {
		Owner           sdk.AccAddress
		VaultAddr       sdk.AccAddress
		MarkerAddr      sdk.AccAddress
		UnderlyingAsset sdk.Coin
		Shares          sdk.Coin
	}

	testDef := msgServerTestDef[types.MsgSwapInRequest, types.MsgSwapInResponse, postCheckArgs]{
		endpointName: "SwapIn",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).SwapIn,
		postCheck: func(msg *types.MsgSwapInRequest, args postCheckArgs) {
			// Check that the marker created by the vault has a supply of 100.
			markerAddr := markertypes.MustGetMarkerAddress(args.Shares.Denom)
			marker, err := s.simApp.MarkerKeeper.GetMarker(s.ctx, markerAddr)
			s.Require().NoError(err, "get marker should not err")
			s.Require().NotNil(marker, "marker should exist")
			supply := s.simApp.BankKeeper.GetSupply(s.ctx, args.Shares.Denom)
			s.Require().Equal(args.Shares.Amount, supply.Amount, "marker supply should be updated")

			// Check that the balance of the vault account has increased by the denom in the Msg.
			vaultBalance := s.simApp.BankKeeper.GetBalance(s.ctx, markerAddr, args.UnderlyingAsset.Denom)
			s.Require().Equal(args.UnderlyingAsset, vaultBalance, "marker balance should be updated")

			// Check that the owner's balance contains the shares.
			ownerBalance := s.simApp.BankKeeper.GetBalance(s.ctx, args.Owner, args.Shares.Denom)
			s.Require().Equal(args.Shares, ownerBalance, "owner should have received shares")
		},
	}

	underlyingDenom := "underlying"
	shareDenom := "vaultshares"
	owner := s.adminAddr
	vaultAddr := types.GetVaultAddress(shareDenom)
	markerAddr := markertypes.MustGetMarkerAddress(shareDenom)
	assets := sdk.NewInt64Coin(underlyingDenom, 100)
	shares := sdk.NewInt64Coin(shareDenom, 100)

	swapInReq := types.MsgSwapInRequest{
		Owner:        owner.String(),
		VaultAddress: vaultAddr.String(),
		Assets:       assets,
	}

	tc := msgServerTestCase[types.MsgSwapInRequest, postCheckArgs]{
		name: "happy path",
		setup: func() {
			s.ctx = s.ctx.WithBlockTime(time.Now())
			s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlyingDenom, math.NewInt(1000)), owner)
			vault, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
				Admin:           owner.String(),
				ShareDenom:      shareDenom,
				UnderlyingAsset: underlyingDenom,
			})
			s.Require().NoError(err)
			vault.SwapInEnabled = true
			s.k.AuthKeeper.SetAccount(s.ctx, vault)
			// Fund owner with underlying assets
			err = FundAccount(s.ctx, s.simApp.BankKeeper, owner, sdk.NewCoins(assets))
			s.Require().NoError(err)
			s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
		},
		msg:                swapInReq,
		expectedErrSubstrs: nil,
		postCheckArgs:      postCheckArgs{Owner: owner, VaultAddr: vaultAddr, MarkerAddr: markerAddr, UnderlyingAsset: assets, Shares: sdk.NewCoin(shareDenom, assets.Amount)},
		expectedEvents:     createSwapInEvents(owner, vaultAddr, markerAddr, assets, shares),
	}
	testDef.expectedResponse = &types.MsgSwapInResponse{SharesReceived: sdk.NewCoin(shareDenom, assets.Amount)}
	runMsgServerTestCase(s, testDef, tc)
}

func (s *TestSuite) TestMsgServer_SwapIn_Failures() {
	testDef := msgServerTestDef[types.MsgSwapInRequest, types.MsgSwapInResponse, any]{
		endpointName: "SwapIn",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).SwapIn,
		postCheck:    nil,
	}

	underlyingDenom := "underlying"
	shareDenom := "vaultshares"
	owner := s.adminAddr
	vaultAddr := types.GetVaultAddress(shareDenom)
	assets := sdk.NewInt64Coin(underlyingDenom, 100)

	setup := func(swapInEnabled bool) func() {
		return func() {
			s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlyingDenom, math.NewInt(1000)), owner)
			vault, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
				Admin:           owner.String(),
				ShareDenom:      shareDenom,
				UnderlyingAsset: underlyingDenom,
			})
			s.Require().NoError(err)
			vault.SwapInEnabled = swapInEnabled
			s.k.AuthKeeper.SetAccount(s.ctx, vault)
			err = FundAccount(s.ctx, s.simApp.BankKeeper, owner, sdk.NewCoins(assets))
			s.Require().NoError(err)
		}
	}

	tests := []msgServerTestCase[types.MsgSwapInRequest, any]{
		{
			name: "vault does not exist",
			msg: types.MsgSwapInRequest{
				Owner:        owner.String(),
				VaultAddress: vaultAddr.String(),
				Assets:       assets,
			},
			expectedErrSubstrs: []string{"vault with address", "not found"},
		},
		{
			name:  "swap in disabled",
			setup: setup(false),
			msg: types.MsgSwapInRequest{
				Owner:        owner.String(),
				VaultAddress: vaultAddr.String(),
				Assets:       assets,
			},
			expectedErrSubstrs: []string{"swaps are not enabled for vault"},
		},
		{
			name:  "underlying asset mismatch",
			setup: setup(true),
			msg: types.MsgSwapInRequest{
				Owner:        owner.String(),
				VaultAddress: vaultAddr.String(),
				Assets:       sdk.NewInt64Coin("othercoin", 100),
			},
			expectedErrSubstrs: []string{"othercoin asset denom not supported for vault, expected one of [underlying]"},
		},
		{
			name: "insufficient funds",
			setup: func() {
				setup(true)()
				// Try to swap 100, but owner only has 50.
				err := s.simApp.BankKeeper.SendCoins(s.ctx, owner, authtypes.NewModuleAddress("burn"), sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, 50)))
				s.Require().NoError(err)
			},
			msg:                types.MsgSwapInRequest{Owner: owner.String(), VaultAddress: vaultAddr.String(), Assets: assets},
			expectedErrSubstrs: []string{"insufficient funds"},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			runMsgServerTestCase(s, testDef, tc)
		})
	}
}

func (s *TestSuite) TestMsgServer_SwapOut() {
	type postCheckArgs struct {
		Owner           sdk.AccAddress
		VaultAddr       sdk.AccAddress
		MarkerAddr      sdk.AccAddress
		UnderlyingAsset sdk.Coin
		Shares          sdk.Coin
	}

	testDef := msgServerTestDef[types.MsgSwapOutRequest, types.MsgSwapOutResponse, postCheckArgs]{
		endpointName: "SwapOut",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).SwapOut,
		postCheck: func(msg *types.MsgSwapOutRequest, args postCheckArgs) {
			// Check that the owner has the correct amount of underlying denom.
			ownerUnderlyingBalance := s.simApp.BankKeeper.GetBalance(s.ctx, args.Owner, args.UnderlyingAsset.Denom)
			s.Require().Equal(args.UnderlyingAsset, ownerUnderlyingBalance, "owner should have received underlying assets")

			// Check that the traded in shares are subtracted from the supply of the shares.
			initialShares := math.NewInt(100)
			expectedSupplyAmount := initialShares.Sub(msg.Assets.Amount)
			supply := s.simApp.BankKeeper.GetSupply(s.ctx, args.Shares.Denom)
			s.Require().Equal(expectedSupplyAmount.String(), supply.Amount.String(), "share supply should be reduced")

			// Check that the underlying asset has been removed from the marker account.
			initialUnderlying := math.NewInt(100)
			expectedMarkerBalance := initialUnderlying.Sub(msg.Assets.Amount)
			markerBalance := s.simApp.BankKeeper.GetBalance(s.ctx, args.MarkerAddr, args.UnderlyingAsset.Denom)
			s.Require().Equal(expectedMarkerBalance.String(), markerBalance.Amount.String(), "marker account should have underlying assets removed")
		},
	}

	underlyingDenom := "underlying"
	shareDenom := "vaultshares"
	owner := s.adminAddr
	vaultAddr := types.GetVaultAddress(shareDenom)
	initialAssets := sdk.NewInt64Coin(underlyingDenom, 100)

	setup := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlyingDenom, math.NewInt(1000)), owner)
		vault, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           owner.String(),
			ShareDenom:      shareDenom,
			UnderlyingAsset: underlyingDenom,
		})
		s.Require().NoError(err)
		vault.SwapInEnabled = true
		vault.SwapOutEnabled = true
		s.k.AuthKeeper.SetAccount(s.ctx, vault)
		// Fund owner with underlying assets
		err = FundAccount(s.ctx, s.simApp.BankKeeper, owner, sdk.NewCoins(initialAssets))
		s.Require().NoError(err)

		// Owner swaps in to get shares
		_, err = s.k.SwapIn(s.ctx, vaultAddr, owner, initialAssets)
		s.Require().NoError(err)

		// Reset event manager for the test
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	}

	tests := []struct {
		name          string
		sharesToTrade int64
	}{
		{"happy path - swap out 30 shares", 30},
		{"happy path - swap out all shares", 100},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			sharesToSwap := sdk.NewInt64Coin(shareDenom, tt.sharesToTrade)
			swapOutReq := types.MsgSwapOutRequest{
				Owner:        owner.String(),
				VaultAddress: vaultAddr.String(),
				Assets:       sharesToSwap,
			}

			tc := msgServerTestCase[types.MsgSwapOutRequest, postCheckArgs]{
				name:           tt.name,
				setup:          setup,
				msg:            swapOutReq,
				postCheckArgs:  postCheckArgs{Owner: owner, VaultAddr: vaultAddr, MarkerAddr: markertypes.MustGetMarkerAddress(shareDenom), UnderlyingAsset: sdk.NewInt64Coin(underlyingDenom, tt.sharesToTrade), Shares: sharesToSwap},
				expectedEvents: createSwapOutEvents(owner, vaultAddr, markertypes.MustGetMarkerAddress(shareDenom), sdk.NewInt64Coin(underlyingDenom, tt.sharesToTrade), sharesToSwap),
			}

			testDef.expectedResponse = &types.MsgSwapOutResponse{SharesBurned: sharesToSwap}
			runMsgServerTestCase(s, testDef, tc)
		})
	}
}

func (s *TestSuite) TestMsgServer_SwapOut_Failures() {
	testDef := msgServerTestDef[types.MsgSwapOutRequest, types.MsgSwapOutResponse, any]{
		endpointName: "SwapOut",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).SwapOut,
		postCheck:    nil,
	}

	underlyingDenom := "underlying"
	shareDenom := "vaultshares"
	owner := s.adminAddr
	vaultAddr := types.GetVaultAddress(shareDenom)
	initialAssets := sdk.NewInt64Coin(underlyingDenom, 100)
	sharesToSwap := sdk.NewInt64Coin(shareDenom, 50)

	setup := func(swapOutEnabled bool) func() {
		return func() {
			s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlyingDenom, math.NewInt(1000)), owner)
			vault, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
				Admin:           owner.String(),
				ShareDenom:      shareDenom,
				UnderlyingAsset: underlyingDenom,
			})
			s.Require().NoError(err)
			vault.SwapInEnabled = true
			vault.SwapOutEnabled = swapOutEnabled
			s.k.AuthKeeper.SetAccount(s.ctx, vault)
			// Fund owner with underlying assets
			err = FundAccount(s.ctx, s.simApp.BankKeeper, owner, sdk.NewCoins(initialAssets))
			s.Require().NoError(err)

			// Owner swaps in to get shares
			_, err = s.k.SwapIn(s.ctx, vaultAddr, owner, initialAssets)
			s.Require().NoError(err)
		}
	}

	tests := []msgServerTestCase[types.MsgSwapOutRequest, any]{
		{
			name: "vault does not exist",
			msg: types.MsgSwapOutRequest{
				Owner:        owner.String(),
				VaultAddress: vaultAddr.String(),
				Assets:       sharesToSwap,
			},
			expectedErrSubstrs: []string{"vault with address", "not found"},
		},
		{

			name:  "asset is not share denom",
			setup: setup(true),
			msg: types.MsgSwapOutRequest{
				Owner:        owner.String(),
				VaultAddress: vaultAddr.String(),
				Assets:       sdk.NewInt64Coin("wrongdenom", 50),
			},
			expectedErrSubstrs: []string{"swap out denom must be share denom", "wrongdenom", shareDenom},
		},
		{
			name:  "insufficient shares",
			setup: setup(true),
			msg: types.MsgSwapOutRequest{
				Owner:        owner.String(),
				VaultAddress: vaultAddr.String(),
				Assets:       sdk.NewInt64Coin(shareDenom, 150),
			},
			expectedErrSubstrs: []string{"failed to send shares to marker", "insufficient funds"},
		},
		{
			name:  "swap out disabled",
			setup: setup(false),
			msg: types.MsgSwapOutRequest{
				Owner:        owner.String(),
				VaultAddress: vaultAddr.String(),
				Assets:       sdk.NewInt64Coin(shareDenom, 150),
			},
			expectedErrSubstrs: []string{"swaps are not enabled for vault"},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			runMsgServerTestCase(s, testDef, tc)
		})
	}
}

func (s *TestSuite) TestMsgServer_ToggleSwapOut() {
	type postCheckArgs struct {
		VaultAddress    sdk.AccAddress
		ExpectedEnabled bool
	}

	testDef := msgServerTestDef[types.MsgToggleSwapOutRequest, types.MsgToggleSwapOutResponse, postCheckArgs]{
		endpointName: "ToggleSwapOut",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).ToggleSwapOut,
		postCheck: func(msg *types.MsgToggleSwapOutRequest, args postCheckArgs) {
			vault, err := s.k.GetVault(s.ctx, args.VaultAddress)
			s.Require().NoError(err, "should be able to get vault")
			s.Assert().Equal(args.ExpectedEnabled, vault.SwapOutEnabled, "vault SwapOutEnabled should match expected value")
		},
	}

	underlyingDenom := "underlying"
	shareDenom := "vaultshares"
	owner := s.adminAddr
	otherUser := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1000))
	vaultAddr := types.GetVaultAddress(shareDenom)

	setup := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlyingDenom, math.NewInt(1000)), owner)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           owner.String(),
			ShareDenom:      shareDenom,
			UnderlyingAsset: underlyingDenom,
		})
		s.Require().NoError(err)
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	}

	tests := []struct {
		name               string
		setup              func()
		msg                types.MsgToggleSwapOutRequest
		postCheckArgs      postCheckArgs
		expectedEvents     sdk.Events
		expectedErrSubstrs []string
	}{
		{
			name:  "happy path - enable swap out",
			setup: setup,
			msg: types.MsgToggleSwapOutRequest{
				Admin:        owner.String(),
				VaultAddress: vaultAddr.String(),
				Enabled:      true,
			},
			postCheckArgs: postCheckArgs{
				VaultAddress:    vaultAddr,
				ExpectedEnabled: true,
			},
			expectedEvents: sdk.Events{
				sdk.NewEvent("vault.v1.EventToggleSwapOut",
					sdk.NewAttribute("admin", owner.String()),
					sdk.NewAttribute("enabled", "true"),
					sdk.NewAttribute("vault_address", vaultAddr.String()),
				),
			},
		},
		{
			name: "happy path - disable swap out",
			setup: func() {
				setup()
				vault, err := s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err)
				vault.SwapOutEnabled = true
				s.k.AuthKeeper.SetAccount(s.ctx, vault)
			},
			msg: types.MsgToggleSwapOutRequest{
				Admin:        owner.String(),
				VaultAddress: vaultAddr.String(),
				Enabled:      false,
			},
			postCheckArgs: postCheckArgs{
				VaultAddress:    vaultAddr,
				ExpectedEnabled: false,
			},
			expectedEvents: sdk.Events{
				sdk.NewEvent("vault.v1.EventToggleSwapOut",
					sdk.NewAttribute("admin", owner.String()),
					sdk.NewAttribute("enabled", "false"),
					sdk.NewAttribute("vault_address", vaultAddr.String()),
				),
			},
		},
		{
			name:  "failure - vault not found",
			setup: func() { /* no setup, so vault doesn't exist */ },
			msg: types.MsgToggleSwapOutRequest{
				Admin:        owner.String(),
				VaultAddress: vaultAddr.String(),
				Enabled:      true,
			},
			expectedErrSubstrs: []string{"vault not found", vaultAddr.String()},
		},
		{
			name:  "failure - unauthorized admin",
			setup: setup,
			msg: types.MsgToggleSwapOutRequest{
				Admin:        otherUser.String(),
				VaultAddress: vaultAddr.String(),
				Enabled:      true,
			},
			expectedErrSubstrs: []string{"unauthorized", otherUser.String(), "is not the vault admin"},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			tc := msgServerTestCase[types.MsgToggleSwapOutRequest, postCheckArgs]{
				name:               tt.name,
				setup:              tt.setup,
				msg:                tt.msg,
				postCheckArgs:      tt.postCheckArgs,
				expectedEvents:     tt.expectedEvents,
				expectedErrSubstrs: tt.expectedErrSubstrs,
			}

			testDef.expectedResponse = &types.MsgToggleSwapOutResponse{}
			runMsgServerTestCase(s, testDef, tc)
		})
	}
}

func (s *TestSuite) TestMsgServer_ToggleSwapIn() {
	type postCheckArgs struct {
		VaultAddress    sdk.AccAddress
		ExpectedEnabled bool
	}

	testDef := msgServerTestDef[types.MsgToggleSwapInRequest, types.MsgToggleSwapInResponse, postCheckArgs]{
		endpointName: "ToggleSwapIn",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).ToggleSwapIn,
		postCheck: func(msg *types.MsgToggleSwapInRequest, args postCheckArgs) {
			vault, err := s.k.GetVault(s.ctx, args.VaultAddress)
			s.Require().NoError(err, "should be able to get vault")
			s.Assert().Equal(args.ExpectedEnabled, vault.SwapInEnabled, "vault SwapInEnabled should match expected value")
		},
	}

	underlyingDenom := "underlying"
	shareDenom := "vaultshares"
	owner := s.adminAddr
	otherUser := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1000))
	vaultAddr := types.GetVaultAddress(shareDenom)

	setup := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlyingDenom, math.NewInt(1000)), owner)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           owner.String(),
			ShareDenom:      shareDenom,
			UnderlyingAsset: underlyingDenom,
		})
		s.Require().NoError(err)
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	}

	tests := []struct {
		name               string
		setup              func()
		msg                types.MsgToggleSwapInRequest
		postCheckArgs      postCheckArgs
		expectedEvents     sdk.Events
		expectedErrSubstrs []string
	}{
		{
			name:  "happy path - enable swap in",
			setup: setup,
			msg: types.MsgToggleSwapInRequest{
				Admin:        owner.String(),
				VaultAddress: vaultAddr.String(),
				Enabled:      true,
			},
			postCheckArgs: postCheckArgs{
				VaultAddress:    vaultAddr,
				ExpectedEnabled: true,
			},
			expectedEvents: sdk.Events{
				sdk.NewEvent("vault.v1.EventToggleSwapIn",
					sdk.NewAttribute("admin", owner.String()),
					sdk.NewAttribute("enabled", "true"),
					sdk.NewAttribute("vault_address", vaultAddr.String()),
				),
			},
		},
		{
			name: "happy path - disable swap in",
			setup: func() {
				setup()
				vault, err := s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err)
				vault.SwapInEnabled = true
				s.k.AuthKeeper.SetAccount(s.ctx, vault)
			},
			msg: types.MsgToggleSwapInRequest{
				Admin:        owner.String(),
				VaultAddress: vaultAddr.String(),
				Enabled:      false,
			},
			postCheckArgs: postCheckArgs{
				VaultAddress:    vaultAddr,
				ExpectedEnabled: false,
			},
			expectedEvents: sdk.Events{
				sdk.NewEvent("vault.v1.EventToggleSwapIn",
					sdk.NewAttribute("admin", owner.String()),
					sdk.NewAttribute("enabled", "false"),
					sdk.NewAttribute("vault_address", vaultAddr.String()),
				),
			},
		},
		{
			name:  "failure - vault not found",
			setup: func() { /* no setup, so vault doesn't exist */ },
			msg: types.MsgToggleSwapInRequest{
				Admin:        owner.String(),
				VaultAddress: vaultAddr.String(),
				Enabled:      true,
			},
			expectedErrSubstrs: []string{"vault not found", vaultAddr.String()},
		},
		{
			name:  "failure - unauthorized admin",
			setup: setup,
			msg: types.MsgToggleSwapInRequest{
				Admin:        otherUser.String(),
				VaultAddress: vaultAddr.String(),
				Enabled:      true,
			},
			expectedErrSubstrs: []string{"unauthorized", otherUser.String(), "is not the vault admin"},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			tc := msgServerTestCase[types.MsgToggleSwapInRequest, postCheckArgs]{
				name:               tt.name,
				setup:              tt.setup,
				msg:                tt.msg,
				postCheckArgs:      tt.postCheckArgs,
				expectedEvents:     tt.expectedEvents,
				expectedErrSubstrs: tt.expectedErrSubstrs,
			}

			testDef.expectedResponse = &types.MsgToggleSwapInResponse{}
			runMsgServerTestCase(s, testDef, tc)
		})
	}
}

func (s *TestSuite) TestMsgServer_UpdateInterestRate() {
	type postCheckArgs struct {
		VaultAddress          sdk.AccAddress
		ExpectedRate          string
		ExpectedPeriodStart   int64
		HasNewInterestDetails bool
	}

	testDef := msgServerTestDef[types.MsgUpdateInterestRateRequest, types.MsgUpdateInterestRateResponse, postCheckArgs]{
		endpointName: "UpdateInterestRate",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).UpdateInterestRate,
		postCheck: func(msg *types.MsgUpdateInterestRateRequest, args postCheckArgs) {
			vault, err := s.k.GetVault(s.ctx, args.VaultAddress)
			s.Require().NoError(err, "should be able to get vault")
			s.Assert().Equal(args.ExpectedRate, vault.CurrentInterestRate, "vault current interest rate should match expected rate")
			s.Assert().Equal(args.ExpectedRate, vault.DesiredInterestRate, "vault desired interest rate should match expected rate")

			if args.HasNewInterestDetails {
				v, err := s.k.VaultInterestDetails.Get(s.ctx, args.VaultAddress)
				s.Require().NoError(err, "should be able to get vault interest details")
				s.Assert().Equal(args.ExpectedPeriodStart, v.PeriodStart, "vault interest details period start should be current block time")
				s.Assert().Equal(int64(0), v.ExpireTime, " vault interest details expire time should be zero")
			} else {
				_, err := s.k.VaultInterestDetails.Get(s.ctx, args.VaultAddress)
				s.Require().ErrorContains(err, "not found", "should not find vault interest details")
			}

		},
	}

	underlyingDenom := "underlying"
	shareDenom := "vaultshares"
	owner := s.adminAddr
	vaultAddr := types.GetVaultAddress(shareDenom)
	currentBlockTime := time.Now()

	setup := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlyingDenom, math.NewInt(1000)), owner)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           owner.String(),
			ShareDenom:      shareDenom,
			UnderlyingAsset: underlyingDenom,
		})
		s.Require().NoError(err)
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
		s.ctx = s.ctx.WithBlockTime(currentBlockTime)
	}

	tests := []struct {
		name           string
		interestRate   string
		setup          func()
		postCheckArgs  postCheckArgs
		expectedEvents sdk.Events
	}{
		{name: "initial setting of interest rate",
			interestRate: "0.05",
			setup:        setup,
			postCheckArgs: postCheckArgs{
				VaultAddress:          vaultAddr,
				ExpectedRate:          "0.05",
				HasNewInterestDetails: true,
				ExpectedPeriodStart:   currentBlockTime.Unix(),
			},
			expectedEvents: sdk.Events{
				sdk.NewEvent("vault.v1.EventVaultInterestChange",
					sdk.NewAttribute("current_rate", "0.05"),
					sdk.NewAttribute("desired_rate", "0.05"),
					sdk.NewAttribute("vault_address", vaultAddr.String()),
				),
			},
		},
		{name: "update current interest rate to non zero, needs to reconcile previous rate",
			interestRate: "4.06",
			setup: func() {
				setup()
				vaultAcc, err := s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err, "should be able to get vault")
				s.k.UpdateInterestRates(s.ctx, vaultAcc, "4.20", "4.20")
				s.k.VaultInterestDetails.Set(s.ctx, vaultAddr, types.VaultInterestDetails{PeriodStart: currentBlockTime.Unix() - 10000})
			},
			postCheckArgs: postCheckArgs{
				VaultAddress:          vaultAddr,
				ExpectedRate:          "4.06",
				HasNewInterestDetails: true,
				ExpectedPeriodStart:   currentBlockTime.Unix(),
			},
			expectedEvents: sdk.Events{
				sdk.NewEvent("vault.v1.EventVaultReconcile",
					sdk.NewAttribute("interest_earned", CoinToJSON(sdk.NewInt64Coin(underlyingDenom, 0))),
					sdk.NewAttribute("principal_after", CoinToJSON(sdk.NewInt64Coin(underlyingDenom, 0))),
					sdk.NewAttribute("principal_before", CoinToJSON(sdk.NewInt64Coin(underlyingDenom, 0))),
					sdk.NewAttribute("rate", "4.20"),
					sdk.NewAttribute("time", "10000"),
					sdk.NewAttribute("vault_address", vaultAddr.String()),
				),
				sdk.NewEvent("vault.v1.EventVaultInterestChange",
					sdk.NewAttribute("current_rate", "4.06"),
					sdk.NewAttribute("desired_rate", "4.06"),
					sdk.NewAttribute("vault_address", vaultAddr.String()),
				),
			},
		},
		{name: "update current interest rate to zero, needs to reconcile previous non zero rate",
			interestRate: "0",
			setup: func() {
				setup()
				vaultAcc, err := s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err, "should be able to get vault")
				s.k.UpdateInterestRates(s.ctx, vaultAcc, "6.12", "6.12")
				s.k.VaultInterestDetails.Set(s.ctx, vaultAddr, types.VaultInterestDetails{PeriodStart: currentBlockTime.Unix() - 10000})
			},
			postCheckArgs: postCheckArgs{
				VaultAddress:          vaultAddr,
				ExpectedRate:          types.ZeroInterestRate,
				HasNewInterestDetails: false,
			},
			expectedEvents: sdk.Events{
				sdk.NewEvent("vault.v1.EventVaultReconcile",
					sdk.NewAttribute("interest_earned", CoinToJSON(sdk.NewInt64Coin(underlyingDenom, 0))),
					sdk.NewAttribute("principal_after", CoinToJSON(sdk.NewInt64Coin(underlyingDenom, 0))),
					sdk.NewAttribute("principal_before", CoinToJSON(sdk.NewInt64Coin(underlyingDenom, 0))),
					sdk.NewAttribute("rate", "6.12"),
					sdk.NewAttribute("time", "10000"),
					sdk.NewAttribute("vault_address", vaultAddr.String()),
				),
				sdk.NewEvent("vault.v1.EventVaultInterestChange",
					sdk.NewAttribute("current_rate", "0.0"),
					sdk.NewAttribute("desired_rate", "0.0"),
					sdk.NewAttribute("vault_address", vaultAddr.String()),
				),
			},
		},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			updateInterestRateReq := types.MsgUpdateInterestRateRequest{
				Admin:        owner.String(),
				VaultAddress: vaultAddr.String(),
				NewRate:      tt.interestRate,
			}

			tc := msgServerTestCase[types.MsgUpdateInterestRateRequest, postCheckArgs]{
				name:           tt.name,
				setup:          tt.setup,
				msg:            updateInterestRateReq,
				postCheckArgs:  tt.postCheckArgs,
				expectedEvents: tt.expectedEvents,
			}

			testDef.expectedResponse = &types.MsgUpdateInterestRateResponse{}
			runMsgServerTestCase(s, testDef, tc)
		})
	}
}

func (s *TestSuite) TestMsgServer_UpdateInterestRate_Failures() {
	testDef := msgServerTestDef[types.MsgUpdateInterestRateRequest, types.MsgUpdateInterestRateResponse, any]{
		endpointName: "UpdateInterestRate",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).UpdateInterestRate,
		postCheck:    nil,
	}

	underlyingDenom := "underlying"
	shareDenom := "vaultshares"
	owner := s.adminAddr
	vaultAddr := types.GetVaultAddress(shareDenom)
	currentBlockTime := time.Now()

	setup := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlyingDenom, math.NewInt(1000)), owner)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           owner.String(),
			ShareDenom:      shareDenom,
			UnderlyingAsset: underlyingDenom,
		})
		s.Require().NoError(err)
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
		s.ctx = s.ctx.WithBlockTime(currentBlockTime)
	}

	tests := []msgServerTestCase[types.MsgUpdateInterestRateRequest, any]{
		{
			name: "vault does not exist",
			msg: types.MsgUpdateInterestRateRequest{
				Admin:        owner.String(),
				VaultAddress: types.GetVaultAddress("invalidvaultaddress").String(),
				NewRate:      "0.05",
			},
			setup:              setup,
			expectedErrSubstrs: []string{"not found"},
		},
		{
			name: "vault invalid vault address",
			msg: types.MsgUpdateInterestRateRequest{
				Admin:        owner.String(),
				VaultAddress: markertypes.MustGetMarkerAddress(shareDenom).String(),
				NewRate:      "0.05",
			},
			setup:              setup,
			expectedErrSubstrs: []string{"failed to get vault", "is not a vault account"},
		},
		{
			name: "unauthorized admin",
			msg: types.MsgUpdateInterestRateRequest{
				Admin:        sdk.AccAddress("invalidadmin").String(),
				VaultAddress: vaultAddr.String(),
				NewRate:      "0.05",
			},
			setup:              setup,
			expectedErrSubstrs: []string{"unauthorized", "is not the vault admin"},
		},
	}
	for _, tc := range tests {
		s.Run(tc.name, func() {
			runMsgServerTestCase(s, testDef, tc)
		})
	}
}

func (s *TestSuite) TestMsgServer_UpdateMinInterestRate() {
	type postCheckArgs struct {
		VaultAddress sdk.AccAddress
		ExpectedMin  string
	}

	testDef := msgServerTestDef[types.MsgUpdateMinInterestRateRequest, types.MsgUpdateMinInterestRateResponse, postCheckArgs]{
		endpointName: "UpdateMinInterestRate",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).UpdateMinInterestRate,
		postCheck: func(msg *types.MsgUpdateMinInterestRateRequest, args postCheckArgs) {
			v, err := s.k.GetVault(s.ctx, args.VaultAddress)
			s.Require().NoError(err)
			s.Assert().Equal(args.ExpectedMin, v.MinInterestRate)
		},
	}

	underlying := "under"
	share := "vaultshares"
	admin := s.adminAddr
	vaultAddr := types.GetVaultAddress(share)

	setup := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlying, math.NewInt(1000)), admin)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           admin.String(),
			ShareDenom:      share,
			UnderlyingAsset: underlying,
		})
		s.Require().NoError(err)
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	}

	tests := []struct {
		name           string
		minRate        string
		postCheckArgs  postCheckArgs
		expectedEvents sdk.Events
	}{
		{
			name:    "set min to non-empty",
			minRate: "0.05",
			postCheckArgs: postCheckArgs{
				VaultAddress: vaultAddr,
				ExpectedMin:  "0.05",
			},
			expectedEvents: sdk.Events{
				sdk.NewEvent("vault.v1.EventMinInterestRateUpdated",
					sdk.NewAttribute("admin", admin.String()),
					sdk.NewAttribute("min_rate", "0.05"),
					sdk.NewAttribute("vault_address", vaultAddr.String()),
				),
			},
		},
		{
			name:    "disable min by empty string",
			minRate: "",
			postCheckArgs: postCheckArgs{
				VaultAddress: vaultAddr,
				ExpectedMin:  "",
			},
			expectedEvents: sdk.Events{
				sdk.NewEvent("vault.v1.EventMinInterestRateUpdated",
					sdk.NewAttribute("admin", admin.String()),
					sdk.NewAttribute("min_rate", ""),
					sdk.NewAttribute("vault_address", vaultAddr.String()),
				),
			},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			tc := msgServerTestCase[types.MsgUpdateMinInterestRateRequest, postCheckArgs]{
				name:  tt.name,
				setup: setup,
				msg: types.MsgUpdateMinInterestRateRequest{
					Admin:        admin.String(),
					VaultAddress: vaultAddr.String(),
					MinRate:      tt.minRate,
				},
				postCheckArgs:  tt.postCheckArgs,
				expectedEvents: tt.expectedEvents,
			}
			testDef.expectedResponse = &types.MsgUpdateMinInterestRateResponse{}
			runMsgServerTestCase(s, testDef, tc)
		})
	}
}

func (s *TestSuite) TestMsgServer_UpdateMinInterestRate_Failures() {
	testDef := msgServerTestDef[types.MsgUpdateMinInterestRateRequest, types.MsgUpdateMinInterestRateResponse, any]{
		endpointName: "UpdateMinInterestRate",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).UpdateMinInterestRate,
		postCheck:    nil,
	}

	underlying := "under"
	share := "vaultshares"
	admin := s.adminAddr
	other := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1000))
	vaultAddr := types.GetVaultAddress(share)

	setup := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlying, math.NewInt(1000)), admin)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           admin.String(),
			ShareDenom:      share,
			UnderlyingAsset: underlying,
		})
		s.Require().NoError(err)
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	}

	tests := []msgServerTestCase[types.MsgUpdateMinInterestRateRequest, any]{
		{
			name: "vault does not exist",
			setup: func() {
				s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
			},
			msg: types.MsgUpdateMinInterestRateRequest{
				Admin:        admin.String(),
				VaultAddress: types.GetVaultAddress("doesnotexist").String(),
				MinRate:      "0.01",
			},
			expectedErrSubstrs: []string{"not found"},
		},
		{
			name:  "invalid vault address (not a vault account)",
			setup: setup,
			msg: types.MsgUpdateMinInterestRateRequest{
				Admin:        admin.String(),
				VaultAddress: markertypes.MustGetMarkerAddress(share).String(),
				MinRate:      "0.01",
			},
			expectedErrSubstrs: []string{"failed to get vault", "is not a vault account"},
		},
		{
			name:  "unauthorized admin",
			setup: setup,
			msg: types.MsgUpdateMinInterestRateRequest{
				Admin:        other.String(),
				VaultAddress: vaultAddr.String(),
				MinRate:      "0.02",
			},
			expectedErrSubstrs: []string{"unauthorized", "is not the vault admin"},
		},
		{
			name: "validation failure: min > existing max",
			setup: func() {
				setup()
				v, err := s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err)
				s.k.SetMaxInterestRate(s.ctx, v, "0.05")
			},
			msg: types.MsgUpdateMinInterestRateRequest{
				Admin:        admin.String(),
				VaultAddress: vaultAddr.String(),
				MinRate:      "0.06",
			},
			expectedErrSubstrs: []string{"minimum interest rate", "greater than", "maximum"},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			runMsgServerTestCase(s, testDef, tc)
		})
	}
}

func (s *TestSuite) TestMsgServer_UpdateMaxInterestRate() {
	type postCheckArgs struct {
		VaultAddress sdk.AccAddress
		ExpectedMax  string
	}

	testDef := msgServerTestDef[types.MsgUpdateMaxInterestRateRequest, types.MsgUpdateMaxInterestRateResponse, postCheckArgs]{
		endpointName: "UpdateMaxInterestRate",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).UpdateMaxInterestRate,
		postCheck: func(msg *types.MsgUpdateMaxInterestRateRequest, args postCheckArgs) {
			v, err := s.k.GetVault(s.ctx, args.VaultAddress)
			s.Require().NoError(err)
			s.Assert().Equal(args.ExpectedMax, v.MaxInterestRate)
		},
	}

	underlying := "under"
	share := "vaultshares"
	admin := s.adminAddr
	vaultAddr := types.GetVaultAddress(share)

	setup := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlying, math.NewInt(1000)), admin)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           admin.String(),
			ShareDenom:      share,
			UnderlyingAsset: underlying,
		})
		s.Require().NoError(err)
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	}

	tests := []struct {
		name           string
		maxRate        string
		postCheckArgs  postCheckArgs
		expectedEvents sdk.Events
	}{
		{
			name:    "set_max_to_non-empty",
			maxRate: "0.50",
			postCheckArgs: postCheckArgs{
				VaultAddress: vaultAddr,
				ExpectedMax:  "0.50",
			},
			expectedEvents: sdk.Events{
				sdk.NewEvent("vault.v1.EventMaxInterestRateUpdated",
					sdk.NewAttribute("admin", admin.String()),
					sdk.NewAttribute("max_rate", "0.50"),
					sdk.NewAttribute("vault_address", vaultAddr.String()),
				),
			},
		},
		{
			name:    "disable_max_by_empty_string",
			maxRate: "",
			postCheckArgs: postCheckArgs{
				VaultAddress: vaultAddr,
				ExpectedMax:  "",
			},
			expectedEvents: sdk.Events{
				sdk.NewEvent("vault.v1.EventMaxInterestRateUpdated",
					sdk.NewAttribute("admin", admin.String()),
					sdk.NewAttribute("max_rate", ""),
					sdk.NewAttribute("vault_address", vaultAddr.String()),
				),
			},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			tc := msgServerTestCase[types.MsgUpdateMaxInterestRateRequest, postCheckArgs]{
				name:  tt.name,
				setup: setup,
				msg: types.MsgUpdateMaxInterestRateRequest{
					Admin:        admin.String(),
					VaultAddress: vaultAddr.String(),
					MaxRate:      tt.maxRate,
				},
				postCheckArgs:  tt.postCheckArgs,
				expectedEvents: tt.expectedEvents,
			}
			testDef.expectedResponse = &types.MsgUpdateMaxInterestRateResponse{}
			runMsgServerTestCase(s, testDef, tc)
		})
	}
}

func (s *TestSuite) TestMsgServer_UpdateMaxInterestRate_Failures() {
	testDef := msgServerTestDef[types.MsgUpdateMaxInterestRateRequest, types.MsgUpdateMaxInterestRateResponse, any]{
		endpointName: "UpdateMaxInterestRate",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).UpdateMaxInterestRate,
		postCheck:    nil,
	}

	underlying := "under"
	share := "vaultshares"
	admin := s.adminAddr
	other := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1000))
	vaultAddr := types.GetVaultAddress(share)

	setup := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlying, math.NewInt(1000)), admin)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           admin.String(),
			ShareDenom:      share,
			UnderlyingAsset: underlying,
		})
		s.Require().NoError(err)
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	}

	tests := []msgServerTestCase[types.MsgUpdateMaxInterestRateRequest, any]{
		{
			name: "vault does not exist",
			setup: func() {
				s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
			},
			msg: types.MsgUpdateMaxInterestRateRequest{
				Admin:        admin.String(),
				VaultAddress: types.GetVaultAddress("doesnotexist").String(),
				MaxRate:      "0.99",
			},
			expectedErrSubstrs: []string{"not found"},
		},
		{
			name:  "invalid vault address (not a vault account)",
			setup: setup,
			msg: types.MsgUpdateMaxInterestRateRequest{
				Admin:        admin.String(),
				VaultAddress: markertypes.MustGetMarkerAddress(share).String(),
				MaxRate:      "0.10",
			},
			expectedErrSubstrs: []string{"failed to get vault", "is not a vault account"},
		},
		{
			name:  "unauthorized admin",
			setup: setup,
			msg: types.MsgUpdateMaxInterestRateRequest{
				Admin:        other.String(),
				VaultAddress: vaultAddr.String(),
				MaxRate:      "0.33",
			},
			expectedErrSubstrs: []string{"unauthorized", "is not the vault admin"},
		},
		{
			name: "validation_failure: max < existing min",
			setup: func() {
				setup()
				_, err := keeper.NewMsgServer(s.simApp.VaultKeeper).UpdateMinInterestRate(
					s.ctx, &types.MsgUpdateMinInterestRateRequest{
						Admin:        admin.String(),
						VaultAddress: vaultAddr.String(),
						MinRate:      "0.50",
					},
				)
				s.Require().NoError(err)
			},
			msg: types.MsgUpdateMaxInterestRateRequest{
				Admin:        admin.String(),
				VaultAddress: vaultAddr.String(),
				MaxRate:      "0.40",
			},
			expectedErrSubstrs: []string{
				"minimum interest rate",
				"cannot be greater than",
				"maximum interest rate",
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			runMsgServerTestCase(s, testDef, tc)
		})
	}
}

func (s *TestSuite) TestMsgServer_DepositInterestFunds() {
	type postCheckArgs struct {
		VaultAddress          sdk.AccAddress
		ExpectedDepositAmount sdk.Coin
		ExpectedVaultBalance  sdk.Coin
		HasNewInterestDetails bool
		ExpectedPeriodStart   int64
	}

	testDef := msgServerTestDef[types.MsgDepositInterestFundsRequest, types.MsgDepositInterestFundsResponse, postCheckArgs]{
		endpointName: "DepositInterestFunds",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).DepositInterestFunds,
		postCheck: func(msg *types.MsgDepositInterestFundsRequest, args postCheckArgs) {
			vaultBal := s.k.BankKeeper.GetBalance(s.ctx, args.VaultAddress, args.ExpectedDepositAmount.Denom)
			s.Assert().Equal(args.ExpectedVaultBalance.Amount.Int64(), vaultBal.Amount.Int64())
			if args.HasNewInterestDetails {
				v, err := s.k.VaultInterestDetails.Get(s.ctx, args.VaultAddress)
				s.Require().NoError(err)
				s.Assert().Equal(args.ExpectedPeriodStart, v.PeriodStart)
			}
		},
	}

	underlying := "under"
	shares := "vaultshares"
	admin := s.adminAddr
	vaultAddr := types.GetVaultAddress(shares)
	blockTime := time.Now()
	amount := sdk.NewInt64Coin(underlying, 500)

	setup := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlying, math.NewInt(1000)), admin)
		FundAccount(s.ctx, s.simApp.BankKeeper, admin, sdk.NewCoins(amount))
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           admin.String(),
			ShareDenom:      shares,
			UnderlyingAsset: underlying,
		})
		s.Require().NoError(err)
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
		s.ctx = s.ctx.WithBlockTime(blockTime)
	}

	s.Run("happy path - deposit interest funds", func() {
		ev := createSendCoinEvents(admin.String(), vaultAddr.String(), sdk.NewCoins(amount).String())
		ev = append(ev, sdk.NewEvent(
			"vault.v1.EventInterestDeposit",
			sdk.NewAttribute("admin", admin.String()),
			sdk.NewAttribute("amount", CoinToJSON(amount)),
			sdk.NewAttribute("vault_address", vaultAddr.String()),
		))

		tc := msgServerTestCase[types.MsgDepositInterestFundsRequest, postCheckArgs]{
			name:  "happy path",
			setup: setup,
			msg: types.MsgDepositInterestFundsRequest{
				Admin:        admin.String(),
				VaultAddress: vaultAddr.String(),
				Amount:       amount,
			},
			postCheckArgs: postCheckArgs{
				VaultAddress:          vaultAddr,
				ExpectedDepositAmount: amount,
				ExpectedVaultBalance:  amount,
				HasNewInterestDetails: true,
				ExpectedPeriodStart:   blockTime.Unix(),
			},
			expectedEvents: ev,
		}

		testDef.expectedResponse = &types.MsgDepositInterestFundsResponse{}
		runMsgServerTestCase(s, testDef, tc)
	})
}

func (s *TestSuite) TestMsgServer_DepositInterestFunds_Failures() {
	testDef := msgServerTestDef[types.MsgDepositInterestFundsRequest, types.MsgDepositInterestFundsResponse, any]{
		endpointName: "DepositInterestFunds",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).DepositInterestFunds,
		postCheck:    nil,
	}

	underlying := "under"
	shares := "vaultshares"
	admin := s.adminAddr
	other := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1000))
	vaultAddr := types.GetVaultAddress(shares)
	amount := sdk.NewInt64Coin(underlying, 500)

	setup := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlying, math.NewInt(1000)), admin)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           admin.String(),
			ShareDenom:      shares,
			UnderlyingAsset: underlying,
		})
		s.Require().NoError(err)
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	}

	setupWithAdminFunds := func() {
		setup()
		FundAccount(s.ctx, s.simApp.BankKeeper, admin, sdk.NewCoins(amount))
	}

	tests := []msgServerTestCase[types.MsgDepositInterestFundsRequest, any]{
		{
			name: "vault does not exist",
			msg: types.MsgDepositInterestFundsRequest{
				Admin:        admin.String(),
				VaultAddress: types.GetVaultAddress("doesnotexist").String(),
				Amount:       amount,
			},
			expectedErrSubstrs: []string{"not found"},
		},
		{
			name:  "invalid vault address not a vault account",
			setup: setup,
			msg: types.MsgDepositInterestFundsRequest{
				Admin:        admin.String(),
				VaultAddress: markertypes.MustGetMarkerAddress(shares).String(),
				Amount:       amount,
			},
			expectedErrSubstrs: []string{"failed to get vault", "is not a vault account"},
		},
		{
			name:  "unauthorized admin",
			setup: setupWithAdminFunds,
			msg: types.MsgDepositInterestFundsRequest{
				Admin:        other.String(),
				VaultAddress: vaultAddr.String(),
				Amount:       amount,
			},
			expectedErrSubstrs: []string{"unauthorized", "is not the vault admin"},
		},
		{
			name:  "insufficient admin balance",
			setup: setup,
			msg: types.MsgDepositInterestFundsRequest{
				Admin:        admin.String(),
				VaultAddress: vaultAddr.String(),
				Amount:       sdk.NewInt64Coin(underlying, 9_999_999),
			},
			expectedErrSubstrs: []string{"failed to deposit funds", "insufficient funds"},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			runMsgServerTestCase(s, testDef, tc)
		})
	}
}

func (s *TestSuite) TestMsgServer_WithdrawInterestFunds() {
	type postCheckArgs struct {
		AdminAddress        sdk.AccAddress
		ExpectedAdminAmount sdk.Coin
	}

	testDef := msgServerTestDef[types.MsgWithdrawInterestFundsRequest, types.MsgWithdrawInterestFundsResponse, postCheckArgs]{
		endpointName: "WithdrawInterestFunds",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).WithdrawInterestFunds,
		postCheck: func(msg *types.MsgWithdrawInterestFundsRequest, args postCheckArgs) {
			bal := s.k.BankKeeper.GetBalance(s.ctx, args.AdminAddress, args.ExpectedAdminAmount.Denom)
			s.Assert().Equal(args.ExpectedAdminAmount, bal)
		},
	}

	underlying := "under"
	shares := "vaultshares"
	admin := s.adminAddr
	vaultAddr := types.GetVaultAddress(shares)
	amount := sdk.NewInt64Coin(underlying, 500)

	setup := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlying, math.NewInt(1000)), admin)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           admin.String(),
			ShareDenom:      shares,
			UnderlyingAsset: underlying,
		})
		s.Require().NoError(err)
		s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, vaultAddr, sdk.NewCoins(amount)))
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	}

	s.Run("happy path - withdraw interest funds", func() {
		ev := createSendCoinEvents(vaultAddr.String(), admin.String(), sdk.NewCoins(amount).String())
		ev = append(ev, sdk.NewEvent(
			"vault.v1.EventInterestWithdrawal",
			sdk.NewAttribute("admin", admin.String()),
			sdk.NewAttribute("amount", CoinToJSON(amount)),
			sdk.NewAttribute("vault_address", vaultAddr.String()),
		))

		tc := msgServerTestCase[types.MsgWithdrawInterestFundsRequest, postCheckArgs]{
			name:  "happy path",
			setup: setup,
			msg: types.MsgWithdrawInterestFundsRequest{
				Admin:        admin.String(),
				VaultAddress: vaultAddr.String(),
				Amount:       amount,
			},
			postCheckArgs: postCheckArgs{
				AdminAddress:        admin,
				ExpectedAdminAmount: amount,
			},
			expectedEvents: ev,
		}

		testDef.expectedResponse = &types.MsgWithdrawInterestFundsResponse{}
		runMsgServerTestCase(s, testDef, tc)
	})
}

func (s *TestSuite) TestMsgServer_WithdrawInterestFunds_Failures() {
	testDef := msgServerTestDef[types.MsgWithdrawInterestFundsRequest, types.MsgWithdrawInterestFundsResponse, any]{
		endpointName: "WithdrawInterestFunds",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).WithdrawInterestFunds,
		postCheck:    nil,
	}

	underlying := "under"
	shares := "vaultshares"
	admin := s.adminAddr
	other := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1000))
	vaultAddr := types.GetVaultAddress(shares)
	markerAddr := markertypes.MustGetMarkerAddress(shares)
	amount := sdk.NewInt64Coin(underlying, 500)

	setup := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlying, math.NewInt(1000)), admin)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           admin.String(),
			ShareDenom:      shares,
			UnderlyingAsset: underlying,
		})
		s.Require().NoError(err)
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	}

	setupWithVaultFunds := func() {
		setup()
		s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, vaultAddr, sdk.NewCoins(amount)))
	}

	tests := []msgServerTestCase[types.MsgWithdrawInterestFundsRequest, any]{
		{
			name: "vault does not exist",
			msg: types.MsgWithdrawInterestFundsRequest{
				Admin:        admin.String(),
				VaultAddress: types.GetVaultAddress("doesnotexist").String(),
				Amount:       amount,
			},
			expectedErrSubstrs: []string{"not found"},
		},
		{
			name:  "invalid vault address not a vault account",
			setup: setup,
			msg: types.MsgWithdrawInterestFundsRequest{
				Admin:        admin.String(),
				VaultAddress: markerAddr.String(),
				Amount:       amount,
			},
			expectedErrSubstrs: []string{"failed to get vault", "is not a vault account"},
		},
		{
			name:  "unauthorized admin",
			setup: setupWithVaultFunds,
			msg: types.MsgWithdrawInterestFundsRequest{
				Admin:        other.String(),
				VaultAddress: vaultAddr.String(),
				Amount:       amount,
			},
			expectedErrSubstrs: []string{"unauthorized", "is not the vault admin"},
		},
		{
			name:  "insufficient vault balance",
			setup: setup,
			msg: types.MsgWithdrawInterestFundsRequest{
				Admin:        admin.String(),
				VaultAddress: vaultAddr.String(),
				Amount:       sdk.NewInt64Coin(underlying, 9_999_999),
			},
			expectedErrSubstrs: []string{"failed to withdraw funds", "insufficient funds"},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			runMsgServerTestCase(s, testDef, tc)
		})
	}
}

func (s *TestSuite) TestMsgServer_DepositPrincipalFunds() {
	type postCheckArgs struct {
		VaultAddress        sdk.AccAddress
		MarkerAddress       sdk.AccAddress
		ExpectedVaultAssets sdk.Coin
	}

	testDef := msgServerTestDef[types.MsgDepositPrincipalFundsRequest, types.MsgDepositPrincipalFundsResponse, postCheckArgs]{
		endpointName: "DepositPrincipalFunds",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).DepositPrincipalFunds,
		postCheck: func(msg *types.MsgDepositPrincipalFundsRequest, args postCheckArgs) {
			balance := s.k.BankKeeper.GetBalance(s.ctx, args.MarkerAddress, args.ExpectedVaultAssets.Denom)
			s.Assert().Equal(args.ExpectedVaultAssets, balance)
		},
	}

	underlying := "under"
	share := "vaultshares"
	admin := s.adminAddr
	vaultAddr := types.GetVaultAddress(share)
	markerAddr := markertypes.MustGetMarkerAddress(share)
	amount := sdk.NewInt64Coin(underlying, 500)

	setup := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlying, math.NewInt(1000)), admin)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           admin.String(),
			ShareDenom:      share,
			UnderlyingAsset: underlying,
		})
		s.Require().NoError(err)
		s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, vaultAddr, sdk.NewCoins(amount)))
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	}

	s.Run("happy path - deposit principal funds", func() {
		// Note: keeper now reconciles interest before the principal change.
		// With interest disabled by default, no reconcile event is emitted.
		ev := createSendCoinEvents(vaultAddr.String(), markerAddr.String(), sdk.NewCoins(amount).String())
		ev = append(ev, sdk.NewEvent(
			"vault.v1.EventDepositPrincipalFunds",
			sdk.NewAttribute("admin", admin.String()),
			sdk.NewAttribute("amount", CoinToJSON(amount)),
			sdk.NewAttribute("vault_address", vaultAddr.String()),
		))

		tc := msgServerTestCase[types.MsgDepositPrincipalFundsRequest, postCheckArgs]{
			name:  "happy path",
			setup: setup,
			msg: types.MsgDepositPrincipalFundsRequest{
				Admin:        admin.String(),
				VaultAddress: vaultAddr.String(),
				Amount:       amount,
			},
			postCheckArgs: postCheckArgs{
				VaultAddress:        vaultAddr,
				MarkerAddress:       markerAddr,
				ExpectedVaultAssets: amount,
			},
			expectedEvents: ev,
		}

		testDef.expectedResponse = &types.MsgDepositPrincipalFundsResponse{}
		runMsgServerTestCase(s, testDef, tc)
	})
}

func (s *TestSuite) TestMsgServer_DepositPrincipalFunds_Failures() {
	testDef := msgServerTestDef[types.MsgDepositPrincipalFundsRequest, types.MsgDepositPrincipalFundsResponse, any]{
		endpointName: "DepositPrincipalFunds",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).DepositPrincipalFunds,
		postCheck:    nil,
	}

	underlying := "under"
	share := "vaultshares"
	admin := s.adminAddr
	other := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1000))
	vaultAddr := types.GetVaultAddress(share)
	amount := sdk.NewInt64Coin(underlying, 500)

	setup := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlying, math.NewInt(1000)), admin)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           admin.String(),
			ShareDenom:      share,
			UnderlyingAsset: underlying,
		})
		s.Require().NoError(err)
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	}

	tests := []msgServerTestCase[types.MsgDepositPrincipalFundsRequest, any]{
		{
			name: "vault does not exist",
			msg: types.MsgDepositPrincipalFundsRequest{
				Admin:        admin.String(),
				VaultAddress: types.GetVaultAddress("doesnotexist").String(),
				Amount:       amount,
			},
			expectedErrSubstrs: []string{"not found"},
		},
		{
			name:  "invalid vault address not a vault account",
			setup: setup,
			msg: types.MsgDepositPrincipalFundsRequest{
				Admin:        admin.String(),
				VaultAddress: markertypes.MustGetMarkerAddress(share).String(),
				Amount:       amount,
			},
			expectedErrSubstrs: []string{"failed to get vault", "is not a vault account"},
		},
		{
			name:  "unauthorized admin",
			setup: setup,
			msg: types.MsgDepositPrincipalFundsRequest{
				Admin:        other.String(),
				VaultAddress: vaultAddr.String(),
				Amount:       amount,
			},
			expectedErrSubstrs: []string{"unauthorized", "is not the vault admin"},
		},
		{
			name:  "invalid asset for vault",
			setup: setup,
			msg: types.MsgDepositPrincipalFundsRequest{
				Admin:        admin.String(),
				VaultAddress: vaultAddr.String(),
				Amount:       sdk.NewInt64Coin("wrongdenom", 500),
			},
			expectedErrSubstrs: []string{"invalid asset for vault"},
		},
		{
			name:  "insufficient vault balance",
			setup: setup,
			msg: types.MsgDepositPrincipalFundsRequest{
				Admin:        admin.String(),
				VaultAddress: vaultAddr.String(),
				Amount:       sdk.NewInt64Coin(underlying, 9_999_999),
			},
			expectedErrSubstrs: []string{"failed to deposit principal funds", "insufficient funds"},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			runMsgServerTestCase(s, testDef, tc)
		})
	}
}

func (s *TestSuite) TestMsgServer_WithdrawPrincipalFunds() {
	type postCheckArgs struct {
		AdminAddress        sdk.AccAddress
		MarkerAddress       sdk.AccAddress
		ExpectedAdminAssets sdk.Coin
	}

	testDef := msgServerTestDef[types.MsgWithdrawPrincipalFundsRequest, types.MsgWithdrawPrincipalFundsResponse, postCheckArgs]{
		endpointName: "WithdrawPrincipalFunds",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).WithdrawPrincipalFunds,
		postCheck: func(msg *types.MsgWithdrawPrincipalFundsRequest, args postCheckArgs) {
			balance := s.k.BankKeeper.GetBalance(s.ctx, args.AdminAddress, args.ExpectedAdminAssets.Denom)
			s.Assert().Equal(args.ExpectedAdminAssets, balance)
		},
	}

	underlying := "under"
	share := "vaultshares"
	admin := s.adminAddr
	vaultAddr := types.GetVaultAddress(share)
	markerAddr := markertypes.MustGetMarkerAddress(share)
	amount := sdk.NewInt64Coin(underlying, 500)

	setup := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlying, math.NewInt(1000)), admin)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           admin.String(),
			ShareDenom:      share,
			UnderlyingAsset: underlying,
		})
		s.Require().NoError(err)
		s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, markerAddr, sdk.NewCoins(amount)))
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	}

	s.Run("happy path - withdraw principal funds", func() {
		ev := createSendCoinEvents(markerAddr.String(), admin.String(), sdk.NewCoins(amount).String())
		ev = append(ev, sdk.NewEvent(
			"vault.v1.EventWithdrawPrincipalFunds",
			sdk.NewAttribute("admin", admin.String()),
			sdk.NewAttribute("amount", CoinToJSON(amount)),
			sdk.NewAttribute("vault_address", vaultAddr.String()),
		))

		tc := msgServerTestCase[types.MsgWithdrawPrincipalFundsRequest, postCheckArgs]{
			name:  "happy path",
			setup: setup,
			msg: types.MsgWithdrawPrincipalFundsRequest{
				Admin:        admin.String(),
				VaultAddress: vaultAddr.String(),
				Amount:       amount,
			},
			postCheckArgs: postCheckArgs{
				AdminAddress:        admin,
				MarkerAddress:       markerAddr,
				ExpectedAdminAssets: amount,
			},
			expectedEvents: ev,
		}

		testDef.expectedResponse = &types.MsgWithdrawPrincipalFundsResponse{}
		runMsgServerTestCase(s, testDef, tc)
	})
}

func (s *TestSuite) TestMsgServer_WithdrawPrincipalFunds_Failures() {
	testDef := msgServerTestDef[types.MsgWithdrawPrincipalFundsRequest, types.MsgWithdrawPrincipalFundsResponse, any]{
		endpointName: "WithdrawPrincipalFunds",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).WithdrawPrincipalFunds,
		postCheck:    nil,
	}

	underlying := "under"
	share := "vaultshares"
	admin := s.adminAddr
	other := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1000))
	vaultAddr := types.GetVaultAddress(share)
	markerAddr := markertypes.MustGetMarkerAddress(share)
	amount := sdk.NewInt64Coin(underlying, 500)

	setup := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlying, math.NewInt(1000)), admin)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           admin.String(),
			ShareDenom:      share,
			UnderlyingAsset: underlying,
		})
		s.Require().NoError(err)
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	}

	tests := []msgServerTestCase[types.MsgWithdrawPrincipalFundsRequest, any]{
		{
			name: "vault does not exist",
			msg: types.MsgWithdrawPrincipalFundsRequest{
				Admin:        admin.String(),
				VaultAddress: types.GetVaultAddress("doesnotexist").String(),
				Amount:       amount,
			},
			expectedErrSubstrs: []string{"not found"},
		},
		{
			name:  "invalid vault address not a vault account",
			setup: setup,
			msg: types.MsgWithdrawPrincipalFundsRequest{
				Admin:        admin.String(),
				VaultAddress: markerAddr.String(),
				Amount:       amount,
			},
			expectedErrSubstrs: []string{"failed to get vault", "is not a vault account"},
		},
		{
			name:  "unauthorized admin",
			setup: setup,
			msg: types.MsgWithdrawPrincipalFundsRequest{
				Admin:        other.String(),
				VaultAddress: vaultAddr.String(),
				Amount:       amount,
			},
			expectedErrSubstrs: []string{"unauthorized", "is not the vault admin"},
		},
		{
			name:  "invalid asset for vault",
			setup: setup,
			msg: types.MsgWithdrawPrincipalFundsRequest{
				Admin:        admin.String(),
				VaultAddress: vaultAddr.String(),
				Amount:       sdk.NewInt64Coin("wrongdenom", 500),
			},
			expectedErrSubstrs: []string{"invalid asset for vault"},
		},
		{
			name:  "insufficient marker balance",
			setup: setup,
			msg: types.MsgWithdrawPrincipalFundsRequest{
				Admin:        admin.String(),
				VaultAddress: vaultAddr.String(),
				Amount:       sdk.NewInt64Coin(underlying, 9_999_999),
			},
			expectedErrSubstrs: []string{"failed to withdraw principal funds", "insufficient funds"},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			runMsgServerTestCase(s, testDef, tc)
		})
	}
}

// createMarkerMintCoinEvents creates events for minting a coin and sending it to a recipient.
func createMarkerMintCoinEvents(markerModule, admin, recipient sdk.AccAddress, coin sdk.Coin) []sdk.Event {
	events := createReceiveCoinsEvents(markerModule.String(), sdk.NewCoins(coin).String())

	sendEvents := createSendCoinEvents(markerModule.String(), recipient.String(), sdk.NewCoins(coin).String())
	events = append(events, sendEvents...)

	// The specific marker mint event
	markerMintEvent := sdk.NewEvent("provenance.marker.v1.EventMarkerMint",
		sdk.NewAttribute("administrator", admin.String()),
		sdk.NewAttribute("amount", coin.Amount.String()),
		sdk.NewAttribute("denom", coin.Denom),
	)
	events = append(events, markerMintEvent)

	return events
}

// createBurnCoinEvents creates events for minting a coin and sending it to a recipient.
func createBurnCoinEvents(burner, amount string) []sdk.Event {
	events := sdk.NewEventManager().Events()

	events = events.AppendEvent(sdk.NewEvent(
		banktypes.EventTypeCoinSpent,
		sdk.NewAttribute(banktypes.AttributeKeySpender, burner),
		sdk.NewAttribute(sdk.AttributeKeyAmount, amount),
	))

	events = events.AppendEvent(sdk.NewEvent(
		banktypes.EventTypeCoinBurn,
		sdk.NewAttribute(banktypes.AttributeKeyBurner, burner),
		sdk.NewAttribute(sdk.AttributeKeyAmount, amount),
	))

	return events
}

// createMarkerWithdraw creates events for withdrawing a coin from a marker.
func createMarkerWithdraw(administrator, sender sdk.AccAddress, recipient sdk.AccAddress, shares sdk.Coin) []sdk.Event {
	events := createSendCoinEvents(sender.String(), recipient.String(), sdk.NewCoins(shares).String())

	// The specific marker withdraw event
	withdrawEvent := sdk.NewEvent("provenance.marker.v1.EventMarkerWithdraw",
		sdk.NewAttribute("administrator", administrator.String()),
		sdk.NewAttribute("coins", sdk.NewCoins(shares).String()),
		sdk.NewAttribute("denom", shares.Denom),
		sdk.NewAttribute("to_address", recipient.String()),
	)

	events = append(events, withdrawEvent)

	return events
}

// createMarkerBurn creates events for burning a coin from a marker.
func createMarkerBurn(admin, markerAddr sdk.AccAddress, shares sdk.Coin) []sdk.Event {
	markerModule := authtypes.NewModuleAddress(markertypes.ModuleName)
	events := createSendCoinEvents(markerAddr.String(), markerModule.String(), sdk.NewCoins(shares).String())

	burnEvents := createBurnCoinEvents(markerModule.String(), shares.String())
	events = append(events, burnEvents...)

	// The specific marker burn event
	markerBurnEvent := sdk.NewEvent("provenance.marker.v1.EventMarkerBurn",
		sdk.NewAttribute("administrator", admin.String()),
		sdk.NewAttribute("amount", shares.Amount.String()),
		sdk.NewAttribute("denom", shares.Denom),
	)
	events = append(events, markerBurnEvent)

	return events
}

// createSwapOutEvents creates the full set of expected events for a successful SwapOut.
func createSwapOutEvents(owner, vaultAddr, markerAddr sdk.AccAddress, assets, shares sdk.Coin) []sdk.Event {
	var allEvents []sdk.Event

	// 1. owner sends shares to markerAddr
	sendToMarkerEvents := createSendCoinEvents(owner.String(), markerAddr.String(), shares.String())
	allEvents = append(allEvents, sendToMarkerEvents...)

	// 2. vaultAddr (as admin) burns shares.
	burnEvents := createMarkerBurn(vaultAddr, markerAddr, shares)
	allEvents = append(allEvents, burnEvents...)

	// 3. vaultAddr sends assets to owner
	sendAssetEvents := createSendCoinEvents(markerAddr.String(), owner.String(), assets.String())
	allEvents = append(allEvents, sendAssetEvents...)

	// 4. The vault's own SwapOut event
	swapOutEvent := sdk.NewEvent("vault.v1.EventSwapOut",
		sdk.NewAttribute("amount_out", CoinToJSON(assets)),
		sdk.NewAttribute("owner", owner.String()),
		sdk.NewAttribute("shares_burned", CoinToJSON(shares)),
		sdk.NewAttribute("vault_address", vaultAddr.String()),
	)
	allEvents = append(allEvents, swapOutEvent)

	return allEvents
}

// createSwapInEvents creates the full set of expected events for a successful SwapIn.
func createSwapInEvents(owner, vaultAddr, markerAddr sdk.AccAddress, asset, shares sdk.Coin) []sdk.Event {
	var allEvents []sdk.Event

	markerModule := authtypes.NewModuleAddress(markertypes.ModuleName)
	mintEvents := createMarkerMintCoinEvents(markerModule, vaultAddr, markerAddr, shares)
	allEvents = append(allEvents, mintEvents...)

	withdrawEvents := createMarkerWithdraw(vaultAddr, markerAddr, owner, shares)
	allEvents = append(allEvents, withdrawEvents...)

	sendAssetEvents := createSendCoinEvents(owner.String(), markerAddr.String(), sdk.NewCoins(asset).String())
	allEvents = append(allEvents, sendAssetEvents...)

	swapInEvent := sdk.NewEvent("vault.v1.EventSwapIn",
		sdk.NewAttribute("amount_in", CoinToJSON(asset)),
		sdk.NewAttribute("owner", owner.String()),
		sdk.NewAttribute("shares_received", CoinToJSON(shares)),
		sdk.NewAttribute("vault_address", vaultAddr.String()),
	)
	allEvents = append(allEvents, swapInEvent)

	return allEvents
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
