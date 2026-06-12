package keeper_test

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"cosmossdk.io/collections"
	"cosmossdk.io/math"
	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	attrtypes "github.com/provenance-io/provenance/x/attribute/types"
	markertypes "github.com/provenance-io/provenance/x/marker/types"

	"github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/types"
	"github.com/provlabs/vault/utils"
)

func (s *TestSuite) TestMsgServer_CreateVault() {
	type postCheckArgs struct {
		UnderlyingAsset string
		ShareDenom      string
		Admin           string
		VaultAddr       sdk.AccAddress
		MinSwapInValue  string
		MinSwapOutValue string
		MaxSwapInValue  string
		MaxSwapOutValue string
	}

	testDef := msgServerTestDef[types.MsgCreateVaultRequest, types.MsgCreateVaultResponse, postCheckArgs]{
		endpointName: "CreateVault",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).CreateVault,
		postCheck: func(msg *types.MsgCreateVaultRequest, postCheckArgs postCheckArgs) {
			markerAddr := markertypes.MustGetMarkerAddress(postCheckArgs.ShareDenom)

			marker, err := s.simApp.MarkerKeeper.GetMarker(s.ctx, markerAddr)
			s.Require().NoError(err, "failed to get marker for share denom %s", postCheckArgs.ShareDenom)

			s.EqualValues(0, marker.GetSupply().Amount.Int64(), "vault marker supply mismatch for share denom %s", postCheckArgs.ShareDenom)
			s.False(marker.AllowsForcedTransfer(), "vault marker %s should not have forced transfer enabled", postCheckArgs.ShareDenom)
			s.False(marker.HasGovernanceEnabled(), "vault marker %s should not have governance enabled", postCheckArgs.ShareDenom)
			s.True(marker.GetMarkerType() == markertypes.MarkerType_Coin, "vault marker %s should be of type COIN", postCheckArgs.ShareDenom)
			s.False(marker.HasGovernanceEnabled(), "vault marker %s should not allow governance control", postCheckArgs.ShareDenom)

			access := marker.GetAccessList()
			s.Require().Len(access, 1, "expected exactly one access entry for vault marker %s", postCheckArgs.ShareDenom)
			s.Equal(types.GetVaultAddress(postCheckArgs.ShareDenom).String(), access[0].Address, "vault marker %s access should be granted to vault account", postCheckArgs.ShareDenom)
			s.ElementsMatch(
				[]markertypes.Access{
					markertypes.Access_Mint,
					markertypes.Access_Burn,
					markertypes.Access_Withdraw,
				},
				access[0].Permissions,
				"vault marker %s permissions mismatch", postCheckArgs.ShareDenom,
			)

			// Check vault record exists
			account := s.simApp.AccountKeeper.GetAccount(s.ctx, postCheckArgs.VaultAddr)
			s.Require().NotNil(account, "vault account %s not found in state", postCheckArgs.VaultAddr)

			vaultAcc, ok := account.(*types.VaultAccount)
			s.Require().True(ok, "account %s is not of type *VaultAccount", postCheckArgs.VaultAddr)

			s.Equal(postCheckArgs.Admin, vaultAcc.GetAdmin(), "vault %s admin address mismatch", postCheckArgs.VaultAddr)
			s.Equal(
				types.GetVaultAddress(postCheckArgs.ShareDenom),
				vaultAcc.GetAddress(),
				"vault %s address mismatch; derived address differs", postCheckArgs.VaultAddr,
			)
			s.Equal(
				postCheckArgs.UnderlyingAsset,
				vaultAcc.GetUnderlyingAsset(),
				"vault %s underlying asset denom mismatch", postCheckArgs.VaultAddr,
			)
			s.Equal(vaultAcc.GetTotalShares(), sdk.NewCoin(postCheckArgs.ShareDenom, math.ZeroInt()), "vault %s initial total shares should be zero", postCheckArgs.VaultAddr)
			s.Equal(postCheckArgs.MinSwapInValue, vaultAcc.MinSwapInValue, "vault %s min swap in value mismatch", postCheckArgs.VaultAddr)
			s.Equal(postCheckArgs.MinSwapOutValue, vaultAcc.MinSwapOutValue, "vault %s min swap out value mismatch", postCheckArgs.VaultAddr)
			s.Equal(postCheckArgs.MaxSwapInValue, vaultAcc.MaxSwapInValue, "vault %s max swap in value mismatch", postCheckArgs.VaultAddr)
			s.Equal(postCheckArgs.MaxSwapOutValue, vaultAcc.MaxSwapOutValue, "vault %s max swap out value mismatch", postCheckArgs.VaultAddr)
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
		MinSwapInValue:  "100",
		MinSwapOutValue: "50",
		MaxSwapInValue:  "1000",
		MaxSwapOutValue: "500",
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
			MinSwapInValue:  "100",
			MinSwapOutValue: "50",
			MaxSwapInValue:  "1000",
			MaxSwapOutValue: "500",
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
			sdk.NewEvent("provlabs.vault.v1.EventVaultCreated",
				sdk.NewAttribute("admin", admin),
				sdk.NewAttribute("share_denom", sharedenom),
				sdk.NewAttribute("underlying_asset", underlying),
				sdk.NewAttribute("vault_address", vaultAddr.String()),
			),
		},
	}

	testDef.expectedResponse = &types.MsgCreateVaultResponse{VaultAddress: vaultAddr.String()}
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

func (s *TestSuite) TestMsgServer_SetShareDenomMetadata() {
	type postCheckArgs struct {
		VaultAddress     sdk.AccAddress
		ExpectedMetadata banktypes.Metadata
	}

	testDef := msgServerTestDef[types.MsgSetShareDenomMetadataRequest, types.MsgSetShareDenomMetadataResponse, postCheckArgs]{
		endpointName: "SetShareDenomMetadata",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).SetShareDenomMetadata,
		postCheck: func(msg *types.MsgSetShareDenomMetadataRequest, args postCheckArgs) {
			actualMetadata, found := s.simApp.BankKeeper.GetDenomMetaData(s.ctx, args.ExpectedMetadata.Base)
			s.Require().True(found, "metadata for denom %s not found in BankKeeper after update", args.ExpectedMetadata.Base)
			s.Assert().Equal(args.ExpectedMetadata.Base, actualMetadata.Base, "metadata base denom mismatch for %s", args.ExpectedMetadata.Base)
			s.Assert().Equal(args.ExpectedMetadata.Display, actualMetadata.Display, "metadata display denom mismatch for %s", args.ExpectedMetadata.Base)
			s.Assert().Equal(args.ExpectedMetadata.Description, actualMetadata.Description, "metadata description mismatch for %s", args.ExpectedMetadata.Base)
			s.Assert().Equal(args.ExpectedMetadata.Name, actualMetadata.Name, "metadata name mismatch for %s", args.ExpectedMetadata.Base)
			s.Assert().Equal(args.ExpectedMetadata.Symbol, actualMetadata.Symbol, "metadata symbol mismatch for %s", args.ExpectedMetadata.Base)
			s.Assert().Equal(len(args.ExpectedMetadata.DenomUnits), len(actualMetadata.DenomUnits), "metadata denom units count mismatch for %s", args.ExpectedMetadata.Base)
		},
	}

	underlying := "under"
	share := "vaultshares"
	admin := s.adminAddr
	vaultAddr := types.GetVaultAddress(share)

	metadata := banktypes.Metadata{
		Base:        share,
		Display:     share,
		Description: "Vault shares for testing",
		Name:        "Vault Shares",
		Symbol:      "VSHARES",
		DenomUnits: []*banktypes.DenomUnit{
			{
				Denom:    share,
				Exponent: 0,
				Aliases:  []string{"vshare"},
			},
			{
				Denom:    "mvshare",
				Exponent: 6,
				Aliases:  []string{},
			},
		},
	}

	setup := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlying, math.NewInt(1000)), admin)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           admin.String(),
			ShareDenom:      share,
			UnderlyingAsset: underlying,
		})
		s.Require().NoError(err, "setup: expected vault creation to succeed")
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	}

	tc := msgServerTestCase[types.MsgSetShareDenomMetadataRequest, postCheckArgs]{
		name:  "happy path",
		setup: setup,
		msg: types.MsgSetShareDenomMetadataRequest{
			Admin:        admin.String(),
			VaultAddress: vaultAddr.String(),
			Metadata:     metadata,
		},
		postCheckArgs: postCheckArgs{
			VaultAddress:     vaultAddr,
			ExpectedMetadata: metadata,
		},
		expectedEvents: sdk.Events{
			sdk.NewEvent("provlabs.vault.v1.EventSetShareDenomMetadata",
				sdk.NewAttribute("administrator", admin.String()),
				sdk.NewAttribute("metadata_base", share),
				sdk.NewAttribute("metadata_denom_units", `[{"denom":"vaultshares","exponent":"0","aliases":["vshare"]},{"denom":"mvshare","exponent":"6","aliases":[]}]`),
				sdk.NewAttribute("metadata_description", "Vault shares for testing"),
				sdk.NewAttribute("metadata_display", share),
				sdk.NewAttribute("metadata_name", "Vault Shares"),
				sdk.NewAttribute("metadata_symbol", "VSHARES"),
				sdk.NewAttribute("vault_address", vaultAddr.String()),
			),
		},
	}

	testDef.expectedResponse = &types.MsgSetShareDenomMetadataResponse{}
	runMsgServerTestCase(s, testDef, tc)
}

func (s *TestSuite) TestMsgServer_SetShareDenomMetadata_Failures() {
	testDef := msgServerTestDef[types.MsgSetShareDenomMetadataRequest, types.MsgSetShareDenomMetadataResponse, any]{
		endpointName: "SetShareDenomMetadata",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).SetShareDenomMetadata,
		postCheck:    nil,
	}

	underlying := "under"
	share := "vaultshares"
	wrongShare := "wrongshares"
	admin := s.adminAddr
	other := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1))
	vaultAddr := types.GetVaultAddress(share)
	markerAddr := markertypes.MustGetMarkerAddress(share)

	validMetadata := banktypes.Metadata{
		Base:        share,
		Display:     share,
		Description: "Vault shares for testing",
		Name:        "Vault Shares",
		Symbol:      "VSHARES",
		DenomUnits: []*banktypes.DenomUnit{
			{
				Denom:    share,
				Exponent: 0,
				Aliases:  []string{},
			},
		},
	}

	wrongMetadata := banktypes.Metadata{
		Base:        wrongShare,
		Display:     wrongShare,
		Description: "Wrong vault shares",
		Name:        "Wrong Shares",
		Symbol:      "WRONG",
		DenomUnits: []*banktypes.DenomUnit{
			{
				Denom:    wrongShare,
				Exponent: 0,
				Aliases:  []string{},
			},
		},
	}

	base := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlying, math.NewInt(1000)), admin)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           admin.String(),
			ShareDenom:      share,
			UnderlyingAsset: underlying,
		})
		s.Require().NoError(err, "base: expected vault creation to succeed")
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	}

	tests := []msgServerTestCase[types.MsgSetShareDenomMetadataRequest, any]{
		{
			name: "vault not found",
			setup: func() {
				s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
			},
			msg: types.MsgSetShareDenomMetadataRequest{
				Admin:        admin.String(),
				VaultAddress: types.GetVaultAddress("missing").String(),
				Metadata:     validMetadata,
			},
			expectedErrSubstrs: []string{"vault not found"},
		},
		{
			name:  "invalid vault address (not a vault account)",
			setup: base,
			msg: types.MsgSetShareDenomMetadataRequest{
				Admin:        admin.String(),
				VaultAddress: markerAddr.String(),
				Metadata:     validMetadata,
			},
			expectedErrSubstrs: []string{"failed to get vault", "is not a vault account"},
		},
		{
			name:  "unauthorized admin",
			setup: base,
			msg: types.MsgSetShareDenomMetadataRequest{
				Admin:        other.String(),
				VaultAddress: vaultAddr.String(),
				Metadata:     validMetadata,
			},
			expectedErrSubstrs: []string{"unauthorized", "is not the vault admin"},
		},
		{
			name:  "metadata base denom does not match vault share denom",
			setup: base,
			msg: types.MsgSetShareDenomMetadataRequest{
				Admin:        admin.String(),
				VaultAddress: vaultAddr.String(),
				Metadata:     wrongMetadata,
			},
			expectedErrSubstrs: []string{"metadata base denom", "does not match vault share denom"},
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
			s.Require().NoError(err, "failed to get marker for share denom %s", args.Shares.Denom)
			s.Require().NotNil(marker, "marker for share denom %s should exist", args.Shares.Denom)
			supply := s.simApp.BankKeeper.GetSupply(s.ctx, args.Shares.Denom)
			s.Require().Equal(args.Shares.Amount, supply.Amount, "marker supply mismatch for share denom %s", args.Shares.Denom)

			// Check that the balance of the vault account has increased by the denom in the Msg.
			vaultBalance := s.simApp.BankKeeper.GetBalance(s.ctx, markerAddr, args.UnderlyingAsset.Denom)
			s.Require().Equal(args.UnderlyingAsset, vaultBalance, "vault balance mismatch for denom %s", args.UnderlyingAsset.Denom)

			// Check that the owner's balance contains the shares.
			ownerBalance := s.simApp.BankKeeper.GetBalance(s.ctx, args.Owner, args.Shares.Denom)
			s.Require().Equal(args.Shares, ownerBalance, "owner share balance mismatch for user %s", args.Owner)
		},
	}

	underlyingDenom := "underlying"
	shareDenom := "vaultshares"
	owner := s.adminAddr
	vaultAddr := types.GetVaultAddress(shareDenom)
	markerAddr := markertypes.MustGetMarkerAddress(shareDenom)
	assets := sdk.NewInt64Coin(underlyingDenom, 100)
	expectedShares := sdk.NewCoin(shareDenom, assets.Amount.Mul(utils.ShareScalar))

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
			s.Require().NoError(err, "failed to create vault in TestMsgServer_SwapIn setup")
			vault.SwapInEnabled = true
			s.k.AuthKeeper.SetAccount(s.ctx, vault)
			// Fund owner with underlying assets
			err = FundAccount(s.ctx, s.simApp.BankKeeper, owner, sdk.NewCoins(assets))
			s.Require().NoError(err, "failed to fund owner in TestMsgServer_SwapIn setup")
			s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
		},
		msg:                swapInReq,
		expectedErrSubstrs: nil,
		postCheckArgs:      postCheckArgs{Owner: owner, VaultAddr: vaultAddr, MarkerAddr: markerAddr, UnderlyingAsset: assets, Shares: expectedShares},
		expectedEvents:     createSwapInEvents(owner, vaultAddr, markerAddr, assets, expectedShares),
	}
	testDef.expectedResponse = &types.MsgSwapInResponse{SharesReceived: expectedShares}
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

	setup := func(swapInEnabled, vaultPaused bool, minSwapIn, maxSwapIn string) func() {
		return func() {
			s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlyingDenom, math.NewInt(1_000_000_000_000_000_000)), owner)
			vault, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
				Admin:           owner.String(),
				ShareDenom:      shareDenom,
				UnderlyingAsset: underlyingDenom,
				MinSwapInValue:  minSwapIn,
				MaxSwapInValue:  maxSwapIn,
			})
			s.Require().NoError(err, "vault creation should succeed")
			vault.SwapInEnabled = swapInEnabled
			vault.Paused = vaultPaused
			s.k.AuthKeeper.SetAccount(s.ctx, vault)

			err = FundAccount(s.ctx, s.simApp.BankKeeper, owner, sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, 1000)))
			s.Require().NoError(err, "funding owner with underlying should succeed")
		}
	}

	tests := []msgServerTestCase[types.MsgSwapInRequest, any]{
		{
			name: "vault not found returns not found error",
			msg: types.MsgSwapInRequest{
				Owner:        owner.String(),
				VaultAddress: vaultAddr.String(),
				Assets:       assets,
			},
			expectedErrSubstrs: []string{"vault", "not found"},
		},
		{
			name:  "swap in below minimum value",
			setup: setup(true, false, "200", ""),
			msg: types.MsgSwapInRequest{
				Owner:        owner.String(),
				VaultAddress: vaultAddr.String(),
				Assets:       sdk.NewInt64Coin(underlyingDenom, 50),
			},
			expectedErrSubstrs: []string{"below the minimum required value"},
		},
		{
			name:  "swap in above maximum value",
			setup: setup(true, false, "", "100"),
			msg: types.MsgSwapInRequest{
				Owner:        owner.String(),
				VaultAddress: vaultAddr.String(),
				Assets:       sdk.NewInt64Coin(underlyingDenom, 150),
			},
			expectedErrSubstrs: []string{"above the maximum allowed value"},
		},

		{
			name:  "swap in enabled, vaulted paused",
			setup: setup(true, true, "", ""),
			msg: types.MsgSwapInRequest{
				Owner:        owner.String(),
				VaultAddress: vaultAddr.String(),
				Assets:       assets,
			},
			expectedErrSubstrs: []string{"vault", "is paused"},
		},
		{
			name:  "swap in disabled is rejected",
			setup: setup(false, false, "", ""),
			msg: types.MsgSwapInRequest{
				Owner:        owner.String(),
				VaultAddress: vaultAddr.String(),
				Assets:       assets,
			},
			expectedErrSubstrs: []string{"swaps are not enabled for vault"},
		},
		{
			name:  "underlying denom mismatch is rejected",
			setup: setup(true, false, "", ""),
			msg: types.MsgSwapInRequest{
				Owner:        owner.String(),
				VaultAddress: vaultAddr.String(),
				Assets:       sdk.NewInt64Coin("othercoin", 100),
			},
			expectedErrSubstrs: []string{"denom not supported for vault; must be \"underlying\": got \"othercoin\""},
		},
		{
			name: "insufficient owner funds is rejected",
			setup: func() {
				setup(true, false, "", "")()
				err := s.simApp.BankKeeper.SendCoins(s.ctx, owner, authtypes.NewModuleAddress("burn"),
					sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, 950)))
				s.Require().NoError(err, "reducing owner's balance should succeed")
			},
			msg:                types.MsgSwapInRequest{Owner: owner.String(), VaultAddress: vaultAddr.String(), Assets: assets},
			expectedErrSubstrs: []string{"insufficient funds"},
		},
		{
			name: "swap in minted shares exceeding maximum mintable supply (precision-scaled above underlying assets) is rejected",
			setup: func() {
				setup(true, false, "", "")()
				err := FundAccount(s.ctx, s.simApp.BankKeeper, owner,
					sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, 1_000_000_000_000_000)))
				s.Require().NoError(err, "funding owner with very large underlying should succeed")
			},
			msg: types.MsgSwapInRequest{
				Owner:        owner.String(),
				VaultAddress: vaultAddr.String(),
				Assets:       sdk.NewCoin(underlyingDenom, math.NewInt(1_000_000_000_000_000)),
			},
			expectedErrSubstrs: []string{
				"requested supply 1000000000000000000000 exceeds maximum allowed value 100000000000000000000",
			},
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
		OwnerAddress         sdk.AccAddress
		VaultAddress         sdk.AccAddress
		ExpectedQueuedPayout sdk.Coin
		ExpectedOwnerShares  math.Int
		EscrowedShares       sdk.Coin
	}

	testDef := msgServerTestDef[types.MsgSwapOutRequest, types.MsgSwapOutResponse, postCheckArgs]{
		endpointName: "SwapOut",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).SwapOut,
		postCheck: func(msg *types.MsgSwapOutRequest, args postCheckArgs) {
			s.assertBalance(args.OwnerAddress, args.EscrowedShares.Denom, args.ExpectedOwnerShares)
			s.assertBalance(args.VaultAddress, args.EscrowedShares.Denom, args.EscrowedShares.Amount)
		},
	}

	underlyingDenom := "underlying"
	shareDenom := "vaultshares"
	ownerAddr := s.adminAddr
	vaultAddr := types.GetVaultAddress(shareDenom)
	deposit := sdk.NewInt64Coin(underlyingDenom, 100)

	setup := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlyingDenom, math.NewInt(1000)), ownerAddr)

		vault, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           ownerAddr.String(),
			ShareDenom:      shareDenom,
			UnderlyingAsset: underlyingDenom,
		})
		s.Require().NoError(err, "vault creation should succeed")

		vault.SwapInEnabled = true
		vault.SwapOutEnabled = true
		s.k.AuthKeeper.SetAccount(s.ctx, vault)

		err = FundAccount(s.ctx, s.simApp.BankKeeper, ownerAddr, sdk.NewCoins(deposit))
		s.Require().NoError(err, "funding owner account should succeed")

		s.ctx = s.ctx.WithBlockTime(time.Now())

		_, err = s.k.SwapIn(s.ctx, vaultAddr, ownerAddr, deposit)
		s.Require().NoError(err, "initial SwapIn should succeed for owner %s and vault %s", ownerAddr, vaultAddr)
	}

	testCases := []struct {
		name               string
		burnSharesScaled   int64
		expectedUnderlying int64
	}{
		{"swap out 30 shares -> queues withdrawal for 30 underlying", 30_000_000, 30},
		{"swap out 100 shares -> queues withdrawal for 100 underlying", 100_000_000, 100},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			escrowedShares := sdk.NewInt64Coin(shareDenom, tc.burnSharesScaled)
			expectedPayout := sdk.NewCoin(underlyingDenom, math.NewInt(tc.expectedUnderlying))
			initialOwnerShares := deposit.Amount.Mul(utils.ShareScalar)

			req := types.MsgSwapOutRequest{
				Owner:        ownerAddr.String(),
				VaultAddress: vaultAddr.String(),
				Assets:       escrowedShares,
			}

			args := postCheckArgs{
				OwnerAddress:         ownerAddr,
				VaultAddress:         vaultAddr,
				ExpectedQueuedPayout: expectedPayout,
				ExpectedOwnerShares:  initialOwnerShares.Sub(escrowedShares.Amount),
				EscrowedShares:       escrowedShares,
			}

			tcDef := msgServerTestCase[types.MsgSwapOutRequest, postCheckArgs]{
				name:           tc.name,
				setup:          setup,
				msg:            req,
				postCheckArgs:  args,
				expectedEvents: createSwapOutEvents(ownerAddr, vaultAddr, expectedPayout, escrowedShares),
			}

			testDef.expectedResponse = &types.MsgSwapOutResponse{RequestId: 0}
			runMsgServerTestCase(s, testDef, tcDef)
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
	initialAssets := sdk.NewInt64Coin(underlyingDenom, 1000)

	setup := func(swapOutEnabled, vaultPaused bool, minSwapOut, maxSwapOut string) func() {
		return func() {
			s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlyingDenom, math.NewInt(2000)), owner)
			vault, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
				Admin:           owner.String(),
				ShareDenom:      shareDenom,
				UnderlyingAsset: underlyingDenom,
				MinSwapOutValue: minSwapOut,
				MaxSwapOutValue: maxSwapOut,
			})
			s.Require().NoError(err, "vault creation should succeed")
			vault.SwapInEnabled = true
			vault.SwapOutEnabled = swapOutEnabled

			s.k.AuthKeeper.SetAccount(s.ctx, vault)

			err = FundAccount(s.ctx, s.simApp.BankKeeper, owner, sdk.NewCoins(initialAssets))
			s.Require().NoError(err, "funding owner should succeed")

			_, err = s.k.SwapIn(s.ctx, vaultAddr, owner, initialAssets)
			s.Require().NoError(err, "initial SwapIn should succeed for vault %s and owner %s", vaultAddr, owner)
			vault.Paused = vaultPaused
			s.k.AuthKeeper.SetAccount(s.ctx, vault)
		}
	}

	randomAddr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()
	mintedShares := initialAssets.Amount.Mul(utils.ShareScalar)
	overdrawShares := mintedShares.Add(math.NewInt(1)) // force insufficient funds

	tests := []msgServerTestCase[types.MsgSwapOutRequest, any]{
		{
			name: "vault does not exist returns not found error",
			msg: types.MsgSwapOutRequest{
				Owner:        owner.String(),
				VaultAddress: randomAddr,
				Assets:       sdk.NewInt64Coin(shareDenom, 50),
			},
			expectedErrSubstrs: []string{"vault with address", "not found"},
		},
		{
			name:  "swap out below minimum value",
			setup: setup(true, false, "200", ""),
			msg: types.MsgSwapOutRequest{
				Owner:        owner.String(),
				VaultAddress: vaultAddr.String(),
				Assets:       sdk.NewInt64Coin(shareDenom, 50),
			},
			expectedErrSubstrs: []string{"below the minimum required value"},
		},
		{
			name:  "swap out above maximum value",
			setup: setup(true, false, "", "100"),
			msg: types.MsgSwapOutRequest{
				Owner:        owner.String(),
				VaultAddress: vaultAddr.String(),
				Assets:       sdk.NewInt64Coin(shareDenom, 150_000_000),
			},
			expectedErrSubstrs: []string{"above the maximum allowed value"},
		},
		{
			name:  "wrong denom rejected",
			setup: setup(true, false, "", ""),
			msg: types.MsgSwapOutRequest{
				Owner:        owner.String(),
				VaultAddress: vaultAddr.String(),
				Assets:       sdk.NewInt64Coin("wrongdenom", 50),
			},
			expectedErrSubstrs: []string{"swap out denom must be share denom", "wrongdenom", shareDenom},
		},
		{
			name:  "insufficient share balance returns insufficient funds",
			setup: setup(true, false, "", ""),
			msg: types.MsgSwapOutRequest{
				Owner:        owner.String(),
				VaultAddress: vaultAddr.String(),
				Assets:       sdk.NewCoin(shareDenom, overdrawShares),
			},
			expectedErrSubstrs: []string{"failed to escrow shares", "insufficient funds"},
		},
		{
			name:  "swap out enabled, vault is paused",
			setup: setup(true, true, "", ""),
			msg: types.MsgSwapOutRequest{
				Owner:        owner.String(),
				VaultAddress: vaultAddr.String(),
				Assets:       sdk.NewInt64Coin(shareDenom, 150_000_000),
			},
			expectedErrSubstrs: []string{"vault", "is paused"},
		},
		{
			name:  "swap out disabled is rejected",
			setup: setup(false, false, "", ""),
			msg: types.MsgSwapOutRequest{
				Owner:        owner.String(),
				VaultAddress: vaultAddr.String(),
				Assets:       sdk.NewInt64Coin(shareDenom, 150_000_000),
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
			s.Require().NoError(err, "failed to get vault at address %s in post-check", args.VaultAddress)
			s.Assert().Equal(args.ExpectedEnabled, vault.SwapOutEnabled, "swap-out enabled state mismatch for vault %s", args.VaultAddress)
		},
	}

	underlyingDenom := "underlying"
	shareDenom := "vaultshares"
	owner := s.adminAddr
	otherUser := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1000))
	vaultAddr := types.GetVaultAddress(shareDenom)

	setup := func(setAccountInvalid bool) func() {
		return func() {
			s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlyingDenom, math.NewInt(1000)), owner)
			_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
				Admin:           owner.String(),
				ShareDenom:      shareDenom,
				UnderlyingAsset: underlyingDenom,
			})
			s.Require().NoError(err, "failed to create vault in ToggleSwapOut setup for share denom %s", shareDenom)
			if setAccountInvalid {
				vault, err := s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err, "failed to get vault in setup for address %s", vaultAddr)
				vault.PaymentDenom = "!TABLES!"
				s.k.AuthKeeper.SetAccount(s.ctx, vault)
			}
			s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
			s.ctx = s.ctx.WithBlockTime(time.Now())
		}
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
			setup: setup(false),
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
				sdk.NewEvent("provlabs.vault.v1.EventToggleSwapOut",
					sdk.NewAttribute("admin", owner.String()),
					sdk.NewAttribute("enabled", "true"),
					sdk.NewAttribute("vault_address", vaultAddr.String()),
				),
			},
		},
		{
			name: "happy path - disable swap out",
			setup: func() {
				setup(false)()
				vault, err := s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err, "failed to get vault %s for manual swap-out toggle", vaultAddr)
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
				sdk.NewEvent("provlabs.vault.v1.EventToggleSwapOut",
					sdk.NewAttribute("admin", owner.String()),
					sdk.NewAttribute("enabled", "false"),
					sdk.NewAttribute("vault_address", vaultAddr.String()),
				),
			},
		},
		{
			name: "happy path - paused vault toggle swap out",
			setup: func() {
				setup(false)()
				vault, err := s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err, "failed to get vault %s for paused toggle setup", vaultAddr)
				vault.SwapOutEnabled = true
				vault.Paused = true
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
				sdk.NewEvent("provlabs.vault.v1.EventToggleSwapOut",
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
			setup: setup(false),
			msg: types.MsgToggleSwapOutRequest{
				Admin:        otherUser.String(),
				VaultAddress: vaultAddr.String(),
				Enabled:      true,
			},
			expectedErrSubstrs: []string{"unauthorized", otherUser.String(), "is not the vault admin"},
		},
		{
			name:  "failure - invalid vault state during toggle",
			setup: setup(true),
			msg: types.MsgToggleSwapOutRequest{
				Admin:        owner.String(),
				VaultAddress: vaultAddr.String(),
				Enabled:      true,
			},
			expectedErrSubstrs: []string{
				"failed to set swap out enable",
				"invalid payment denom: \"!TABLES!\": invalid denom: !TABLES!",
			},
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

func (s *TestSuite) TestMsgServer_UpdateMinSwapInValue() {
	type postCheckArgs struct {
		VaultAddress sdk.AccAddress
		NewValue     string
		Authority    string
	}

	testDef := msgServerTestDef[types.MsgUpdateMinSwapInValueRequest, types.MsgUpdateMinSwapInValueResponse, postCheckArgs]{
		endpointName: "UpdateMinSwapInValue",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).UpdateMinSwapInValue,
		postCheck: func(msg *types.MsgUpdateMinSwapInValueRequest, args postCheckArgs) {
			vault, err := s.k.GetVault(s.ctx, args.VaultAddress)
			s.Require().NoError(err, "failed to fetch vault after UpdateMinSwapInValue")
			s.Require().Equal(args.NewValue, vault.MinSwapInValue, "vault MinSwapInValue mismatch: expected %s got %s", args.NewValue, vault.MinSwapInValue)
		},
	}

	underlying := "under"
	share := "vaultshares"
	admin := s.adminAddr
	assetManager := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1))
	var vaultAddr sdk.AccAddress

	setup := func() {
		vaultAddr = s.CreateAndActivateVault(admin, share, underlying)
		vault, err := s.k.GetVault(s.ctx, vaultAddr)
		s.Require().NoError(err, "unexpected error fetching vault for asset manager setup at address %s", vaultAddr.String())
		vault.AssetManager = assetManager.String()
		s.k.AuthKeeper.SetAccount(s.ctx, vault)
	}

	tests := []msgServerTestCase[types.MsgUpdateMinSwapInValueRequest, postCheckArgs]{
		{
			name:  "happy path - admin",
			setup: setup,
			msg: types.MsgUpdateMinSwapInValueRequest{
				Authority:      admin.String(),
				VaultAddress:   types.GetVaultAddress(share).String(),
				MinSwapInValue: "500",
			},
			postCheckArgs: postCheckArgs{
				VaultAddress: types.GetVaultAddress(share),
				NewValue:     "500",
				Authority:    admin.String(),
			},
			expectedEvents: sdk.Events{
				sdk.NewEvent("provlabs.vault.v1.EventMinSwapInValueUpdated",
					sdk.NewAttribute("authority", admin.String()),
					sdk.NewAttribute("min_swap_in", "500"),
					sdk.NewAttribute("vault_address", types.GetVaultAddress(share).String()),
				),
			},
		},
		{
			name:  "happy path - asset manager",
			setup: setup,
			msg: types.MsgUpdateMinSwapInValueRequest{
				Authority:      assetManager.String(),
				VaultAddress:   types.GetVaultAddress(share).String(),
				MinSwapInValue: "600",
			},
			postCheckArgs: postCheckArgs{
				VaultAddress: types.GetVaultAddress(share),
				NewValue:     "600",
				Authority:    assetManager.String(),
			},
			expectedEvents: sdk.Events{
				sdk.NewEvent("provlabs.vault.v1.EventMinSwapInValueUpdated",
					sdk.NewAttribute("authority", assetManager.String()),
					sdk.NewAttribute("min_swap_in", "600"),
					sdk.NewAttribute("vault_address", types.GetVaultAddress(share).String()),
				),
			},
		},
	}

	testDef.expectedResponse = &types.MsgUpdateMinSwapInValueResponse{}
	for _, tc := range tests {
		s.Run(tc.name, func() {
			runMsgServerTestCase(s, testDef, tc)
		})
	}
}

