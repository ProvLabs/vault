package simulation

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	attrkeeper "github.com/provenance-io/provenance/x/attribute/keeper"
	attrtypes "github.com/provenance-io/provenance/x/attribute/types"
	markerkeeper "github.com/provenance-io/provenance/x/marker/keeper"
	"github.com/provenance-io/provenance/x/marker/types"
	namekeeper "github.com/provenance-io/provenance/x/name/keeper"
)

const (
	RequiredMarkerAttribute = "kyc.jackthecat.vault"
)

func CreateMarker(ctx context.Context, coin sdk.Coin, admin sdk.AccAddress, keeper markerkeeper.Keeper) error {
	// Add a marker with deposit permissions so that it can be found by the sim.
	newMarker := &types.MsgAddFinalizeActivateMarkerRequest{
		Amount:      coin,
		Manager:     admin.String(),
		FromAddress: admin.String(),
		MarkerType:  types.MarkerType_RestrictedCoin,
		AccessList: []types.AccessGrant{
			{
				Address: admin.String(),
				Permissions: types.AccessList{
					types.Access_Mint, types.Access_Burn, types.Access_Withdraw,
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
	nk.SetNameRecord(sdk.UnwrapSDKContext(ctx), attr, acc, false)

	newAttr := &attrtypes.MsgAddAttributeRequest{
		Name:          attr,
		Value:         []byte("abc"),
		AttributeType: attrtypes.AttributeType_String,
		Account:       acc.String(),
		Owner:         acc.String(),
	}
	attrMsgServer := attrkeeper.NewMsgServerImpl(ak)
	_, err := attrMsgServer.AddAttribute(ctx, newAttr)
	return err
}
