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

// Obtain the marker for UnderlyingAsset
// Obtain the ModuleAccount
// Create a marker named vault/${UnderlyingAsset}
// It should have the following attributes:
// - The marker owner should be the ModuleAccount
// - Must have supply of zero
// - Must not be fixed supply
// - Must not have governance control
// - Must not have forced transfer
// - Must be restricted
// - Must have Admin, Mint, Burn, Withdraw, Transfer
// - Must have correct ending marker state
// The Vault Store should be updated containing the following mapping [MarkerAddress] -> Vault
// The Vault should have correctly populated fields
// The Vault should have the following attributes:
// - vault_address containing the address of the newly created marker
// - underlying_asset containing the name of the underlying asset for the vault
// - admin contains the address of the vault admin
// The EventVaultCreated event should be emitted containing:
// - The admin of the vault
// - The vault address
// - The underlying asset
// It should return a MsgCreateVaultResponse containing the new marker address

// ValidateBasic
// Verify the admin is valid syntax
// Verify the string share denom is valid syntax
// Verify the string for supported coin is valid syntax

// Mandatory tests in msg_server_test, query_server_test, msgs_test, and genesis
// Complicated code in keepers should have tests
func (k msgServer) CreateVault(goCtx context.Context, msg *types.MsgCreateVaultRequest) (*types.MsgCreateVaultResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	marker, err := k.MarkerKeeper.GetMarker(ctx, markertypes.MustGetMarkerAddress(msg.UnderlyingAsset))
	if err != nil {
		return nil, fmt.Errorf("unable to find underlying asset: %w", err)
	}
	vaultDenom := fmt.Sprintf("vault/%s", marker.GetDenom())

	// TODO Does the owner just own the vault, and the marker created by the vault?
	// TODO Should these be separate owners?
	owner := msg.Admin

	const (
		// TODO We may want the supply in the message.
		Supply          = 10_000
		FixedSupply     = true
		NoForceTransfer = false
		NoGovControl    = false
	)
	rMarkerBaseAcct := authtypes.NewBaseAccountWithAddress(markertypes.MustGetMarkerAddress(vaultDenom))
	rMarkerAcct := markertypes.NewMarkerAccount(rMarkerBaseAcct, sdk.NewInt64Coin(vaultDenom, Supply), sdk.MustAccAddressFromBech32(owner),
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