func (s *TestSuite) TestMsgServer_UpdateMinSwapInValue_Failures() {
	testDef := msgServerTestDef[types.MsgUpdateMinSwapInValueRequest, types.MsgUpdateMinSwapInValueResponse, any]{
		endpointName: "UpdateMinSwapInValue",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).UpdateMinSwapInValue,
		postCheck:    nil,
	}

	underlying := "under"
	share := "vaultshares"
	admin := s.adminAddr
	other := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1))

	setup := func() {
		_ = s.CreateAndActivateVault(admin, share, underlying)
	}

	tests := []msgServerTestCase[types.MsgUpdateMinSwapInValueRequest, any]{
		{
			name: "vault not found",
			msg: types.MsgUpdateMinSwapInValueRequest{
				Authority:      admin.String(),
				VaultAddress:   types.GetVaultAddress("missing").String(),
				MinSwapInValue: "100",
			},
			expectedErrSubstrs: []string{"vault not found"},
		},
		{
			name:  "unauthorized",
			setup: setup,
			msg: types.MsgUpdateMinSwapInValueRequest{
				Authority:      other.String(),
				VaultAddress:   types.GetVaultAddress(share).String(),
				MinSwapInValue: "100",
			},
			expectedErrSubstrs: []string{"unauthorized"},
		},
		{
			name:  "invalid value",
			setup: setup,
			msg: types.MsgUpdateMinSwapInValueRequest{
				Authority:      admin.String(),
				VaultAddress:   types.GetVaultAddress(share).String(),
				MinSwapInValue: "invalid",
			},
			expectedErrSubstrs: []string{"failed to set MinSwapIn", "invalid min value"},
		},
		{
			name:  "negative value",
			setup: setup,
			msg: types.MsgUpdateMinSwapInValueRequest{
				Authority:      admin.String(),
				VaultAddress:   types.GetVaultAddress(share).String(),
				MinSwapInValue: "-1",
			},
			expectedErrSubstrs: []string{"failed to set MinSwapIn", "must be non-negative"},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			runMsgServerTestCase(s, testDef, tc)
		})
	}
}

func (s *TestSuite) TestMsgServer_UpdateMinSwapOutValue() {
	type postCheckArgs struct {
		VaultAddress sdk.AccAddress
		NewValue     string
		Authority    string
	}

	testDef := msgServerTestDef[types.MsgUpdateMinSwapOutValueRequest, types.MsgUpdateMinSwapOutValueResponse, postCheckArgs]{
		endpointName: "UpdateMinSwapOutValue",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).UpdateMinSwapOutValue,
		postCheck: func(msg *types.MsgUpdateMinSwapOutValueRequest, args postCheckArgs) {
			vault, err := s.k.GetVault(s.ctx, args.VaultAddress)
			s.Require().NoError(err, "failed to fetch vault after UpdateMinSwapOutValue")
			s.Require().Equal(args.NewValue, vault.MinSwapOutValue, "vault MinSwapOutValue mismatch: expected %s got %s", args.NewValue, vault.MinSwapOutValue)
		},
	}

	underlying := "under"
	share := "vaultshares"
	admin := s.adminAddr
	assetManager := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1))
	var vaultAddr sdk.AccAddress

	setup := func() {
		vaultAddr = s.CreateAndActivateVault(admin, share, underlying)
		vault, err := s.k.GetVault(s.ctx, vaultAddr)
		s.Require().NoError(err, "unexpected error fetching vault for asset manager setup at address %s", vaultAddr.String())
		vault.AssetManager = assetManager.String()
		s.k.AuthKeeper.SetAccount(s.ctx, vault)
	}

	tests := []msgServerTestCase[types.MsgUpdateMinSwapOutValueRequest, postCheckArgs]{
		{
			name:  "happy path - admin",
			setup: setup,
			msg: types.MsgUpdateMinSwapOutValueRequest{
				Authority:       admin.String(),
				VaultAddress:    types.GetVaultAddress(share).String(),
				MinSwapOutValue: "300",
			},
			postCheckArgs: postCheckArgs{
				VaultAddress: types.GetVaultAddress(share),
				NewValue:     "300",
				Authority:    admin.String(),
			},
			expectedEvents: sdk.Events{
				sdk.NewEvent("provlabs.vault.v1.EventMinSwapOutValueUpdated",
					sdk.NewAttribute("authority", admin.String()),
					sdk.NewAttribute("min_swap_out", "300"),
					sdk.NewAttribute("vault_address", types.GetVaultAddress(share).String()),
				),
			},
		},
		{
			name:  "happy path - asset manager",
			setup: setup,
			msg: types.MsgUpdateMinSwapOutValueRequest{
				Authority:       assetManager.String(),
				VaultAddress:    types.GetVaultAddress(share).String(),
				MinSwapOutValue: "400",
			},
			postCheckArgs: postCheckArgs{
				VaultAddress: types.GetVaultAddress(share),
				NewValue:     "400",
				Authority:    assetManager.String(),
			},
			expectedEvents: sdk.Events{
				sdk.NewEvent("provlabs.vault.v1.EventMinSwapOutValueUpdated",
					sdk.NewAttribute("authority", assetManager.String()),
					sdk.NewAttribute("min_swap_out", "400"),
					sdk.NewAttribute("vault_address", types.GetVaultAddress(share).String()),
				),
			},
		},
	}

	testDef.expectedResponse = &types.MsgUpdateMinSwapOutValueResponse{}
	for _, tc := range tests {
		s.Run(tc.name, func() {
			runMsgServerTestCase(s, testDef, tc)
		})
	}
}

func (s *TestSuite) TestMsgServer_UpdateMinSwapOutValue_Failures() {
	testDef := msgServerTestDef[types.MsgUpdateMinSwapOutValueRequest, types.MsgUpdateMinSwapOutValueResponse, any]{
		endpointName: "UpdateMinSwapOutValue",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).UpdateMinSwapOutValue,
		postCheck:    nil,
	}

	underlying := "under"
	share := "vaultshares"
	admin := s.adminAddr
	other := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1))

	setup := func() {
		_ = s.CreateAndActivateVault(admin, share, underlying)
	}

	tests := []msgServerTestCase[types.MsgUpdateMinSwapOutValueRequest, any]{
		{
			name: "vault not found",
			msg: types.MsgUpdateMinSwapOutValueRequest{
				Authority:       admin.String(),
				VaultAddress:    types.GetVaultAddress("missing").String(),
				MinSwapOutValue: "100",
			},
			expectedErrSubstrs: []string{"vault not found"},
		},
		{
			name:  "unauthorized",
			setup: setup,
			msg: types.MsgUpdateMinSwapOutValueRequest{
				Authority:       other.String(),
				VaultAddress:    types.GetVaultAddress(share).String(),
				MinSwapOutValue: "100",
			},
			expectedErrSubstrs: []string{"unauthorized"},
		},
		{
			name:  "invalid value",
			setup: setup,
			msg: types.MsgUpdateMinSwapOutValueRequest{
				Authority:       admin.String(),
				VaultAddress:    types.GetVaultAddress(share).String(),
				MinSwapOutValue: "invalid",
			},
			expectedErrSubstrs: []string{"failed to set MinSwapOut", "invalid min value"},
		},
		{
			name:  "negative value",
			setup: setup,
			msg: types.MsgUpdateMinSwapOutValueRequest{
				Authority:       admin.String(),
				VaultAddress:    types.GetVaultAddress(share).String(),
				MinSwapOutValue: "-1",
			},
			expectedErrSubstrs: []string{"failed to set MinSwapOut", "must be non-negative"},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			runMsgServerTestCase(s, testDef, tc)
		})
	}
}

func (s *TestSuite) TestMsgServer_UpdateMaxSwapInValue() {
	type postCheckArgs struct {
		VaultAddress sdk.AccAddress
		NewValue     string
		Authority    string
	}

	testDef := msgServerTestDef[types.MsgUpdateMaxSwapInValueRequest, types.MsgUpdateMaxSwapInValueResponse, postCheckArgs]{
		endpointName: "UpdateMaxSwapInValue",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).UpdateMaxSwapInValue,
		postCheck: func(msg *types.MsgUpdateMaxSwapInValueRequest, args postCheckArgs) {
			vault, err := s.k.GetVault(s.ctx, args.VaultAddress)
			s.Require().NoError(err, "failed to fetch vault after UpdateMaxSwapInValue")
			s.Require().Equal(args.NewValue, vault.MaxSwapInValue, "vault MaxSwapInValue mismatch: expected %s got %s", args.NewValue, vault.MaxSwapInValue)
		},
	}

	underlying := "under"
	share := "vaultshares"
	admin := s.adminAddr
	assetManager := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1))
	var vaultAddr sdk.AccAddress

	setup := func() {
		vaultAddr = s.CreateAndActivateVault(admin, share, underlying)
		vault, err := s.k.GetVault(s.ctx, vaultAddr)
		s.Require().NoError(err, "unexpected error fetching vault for asset manager setup at address %s", vaultAddr.String())
		vault.AssetManager = assetManager.String()
		s.k.AuthKeeper.SetAccount(s.ctx, vault)
	}

	tests := []msgServerTestCase[types.MsgUpdateMaxSwapInValueRequest, postCheckArgs]{
		{
			name:  "happy path - admin",
			setup: setup,
			msg: types.MsgUpdateMaxSwapInValueRequest{
				Authority:      admin.String(),
				VaultAddress:   types.GetVaultAddress(share).String(),
				MaxSwapInValue: "5000",
			},
			postCheckArgs: postCheckArgs{
				VaultAddress: types.GetVaultAddress(share),
				NewValue:     "5000",
				Authority:    admin.String(),
			},
			expectedEvents: sdk.Events{
				sdk.NewEvent("provlabs.vault.v1.EventMaxSwapInValueUpdated",
					sdk.NewAttribute("authority", admin.String()),
					sdk.NewAttribute("max_swap_in", "5000"),
					sdk.NewAttribute("vault_address", types.GetVaultAddress(share).String()),
				),
			},
		},
		{
			name:  "happy path - asset manager",
			setup: setup,
			msg: types.MsgUpdateMaxSwapInValueRequest{
				Authority:      assetManager.String(),
				VaultAddress:   types.GetVaultAddress(share).String(),
				MaxSwapInValue: "6000",
			},
			postCheckArgs: postCheckArgs{
				VaultAddress: types.GetVaultAddress(share),
				NewValue:     "6000",
				Authority:    assetManager.String(),
			},
			expectedEvents: sdk.Events{
				sdk.NewEvent("provlabs.vault.v1.EventMaxSwapInValueUpdated",
					sdk.NewAttribute("authority", assetManager.String()),
					sdk.NewAttribute("max_swap_in", "6000"),
					sdk.NewAttribute("vault_address", types.GetVaultAddress(share).String()),
				),
			},
		},
	}

	testDef.expectedResponse = &types.MsgUpdateMaxSwapInValueResponse{}
	for _, tc := range tests {
		s.Run(tc.name, func() {
			runMsgServerTestCase(s, testDef, tc)
		})
	}
}

func (s *TestSuite) TestMsgServer_UpdateMaxSwapInValue_Failures() {
	testDef := msgServerTestDef[types.MsgUpdateMaxSwapInValueRequest, types.MsgUpdateMaxSwapInValueResponse, any]{
		endpointName: "UpdateMaxSwapInValue",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).UpdateMaxSwapInValue,
		postCheck:    nil,
	}

	underlying := "under"
	share := "vaultshares"
	admin := s.adminAddr
	other := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1))

	setup := func() {
		_ = s.CreateAndActivateVault(admin, share, underlying)
	}

	tests := []msgServerTestCase[types.MsgUpdateMaxSwapInValueRequest, any]{
		{
			name: "vault not found",
			msg: types.MsgUpdateMaxSwapInValueRequest{
				Authority:      admin.String(),
				VaultAddress:   types.GetVaultAddress("missing").String(),
				MaxSwapInValue: "100",
			},
			expectedErrSubstrs: []string{"vault not found"},
		},
		{
			name:  "unauthorized",
			setup: setup,
			msg: types.MsgUpdateMaxSwapInValueRequest{
				Authority:      other.String(),
				VaultAddress:   types.GetVaultAddress(share).String(),
				MaxSwapInValue: "100",
			},
			expectedErrSubstrs: []string{"unauthorized"},
		},
		{
			name:  "invalid value",
			setup: setup,
			msg: types.MsgUpdateMaxSwapInValueRequest{
				Authority:      admin.String(),
				VaultAddress:   types.GetVaultAddress(share).String(),
				MaxSwapInValue: "invalid",
			},
			expectedErrSubstrs: []string{"failed to set MaxSwapIn", "invalid max value"},
		},
		{
			name:  "negative value",
			setup: setup,
			msg: types.MsgUpdateMaxSwapInValueRequest{
				Authority:      admin.String(),
				VaultAddress:   types.GetVaultAddress(share).String(),
				MaxSwapInValue: "-1",
			},
			expectedErrSubstrs: []string{"failed to set MaxSwapIn", "must be non-negative"},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			runMsgServerTestCase(s, testDef, tc)
		})
	}
}

func (s *TestSuite) TestMsgServer_UpdateMaxSwapOutValue() {
	type postCheckArgs struct {
		VaultAddress sdk.AccAddress
		NewValue     string
		Authority    string
	}

	testDef := msgServerTestDef[types.MsgUpdateMaxSwapOutValueRequest, types.MsgUpdateMaxSwapOutValueResponse, postCheckArgs]{
		endpointName: "UpdateMaxSwapOutValue",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).UpdateMaxSwapOutValue,
		postCheck: func(msg *types.MsgUpdateMaxSwapOutValueRequest, args postCheckArgs) {
			vault, err := s.k.GetVault(s.ctx, args.VaultAddress)
			s.Require().NoError(err, "failed to fetch vault after UpdateMaxSwapOutValue")
			s.Require().Equal(args.NewValue, vault.MaxSwapOutValue, "vault MaxSwapOutValue mismatch: expected %s got %s", args.NewValue, vault.MaxSwapOutValue)
		},
	}

	underlying := "under"
	share := "vaultshares"
	admin := s.adminAddr
	assetManager := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1))
	var vaultAddr sdk.AccAddress

	setup := func() {
		vaultAddr = s.CreateAndActivateVault(admin, share, underlying)
		vault, err := s.k.GetVault(s.ctx, vaultAddr)
		s.Require().NoError(err, "unexpected error fetching vault for asset manager setup at address %s", vaultAddr.String())
		vault.AssetManager = assetManager.String()
		s.k.AuthKeeper.SetAccount(s.ctx, vault)
	}

	tests := []msgServerTestCase[types.MsgUpdateMaxSwapOutValueRequest, postCheckArgs]{
		{
			name:  "happy path - admin",
			setup: setup,
			msg: types.MsgUpdateMaxSwapOutValueRequest{
				Authority:       admin.String(),
				VaultAddress:    types.GetVaultAddress(share).String(),
				MaxSwapOutValue: "3000",
			},
			postCheckArgs: postCheckArgs{
				VaultAddress: types.GetVaultAddress(share),
				NewValue:     "3000",
				Authority:    admin.String(),
			},
			expectedEvents: sdk.Events{
				sdk.NewEvent("provlabs.vault.v1.EventMaxSwapOutValueUpdated",
					sdk.NewAttribute("authority", admin.String()),
					sdk.NewAttribute("max_swap_out", "3000"),
					sdk.NewAttribute("vault_address", types.GetVaultAddress(share).String()),
				),
			},
		},
		{
			name:  "happy path - asset manager",
			setup: setup,
			msg: types.MsgUpdateMaxSwapOutValueRequest{
				Authority:       assetManager.String(),
				VaultAddress:    types.GetVaultAddress(share).String(),
				MaxSwapOutValue: "4000",
			},
			postCheckArgs: postCheckArgs{
				VaultAddress: types.GetVaultAddress(share),
				NewValue:     "4000",
				Authority:    assetManager.String(),
			},
			expectedEvents: sdk.Events{
				sdk.NewEvent("provlabs.vault.v1.EventMaxSwapOutValueUpdated",
					sdk.NewAttribute("authority", assetManager.String()),
					sdk.NewAttribute("max_swap_out", "4000"),
					sdk.NewAttribute("vault_address", types.GetVaultAddress(share).String()),
				),
			},
		},
	}

	testDef.expectedResponse = &types.MsgUpdateMaxSwapOutValueResponse{}
	for _, tc := range tests {
		s.Run(tc.name, func() {
			runMsgServerTestCase(s, testDef, tc)
		})
	}
}

