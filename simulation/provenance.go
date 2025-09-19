package simulation

import (
	"context"
	"fmt"

	"github.com/provlabs/vault/interest"
	"github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/simapp"
	"github.com/provlabs/vault/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"

	attrkeeper "github.com/provenance-io/provenance/x/attribute/keeper"
	attrtypes "github.com/provenance-io/provenance/x/attribute/types"
	markerkeeper "github.com/provenance-io/provenance/x/marker/keeper"
	markertypes "github.com/provenance-io/provenance/x/marker/types"
	namekeeper "github.com/provenance-io/provenance/x/name/keeper"
)

const (
	RequiredMarkerAttribute = "kyc.jackthecat.vault"
)

func CreateMarker(ctx context.Context, coin sdk.Coin, admin sdk.AccAddress, keeper markerkeeper.Keeper) error {
	// Add a marker with deposit permissions so that it can be found by the sim.
	newMarker := &markertypes.MsgAddFinalizeActivateMarkerRequest{
		Amount:      coin,
		Manager:     admin.String(),
		FromAddress: admin.String(),
		MarkerType:  markertypes.MarkerType_RestrictedCoin,
		AccessList: []markertypes.AccessGrant{
			{
				Address: admin.String(),
				Permissions: markertypes.AccessList{
					markertypes.Access_Mint, markertypes.Access_Burn, markertypes.Access_Withdraw,
				},
			},
		},
		SupplyFixed:            true,
		AllowGovernanceControl: true,
		AllowForcedTransfer:    false,
		RequiredAttributes:     []string{RequiredMarkerAttribute},
	}
	markerMsgServer := markerkeeper.NewMsgServerImpl(keeper)
	_, err := markerMsgServer.AddFinalizeActivateMarker(ctx, newMarker)
	return err
}

func AddAttribute(ctx context.Context, acc sdk.AccAddress, attr string, nk namekeeper.Keeper, ak attrkeeper.Keeper) error {
	err := nk.SetNameRecord(sdk.UnwrapSDKContext(ctx), attr, acc, false)
	if err != nil {
		return err
	}

	newAttr := &attrtypes.MsgAddAttributeRequest{
		Name:          attr,
		Value:         []byte("abc"),
		AttributeType: attrtypes.AttributeType_String,
		Account:       acc.String(),
		Owner:         acc.String(),
	}
	attrMsgServer := attrkeeper.NewMsgServerImpl(ak)
	_, err = attrMsgServer.AddAttribute(ctx, newAttr)
	return err
}

// CreateVault creates a new vault with a marker and funds accounts.
func CreateVault(ctx sdk.Context, app *simapp.SimApp, underlying, share string, admin simtypes.Account, accs []simtypes.Account) error {
	if !MarkerExists(ctx, app, underlying) {
		if err := CreateGlobalMarker(ctx, app, sdk.NewInt64Coin(underlying, 1000), accs); err != nil {
			return fmt.Errorf("unable to create global marker: %w", err)
		}
	}

	// Create a vault that uses the marker as an underlying asset
	newVault := &types.MsgCreateVaultRequest{
		Admin:                  admin.Address.String(),
		ShareDenom:             share,
		UnderlyingAsset:        underlying,
		PaymentDenom:           "",
		WithdrawalDelaySeconds: interest.SecondsPerDay,
	}
	msgServer := keeper.NewMsgServer(app.VaultKeeper)
	_, err := msgServer.CreateVault(sdk.WrapSDKContext(ctx), newVault)
	return err
}

// SwapIn performs a swap in for a user.
func SwapIn(ctx sdk.Context, app *simapp.SimApp, user simtypes.Account, shareDenom string, amount sdk.Coin) (*types.MsgSwapInResponse, error) {
	vaultAddress := types.GetVaultAddress(shareDenom)
	swapIn := &types.MsgSwapInRequest{
		Owner:        user.Address.String(),
		VaultAddress: vaultAddress.String(),
		Assets:       amount,
	}
	msgServer := keeper.NewMsgServer(app.VaultKeeper)
	return msgServer.SwapIn(ctx, swapIn)
}

func MarkerExists(ctx sdk.Context, app *simapp.SimApp, denom string) bool {
	marker, err := app.MarkerKeeper.GetMarker(ctx, markertypes.MustGetMarkerAddress(denom))
	return marker != nil && err == nil
}

func CreateGlobalMarker(ctx sdk.Context, app *simapp.SimApp, underlying sdk.Coin, accs []simtypes.Account) error {
	err := CreateMarker(sdk.UnwrapSDKContext(ctx), sdk.NewInt64Coin(underlying.Denom, underlying.Amount.Int64()), app.AccountKeeper.GetModuleAddress("mint"), app.MarkerKeeper)
	if err != nil {
		return fmt.Errorf("CreateMarker: %w", err)
	}

	for _, acc := range accs {
		amount := underlying.Amount.Quo(math.NewInt(int64(len(accs))))
		err = FundAccount(ctx, app, acc.Address, sdk.NewCoins(sdk.NewInt64Coin(underlying.Denom, amount.Int64())))
		if err != nil {
			return fmt.Errorf("FundAccount for %s: %w", acc.Address, err)
		}
	}

	return nil
}

func FundAccount(ctx context.Context, app *simapp.SimApp, addr sdk.AccAddress, amounts sdk.Coins) error {
	if err := app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, amounts); err != nil {
		return err
	}
	ctx = markertypes.WithBypass(ctx) // Bypass marker checks for this operation.
	return app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, addr, amounts)
}

// PauseVault pauses a vault.
func PauseVault(ctx sdk.Context, app *simapp.SimApp, shareDenom string) error {
	vaultAddress := types.GetVaultAddress(shareDenom)
	vault, err := app.VaultKeeper.GetVault(ctx, vaultAddress)
	if err != nil {
		return err
	}
	msgServer := keeper.NewMsgServer(app.VaultKeeper)
	_, err = msgServer.PauseVault(ctx, &types.MsgPauseVaultRequest{Admin: vault.Admin, VaultAddress: vault.Address, Reason: "test"})
	return err
}
