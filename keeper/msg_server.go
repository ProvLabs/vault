package keeper

import (
	"context"

	"github.com/provlabs/vault/types"
)

var _ types.MsgServer = &msgServer{}

type msgServer struct {
	*Keeper
}

func NewMsgServer(keeper *Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

// CreateVault creates a new vault.
func (k msgServer) CreateVault(goCtx context.Context, msg *types.MsgCreateVaultRequest) (*types.MsgCreateVaultResponse, error) {
	panic("not implemented")

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
