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
			_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
				Admin:           owner.String(),
				ShareDenom:      shareDenom,
				UnderlyingAsset: underlyingDenom,
			})
			s.Require().NoError(err)
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

	// Base setup for many tests
	setup := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlyingDenom, math.NewInt(1000)), owner)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           owner.String(),
			ShareDenom:      shareDenom,
			UnderlyingAsset: underlyingDenom,
		})
		s.Require().NoError(err)
		err = FundAccount(s.ctx, s.simApp.BankKeeper, owner, sdk.NewCoins(assets))
		s.Require().NoError(err)
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
			name:  "underlying asset mismatch",
			setup: setup,
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
				setup()
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
		// Create marker for underlying asset
		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlyingDenom, math.NewInt(1000)), owner)
		// Create the vault
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           owner.String(),
			ShareDenom:      shareDenom,
			UnderlyingAsset: underlyingDenom,
		})
		s.Require().NoError(err)
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

	// Base setup for many tests
	setup := func() {
		// Create marker for underlying asset
		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlyingDenom, math.NewInt(1000)), owner)
		// Create the vault
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           owner.String(),
			ShareDenom:      shareDenom,
			UnderlyingAsset: underlyingDenom,
		})
		s.Require().NoError(err)
		// Fund owner with underlying assets
		err = FundAccount(s.ctx, s.simApp.BankKeeper, owner, sdk.NewCoins(initialAssets))
		s.Require().NoError(err)

		// Owner swaps in to get shares
		_, err = s.k.SwapIn(s.ctx, vaultAddr, owner, initialAssets)
		s.Require().NoError(err)
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
			setup: setup,
			msg: types.MsgSwapOutRequest{
				Owner:        owner.String(),
				VaultAddress: vaultAddr.String(),
				Assets:       sdk.NewInt64Coin("wrongdenom", 50),
			},
			expectedErrSubstrs: []string{"swap out denom must be share denom", "wrongdenom", shareDenom},
		},
		{
			name:  "insufficient shares",
			setup: setup,
			msg: types.MsgSwapOutRequest{
				Owner:        owner.String(),
				VaultAddress: vaultAddr.String(),
				Assets:       sdk.NewInt64Coin(shareDenom, 150),
			},
			expectedErrSubstrs: []string{"failed to send shares to marker", "insufficient funds"},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
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
				sdk.NewEvent("vault.v1.EventInterestRateUpdated",
					sdk.NewAttribute("vault_address", vaultAddr.String()),
					sdk.NewAttribute("new_rate", "0.05"),
					sdk.NewAttribute("previous_rate", "0.00"),
				),
			},
		},
		// {name: "update current interest rate to non zero, needs to reconcile previous rate",
		// 	interestRate: "4.06",
		// 	setup: func() {
		// 		setup()
		// 		vaultAcc, err := s.k.GetVault(s.ctx, vaultAddr)
		// 		s.Require().NoError(err, "should be able to get vault")
		// 		s.k.UpdateInterestRates(s.ctx, vaultAcc, "4.20", "4.20")
		// 		s.k.VaultInterestDetails.Set(s.ctx, vaultAddr, types.VaultInterestDetails{PeriodStart: currentBlockTime.Unix() - 10000})
		// 	},
		// 	postCheckArgs: postCheckArgs{
		// 		VaultAddress:          vaultAddr,
		// 		ExpectedRate:          "4.06",
		// 		HasNewInterestDetails: true,
		// 		ExpectedPeriodStart:   currentBlockTime.Unix(),
		// 	},
		// 	expectedEvents: sdk.Events{
		// 		sdk.NewEvent("vault.v1.EventInterestRateUpdated",
		// 			sdk.NewAttribute("vault_address", vaultAddr.String()),
		// 			sdk.NewAttribute("new_rate", "0.05"),
		// 			sdk.NewAttribute("previous_rate", "0.00"),
		// 		),
		// 	},
		// },
		// {name: "update current interest rate to zero, needs to reconcile previous non zero rate",
		// 	interestRate: "0",
		// 	setup: func() {
		// 		setup()
		// 		vaultAcc, err := s.k.GetVault(s.ctx, vaultAddr)
		// 		s.Require().NoError(err, "should be able to get vault")
		// 		s.k.UpdateInterestRates(s.ctx, vaultAcc, "6.12", "6.12")
		// 		s.k.VaultInterestDetails.Set(s.ctx, vaultAddr, types.VaultInterestDetails{PeriodStart: currentBlockTime.Unix() - 10000})
		// 	},
		// 	postCheckArgs: postCheckArgs{
		// 		VaultAddress:          vaultAddr,
		// 		ExpectedRate:          types.ZeroInterestRate,
		// 		HasNewInterestDetails: false,
		// 	},
		// 	expectedEvents: sdk.Events{
		// 		sdk.NewEvent("vault.v1.EventInterestRateUpdated",
		// 			sdk.NewAttribute("vault_address", vaultAddr.String()),
		// 			sdk.NewAttribute("new_rate", "0.05"),
		// 			sdk.NewAttribute("previous_rate", "0.00"),
		// 		),
		// 	},
		// },
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

// createMarkerMintCoinEvents creates events for minting a coin and sending it to a recipient.
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
