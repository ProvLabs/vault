package keeper

import (
	"context"
	"fmt"

	"github.com/provlabs/vault/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
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
	vault, ok := k.tryGetVault(ctx, vaultAddr)
	if !ok {
		return nil, fmt.Errorf("failed to get vault: %v", msg.VaultAddress)
	}
	if vault.Admin != msg.Admin {
		return nil, fmt.Errorf("unauthorized: %s is not the interest admin", msg.Admin)
	}

	newRate := sdkmath.LegacyMustNewDecFromStr(msg.NewRate)
	isValidRate, err := vault.IsInterestRateInRange(newRate)
	if err != nil {
		return nil, fmt.Errorf("failed to validate interest rate: %w", err)
	}
	if !isValidRate {
		return nil, fmt.Errorf("interest rate %s is out of bounds for vault %s", newRate, vault.GetAddress())
	}

	if newRate.IsZero() {
		msg.NewRate = types.ZeroInterestRate
	}

	reconciled := false
	currRate := sdkmath.LegacyMustNewDecFromStr(vault.CurrentInterestRate)
	if !currRate.IsZero() {
		if err := k.ReconcileVaultInterest(ctx, vault); err != nil {
			return nil, fmt.Errorf("failed to reconcile before rate change: %w", err)
		}
		reconciled = true
	}

	k.UpdateInterestRates(ctx, vault, msg.NewRate, msg.NewRate)

	if !reconciled && vault.InterestEnabled() {
		err := k.VaultInterestDetails.Set(ctx, vault.GetAddress(), types.VaultInterestDetails{PeriodStart: ctx.BlockTime().Unix()})
		if err != nil {
			return nil, fmt.Errorf("failed to set vault interest details: %w", err)
		}
	} else if reconciled && !vault.InterestEnabled() {
		err := k.VaultInterestDetails.Remove(ctx, vault.GetAddress())
		if err != nil {
			return nil, fmt.Errorf("failed to remove vault interest details: %w", err)
		}
	}

	return &types.MsgUpdateInterestRateResponse{}, nil
}

func (k msgServer) DepositInterestFunds(goCtx context.Context, msg *types.MsgDepositInterestFundsRequest) (*types.MsgDepositInterestFundsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	adminAddr := sdk.MustAccAddressFromBech32(msg.Admin)
	vaultAddr := sdk.MustAccAddressFromBech32(msg.VaultAddress)

	vault, ok := k.tryGetVault(ctx, vaultAddr)
	if ok {
		return nil, fmt.Errorf("failed to get vault: %v", msg.VaultAddress)
	}
	if vault.Admin != msg.Admin {
		return nil, fmt.Errorf("unauthorized: %s is not the interest admin", msg.Admin)
	}

	if err := k.BankKeeper.SendCoins(ctx, adminAddr, vaultAddr, sdk.NewCoins(msg.Amount)); err != nil {
		return nil, fmt.Errorf("failed to deposit funds: %w", err)
	}

	err := k.ReconcileVaultInterest(ctx, vault)
	if err != nil {
		return nil, fmt.Errorf("failed to reconcile vault interest before withdrawal: %w", err)
	}

	k.emitEvent(ctx, types.NewEventInterestDeposit(msg.VaultAddress, msg.Admin, msg.Amount))

	return &types.MsgDepositInterestFundsResponse{}, nil
}

func (k msgServer) WithdrawInterestFunds(goCtx context.Context, msg *types.MsgWithdrawInterestFundsRequest) (*types.MsgWithdrawInterestFundsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	adminAddr := sdk.MustAccAddressFromBech32(msg.Admin)
	vaultAddr := sdk.MustAccAddressFromBech32(msg.VaultAddress)

	vault, ok := k.tryGetVault(ctx, vaultAddr)
	if ok {
		return nil, fmt.Errorf("failed to get vault: %v", msg.VaultAddress)
	}
	if vault.Admin != msg.Admin {
		return nil, fmt.Errorf("unauthorized: %s is not the interest admin", msg.Admin)
	}

	err := k.ReconcileVaultInterest(ctx, vault)
	if err != nil {
		return nil, fmt.Errorf("failed to reconcile vault interest before withdrawal: %w", err)
	}

	if err := k.BankKeeper.SendCoins(ctx, vaultAddr, adminAddr, sdk.NewCoins(msg.Amount)); err != nil {
		return nil, fmt.Errorf("failed to withdraw funds: %w", err)
	}

	k.emitEvent(ctx, types.NewEventInterestWithdrawal(msg.VaultAddress, msg.Admin, msg.Amount))

	return &types.MsgWithdrawInterestFundsResponse{}, nil
}

func (k msgServer) ToggleSwapIn(goCtx context.Context, msg *types.MsgToggleSwapInRequest) (*types.MsgToggleSwapInResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	vaultAddr := sdk.MustAccAddressFromBech32(msg.VaultAddress)

	vault, ok := k.tryGetVault(ctx, vaultAddr)
	if !ok {
		return nil, fmt.Errorf("vault not found: %s", msg.VaultAddress)
	}
	if vault.Admin != msg.Admin {
		return nil, fmt.Errorf("unauthorized: %s is not the vault admin", msg.Admin)
	}

	k.SetSwapInEnable(ctx, vault, msg.Enabled)

	return &types.MsgToggleSwapInResponse{}, nil
}

func (k msgServer) ToggleSwapOut(goCtx context.Context, msg *types.MsgToggleSwapOutRequest) (*types.MsgToggleSwapOutResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	vaultAddr := sdk.MustAccAddressFromBech32(msg.VaultAddress)

	vault, ok := k.tryGetVault(ctx, vaultAddr)
	if !ok {
		return nil, fmt.Errorf("vault not found: %s", msg.VaultAddress)
	}
	if vault.Admin != msg.Admin {
		return nil, fmt.Errorf("unauthorized: %s is not the vault admin", msg.Admin)
	}

	k.SetSwapOutEnable(ctx, vault, msg.Enabled)

	return &types.MsgToggleSwapOutResponse{}, nil
}
