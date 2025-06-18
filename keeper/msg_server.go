package keeper

import (
	"context"

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

// CreateVault creates a new vault.
func (k msgServer) CreateVault(goCtx context.Context, msg *types.MsgCreateVaultRequest) (*types.MsgCreateVaultResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// TODO We will want to have the denom rather than the address in the message.
	denom := msg.MarkerAddress
	// TODO We probably want to use the module account as the owner.
	owner := msg.Admin

	const (
		// TODO We may want the supply in the message.
		Supply          = 10_000
		FixedSupply     = true
		NoForceTransfer = false
		NoGovControl    = false
	)
	rMarkerBaseAcct := authtypes.NewBaseAccountWithAddress(markertypes.MustGetMarkerAddress(denom))
	rMarkerAcct := markertypes.NewMarkerAccount(rMarkerBaseAcct, sdk.NewInt64Coin(denom, Supply), sdk.MustAccAddressFromBech32(owner),
		[]markertypes.AccessGrant{
			{
				Address:     owner,
				Permissions: []markertypes.Access{markertypes.Access_Admin, markertypes.Access_Transfer, markertypes.Access_Mint, markertypes.Access_Burn, markertypes.Access_Withdraw},
			},
		},
		markertypes.StatusProposed,
		markertypes.MarkerType_RestrictedCoin,
		FixedSupply,
		NoGovControl,
		NoForceTransfer,
		[]string{},
	)
	k.MarkerKeeper.AddFinalizeAndActivateMarker(ctx, rMarkerAcct)
	return &types.MsgCreateVaultResponse{
		VaultAddress: rMarkerAcct.Address,
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