func (s *TestSuite) TestMsgServer_UpdateMaxSwapOutValue_Failures() {
	testDef := msgServerTestDef[types.MsgUpdateMaxSwapOutValueRequest, types.MsgUpdateMaxSwapOutValueResponse, any]{
		endpointName: "UpdateMaxSwapOutValue",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).UpdateMaxSwapOutValue,
		postCheck:    nil,
	}

	underlying := "under"
	share := "vaultshares"
	admin := s.adminAddr
	other := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1))

	setup := func() {
		_ = s.CreateAndActivateVault(admin, share, underlying)
	}

	tests := []msgServerTestCase[types.MsgUpdateMaxSwapOutValueRequest, any]{
		{
			name: "vault not found",
			msg: types.MsgUpdateMaxSwapOutValueRequest{
				Authority:       admin.String(),
				VaultAddress:    types.GetVaultAddress("missing").String(),
				MaxSwapOutValue: "100",
			},
			expectedErrSubstrs: []string{"vault not found"},
		},
		{
			name:  "unauthorized",
			setup: setup,
			msg: types.MsgUpdateMaxSwapOutValueRequest{
				Authority:       other.String(),
				VaultAddress:    types.GetVaultAddress(share).String(),
				MaxSwapOutValue: "100",
			},
			expectedErrSubstrs: []string{"unauthorized"},
		},
		{
			name:  "invalid value",
			setup: setup,
			msg: types.MsgUpdateMaxSwapOutValueRequest{
				Authority:       admin.String(),
				VaultAddress:    types.GetVaultAddress(share).String(),
				MaxSwapOutValue: "invalid",
			},
			expectedErrSubstrs: []string{"failed to set MaxSwapOut", "invalid max value"},
		},
		{
			name:  "negative value",
			setup: setup,
			msg: types.MsgUpdateMaxSwapOutValueRequest{
				Authority:       admin.String(),
				VaultAddress:    types.GetVaultAddress(share).String(),
				MaxSwapOutValue: "-1",
			},
			expectedErrSubstrs: []string{"failed to set MaxSwapOut", "must be non-negative"},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
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
			s.Require().NoError(err, "failed to get vault at address %s in post-check", args.VaultAddress)
			s.Assert().Equal(args.ExpectedEnabled, vault.SwapInEnabled, "swap-in enabled state mismatch for vault %s", args.VaultAddress)
		},
	}

	underlyingDenom := "underlying"
	shareDenom := "vaultshares"
	owner := s.adminAddr
	otherUser := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1000))
	vaultAddr := types.GetVaultAddress(shareDenom)

	setup := func(setAccountInvalid bool) func() {
		return func() {
			s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlyingDenom, math.NewInt(1000)), owner)
			_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
				Admin:           owner.String(),
				ShareDenom:      shareDenom,
				UnderlyingAsset: underlyingDenom,
			})
			s.Require().NoError(err, "failed to create vault in ToggleSwapIn setup for share denom %s", shareDenom)
			if setAccountInvalid {
				vault, err := s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err, "failed to get vault in setup for address %s", vaultAddr)
				vault.PaymentDenom = "!TABLES!"
				s.k.AuthKeeper.SetAccount(s.ctx, vault)
			}
			s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
		}
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
			setup: setup(false),
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
				sdk.NewEvent("provlabs.vault.v1.EventToggleSwapIn",
					sdk.NewAttribute("admin", owner.String()),
					sdk.NewAttribute("enabled", "true"),
					sdk.NewAttribute("vault_address", vaultAddr.String()),
				),
			},
		},
		{
			name: "happy path - disable swap in",
			setup: func() {
				setup(false)()
				vault, err := s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err, "failed to get vault %s", vaultAddr)
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
				sdk.NewEvent("provlabs.vault.v1.EventToggleSwapIn",
					sdk.NewAttribute("admin", owner.String()),
					sdk.NewAttribute("enabled", "false"),
					sdk.NewAttribute("vault_address", vaultAddr.String()),
				),
			},
		},
		{
			name: "happy path - vault paused toggle swap in",
			setup: func() {
				setup(false)()
				vault, err := s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err, "failed to get vault %s", vaultAddr)
				vault.SwapInEnabled = true
				vault.Paused = true
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
				sdk.NewEvent("provlabs.vault.v1.EventToggleSwapIn",
					sdk.NewAttribute("admin", owner.String()),
					sdk.NewAttribute("enabled", "false"),
					sdk.NewAttribute("vault_address", vaultAddr.String()),
				),
			},
		},
		{
			name:  "failure - vault not found",
			setup: func() {},
			msg: types.MsgToggleSwapInRequest{
				Admin:        owner.String(),
				VaultAddress: vaultAddr.String(),
				Enabled:      true,
			},
			expectedErrSubstrs: []string{"vault not found", vaultAddr.String()},
		},
		{
			name:  "failure - unauthorized admin",
			setup: setup(false),
			msg: types.MsgToggleSwapInRequest{
				Admin:        otherUser.String(),
				VaultAddress: vaultAddr.String(),
				Enabled:      true,
			},
			expectedErrSubstrs: []string{"unauthorized", otherUser.String(), "is not the vault admin"},
		},
		{
			name:  "failure - invalid vault state during toggle",
			setup: setup(true),
			msg: types.MsgToggleSwapInRequest{
				Admin:        owner.String(),
				VaultAddress: vaultAddr.String(),
				Enabled:      true,
			},
			expectedErrSubstrs: []string{
				"failed to set swap in enable",
				"invalid payment denom: \"!TABLES!\": invalid denom: !TABLES!",
			},
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
		VaultAddress              sdk.AccAddress
		ExpectedRate              string
		ExpectedPeriodStart       int64
		ExpectInVerificationQueue bool
	}

	testDef := msgServerTestDef[types.MsgUpdateInterestRateRequest, types.MsgUpdateInterestRateResponse, postCheckArgs]{
		endpointName: "UpdateInterestRate",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).UpdateInterestRate,
		postCheck: func(msg *types.MsgUpdateInterestRateRequest, args postCheckArgs) {
			vault, err := s.k.GetVault(s.ctx, args.VaultAddress)
			s.Require().NoError(err, "should be able to get vault")
			s.Assert().Equal(args.ExpectedRate, vault.CurrentInterestRate, "vault current interest rate should match expected rate")
			s.Assert().Equal(args.ExpectedRate, vault.DesiredInterestRate, "vault desired interest rate should match expected rate")
			if args.ExpectedPeriodStart > 0 {
				s.Assert().Equal(args.ExpectedPeriodStart, vault.PeriodStart, "unexpected period start")
			}
			s.assertInPayoutVerificationQueue(vault.GetAddress(), args.ExpectInVerificationQueue)
		},
	}

	underlyingDenom := "underlying"
	shareDenom := "vaultshares"
	owner := s.adminAddr
	assetMgr := sdk.AccAddress("assetManagerAddr___________")
	vaultAddr := types.GetVaultAddress(shareDenom)
	currentBlockTime := time.Now()

	setup := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlyingDenom, math.NewInt(1000)), owner)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           owner.String(),
			ShareDenom:      shareDenom,
			UnderlyingAsset: underlyingDenom,
		})
		s.Require().NoError(err, "failed to create vault for share denom %s", shareDenom)
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
		s.ctx = s.ctx.WithBlockTime(currentBlockTime)
	}

	tests := []struct {
		name           string
		interestRate   string
		authority      string
		setup          func()
		postCheckArgs  postCheckArgs
		expectedEvents sdk.Events
	}{
		{
			name:         "enable interest from zero to non-zero (disabled → enabled)",
			interestRate: "0.05",
			setup:        setup,
			postCheckArgs: postCheckArgs{
				VaultAddress:              vaultAddr,
				ExpectedRate:              "0.05",
				ExpectInVerificationQueue: true,
				ExpectedPeriodStart:       currentBlockTime.Unix(),
			},
			expectedEvents: sdk.Events{
				sdk.NewEvent("provlabs.vault.v1.EventVaultInterestChange",
					sdk.NewAttribute("current_rate", "0.05"),
					sdk.NewAttribute("desired_rate", "0.05"),
					sdk.NewAttribute("vault_address", vaultAddr.String()),
				),
			},
		},
		{
			name:         "change non-zero rate with reconcile (enabled → enabled)",
			interestRate: "4.06",
			setup: func() {
				setup()
				vaultAcc, err := s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err, "failed to get vault %s", vaultAddr)
				s.Require().NoError(s.k.UpdateInterestRates(s.ctx, vaultAcc, "4.20", "4.20"), "initial interest rate update should succeed")
				vaultAcc.PeriodStart = currentBlockTime.Unix() - 10000
				s.Require().NoError(s.k.SetVaultAccount(s.ctx, vaultAcc), "setting vault account should succeed")
			},
			postCheckArgs: postCheckArgs{
				VaultAddress:              vaultAddr,
				ExpectedRate:              "4.06",
				ExpectInVerificationQueue: true,
				ExpectedPeriodStart:       currentBlockTime.Unix(),
			},
			expectedEvents: sdk.Events{
				sdk.NewEvent("provlabs.vault.v1.EventVaultReconcile",
					sdk.NewAttribute("interest_earned", sdk.NewInt64Coin(underlyingDenom, 0).String()),
					sdk.NewAttribute("principal_after", sdk.NewInt64Coin(underlyingDenom, 0).String()),
					sdk.NewAttribute("principal_before", sdk.NewInt64Coin(underlyingDenom, 0).String()),
					sdk.NewAttribute("rate", "4.20"),
					sdk.NewAttribute("time", "10000"),
					sdk.NewAttribute("vault_address", vaultAddr.String()),
				),
				sdk.NewEvent("provlabs.vault.v1.EventVaultInterestChange",
					sdk.NewAttribute("current_rate", "4.06"),
					sdk.NewAttribute("desired_rate", "4.06"),
					sdk.NewAttribute("vault_address", vaultAddr.String()),
				),
			},
		},
		{
			name:         "disable interest by setting zero (enabled → disabled)",
			interestRate: "0",
			setup: func() {
				setup()
				vaultAcc, err := s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err, "failed to get vault %s", vaultAddr)
				s.Require().NoError(s.k.UpdateInterestRates(s.ctx, vaultAcc, "6.12", "6.12"), "initial interest rate update should succeed")
				vaultAcc.PeriodStart = currentBlockTime.Unix() - 10000
				s.Require().NoError(s.k.SetVaultAccount(s.ctx, vaultAcc), "setting vault account should succeed")
			},
			postCheckArgs: postCheckArgs{
				VaultAddress:              vaultAddr,
				ExpectedRate:              types.ZeroInterestRate,
				ExpectInVerificationQueue: false,
			},
			expectedEvents: sdk.Events{
				sdk.NewEvent("provlabs.vault.v1.EventVaultReconcile",
					sdk.NewAttribute("interest_earned", sdk.NewInt64Coin(underlyingDenom, 0).String()),
					sdk.NewAttribute("principal_after", sdk.NewInt64Coin(underlyingDenom, 0).String()),
					sdk.NewAttribute("principal_before", sdk.NewInt64Coin(underlyingDenom, 0).String()),
					sdk.NewAttribute("rate", "6.12"),
					sdk.NewAttribute("time", "10000"),
					sdk.NewAttribute("vault_address", vaultAddr.String()),
				),
				sdk.NewEvent("provlabs.vault.v1.EventVaultInterestChange",
					sdk.NewAttribute("current_rate", "0.0"),
					sdk.NewAttribute("desired_rate", "0.0"),
					sdk.NewAttribute("vault_address", vaultAddr.String()),
				),
			},
		},
		{
			name:         "no-op update with same non-zero rate (enabled → enabled unchanged)",
			interestRate: "3.33",
			setup: func() {
				setup()
				vaultAcc, err := s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err, "failed to get vault %s", vaultAddr)
				s.Require().NoError(s.k.UpdateInterestRates(s.ctx, vaultAcc, "3.33", "3.33"), "initial interest rate update should succeed")
				vaultAcc.PeriodStart = currentBlockTime.Unix() - 5000
				s.Require().NoError(s.k.SetVaultAccount(s.ctx, vaultAcc), "setting vault account should succeed")
			},
			postCheckArgs: postCheckArgs{
				VaultAddress:              vaultAddr,
				ExpectedRate:              "3.33",
				ExpectInVerificationQueue: true,
				ExpectedPeriodStart:       currentBlockTime.Unix(),
			},
			expectedEvents: sdk.Events{
				sdk.NewEvent("provlabs.vault.v1.EventVaultReconcile",
					sdk.NewAttribute("interest_earned", sdk.NewInt64Coin(underlyingDenom, 0).String()),
					sdk.NewAttribute("principal_after", sdk.NewInt64Coin(underlyingDenom, 0).String()),
					sdk.NewAttribute("principal_before", sdk.NewInt64Coin(underlyingDenom, 0).String()),
					sdk.NewAttribute("rate", "3.33"),
					sdk.NewAttribute("time", "5000"),
					sdk.NewAttribute("vault_address", vaultAddr.String()),
				),
				sdk.NewEvent("provlabs.vault.v1.EventVaultInterestChange",
					sdk.NewAttribute("current_rate", "3.33"),
					sdk.NewAttribute("desired_rate", "3.33"),
					sdk.NewAttribute("vault_address", vaultAddr.String()),
				),
			},
		},
		{
			name:         "no-op update with zero rate (disabled → disabled unchanged)",
			interestRate: "0.0",
			setup:        setup,
			postCheckArgs: postCheckArgs{
				VaultAddress:              vaultAddr,
				ExpectedRate:              types.ZeroInterestRate,
				ExpectInVerificationQueue: false,
			},
			expectedEvents: sdk.Events{
				sdk.NewEvent("provlabs.vault.v1.EventVaultInterestChange",
					sdk.NewAttribute("current_rate", "0.0"),
					sdk.NewAttribute("desired_rate", "0.0"),
					sdk.NewAttribute("vault_address", vaultAddr.String()),
				),
			},
		},
		{
			name:         "re-enable after prior disable with stale period fields present (disabled → enabled)",
			interestRate: "1.25",
			setup: func() {
				setup()
				vaultAcc, err := s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err, "failed to get vault %s", vaultAddr)
				s.Require().NoError(s.k.UpdateInterestRates(s.ctx, vaultAcc, "0.0", "0.0"), "initial interest rate update should succeed")
				vaultAcc.PeriodStart = currentBlockTime.Unix() - 1234
				vaultAcc.PeriodTimeout = currentBlockTime.Unix() + 9999
				s.Require().NoError(s.k.SetVaultAccount(s.ctx, vaultAcc), "setting vault account should succeed")
			},
			postCheckArgs: postCheckArgs{
				VaultAddress:              vaultAddr,
				ExpectedRate:              "1.25",
				ExpectInVerificationQueue: true,
				ExpectedPeriodStart:       currentBlockTime.Unix(),
			},
			expectedEvents: sdk.Events{
				sdk.NewEvent("provlabs.vault.v1.EventVaultInterestChange",
					sdk.NewAttribute("current_rate", "1.25"),
					sdk.NewAttribute("desired_rate", "1.25"),
					sdk.NewAttribute("vault_address", vaultAddr.String()),
				),
			},
		},
		{
			name:         "paused: change non-zero rate (enabled → enabled) skips reconcile and keeps period start unchanged",
			interestRate: "4.06",
			setup: func() {
				setup()
				vaultAcc, err := s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err, "failed to get vault %s", vaultAddr)
				s.Require().NoError(s.k.UpdateInterestRates(s.ctx, vaultAcc, "4.20", "4.20"), "initial interest rate update should succeed")
				priorStart := currentBlockTime.Unix() - 7777
				vaultAcc.PeriodStart = priorStart
				vaultAcc.Paused = true
				s.Require().NoError(s.k.SetVaultAccount(s.ctx, vaultAcc), "setting vault account should succeed")
				s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
			},
			postCheckArgs: postCheckArgs{
				VaultAddress:              vaultAddr,
				ExpectedRate:              "4.06",
				ExpectInVerificationQueue: false,
				ExpectedPeriodStart:       currentBlockTime.Unix() - 7777,
			},
			expectedEvents: sdk.Events{
				sdk.NewEvent("provlabs.vault.v1.EventVaultInterestChange",
					sdk.NewAttribute("current_rate", "4.06"),
					sdk.NewAttribute("desired_rate", "4.06"),
					sdk.NewAttribute("vault_address", vaultAddr.String()),
				),
			},
		},
		{
			name:         "paused: enable from zero (disabled → enabled) enqueues verification, no reconcile",
			interestRate: "1.25",
			setup: func() {
				setup()
				vaultAcc, err := s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err, "failed to get vault %s", vaultAddr)
				s.Require().NoError(s.k.UpdateInterestRates(s.ctx, vaultAcc, "0.0", "0.0"), "initial interest rate update should succeed")
				vaultAcc.Paused = true
				s.Require().NoError(s.k.SetVaultAccount(s.ctx, vaultAcc), "setting vault account should succeed")
				s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
			},
			postCheckArgs: postCheckArgs{
				VaultAddress:              vaultAddr,
				ExpectedRate:              "1.25",
				ExpectInVerificationQueue: true,
			},
			expectedEvents: sdk.Events{
				sdk.NewEvent("provlabs.vault.v1.EventVaultInterestChange",
					sdk.NewAttribute("current_rate", "1.25"),
					sdk.NewAttribute("desired_rate", "1.25"),
					sdk.NewAttribute("vault_address", vaultAddr.String()),
				),
			},
		},
		{
			name:         "paused: disable interest (enabled → disabled) clears period fields and removes verification",
			interestRate: "0.0",
			setup: func() {
				setup()
				vaultAcc, err := s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err, "failed to get vault %s", vaultAddr)
				s.Require().NoError(s.k.UpdateInterestRates(s.ctx, vaultAcc, "2.50", "2.50"), "initial interest rate update should succeed")
				vaultAcc.PeriodStart = currentBlockTime.Unix() - 5000
				vaultAcc.PeriodTimeout = currentBlockTime.Unix() + 3600
				vaultAcc.Paused = true
				s.Require().NoError(s.k.SetVaultAccount(s.ctx, vaultAcc), "setting vault account should succeed")
				s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
			},
			postCheckArgs: postCheckArgs{
				VaultAddress:              vaultAddr,
				ExpectedRate:              types.ZeroInterestRate,
				ExpectInVerificationQueue: false,
			},
			expectedEvents: sdk.Events{
				sdk.NewEvent("provlabs.vault.v1.EventVaultInterestChange",
					sdk.NewAttribute("current_rate", "0.0"),
					sdk.NewAttribute("desired_rate", "0.0"),
					sdk.NewAttribute("vault_address", vaultAddr.String()),
				),
			},
		},
		{
			name:         "asset manager can update non-zero rate",
			interestRate: "0.75",
			authority:    assetMgr.String(),
			setup: func() {
				setup()
				v, err := s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err, "failed to get vault %s", vaultAddr)
				v.AssetManager = assetMgr.String()
				s.Require().NoError(s.k.SetVaultAccount(s.ctx, v), "failed to set vault account with asset manager %s", assetMgr)
			},
			postCheckArgs: postCheckArgs{
				VaultAddress:              vaultAddr,
				ExpectedRate:              "0.75",
				ExpectInVerificationQueue: true,
				ExpectedPeriodStart:       currentBlockTime.Unix(),
			},
			expectedEvents: sdk.Events{
				sdk.NewEvent("provlabs.vault.v1.EventVaultInterestChange",
					sdk.NewAttribute("current_rate", "0.75"),
					sdk.NewAttribute("desired_rate", "0.75"),
					sdk.NewAttribute("vault_address", vaultAddr.String()),
				),
			},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			updateInterestRateReq := types.MsgUpdateInterestRateRequest{
				Authority:    owner.String(),
				VaultAddress: vaultAddr.String(),
				NewRate:      tt.interestRate,
			}
			if tt.authority != "" {
				updateInterestRateReq.Authority = tt.authority
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
		s.Require().NoError(err, "failed to create vault for share denom %s", shareDenom)
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
		s.ctx = s.ctx.WithBlockTime(currentBlockTime)
	}

	tests := []msgServerTestCase[types.MsgUpdateInterestRateRequest, any]{
		{
			name: "vault does not exist",
			msg: types.MsgUpdateInterestRateRequest{
				Authority:    owner.String(),
				VaultAddress: types.GetVaultAddress("invalidvaultaddress").String(),
				NewRate:      "0.05",
			},
			setup:              setup,
			expectedErrSubstrs: []string{"not found"},
		},
		{
			name: "vault invalid vault address",
			msg: types.MsgUpdateInterestRateRequest{
				Authority:    owner.String(),
				VaultAddress: markertypes.MustGetMarkerAddress(shareDenom).String(),
				NewRate:      "0.05",
			},
			setup:              setup,
			expectedErrSubstrs: []string{"failed to get vault", "is not a vault account"},
		},
		{
			name: "unauthorized authority",
			msg: types.MsgUpdateInterestRateRequest{
				Authority:    sdk.AccAddress("bad").String(),
				VaultAddress: vaultAddr.String(),
				NewRate:      "0.05",
			},
			setup:              setup,
			expectedErrSubstrs: []string{"unauthorized authority"},
		},
		{
			name: "rate above absolute ceiling is rejected",
			msg: types.MsgUpdateInterestRateRequest{
				Authority:    owner.String(),
				VaultAddress: vaultAddr.String(),
				NewRate:      "150.0",
			},
			setup:              setup,
			expectedErrSubstrs: []string{"invalid new rate", "exceeds maximum allowed magnitude"},
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
			s.Require().NoError(err, "post-check: failed to get vault %s", args.VaultAddress)
			s.Assert().Equal(args.ExpectedMin, v.MinInterestRate, "post-check: min interest rate mismatch")
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
		s.Require().NoError(err, "failed to create vault for share denom %s", share)
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
			minRate: "-0.05",
			postCheckArgs: postCheckArgs{
				VaultAddress: vaultAddr,
				ExpectedMin:  "-0.05",
			},
			expectedEvents: sdk.Events{
				sdk.NewEvent("provlabs.vault.v1.EventMinInterestRateUpdated",
					sdk.NewAttribute("admin", admin.String()),
					sdk.NewAttribute("min_rate", "-0.05"),
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
			expectedEvents: sdk.Events{},
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
		s.Require().NoError(err, "failed to create vault for share denom %s", share)
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
				s.Require().NoError(err, "failed to get vault %s", vaultAddr)
				s.Require().NoError(s.k.SetMaxInterestRate(s.ctx, v, "0.05"), "failed to set max interest rate")
			},
			msg: types.MsgUpdateMinInterestRateRequest{
				Admin:        admin.String(),
				VaultAddress: vaultAddr.String(),
				MinRate:      "0.06",
			},
			expectedErrSubstrs: []string{"minimum interest rate", "greater than", "maximum"},
		},
		{
			name:  "min below negative absolute ceiling is rejected",
			setup: setup,
			msg: types.MsgUpdateMinInterestRateRequest{
				Admin:        admin.String(),
				VaultAddress: vaultAddr.String(),
				MinRate:      "-150.0",
			},
			expectedErrSubstrs: []string{"invalid min interest rate", "exceeds maximum allowed magnitude"},
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
			s.Require().NoError(err, "post-check: failed to get vault %s", args.VaultAddress)
			s.Assert().Equal(args.ExpectedMax, v.MaxInterestRate, "post-check: max interest rate mismatch")
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
		s.Require().NoError(err, "failed to create vault for share denom %s", share)
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
				sdk.NewEvent("provlabs.vault.v1.EventMaxInterestRateUpdated",
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
			expectedEvents: sdk.Events{},
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

	baseSetup := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlying, math.NewInt(1000)), admin)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           admin.String(),
			ShareDenom:      share,
			UnderlyingAsset: underlying,
		})
		s.Require().NoError(err, "failed to create vault for share denom %s", share)
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	}

	tests := []msgServerTestCase[types.MsgUpdateMaxInterestRateRequest, any]{
		{
			name:  "vault does not exist",
			setup: func() { s.ctx = s.ctx.WithEventManager(sdk.NewEventManager()) },
			msg: types.MsgUpdateMaxInterestRateRequest{
				Admin:        admin.String(),
				VaultAddress: types.GetVaultAddress("doesnotexist").String(),
				MaxRate:      "0.99",
			},
			expectedErrSubstrs: []string{"not found"},
		},
		{
			name:  "invalid vault address (not a vault account)",
			setup: baseSetup,
			msg: types.MsgUpdateMaxInterestRateRequest{
				Admin:        admin.String(),
				VaultAddress: markertypes.MustGetMarkerAddress(share).String(),
				MaxRate:      "0.10",
			},
			expectedErrSubstrs: []string{"failed to get vault", "is not a vault account"},
		},
		{
			name:  "unauthorized admin",
			setup: baseSetup,
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
				baseSetup()
				_, err := keeper.NewMsgServer(s.simApp.VaultKeeper).UpdateInterestRate(
					s.ctx, &types.MsgUpdateInterestRateRequest{
						Authority:    admin.String(),
						VaultAddress: vaultAddr.String(),
						NewRate:      "0.50",
					},
				)
				s.Require().NoError(err, "initial interest rate update should succeed")

				_, err = keeper.NewMsgServer(s.simApp.VaultKeeper).UpdateMinInterestRate(
					s.ctx, &types.MsgUpdateMinInterestRateRequest{
						Admin:        admin.String(),
						VaultAddress: vaultAddr.String(),
						MinRate:      "0.50",
					},
				)
				s.Require().NoError(err, "initial min interest rate update should succeed")

				s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
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
		{
			name:  "max above absolute ceiling is rejected",
			setup: baseSetup,
			msg: types.MsgUpdateMaxInterestRateRequest{
				Admin:        admin.String(),
				VaultAddress: vaultAddr.String(),
				MaxRate:      "150.0",
			},
			expectedErrSubstrs: []string{"invalid max interest rate", "exceeds maximum allowed magnitude"},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			runMsgServerTestCase(s, testDef, tc)
		})
	}
}

func (s *TestSuite) TestMsgServer_UpdateWithdrawalDelay() {
	type postCheckArgs struct {
		VaultAddress         sdk.AccAddress
		ExpectedDelaySeconds uint64
		ExpectedAuthority    string
	}

	testDef := msgServerTestDef[types.MsgUpdateWithdrawalDelayRequest, types.MsgUpdateWithdrawalDelayResponse, postCheckArgs]{
		endpointName: "UpdateWithdrawalDelay",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).UpdateWithdrawalDelay,
		postCheck: func(msg *types.MsgUpdateWithdrawalDelayRequest, args postCheckArgs) {
			v, err := s.k.GetVault(s.ctx, args.VaultAddress)
			s.Require().NoError(err, "post-check: should load vault for verification")
			s.Assert().Equal(args.ExpectedDelaySeconds, v.WithdrawalDelaySeconds, "post-check: expected WithdrawalDelaySeconds to match")
		},
	}

	underlying := "under"
	share := "vaultshares"
	admin := s.adminAddr
	assetMgr := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1))
	vaultAddr := types.GetVaultAddress(share)

	delayAdmin := uint64(3600)
	delayAssetMgr := uint64(7200)

	setup := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlying, math.NewInt(1000)), admin)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           admin.String(),
			ShareDenom:      share,
			UnderlyingAsset: underlying,
		})
		s.Require().NoError(err, "setup: expected vault creation to succeed")

		v, err := s.k.GetVault(s.ctx, vaultAddr)
		s.Require().NoError(err, "setup: should load vault")
		v.AssetManager = assetMgr.String()
		s.k.AuthKeeper.SetAccount(s.ctx, v)

		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	}

	tests := []msgServerTestCase[types.MsgUpdateWithdrawalDelayRequest, postCheckArgs]{
		{
			name:  "happy path - admin is authority",
			setup: setup,
			msg: types.MsgUpdateWithdrawalDelayRequest{
				Authority:              admin.String(),
				VaultAddress:           vaultAddr.String(),
				WithdrawalDelaySeconds: delayAdmin,
			},
			postCheckArgs: postCheckArgs{
				VaultAddress:         vaultAddr,
				ExpectedDelaySeconds: delayAdmin,
				ExpectedAuthority:    admin.String(),
			},
			expectedEvents: sdk.Events{
				sdk.NewEvent("provlabs.vault.v1.EventWithdrawalDelayUpdated",
					sdk.NewAttribute("authority", admin.String()),
					sdk.NewAttribute("vault_address", vaultAddr.String()),
					sdk.NewAttribute("withdrawal_delay_seconds", fmt.Sprintf("%d", delayAdmin)),
				),
			},
		},
		{
			name:  "happy path - asset manager is authority",
			setup: setup,
			msg: types.MsgUpdateWithdrawalDelayRequest{
				Authority:              assetMgr.String(),
				VaultAddress:           vaultAddr.String(),
				WithdrawalDelaySeconds: delayAssetMgr,
			},
			postCheckArgs: postCheckArgs{
				VaultAddress:         vaultAddr,
				ExpectedDelaySeconds: delayAssetMgr,
				ExpectedAuthority:    assetMgr.String(),
			},
			expectedEvents: sdk.Events{
				sdk.NewEvent("provlabs.vault.v1.EventWithdrawalDelayUpdated",
					sdk.NewAttribute("authority", assetMgr.String()),
					sdk.NewAttribute("vault_address", vaultAddr.String()),
					sdk.NewAttribute("withdrawal_delay_seconds", fmt.Sprintf("%d", delayAssetMgr)),
				),
			},
		},
	}

	testDef.expectedResponse = &types.MsgUpdateWithdrawalDelayResponse{}
	for _, tc := range tests {
		s.Run(tc.name, func() {
			runMsgServerTestCase(s, testDef, tc)
		})
	}
}

