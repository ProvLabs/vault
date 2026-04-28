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
	RequiredMarkerAttribute = "simulation.restricted"
)

// CreateMarker creates a new restricted marker of type COIN.
func CreateMarker(ctx context.Context, coin sdk.Coin, admin sdk.AccAddress, keeper markerkeeper.Keeper, feeCollector sdk.AccAddress, accs []simtypes.Account) error {
	accessList := []markertypes.AccessGrant{
		{
			Address: admin.String(),
			Permissions: markertypes.AccessList{
				markertypes.Access_Mint, markertypes.Access_Burn, markertypes.Access_Withdraw, markertypes.Access_Admin,
			},
		},
		{
			Address: feeCollector.String(),
			Permissions: markertypes.AccessList{
				markertypes.Access_Transfer,
			},
		},
	}

	for _, acc := range accs {
		accessList = append(accessList, markertypes.AccessGrant{
			Address: acc.Address.String(),
			Permissions: markertypes.AccessList{
				markertypes.Access_Withdraw,
			},
		})
	}

	// Add a marker with deposit permissions so that it can be found by the sim.
	newMarker := &markertypes.MsgAddFinalizeActivateMarkerRequest{
		Amount:             coin,
		Manager:            admin.String(),
		FromAddress:        admin.String(),
		MarkerType:         markertypes.MarkerType_RestrictedCoin,
		AccessList:         accessList,
		SupplyFixed:        true,
		AllowGovernanceControl: true,
		AllowForcedTransfer: false,
		RequiredAttributes: []string{RequiredMarkerAttribute},
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
					markertypes.Access_Mint, markertypes.Access_Burn, markertypes.Access_Withdraw, markertypes.Access_Admin,
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

// GrantTransferPermission grants transfer permission to an account for a given marker.
func GrantTransferPermission(ctx context.Context, keeper markerkeeper.Keeper, denom string, grantee sdk.AccAddress, admin sdk.AccAddress) error {
	return GrantAccess(ctx, keeper, denom, grantee, admin, markertypes.AccessList{markertypes.Access_Transfer})
}

// GrantWithdrawPermission grants withdraw permission to an account for a given marker.
func GrantWithdrawPermission(ctx context.Context, keeper markerkeeper.Keeper, denom string, grantee sdk.AccAddress, admin sdk.AccAddress) error {
	return GrantAccess(ctx, keeper, denom, grantee, admin, markertypes.AccessList{markertypes.Access_Withdraw})
}

// GrantAccess grants specified permissions to an account for a given marker.
func GrantAccess(ctx context.Context, keeper markerkeeper.Keeper, denom string, grantee sdk.AccAddress, admin sdk.AccAddress, access markertypes.AccessList) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	m, err := keeper.GetMarker(sdkCtx, markertypes.MustGetMarkerAddress(denom))
	if err != nil {
		return fmt.Errorf("failed to get marker %s: %w", denom, err)
	}
	if m == nil {
		return fmt.Errorf("marker %s not found", denom)
	}

	if err := m.GrantAccess(markertypes.NewAccessGrant(grantee, access)); err != nil {
		return fmt.Errorf("failed to grant access to %s on %s: %w", grantee, denom, err)
	}
	keeper.SetMarker(sdkCtx, m)
	return nil
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

// AddAttribute adds an attribute to an account using the provided owner to authorize the action.
func AddAttribute(ctx context.Context, owner sdk.AccAddress, acc sdk.AccAddress, attrName string, nk types.NameKeeper, ak types.AttributeKeeper) error {
	attr := NewAttribute(
		attrName,
		acc.String(),
		attrtypes.AttributeType_String,
		[]byte("abc"),
		nil,
	)

	return ak.SetAttribute(sdk.UnwrapSDKContext(ctx), attr, owner)
}

// BindName ensures that a name is bound to an address if it doesn't already exist.
// If the name has multiple segments (separated by dots), it attempts to bind
// each parent segment recursively from right to left.
func BindName(ctx context.Context, acc sdk.AccAddress, name string, nk types.NameKeeper) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	segments := strings.Split(name, ".")
	for i := len(segments); i > 0; i-- {
		currentName := strings.Join(segments[i-1:], ".")
		if !nk.NameExists(sdkCtx, currentName) {
			err := nk.SetNameRecord(sdkCtx, currentName, acc, false)
			if err != nil && !strings.Contains(err.Error(), "already bound") {
				return fmt.Errorf("failed to bind name %s: %w", currentName, err)
			}
		}
	}
	return nil
}

// MarkerExists checks if a marker with the given denom exists.
func MarkerExists(ctx sdk.Context, markerKeeper types.MarkerKeeper, denom string) bool {
	marker, err := markerKeeper.GetMarker(ctx, markertypes.MustGetMarkerAddress(denom))
	return marker != nil && err == nil
}

// CreateGlobalMarker creates a new marker and distributes its coins to a given set of accounts.
func CreateGlobalMarker(ctx sdk.Context, ak types.AccountKeeper, bk types.BankKeeper, mk markerkeeper.Keeper, underlying sdk.Coin, accs []simtypes.Account, restricted bool, feeCollector sdk.AccAddress) error {
	var err error
	if restricted {
		err = CreateMarker(sdk.UnwrapSDKContext(ctx), sdk.NewInt64Coin(underlying.Denom, underlying.Amount.Int64()), ak.GetModuleAddress("mint"), mk, feeCollector, accs)
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
