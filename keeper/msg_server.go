package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	markertypes "github.com/provenance-io/provenance/x/marker/types"
	"github.com/provlabs/vault/types"
)

var _ types.MsgServer = &msgServer{}

type msgServer struct {
	*Keeper
}

func NewMsgServer(keeper *Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

func (k msgServer) CreateVault(goCtx context.Context, msg *types.MsgCreateVaultRequest) (*types.MsgCreateVaultResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	underlyingAssetAddr := markertypes.MustGetMarkerAddress(msg.UnderlyingAsset)
	if found := k.MarkerKeeper.IsMarkerAccount(ctx, underlyingAssetAddr); !found {
		return nil, fmt.Errorf("underlying asset marker %q not found", msg.UnderlyingAsset)
	}
	// TODO How to check if it is a properly setup marker

	marker, err := k.CreateVaultMarker(ctx, msg.Admin, msg.ShareDenom, msg.UnderlyingAsset)
	if err != nil {
		return nil, fmt.Errorf("failed to create vault marker: %w", err)
	}

	vault := types.NewVault(msg.Admin, marker.GetAddress().String(), msg.UnderlyingAsset)

	if err := k.SetVault(ctx, vault); err != nil {
		return nil, fmt.Errorf("failed to store new vault: %w", err)
	}

	if err := k.eventService.EventManager(ctx).Emit(ctx, types.NewEventVaultCreated(
		vault.VaultAddress,
		msg.Admin,
		msg.ShareDenom,
		msg.UnderlyingAsset,
	)); err != nil {
		k.getLogger(ctx).Error("failed to emit EventVaultCreated", "error", err)
	}

	return &types.MsgCreateVaultResponse{
		VaultAddress: vault.VaultAddress,
	}, nil
}

// Deposit deposits assets into a vault.
func (k msgServer) Deposit(goCtx context.Context, msg *types.MsgDepositRequest) (*types.MsgDepositResponse, error) {
	panic("not implemented")
}

// Withdraw withdraws assets from a vault.
func (k msgServer) Withdraw(goCtx context.Context, msg *types.MsgWithdrawRequest) (*types.MsgWithdrawResponse, error) {
	panic("not implemented")
}

// Redeem redeems shares for underlying assets.
func (k msgServer) Redeem(goCtx context.Context, msg *types.MsgRedeemRequest) (*types.MsgRedeemResponse, error) {
	panic("not implemented")
}

// UpdateParams updates the params for the module.
func (k msgServer) UpdateParams(ctx context.Context, req *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	panic("not implemented")
}
