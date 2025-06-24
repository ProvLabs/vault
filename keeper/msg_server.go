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

	// Obtain the marker for the underlying asset to ensure it exists.

	underlyingAssetAddr := markertypes.MustGetMarkerAddress(msg.UnderlyingAsset)
	if found := k.MarkerKeeper.IsMarkerAccount(ctx, underlyingAssetAddr); !found {
		return nil, fmt.Errorf("underlying asset marker %q not found", msg.UnderlyingAsset)
	}
	// TODO How to check if it is a properly setup marker

	// Obtain the module account, which will be the manager of the new vault marker.
	moduleAcc := authtypes.NewModuleAddress(types.ModuleName)

	// Create a new marker for the vault shares.
	vaultShareMarkerAddress := markertypes.MustGetMarkerAddress(msg.ShareDenom)
	if found := k.MarkerKeeper.IsMarkerAccount(ctx, vaultShareMarkerAddress); found {
		return nil, fmt.Errorf("a marker with the share denomination %q already exists", msg.ShareDenom)
	}

	const (
		Supply          = 0
		NoFixedSupply   = false
		NoForceTransfer = false
		NoGovControl    = false
	)

	// Create the new marker account for the vault shares.
	baseAccount := authtypes.NewBaseAccountWithAddress(vaultShareMarkerAddress)
	newMarker := markertypes.NewMarkerAccount(
		baseAccount,
		sdk.NewInt64Coin(msg.ShareDenom, Supply),
		moduleAcc, // The marker manager is the vault module account.
		[]markertypes.AccessGrant{
			{
				Address: moduleAcc.String(),
				Permissions: []markertypes.Access{
					markertypes.Access_Admin,
					markertypes.Access_Mint,
					markertypes.Access_Burn,
					markertypes.Access_Withdraw,
					markertypes.Access_Transfer,
				},
			},
		},
		markertypes.StatusProposed,
		markertypes.MarkerType_RestrictedCoin,
		NoFixedSupply,
		NoGovControl,
		NoForceTransfer,
		[]string{}, // No required attributes.
	)

	// Add, finalize, and activate the new marker.
	if err := k.MarkerKeeper.AddFinalizeAndActivateMarker(ctx, newMarker); err != nil {
		return nil, fmt.Errorf("failed to create and activate vault share marker: %w", err)
	}

	// Create and store the Vault object.
	vault := types.Vault{
		VaultAddress:    newMarker.GetAddress().String(),
		UnderlyingAsset: msg.UnderlyingAsset,
		Admin:           msg.Admin,
	}

	// The vault is keyed by its address, which is the new marker's address.
	if err := k.Vaults.Set(ctx, newMarker.GetAddress(), vault); err != nil {
		// If storing the vault fails after the marker has been created, we are in an inconsistent state.
		// This should ideally be handled, e.g., by attempting to delete the created marker.
		return nil, fmt.Errorf("failed to store new vault: %w", err)
	}

	// Emit the EventVaultCreated event.
	if err := k.eventService.EventManager(ctx).Emit(ctx, types.NewEventVaultCreated(
		newMarker.GetAddress(),
		msg.Admin,
		msg.ShareDenom,
		msg.UnderlyingAsset,
	)); err != nil {
		// Log the error, but don't fail the transaction as event emission is not critical.
		k.getLogger(ctx).Error("failed to emit EventVaultCreated", "error", err)
	}

	// Return the response containing the new vault's address.
	return &types.MsgCreateVaultResponse{
		VaultAddress: newMarker.GetAddress().String(),
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