func (s *TestSuite) TestMsgServer_UpdateWithdrawalDelay_Failures() {
	testDef := msgServerTestDef[types.MsgUpdateWithdrawalDelayRequest, types.MsgUpdateWithdrawalDelayResponse, any]{
		endpointName: "UpdateWithdrawalDelay",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).UpdateWithdrawalDelay,
		postCheck:    nil,
	}

	underlying := "under"
	share := "vaultshares"
	admin := s.adminAddr
	other := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1))
	vaultAddr := types.GetVaultAddress(share)
	markerAddr := markertypes.MustGetMarkerAddress(share)

	base := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlying, math.NewInt(1000)), admin)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           admin.String(),
			ShareDenom:      share,
			UnderlyingAsset: underlying,
		})
		s.Require().NoError(err, "base: expected vault creation to succeed")
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	}

	tests := []msgServerTestCase[types.MsgUpdateWithdrawalDelayRequest, any]{
		{
			name: "vault not found",
			setup: func() {
				s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
			},
			msg: types.MsgUpdateWithdrawalDelayRequest{
				Authority:              admin.String(),
				VaultAddress:           types.GetVaultAddress("missing").String(),
				WithdrawalDelaySeconds: 10,
			},
			expectedErrSubstrs: []string{"vault not found"},
		},
		{
			name:  "invalid vault address (not a vault account)",
			setup: base,
			msg: types.MsgUpdateWithdrawalDelayRequest{
				Authority:              admin.String(),
				VaultAddress:           markerAddr.String(),
				WithdrawalDelaySeconds: 10,
			},
			expectedErrSubstrs: []string{"failed to get vault", "is not a vault account"},
		},
		{
			name:  "unauthorized admin",
			setup: base,
			msg: types.MsgUpdateWithdrawalDelayRequest{
				Authority:              other.String(),
				VaultAddress:           vaultAddr.String(),
				WithdrawalDelaySeconds: 10,
			},
			expectedErrSubstrs: []string{"unauthorized"},
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
		InVerificationQueue   bool
		ExpectedPeriodStart   int64
	}

	testDef := msgServerTestDef[types.MsgDepositInterestFundsRequest, types.MsgDepositInterestFundsResponse, postCheckArgs]{
		endpointName: "DepositInterestFunds",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).DepositInterestFunds,
		postCheck: func(msg *types.MsgDepositInterestFundsRequest, args postCheckArgs) {
			vaultBal := s.k.BankKeeper.GetBalance(s.ctx, args.VaultAddress, args.ExpectedDepositAmount.Denom)
			s.Assert().Equal(args.ExpectedVaultBalance.Amount.Int64(), vaultBal.Amount.Int64(), "vault balance mismatch")

			s.assertInPayoutVerificationQueue(args.VaultAddress, args.InVerificationQueue)
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
		s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, admin, sdk.NewCoins(amount)), "failed to fund account")
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           admin.String(),
			ShareDenom:      shares,
			UnderlyingAsset: underlying,
		})
		s.Require().NoError(err, "failed to create vault during setup")
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
		s.ctx = s.ctx.WithBlockTime(blockTime)
	}

	setupRestricted := func() {
		requiredAttribute := "thisisrequired"
		s.simApp.AccountKeeper.SetAccount(s.ctx, s.simApp.AccountKeeper.NewAccountWithAddress(s.ctx, admin))
		s.Require().NoError(s.simApp.NameKeeper.SetNameRecord(s.ctx, requiredAttribute, s.adminAddr, false), "should successfully bind the name to the redeemer's address")
		expireTime := time.Now().Add(24 * time.Hour)
		attribute := attrtypes.NewAttribute(requiredAttribute, admin.String(), attrtypes.AttributeType_String, []byte("true"), &expireTime, "")
		s.Require().NoError(s.simApp.AttributeKeeper.SetAttribute(s.ctx, attribute, s.adminAddr), "should successfully set the required attribute on the redeemer")

		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlying, math.NewInt(1000)), admin, requiredAttribute)
		s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, admin, sdk.NewCoins(amount)), "failed to fund account")
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           admin.String(),
			ShareDenom:      shares,
			UnderlyingAsset: underlying,
		})
		s.Require().NoError(err, "should successfully create restricted vault")
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
		s.ctx = s.ctx.WithBlockTime(blockTime)
	}

	s.Run("happy path - deposit interest funds", func() {
		ev := createSendCoinEvents(admin.String(), vaultAddr.String(), sdk.NewCoins(amount).String())
		ev = append(ev, sdk.NewEvent(
			"provlabs.vault.v1.EventInterestDeposit",
			sdk.NewAttribute("amount", amount.String()),
			sdk.NewAttribute("authority", admin.String()),
			sdk.NewAttribute("vault_address", vaultAddr.String()),
		))

		tc := msgServerTestCase[types.MsgDepositInterestFundsRequest, postCheckArgs]{
			name:  "happy path",
			setup: setup,
			msg: types.MsgDepositInterestFundsRequest{
				Authority:    admin.String(),
				VaultAddress: vaultAddr.String(),
				Amount:       amount,
			},
			postCheckArgs: postCheckArgs{
				VaultAddress:          vaultAddr,
				ExpectedDepositAmount: amount,
				ExpectedVaultBalance:  amount,
				InVerificationQueue:   true,
				ExpectedPeriodStart:   blockTime.Unix(),
			},
			expectedEvents: ev,
		}

		testDef.expectedResponse = &types.MsgDepositInterestFundsResponse{}
		runMsgServerTestCase(s, testDef, tc)
	})

	s.Run("happy path - deposit interest funds - restricted marker with attributes", func() {
		ev := createSendCoinEvents(admin.String(), vaultAddr.String(), sdk.NewCoins(amount).String())
		ev = append(ev, sdk.NewEvent(
			"provlabs.vault.v1.EventInterestDeposit",
			sdk.NewAttribute("amount", amount.String()),
			sdk.NewAttribute("authority", admin.String()),
			sdk.NewAttribute("vault_address", vaultAddr.String()),
		))

		tc := msgServerTestCase[types.MsgDepositInterestFundsRequest, postCheckArgs]{
			name:  "happy path restricted",
			setup: setupRestricted,
			msg: types.MsgDepositInterestFundsRequest{
				Authority:    admin.String(),
				VaultAddress: vaultAddr.String(),
				Amount:       amount,
			},
			postCheckArgs: postCheckArgs{
				VaultAddress:          vaultAddr,
				ExpectedDepositAmount: amount,
				ExpectedVaultBalance:  amount,
				InVerificationQueue:   true,
				ExpectedPeriodStart:   blockTime.Unix(),
			},
			expectedEvents: ev,
		}

		testDef.expectedResponse = &types.MsgDepositInterestFundsResponse{}
		runMsgServerTestCase(s, testDef, tc)
	})

	s.Run("happy path - deposit interest funds as asset manager", func() {
		assetMgr := s.CreateAndFundAccount(amount)
		setupWithAssetMgr := func() {
			setup()
			_, err := keeper.NewMsgServer(s.simApp.VaultKeeper).SetAssetManager(s.ctx, &types.MsgSetAssetManagerRequest{
				Admin:        admin.String(),
				VaultAddress: vaultAddr.String(),
				AssetManager: assetMgr.String(),
			})
			s.Require().NoError(err, "failed to set asset manager")
			s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, assetMgr, sdk.NewCoins(amount)), "failed to fund asset manager account")
		}

		ev := createSendCoinEvents(assetMgr.String(), vaultAddr.String(), sdk.NewCoins(amount).String())
		ev = append(ev, sdk.NewEvent(
			"provlabs.vault.v1.EventInterestDeposit",
			sdk.NewAttribute("amount", amount.String()),
			sdk.NewAttribute("authority", assetMgr.String()),
			sdk.NewAttribute("vault_address", vaultAddr.String()),
		))

		tc := msgServerTestCase[types.MsgDepositInterestFundsRequest, postCheckArgs]{
			name:  "happy path asset manager",
			setup: setupWithAssetMgr,
			msg: types.MsgDepositInterestFundsRequest{
				Authority:    assetMgr.String(),
				VaultAddress: vaultAddr.String(),
				Amount:       amount,
			},
			postCheckArgs: postCheckArgs{
				VaultAddress:          vaultAddr,
				ExpectedDepositAmount: amount,
				ExpectedVaultBalance:  amount,
				InVerificationQueue:   true,
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
	unsupportedDenom := "unsupportedDenom"
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
		s.Require().NoError(err, "failed to create vault")
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	}

	setupWithAdminFunds := func() {
		setup()
		s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, admin, sdk.NewCoins(amount)), "failed to fund admin account")
	}

	tests := []msgServerTestCase[types.MsgDepositInterestFundsRequest, any]{
		{
			name: "vault does not exist",
			msg: types.MsgDepositInterestFundsRequest{
				Authority:    admin.String(),
				VaultAddress: types.GetVaultAddress("doesnotexist").String(),
				Amount:       amount,
			},
			expectedErrSubstrs: []string{"not found"},
		},
		{
			name:  "invalid vault address not a vault account",
			setup: setup,
			msg: types.MsgDepositInterestFundsRequest{
				Authority:    admin.String(),
				VaultAddress: markertypes.MustGetMarkerAddress(shares).String(),
				Amount:       amount,
			},
			expectedErrSubstrs: []string{"failed to get vault", "is not a vault account"},
		},
		{
			name:  "unauthorized authority",
			setup: setupWithAdminFunds,
			msg: types.MsgDepositInterestFundsRequest{
				Authority:    other.String(),
				VaultAddress: vaultAddr.String(),
				Amount:       amount,
			},
			expectedErrSubstrs: []string{"unauthorized authority"},
		},
		{
			name:  "incorrect underlying asset",
			setup: setup,
			msg: types.MsgDepositInterestFundsRequest{
				Authority:    admin.String(),
				VaultAddress: vaultAddr.String(),
				Amount:       sdk.NewInt64Coin(unsupportedDenom, 9_999_999),
			},
			expectedErrSubstrs: []string{"denom not supported for vault", "under", unsupportedDenom},
		},
		{
			name:  "insufficient admin balance",
			setup: setup,
			msg: types.MsgDepositInterestFundsRequest{
				Authority:    admin.String(),
				VaultAddress: vaultAddr.String(),
				Amount:       sdk.NewInt64Coin(underlying, 9_999_999),
			},
			expectedErrSubstrs: []string{"failed to deposit funds", "insufficient funds"},
		},
		{
			name: "reconcile failure propagates",
			setup: func() {
				setupWithAdminFunds()
				v, err := s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err, "failed to get vault for manual reconciliation setup")
				s.Require().NotNil(v, "vault should exist for manual reconciliation setup")
				v.CurrentInterestRate = "invalid"
				v.PeriodStart = s.ctx.BlockTime().Unix() - 3600
				s.simApp.AccountKeeper.SetAccount(s.ctx, v)
			},
			msg: types.MsgDepositInterestFundsRequest{
				Authority:    admin.String(),
				VaultAddress: vaultAddr.String(),
				Amount:       sdk.NewInt64Coin(underlying, 1),
			},
			expectedErrSubstrs: []string{"failed to reconcile vault after deposit", "failed to calculate interest"},
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
		AuthorityAddress     sdk.AccAddress
		ExpectedAuthorityAmt sdk.Coin
	}

	testDef := msgServerTestDef[types.MsgWithdrawInterestFundsRequest, types.MsgWithdrawInterestFundsResponse, postCheckArgs]{
		endpointName: "WithdrawInterestFunds",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).WithdrawInterestFunds,
		postCheck: func(msg *types.MsgWithdrawInterestFundsRequest, args postCheckArgs) {
			bal := s.k.BankKeeper.GetBalance(s.ctx, args.AuthorityAddress, args.ExpectedAuthorityAmt.Denom)
			s.Assert().Equal(args.ExpectedAuthorityAmt, bal, "authority balance after withdrawal")
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
		s.Require().NoError(err, "failed to create vault")
		err = FundAccount(s.ctx, s.simApp.BankKeeper, vaultAddr, sdk.NewCoins(amount))
		s.Require().NoError(err, "failed to fund vault account")
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	}

	s.Run("happy path - withdraw interest funds", func() {
		expectedEvents := createSendCoinEvents(vaultAddr.String(), admin.String(), sdk.NewCoins(amount).String())
		expectedEvents = append(expectedEvents, sdk.NewEvent(
			"provlabs.vault.v1.EventInterestWithdrawal",
			sdk.NewAttribute("amount", amount.String()),
			sdk.NewAttribute("authority", admin.String()),
			sdk.NewAttribute("vault_address", vaultAddr.String()),
		))

		tc := msgServerTestCase[types.MsgWithdrawInterestFundsRequest, postCheckArgs]{
			name:  "happy path",
			setup: setup,
			msg: types.MsgWithdrawInterestFundsRequest{
				Authority:    admin.String(),
				VaultAddress: vaultAddr.String(),
				Amount:       amount,
			},
			postCheckArgs: postCheckArgs{
				AuthorityAddress:     admin,
				ExpectedAuthorityAmt: amount,
			},
			expectedEvents: expectedEvents,
		}

		testDef.expectedResponse = &types.MsgWithdrawInterestFundsResponse{}
		runMsgServerTestCase(s, testDef, tc)
	})

	s.Run("happy path - withdraw interest funds as asset manager", func() {
		assetMgr := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1))

		setupWithAssetMgr := func() {
			setup()
			_, err := keeper.NewMsgServer(s.simApp.VaultKeeper).SetAssetManager(s.ctx, &types.MsgSetAssetManagerRequest{
				Admin:        admin.String(),
				VaultAddress: vaultAddr.String(),
				AssetManager: assetMgr.String(),
			})
			s.Require().NoError(err, "failed to set asset manager")
		}

		expectedEvents := createSendCoinEvents(vaultAddr.String(), assetMgr.String(), sdk.NewCoins(amount).String())
		expectedEvents = append(expectedEvents, sdk.NewEvent(
			"provlabs.vault.v1.EventInterestWithdrawal",
			sdk.NewAttribute("amount", amount.String()),
			sdk.NewAttribute("authority", assetMgr.String()),
			sdk.NewAttribute("vault_address", vaultAddr.String()),
		))

		tc := msgServerTestCase[types.MsgWithdrawInterestFundsRequest, postCheckArgs]{
			name:  "happy path asset manager",
			setup: setupWithAssetMgr,
			msg: types.MsgWithdrawInterestFundsRequest{
				Authority:    assetMgr.String(),
				VaultAddress: vaultAddr.String(),
				Amount:       amount,
			},
			postCheckArgs: postCheckArgs{
				AuthorityAddress:     assetMgr,
				ExpectedAuthorityAmt: amount,
			},
			expectedEvents: expectedEvents,
		}

		testDef.expectedResponse = &types.MsgWithdrawInterestFundsResponse{}
		runMsgServerTestCase(s, testDef, tc)
	})

	s.Run("happy path - withdraw interest funds with receipt token (admin)", func() {
		receipt := "receiptunder"
		receiptSupply := math.NewInt(1000)
		withdrawAmt := sdk.NewInt64Coin(receipt, 500)
		receiptVaultAddr := types.GetVaultAddress(shares)

		setupReceiptAdmin := func() {
			s.requireAddFinalizeAndActivateReceiptMarker(sdk.NewCoin(receipt, receiptSupply), admin, receiptVaultAddr)
			_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
				Admin:           admin.String(),
				ShareDenom:      shares,
				UnderlyingAsset: receipt,
			})
			s.Require().NoError(err, "failed to create vault with receipt underlying")
			err = FundAccount(markertypes.WithBypass(s.ctx), s.simApp.BankKeeper, receiptVaultAddr, sdk.NewCoins(withdrawAmt))
			s.Require().NoError(err, "failed to fund vault account with receipt token")
			s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
		}

		expecteEvents := createSendCoinEvents(receiptVaultAddr.String(), admin.String(), sdk.NewCoins(withdrawAmt).String())
		expecteEvents = append(expecteEvents, sdk.NewEvent(
			"provlabs.vault.v1.EventInterestWithdrawal",
			sdk.NewAttribute("amount", withdrawAmt.String()),
			sdk.NewAttribute("authority", admin.String()),
			sdk.NewAttribute("vault_address", receiptVaultAddr.String()),
		))

		tc := msgServerTestCase[types.MsgWithdrawInterestFundsRequest, postCheckArgs]{
			name:  "receipt token admin",
			setup: setupReceiptAdmin,
			msg: types.MsgWithdrawInterestFundsRequest{
				Authority:    admin.String(),
				VaultAddress: receiptVaultAddr.String(),
				Amount:       withdrawAmt,
			},
			postCheckArgs: postCheckArgs{
				AuthorityAddress:     admin,
				ExpectedAuthorityAmt: withdrawAmt,
			},
			expectedEvents: expecteEvents,
		}

		testDef.expectedResponse = &types.MsgWithdrawInterestFundsResponse{}
		runMsgServerTestCase(s, testDef, tc)
	})

	s.Run("happy path - withdraw interest funds with receipt token (asset manager)", func() {
		receipt := "receiptunder2"
		receiptSupply := math.NewInt(1000)
		withdrawAmt := sdk.NewInt64Coin(receipt, 500)
		assetMgr := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1))
		receiptVaultAddr := types.GetVaultAddress(shares)

		setupReceiptAssetMgr := func() {
			s.requireAddFinalizeAndActivateReceiptMarker(sdk.NewCoin(receipt, receiptSupply), assetMgr, receiptVaultAddr)
			_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
				Admin:           admin.String(),
				ShareDenom:      shares,
				UnderlyingAsset: receipt,
			})
			s.Require().NoError(err, "failed to create vault with receipt underlying")
			err = FundAccount(markertypes.WithBypass(s.ctx), s.simApp.BankKeeper, receiptVaultAddr, sdk.NewCoins(withdrawAmt))
			s.Require().NoError(err, "failed to fund vault account with receipt token")
			s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
			_, err = keeper.NewMsgServer(s.simApp.VaultKeeper).SetAssetManager(s.ctx, &types.MsgSetAssetManagerRequest{
				Admin:        admin.String(),
				VaultAddress: receiptVaultAddr.String(),
				AssetManager: assetMgr.String(),
			})
			s.Require().NoError(err, "failed to set asset manager for receipt case")
		}

		expectedEvent := createSendCoinEvents(receiptVaultAddr.String(), assetMgr.String(), sdk.NewCoins(withdrawAmt).String())
		expectedEvent = append(expectedEvent, sdk.NewEvent(
			"provlabs.vault.v1.EventInterestWithdrawal",
			sdk.NewAttribute("amount", withdrawAmt.String()),
			sdk.NewAttribute("authority", assetMgr.String()),
			sdk.NewAttribute("vault_address", receiptVaultAddr.String()),
		))

		tc := msgServerTestCase[types.MsgWithdrawInterestFundsRequest, postCheckArgs]{
			name:  "receipt token asset manager",
			setup: setupReceiptAssetMgr,
			msg: types.MsgWithdrawInterestFundsRequest{
				Authority:    assetMgr.String(),
				VaultAddress: receiptVaultAddr.String(),
				Amount:       withdrawAmt,
			},
			postCheckArgs: postCheckArgs{
				AuthorityAddress:     assetMgr,
				ExpectedAuthorityAmt: withdrawAmt,
			},
			expectedEvents: expectedEvent,
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
	receiptUnderlying := "receiptunder"
	shares := "vaultshares"
	unsupportedDenom := "unsupportedDenom"
	admin := s.adminAddr
	other := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1000))
	vaultAddr := types.GetVaultAddress(shares)
	markerAddr := markertypes.MustGetMarkerAddress(shares)
	amountRegular := sdk.NewInt64Coin(underlying, 500)
	amountReceipt := sdk.NewInt64Coin(receiptUnderlying, 500)

	setupRegular := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlying, math.NewInt(1000)), admin)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           admin.String(),
			ShareDenom:      shares,
			UnderlyingAsset: underlying,
		})
		s.Require().NoError(err, "failed to create vault")
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	}

	setupRegularWithVaultFunds := func() {
		setupRegular()
		err := FundAccount(s.ctx, s.simApp.BankKeeper, vaultAddr, sdk.NewCoins(amountRegular))
		s.Require().NoError(err, "failed to fund vault account")
	}

	setupReceipt := func() {
		s.requireAddFinalizeAndActivateReceiptMarker(sdk.NewCoin(receiptUnderlying, math.NewInt(1000)), admin, types.GetVaultAddress(shares))
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           admin.String(),
			ShareDenom:      shares,
			UnderlyingAsset: receiptUnderlying,
		})
		s.Require().NoError(err, "failed to create receipt-underlying vault")
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	}

	setupReceiptWithVaultFunds := func() {
		setupReceipt()
		err := FundAccount(markertypes.WithBypass(s.ctx), s.simApp.BankKeeper, vaultAddr, sdk.NewCoins(amountReceipt))
		s.Require().NoError(err, "failed to fund receipt-underlying vault account")
	}

	setupSendFailsNoTransferPerm := func() {
		thirdParty := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1))
		s.requireAddFinalizeAndActivateReceiptMarker(sdk.NewCoin(receiptUnderlying, math.NewInt(2000)), thirdParty, markertypes.MustGetMarkerAddress(shares))
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           admin.String(),
			ShareDenom:      shares,
			UnderlyingAsset: receiptUnderlying,
		})
		s.Require().NoError(err, "failed to create vault for send-fails case")
		err = FundAccount(markertypes.WithBypass(s.ctx), s.simApp.BankKeeper, vaultAddr, sdk.NewCoins(amountReceipt))
		s.Require().NoError(err, "failed to fund receipt-underlying vault account for send-fails case")
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	}

	tests := []msgServerTestCase[types.MsgWithdrawInterestFundsRequest, any]{
		{
			name: "vault does not exist",
			msg: types.MsgWithdrawInterestFundsRequest{
				Authority:    admin.String(),
				VaultAddress: types.GetVaultAddress("doesnotexist").String(),
				Amount:       amountRegular,
			},
			expectedErrSubstrs: []string{"not found"},
		},
		{
			name:  "invalid vault address not a vault account",
			setup: setupRegular,
			msg: types.MsgWithdrawInterestFundsRequest{
				Authority:    admin.String(),
				VaultAddress: markerAddr.String(),
				Amount:       amountRegular,
			},
			expectedErrSubstrs: []string{"failed to get vault", "is not a vault account"},
		},
		{
			name:  "unauthorized authority",
			setup: setupRegularWithVaultFunds,
			msg: types.MsgWithdrawInterestFundsRequest{
				Authority:    other.String(),
				VaultAddress: vaultAddr.String(),
				Amount:       amountRegular,
			},
			expectedErrSubstrs: []string{"unauthorized authority"},
		},
		{
			name:  "insufficient vault balance",
			setup: setupRegular,
			msg: types.MsgWithdrawInterestFundsRequest{
				Authority:    admin.String(),
				VaultAddress: vaultAddr.String(),
				Amount:       sdk.NewInt64Coin(underlying, 9_999_999),
			},
			expectedErrSubstrs: []string{"failed to withdraw interest funds", "insufficient funds"},
		},
		{
			name:  "incorrect underlying asset",
			setup: setupRegular,
			msg: types.MsgWithdrawInterestFundsRequest{
				Authority:    admin.String(),
				VaultAddress: vaultAddr.String(),
				Amount:       sdk.NewInt64Coin(unsupportedDenom, 9_999_999),
			},
			expectedErrSubstrs: []string{"denom not supported for vault", underlying, unsupportedDenom},
		},
		{
			name: "reconcile failure propagates",
			setup: func() {
				setupRegularWithVaultFunds()
				v, err := s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err, "failed to get vault")
				s.Require().NotNil(v, "vault should not be nil")
				v.CurrentInterestRate = "invalid"
				v.PeriodStart = s.ctx.BlockTime().Unix() - 3600
				s.simApp.AccountKeeper.SetAccount(s.ctx, v)
			},
			msg: types.MsgWithdrawInterestFundsRequest{
				Authority:    admin.String(),
				VaultAddress: vaultAddr.String(),
				Amount:       sdk.NewInt64Coin(underlying, 1),
			},
			expectedErrSubstrs: []string{"failed to reconcile vault before withdrawal", "failed to calculate interest"},
		},
		{
			name:  "receipt underlying: unauthorized authority",
			setup: setupReceiptWithVaultFunds,
			msg: types.MsgWithdrawInterestFundsRequest{
				Authority:    other.String(),
				VaultAddress: vaultAddr.String(),
				Amount:       amountReceipt,
			},
			expectedErrSubstrs: []string{"unauthorized authority"},
		},
		{
			name:  "receipt underlying: insufficient vault balance",
			setup: setupReceipt,
			msg: types.MsgWithdrawInterestFundsRequest{
				Authority:    admin.String(),
				VaultAddress: vaultAddr.String(),
				Amount:       sdk.NewInt64Coin(receiptUnderlying, 9_999_999),
			},
			expectedErrSubstrs: []string{"failed to withdraw interest funds", "insufficient funds"},
		},
		{
			name:  "receipt underlying: incorrect underlying asset",
			setup: setupReceipt,
			msg: types.MsgWithdrawInterestFundsRequest{
				Authority:    admin.String(),
				VaultAddress: vaultAddr.String(),
				Amount:       sdk.NewInt64Coin(unsupportedDenom, 9_999_999),
			},
			expectedErrSubstrs: []string{"denom not supported for vault", receiptUnderlying, unsupportedDenom},
		},
		{
			name:  "receipt underlying: send fails without transfer permission on receipt token",
			setup: setupSendFailsNoTransferPerm,
			msg: types.MsgWithdrawInterestFundsRequest{
				Authority:    admin.String(),
				VaultAddress: vaultAddr.String(),
				Amount:       amountReceipt,
			},
			expectedErrSubstrs: []string{"failed to withdraw interest funds", "does not have transfer permissions for", receiptUnderlying},
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
			s.Assert().Equal(args.ExpectedVaultAssets, balance, "marker balance mismatch after principal deposit")
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
		s.Require().NoError(err, "failed to create vault for share denom %s", share)
		s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, admin, sdk.NewCoins(amount)), "failed to fund admin account during setup")
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
		vault, err := s.k.GetVault(s.ctx, vaultAddr)
		s.Require().NoError(err, "failed to get vault %s", vaultAddr)
		vault.Paused = true
		s.k.AuthKeeper.SetAccount(s.ctx, vault)
	}

	s.Run("happy path - deposit principal funds", func() {
		ev := createSendCoinEvents(admin.String(), markerAddr.String(), sdk.NewCoins(amount).String())
		ev = append(ev, sdk.NewEvent(
			"provlabs.vault.v1.EventDepositPrincipalFunds",
			sdk.NewAttribute("amount", amount.String()),
			sdk.NewAttribute("authority", admin.String()),
			sdk.NewAttribute("vault_address", vaultAddr.String()),
		))

		tc := msgServerTestCase[types.MsgDepositPrincipalFundsRequest, postCheckArgs]{
			name:  "happy path",
			setup: setup,
			msg: types.MsgDepositPrincipalFundsRequest{
				Authority:    admin.String(),
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

	s.Run("happy path - deposit principal funds as asset manager", func() {
		assetMgr := s.CreateAndFundAccount(sdk.NewInt64Coin(underlying, amount.Amount.Int64()))

		setupWithAssetMgr := func() {
			// base setup with marker/vault, but fund the asset manager for the deposit flow
			s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlying, math.NewInt(1000)), admin)
			_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
				Admin:           admin.String(),
				ShareDenom:      share,
				UnderlyingAsset: underlying,
			})
			s.Require().NoError(err, "failed to create vault during asset manager setup")

			s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, assetMgr, sdk.NewCoins(amount)), "failed to fund asset manager during setup")

			s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
			vault, err := s.k.GetVault(s.ctx, vaultAddr)
			s.Require().NoError(err, "failed to get vault during asset manager setup")
			vault.Paused = true
			s.k.AuthKeeper.SetAccount(s.ctx, vault)

			_, err = keeper.NewMsgServer(s.simApp.VaultKeeper).SetAssetManager(s.ctx, &types.MsgSetAssetManagerRequest{
				Admin:        admin.String(),
				VaultAddress: vaultAddr.String(),
				AssetManager: assetMgr.String(),
			})
			s.Require().NoError(err, "failed to set asset manager")
		}

		ev := createSendCoinEvents(assetMgr.String(), markerAddr.String(), sdk.NewCoins(amount).String())
		ev = append(ev, sdk.NewEvent(
			"provlabs.vault.v1.EventDepositPrincipalFunds",
			sdk.NewAttribute("amount", amount.String()),
			sdk.NewAttribute("authority", assetMgr.String()),
			sdk.NewAttribute("vault_address", vaultAddr.String()),
		))

		tc := msgServerTestCase[types.MsgDepositPrincipalFundsRequest, postCheckArgs]{
			name:  "happy path asset manager",
			setup: setupWithAssetMgr,
			msg: types.MsgDepositPrincipalFundsRequest{
				Authority:    assetMgr.String(),
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
		s.Require().NoError(err, "failed to create vault for share denom %s", share)
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
		vault, err := s.k.GetVault(s.ctx, vaultAddr)
		s.Require().NoError(err, "failed to get vault %s", vaultAddr)
		vault.Paused = true
		s.k.AuthKeeper.SetAccount(s.ctx, vault)
	}

	setupNotPaused := func() {
		setup()
		vault, err := s.k.GetVault(s.ctx, vaultAddr)
		s.Require().NoError(err, "failed to get vault %s", vaultAddr)
		vault.Paused = false
		s.k.AuthKeeper.SetAccount(s.ctx, vault)
	}

	tests := []msgServerTestCase[types.MsgDepositPrincipalFundsRequest, any]{
		{
			name: "vault does not exist",
			msg: types.MsgDepositPrincipalFundsRequest{
				Authority:    admin.String(),
				VaultAddress: types.GetVaultAddress("doesnotexist").String(),
				Amount:       amount,
			},
			expectedErrSubstrs: []string{"not found"},
		},
		{
			name:  "invalid vault address not a vault account",
			setup: setup,
			msg: types.MsgDepositPrincipalFundsRequest{
				Authority:    admin.String(),
				VaultAddress: markertypes.MustGetMarkerAddress(share).String(),
				Amount:       amount,
			},
			expectedErrSubstrs: []string{"failed to get vault", "is not a vault account"},
		},
		{
			name:  "unauthorized admin",
			setup: setup,
			msg: types.MsgDepositPrincipalFundsRequest{
				Authority:    other.String(),
				VaultAddress: vaultAddr.String(),
				Amount:       amount,
			},
			expectedErrSubstrs: []string{"unauthorized authority"},
		},
		{
			name:  "vault is not paused",
			setup: setupNotPaused,
			msg: types.MsgDepositPrincipalFundsRequest{
				Authority:    admin.String(),
				VaultAddress: vaultAddr.String(),
				Amount:       amount,
			},
			expectedErrSubstrs: []string{"vault must be paused to deposit principal funds"},
		},
		{
			name:  "invalid asset for vault",
			setup: setup,
			msg: types.MsgDepositPrincipalFundsRequest{
				Authority:    admin.String(),
				VaultAddress: vaultAddr.String(),
				Amount:       sdk.NewInt64Coin("wrongdenom", 500),
			},
			expectedErrSubstrs: []string{"denom not supported for vault", "under", "wrongdenom"},
		},
		{
			name:  "insufficient admin balance",
			setup: setup,
			msg: types.MsgDepositPrincipalFundsRequest{
				Authority:    admin.String(),
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
			s.Assert().Equal(args.ExpectedAdminAssets, balance, "admin balance after principal withdrawal")
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
		s.Require().NoError(err, "failed to create vault")
		err = FundAccount(s.ctx, s.simApp.BankKeeper, markerAddr, sdk.NewCoins(amount))
		s.Require().NoError(err, "failed to fund marker account")
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
		vault, err := s.k.GetVault(s.ctx, vaultAddr)
		s.Require().NoError(err, "failed to get vault")
		vault.Paused = true
		s.k.AuthKeeper.SetAccount(s.ctx, vault)
	}

	s.Run("happy path - withdraw principal funds", func() {
		ev := createSendCoinEvents(markerAddr.String(), admin.String(), sdk.NewCoins(amount).String())
		ev = append(ev, sdk.NewEvent(
			"provlabs.vault.v1.EventWithdrawPrincipalFunds",
			sdk.NewAttribute("amount", amount.String()),
			sdk.NewAttribute("authority", admin.String()),
			sdk.NewAttribute("vault_address", vaultAddr.String()),
		))

		tc := msgServerTestCase[types.MsgWithdrawPrincipalFundsRequest, postCheckArgs]{
			name:  "happy path",
			setup: setup,
			msg: types.MsgWithdrawPrincipalFundsRequest{
				Authority:    admin.String(),
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

	s.Run("happy path - withdraw principal funds as asset manager", func() {
		assetMgr := s.CreateAndFundAccount(sdk.Coin{Denom: underlying, Amount: math.ZeroInt()})

		setupWithAssetMgr := func() {
			s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlying, math.NewInt(1000)), admin)
			_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
				Admin:           admin.String(),
				ShareDenom:      share,
				UnderlyingAsset: underlying,
			})
			s.Require().NoError(err, "failed to create vault")
			err = FundAccount(s.ctx, s.simApp.BankKeeper, markerAddr, sdk.NewCoins(amount))
			s.Require().NoError(err, "failed to fund marker account")
			s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
			vault, err := s.k.GetVault(s.ctx, vaultAddr)
			s.Require().NoError(err, "failed to get vault")
			vault.Paused = true
			s.k.AuthKeeper.SetAccount(s.ctx, vault)
			_, err = keeper.NewMsgServer(s.simApp.VaultKeeper).SetAssetManager(s.ctx, &types.MsgSetAssetManagerRequest{
				Admin:        admin.String(),
				VaultAddress: vaultAddr.String(),
				AssetManager: assetMgr.String(),
			})
			s.Require().NoError(err, "failed to set asset manager")
		}

		ev := createSendCoinEvents(markerAddr.String(), assetMgr.String(), sdk.NewCoins(amount).String())
		ev = append(ev, sdk.NewEvent(
			"provlabs.vault.v1.EventWithdrawPrincipalFunds",
			sdk.NewAttribute("amount", amount.String()),
			sdk.NewAttribute("authority", assetMgr.String()),
			sdk.NewAttribute("vault_address", vaultAddr.String()),
		))

		tc := msgServerTestCase[types.MsgWithdrawPrincipalFundsRequest, postCheckArgs]{
			name:  "happy path asset manager",
			setup: setupWithAssetMgr,
			msg: types.MsgWithdrawPrincipalFundsRequest{
				Authority:    assetMgr.String(),
				VaultAddress: vaultAddr.String(),
				Amount:       amount,
			},
			postCheckArgs: postCheckArgs{
				AdminAddress:        assetMgr,
				MarkerAddress:       markerAddr,
				ExpectedAdminAssets: amount,
			},
			expectedEvents: ev,
		}

		testDef.expectedResponse = &types.MsgWithdrawPrincipalFundsResponse{}
		runMsgServerTestCase(s, testDef, tc)
	})

	s.Run("happy path - withdraw principal funds with receipt token (admin)", func() {
		receipt := "receiptunder"
		receiptSupply := math.NewInt(1000)
		withdrawAmt := sdk.NewInt64Coin(receipt, 500)
		receiptVaultAddr := types.GetVaultAddress(share)
		receiptMarkerAddr := markertypes.MustGetMarkerAddress(share)

		setupReceiptAdmin := func() {
			s.requireAddFinalizeAndActivateReceiptMarker(sdk.NewCoin(receipt, receiptSupply), admin, receiptVaultAddr)
			_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
				Admin:           admin.String(),
				ShareDenom:      share,
				UnderlyingAsset: receipt,
			})
			s.Require().NoError(err, "failed to create vault with receipt underlying")
			err = FundAccount(markertypes.WithBypass(s.ctx), s.simApp.BankKeeper, receiptMarkerAddr, sdk.NewCoins(withdrawAmt))
			s.Require().NoError(err, "failed to fund receipt marker account")
			s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
			vault, err := s.k.GetVault(s.ctx, receiptVaultAddr)
			s.Require().NoError(err, "failed to get vault")
			vault.Paused = true
			s.k.AuthKeeper.SetAccount(s.ctx, vault)
		}

		ev := createSendCoinEvents(receiptMarkerAddr.String(), admin.String(), sdk.NewCoins(withdrawAmt).String())
		ev = append(ev, sdk.NewEvent(
			"provlabs.vault.v1.EventWithdrawPrincipalFunds",
			sdk.NewAttribute("amount", withdrawAmt.String()),
			sdk.NewAttribute("authority", admin.String()),
			sdk.NewAttribute("vault_address", receiptVaultAddr.String()),
		))

		tc := msgServerTestCase[types.MsgWithdrawPrincipalFundsRequest, postCheckArgs]{
			name:  "receipt token admin",
			setup: setupReceiptAdmin,
			msg: types.MsgWithdrawPrincipalFundsRequest{
				Authority:    admin.String(),
				VaultAddress: receiptVaultAddr.String(),
				Amount:       withdrawAmt,
			},
			postCheckArgs: postCheckArgs{
				AdminAddress:        admin,
				MarkerAddress:       receiptMarkerAddr,
				ExpectedAdminAssets: withdrawAmt,
			},
			expectedEvents: ev,
		}

		testDef.expectedResponse = &types.MsgWithdrawPrincipalFundsResponse{}
		runMsgServerTestCase(s, testDef, tc)
	})

	s.Run("happy path - withdraw principal funds with receipt token (asset manager)", func() {
		receiptDenom := "receiptunder2"
		receiptSupply := math.NewInt(1000)
		withdrawAmt := sdk.NewInt64Coin(receiptDenom, 500)
		assetMgr := s.CreateAndFundAccount(sdk.Coin{Denom: receiptDenom, Amount: math.ZeroInt()})
		receiptVaultAddr := types.GetVaultAddress(share)
		receiptMarkerAddr := markertypes.MustGetMarkerAddress(share)

		setupReceiptAssetMgr := func() {
			s.requireAddFinalizeAndActivateReceiptMarker(sdk.NewCoin(receiptDenom, receiptSupply), assetMgr, receiptVaultAddr)
			_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
				Admin:           admin.String(),
				ShareDenom:      share,
				UnderlyingAsset: receiptDenom,
			})
			s.Require().NoError(err, "failed to create vault with receipt underlying")
			err = FundAccount(markertypes.WithBypass(s.ctx), s.simApp.BankKeeper, receiptMarkerAddr, sdk.NewCoins(withdrawAmt))
			s.Require().NoError(err, "failed to fund receipt marker account")
			s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
			vault, err := s.k.GetVault(s.ctx, receiptVaultAddr)
			s.Require().NoError(err, "failed to get vault")
			vault.Paused = true
			s.k.AuthKeeper.SetAccount(s.ctx, vault)
			_, err = keeper.NewMsgServer(s.simApp.VaultKeeper).SetAssetManager(s.ctx, &types.MsgSetAssetManagerRequest{
				Admin:        admin.String(),
				VaultAddress: receiptVaultAddr.String(),
				AssetManager: assetMgr.String(),
			})
			s.Require().NoError(err, "failed to set asset manager for receipt case")
		}

		ev := createSendCoinEvents(receiptMarkerAddr.String(), assetMgr.String(), sdk.NewCoins(withdrawAmt).String())
		ev = append(ev, sdk.NewEvent(
			"provlabs.vault.v1.EventWithdrawPrincipalFunds",
			sdk.NewAttribute("amount", withdrawAmt.String()),
			sdk.NewAttribute("authority", assetMgr.String()),
			sdk.NewAttribute("vault_address", receiptVaultAddr.String()),
		))

		tc := msgServerTestCase[types.MsgWithdrawPrincipalFundsRequest, postCheckArgs]{
			name:  "receipt token asset manager",
			setup: setupReceiptAssetMgr,
			msg: types.MsgWithdrawPrincipalFundsRequest{
				Authority:    assetMgr.String(),
				VaultAddress: receiptVaultAddr.String(),
				Amount:       withdrawAmt,
			},
			postCheckArgs: postCheckArgs{
				AdminAddress:        assetMgr,
				MarkerAddress:       receiptMarkerAddr,
				ExpectedAdminAssets: withdrawAmt,
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
	receiptUnderlying := "receiptunder"
	share := "vaultshares"
	admin := s.adminAddr
	other := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1000))
	vaultAddr := types.GetVaultAddress(share)
	shareMarkerAddr := markertypes.MustGetMarkerAddress(share)
	amountRegular := sdk.NewInt64Coin(underlying, 500)
	amountReceipt := sdk.NewInt64Coin(receiptUnderlying, 500)
	requireAttrs := "thisisattri.restricted"

	setupRegular := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlying, math.NewInt(1000)), admin, requireAttrs)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           admin.String(),
			ShareDenom:      share,
			UnderlyingAsset: underlying,
		})
		s.Require().NoError(err, "failed to create vault")
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
		vault, err := s.k.GetVault(s.ctx, vaultAddr)
		s.Require().NoError(err, "failed to get vault")
		vault.Paused = true
		s.k.AuthKeeper.SetAccount(s.ctx, vault)
	}

	setupRegularNotPaused := func() {
		setupRegular()
		vault, err := s.k.GetVault(s.ctx, vaultAddr)
		s.Require().NoError(err, "failed to get vault")
		vault.Paused = false
		s.k.AuthKeeper.SetAccount(s.ctx, vault)
	}

	setupRegularWithShareFunds := func() {
		setupRegular()
		err := FundAccount(markertypes.WithBypass(s.ctx), s.simApp.BankKeeper, shareMarkerAddr, sdk.NewCoins(amountRegular))
		s.Require().NoError(err, "failed to fund share marker account")
	}

	setupReceipt := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(receiptUnderlying, math.NewInt(1000)), admin, requireAttrs)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           admin.String(),
			ShareDenom:      share,
			UnderlyingAsset: receiptUnderlying,
		})
		s.Require().NoError(err, "failed to create receipt-underlying vault")
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
		vault, err := s.k.GetVault(s.ctx, vaultAddr)
		s.Require().NoError(err, "failed to get vault")
		vault.Paused = true
		s.k.AuthKeeper.SetAccount(s.ctx, vault)
	}

	setupReceiptWithShareFunds := func() {
		setupReceipt()
		err := FundAccount(markertypes.WithBypass(s.ctx), s.simApp.BankKeeper, shareMarkerAddr, sdk.NewCoins(amountReceipt))
		s.Require().NoError(err, "failed to fund share marker account with receipt token")
	}

	setupSendFailsNoTransferPerm := func() {
		thirdParty := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1))
		// The receipt token is created and managed by thirdParty. admin has NO permissions on it.
		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(receiptUnderlying, math.NewInt(2000)), thirdParty, requireAttrs)

		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           admin.String(),
			ShareDenom:      share,
			UnderlyingAsset: receiptUnderlying,
		})
		s.Require().NoError(err, "failed to create vault for send-fails case")

		m, err := s.simApp.MarkerKeeper.GetMarkerByDenom(s.ctx, share)
		s.Require().NoError(err, "failed to get share marker to grant withdraw")
		mk := m.(*markertypes.MarkerAccount)
		// admin is granted Withdraw on the VAULT'S share marker, but still lacks permission on the receipt underlying marker.
		mk.AccessControl = append(mk.AccessControl, markertypes.AccessGrant{
			Address:     admin.String(),
			Permissions: []markertypes.Access{markertypes.Access_Withdraw},
		})
		s.simApp.MarkerKeeper.SetMarker(s.ctx, mk)

		err = FundAccount(markertypes.WithBypass(s.ctx), s.simApp.BankKeeper, shareMarkerAddr, sdk.NewCoins(amountReceipt))
		s.Require().NoError(err, "failed to fund share marker account with receipt token for send-fails case")

		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
		vault, err := s.k.GetVault(s.ctx, vaultAddr)
		s.Require().NoError(err, "failed to get vault")
		vault.Paused = true
		s.k.AuthKeeper.SetAccount(s.ctx, vault)
	}

	tests := []msgServerTestCase[types.MsgWithdrawPrincipalFundsRequest, any]{
		{
			name: "vault does not exist",
			msg: types.MsgWithdrawPrincipalFundsRequest{
				Authority:    admin.String(),
				VaultAddress: types.GetVaultAddress("doesnotexist").String(),
				Amount:       amountRegular,
			},
			expectedErrSubstrs: []string{"not found"},
		},
		{
			name:  "invalid vault address not a vault account",
			setup: setupRegular,
			msg: types.MsgWithdrawPrincipalFundsRequest{
				Authority:    admin.String(),
				VaultAddress: shareMarkerAddr.String(),
				Amount:       amountRegular,
			},
			expectedErrSubstrs: []string{"failed to get vault", "is not a vault account"},
		},
		{
			name:  "unauthorized authority",
			setup: setupRegularWithShareFunds,
			msg: types.MsgWithdrawPrincipalFundsRequest{
				Authority:    other.String(),
				VaultAddress: vaultAddr.String(),
				Amount:       amountRegular,
			},
			expectedErrSubstrs: []string{"unauthorized authority"},
		},
		{
			name:  "vault is not paused",
			setup: setupRegularNotPaused,
			msg: types.MsgWithdrawPrincipalFundsRequest{
				Authority:    admin.String(),
				VaultAddress: vaultAddr.String(),
				Amount:       amountRegular,
			},
			expectedErrSubstrs: []string{"vault must be paused to withdraw principal funds"},
		},
		{
			name:  "invalid asset for vault",
			setup: setupRegular,
			msg: types.MsgWithdrawPrincipalFundsRequest{
				Authority:    admin.String(),
				VaultAddress: vaultAddr.String(),
				Amount:       sdk.NewInt64Coin("wrongdenom", 500),
			},
			expectedErrSubstrs: []string{"denom not supported for vault", "under", "wrongdenom"},
		},
		{
			name:  "insufficient share marker balance",
			setup: setupRegular,
			msg: types.MsgWithdrawPrincipalFundsRequest{
				Authority:    admin.String(),
				VaultAddress: vaultAddr.String(),
				Amount:       sdk.NewInt64Coin(underlying, 9_999_999),
			},
			expectedErrSubstrs: []string{"failed to withdraw principal funds", "insufficient funds"},
		},
		{
			name:  "receipt underlying: unauthorized authority",
			setup: setupReceiptWithShareFunds,
			msg: types.MsgWithdrawPrincipalFundsRequest{
				Authority:    other.String(),
				VaultAddress: vaultAddr.String(),
				Amount:       amountReceipt,
			},
			expectedErrSubstrs: []string{"unauthorized authority"},
		},
		{
			name:  "receipt underlying: insufficient share marker balance",
			setup: setupReceipt,
			msg: types.MsgWithdrawPrincipalFundsRequest{
				Authority:    admin.String(),
				VaultAddress: vaultAddr.String(),
				Amount:       sdk.NewInt64Coin(receiptUnderlying, 9_999_999),
			},
			expectedErrSubstrs: []string{"failed to withdraw principal funds", "insufficient funds"},
		},
		{
			name:  "receipt underlying: invalid asset for vault",
			setup: setupReceipt,
			msg: types.MsgWithdrawPrincipalFundsRequest{
				Authority:    admin.String(),
				VaultAddress: vaultAddr.String(),
				Amount:       sdk.NewInt64Coin("wrongdenom", 500),
			},
			expectedErrSubstrs: []string{"denom not supported for vault", receiptUnderlying, "wrongdenom"},
		},
		{
			name:  "receipt underlying: send fails without transfer permission on receipt token",
			setup: setupSendFailsNoTransferPerm,
			msg: types.MsgWithdrawPrincipalFundsRequest{
				Authority:    admin.String(),
				VaultAddress: vaultAddr.String(),
				Amount:       amountReceipt,
			},
			expectedErrSubstrs: []string{
				"failed to withdraw principal funds",
				"does not contain the \"receiptunder\" required attribute",
				requireAttrs,
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			runMsgServerTestCase(s, testDef, tc)
		})
	}
}

