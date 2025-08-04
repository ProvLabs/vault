package keeper

import (
	"context"
	"fmt"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/provlabs/vault/types"
)

var _ types.MsgServer = &msgServer{}

type msgServer struct {
	*Keeper
}

func NewMsgServer(keeper *Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

// CreateVault creates a vault.
func (k msgServer) CreateVault(goCtx context.Context, msg *types.MsgCreateVaultRequest) (*types.MsgCreateVaultResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	vault, err := k.Keeper.CreateVault(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("failed to create vault: %w", err)
	}

	return &types.MsgCreateVaultResponse{
		VaultAddress: vault.Address,
	}, nil
}

// SwapIn handles depositing underlying assets into a vault and mints vault shares to the recipient.
func (k msgServer) SwapIn(goCtx context.Context, msg *types.MsgSwapInRequest) (*types.MsgSwapInResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	vaultAddr := sdk.MustAccAddressFromBech32(msg.VaultAddress)
	ownerAddr := sdk.MustAccAddressFromBech32(msg.Owner)

	shares, err := k.Keeper.SwapIn(ctx, vaultAddr, ownerAddr, msg.Assets)
	if err != nil {
		return nil, err
	}

	return &types.MsgSwapInResponse{SharesReceived: *shares}, nil
}

// SwapOut handles redeeming vault shares for underlying assets and transfers the assets to the recipient.
func (k msgServer) SwapOut(goCtx context.Context, msg *types.MsgSwapOutRequest) (*types.MsgSwapOutResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	vaultAddr := sdk.MustAccAddressFromBech32(msg.VaultAddress)
	ownerAddr := sdk.MustAccAddressFromBech32(msg.Owner)

	shares, err := k.Keeper.SwapOut(ctx, vaultAddr, ownerAddr, msg.Assets)
	if err != nil {
		return nil, err
	}

	return &types.MsgSwapOutResponse{SharesBurned: *shares}, nil
}

// UpdateParams updates the params for the module.
func (k msgServer) UpdateParams(ctx context.Context, req *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	panic("not implemented")
}

func (k msgServer) UpdateInterestRate(goCtx context.Context, msg *types.MsgUpdateInterestRateRequest) (*types.MsgUpdateInterestRateResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	vaultAddr := sdk.MustAccAddressFromBech32(msg.VaultAddress)
	vault, err := k.GetVault(ctx, vaultAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get vault: %w", err)
	}
	if vault == nil {
		return nil, fmt.Errorf("vault not found: %s", msg.VaultAddress)
	}

	if vault.Admin != msg.Admin {
		return nil, fmt.Errorf("unauthorized: %s is not the interest admin", msg.Admin)
	}

	if vault.CurrentInterestRate != "" {
		currRate, err := sdkmath.LegacyNewDecFromStr(vault.CurrentInterestRate)
		if err != nil {
			return nil, fmt.Errorf("invalid stored interest rate: %w", err)
		}
		if !currRate.IsZero() {
			if err := k.ReconcileVaultInterest(ctx, vault); err != nil {
				return nil, fmt.Errorf("failed to reconcile before rate change: %w", err)
			}
		}
	}

	k.UpdateInterestRates(ctx, vault, msg.NewRate, msg.NewRate)

	err = k.VaultInterestDetails.Set(ctx, vault.GetAddress(), types.VaultInterestDetails{PeriodStart: ctx.BlockTime().Unix()})
	if err != nil {
		return nil, fmt.Errorf("failed to set vault interest details: %w", err)
	}

	// k.emitEvent(ctx, types.NewEventInterestRateUpdated(msg.VaultAddress, msg.InterestAdmin, newRate.String()))

	return &types.MsgUpdateInterestRateResponse{}, nil
}

func (k msgServer) DepositInterestFunds(goCtx context.Context, msg *types.MsgDepositInterestFundsRequest) (*types.MsgDepositInterestFundsResponse, error) {
	return nil, fmt.Errorf("TODO")
}

func (k msgServer) WithdrawInterestFunds(goCtx context.Context, msg *types.MsgWithdrawInterestFundsRequest) (*types.MsgWithdrawInterestFundsResponse, error) {
	return nil, fmt.Errorf("TODO")
}

func (k msgServer) ToggleSwaps(goCtx context.Context, msg *types.MsgToggleSwapsRequest) (*types.MsgToggleSwapsResponse, error) {
	return nil, fmt.Errorf("TODO")
}
