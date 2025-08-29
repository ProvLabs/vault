package keeper

import (
	"context"
	"fmt"

	"github.com/provlabs/vault/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"

	markertypes "github.com/provenance-io/provenance/x/marker/types"
)

var _ types.MsgServer = &msgServer{}

type msgServer struct {
	*Keeper
}

// NewMsgServer creates a new MsgServer for the module.
func NewMsgServer(keeper *Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

// CreateVault creates a vault.
func (k msgServer) CreateVault(goCtx context.Context, msg *types.MsgCreateVaultRequest) (*types.MsgCreateVaultResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	_, err := k.Keeper.CreateVault(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("failed to create vault: %w", err)
	}

	return &types.MsgCreateVaultResponse{}, nil
}

// SwapIn handles depositing assets accepted by the vault and mints vault shares to the recipient.
func (k msgServer) SwapIn(goCtx context.Context, msg *types.MsgSwapInRequest) (*types.MsgSwapInResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	vaultAddr := sdk.MustAccAddressFromBech32(msg.VaultAddress)
	ownerAddr := sdk.MustAccAddressFromBech32(msg.Owner)

	_, err := k.Keeper.SwapIn(ctx, vaultAddr, ownerAddr, msg.Assets)
	if err != nil {
		return nil, err
	}

	return &types.MsgSwapInResponse{}, nil
}

// SwapOut handles redeeming vault shares for assets accepted by the vault and transfers them to the recipient.
func (k msgServer) SwapOut(goCtx context.Context, msg *types.MsgSwapOutRequest) (*types.MsgSwapOutResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	vaultAddr := sdk.MustAccAddressFromBech32(msg.VaultAddress)
	ownerAddr := sdk.MustAccAddressFromBech32(msg.Owner)

	_, err := k.Keeper.SwapOut(ctx, vaultAddr, ownerAddr, msg.Assets, msg.RedeemDenom)
	if err != nil {
		return nil, err
	}

	return &types.MsgSwapOutResponse{}, nil
}

// UpdateMinInterestRate sets the minimum allowable interest rate for a vault.
// Only the vault admin is authorized to perform this operation.
// An empty string disables the minimum interest rate limit.
func (k msgServer) UpdateMinInterestRate(goCtx context.Context, msg *types.MsgUpdateMinInterestRateRequest) (*types.MsgUpdateMinInterestRateResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	vaultAddr := sdk.MustAccAddressFromBech32(msg.VaultAddress)
	vault, err := k.GetVault(ctx, vaultAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get vault: %w", err)
	}
	if vault == nil {
		return nil, fmt.Errorf("vault not found: %s", msg.VaultAddress)
	}
	if err := vault.ValidateAdmin(msg.Admin); err != nil {
		return nil, err
	}

	if err := k.SetMinInterestRate(ctx, vault, msg.MinRate); err != nil {
		return nil, fmt.Errorf("failed to set min interest rate: %w", err)
	}

	return &types.MsgUpdateMinInterestRateResponse{}, nil
}

// UpdateMaxInterestRate sets the maximum allowable interest rate for a vault.
// Only the vault admin is authorized to perform this operation.
// An empty string disables the maximum interest rate limit.
func (k msgServer) UpdateMaxInterestRate(goCtx context.Context, msg *types.MsgUpdateMaxInterestRateRequest) (*types.MsgUpdateMaxInterestRateResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	vaultAddr := sdk.MustAccAddressFromBech32(msg.VaultAddress)
	vault, err := k.GetVault(ctx, vaultAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get vault: %w", err)
	}
	if vault == nil {
		return nil, fmt.Errorf("vault not found: %s", msg.VaultAddress)
	}
	if err := vault.ValidateAdmin(msg.Admin); err != nil {
		return nil, err
	}

	if err := k.SetMaxInterestRate(ctx, vault, msg.MaxRate); err != nil {
		return nil, fmt.Errorf("failed to set max interest rate: %w", err)
	}

	return &types.MsgUpdateMaxInterestRateResponse{}, nil
}

// UpdateInterestRate updates the interest rate for a vault.
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
	if err := vault.ValidateAdmin(msg.Admin); err != nil {
		return nil, err
	}

	newRate, err := sdkmath.LegacyNewDecFromStr(msg.NewRate)
	if err != nil {
		return nil, fmt.Errorf("invalid new rate: %w", err)
	}
	ok, err := vault.IsInterestRateInRange(newRate)
	if err != nil {
		return nil, fmt.Errorf("failed to validate interest range: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("interest rate %s is out of bounds for vault %s", newRate, vault.GetAddress())
	}
	if newRate.IsZero() {
		msg.NewRate = types.ZeroInterestRate
	}

	curRate, err := sdkmath.LegacyNewDecFromStr(vault.CurrentInterestRate)
	if err != nil {
		return nil, fmt.Errorf("invalid current rate: %w", err)
	}

	prevEnabled := vault.InterestEnabled()
	if !curRate.IsZero() {
		if err := k.ReconcileVaultInterest(ctx, vault); err != nil {
			return nil, fmt.Errorf("failed to reconcile before rate change: %w", err)
		}
	}

	k.UpdateInterestRates(ctx, vault, msg.NewRate, msg.NewRate)

	nextEnabled := vault.InterestEnabled()

	switch {
	case !prevEnabled && nextEnabled:
		if err := k.SafeEnqueueVerification(ctx, vault); err != nil {
			return nil, fmt.Errorf("failed to enqueue vault payout verification: %w", err)
		}
	case prevEnabled && !nextEnabled:
		vault.PeriodStart = 0
		vault.PeriodTimeout = 0
		if err := k.SetVaultAccount(ctx, vault); err != nil {
			return nil, fmt.Errorf("failed to set vault account: %w", err)
		}
		if err := k.DequeuePayoutVerification(ctx, vault.GetAddress()); err != nil {
			return nil, fmt.Errorf("failed to remove payout verification entries: %w", err)
		}
		if err := k.RemoveAllPayoutTimeoutsForVault(ctx, vault.GetAddress()); err != nil {
			return nil, fmt.Errorf("failed to remove payout timeout entries: %w", err)
		}
	}

	return &types.MsgUpdateInterestRateResponse{}, nil
}

// ToggleSwapIn enables or disables swap-in operations for a vault.
func (k msgServer) ToggleSwapIn(goCtx context.Context, msg *types.MsgToggleSwapInRequest) (*types.MsgToggleSwapInResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	vaultAddr := sdk.MustAccAddressFromBech32(msg.VaultAddress)

	vault, err := k.GetVault(ctx, vaultAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get vault: %w", err)
	}
	if vault == nil {
		return nil, fmt.Errorf("vault not found: %s", msg.VaultAddress)
	}
	if err := vault.ValidateAdmin(msg.Admin); err != nil {
		return nil, err
	}

	k.SetSwapInEnable(ctx, vault, msg.Enabled)

	return &types.MsgToggleSwapInResponse{}, nil
}

// ToggleSwapOut enables or disables swap-out operations for a vault.
func (k msgServer) ToggleSwapOut(goCtx context.Context, msg *types.MsgToggleSwapOutRequest) (*types.MsgToggleSwapOutResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	vaultAddr := sdk.MustAccAddressFromBech32(msg.VaultAddress)

	vault, err := k.GetVault(ctx, vaultAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get vault: %w", err)
	}
	if vault == nil {
		return nil, fmt.Errorf("vault not found: %s", msg.VaultAddress)
	}
	if err := vault.ValidateAdmin(msg.Admin); err != nil {
		return nil, err
	}

	k.SetSwapOutEnable(ctx, vault, msg.Enabled)

	return &types.MsgToggleSwapOutResponse{}, nil
}

// DepositInterestFunds handles depositing funds into the vault for paying interest.
func (k msgServer) DepositInterestFunds(goCtx context.Context, msg *types.MsgDepositInterestFundsRequest) (*types.MsgDepositInterestFundsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	adminAddr := sdk.MustAccAddressFromBech32(msg.Admin)
	vaultAddr := sdk.MustAccAddressFromBech32(msg.VaultAddress)

	vault, err := k.GetVault(ctx, vaultAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get vault: %w", err)
	}
	if vault == nil {
		return nil, fmt.Errorf("vault not found: %s", msg.VaultAddress)
	}

	if err := vault.ValidateAdmin(msg.Admin); err != nil {
		return nil, err
	}

	if vault.UnderlyingAsset != msg.Amount.Denom {
		return nil, fmt.Errorf("denom not supported for vault must be of type \"%s\" : got \"%s\"", vault.UnderlyingAsset, msg.Amount.Denom)
	}

	if err := k.BankKeeper.SendCoins(ctx, adminAddr, vaultAddr, sdk.NewCoins(msg.Amount)); err != nil {
		return nil, fmt.Errorf("failed to deposit funds: %w", err)
	}

	if err := k.ReconcileVaultInterest(ctx, vault); err != nil {
		return nil, fmt.Errorf("failed to reconcile vault interest before withdrawal: %w", err)
	}

	k.emitEvent(ctx, types.NewEventInterestDeposit(msg.VaultAddress, msg.Admin, msg.Amount))

	return &types.MsgDepositInterestFundsResponse{}, nil
}

// WithdrawInterestFunds handles withdrawing unused interest funds from the vault.
func (k msgServer) WithdrawInterestFunds(goCtx context.Context, msg *types.MsgWithdrawInterestFundsRequest) (*types.MsgWithdrawInterestFundsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	adminAddr := sdk.MustAccAddressFromBech32(msg.Admin)
	vaultAddr := sdk.MustAccAddressFromBech32(msg.VaultAddress)

	vault, err := k.GetVault(ctx, vaultAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get vault: %w", err)
	}
	if vault == nil {
		return nil, fmt.Errorf("vault not found: %s", msg.VaultAddress)
	}

	if err := vault.ValidateAdmin(msg.Admin); err != nil {
		return nil, err
	}

	if vault.UnderlyingAsset != msg.Amount.Denom {
		return nil, fmt.Errorf("denom not supported for vault must be of type \"%s\" : got \"%s\"", vault.UnderlyingAsset, msg.Amount.Denom)
	}

	if err := k.ReconcileVaultInterest(ctx, vault); err != nil {
		return nil, fmt.Errorf("failed to reconcile vault interest before withdrawal: %w", err)
	}

	if err := k.BankKeeper.SendCoins(ctx, vaultAddr, adminAddr, sdk.NewCoins(msg.Amount)); err != nil {
		return nil, fmt.Errorf("failed to withdraw funds: %w", err)
	}

	k.emitEvent(ctx, types.NewEventInterestWithdrawal(msg.VaultAddress, msg.Admin, msg.Amount))

	return &types.MsgWithdrawInterestFundsResponse{}, nil
}

// DepositPrincipalFunds allows an admin to deposit principal funds into a vault.
func (k msgServer) DepositPrincipalFunds(goCtx context.Context, msg *types.MsgDepositPrincipalFundsRequest) (*types.MsgDepositPrincipalFundsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	vaultAddr := sdk.MustAccAddressFromBech32(msg.VaultAddress)
	vault, err := k.GetVault(ctx, vaultAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get vault: %w", err)
	}
	if vault == nil {
		return nil, fmt.Errorf("vault not found: %s", msg.VaultAddress)
	}
	if err := vault.ValidateAdmin(msg.Admin); err != nil {
		return nil, err
	}

	if err := k.ReconcileVaultInterest(ctx, vault); err != nil {
		return nil, fmt.Errorf("failed to reconcile vault interest before principal change: %w", err)
	}

	depositFromAddress := sdk.MustAccAddressFromBech32(msg.Admin)
	principalAddress := vault.PrincipalMarkerAddress()

	if err := vault.ValidateAcceptedCoin(msg.Amount); err != nil {
		return nil, err
	}

	if err := k.BankKeeper.SendCoins(markertypes.WithBypass(ctx),
		depositFromAddress,
		principalAddress,
		sdk.NewCoins(msg.Amount),
	); err != nil {
		return nil, fmt.Errorf("failed to deposit principal funds: %w", err)
	}

	k.emitEvent(ctx, types.NewEventDepositPrincipalFunds(msg.VaultAddress, msg.Admin, msg.Amount))
	return &types.MsgDepositPrincipalFundsResponse{}, nil
}

// WithdrawPrincipalFunds allows an admin to withdraw principal funds from a vault.
func (k msgServer) WithdrawPrincipalFunds(goCtx context.Context, msg *types.MsgWithdrawPrincipalFundsRequest) (*types.MsgWithdrawPrincipalFundsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	vaultAddr := sdk.MustAccAddressFromBech32(msg.VaultAddress)
	vault, err := k.GetVault(ctx, vaultAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get vault: %w", err)
	}
	if vault == nil {
		return nil, fmt.Errorf("vault not found: %s", msg.VaultAddress)
	}
	if err := vault.ValidateAdmin(msg.Admin); err != nil {
		return nil, err
	}

	if err := k.ReconcileVaultInterest(ctx, vault); err != nil {
		return nil, fmt.Errorf("failed to reconcile vault interest before principal change: %w", err)
	}

	withdrawAddress := sdk.MustAccAddressFromBech32(msg.Admin)
	principalAddress := vault.PrincipalMarkerAddress()

	if err := vault.ValidateAcceptedCoin(msg.Amount); err != nil {
		return nil, err
	}

	if err := k.BankKeeper.SendCoins(markertypes.WithTransferAgents(ctx, vaultAddr),
		principalAddress,
		withdrawAddress,
		sdk.NewCoins(msg.Amount),
	); err != nil {
		return nil, fmt.Errorf("failed to withdraw principal funds: %w", err)
	}

	k.emitEvent(ctx, types.NewEventWithdrawPrincipalFunds(msg.VaultAddress, msg.Admin, msg.Amount))
	return &types.MsgWithdrawPrincipalFundsResponse{}, nil
}
