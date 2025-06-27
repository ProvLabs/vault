package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
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

	vaultAddr := types.GetVaultAddress(msg.ShareDenom)
	vault := types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(vaultAddr), msg.Admin, msg.ShareDenom, []string{msg.UnderlyingAsset})
	if err := k.SetVault(ctx, vault); err != nil {
		return nil, fmt.Errorf("failed to store new vault: %w", err)
	}
	vaultAcc := k.AuthKeeper.GetAccount(ctx, vaultAddr)
	if vaultAcc != nil {
		_, ok := vaultAcc.(types.VaultAccountI)
		if ok {
			return nil, fmt.Errorf("vault address already exists for %s", vaultAddr.String())
		} else if vaultAcc.GetSequence() > 0 {
			// account exists, is not a vault, and has been signed for
			return nil, fmt.Errorf("account at %s is not a vault account", vaultAddr.String())
		}
	}
	vaultAcc = k.AuthKeeper.NewAccount(ctx, vault).(types.VaultAccountI)
	k.AuthKeeper.SetAccount(ctx, vaultAcc)

	_, err := k.CreateVaultMarker(ctx, vaultAddr, msg.ShareDenom, msg.UnderlyingAsset)
	if err != nil {
		return nil, fmt.Errorf("failed to create vault marker: %w", err)
	}

	k.emitEvent(ctx, types.NewEventVaultCreated(
		vault.Address,
		msg.Admin,
		msg.ShareDenom,
		msg.UnderlyingAsset,
	))

	return &types.MsgCreateVaultResponse{
		VaultAddress: vault.Address,
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