func (s *TestSuite) TestMsgServer_ExpeditePendingSwapOut() {
	type postCheckArgs struct {
		RequestId uint64
	}

	testDef := msgServerTestDef[types.MsgExpeditePendingSwapOutRequest, types.MsgExpeditePendingSwapOutResponse, postCheckArgs]{
		endpointName: "ExpeditePendingSwapOut",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).ExpeditePendingSwapOut,
		postCheck: func(msg *types.MsgExpeditePendingSwapOutRequest, args postCheckArgs) {
			release, withdrawal, err := s.k.PendingSwapOutQueue.GetByID(s.ctx, args.RequestId)
			s.Require().NoError(err, "should be able to get swap out request by ID")
			s.Assert().Equal(int64(0), release, "release time should be expedited to 0")
			s.Assert().NotNil(withdrawal, "swap out should not be nil after expedite")
		},
	}

	underlying := "under"
	share := "vaultshares"
	admin := s.adminAddr
	other := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1000))
	vaultAddr := types.GetVaultAddress(share)
	blockTime := time.Now()

	var id uint64
	setup := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlying, math.NewInt(1000)), admin)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           admin.String(),
			ShareDenom:      share,
			UnderlyingAsset: underlying,
		})
		s.Require().NoError(err, "should create vault")

		validOwner := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1)).String()
		validShares := sdk.NewInt64Coin(share, 1)

		var qErr error
		pendingReq := types.NewPendingSwapOut(
			sdk.MustAccAddressFromBech32(validOwner),
			vaultAddr,
			validShares,
			underlying,
		)
		id, qErr = s.k.PendingSwapOutQueue.Enqueue(s.ctx, blockTime.Unix(), &pendingReq)
		s.Require().NoError(qErr, "should successfully enqueue pending swap out request")
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	}

	setupWithAssetMgr := func(assetMgr sdk.AccAddress) {
		setup()
		_, err := keeper.NewMsgServer(s.simApp.VaultKeeper).SetAssetManager(s.ctx, &types.MsgSetAssetManagerRequest{
			Admin:        admin.String(),
			VaultAddress: vaultAddr.String(),
			AssetManager: assetMgr.String(),
		})
		s.Require().NoError(err, "should set asset manager")
	}

	tests := []msgServerTestCase[types.MsgExpeditePendingSwapOutRequest, postCheckArgs]{
		{
			name:  "happy path",
			setup: setup,
			msg: types.MsgExpeditePendingSwapOutRequest{
				Authority: admin.String(),
				RequestId: id,
			},
			postCheckArgs: postCheckArgs{
				RequestId: id,
			},
			expectedEvents: sdk.Events{
				sdk.NewEvent(
					"provlabs.vault.v1.EventPendingSwapOutExpedited",
					sdk.NewAttribute("authority", admin.String()),
					sdk.NewAttribute("request_id", fmt.Sprintf("%d", id)),
					sdk.NewAttribute("vault", vaultAddr.String()),
				),
			},
		},
		{
			name:  "unauthorized authority",
			setup: setup,
			msg: types.MsgExpeditePendingSwapOutRequest{
				Authority: other.String(),
				RequestId: id,
			},
			expectedErrSubstrs: []string{"unauthorized"},
		},
		{
			name:  "request id does not exist",
			setup: setup,
			msg: types.MsgExpeditePendingSwapOutRequest{
				Authority: admin.String(),
				RequestId: 999,
			},
			expectedErrSubstrs: []string{"failed to get pending swap out"},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			testDef.expectedResponse = &types.MsgExpeditePendingSwapOutResponse{}
			runMsgServerTestCase(s, testDef, tc)
		})
	}

	s.Run("happy path - asset manager authority", func() {
		assetMgr := s.CreateAndFundAccount(sdk.Coin{Denom: underlying, Amount: math.ZeroInt()})
		setupWithAssetMgr(assetMgr)

		ev := sdk.Events{
			sdk.NewEvent(
				"provlabs.vault.v1.EventPendingSwapOutExpedited",
				sdk.NewAttribute("authority", assetMgr.String()),
				sdk.NewAttribute("request_id", fmt.Sprintf("%d", id)),
				sdk.NewAttribute("vault", vaultAddr.String()),
			),
		}

		tc := msgServerTestCase[types.MsgExpeditePendingSwapOutRequest, postCheckArgs]{
			name: "happy path - asset manager authority",
			msg: types.MsgExpeditePendingSwapOutRequest{
				Authority: assetMgr.String(),
				RequestId: id,
			},
			postCheckArgs: postCheckArgs{
				RequestId: id,
			},
			expectedEvents: ev,
		}

		testDef.expectedResponse = &types.MsgExpeditePendingSwapOutResponse{}
		runMsgServerTestCase(s, testDef, tc)
	})
}

func (s *TestSuite) TestMsgServer_PauseVault() {
	type postCheckArgs struct {
		VaultAddress         sdk.AccAddress
		ExpectedPaused       bool
		ExpectedPauseDenom   string
		ExpectedPauseAmount  int64
		ExpectedPausedReason string
		ExpectedDesiredRate  string
		ExpectedCurrentRate  string
	}

	testDef := msgServerTestDef[types.MsgPauseVaultRequest, types.MsgPauseVaultResponse, postCheckArgs]{
		endpointName: "PauseVault",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).PauseVault,
		postCheck: func(msg *types.MsgPauseVaultRequest, args postCheckArgs) {
			v, err := s.k.GetVault(s.ctx, args.VaultAddress)
			s.Require().NoError(err, "expected to load vault %s for post-check", args.VaultAddress)
			s.Assert().Equal(args.ExpectedPaused, v.Paused, "vault paused status")
			s.Assert().Equal(args.ExpectedPauseDenom, v.PausedBalance.Denom, "vault paused balance denom")
			s.Assert().Equal(args.ExpectedPauseAmount, v.PausedBalance.Amount.Int64(), "vault paused balance amount")
			s.Assert().Equal(args.ExpectedPausedReason, v.PausedReason, "vault paused reason")
			s.Assert().Equal(args.ExpectedDesiredRate, v.DesiredInterestRate, "vault desired interest rate")
			s.Assert().Equal(args.ExpectedCurrentRate, v.CurrentInterestRate, "vault current interest rate")
		},
	}

	underlying := "under"
	share := "vaultshares"
	admin := s.adminAddr
	reason := "maintenance"
	interestRate := "0.0406"
	vaultAddr := types.GetVaultAddress(share)

	setup := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlying, math.NewInt(10_000)), admin)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           admin.String(),
			ShareDenom:      share,
			UnderlyingAsset: underlying,
		})
		s.Require().NoError(err, "failed to create vault for share denom %s", share)
		vault, err := s.k.GetVault(s.ctx, vaultAddr)
		s.Require().NoError(err, "failed to get vault in setup")
		s.Require().NoError(s.k.UpdateInterestRates(s.ctx, vault, interestRate, interestRate), "initial interest rate update should succeed")
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	}

	setupWithAssetMgr := func(assetMgr sdk.AccAddress) {
		setup()
		_, err := keeper.NewMsgServer(s.simApp.VaultKeeper).SetAssetManager(s.ctx, &types.MsgSetAssetManagerRequest{
			Admin:        admin.String(),
			VaultAddress: vaultAddr.String(),
			AssetManager: assetMgr.String(),
		})
		s.Require().NoError(err, "failed to set asset manager in setup")
	}

	s.Run("happy path - admin pause vault", func() {
		ev := sdk.Events{
			sdk.NewEvent(
				"provlabs.vault.v1.EventVaultInterestChange",
				sdk.NewAttribute("current_rate", types.ZeroInterestRate),
				sdk.NewAttribute("desired_rate", interestRate),
				sdk.NewAttribute("vault_address", vaultAddr.String()),
			),
			sdk.NewEvent(
				"provlabs.vault.v1.EventVaultPaused",
				sdk.NewAttribute("authority", admin.String()),
				sdk.NewAttribute("reason", reason),
				sdk.NewAttribute("total_vault_value", sdk.NewInt64Coin(underlying, 0).String()),
				sdk.NewAttribute("vault_address", vaultAddr.String()),
			),
		}

		tc := msgServerTestCase[types.MsgPauseVaultRequest, postCheckArgs]{
			name:  "happy path - admin pause",
			setup: setup,
			msg: types.MsgPauseVaultRequest{
				Authority:    admin.String(),
				VaultAddress: vaultAddr.String(),
				Reason:       reason,
			},
			postCheckArgs: postCheckArgs{
				VaultAddress:         vaultAddr,
				ExpectedPaused:       true,
				ExpectedPauseDenom:   underlying,
				ExpectedPauseAmount:  0,
				ExpectedPausedReason: reason,
				ExpectedDesiredRate:  interestRate,
				ExpectedCurrentRate:  types.ZeroInterestRate,
			},
			expectedEvents: ev,
		}

		testDef.expectedResponse = &types.MsgPauseVaultResponse{}
		runMsgServerTestCase(s, testDef, tc)
	})

	s.Run("happy path - asset manager pause vault", func() {
		assetMgr := s.CreateAndFundAccount(sdk.NewInt64Coin(underlying, 100))
		setupWithAssetMgr(assetMgr)

		ev := sdk.Events{
			sdk.NewEvent(
				"provlabs.vault.v1.EventVaultInterestChange",
				sdk.NewAttribute("current_rate", types.ZeroInterestRate),
				sdk.NewAttribute("desired_rate", interestRate),
				sdk.NewAttribute("vault_address", vaultAddr.String()),
			),
			sdk.NewEvent(
				"provlabs.vault.v1.EventVaultPaused",
				sdk.NewAttribute("authority", assetMgr.String()),
				sdk.NewAttribute("reason", reason),
				sdk.NewAttribute("total_vault_value", sdk.NewInt64Coin(underlying, 0).String()),
				sdk.NewAttribute("vault_address", vaultAddr.String()),
			),
		}

		tc := msgServerTestCase[types.MsgPauseVaultRequest, postCheckArgs]{
			name:  "happy path - asset manager pause",
			setup: func() {},
			msg: types.MsgPauseVaultRequest{
				Authority:    assetMgr.String(),
				VaultAddress: vaultAddr.String(),
				Reason:       reason,
			},
			postCheckArgs: postCheckArgs{
				VaultAddress:         vaultAddr,
				ExpectedPaused:       true,
				ExpectedPauseDenom:   underlying,
				ExpectedPauseAmount:  0,
				ExpectedPausedReason: reason,
				ExpectedDesiredRate:  interestRate,
				ExpectedCurrentRate:  types.ZeroInterestRate,
			},
			expectedEvents: ev,
		}

		testDef.expectedResponse = &types.MsgPauseVaultResponse{}
		runMsgServerTestCase(s, testDef, tc)
	})
}

func (s *TestSuite) TestMsgServer_PauseVault_Failures() {
	testDef := msgServerTestDef[types.MsgPauseVaultRequest, types.MsgPauseVaultResponse, any]{
		endpointName: "PauseVault",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).PauseVault,
		postCheck:    nil,
	}

	underlying := "under"
	share := "vaultshares"
	admin := s.adminAddr
	other := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1000))
	vaultAddr := types.GetVaultAddress(share)
	markerAddr := markertypes.MustGetMarkerAddress(share)

	base := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlying, math.NewInt(10_000)), admin)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           admin.String(),
			ShareDenom:      share,
			UnderlyingAsset: underlying,
		})
		s.Require().NoError(err, "expected base vault creation to succeed")
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	}

	tests := []msgServerTestCase[types.MsgPauseVaultRequest, any]{
		{
			name: "vault not found",
			msg: types.MsgPauseVaultRequest{
				Authority:    admin.String(),
				VaultAddress: types.GetVaultAddress("nope").String(),
				Reason:       "x",
			},
			expectedErrSubstrs: []string{"not found"},
		},
		{
			name:  "invalid vault address (not a vault account)",
			setup: base,
			msg: types.MsgPauseVaultRequest{
				Authority:    admin.String(),
				VaultAddress: markerAddr.String(),
				Reason:       "x",
			},
			expectedErrSubstrs: []string{"failed to get vault", "is not a vault account"},
		},
		{
			name:  "unauthorized admin",
			setup: base,
			msg: types.MsgPauseVaultRequest{
				Authority:    other.String(),
				VaultAddress: vaultAddr.String(),
				Reason:       "x",
			},
			expectedErrSubstrs: []string{"unauthorized authority"},
		},
		{
			name: "already paused",
			setup: func() {
				base()
				_, err := keeper.NewMsgServer(s.simApp.VaultKeeper).PauseVault(s.ctx, &types.MsgPauseVaultRequest{
					Authority:    admin.String(),
					VaultAddress: vaultAddr.String(),
					Reason:       "first",
				})
				s.Require().NoError(err, "expected initial pause to succeed")
				s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
			},
			msg: types.MsgPauseVaultRequest{
				Authority:    admin.String(),
				VaultAddress: vaultAddr.String(),
				Reason:       "second",
			},
			expectedErrSubstrs: []string{"already paused"},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			runMsgServerTestCase(s, testDef, tc)
		})
	}
}

func (s *TestSuite) TestMsgServer_UnpauseVault() {
	type postCheckArgs struct {
		VaultAddress          sdk.AccAddress
		ExpectedPaused        bool
		ExpectedEmptyDenom    string
		ExpectedEmptyAmount   int64
		ExpectedDesiredRate   string
		ExpectedCurrentRate   string
		ExpectedPeriodStart   int64
		ExpectedPeriodTimeout int64
	}

	testDef := msgServerTestDef[types.MsgUnpauseVaultRequest, types.MsgUnpauseVaultResponse, postCheckArgs]{
		endpointName: "UnpauseVault",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).UnpauseVault,
		postCheck: func(msg *types.MsgUnpauseVaultRequest, args postCheckArgs) {
			vault, err := s.k.GetVault(s.ctx, args.VaultAddress)
			s.Require().NoError(err, "expected to load vault %s for post-check", args.VaultAddress)
			s.Assert().Equal(args.ExpectedPaused, vault.Paused, "vault paused status")
			s.Assert().Equal(args.ExpectedEmptyDenom, vault.PausedBalance.Denom, "vault paused balance denom")
			s.Assert().Equal(args.ExpectedEmptyAmount, vault.PausedBalance.Amount.Int64(), "vault paused balance amount")
			s.Assert().Empty(vault.PausedReason, "vault paused reason")
			s.Assert().Equal(args.ExpectedDesiredRate, vault.DesiredInterestRate, "vault desired interest rate")
			s.Assert().Equal(args.ExpectedCurrentRate, vault.CurrentInterestRate, "vault current interest rate")
			s.Assert().Equal(args.ExpectedPeriodStart, vault.PeriodStart, "vault interest period start")
			s.Assert().Equal(args.ExpectedPeriodTimeout, vault.PeriodTimeout, "vault interest period timeout")
			found, err := s.k.PayoutVerificationSet.Has(s.ctx, args.VaultAddress)
			s.Require().NoError(err, "error checking payout verification set for vault %s", args.VaultAddress)
			s.Assert().True(found, "vault should be in payout verification set")
		},
	}

	underlying := "under"
	share := "vaultshares"
	admin := s.adminAddr
	vaultAddr := types.GetVaultAddress(share)
	interestRate := "0.0406"
	now := time.Now()

	setup := func() {
		s.ctx = s.ctx.WithBlockTime(now)
		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlying, math.NewInt(10_000)), admin)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           admin.String(),
			ShareDenom:      share,
			UnderlyingAsset: underlying,
		})
		s.Require().NoError(err, "failed to create vault for share denom %s", share)
		vault, err := s.k.GetVault(s.ctx, vaultAddr)
		s.Require().NoError(err, "failed to get vault in setup")
		s.Require().NoError(s.k.UpdateInterestRates(s.ctx, vault, interestRate, interestRate), "initial interest rate update should succeed")
		vault.PeriodStart = s.ctx.BlockTime().Unix()
		vault.PeriodTimeout = s.ctx.BlockTime().Add(24 * time.Hour).Unix()
		err = s.k.SetVaultAccount(s.ctx, vault)
		s.Require().NoError(err, "failed to set vault account in setup")
		_, err = keeper.NewMsgServer(s.simApp.VaultKeeper).PauseVault(s.ctx, &types.MsgPauseVaultRequest{
			Authority:    admin.String(),
			VaultAddress: vaultAddr.String(),
			Reason:       "maintenance",
		})
		s.Require().NoError(err, "expected pause to succeed in setup")
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	}

	setupWithAssetMgr := func(assetMgr sdk.AccAddress) {
		setup()
		_, err := keeper.NewMsgServer(s.simApp.VaultKeeper).SetAssetManager(s.ctx, &types.MsgSetAssetManagerRequest{
			Admin:        admin.String(),
			VaultAddress: vaultAddr.String(),
			AssetManager: assetMgr.String(),
		})
		s.Require().NoError(err, "failed to set asset manager in setup")
	}

	s.Run("happy path - admin unpause vault", func() {
		ev := sdk.Events{
			sdk.NewEvent(
				"provlabs.vault.v1.EventVaultInterestChange",
				sdk.NewAttribute("current_rate", interestRate),
				sdk.NewAttribute("desired_rate", interestRate),
				sdk.NewAttribute("vault_address", vaultAddr.String()),
			),
			sdk.NewEvent(
				"provlabs.vault.v1.EventVaultUnpaused",
				sdk.NewAttribute("authority", admin.String()),
				sdk.NewAttribute("total_vault_value", sdk.NewInt64Coin(underlying, 0).String()),
				sdk.NewAttribute("vault_address", vaultAddr.String()),
			),
		}

		tc := msgServerTestCase[types.MsgUnpauseVaultRequest, postCheckArgs]{
			name:  "happy path - admin unpause",
			setup: setup,
			msg: types.MsgUnpauseVaultRequest{
				Authority:    admin.String(),
				VaultAddress: vaultAddr.String(),
			},
			postCheckArgs: postCheckArgs{
				VaultAddress:          vaultAddr,
				ExpectedPaused:        false,
				ExpectedEmptyDenom:    "",
				ExpectedEmptyAmount:   0,
				ExpectedDesiredRate:   interestRate,
				ExpectedCurrentRate:   interestRate,
				ExpectedPeriodStart:   now.Unix(),
				ExpectedPeriodTimeout: 0,
			},
			expectedEvents: ev,
		}

		testDef.expectedResponse = &types.MsgUnpauseVaultResponse{}
		runMsgServerTestCase(s, testDef, tc)
	})

	s.Run("happy path - asset manager unpause vault", func() {
		assetMgr := s.CreateAndFundAccount(sdk.NewInt64Coin(underlying, 100))
		setupWithAssetMgr(assetMgr)

		ev := sdk.Events{
			sdk.NewEvent(
				"provlabs.vault.v1.EventVaultInterestChange",
				sdk.NewAttribute("current_rate", interestRate),
				sdk.NewAttribute("desired_rate", interestRate),
				sdk.NewAttribute("vault_address", vaultAddr.String()),
			),
			sdk.NewEvent(
				"provlabs.vault.v1.EventVaultUnpaused",
				sdk.NewAttribute("authority", assetMgr.String()),
				sdk.NewAttribute("total_vault_value", sdk.NewInt64Coin(underlying, 0).String()),
				sdk.NewAttribute("vault_address", vaultAddr.String()),
			),
		}

		tc := msgServerTestCase[types.MsgUnpauseVaultRequest, postCheckArgs]{
			name:  "happy path - asset manager unpause",
			setup: func() {},
			msg: types.MsgUnpauseVaultRequest{
				Authority:    assetMgr.String(),
				VaultAddress: vaultAddr.String(),
			},
			postCheckArgs: postCheckArgs{
				VaultAddress:          vaultAddr,
				ExpectedPaused:        false,
				ExpectedEmptyDenom:    "",
				ExpectedEmptyAmount:   0,
				ExpectedCurrentRate:   interestRate,
				ExpectedDesiredRate:   interestRate,
				ExpectedPeriodStart:   now.Unix(),
				ExpectedPeriodTimeout: 0,
			},
			expectedEvents: ev,
		}

		testDef.expectedResponse = &types.MsgUnpauseVaultResponse{}
		runMsgServerTestCase(s, testDef, tc)
	})
}

func (s *TestSuite) TestMsgServer_UnpauseVault_Failures() {
	testDef := msgServerTestDef[types.MsgUnpauseVaultRequest, types.MsgUnpauseVaultResponse, any]{
		endpointName: "UnpauseVault",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).UnpauseVault,
		postCheck:    nil,
	}

	underlying := "under"
	share := "vaultshares"
	admin := s.adminAddr
	other := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1000))
	vaultAddr := types.GetVaultAddress(share)
	markerAddr := markertypes.MustGetMarkerAddress(share)

	base := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlying, math.NewInt(10_000)), admin)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           admin.String(),
			ShareDenom:      share,
			UnderlyingAsset: underlying,
		})
		s.Require().NoError(err, "expected base vault creation to succeed")
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	}

	paused := func() {
		base()
		_, err := keeper.NewMsgServer(s.simApp.VaultKeeper).PauseVault(s.ctx, &types.MsgPauseVaultRequest{
			Authority:    admin.String(),
			VaultAddress: vaultAddr.String(),
			Reason:       "maintenance",
		})
		s.Require().NoError(err, "expected pause to succeed for paused-state tests")
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	}

	tests := []msgServerTestCase[types.MsgUnpauseVaultRequest, any]{
		{
			name: "vault not found",
			msg: types.MsgUnpauseVaultRequest{
				Authority:    admin.String(),
				VaultAddress: types.GetVaultAddress("missing").String(),
			},
			expectedErrSubstrs: []string{"not found"},
		},
		{
			name:  "invalid vault address (not a vault account)",
			setup: base,
			msg: types.MsgUnpauseVaultRequest{
				Authority:    admin.String(),
				VaultAddress: markerAddr.String(),
			},
			expectedErrSubstrs: []string{"failed to get vault", "is not a vault account"},
		},
		{
			name:  "unauthorized admin",
			setup: paused,
			msg: types.MsgUnpauseVaultRequest{
				Authority:    other.String(),
				VaultAddress: vaultAddr.String(),
			},
			expectedErrSubstrs: []string{"unauthorized authority"},
		},
		{
			name:  "not paused",
			setup: base,
			msg: types.MsgUnpauseVaultRequest{
				Authority:    admin.String(),
				VaultAddress: vaultAddr.String(),
			},
			expectedErrSubstrs: []string{"is not paused"},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			runMsgServerTestCase(s, testDef, tc)
		})
	}
}

func (s *TestSuite) TestMsgServer_SetBridgeAddress() {
	type postCheckArgs struct {
		VaultAddress   sdk.AccAddress
		ExpectedBridge string
	}

	testDef := msgServerTestDef[types.MsgSetBridgeAddressRequest, types.MsgSetBridgeAddressResponse, postCheckArgs]{
		endpointName: "SetBridgeAddress",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).SetBridgeAddress,
		postCheck: func(msg *types.MsgSetBridgeAddressRequest, args postCheckArgs) {
			v, err := s.k.GetVault(s.ctx, args.VaultAddress)
			s.Require().NoError(err, "post-check: should load vault for verification")
			s.Assert().Equal(args.ExpectedBridge, v.BridgeAddress, "post-check: expected vault BridgeAddress to match")
		},
	}

	underlying := "under"
	share := "vaultshares"
	admin := s.adminAddr
	bridge := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1))
	vaultAddr := types.GetVaultAddress(share)

	setup := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlying, math.NewInt(1000)), admin)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           admin.String(),
			ShareDenom:      share,
			UnderlyingAsset: underlying,
		})
		s.Require().NoError(err, "setup: expected vault creation to succeed")
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	}

	tc := msgServerTestCase[types.MsgSetBridgeAddressRequest, postCheckArgs]{
		name:  "happy path",
		setup: setup,
		msg: types.MsgSetBridgeAddressRequest{
			Admin:         admin.String(),
			VaultAddress:  vaultAddr.String(),
			BridgeAddress: bridge.String(),
		},
		postCheckArgs: postCheckArgs{
			VaultAddress:   vaultAddr,
			ExpectedBridge: bridge.String(),
		},
		expectedEvents: sdk.Events{
			sdk.NewEvent("provlabs.vault.v1.EventBridgeAddressSet",
				sdk.NewAttribute("admin", admin.String()),
				sdk.NewAttribute("bridge_address", bridge.String()),
				sdk.NewAttribute("vault_address", vaultAddr.String()),
			),
		},
	}

	testDef.expectedResponse = &types.MsgSetBridgeAddressResponse{}
	runMsgServerTestCase(s, testDef, tc)
}

