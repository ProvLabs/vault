package simulation

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/provenance-io/provenance/x/marker/keeper"
	"github.com/provenance-io/provenance/x/marker/types"
)

func CreateMarker(ctx context.Context, coin sdk.Coin, admin sdk.AccAddress, markerkeeper keeper.Keeper) error {
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
					types.Access_Mint, types.Access_Burn, types.Access_Withdraw, types.Access_Admin,
					types.Access_Transfer, types.Access_Deposit, types.Access_Delete,
				},
			},
		},
		SupplyFixed:            true,
		AllowGovernanceControl: true,
		AllowForcedTransfer:    false,
		RequiredAttributes:     nil,
	}
	markerMsgServer := keeper.NewMsgServerImpl(markerkeeper)
	_, err := markerMsgServer.AddFinalizeActivateMarker(ctx, newMarker)
	return err
}
