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

	vault, err := k.Keeper.CreateVault(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("failed to create vault: %w", err)
	}

	return &types.MsgCreateVaultResponse{
		VaultAddress: vault.Address,
	}, nil
}

// SwapIn handles depositing assets accepted by the vault and mints vault shares to the recipient.
func (k msgServer) SwapIn(goCtx context.Context, msg *types.MsgSwapInRequest) (*types.MsgSwapInResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	vaultAddr := sdk.MustAccAddressFromBech32(msg.VaultAddress)
	ownerAddr := sdk.MustAccAddressFromBech32(msg.Owner)

	sharesReceived, err := k.Keeper.SwapIn(ctx, vaultAddr, ownerAddr, msg.Assets)
	if err != nil {
		return nil, err
	}

	return &types.MsgSwapInResponse{SharesReceived: *sharesReceived}, nil
}

// SwapOut handles redeeming vault shares for assets accepted by the vault and transfers them to the recipient.
func (k msgServer) SwapOut(goCtx context.Context, msg *types.MsgSwapOutRequest) (*types.MsgSwapOutResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	vaultAddr := sdk.MustAccAddressFromBech32(msg.VaultAddress)
	ownerAddr := sdk.MustAccAddressFromBech32(msg.Owner)

	requestID, err := k.Keeper.SwapOut(ctx, vaultAddr, ownerAddr, msg.Assets, msg.RedeemDenom)
	if err != nil {
		return nil, err
	}

	return &types.MsgSwapOutResponse{RequestId: requestID}, nil
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
	if err := vault.ValidateManagementAuthority(msg.Authority); err != nil {
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
	if !curRate.IsZero() && !vault.Paused {
		if err := k.ReconcileVaultInterest(ctx, vault); err != nil {
			return nil, fmt.Errorf("failed to reconcile before rate change: %w", err)
		}
	}

	k.UpdateInterestRates(ctx, vault, msg.NewRate, msg.NewRate)

	nextEnabled := vault.InterestEnabled()

	switch {
	case !prevEnabled && nextEnabled:
		if err := k.SafeAddVerification(ctx, vault); err != nil {
			return nil, fmt.Errorf("failed to enqueue vault payout verification: %w", err)
		}
	case prevEnabled && !nextEnabled:
		vault.PeriodStart = 0
		vault.PeriodTimeout = 0
		if err := k.SetVaultAccount(ctx, vault); err != nil {
			return nil, fmt.Errorf("failed to set vault account: %w", err)
		}
		if err := k.PayoutVerificationSet.Remove(ctx, vault.GetAddress()); err != nil {
			return nil, fmt.Errorf("failed to remove payout verification entries: %w", err)
		}
		if err := k.PayoutTimeoutQueue.RemoveAllForVault(ctx, vault.GetAddress()); err != nil {
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

	authorityAddr := sdk.MustAccAddressFromBech32(msg.Authority)
	vaultAddr := sdk.MustAccAddressFromBech32(msg.VaultAddress)

	vault, err := k.GetVault(ctx, vaultAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get vault: %w", err)
	}
	if vault == nil {
		return nil, fmt.Errorf("vault not found: %s", msg.VaultAddress)
	}

	if err := vault.ValidateManagementAuthority(msg.Authority); err != nil {
		return nil, err
	}

	if vault.UnderlyingAsset != msg.Amount.Denom {
		return nil, fmt.Errorf("denom not supported for vault must be of type \"%s\" : got \"%s\"", vault.UnderlyingAsset, msg.Amount.Denom)
	}

	if err := k.BankKeeper.SendCoins(markertypes.WithBypass(ctx), authorityAddr, vaultAddr, sdk.NewCoins(msg.Amount)); err != nil {
		return nil, fmt.Errorf("failed to deposit funds: %w", err)
	}

	if err := k.ReconcileVaultInterest(ctx, vault); err != nil {
		return nil, fmt.Errorf("failed to reconcile vault interest after deposit: %w", err)
	}

	k.emitEvent(ctx, types.NewEventInterestDeposit(msg.VaultAddress, msg.Authority, msg.Amount))

	return &types.MsgDepositInterestFundsResponse{}, nil
}

// WithdrawInterestFunds handles withdrawing unused interest funds from the vault.
func (k msgServer) WithdrawInterestFunds(goCtx context.Context, msg *types.MsgWithdrawInterestFundsRequest) (*types.MsgWithdrawInterestFundsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	authorityAddr := sdk.MustAccAddressFromBech32(msg.Authority)
	vaultAddr := sdk.MustAccAddressFromBech32(msg.VaultAddress)

	vault, err := k.GetVault(ctx, vaultAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get vault: %w", err)
	}
	if vault == nil {
		return nil, fmt.Errorf("vault not found: %s", msg.VaultAddress)
	}

	if err := vault.ValidateManagementAuthority(msg.Authority); err != nil {
		return nil, err
	}
	if vault.UnderlyingAsset != msg.Amount.Denom {
		return nil, fmt.Errorf("denom not supported for vault must be of type \"%s\" : got \"%s\"", vault.UnderlyingAsset, msg.Amount.Denom)
	}

	if err := k.ReconcileVaultInterest(ctx, vault); err != nil {
		return nil, fmt.Errorf("failed to reconcile vault interest before withdrawal: %w", err)
	}

	// for receipt tokens, the transfer agent for the authority address is used
	if err := k.BankKeeper.SendCoins(markertypes.WithTransferAgents(ctx, authorityAddr), vaultAddr, authorityAddr, sdk.NewCoins(msg.Amount)); err != nil {
		return nil, fmt.Errorf("failed to withdraw funds: %w", err)
	}

	k.emitEvent(ctx, types.NewEventInterestWithdrawal(msg.VaultAddress, msg.Authority, msg.Amount))

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
	if err := vault.ValidateManagementAuthority(msg.Authority); err != nil {
		return nil, err
	}

	if !vault.Paused {
		return nil, fmt.Errorf("vault must be paused to deposit principal funds")
	}

	if err := k.ReconcileVaultInterest(ctx, vault); err != nil {
		return nil, fmt.Errorf("failed to reconcile vault interest before principal change: %w", err)
	}

	depositFromAddress := sdk.MustAccAddressFromBech32(msg.Authority)
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

	k.emitEvent(ctx, types.NewEventDepositPrincipalFunds(msg.VaultAddress, msg.Authority, msg.Amount))
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
	if err := vault.ValidateManagementAuthority(msg.Authority); err != nil {
		return nil, err
	}

	if !vault.Paused {
		return nil, fmt.Errorf("vault must be paused to withdraw principal funds")
	}

	if err := k.ReconcileVaultInterest(ctx, vault); err != nil {
		return nil, fmt.Errorf("failed to reconcile vault interest before principal change: %w", err)
	}

	authorityAddress := sdk.MustAccAddressFromBech32(msg.Authority)
	principalAddress := vault.PrincipalMarkerAddress()

	if err := vault.ValidateAcceptedCoin(msg.Amount); err != nil {
		return nil, err
	}

	// for receipt tokens, the transfer agent for the authority address is used
	// transfer agent of vault address is required for all token types since principal is marker account
	if err := k.BankKeeper.SendCoins(markertypes.WithTransferAgents(ctx, authorityAddress, vaultAddr),
		principalAddress,
		authorityAddress,
		sdk.NewCoins(msg.Amount),
	); err != nil {
		return nil, fmt.Errorf("failed to withdraw principal funds: %w", err)
	}

	k.emitEvent(ctx, types.NewEventWithdrawPrincipalFunds(msg.VaultAddress, msg.Authority, msg.Amount))

	return &types.MsgWithdrawPrincipalFundsResponse{}, nil
}

// ExpeditePendingSwapOut expedites a pending swap out from a vault.
func (k msgServer) ExpeditePendingSwapOut(goCtx context.Context, msg *types.MsgExpeditePendingSwapOutRequest) (*types.MsgExpeditePendingSwapOutResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	_, swapOut, err := k.PendingSwapOutQueue.GetByID(ctx, msg.RequestId)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending swap out: %w", err)
	}

	vaultAddr := sdk.MustAccAddressFromBech32(swapOut.VaultAddress)
	vault, err := k.GetVault(ctx, vaultAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get vault: %w", err)
	}
	if vault == nil {
		return nil, fmt.Errorf("vault not found: %s", vaultAddr)
	}
	if err := vault.ValidateManagementAuthority(msg.Authority); err != nil {
		return nil, err
	}

	if err := k.PendingSwapOutQueue.ExpediteSwapOut(ctx, msg.RequestId); err != nil {
		return nil, fmt.Errorf("failed to expedite swap out: %w", err)
	}

	k.emitEvent(ctx, types.NewEventPendingSwapOutExpedited(msg.RequestId, swapOut.VaultAddress, msg.Authority))

	return &types.MsgExpeditePendingSwapOutResponse{}, nil
}

// PauseVault pauses a vault, disabling all user-facing operations.
func (k msgServer) PauseVault(goCtx context.Context, msg *types.MsgPauseVaultRequest) (*types.MsgPauseVaultResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	vaultAddr := sdk.MustAccAddressFromBech32(msg.VaultAddress)
	vault, err := k.GetVault(ctx, vaultAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get vault: %w", err)
	}
	if vault == nil {
		return nil, fmt.Errorf("vault not found: %s", msg.VaultAddress)
	}
	if err := vault.ValidateManagementAuthority(msg.Authority); err != nil {
		return nil, err
	}

	if vault.Paused {
		return nil, fmt.Errorf("vault %s is already paused", msg.VaultAddress)
	}
	if err := k.ReconcileVaultInterest(ctx, vault); err != nil {
		return nil, fmt.Errorf("failed to reconcile interest before pausing: %w", err)
	}

	tvv, err := k.GetTVVInUnderlyingAsset(ctx, *vault)
	if err != nil {
		return nil, fmt.Errorf("failed to get TVV before pausing: %w", err)
	}

	vault.PausedBalance = sdk.NewCoin(vault.UnderlyingAsset, tvv)
	vault.Paused = true
	vault.PausedReason = msg.Reason
	if err := k.SetVaultAccount(ctx, vault); err != nil {
		return nil, fmt.Errorf("failed to set vault account: %w", err)
	}

	k.emitEvent(ctx, types.NewEventVaultPaused(msg.VaultAddress, msg.Authority, msg.Reason, vault.PausedBalance))

	return &types.MsgPauseVaultResponse{}, nil
}

// UnpauseVault unpauses a vault, re-enabling all user-facing operations after a NAV recalculation.
func (k msgServer) UnpauseVault(goCtx context.Context, msg *types.MsgUnpauseVaultRequest) (*types.MsgUnpauseVaultResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	vaultAddr := sdk.MustAccAddressFromBech32(msg.VaultAddress)
	vault, err := k.GetVault(ctx, vaultAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get vault: %w", err)
	}
	if vault == nil {
		return nil, fmt.Errorf("vault not found: %s", msg.VaultAddress)
	}
	if err := vault.ValidateManagementAuthority(msg.Authority); err != nil {
		return nil, err
	}

	if !vault.Paused {
		return nil, fmt.Errorf("vault %s is not paused", msg.VaultAddress)
	}

	vault.PausedBalance = sdk.Coin{}
	vault.Paused = false
	vault.PausedReason = ""
	if err := k.SetVaultAccount(ctx, vault); err != nil {
		return nil, fmt.Errorf("failed to set vault account: %w", err)
	}

	tvv, err := k.GetTVVInUnderlyingAsset(ctx, *vault)
	if err != nil {
		return nil, fmt.Errorf("failed to get TVV before pausing: %w", err)
	}

	k.emitEvent(ctx, types.NewEventVaultUnpaused(msg.VaultAddress, msg.Authority, sdk.NewCoin(vault.UnderlyingAsset, tvv)))

	return &types.MsgUnpauseVaultResponse{}, nil
}

// SetBridgeAddress sets the single external bridge address allowed to mint or burn shares for a vault.
func (k msgServer) SetBridgeAddress(goCtx context.Context, msg *types.MsgSetBridgeAddressRequest) (*types.MsgSetBridgeAddressResponse, error) {
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

	vault.BridgeAddress = msg.BridgeAddress
	if err := k.SetVaultAccount(ctx, vault); err != nil {
		return nil, fmt.Errorf("failed to set vault account: %w", err)
	}

	k.emitEvent(ctx, types.NewEventBridgeAddressSet(vaultAddr.String(), msg.Admin, msg.BridgeAddress))

	return &types.MsgSetBridgeAddressResponse{}, nil
}

// ToggleBridge enables or disables the bridge functionality for a vault.
func (k msgServer) ToggleBridge(goCtx context.Context, msg *types.MsgToggleBridgeRequest) (*types.MsgToggleBridgeResponse, error) {
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

	vault.BridgeEnabled = msg.Enabled
	if err := k.SetVaultAccount(ctx, vault); err != nil {
		return nil, fmt.Errorf("failed to set vault account: %w", err)
	}

	k.emitEvent(ctx, types.NewEventBridgeToggled(msg.VaultAddress, msg.Admin, msg.Enabled))

	return &types.MsgToggleBridgeResponse{}, nil
}

// BridgeMintShares mints local share marker supply for a vault; the signer must match the configured bridge address
// and the mint amount must not exceed (total_shares - current marker supply).
func (k msgServer) BridgeMintShares(goCtx context.Context, msg *types.MsgBridgeMintSharesRequest) (*types.MsgBridgeMintSharesResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	vaultAddr := sdk.MustAccAddressFromBech32(msg.VaultAddress)
	bridgeAddr := sdk.MustAccAddressFromBech32(msg.Bridge)

	vault, err := k.GetVault(ctx, vaultAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get vault: %w", err)
	}
	if vault == nil {
		return nil, fmt.Errorf("vault not found: %s", msg.VaultAddress)
	}
	if !vault.BridgeEnabled {
		return nil, fmt.Errorf("bridge is disabled for vault %s", msg.VaultAddress)
	}
	if vault.BridgeAddress != bridgeAddr.String() {
		return nil, fmt.Errorf("unauthorized bridge: expected %s got %s", vault.BridgeAddress, bridgeAddr.String())
	}
	if msg.Shares.Denom != vault.TotalShares.Denom {
		return nil, fmt.Errorf("invalid shares denom: expected %s got %s", vault.TotalShares.Denom, msg.Shares.Denom)
	}
	if !msg.Shares.Amount.IsPositive() {
		return nil, fmt.Errorf("mint amount must be positive")
	}

	currentSupply := k.BankKeeper.GetSupply(ctx, vault.TotalShares.Denom)
	available := vault.TotalShares.Sub(currentSupply)
	if msg.Shares.Amount.GT(available.Amount) {
		return nil, fmt.Errorf("mint exceeds capacity: requested %s available %s", msg.Shares.Amount.String(), available.Amount.String())
	}

	if err := k.MarkerKeeper.MintCoin(ctx, vaultAddr, msg.Shares); err != nil {
		return nil, fmt.Errorf("failed to mint shares to bridge: %w", err)
	}

	if err := k.MarkerKeeper.WithdrawCoins(ctx, vaultAddr, bridgeAddr, msg.Shares.Denom, sdk.NewCoins(msg.Shares)); err != nil {
		return nil, fmt.Errorf("failed to transfer minted shares to bridge: %w", err)
	}

	k.emitEvent(ctx, types.NewEventBridgeMintShares(vaultAddr.String(), bridgeAddr.String(), msg.Shares))

	return &types.MsgBridgeMintSharesResponse{}, nil
}

// BridgeBurnShares burns local share marker supply for a vault; the signer must match the configured bridge address
// and the burn amount must not exceed the current marker supply. Shares are burned from the bridge account balance.
func (k msgServer) BridgeBurnShares(goCtx context.Context, msg *types.MsgBridgeBurnSharesRequest) (*types.MsgBridgeBurnSharesResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	vaultAddr := sdk.MustAccAddressFromBech32(msg.VaultAddress)
	bridgeAddr := sdk.MustAccAddressFromBech32(msg.Bridge)

	vault, err := k.GetVault(ctx, vaultAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get vault: %w", err)
	}
	if vault == nil {
		return nil, fmt.Errorf("vault not found: %s", msg.VaultAddress)
	}
	if !vault.BridgeEnabled {
		return nil, fmt.Errorf("bridge is disabled for vault %s", msg.VaultAddress)
	}
	if vault.BridgeAddress != bridgeAddr.String() {
		return nil, fmt.Errorf("unauthorized bridge: expected %s got %s", vault.BridgeAddress, bridgeAddr.String())
	}
	if msg.Shares.Denom != vault.TotalShares.Denom {
		return nil, fmt.Errorf("invalid shares denom: expected %s got %s", vault.TotalShares.Denom, msg.Shares.Denom)
	}
	if !msg.Shares.Amount.IsPositive() {
		return nil, fmt.Errorf("burn amount must be positive")
	}

	if err := k.BankKeeper.SendCoins(markertypes.WithTransferAgents(ctx, vaultAddr), bridgeAddr, vault.PrincipalMarkerAddress(), sdk.NewCoins(msg.Shares)); err != nil {
		return nil, fmt.Errorf("failed to transfer shares from bridge to vault: %w", err)
	}

	if err := k.MarkerKeeper.BurnCoin(ctx, vaultAddr, msg.Shares); err != nil {
		return nil, fmt.Errorf("failed to burn shares from marker: %w", err)
	}

	k.emitEvent(ctx, types.NewEventBridgeBurnShares(vaultAddr.String(), bridgeAddr.String(), msg.Shares))

	return &types.MsgBridgeBurnSharesResponse{}, nil
}

// SetAssetManager sets or clears the optional asset manager address for a vault.
// Only the vault admin may call this. Passing an empty AssetManager clears it.
func (k msgServer) SetAssetManager(goCtx context.Context, msg *types.MsgSetAssetManagerRequest) (*types.MsgSetAssetManagerResponse, error) {
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

	vault.AssetManager = msg.AssetManager

	if err := k.SetVaultAccount(ctx, vault); err != nil {
		return nil, fmt.Errorf("failed to set vault account: %w", err)
	}

	k.emitEvent(ctx, types.NewEventAssetManagerSet(vaultAddr.String(), msg.Admin, msg.AssetManager))

	return &types.MsgSetAssetManagerResponse{}, nil
}