func (s *TestSuite) TestMsgServer_SetBridgeAddress_Failures() {
	testDef := msgServerTestDef[types.MsgSetBridgeAddressRequest, types.MsgSetBridgeAddressResponse, any]{
		endpointName: "SetBridgeAddress",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).SetBridgeAddress,
		postCheck:    nil,
	}

	underlying := "under"
	share := "vaultshares"
	admin := s.adminAddr
	other := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1))
	bridge := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1))
	vaultAddr := types.GetVaultAddress(share)
	markerAddr := markertypes.MustGetMarkerAddress(share)

	base := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlying, math.NewInt(1000)), admin)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           admin.String(),
			ShareDenom:      share,
			UnderlyingAsset: underlying,
		})
		s.Require().NoError(err, "base: expected vault creation to succeed")
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	}

	tests := []msgServerTestCase[types.MsgSetBridgeAddressRequest, any]{
		{
			name: "vault not found",
			setup: func() {
				s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
			},
			msg: types.MsgSetBridgeAddressRequest{
				Admin:         admin.String(),
				VaultAddress:  types.GetVaultAddress("missing").String(),
				BridgeAddress: bridge.String(),
			},
			expectedErrSubstrs: []string{"vault not found"},
		},
		{
			name:  "invalid vault address (not a vault account)",
			setup: base,
			msg: types.MsgSetBridgeAddressRequest{
				Admin:         admin.String(),
				VaultAddress:  markerAddr.String(),
				BridgeAddress: bridge.String(),
			},
			expectedErrSubstrs: []string{"failed to get vault", "is not a vault account"},
		},
		{
			name:  "unauthorized admin",
			setup: base,
			msg: types.MsgSetBridgeAddressRequest{
				Admin:         other.String(),
				VaultAddress:  vaultAddr.String(),
				BridgeAddress: bridge.String(),
			},
			expectedErrSubstrs: []string{"unauthorized", "is not the vault admin"},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			runMsgServerTestCase(s, testDef, tc)
		})
	}
}

func (s *TestSuite) TestMsgServer_ToggleBridge() {
	type postCheckArgs struct {
		VaultAddress    sdk.AccAddress
		ExpectedEnabled bool
	}

	testDef := msgServerTestDef[types.MsgToggleBridgeRequest, types.MsgToggleBridgeResponse, postCheckArgs]{
		endpointName: "ToggleBridge",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).ToggleBridge,
		postCheck: func(msg *types.MsgToggleBridgeRequest, args postCheckArgs) {
			v, err := s.k.GetVault(s.ctx, args.VaultAddress)
			s.Require().NoError(err, "post-check: should load vault for verification")
			s.Assert().Equal(args.ExpectedEnabled, v.BridgeEnabled, "post-check: expected BridgeEnabled to match")
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
		s.Require().NoError(err, "setup: expected vault creation to succeed")
		vault, err := s.k.GetVault(s.ctx, vaultAddr)
		s.Require().NoError(err, "setup: should load vault")
		vault.BridgeAddress = s.adminAddr.String()
		s.k.AuthKeeper.SetAccount(s.ctx, vault)
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())

	}

	tc := msgServerTestCase[types.MsgToggleBridgeRequest, postCheckArgs]{
		name:  "happy path - enable bridge",
		setup: setup,
		msg: types.MsgToggleBridgeRequest{
			Admin:        admin.String(),
			VaultAddress: vaultAddr.String(),
			Enabled:      true,
		},
		postCheckArgs: postCheckArgs{
			VaultAddress:    vaultAddr,
			ExpectedEnabled: true,
		},
		expectedEvents: sdk.Events{
			sdk.NewEvent("provlabs.vault.v1.EventBridgeToggled",
				sdk.NewAttribute("admin", admin.String()),
				sdk.NewAttribute("enabled", "true"),
				sdk.NewAttribute("vault_address", vaultAddr.String()),
			),
		},
	}

	testDef.expectedResponse = &types.MsgToggleBridgeResponse{}
	runMsgServerTestCase(s, testDef, tc)
}

func (s *TestSuite) TestMsgServer_ToggleBridge_Failures() {
	testDef := msgServerTestDef[types.MsgToggleBridgeRequest, types.MsgToggleBridgeResponse, any]{
		endpointName: "ToggleBridge",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).ToggleBridge,
		postCheck:    nil,
	}

	underlying := "under"
	share := "vaultshares"
	admin := s.adminAddr
	other := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1))
	vaultAddr := types.GetVaultAddress(share)
	markerAddr := markertypes.MustGetMarkerAddress(share)

	base := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlying, math.NewInt(1000)), admin)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           admin.String(),
			ShareDenom:      share,
			UnderlyingAsset: underlying,
		})
		s.Require().NoError(err, "base: expected vault creation to succeed")
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	}

	tests := []msgServerTestCase[types.MsgToggleBridgeRequest, any]{
		{
			name: "vault not found",
			setup: func() {
				s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
			},
			msg: types.MsgToggleBridgeRequest{
				Admin:        admin.String(),
				VaultAddress: types.GetVaultAddress("missing").String(),
				Enabled:      true,
			},
			expectedErrSubstrs: []string{"not found"},
		},
		{
			name:  "invalid vault address (not a vault account)",
			setup: base,
			msg: types.MsgToggleBridgeRequest{
				Admin:        admin.String(),
				VaultAddress: markerAddr.String(),
				Enabled:      true,
			},
			expectedErrSubstrs: []string{"failed to get vault", "is not a vault account"},
		},
		{
			name:  "unauthorized admin",
			setup: base,
			msg: types.MsgToggleBridgeRequest{
				Admin:        other.String(),
				VaultAddress: vaultAddr.String(),
				Enabled:      true,
			},
			expectedErrSubstrs: []string{"unauthorized", "is not the vault admin"},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			runMsgServerTestCase(s, testDef, tc)
		})
	}
}

func (s *TestSuite) TestMsgServer_BridgeMintShares() {
	type postCheckArgs struct {
		BridgeAddress  sdk.AccAddress
		VaultAddress   sdk.AccAddress
		MintedShares   sdk.Coin
		ShareDenom     string
		ExpectedSupply math.Int
	}

	testDef := msgServerTestDef[types.MsgBridgeMintSharesRequest, types.MsgBridgeMintSharesResponse, postCheckArgs]{
		endpointName: "BridgeMintShares",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).BridgeMintShares,
		postCheck: func(msg *types.MsgBridgeMintSharesRequest, args postCheckArgs) {
			supply := s.k.BankKeeper.GetSupply(s.ctx, args.ShareDenom)
			s.Assert().Equal(args.ExpectedSupply.String(), supply.Amount.String(), "supply after bridge mint should equal minted amount")
			bal := s.k.BankKeeper.GetBalance(s.ctx, args.BridgeAddress, args.ShareDenom)
			s.Assert().Equal(args.MintedShares, bal, "bridge account balance should equal minted amount")
		},
	}

	underlying := "under"
	share := "vaultshares"
	bridge := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1))
	vaultAddr := types.GetVaultAddress(share)
	minted := sdk.NewInt64Coin(share, 250_000_000)

	setup := func() {
		s.setupBridgeVault(underlying, share, bridge, math.NewInt(10_000_000_000))
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	}

	tc := msgServerTestCase[types.MsgBridgeMintSharesRequest, postCheckArgs]{
		name:  "happy path",
		setup: setup,
		msg: types.MsgBridgeMintSharesRequest{
			VaultAddress: vaultAddr.String(),
			Bridge:       bridge.String(),
			Shares:       minted,
		},
		postCheckArgs: postCheckArgs{
			BridgeAddress:  bridge,
			VaultAddress:   vaultAddr,
			MintedShares:   minted,
			ShareDenom:     share,
			ExpectedSupply: minted.Amount,
		},
		expectedEvents: createBridgeMintSharesEventsExact(vaultAddr, bridge, share, minted),
	}

	testDef.expectedResponse = &types.MsgBridgeMintSharesResponse{}
	runMsgServerTestCase(s, testDef, tc)
}

func (s *TestSuite) TestMsgServer_BridgeMintShares_SupplyAndBalances() {
	type postCheckArgs struct {
		VaultAddress  sdk.AccAddress
		BridgeAddress sdk.AccAddress
		Minted        sdk.Coin
	}

	testDef := msgServerTestDef[types.MsgBridgeMintSharesRequest, types.MsgBridgeMintSharesResponse, postCheckArgs]{
		endpointName: "BridgeMintShares",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).BridgeMintShares,
		postCheck: func(msg *types.MsgBridgeMintSharesRequest, args postCheckArgs) {
			supply := s.k.BankKeeper.GetSupply(s.ctx, args.Minted.Denom)
			s.Assert().True(supply.Amount.Equal(args.Minted.Amount), "supply check should equal minted amount")
			b := s.k.BankKeeper.GetBalance(s.ctx, args.BridgeAddress, args.Minted.Denom)
			s.Assert().Equal(args.Minted, b, "bridge balance check should equal minted amount")
		},
	}

	underlying := "underx"
	share := "vaultsharex"
	bridge := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1))
	vaultAddr := types.GetVaultAddress(share)
	minted := sdk.NewInt64Coin(share, 1_000)

	setup := func() {
		s.setupBridgeVault(underlying, share, bridge, math.NewInt(9_999_999))
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	}

	tc := msgServerTestCase[types.MsgBridgeMintSharesRequest, postCheckArgs]{
		name:  "supply and balance update",
		setup: setup,
		msg: types.MsgBridgeMintSharesRequest{
			VaultAddress: vaultAddr.String(),
			Bridge:       bridge.String(),
			Shares:       minted,
		},
		postCheckArgs: postCheckArgs{
			VaultAddress:  vaultAddr,
			BridgeAddress: bridge,
			Minted:        minted,
		},
		expectedEvents: createBridgeMintSharesEventsExact(vaultAddr, bridge, share, minted),
	}

	testDef.expectedResponse = &types.MsgBridgeMintSharesResponse{}
	runMsgServerTestCase(s, testDef, tc)
}

func (s *TestSuite) TestMsgServer_BridgeMintShares_ErrorMessages() {
	testDef := msgServerTestDef[types.MsgBridgeMintSharesRequest, types.MsgBridgeMintSharesResponse, any]{
		endpointName: "BridgeMintShares",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).BridgeMintShares,
		postCheck:    nil,
	}

	underlying := "erru"
	share := "errshare"
	bridge := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1))
	vaultAddr := types.GetVaultAddress(share)

	setup := func() {
		s.setupBridgeVault(underlying, share, bridge, math.NewInt(100))
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	}

	tc := msgServerTestCase[types.MsgBridgeMintSharesRequest, any]{
		name:  "capacity message content",
		setup: setup,
		msg: types.MsgBridgeMintSharesRequest{
			VaultAddress: vaultAddr.String(),
			Bridge:       bridge.String(),
			Shares:       sdk.NewInt64Coin(share, 101),
		},
		expectedErrSubstrs: []string{
			"mint exceeds capacity",
			fmt.Sprintf("requested %d", 101),
			fmt.Sprintf("available %d", 100),
		},
	}
	runMsgServerTestCase(s, testDef, tc)
}

func (s *TestSuite) TestMsgServer_BridgeBurnShares() {
	type postCheckArgs struct {
		VaultAddr  sdk.AccAddress
		BridgeAddr sdk.AccAddress
		Shares     sdk.Coin
	}

	testDef := msgServerTestDef[types.MsgBridgeBurnSharesRequest, types.MsgBridgeBurnSharesResponse, postCheckArgs]{
		endpointName: "BridgeBurnShares",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).BridgeBurnShares,
		postCheck: func(msg *types.MsgBridgeBurnSharesRequest, args postCheckArgs) {
			supply := s.simApp.BankKeeper.GetSupply(s.ctx, args.Shares.Denom)
			s.Assert().Equal(int64(0), supply.Amount.Int64(), "expected total supply to be zero after burn")
			s.assertBalance(args.BridgeAddr, args.Shares.Denom, sdkmath.ZeroInt())
		},
	}

	underlying := "under"
	share := "vaultshares"
	vaultAddr := types.GetVaultAddress(share)
	markerAddr := markertypes.MustGetMarkerAddress(share)
	bridgeAddr := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1))
	burn := sdk.NewInt64Coin(share, 100_000_000)

	setup := func() {
		s.setupBridgeVault(underlying, share, bridgeAddr, math.ZeroInt())

		err := s.k.MarkerKeeper.MintCoin(s.ctx, vaultAddr, burn)
		s.Require().NoError(err, "expected marker mint to succeed")
		err = s.k.MarkerKeeper.WithdrawCoins(s.ctx, vaultAddr, bridgeAddr, share, sdk.NewCoins(burn))
		s.Require().NoError(err, "expected marker withdraw to bridge to succeed")

		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	}

	events := createSendCoinEvents(bridgeAddr.String(), markerAddr.String(), sdk.NewCoins(burn).String())
	events = append(events, createMarkerBurn(vaultAddr, markerAddr, burn)...)
	events = append(events, sdk.NewEvent("provlabs.vault.v1.EventBridgeBurnShares",
		sdk.NewAttribute("bridge", bridgeAddr.String()),
		sdk.NewAttribute("shares", burn.String()),
		sdk.NewAttribute("vault_address", vaultAddr.String()),
	))

	tc := msgServerTestCase[types.MsgBridgeBurnSharesRequest, postCheckArgs]{
		name:  "happy path",
		setup: setup,
		msg: types.MsgBridgeBurnSharesRequest{
			VaultAddress: vaultAddr.String(),
			Bridge:       bridgeAddr.String(),
			Shares:       burn,
		},
		postCheckArgs:  postCheckArgs{VaultAddr: vaultAddr, BridgeAddr: bridgeAddr, Shares: burn},
		expectedEvents: events,
	}

	testDef.expectedResponse = &types.MsgBridgeBurnSharesResponse{}
	runMsgServerTestCase(s, testDef, tc)
}

func (s *TestSuite) TestMsgServer_BridgeBurnShares_Failures() {
	testDef := msgServerTestDef[types.MsgBridgeBurnSharesRequest, types.MsgBridgeBurnSharesResponse, any]{
		endpointName: "BridgeBurnShares",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).BridgeBurnShares,
		postCheck:    nil,
	}

	underlying := "under"
	share := "vaultshares"
	admin := s.adminAddr
	vaultAddr := types.GetVaultAddress(share)
	bridgeAddr := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1))
	otherBridge := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1))
	burn := sdk.NewInt64Coin(share, 100_000_000)

	base := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlying, 2_000_000), admin)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           admin.String(),
			ShareDenom:      share,
			UnderlyingAsset: underlying,
		})
		s.Require().NoError(err, "expected vault creation to succeed")
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	}

	enabled := func() {
		s.setupBridgeVault(underlying, share, bridgeAddr, math.ZeroInt())
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	}

	enabledWithBridgeFunds := func() {
		enabled()
		err := s.k.MarkerKeeper.MintCoin(s.ctx, vaultAddr, burn)
		s.Require().NoError(err, "expected marker mint to succeed")
		err = s.k.MarkerKeeper.WithdrawCoins(s.ctx, vaultAddr, bridgeAddr, share, sdk.NewCoins(burn))
		s.Require().NoError(err, "expected marker withdraw to bridge to succeed")
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	}

	tests := []msgServerTestCase[types.MsgBridgeBurnSharesRequest, any]{
		{
			name: "vault not found",
			msg: types.MsgBridgeBurnSharesRequest{
				VaultAddress: types.GetVaultAddress("nosuch").String(),
				Bridge:       bridgeAddr.String(),
				Shares:       burn,
			},
			expectedErrSubstrs: []string{"vault not found"},
		},
		{
			name:  "bridge disabled",
			setup: base,
			msg: types.MsgBridgeBurnSharesRequest{
				VaultAddress: vaultAddr.String(),
				Bridge:       bridgeAddr.String(),
				Shares:       burn,
			},
			expectedErrSubstrs: []string{"bridge is disabled"},
		},
		{
			name:  "unauthorized bridge",
			setup: enabled,
			msg: types.MsgBridgeBurnSharesRequest{
				VaultAddress: vaultAddr.String(),
				Bridge:       otherBridge.String(),
				Shares:       burn,
			},
			expectedErrSubstrs: []string{"unauthorized bridge"},
		},
		{
			name:  "invalid shares denom",
			setup: enabledWithBridgeFunds,
			msg: types.MsgBridgeBurnSharesRequest{
				VaultAddress: vaultAddr.String(),
				Bridge:       bridgeAddr.String(),
				Shares:       sdk.NewInt64Coin("wrongdenom", 10),
			},
			expectedErrSubstrs: []string{"invalid shares denom"},
		},
		{
			name:  "non-positive amount",
			setup: enabledWithBridgeFunds,
			msg: types.MsgBridgeBurnSharesRequest{
				VaultAddress: vaultAddr.String(),
				Bridge:       bridgeAddr.String(),
				Shares:       sdk.NewInt64Coin(share, 0),
			},
			expectedErrSubstrs: []string{"burn amount must be positive"},
		},
		{
			name:  "insufficient bridge balance",
			setup: enabled,
			msg: types.MsgBridgeBurnSharesRequest{
				VaultAddress: vaultAddr.String(),
				Bridge:       bridgeAddr.String(),
				Shares:       burn,
			},
			expectedErrSubstrs: []string{"failed to transfer shares from bridge to vault", "insufficient funds"},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			runMsgServerTestCase(s, testDef, tc)
		})
	}
}

// TestMsgServer_BridgeShares_MintBurnMintCycle_TotalSharesInvariant pins the bridge supply-of-record
// model: bridge mint/burn move the local/remote split of an existing cross-chain supply and never
// mutate vault.TotalShares, so a burn re-widens the mint capacity (TotalShares - local_supply) by the
// burned amount while local supply can never exceed TotalShares. See spec/01_concepts.md.
func (s *TestSuite) TestMsgServer_BridgeShares_MintBurnMintCycle_TotalSharesInvariant() {
	underlying := "cyclu"
	share := "cycleshare"
	vaultAddr := types.GetVaultAddress(share)
	bridgeAddr := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1))
	totalShares := math.NewInt(1_000_000)

	msgServer := keeper.NewMsgServer(s.simApp.VaultKeeper)

	s.setupBridgeVault(underlying, share, bridgeAddr, totalShares)

	assertInvariant := func(step string) {
		v, err := s.k.GetVault(s.ctx, vaultAddr)
		s.Require().NoError(err, "%s: expected vault retrieval to succeed", step)
		s.Assert().Equal(totalShares.String(), v.TotalShares.Amount.String(),
			"%s: TotalShares must be invariant across bridge ops", step)
		supply := s.k.BankKeeper.GetSupply(s.ctx, share)
		s.Assert().False(supply.Amount.GT(v.TotalShares.Amount),
			"%s: local supply %s must never exceed TotalShares %s", step, supply.Amount, v.TotalShares.Amount)
	}

	_, err := msgServer.BridgeMintShares(s.ctx, &types.MsgBridgeMintSharesRequest{
		VaultAddress: vaultAddr.String(),
		Bridge:       bridgeAddr.String(),
		Shares:       sdk.NewInt64Coin(share, 600_000),
	})
	s.Require().NoError(err, "first mint within capacity should succeed")
	s.Assert().Equal(int64(600_000), s.k.BankKeeper.GetSupply(s.ctx, share).Amount.Int64(),
		"local supply should reflect first mint")
	s.assertBalance(bridgeAddr, share, math.NewInt(600_000))
	assertInvariant("after first mint")

	_, err = msgServer.BridgeBurnShares(s.ctx, &types.MsgBridgeBurnSharesRequest{
		VaultAddress: vaultAddr.String(),
		Bridge:       bridgeAddr.String(),
		Shares:       sdk.NewInt64Coin(share, 400_000),
	})
	s.Require().NoError(err, "burn within bridge balance should succeed")
	s.Assert().Equal(int64(200_000), s.k.BankKeeper.GetSupply(s.ctx, share).Amount.Int64(),
		"local supply should drop by the burned amount")
	s.assertBalance(bridgeAddr, share, math.NewInt(200_000))
	assertInvariant("after burn")

	_, err = msgServer.BridgeMintShares(s.ctx, &types.MsgBridgeMintSharesRequest{
		VaultAddress: vaultAddr.String(),
		Bridge:       bridgeAddr.String(),
		Shares:       sdk.NewInt64Coin(share, 800_000),
	})
	s.Require().NoError(err, "burn re-widened capacity to 800_000; second mint to the cap should succeed")
	s.Assert().Equal(int64(1_000_000), s.k.BankKeeper.GetSupply(s.ctx, share).Amount.Int64(),
		"local supply should reach TotalShares after minting to the re-widened cap")
	assertInvariant("after second mint")

	_, err = msgServer.BridgeMintShares(s.ctx, &types.MsgBridgeMintSharesRequest{
		VaultAddress: vaultAddr.String(),
		Bridge:       bridgeAddr.String(),
		Shares:       sdk.NewInt64Coin(share, 1),
	})
	s.Require().Error(err, "minting beyond TotalShares must be rejected: local supply can never exceed the supply-of-record")
	s.Assert().Contains(err.Error(), "mint exceeds capacity", "expected capacity rejection")
	assertInvariant("after over-cap mint rejected")
}

func (s *TestSuite) TestMsgServer_SetAssetManager_Failures() {
	testDef := msgServerTestDef[types.MsgSetAssetManagerRequest, types.MsgSetAssetManagerResponse, any]{
		endpointName: "SetAssetManager",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).SetAssetManager,
		postCheck:    nil,
	}

	underlying := "under"
	share := "vaultshares"
	admin := s.adminAddr
	vaultAddr := types.GetVaultAddress(share)
	assetMgr := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1))
	otherAdmin := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1))

	base := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlying, 2_000_000), admin)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           admin.String(),
			ShareDenom:      share,
			UnderlyingAsset: underlying,
		})
		s.Require().NoError(err, "expected vault creation to succeed")
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	}

	tests := []msgServerTestCase[types.MsgSetAssetManagerRequest, any]{
		{
			name: "vault not found",
			msg: types.MsgSetAssetManagerRequest{
				Admin:        admin.String(),
				VaultAddress: types.GetVaultAddress("nosuch").String(),
				AssetManager: assetMgr.String(),
			},
			expectedErrSubstrs: []string{"vault not found"},
		},
		{
			name:  "unauthorized admin",
			setup: base,
			msg: types.MsgSetAssetManagerRequest{
				Admin:        otherAdmin.String(),
				VaultAddress: vaultAddr.String(),
				AssetManager: assetMgr.String(),
			},
			expectedErrSubstrs: []string{"unauthorized"},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			runMsgServerTestCase(s, testDef, tc)
		})
	}
}

func (s *TestSuite) TestMsgServer_UpdateVaultAUMFeeBips() {
	type postCheckArgs struct {
		VaultAddress    sdk.AccAddress
		ExpectedEnabled uint32
	}

	testDef := msgServerTestDef[types.MsgUpdateVaultAUMFeeBipsRequest, types.MsgUpdateVaultAUMFeeBipsResponse, postCheckArgs]{
		endpointName: "UpdateVaultAUMFeeBips",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).UpdateVaultAUMFeeBips,
		postCheck: func(msg *types.MsgUpdateVaultAUMFeeBipsRequest, args postCheckArgs) {
			vault, err := s.k.GetVault(s.ctx, args.VaultAddress)
			s.Require().NoError(err, "failed to get vault at address %s in post-check", args.VaultAddress)
			s.Assert().Equal(args.ExpectedEnabled, vault.AumFeeBips, "AUM fee bips state mismatch for vault %s", args.VaultAddress)
		},
	}

	underlyingDenom := "underlying"
	shareDenom := "vaultshares"
	admin := s.adminAddr
	techFeeAddr := s.EnsureTechFeeAccount()
	otherUser := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1000))
	vaultAddr := types.GetVaultAddress(shareDenom)

	setup := func() {
		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlyingDenom, math.NewInt(1000)), admin)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           admin.String(),
			ShareDenom:      shareDenom,
			UnderlyingAsset: underlyingDenom,
		})
		s.Require().NoError(err, "failed to create vault in UpdateVaultAUMFeeBips setup for share denom %s", shareDenom)
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
		s.ctx = s.ctx.WithBlockTime(time.Now())
	}

	tests := []struct {
		name               string
		setup              func()
		msg                types.MsgUpdateVaultAUMFeeBipsRequest
		postCheckArgs      postCheckArgs
		expectedEvents     sdk.Events
		expectedErrSubstrs []string
	}{
		{
			name:  "happy path - update bips",
			setup: setup,
			msg: types.MsgUpdateVaultAUMFeeBipsRequest{
				Authority:    techFeeAddr.String(),
				VaultAddress: vaultAddr.String(),
				AumFeeBips:   20,
			},
			postCheckArgs: postCheckArgs{
				VaultAddress:    vaultAddr,
				ExpectedEnabled: 20,
			},
			expectedEvents: sdk.Events{
				sdk.NewEvent("provlabs.vault.v1.EventVaultAUMFeeBipsUpdated",
					sdk.NewAttribute("aum_fee_bips", "20"),
					sdk.NewAttribute("authority", techFeeAddr.String()),
					sdk.NewAttribute("vault_address", vaultAddr.String()),
				),
			},
		},
		{
			name:  "failure - vault not found",
			setup: func() { /* no setup */ },
			msg: types.MsgUpdateVaultAUMFeeBipsRequest{
				Authority:    techFeeAddr.String(),
				VaultAddress: vaultAddr.String(),
				AumFeeBips:   20,
			},
			expectedErrSubstrs: []string{"vault not found"},
		},
		{
			name:  "failure - unauthorized authority",
			setup: setup,
			msg: types.MsgUpdateVaultAUMFeeBipsRequest{
				Authority:    otherUser.String(),
				VaultAddress: vaultAddr.String(),
				AumFeeBips:   20,
			},
			expectedErrSubstrs: []string{"unauthorized"},
		},
		{
			name:  "failure - invalid bips",
			setup: setup,
			msg: types.MsgUpdateVaultAUMFeeBipsRequest{
				Authority:    techFeeAddr.String(),
				VaultAddress: vaultAddr.String(),
				AumFeeBips:   10001,
			},
			expectedErrSubstrs: []string{"invalid AUM fee bips"},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			tc := msgServerTestCase[types.MsgUpdateVaultAUMFeeBipsRequest, postCheckArgs]{
				name:               tt.name,
				setup:              tt.setup,
				msg:                tt.msg,
				postCheckArgs:      tt.postCheckArgs,
				expectedEvents:     tt.expectedEvents,
				expectedErrSubstrs: tt.expectedErrSubstrs,
			}

			testDef.expectedResponse = &types.MsgUpdateVaultAUMFeeBipsResponse{}
			runMsgServerTestCase(s, testDef, tc)
		})
	}
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
		s.Require().Errorf(err, "%s error", td.endpointName)
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

// TestMsgServer_UpdateVaultNAV exercises the happy paths of the UpdateVaultNAV
// handler: a first NAV write, an overwrite of an existing entry, and a write
// against a paused vault (NAV updates are administrative and remain allowed).
func (s *TestSuite) TestMsgServer_UpdateVaultNAV() {
	underlying := "under"
	share := "vaultshares"
	navDenom := "rwa"
	admin := s.adminAddr
	vaultAddr := types.GetVaultAddress(share)

	baseSetup := func() {
		s.setupBaseVault(underlying, share)
		s.requireSimpleMarker(navDenom)
		s.ctx = s.ctx.WithBlockHeight(100)
	}

	type postCheckArgs struct {
		denom  string
		price  sdk.Coin
		volume sdkmath.Int
		source string
		height int64
	}

	testDef := msgServerTestDef[types.MsgUpdateVaultNAVRequest, types.MsgUpdateVaultNAVResponse, postCheckArgs]{
		endpointName:     "UpdateVaultNAV",
		endpoint:         keeper.NewMsgServer(s.simApp.VaultKeeper).UpdateVaultNAV,
		expectedResponse: &types.MsgUpdateVaultNAVResponse{},
		postCheck: func(_ *types.MsgUpdateVaultNAVRequest, args postCheckArgs) {
			nav, err := s.k.GetVaultNAV(s.ctx, vaultAddr, args.denom)
			s.Require().NoError(err, "post-check: failed to get NAV for %s", args.denom)
			s.Assert().Equal(args.denom, nav.Denom, "post-check: NAV denom mismatch")
			s.Assert().Equal(args.price, nav.Price, "post-check: NAV price mismatch")
			s.Assert().Equal(args.volume, nav.Volume, "post-check: NAV volume mismatch")
			s.Assert().Equal(args.source, nav.Source, "post-check: NAV source mismatch")
			s.Assert().Equal(args.height, nav.UpdatedBlockHeight, "post-check: NAV block height mismatch")
			s.Assert().Equal(s.ctx.BlockTime().UTC(), nav.UpdatedTime, "post-check: NAV updated time mismatch")
		},
	}

	tests := []struct {
		name   string
		setup  func()
		price  sdk.Coin
		volume sdkmath.Int
		source string
	}{
		{
			name:   "first NAV write",
			setup:  baseSetup,
			price:  sdk.NewInt64Coin(underlying, 100),
			volume: sdkmath.NewInt(1),
			source: "oracle-a",
		},
		{
			name: "overwrite existing NAV",
			setup: func() {
				baseSetup()
				vault, err := s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err, "failed to get vault for overwrite setup")
				seed := types.VaultNAV{Denom: navDenom, Price: sdk.NewInt64Coin(underlying, 50), Volume: sdkmath.NewInt(2)}
				s.Require().NoError(s.k.SetVaultNAV(s.ctx, vault, seed, admin.String()), "failed to seed NAV")
			},
			price:  sdk.NewInt64Coin(underlying, 250),
			volume: sdkmath.NewInt(5),
			source: "oracle-b",
		},
		{
			name: "paused vault still accepts NAV update",
			setup: func() {
				baseSetup()
				vault, err := s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err, "failed to get vault for paused setup")
				vault.Paused = true
				s.k.AuthKeeper.SetAccount(s.ctx, vault)
			},
			price:  sdk.NewInt64Coin(underlying, 300),
			volume: sdkmath.NewInt(3),
			source: "oracle-c",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			tc := msgServerTestCase[types.MsgUpdateVaultNAVRequest, postCheckArgs]{
				name:  tt.name,
				setup: tt.setup,
				msg: types.MsgUpdateVaultNAVRequest{
					Signer:       admin.String(),
					VaultAddress: vaultAddr.String(),
					Denom:        navDenom,
					Price:        tt.price,
					Volume:       tt.volume,
					Source:       tt.source,
				},
				postCheckArgs: postCheckArgs{denom: navDenom, price: tt.price, volume: tt.volume, source: tt.source, height: 100},
				expectedEvents: sdk.Events{
					sdk.NewEvent("provlabs.vault.v1.EventNAVUpdated",
						sdk.NewAttribute("denom", navDenom),
						sdk.NewAttribute("price", tt.price.String()),
						sdk.NewAttribute("signer", admin.String()),
						sdk.NewAttribute("source", tt.source),
						sdk.NewAttribute("updated_block_height", "100"),
						sdk.NewAttribute("vault_address", vaultAddr.String()),
						sdk.NewAttribute("volume", tt.volume.String()),
					),
					sdk.NewEvent("provenance.marker.v1.EventSetNetAssetValue",
						sdk.NewAttribute("denom", navDenom),
						sdk.NewAttribute("price", tt.price.String()),
						sdk.NewAttribute("source", vaultAddr.String()),
						sdk.NewAttribute("volume", tt.volume.String()),
					),
				},
			}
			runMsgServerTestCase(s, testDef, tc)
		})
	}
}

