package simulation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/provlabs/vault/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"

	attrtypes "github.com/provenance-io/provenance/x/attribute/types"
	markerkeeper "github.com/provenance-io/provenance/x/marker/keeper"
	markertypes "github.com/provenance-io/provenance/x/marker/types"
)

const (
	RequiredMarkerAttribute = "kyc.jackthecat.vault"
)

// CreateMarker creates a new restricted marker of type COIN.
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

// CreateUnrestrictedMarker creates a new unrestricted marker of type COIN.
func CreateUnrestrictedMarker(ctx context.Context, coin sdk.Coin, admin sdk.AccAddress, keeper markerkeeper.Keeper) error {
	newMarker := &markertypes.MsgAddFinalizeActivateMarkerRequest{
		Amount:      coin,
		Manager:     admin.String(),
		FromAddress: admin.String(),
		MarkerType:  markertypes.MarkerType_Coin,
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
		RequiredAttributes:     []string{},
	}
	markerMsgServer := markerkeeper.NewMsgServerImpl(keeper)
	_, err := markerMsgServer.AddFinalizeActivateMarker(ctx, newMarker)
	return err
}

// AddNav adds a net asset value to a marker.
func AddNav(ctx context.Context, keeper markerkeeper.Keeper, denom string, admin sdk.AccAddress, price sdk.Coin, volume uint64) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	nav := []markertypes.NetAssetValue{
		{
			Price:              price,
			Volume:             volume,
			UpdatedBlockHeight: uint64(sdkCtx.BlockHeight()),
		},
	}

	msg := &markertypes.MsgAddNetAssetValuesRequest{
		Denom:          denom,
		Administrator:  admin.String(),
		NetAssetValues: nav,
	}

	markerMsgServer := markerkeeper.NewMsgServerImpl(keeper)
	_, err := markerMsgServer.AddNetAssetValues(ctx, msg)
	return err
}

// AddAttribute adds an attribute to an account.
func AddAttribute(ctx context.Context, acc sdk.AccAddress, attrName string, nk types.NameKeeper, ak types.AttributeKeeper) error {
	err := nk.SetNameRecord(sdk.UnwrapSDKContext(ctx), attrName, acc, false)
	if err != nil {
		return err
	}

	// name string, address string, attrType AttributeType, value []byte, expirationDate *time.Time, concreteType string
	attr := NewAttribute(
		attrName,
		acc.String(),
		attrtypes.AttributeType_String,
		[]byte("abc"),
		nil,
	)

	return ak.SetAttribute(sdk.UnwrapSDKContext(ctx), attr, acc)
}

// MarkerExists checks if a marker with the given denom exists.
func MarkerExists(ctx sdk.Context, markerKeeper types.MarkerKeeper, denom string) bool {
	marker, err := markerKeeper.GetMarker(ctx, markertypes.MustGetMarkerAddress(denom))
	return marker != nil && err == nil
}

// CreateGlobalMarker creates a new marker and distributes its coins to a given set of accounts.
func CreateGlobalMarker(ctx sdk.Context, ak types.AccountKeeper, bk types.BankKeeper, mk markerkeeper.Keeper, underlying sdk.Coin, accs []simtypes.Account, restricted bool) error {
	var err error
	if restricted {
		err = CreateMarker(sdk.UnwrapSDKContext(ctx), sdk.NewInt64Coin(underlying.Denom, underlying.Amount.Int64()), ak.GetModuleAddress("mint"), mk)
	} else {
		err = CreateUnrestrictedMarker(sdk.UnwrapSDKContext(ctx), sdk.NewInt64Coin(underlying.Denom, underlying.Amount.Int64()), ak.GetModuleAddress("mint"), mk)
	}

	if err != nil {
		return fmt.Errorf("CreateMarker: %w", err)
	}

	for _, acc := range accs {
		amount := underlying.Amount.Quo(math.NewInt(int64(len(accs))))
		err = FundAccount(ctx, bk, acc.Address, sdk.NewCoins(sdk.NewInt64Coin(underlying.Denom, amount.Int64())))
		if err != nil {
			return fmt.Errorf("FundAccount for %s: %w", acc.Address, err)
		}
	}

	return nil
}

// FundAccount mints new coins and sends them to an account.
func FundAccount(ctx context.Context, bk types.BankKeeper, addr sdk.AccAddress, amounts sdk.Coins) error {
	if err := bk.MintCoins(ctx, minttypes.ModuleName, amounts); err != nil {
		return err
	}
	ctx = markertypes.WithBypass(ctx) // Bypass marker checks for this operation.
	return bk.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, addr, amounts)
}

// NewAttribute creates a new instance of an Attribute.
func NewAttribute(name string, address string, attrType attrtypes.AttributeType, value []byte, expirationDate *time.Time) attrtypes.Attribute {
	// Ensure string type values are trimmed.
	if attrType != attrtypes.AttributeType_Bytes && attrType != attrtypes.AttributeType_Proto {
		trimmed := strings.TrimSpace(string(value))
		value = []byte(trimmed)
	}
	return attrtypes.Attribute{
		Name:           name,
		Address:        address,
		AttributeType:  attrType,
		Value:          value,
		ExpirationDate: expirationDate,
	}
}