// TestMsgServer_UpdateVaultNAV_Failures covers authority enforcement, denom
// validation, price/volume validation, and the missing-vault path.
func (s *TestSuite) TestMsgServer_UpdateVaultNAV_Failures() {
	underlying := "under"
	share := "vaultshares"
	admin := s.adminAddr
	other := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1000))
	vaultAddr := types.GetVaultAddress(share)

	setup := func() {
		s.setupBaseVault(underlying, share)
	}

	testDef := msgServerTestDef[types.MsgUpdateVaultNAVRequest, types.MsgUpdateVaultNAVResponse, any]{
		endpointName: "UpdateVaultNAV",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).UpdateVaultNAV,
		postCheck:    nil,
	}

	tests := []msgServerTestCase[types.MsgUpdateVaultNAVRequest, any]{
		{
			name:  "vault does not exist",
			setup: func() {},
			msg: types.MsgUpdateVaultNAVRequest{
				Signer:       admin.String(),
				VaultAddress: types.GetVaultAddress("doesnotexist").String(),
				Denom:        "rwa",
				Price:        sdk.NewInt64Coin(underlying, 100),
				Volume:       sdkmath.NewInt(1),
			},
			expectedErrSubstrs: []string{"vault not found"},
		},
		{
			name:  "wrong signer is not the NAV authority",
			setup: setup,
			msg: types.MsgUpdateVaultNAVRequest{
				Signer:       other.String(),
				VaultAddress: vaultAddr.String(),
				Denom:        "rwa",
				Price:        sdk.NewInt64Coin(underlying, 100),
				Volume:       sdkmath.NewInt(1),
			},
			expectedErrSubstrs: []string{"unauthorized", "is not the vault NAV authority"},
		},
		{
			name:  "rejects vault share denom",
			setup: setup,
			msg: types.MsgUpdateVaultNAVRequest{
				Signer:       admin.String(),
				VaultAddress: vaultAddr.String(),
				Denom:        share,
				Price:        sdk.NewInt64Coin(underlying, 100),
				Volume:       sdkmath.NewInt(1),
			},
			expectedErrSubstrs: []string{"cannot set NAV for vault share denom"},
		},
		{
			name:  "rejects unregistered marker denom",
			setup: setup,
			msg: types.MsgUpdateVaultNAVRequest{
				Signer:       admin.String(),
				VaultAddress: vaultAddr.String(),
				Denom:        "notamarker",
				Price:        sdk.NewInt64Coin(underlying, 100),
				Volume:       sdkmath.NewInt(1),
			},
			expectedErrSubstrs: []string{"is not a registered marker"},
		},
		{
			name:  "rejects zero price",
			setup: setup,
			msg: types.MsgUpdateVaultNAVRequest{
				Signer:       admin.String(),
				VaultAddress: vaultAddr.String(),
				Denom:        "rwa",
				Price:        sdk.NewInt64Coin(underlying, 0),
				Volume:       sdkmath.NewInt(1),
			},
			expectedErrSubstrs: []string{"NAV price amount must be positive"},
		},
		{
			name:  "rejects zero volume",
			setup: setup,
			msg: types.MsgUpdateVaultNAVRequest{
				Signer:       admin.String(),
				VaultAddress: vaultAddr.String(),
				Denom:        "rwa",
				Price:        sdk.NewInt64Coin(underlying, 100),
				Volume:       sdkmath.ZeroInt(),
			},
			expectedErrSubstrs: []string{"NAV volume must be positive"},
		},
		{
			name:  "rejects negative volume",
			setup: setup,
			msg: types.MsgUpdateVaultNAVRequest{
				Signer:       admin.String(),
				VaultAddress: vaultAddr.String(),
				Denom:        "rwa",
				Price:        sdk.NewInt64Coin(underlying, 100),
				Volume:       sdkmath.NewInt(-1),
			},
			expectedErrSubstrs: []string{"NAV volume must be positive"},
		},
		{
			name: "rejects volume that overflows the marker NAV volume",
			setup: func() {
				setup()
				s.requireSimpleMarker("rwa")
			},
			msg: types.MsgUpdateVaultNAVRequest{
				Signer:       admin.String(),
				VaultAddress: vaultAddr.String(),
				Denom:        "rwa",
				Price:        sdk.NewInt64Coin(underlying, 100),
				Volume:       sdkmath.NewIntFromBigInt(new(big.Int).Lsh(big.NewInt(1), 70)),
			},
			expectedErrSubstrs: []string{"failed to publish vault NAV to marker", "overflows the marker NAV volume"},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			runMsgServerTestCase(s, testDef, tc)
		})
	}
}

func (s *TestSuite) TestMsgServer_UpdateVaultNAV_Reconcile() {
	underlying := "under"
	share := "vaultshares"
	navDenom := "rwa"
	testBlockTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name                 string
		interestRate         string
		paused               bool
		useUnregisteredDenom bool
		expectedErrContains  string
		expectedReconciles   int
	}{
		{
			name:               "successful NAV update with an elapsed interest period reconciles exactly once, before the NAV write",
			interestRate:       "4.20",
			expectedReconciles: 1,
		},
		{
			name:               "paused vault accepts the NAV update but does not reconcile",
			interestRate:       "4.20",
			paused:             true,
			expectedReconciles: 0,
		},
		{
			name:                 "NAV update that fails validation reconciles first, relying on tx revert to discard it",
			interestRate:         "4.20",
			useUnregisteredDenom: true,
			expectedErrContains:  "is not a registered marker",
			expectedReconciles:   1,
		},
		{
			name:                "failed reconcile aborts the NAV update",
			interestRate:        "1000000.0",
			expectedErrContains: "failed to reconcile vault before NAV update",
			expectedReconciles:  0,
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			origCtx := s.ctx
			defer func() { s.ctx = origCtx }()
			s.ctx, _ = s.ctx.CacheContext()

			vaultAddr, _ := s.setupReconcileVault(tc.interestRate, testBlockTime.Add(-3*time.Hour).Unix(), tc.paused, sdk.NewInt64Coin(underlying, 1_000), share, sdk.NewInt64Coin(share, 0), testBlockTime)
			denom := navDenom
			if tc.useUnregisteredDenom {
				denom = "notamarker"
			} else {
				s.requireSimpleMarker(navDenom)
			}

			s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
			_, err := keeper.NewMsgServer(s.simApp.VaultKeeper).UpdateVaultNAV(s.ctx, &types.MsgUpdateVaultNAVRequest{
				Signer:       s.adminAddr.String(),
				VaultAddress: vaultAddr.String(),
				Denom:        denom,
				Price:        sdk.NewInt64Coin(underlying, 100),
				Volume:       sdkmath.NewInt(1),
				Source:       "oracle",
			})

			reconcileCount := 0
			reconcileIdx := -1
			updatedIdx := -1
			for i, ev := range s.ctx.EventManager().Events() {
				switch ev.Type {
				case "provlabs.vault.v1.EventVaultReconcile":
					reconcileCount++
					reconcileIdx = i
				case "provlabs.vault.v1.EventNAVUpdated":
					updatedIdx = i
				}
			}

			if tc.expectedErrContains != "" {
				s.Require().ErrorContains(err, tc.expectedErrContains, "UpdateVaultNAV should fail for case %q", tc.name)
			} else {
				s.Require().NoError(err, "UpdateVaultNAV should succeed for case %q", tc.name)
			}
			s.Assert().Equal(tc.expectedReconciles, reconcileCount, "EventVaultReconcile count mismatch for case %q", tc.name)
			if tc.expectedErrContains == "" && tc.expectedReconciles > 0 {
				s.Require().GreaterOrEqual(updatedIdx, 0, "EventNAVUpdated should be emitted for case %q", tc.name)
				s.Assert().Less(reconcileIdx, updatedIdx, "reconcile events should precede EventNAVUpdated for case %q", tc.name)
			}
		})
	}
}

// TestMsgServer_UpdateNAVAuthority verifies that the vault admin can rotate the
// NAV authority and that the rotation is persisted on the vault account.
func (s *TestSuite) TestMsgServer_UpdateNAVAuthority() {
	underlying := "under"
	share := "vaultshares"
	admin := s.adminAddr
	newAuthority := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1000))
	vaultAddr := types.GetVaultAddress(share)

	testDef := msgServerTestDef[types.MsgUpdateNAVAuthorityRequest, types.MsgUpdateNAVAuthorityResponse, string]{
		endpointName:     "UpdateNAVAuthority",
		endpoint:         keeper.NewMsgServer(s.simApp.VaultKeeper).UpdateNAVAuthority,
		expectedResponse: &types.MsgUpdateNAVAuthorityResponse{},
		postCheck: func(_ *types.MsgUpdateNAVAuthorityRequest, expectedAuthority string) {
			vault, err := s.k.GetVault(s.ctx, vaultAddr)
			s.Require().NoError(err, "post-check: failed to get vault")
			s.Assert().Equal(expectedAuthority, vault.NavAuthority, "post-check: nav authority mismatch")
			s.Assert().Equal(expectedAuthority, vault.GetNAVAuthority(), "post-check: resolved nav authority mismatch")
		},
	}

	tc := msgServerTestCase[types.MsgUpdateNAVAuthorityRequest, string]{
		name: "admin rotates nav authority",
		setup: func() {
			s.setupBaseVault(underlying, share)
		},
		msg: types.MsgUpdateNAVAuthorityRequest{
			Signer:       admin.String(),
			VaultAddress: vaultAddr.String(),
			NewAuthority: newAuthority.String(),
		},
		postCheckArgs: newAuthority.String(),
		expectedEvents: sdk.Events{
			sdk.NewEvent("provlabs.vault.v1.EventNAVAuthorityUpdated",
				sdk.NewAttribute("admin", admin.String()),
				sdk.NewAttribute("new_authority", newAuthority.String()),
				sdk.NewAttribute("vault_address", vaultAddr.String()),
			),
		},
	}

	s.Run(tc.name, func() {
		runMsgServerTestCase(s, testDef, tc)
	})
}

// TestMsgServer_UpdateNAVAuthority_Failures covers admin enforcement and the
// missing-vault path for the UpdateNAVAuthority handler.
func (s *TestSuite) TestMsgServer_UpdateNAVAuthority_Failures() {
	underlying := "under"
	share := "vaultshares"
	admin := s.adminAddr
	other := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1000))
	vaultAddr := types.GetVaultAddress(share)

	setup := func() {
		s.setupBaseVault(underlying, share)
	}

	testDef := msgServerTestDef[types.MsgUpdateNAVAuthorityRequest, types.MsgUpdateNAVAuthorityResponse, any]{
		endpointName: "UpdateNAVAuthority",
		endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).UpdateNAVAuthority,
		postCheck:    nil,
	}

	tests := []msgServerTestCase[types.MsgUpdateNAVAuthorityRequest, any]{
		{
			name:  "vault does not exist",
			setup: func() {},
			msg: types.MsgUpdateNAVAuthorityRequest{
				Signer:       admin.String(),
				VaultAddress: types.GetVaultAddress("doesnotexist").String(),
				NewAuthority: other.String(),
			},
			expectedErrSubstrs: []string{"vault not found"},
		},
		{
			name:  "signer is not the vault admin",
			setup: setup,
			msg: types.MsgUpdateNAVAuthorityRequest{
				Signer:       other.String(),
				VaultAddress: vaultAddr.String(),
				NewAuthority: other.String(),
			},
			expectedErrSubstrs: []string{"unauthorized", "is not the vault admin"},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			runMsgServerTestCase(s, testDef, tc)
		})
	}
}

// TestMsgServer_UpdateNAVAuthority_RotationChangesNAVAuthority verifies that
// after a rotation only the new authority can update NAVs and the prior admin
// loses NAV authority.
func (s *TestSuite) TestMsgServer_UpdateNAVAuthority_RotationChangesNAVAuthority() {
	underlying := "under"
	share := "vaultshares"
	admin := s.adminAddr
	oracle := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1000))
	vaultAddr := types.GetVaultAddress(share)

	s.setupBaseVault(underlying, share)
	s.requireSimpleMarker("rwa")
	msgServer := keeper.NewMsgServer(s.simApp.VaultKeeper)

	_, err := msgServer.UpdateNAVAuthority(s.ctx, &types.MsgUpdateNAVAuthorityRequest{
		Signer:       admin.String(),
		VaultAddress: vaultAddr.String(),
		NewAuthority: oracle.String(),
	})
	s.Require().NoError(err, "admin should be able to rotate the nav authority")

	_, err = msgServer.UpdateVaultNAV(s.ctx, &types.MsgUpdateVaultNAVRequest{
		Signer:       admin.String(),
		VaultAddress: vaultAddr.String(),
		Denom:        "rwa",
		Price:        sdk.NewInt64Coin(underlying, 100),
		Volume:       sdkmath.NewInt(1),
	})
	s.Require().Error(err, "former admin should no longer be the nav authority")
	s.Assert().Contains(err.Error(), "is not the vault NAV authority", "unexpected error for stale authority")

	_, err = msgServer.UpdateVaultNAV(s.ctx, &types.MsgUpdateVaultNAVRequest{
		Signer:       oracle.String(),
		VaultAddress: vaultAddr.String(),
		Denom:        "rwa",
		Price:        sdk.NewInt64Coin(underlying, 100),
		Volume:       sdkmath.NewInt(1),
	})
	s.Require().NoError(err, "new nav authority should be able to update NAVs")

	nav, err := s.k.GetVaultNAV(s.ctx, vaultAddr, "rwa")
	s.Require().NoError(err, "NAV should exist after the new authority update")
	s.Assert().Equal(sdk.NewInt64Coin(underlying, 100), nav.Price, "NAV price mismatch")
}

// TestMsgServer_UpdateNAVAuthority_ResetToEmptyFallsBackToAdmin verifies that an
// UpdateNAVAuthority message with an empty new authority clears the explicit NAV
// authority, after which the vault admin again resolves as the NAV authority and
// the previously delegated authority loses its NAV update rights.
func (s *TestSuite) TestMsgServer_UpdateNAVAuthority_ResetToEmptyFallsBackToAdmin() {
	underlying := "under"
	share := "vaultshares"
	admin := s.adminAddr
	oracle := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1000))
	vaultAddr := types.GetVaultAddress(share)

	s.setupBaseVault(underlying, share)
	s.requireSimpleMarker("rwa")
	msgServer := keeper.NewMsgServer(s.simApp.VaultKeeper)

	_, err := msgServer.UpdateNAVAuthority(s.ctx, &types.MsgUpdateNAVAuthorityRequest{
		Signer:       admin.String(),
		VaultAddress: vaultAddr.String(),
		NewAuthority: oracle.String(),
	})
	s.Require().NoError(err, "admin should be able to delegate the nav authority to the oracle")

	_, err = msgServer.UpdateNAVAuthority(s.ctx, &types.MsgUpdateNAVAuthorityRequest{
		Signer:       admin.String(),
		VaultAddress: vaultAddr.String(),
		NewAuthority: "",
	})
	s.Require().NoError(err, "admin should be able to reset the nav authority to empty")

	vault, err := s.k.GetVault(s.ctx, vaultAddr)
	s.Require().NoError(err, "failed to get vault after resetting the nav authority")
	s.Assert().Empty(vault.NavAuthority, "reset should clear the explicit nav authority")
	s.Assert().Equal(admin.String(), vault.GetNAVAuthority(), "cleared nav authority should fall back to the vault admin")

	_, err = msgServer.UpdateVaultNAV(s.ctx, &types.MsgUpdateVaultNAVRequest{
		Signer:       oracle.String(),
		VaultAddress: vaultAddr.String(),
		Denom:        "rwa",
		Price:        sdk.NewInt64Coin(underlying, 100),
		Volume:       sdkmath.NewInt(1),
	})
	s.Require().Error(err, "the former delegated authority should lose NAV update rights after the reset")
	s.Assert().Contains(err.Error(), "is not the vault NAV authority", "unexpected error for the former authority")

	_, err = msgServer.UpdateVaultNAV(s.ctx, &types.MsgUpdateVaultNAVRequest{
		Signer:       admin.String(),
		VaultAddress: vaultAddr.String(),
		Denom:        "rwa",
		Price:        sdk.NewInt64Coin(underlying, 100),
		Volume:       sdkmath.NewInt(1),
	})
	s.Require().NoError(err, "the admin should regain NAV update rights after the reset")

	nav, err := s.k.GetVaultNAV(s.ctx, vaultAddr, "rwa")
	s.Require().NoError(err, "NAV should exist after the admin update following the reset")
	s.Assert().Equal(sdk.NewInt64Coin(underlying, 100), nav.Price, "NAV price mismatch after reset")
}

// TestMsgServer_UpdateNAVAuthority_NoOpWhenUnchanged verifies that calling
// UpdateNAVAuthority with the current NAV authority is a no-op: the call
// succeeds, the vault is left untouched, and no EventNAVAuthorityUpdated event
// is emitted.
func (s *TestSuite) TestMsgServer_UpdateNAVAuthority_NoOpWhenUnchanged() {
	underlying := "under"
	share := "vaultshares"
	admin := s.adminAddr
	oracle := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1000))
	vaultAddr := types.GetVaultAddress(share)

	s.setupBaseVault(underlying, share)
	msgServer := keeper.NewMsgServer(s.simApp.VaultKeeper)

	_, err := msgServer.UpdateNAVAuthority(s.ctx, &types.MsgUpdateNAVAuthorityRequest{
		Signer:       admin.String(),
		VaultAddress: vaultAddr.String(),
		NewAuthority: oracle.String(),
	})
	s.Require().NoError(err, "initial rotation should succeed")

	vaultBefore, err := s.k.GetVault(s.ctx, vaultAddr)
	s.Require().NoError(err, "failed to read vault after initial rotation")

	s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	_, err = msgServer.UpdateNAVAuthority(s.ctx, &types.MsgUpdateNAVAuthorityRequest{
		Signer:       admin.String(),
		VaultAddress: vaultAddr.String(),
		NewAuthority: oracle.String(),
	})
	s.Require().NoError(err, "re-setting the NAV authority to its current value should be a no-op")

	for _, ev := range s.ctx.EventManager().Events() {
		s.Assert().NotEqualf(
			"provlabs.vault.v1.EventNAVAuthorityUpdated", ev.Type,
			"no-op UpdateNAVAuthority should not emit EventNAVAuthorityUpdated",
		)
	}

	vaultAfter, err := s.k.GetVault(s.ctx, vaultAddr)
	s.Require().NoError(err, "failed to read vault after no-op rotation")
	s.Assert().Equal(vaultBefore.NavAuthority, vaultAfter.NavAuthority, "no-op rotation should leave NavAuthority untouched")
}

func (s *TestSuite) TestMsgServer_AcceptAsset_Inbound() {
	underlying, share, paymentDenom, asset := "under", "vshare", "pay", "rwacoin"
	externalID := "p2p-inbound"

	vault, principalAddr := s.setupAssetSettlementVault(underlying, share, paymentDenom)
	vaultAddr := vault.GetAddress()
	s.requireSimpleMarker(asset)

	source := s.CreateAndFundAccount(sdk.NewInt64Coin(asset, 10))
	s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, principalAddr, sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 5))), "failed to fund principal with payment denom")

	sourceAmount := sdk.NewCoins(sdk.NewInt64Coin(asset, 10))
	targetAmount := sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 5))
	s.createPayment(source, vaultAddr, sourceAmount, targetAmount, externalID)

	s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	resp, err := keeper.NewMsgServer(s.simApp.VaultKeeper).AcceptAsset(s.ctx, &types.MsgAcceptAssetRequest{
		Authority:    s.adminAddr.String(),
		VaultAddress: vaultAddr.String(),
		Source:       source.String(),
		ExternalId:   externalID,
	})
	s.Require().NoError(err, "AcceptAsset inbound should succeed")
	s.Assert().Equal(&types.MsgAcceptAssetResponse{}, resp, "AcceptAsset inbound response")

	s.assertBalance(source, asset, sdkmath.NewInt(0))
	s.assertBalance(source, paymentDenom, sdkmath.NewInt(5))
	s.assertBalance(principalAddr, asset, sdkmath.NewInt(10))
	s.assertBalance(principalAddr, paymentDenom, sdkmath.NewInt(0))
	s.assertBalance(vaultAddr, asset, sdkmath.NewInt(0))
	s.assertBalance(vaultAddr, paymentDenom, sdkmath.NewInt(0))

	s.requireTypedEventEmitted(types.NewEventAssetAccepted(vaultAddr.String(), source.String(), externalID, sourceAmount, targetAmount, types.AssetDirectionInbound))
}

func (s *TestSuite) TestMsgServer_AcceptAsset_Outbound() {
	underlying, share, paymentDenom, asset := "under", "vshare", "pay", "rwacoin"
	externalID := "p2p-outbound"

	vault, principalAddr := s.setupAssetSettlementVault(underlying, share, paymentDenom)
	vaultAddr := vault.GetAddress()
	s.requireSimpleMarker(asset)

	assetMgr := s.CreateAndFundAccount(sdk.NewInt64Coin(underlying, 1))
	_, err := keeper.NewMsgServer(s.simApp.VaultKeeper).SetAssetManager(s.ctx, &types.MsgSetAssetManagerRequest{
		Admin:        s.adminAddr.String(),
		VaultAddress: vaultAddr.String(),
		AssetManager: assetMgr.String(),
	})
	s.Require().NoError(err, "failed to set asset manager")

	source := s.CreateAndFundAccount(sdk.NewInt64Coin(paymentDenom, 5))
	s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, principalAddr, sdk.NewCoins(sdk.NewInt64Coin(asset, 10))), "failed to fund principal with asset")

	sourceAmount := sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 5))
	targetAmount := sdk.NewCoins(sdk.NewInt64Coin(asset, 10))
	s.createPayment(source, vaultAddr, sourceAmount, targetAmount, externalID)

	s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	resp, err := keeper.NewMsgServer(s.simApp.VaultKeeper).AcceptAsset(s.ctx, &types.MsgAcceptAssetRequest{
		Authority:    assetMgr.String(),
		VaultAddress: vaultAddr.String(),
		Source:       source.String(),
		ExternalId:   externalID,
	})
	s.Require().NoError(err, "AcceptAsset outbound should succeed")
	s.Assert().Equal(&types.MsgAcceptAssetResponse{}, resp, "AcceptAsset outbound response")

	s.assertBalance(source, paymentDenom, sdkmath.NewInt(0))
	s.assertBalance(source, asset, sdkmath.NewInt(10))
	s.assertBalance(principalAddr, asset, sdkmath.NewInt(0))
	s.assertBalance(principalAddr, paymentDenom, sdkmath.NewInt(5))
	s.assertBalance(vaultAddr, asset, sdkmath.NewInt(0))
	s.assertBalance(vaultAddr, paymentDenom, sdkmath.NewInt(0))

	s.requireTypedEventEmitted(types.NewEventAssetAccepted(vaultAddr.String(), source.String(), externalID, sourceAmount, targetAmount, types.AssetDirectionOutbound))
}

func (s *TestSuite) TestMsgServer_AcceptAsset_RestrictedMarker() {
	underlying, share, paymentDenom, restrictedDenom := "under", "vshare", "pay", "restrictedrwa"
	externalID := "p2p-restricted"

	tests := []struct {
		name                      string
		grantVaultMarkerTransfer  bool
		grantSourceMarkerTransfer bool
		expectedErrSubstrs        []string
	}{
		{
			name:                      "vault and source both hold marker transfer access, restricted settlement succeeds",
			grantVaultMarkerTransfer:  true,
			grantSourceMarkerTransfer: true,
		},
		{
			name:                     "source without marker transfer access, agentless exchange escrow transfer is blocked",
			grantVaultMarkerTransfer: true,
			expectedErrSubstrs:       []string{"failed to accept payment", "does not have transfer permissions"},
		},
		{
			name:                      "vault without marker transfer access, vault-as-transfer-agent hop to principal is blocked",
			grantSourceMarkerTransfer: true,
			expectedErrSubstrs:        []string{"failed to move source amount from vault to principal", "have ACCESS_TRANSFER on restrictedrwa marker"},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			origCtx := s.ctx
			defer func() { s.ctx = origCtx }()
			s.ctx, _ = s.ctx.CacheContext()

			vault, principalAddr := s.setupAssetSettlementVault(underlying, share, paymentDenom)
			vaultAddr := vault.GetAddress()
			source := s.CreateAndFundAccount(sdk.NewInt64Coin(underlying, 1))

			grants := []markertypes.AccessGrant{
				{Address: s.adminAddr.String(), Permissions: markertypes.AccessList{
					markertypes.Access_Mint, markertypes.Access_Burn, markertypes.Access_Withdraw,
					markertypes.Access_Admin, markertypes.Access_Transfer,
				}},
			}
			if tc.grantVaultMarkerTransfer {
				grants = append(grants, markertypes.AccessGrant{Address: vaultAddr.String(), Permissions: markertypes.AccessList{markertypes.Access_Transfer}})
			}
			if tc.grantSourceMarkerTransfer {
				grants = append(grants, markertypes.AccessGrant{Address: source.String(), Permissions: markertypes.AccessList{markertypes.Access_Transfer}})
			}
			restrictedMarker := markertypes.NewMarkerAccount(
				authtypes.NewBaseAccountWithAddress(markertypes.MustGetMarkerAddress(restrictedDenom)),
				sdk.NewInt64Coin(restrictedDenom, 1_000_000),
				s.adminAddr,
				grants,
				markertypes.StatusProposed,
				markertypes.MarkerType_RestrictedCoin,
				false, true, false, []string{},
			)
			s.Require().NoError(s.simApp.MarkerKeeper.AddFinalizeAndActivateMarker(s.ctx, restrictedMarker), "failed to create restricted marker %s", restrictedDenom)
			s.Require().NoError(s.simApp.MarkerKeeper.WithdrawCoins(s.ctx, s.adminAddr, source, restrictedDenom, sdk.NewCoins(sdk.NewInt64Coin(restrictedDenom, 10))), "failed to fund source %s with restricted marker denom %s", source, restrictedDenom)
			s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, principalAddr, sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 5))), "failed to fund principal %s with payment denom %s", principalAddr, paymentDenom)

			sourceAmount := sdk.NewCoins(sdk.NewInt64Coin(restrictedDenom, 10))
			targetAmount := sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 5))
			s.createPayment(source, vaultAddr, sourceAmount, targetAmount, externalID)

			s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
			resp, err := keeper.NewMsgServer(s.simApp.VaultKeeper).AcceptAsset(s.ctx, &types.MsgAcceptAssetRequest{
				Authority:    s.adminAddr.String(),
				VaultAddress: vaultAddr.String(),
				Source:       source.String(),
				ExternalId:   externalID,
			})

			if len(tc.expectedErrSubstrs) > 0 {
				s.Require().Error(err, "AcceptAsset of restricted marker denom %s should fail for case %q", restrictedDenom, tc.name)
				s.Assert().Nil(resp, "AcceptAsset response should be nil on error for case %q", tc.name)
				for _, substr := range tc.expectedErrSubstrs {
					s.Assert().Containsf(err.Error(), substr, "AcceptAsset error missing expected substring for case %q", tc.name)
				}
				return
			}

			s.Require().NoError(err, "AcceptAsset of restricted marker denom %s should succeed when vault %s and source %s hold transfer access", restrictedDenom, vaultAddr, source)
			s.Assert().Equal(&types.MsgAcceptAssetResponse{}, resp, "AcceptAsset restricted marker response")

			s.assertBalance(source, restrictedDenom, sdkmath.NewInt(0))
			s.assertBalance(source, paymentDenom, sdkmath.NewInt(5))
			s.assertBalance(principalAddr, restrictedDenom, sdkmath.NewInt(10))
			s.assertBalance(principalAddr, paymentDenom, sdkmath.NewInt(0))
			s.assertBalance(vaultAddr, restrictedDenom, sdkmath.NewInt(0))
			s.assertBalance(vaultAddr, paymentDenom, sdkmath.NewInt(0))

			s.requireTypedEventEmitted(types.NewEventAssetAccepted(vaultAddr.String(), source.String(), externalID, sourceAmount, targetAmount, types.AssetDirectionInbound))
		})
	}
}

func (s *TestSuite) TestMsgServer_AcceptAsset_Failures() {
	underlying, share, paymentDenom, asset := "under", "vshare", "pay", "rwacoin"

	tests := []struct {
		name string
		// setup returns the request to send. It is responsible for any vault, payment, and
		// funding state required by the case.
		setup              func() *types.MsgAcceptAssetRequest
		expectedErrSubstrs []string
	}{
		{
			name: "vault does not exist",
			setup: func() *types.MsgAcceptAssetRequest {
				source := s.CreateAndFundAccount(sdk.NewInt64Coin(asset, 10))
				return &types.MsgAcceptAssetRequest{
					Authority:    s.adminAddr.String(),
					VaultAddress: types.GetVaultAddress("missing").String(),
					Source:       source.String(),
					ExternalId:   "none",
				}
			},
			expectedErrSubstrs: []string{"not found"},
		},
		{
			name: "unauthorized signer",
			setup: func() *types.MsgAcceptAssetRequest {
				vault, principalAddr := s.setupAssetSettlementVault(underlying, share, paymentDenom)
				vaultAddr := vault.GetAddress()
				source := s.CreateAndFundAccount(sdk.NewInt64Coin(asset, 10))
				stranger := s.CreateAndFundAccount(sdk.NewInt64Coin(underlying, 1))
				s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, principalAddr, sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 5))), "fund principal")
				s.createPayment(source, vaultAddr, sdk.NewCoins(sdk.NewInt64Coin(asset, 10)), sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 5)), "unauth")
				return &types.MsgAcceptAssetRequest{
					Authority:    stranger.String(),
					VaultAddress: vaultAddr.String(),
					Source:       source.String(),
					ExternalId:   "unauth",
				}
			},
			expectedErrSubstrs: []string{"unauthorized authority"},
		},
		{
			name: "payment not found",
			setup: func() *types.MsgAcceptAssetRequest {
				vault, _ := s.setupAssetSettlementVault(underlying, share, paymentDenom)
				vaultAddr := vault.GetAddress()
				source := s.CreateAndFundAccount(sdk.NewInt64Coin(asset, 10))
				return &types.MsgAcceptAssetRequest{
					Authority:    s.adminAddr.String(),
					VaultAddress: vaultAddr.String(),
					Source:       source.String(),
					ExternalId:   "ghost",
				}
			},
			expectedErrSubstrs: []string{"payment not found"},
		},
		{
			name: "payment target mismatch",
			setup: func() *types.MsgAcceptAssetRequest {
				vault, _ := s.setupAssetSettlementVault(underlying, share, paymentDenom)
				vaultAddr := vault.GetAddress()
				source := s.CreateAndFundAccount(sdk.NewInt64Coin(asset, 10))
				other := s.CreateAndFundAccount(sdk.NewInt64Coin(underlying, 1))
				s.createPayment(source, other, sdk.NewCoins(sdk.NewInt64Coin(asset, 10)), sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 5)), "mismatch")
				return &types.MsgAcceptAssetRequest{
					Authority:    s.adminAddr.String(),
					VaultAddress: vaultAddr.String(),
					Source:       source.String(),
					ExternalId:   "mismatch",
				}
			},
			expectedErrSubstrs: []string{"is not vault"},
		},
		{
			name: "payment denom on neither leg",
			setup: func() *types.MsgAcceptAssetRequest {
				vault, _ := s.setupAssetSettlementVault(underlying, share, paymentDenom)
				vaultAddr := vault.GetAddress()
				source := s.CreateAndFundAccount(sdk.NewInt64Coin(asset, 10))
				s.createPayment(source, vaultAddr, sdk.NewCoins(sdk.NewInt64Coin(asset, 10)), sdk.NewCoins(sdk.NewInt64Coin(underlying, 5)), "noleg")
				return &types.MsgAcceptAssetRequest{
					Authority:    s.adminAddr.String(),
					VaultAddress: vaultAddr.String(),
					Source:       source.String(),
					ExternalId:   "noleg",
				}
			},
			expectedErrSubstrs: []string{"exactly one leg"},
		},
		{
			name: "payment denom on both legs",
			setup: func() *types.MsgAcceptAssetRequest {
				vault, _ := s.setupAssetSettlementVault(underlying, share, paymentDenom)
				vaultAddr := vault.GetAddress()
				source := s.CreateAndFundAccount(sdk.NewInt64Coin(paymentDenom, 5))
				s.createPayment(source, vaultAddr, sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 5)), sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 3), sdk.NewInt64Coin(asset, 2)), "bothlegs")
				return &types.MsgAcceptAssetRequest{
					Authority:    s.adminAddr.String(),
					VaultAddress: vaultAddr.String(),
					Source:       source.String(),
					ExternalId:   "bothlegs",
				}
			},
			expectedErrSubstrs: []string{"exactly one leg"},
		},
		{
			name: "insufficient principal balance",
			setup: func() *types.MsgAcceptAssetRequest {
				vault, principalAddr := s.setupAssetSettlementVault(underlying, share, paymentDenom)
				vaultAddr := vault.GetAddress()
				source := s.CreateAndFundAccount(sdk.NewInt64Coin(asset, 10))
				s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, principalAddr, sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 4))), "fund principal short")
				s.createPayment(source, vaultAddr, sdk.NewCoins(sdk.NewInt64Coin(asset, 10)), sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 5)), "short")
				return &types.MsgAcceptAssetRequest{
					Authority:    s.adminAddr.String(),
					VaultAddress: vaultAddr.String(),
					Source:       source.String(),
					ExternalId:   "short",
				}
			},
			expectedErrSubstrs: []string{"insufficient principal balance"},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			origCtx := s.ctx
			defer func() { s.ctx = origCtx }()
			s.ctx, _ = s.ctx.CacheContext()

			msg := tc.setup()
			s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())

			resp, err := keeper.NewMsgServer(s.simApp.VaultKeeper).AcceptAsset(s.ctx, msg)
			s.Require().Error(err, "AcceptAsset should fail for case %q", tc.name)
			s.Assert().Nil(resp, "AcceptAsset response should be nil on error for case %q", tc.name)
			for _, substr := range tc.expectedErrSubstrs {
				s.Assert().Containsf(err.Error(), substr, "AcceptAsset error missing expected substring for case %q", tc.name)
			}
		})
	}
}

func (s *TestSuite) TestMsgServer_AcceptAsset_NAVGuardrail() {
	underlying, share, paymentDenom, asset := "under", "vshare", "pay", "rwacoin"
	externalID := "guardrail"

	tests := []struct {
		name               string
		seedNav            *types.VaultNAV
		fundSource         sdk.Coins
		fundPrincipal      sdk.Coins
		sourceAmount       sdk.Coins
		targetAmount       sdk.Coins
		expectedErrSubstrs []string
	}{
		{
			name:          "inbound settlement at the exact NAV price passes the guardrail",
			seedNav:       &types.VaultNAV{Denom: asset, Price: sdk.NewInt64Coin(paymentDenom, 5), Volume: sdkmath.NewInt(10)},
			fundSource:    sdk.NewCoins(sdk.NewInt64Coin(asset, 10)),
			fundPrincipal: sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 5)),
			sourceAmount:  sdk.NewCoins(sdk.NewInt64Coin(asset, 10)),
			targetAmount:  sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 5)),
		},
		{
			name:               "inbound settlement off the NAV price is rejected",
			seedNav:            &types.VaultNAV{Denom: asset, Price: sdk.NewInt64Coin(paymentDenom, 6), Volume: sdkmath.NewInt(10)},
			fundSource:         sdk.NewCoins(sdk.NewInt64Coin(asset, 10)),
			fundPrincipal:      sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 5)),
			sourceAmount:       sdk.NewCoins(sdk.NewInt64Coin(asset, 10)),
			targetAmount:       sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 5)),
			expectedErrSubstrs: []string{"does not match internal NAV", "10rwacoin", "5pay", "6pay"},
		},
		{
			name:          "no NAV entry skips the guardrail for a first acquisition",
			fundSource:    sdk.NewCoins(sdk.NewInt64Coin(asset, 10)),
			fundPrincipal: sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 5)),
			sourceAmount:  sdk.NewCoins(sdk.NewInt64Coin(asset, 10)),
			targetAmount:  sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 5)),
		},
		{
			name:          "outbound settlement at the exact NAV price passes the guardrail",
			seedNav:       &types.VaultNAV{Denom: asset, Price: sdk.NewInt64Coin(paymentDenom, 5), Volume: sdkmath.NewInt(10)},
			fundSource:    sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 5)),
			fundPrincipal: sdk.NewCoins(sdk.NewInt64Coin(asset, 10)),
			sourceAmount:  sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 5)),
			targetAmount:  sdk.NewCoins(sdk.NewInt64Coin(asset, 10)),
		},
		{
			name:               "outbound settlement off the NAV price is rejected",
			seedNav:            &types.VaultNAV{Denom: asset, Price: sdk.NewInt64Coin(paymentDenom, 5), Volume: sdkmath.NewInt(10)},
			fundSource:         sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 4)),
			fundPrincipal:      sdk.NewCoins(sdk.NewInt64Coin(asset, 10)),
			sourceAmount:       sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 4)),
			targetAmount:       sdk.NewCoins(sdk.NewInt64Coin(asset, 10)),
			expectedErrSubstrs: []string{"does not match internal NAV", "10rwacoin", "4pay", "5pay"},
		},
		{
			name:               "fractional NAV price that floor math would accept is rejected",
			seedNav:            &types.VaultNAV{Denom: asset, Price: sdk.NewInt64Coin(paymentDenom, 3), Volume: sdkmath.NewInt(2)},
			fundSource:         sdk.NewCoins(sdk.NewInt64Coin(asset, 3)),
			fundPrincipal:      sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 4)),
			sourceAmount:       sdk.NewCoins(sdk.NewInt64Coin(asset, 3)),
			targetAmount:       sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 4)),
			expectedErrSubstrs: []string{"does not match internal NAV"},
		},
		{
			name:          "fractional NAV price settled at an exact multiple passes the guardrail",
			seedNav:       &types.VaultNAV{Denom: asset, Price: sdk.NewInt64Coin(paymentDenom, 3), Volume: sdkmath.NewInt(2)},
			fundSource:    sdk.NewCoins(sdk.NewInt64Coin(asset, 4)),
			fundPrincipal: sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 6)),
			sourceAmount:  sdk.NewCoins(sdk.NewInt64Coin(asset, 4)),
			targetAmount:  sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 6)),
		},
		{
			name:               "NAV priced in a denom other than the settlement payment is rejected",
			seedNav:            &types.VaultNAV{Denom: asset, Price: sdk.NewInt64Coin(underlying, 5), Volume: sdkmath.NewInt(10)},
			fundSource:         sdk.NewCoins(sdk.NewInt64Coin(asset, 10)),
			fundPrincipal:      sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 5)),
			sourceAmount:       sdk.NewCoins(sdk.NewInt64Coin(asset, 10)),
			targetAmount:       sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 5)),
			expectedErrSubstrs: []string{"is priced in"},
		},
		{
			name:               "asset leg with multiple coins cannot be priced and is rejected",
			fundSource:         sdk.NewCoins(sdk.NewInt64Coin(asset, 10), sdk.NewInt64Coin("othercoin", 5)),
			fundPrincipal:      sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 5)),
			sourceAmount:       sdk.NewCoins(sdk.NewInt64Coin(asset, 10), sdk.NewInt64Coin("othercoin", 5)),
			targetAmount:       sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 5)),
			expectedErrSubstrs: []string{"exactly one coin"},
		},
		{
			name:               "empty asset leg cannot be priced and is rejected",
			fundPrincipal:      sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 5)),
			sourceAmount:       sdk.NewCoins(),
			targetAmount:       sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 5)),
			expectedErrSubstrs: []string{"exactly one coin"},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			origCtx := s.ctx
			defer func() { s.ctx = origCtx }()
			s.ctx, _ = s.ctx.CacheContext()

			vault, principalAddr := s.setupAssetSettlementVault(underlying, share, paymentDenom)
			vaultAddr := vault.GetAddress()
			s.requireSimpleMarker(asset)

			if tc.seedNav != nil {
				s.Require().NoError(
					s.k.SetVaultNAV(s.ctx, vault, *tc.seedNav, s.adminAddr.String()),
					"failed to seed internal NAV for denom %s", tc.seedNav.Denom,
				)
			}

			source := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1_000))
			if !tc.fundSource.IsZero() {
				s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, source, tc.fundSource), "failed to fund source with %s", tc.fundSource)
			}
			if !tc.fundPrincipal.IsZero() {
				s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, principalAddr, tc.fundPrincipal), "failed to fund principal with %s", tc.fundPrincipal)
			}

			s.createPayment(source, vaultAddr, tc.sourceAmount, tc.targetAmount, externalID)

			resp, err := keeper.NewMsgServer(s.simApp.VaultKeeper).AcceptAsset(s.ctx, &types.MsgAcceptAssetRequest{
				Authority:    s.adminAddr.String(),
				VaultAddress: vaultAddr.String(),
				Source:       source.String(),
				ExternalId:   externalID,
			})

			if len(tc.expectedErrSubstrs) == 0 {
				s.Require().NoError(err, "AcceptAsset should pass the guardrail for case %q", tc.name)
				s.Assert().Equal(&types.MsgAcceptAssetResponse{}, resp, "AcceptAsset response for case %q", tc.name)
				return
			}

			s.Require().Error(err, "AcceptAsset should fail the guardrail for case %q", tc.name)
			s.Assert().Nil(resp, "AcceptAsset response should be nil on error for case %q", tc.name)
			for _, substr := range tc.expectedErrSubstrs {
				s.Assert().Containsf(err.Error(), substr, "AcceptAsset error missing expected substring for case %q", tc.name)
			}
		})
	}
}

func (s *TestSuite) TestMsgServer_AcceptAsset_SettlementNAV() {
	underlying := "under"
	share := "vshare"
	paymentDenom := "pay"
	asset := "rwacoin"
	externalID := "settle-nav"

	tests := []struct {
		name                string
		registerAssetMarker bool
		seedNav             *types.VaultNAV
		fundSource          sdk.Coins
		fundPrincipal       sdk.Coins
		sourceAmount        sdk.Coins
		targetAmount        sdk.Coins
		expectedNavPrice    sdk.Coin
		expectedNavVolume   sdkmath.Int
		expectNavRemoved    bool
		expectedErrContains string
	}{
		{
			name:                "inbound first acquisition seeds the internal NAV from the settlement price",
			registerAssetMarker: true,
			fundSource:          sdk.NewCoins(sdk.NewInt64Coin(asset, 10)),
			fundPrincipal:       sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 5)),
			sourceAmount:        sdk.NewCoins(sdk.NewInt64Coin(asset, 10)),
			targetAmount:        sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 5)),
			expectedNavPrice:    sdk.NewInt64Coin(paymentDenom, 5),
			expectedNavVolume:   sdkmath.NewInt(10),
		},
		{
			name:                "inbound settlement at the NAV price re-prices the entry from the settlement legs",
			registerAssetMarker: true,
			seedNav:             &types.VaultNAV{Denom: asset, Price: sdk.NewInt64Coin(paymentDenom, 1), Volume: sdkmath.NewInt(2)},
			fundSource:          sdk.NewCoins(sdk.NewInt64Coin(asset, 4)),
			fundPrincipal:       sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 2)),
			sourceAmount:        sdk.NewCoins(sdk.NewInt64Coin(asset, 4)),
			targetAmount:        sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 2)),
			expectedNavPrice:    sdk.NewInt64Coin(paymentDenom, 2),
			expectedNavVolume:   sdkmath.NewInt(4),
		},
		{
			name:                "fractional first acquisition stores the exact settlement price and volume",
			registerAssetMarker: true,
			fundSource:          sdk.NewCoins(sdk.NewInt64Coin(asset, 3)),
			fundPrincipal:       sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 10)),
			sourceAmount:        sdk.NewCoins(sdk.NewInt64Coin(asset, 3)),
			targetAmount:        sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 10)),
			expectedNavPrice:    sdk.NewInt64Coin(paymentDenom, 10),
			expectedNavVolume:   sdkmath.NewInt(3),
		},
		{
			name:                "outbound settlement leaving a principal balance keeps the NAV entry",
			registerAssetMarker: true,
			seedNav:             &types.VaultNAV{Denom: asset, Price: sdk.NewInt64Coin(paymentDenom, 5), Volume: sdkmath.NewInt(10)},
			fundSource:          sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 5)),
			fundPrincipal:       sdk.NewCoins(sdk.NewInt64Coin(asset, 20)),
			sourceAmount:        sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 5)),
			targetAmount:        sdk.NewCoins(sdk.NewInt64Coin(asset, 10)),
			expectedNavPrice:    sdk.NewInt64Coin(paymentDenom, 5),
			expectedNavVolume:   sdkmath.NewInt(10),
		},
		{
			name:                "outbound settlement draining the principal removes the NAV entry",
			registerAssetMarker: true,
			seedNav:             &types.VaultNAV{Denom: asset, Price: sdk.NewInt64Coin(paymentDenom, 5), Volume: sdkmath.NewInt(10)},
			fundSource:          sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 5)),
			fundPrincipal:       sdk.NewCoins(sdk.NewInt64Coin(asset, 10)),
			sourceAmount:        sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 5)),
			targetAmount:        sdk.NewCoins(sdk.NewInt64Coin(asset, 10)),
			expectedNavPrice:    sdk.NewInt64Coin(paymentDenom, 5),
			expectedNavVolume:   sdkmath.NewInt(10),
			expectNavRemoved:    true,
		},
		{
			name:                "asset denom that is not a registered marker fails the settlement",
			registerAssetMarker: false,
			fundSource:          sdk.NewCoins(sdk.NewInt64Coin(asset, 10)),
			fundPrincipal:       sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 5)),
			sourceAmount:        sdk.NewCoins(sdk.NewInt64Coin(asset, 10)),
			targetAmount:        sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 5)),
			expectedErrContains: "is not a registered marker",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			origCtx := s.ctx
			defer func() { s.ctx = origCtx }()
			s.ctx, _ = s.ctx.CacheContext()

			vault, principalAddr := s.setupAssetSettlementVault(underlying, share, paymentDenom)
			vaultAddr := vault.GetAddress()
			if tc.registerAssetMarker {
				s.requireSimpleMarker(asset)
			}

			if tc.seedNav != nil {
				s.Require().NoError(
					s.k.SetVaultNAV(s.ctx, vault, *tc.seedNav, s.adminAddr.String()),
					"failed to seed internal NAV for denom %s", tc.seedNav.Denom,
				)
			}

			source := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1_000))
			s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, source, tc.fundSource), "failed to fund source with %s", tc.fundSource)
			s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, principalAddr, tc.fundPrincipal), "failed to fund principal with %s", tc.fundPrincipal)

			s.createPayment(source, vaultAddr, tc.sourceAmount, tc.targetAmount, externalID)

			s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
			_, err := keeper.NewMsgServer(s.simApp.VaultKeeper).AcceptAsset(s.ctx, &types.MsgAcceptAssetRequest{
				Authority:    s.adminAddr.String(),
				VaultAddress: vaultAddr.String(),
				Source:       source.String(),
				ExternalId:   externalID,
			})

			if tc.expectedErrContains != "" {
				s.Require().ErrorContains(err, tc.expectedErrContains, "AcceptAsset should fail for case %q", tc.name)
				return
			}
			s.Require().NoError(err, "AcceptAsset should succeed for case %q", tc.name)

			eventIndex := map[string]int{}
			for i, ev := range s.ctx.EventManager().Events() {
				if _, seen := eventIndex[ev.Type]; !seen {
					eventIndex[ev.Type] = i
				}
			}
			acceptedIdx, ok := eventIndex["provlabs.vault.v1.EventAssetAccepted"]
			s.Require().True(ok, "EventAssetAccepted should be emitted for case %q", tc.name)
			updatedIdx, ok := eventIndex["provlabs.vault.v1.EventNAVUpdated"]
			s.Require().True(ok, "EventNAVUpdated should be emitted for case %q", tc.name)
			s.Assert().Less(acceptedIdx, updatedIdx, "EventAssetAccepted should be emitted before EventNAVUpdated")
			s.requireTypedEventEmitted(markertypes.NewEventSetNetAssetValue(asset, tc.expectedNavPrice, tc.expectedNavVolume.Uint64(), vaultAddr.String()))

			if tc.expectNavRemoved {
				_, err := s.k.GetVaultNAV(s.ctx, vaultAddr, asset)
				s.Assert().ErrorIs(err, collections.ErrNotFound, "NAV entry for %s should be removed after draining the principal", asset)
				removedIdx, ok := eventIndex["provlabs.vault.v1.EventNAVRemoved"]
				s.Require().True(ok, "EventNAVRemoved should be emitted for case %q", tc.name)
				s.Assert().Less(updatedIdx, removedIdx, "EventNAVUpdated should be emitted before EventNAVRemoved")
				s.requireTypedEventEmitted(types.NewEventNAVRemoved(vaultAddr.String(), types.VaultNAV{
					Denom:  asset,
					Price:  tc.expectedNavPrice,
					Volume: tc.expectedNavVolume,
				}))
				return
			}

			s.Assert().NotContains(eventIndex, "provlabs.vault.v1.EventNAVRemoved", "EventNAVRemoved should not be emitted for case %q", tc.name)
			stored, err := s.k.GetVaultNAV(s.ctx, vaultAddr, asset)
			s.Require().NoError(err, "NAV entry for %s should exist after settlement", asset)
			s.Assert().Equal(tc.expectedNavPrice, stored.Price, "stored NAV price mismatch for case %q", tc.name)
			s.Assert().Equal(tc.expectedNavVolume, stored.Volume, "stored NAV volume mismatch for case %q", tc.name)
			s.Assert().Equal(vaultAddr.String(), stored.Source, "stored NAV source should be the vault address for case %q", tc.name)
			s.requireTypedEventEmitted(types.NewEventNAVUpdated(vaultAddr.String(), stored, s.adminAddr.String()))
		})
	}
}

func (s *TestSuite) TestMsgServer_AcceptAsset_Reconcile() {
	underlying := "under"
	share := "vshare"
	paymentDenom := "pay"
	asset := "rwacoin"
	externalID := "settle-reconcile"

	tests := []struct {
		name                string
		interestRate        string
		seedNav             *types.VaultNAV
		expectedErrContains string
		expectedReconciles  int
	}{
		{
			name:               "successful settlement with an elapsed interest period reconciles exactly once, before the transfers",
			interestRate:       "4.20",
			expectedReconciles: 1,
		},
		{
			name:                "settlement rejected by the NAV guardrail does not reconcile",
			interestRate:        "4.20",
			seedNav:             &types.VaultNAV{Denom: asset, Price: sdk.NewInt64Coin(paymentDenom, 9), Volume: sdkmath.NewInt(10)},
			expectedErrContains: "does not match internal NAV",
			expectedReconciles:  0,
		},
		{
			name:                "failed reconcile aborts the settlement",
			interestRate:        "1000000.0",
			expectedErrContains: "failed to reconcile vault before settlement",
			expectedReconciles:  0,
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			origCtx := s.ctx
			defer func() { s.ctx = origCtx }()
			s.ctx, _ = s.ctx.CacheContext()

			vault, principalAddr := s.setupAssetSettlementVault(underlying, share, paymentDenom)
			vaultAddr := vault.GetAddress()
			s.requireSimpleMarker(asset)

			if tc.seedNav != nil {
				s.Require().NoError(
					s.k.SetVaultNAV(s.ctx, vault, *tc.seedNav, s.adminAddr.String()),
					"failed to seed internal NAV for denom %s", tc.seedNav.Denom,
				)
			}

			// An elapsed interest period makes the reconcile observable via EventVaultReconcile.
			vault.CurrentInterestRate = tc.interestRate
			vault.DesiredInterestRate = tc.interestRate
			vault.PeriodStart = s.ctx.BlockTime().Add(-3 * time.Hour).Unix()
			s.k.AuthKeeper.SetAccount(s.ctx, vault)

			source := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1_000))
			s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, source, sdk.NewCoins(sdk.NewInt64Coin(asset, 10))), "failed to fund source with the settlement asset")
			s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, principalAddr, sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 5))), "failed to fund principal with the settlement payment")
			s.createPayment(source, vaultAddr, sdk.NewCoins(sdk.NewInt64Coin(asset, 10)), sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 5)), externalID)

			s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
			_, err := keeper.NewMsgServer(s.simApp.VaultKeeper).AcceptAsset(s.ctx, &types.MsgAcceptAssetRequest{
				Authority:    s.adminAddr.String(),
				VaultAddress: vaultAddr.String(),
				Source:       source.String(),
				ExternalId:   externalID,
			})

			reconcileCount := 0
			reconcileIdx := -1
			acceptedIdx := -1
			for i, ev := range s.ctx.EventManager().Events() {
				switch ev.Type {
				case "provlabs.vault.v1.EventVaultReconcile":
					reconcileCount++
					reconcileIdx = i
				case "provlabs.vault.v1.EventAssetAccepted":
					acceptedIdx = i
				}
			}

			if tc.expectedErrContains != "" {
				s.Require().ErrorContains(err, tc.expectedErrContains, "AcceptAsset should fail for case %q", tc.name)
			} else {
				s.Require().NoError(err, "AcceptAsset should succeed for case %q", tc.name)
			}
			s.Assert().Equal(tc.expectedReconciles, reconcileCount, "EventVaultReconcile count mismatch for case %q", tc.name)
			if tc.expectedErrContains == "" && tc.expectedReconciles > 0 {
				s.Require().GreaterOrEqual(acceptedIdx, 0, "EventAssetAccepted should be emitted for case %q", tc.name)
				s.Assert().Less(reconcileIdx, acceptedIdx, "reconcile events should precede EventAssetAccepted for case %q", tc.name)
			}
		})
	}
}

func (s *TestSuite) TestMsgServer_AcceptAsset_InsufficientPrincipalDoesNotSettle() {
	underlying, share, paymentDenom, asset := "under", "vshare", "pay", "rwacoin"
	externalID := "short-noop"

	vault, principalAddr := s.setupAssetSettlementVault(underlying, share, paymentDenom)
	vaultAddr := vault.GetAddress()

	source := s.CreateAndFundAccount(sdk.NewInt64Coin(asset, 10))
	s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, principalAddr, sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 4))), "fund principal short")
	s.createPayment(source, vaultAddr, sdk.NewCoins(sdk.NewInt64Coin(asset, 10)), sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 5)), externalID)

	_, err := keeper.NewMsgServer(s.simApp.VaultKeeper).AcceptAsset(s.ctx, &types.MsgAcceptAssetRequest{
		Authority:    s.adminAddr.String(),
		VaultAddress: vaultAddr.String(),
		Source:       source.String(),
		ExternalId:   externalID,
	})
	s.Require().Error(err, "AcceptAsset should fail when principal is short")

	stored, getErr := s.simApp.ExchangeKeeper.GetPayment(s.ctx, source, externalID)
	s.Require().NoError(getErr, "GetPayment after failed settle")
	s.Require().NotNil(stored, "payment should remain in escrow after a failed settle")
	s.assertBalance(source, asset, sdkmath.NewInt(10))
	s.assertBalance(principalAddr, paymentDenom, sdkmath.NewInt(4))
}

func (s *TestSuite) TestMsgServer_RejectAsset() {
	underlying, share, paymentDenom, asset := "under", "vshare", "pay", "rwacoin"
	externalID := "p2p-reject"

	vault, _ := s.setupAssetSettlementVault(underlying, share, paymentDenom)
	vaultAddr := vault.GetAddress()

	source := s.CreateAndFundAccount(sdk.NewInt64Coin(asset, 10))
	s.createPayment(source, vaultAddr, sdk.NewCoins(sdk.NewInt64Coin(asset, 10)), sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 5)), externalID)

	s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	resp, err := keeper.NewMsgServer(s.simApp.VaultKeeper).RejectAsset(s.ctx, &types.MsgRejectAssetRequest{
		Authority:    s.adminAddr.String(),
		VaultAddress: vaultAddr.String(),
		Source:       source.String(),
		ExternalId:   externalID,
	})
	s.Require().NoError(err, "RejectAsset should succeed")
	s.Assert().Equal(&types.MsgRejectAssetResponse{}, resp, "RejectAsset response")

	stored, getErr := s.simApp.ExchangeKeeper.GetPayment(s.ctx, source, externalID)
	s.Require().NoError(getErr, "GetPayment after reject")
	s.Assert().Nil(stored, "payment should be deleted after reject")
	s.assertBalance(source, asset, sdkmath.NewInt(10))

	s.requireTypedEventEmitted(types.NewEventAssetRejected(vaultAddr.String(), source.String(), externalID))
}

func (s *TestSuite) TestMsgServer_RejectAsset_Failures() {
	underlying, share, paymentDenom, asset := "under", "vshare", "pay", "rwacoin"

	tests := []struct {
		name               string
		setup              func() *types.MsgRejectAssetRequest
		expectedErrSubstrs []string
	}{
		{
			name: "vault does not exist",
			setup: func() *types.MsgRejectAssetRequest {
				source := s.CreateAndFundAccount(sdk.NewInt64Coin(asset, 10))
				return &types.MsgRejectAssetRequest{
					Authority:    s.adminAddr.String(),
					VaultAddress: types.GetVaultAddress("missing").String(),
					Source:       source.String(),
					ExternalId:   "none",
				}
			},
			expectedErrSubstrs: []string{"not found"},
		},
		{
			name: "unauthorized signer",
			setup: func() *types.MsgRejectAssetRequest {
				vault, _ := s.setupAssetSettlementVault(underlying, share, paymentDenom)
				vaultAddr := vault.GetAddress()
				source := s.CreateAndFundAccount(sdk.NewInt64Coin(asset, 10))
				stranger := s.CreateAndFundAccount(sdk.NewInt64Coin(underlying, 1))
				s.createPayment(source, vaultAddr, sdk.NewCoins(sdk.NewInt64Coin(asset, 10)), sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 5)), "unauth")
				return &types.MsgRejectAssetRequest{
					Authority:    stranger.String(),
					VaultAddress: vaultAddr.String(),
					Source:       source.String(),
					ExternalId:   "unauth",
				}
			},
			expectedErrSubstrs: []string{"unauthorized authority"},
		},
		{
			name: "payment does not exist",
			setup: func() *types.MsgRejectAssetRequest {
				vault, _ := s.setupAssetSettlementVault(underlying, share, paymentDenom)
				vaultAddr := vault.GetAddress()
				source := s.CreateAndFundAccount(sdk.NewInt64Coin(asset, 10))
				return &types.MsgRejectAssetRequest{
					Authority:    s.adminAddr.String(),
					VaultAddress: vaultAddr.String(),
					Source:       source.String(),
					ExternalId:   "ghost",
				}
			},
			expectedErrSubstrs: []string{"failed to reject payment"},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			origCtx := s.ctx
			defer func() { s.ctx = origCtx }()
			s.ctx, _ = s.ctx.CacheContext()

			msg := tc.setup()
			s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())

			resp, err := keeper.NewMsgServer(s.simApp.VaultKeeper).RejectAsset(s.ctx, msg)
			s.Require().Error(err, "RejectAsset should fail for case %q", tc.name)
			s.Assert().Nil(resp, "RejectAsset response should be nil on error for case %q", tc.name)
			for _, substr := range tc.expectedErrSubstrs {
				s.Assert().Containsf(err.Error(), substr, "RejectAsset error missing expected substring for case %q", tc.name)
			}
		})
	}
}
