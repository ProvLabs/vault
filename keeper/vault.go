package keeper

import (
	"fmt"

	"github.com/provlabs/vault/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	markertypes "github.com/provenance-io/provenance/x/marker/types"
)

const (
	Supply          = 0
	NoFixedSupply   = false
	NoForceTransfer = false
	NoGovControl    = false
)

// VaultAttributer provides the attributes for creating a new vault.
type VaultAttributer interface {
	GetAdmin() string
	GetShareDenom() string
	GetUnderlyingAsset() string
	GetPaymentDenom() string
	GetWithdrawalDelaySeconds() uint64
	GetMinSwapInValue() string
	GetMinSwapOutValue() string
	GetMaxSwapInValue() string
	GetMaxSwapOutValue() string
}

// CreateVault creates a new vault and its corresponding share marker atomically.
//
// The process involves:
//  1. Creating and persisting a new VaultAccount and its lookup entries.
//  2. Initializing the fee timeout queue for the new vault.
//  3. Creating, finalizing, and activating a restricted marker for the vault's shares.
//  4. Performing a pre-flight check against the principal/payment path by calling
//     SendRestrictionFn with vault.PrincipalMarkerAddress() to ensure the fee
//     collection address is permissioned to receive the payment denomination.
//
// All steps are performed within a cache context. If any step fails, including the
// pre-flight permission check, all state changes are discarded to prevent the creation
// of inconsistent or "orphan" vaults.
func (k *Keeper) CreateVault(ctx sdk.Context, attributes VaultAttributer) (*types.VaultAccount, error) {
	underlying := attributes.GetUnderlyingAsset()
	payment := attributes.GetPaymentDenom()
	withdrawalDelay := attributes.GetWithdrawalDelaySeconds()
	minSwapIn := attributes.GetMinSwapInValue()
	minSwapOut := attributes.GetMinSwapOutValue()
	maxSwapIn := attributes.GetMaxSwapInValue()
	maxSwapOut := attributes.GetMaxSwapOutValue()

	underlyingAssetAddr, err := markertypes.MarkerAddress(underlying)
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying asset marker address: %w", err)
	}
	if found := k.MarkerKeeper.IsMarkerAccount(ctx, underlyingAssetAddr); !found {
		return nil, fmt.Errorf("underlying asset marker %q not found", underlying)
	}

	cacheCtx, write := ctx.CacheContext()

	vault, err := k.createVaultAccount(cacheCtx, attributes.GetAdmin(), attributes.GetShareDenom(), underlying, payment, withdrawalDelay, minSwapIn, minSwapOut, maxSwapIn, maxSwapOut)
	if err != nil {
		return nil, fmt.Errorf("failed to create vault account: %w", err)
	}

	if _, err := k.createVaultMarker(cacheCtx, vault.GetAddress(), vault.TotalShares.Denom, vault.UnderlyingAsset); err != nil {
		return nil, fmt.Errorf("failed to create vault marker: %w", err)
	}

	provlabsAddr, err := k.GetAUMFeeAddress(cacheCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to get AUM fee address: %w", err)
	}

	if recipient, err := k.MarkerKeeper.SendRestrictionFn(
		markertypes.WithTransferAgents(cacheCtx, vault.GetAddress()),
		vault.PrincipalMarkerAddress(),
		provlabsAddr,
		sdk.NewCoins(sdk.NewInt64Coin(vault.PaymentDenom, 1)),
	); err != nil {
		return nil, fmt.Errorf("fee account %s is not permissioned to receive payment denom %s: %w", provlabsAddr.String(), vault.PaymentDenom, err)
	} else if !recipient.Equals(provlabsAddr) {
		return nil, fmt.Errorf("effective recipient %s differs from expected fee collector %s for payment denom %s", recipient.String(), provlabsAddr.String(), vault.PaymentDenom)
	}

	write()
	k.emitEvent(ctx, types.NewEventVaultCreated(vault))
	return vault, nil
}

// GetVault returns the vault account for the given address.
// This function will return nil if nothing exists at this address.
func (k Keeper) GetVault(ctx sdk.Context, address sdk.AccAddress) (*types.VaultAccount, error) {
	mac := k.AuthKeeper.GetAccount(ctx, address)
	if mac != nil {
		macc, ok := mac.(*types.VaultAccount)
		if !ok {
			return nil, fmt.Errorf("account at %s is not a vault account", address.String())
		}
		return macc, nil
	}
	return nil, nil
}

// getVault returns the vault at the given address, or an error if it does not exist.
func (k Keeper) getVault(ctx sdk.Context, addr sdk.AccAddress) (*types.VaultAccount, error) {
	vault, err := k.GetVault(ctx, addr)
	if err != nil {
		return nil, fmt.Errorf("failed to get vault: %w", err)
	}
	if vault == nil {
		return nil, fmt.Errorf("vault not found: %s", addr.String())
	}
	return vault, nil
}

// createVaultAccount creates and stores a new vault account and initializes its fee tracking.
// It verifies that the vault address is available and not already associated with another account.
func (k *Keeper) createVaultAccount(ctx sdk.Context, admin, shareDenom, underlyingAsset, paymentDenom string, withdrawalDelay uint64, minSwapIn, minSwapOut, maxSwapIn, maxSwapOut string) (*types.VaultAccount, error) {
	vaultAddr := types.GetVaultAddress(shareDenom)

	params, err := k.Params.Get(ctx)
	if err != nil {
		params = types.DefaultParams()
	}

	vault := types.NewVaultAccount(
		authtypes.NewBaseAccountWithAddress(vaultAddr),
		admin,
		shareDenom,
		underlyingAsset,
		paymentDenom,
		withdrawalDelay,
		params.DefaultAumFeeBips,
		minSwapIn,
		minSwapOut,
		maxSwapIn,
		maxSwapOut,
	)

	if err := vault.Validate(); err != nil {
		return nil, fmt.Errorf("failed to validate vault account: %w", err)
	}

	if err := k.SetVaultLookup(ctx, vault); err != nil {
		return nil, fmt.Errorf("failed to store new vault: %w", err)
	}

	vaultAcc := k.AuthKeeper.GetAccount(ctx, vault.GetAddress())
	if vaultAcc != nil {
		if _, ok := vaultAcc.(types.VaultAccountI); ok {
			return nil, fmt.Errorf("vault address already exists for %s", vaultAddr.String())
		}
		return nil, fmt.Errorf("account at %s already exists and is not a vault account", vaultAddr.String())
	}
	vaultAcc = k.AuthKeeper.NewAccount(ctx, vault).(types.VaultAccountI)
	k.AuthKeeper.SetAccount(ctx, vaultAcc)

	if err := k.SafeEnqueueFeeTimeout(ctx, vault); err != nil {
		return nil, fmt.Errorf("failed to enqueue initial fee timeout: %w", err)
	}

	return vault, nil
}

// createVaultMarker creates, finalizes, and activates a new restricted marker for the vault's share denomination.
func (k *Keeper) createVaultMarker(ctx sdk.Context, markerManager sdk.AccAddress, shareDenom, underlyingAsset string) (*markertypes.MarkerAccount, error) {
	vaultShareMarkerAddress, err := markertypes.MarkerAddress(shareDenom)
	if err != nil {
		return nil, fmt.Errorf("failed to get vault share marker address: %w", err)
	}

	if found := k.MarkerKeeper.IsMarkerAccount(ctx, vaultShareMarkerAddress); found {
		return nil, fmt.Errorf("a marker with the share denomination %q already exists", shareDenom)
	}

	baseAccount := authtypes.NewBaseAccountWithAddress(vaultShareMarkerAddress)
	newMarker := markertypes.NewMarkerAccount(
		baseAccount,
		sdk.NewInt64Coin(shareDenom, Supply),
		markerManager,
		[]markertypes.AccessGrant{
			{
				Address: markerManager.String(),
				Permissions: []markertypes.Access{
					markertypes.Access_Mint,
					markertypes.Access_Burn,
					markertypes.Access_Withdraw,
				},
			},
		},
		markertypes.StatusProposed,
		markertypes.MarkerType_Coin,
		NoFixedSupply,
		NoGovControl,
		NoForceTransfer,
		[]string{},
	)

	if err := k.MarkerKeeper.AddFinalizeAndActivateMarker(ctx, newMarker); err != nil {
		return nil, fmt.Errorf("failed to create and activate vault share marker: %w", err)
	}

	return newMarker, nil
}

// SwapIn handles the process of depositing underlying assets into a vault in
// exchange for newly minted vault shares.
//
// It performs the following steps:
//  1. Retrieves the vault configuration for the given vault address.
//  2. Verifies that swap-in is enabled for the vault.
//  3. Reconciles the vault (interest and AUM fees) if due.
//  4. Resolves the vault share marker address.
//  5. Validates that the provided underlying asset matches the vault’s configured underlying denom.
//  6. Calculates the number of shares to mint based on the deposit, current supply, and vault balance.
//  7. Mints the computed amount of shares under the vault’s admin authority.
//  8. Withdraws the minted shares from the vault to the recipient address.
//  9. Sends the underlying asset from the recipient to the vault’s marker account.
//
// 10. Emits a SwapIn event with metadata for indexing and audit.
//
// Returns the minted share amount on success, or an error if any step fails.
func (k *Keeper) SwapIn(ctx sdk.Context, vaultAddr, recipient sdk.AccAddress, asset sdk.Coin) (*sdk.Coin, error) {
	vault, err := k.GetVault(ctx, vaultAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get vault: %w", err)
	}
	if vault == nil {
		return nil, fmt.Errorf("vault with address %v not found", vaultAddr.String())
	}

	if vault.Paused {
		return nil, fmt.Errorf("vault %s is paused", vaultAddr.String())
	}

	if !vault.SwapInEnabled {
		return nil, fmt.Errorf("swaps are not enabled for vault %s", vaultAddr.String())
	}

	if err := vault.ValidateAcceptedCoin(asset); err != nil {
		return nil, fmt.Errorf("failed to validate asset: %w", err)
	}

	accept, reason, err := k.AllowSwapInAmount(ctx, asset, *vault)
	if err != nil {
		return nil, fmt.Errorf("swap in amount not allowed: %w", err)
	}
	if !accept {
		return nil, fmt.Errorf("failed to swap in: swap in amount %s %s for vault %s", asset.String(), reason, vaultAddr.String())
	}

	if err := k.reconcileVault(ctx, vault); err != nil {
		return nil, fmt.Errorf("failed to reconcile vault: %w", err)
	}

	principalAddress := vault.PrincipalMarkerAddress()

	shares, err := k.ConvertDepositToSharesInUnderlyingAsset(ctx, *vault, asset)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate shares from assets: %w", err)
	}

	if err := shares.Validate(); err != nil {
		return nil, fmt.Errorf("failed to validate shares: %w", err)
	}

	if err := k.MarkerKeeper.MintCoin(ctx, vault.GetAddress(), shares); err != nil {
		return nil, fmt.Errorf("failed to mint shares: %w", err)
	}

	vault.TotalShares = vault.TotalShares.Add(shares)
	if err := k.SetVaultAccount(ctx, vault); err != nil {
		return nil, fmt.Errorf("failed to update vault account: %w", err)
	}

	if err := k.MarkerKeeper.WithdrawCoins(ctx, vault.GetAddress(), recipient, shares.Denom, sdk.NewCoins(shares)); err != nil {
		return nil, fmt.Errorf("failed to withdraw shares: %w", err)
	}

	if err := k.BankKeeper.SendCoins(markertypes.WithBypass(ctx), recipient, principalAddress, sdk.NewCoins(asset)); err != nil {
		return nil, fmt.Errorf("failed to send asset to principal: %w", err)
	}

	k.emitEvent(ctx, types.NewEventSwapIn(vaultAddr.String(), recipient.String(), asset, shares))
	return &shares, nil
}

// checkPayoutRestrictions performs a pre-flight check to ensure a user is permissioned to receive
// the assets from a vault's principal marker. This prevents queueing a withdrawal that is guaranteed
// to fail later due to marker transfer restrictions (e.g., required attributes).
func (k *Keeper) checkPayoutRestrictions(ctx sdk.Context, vault *types.VaultAccount, owner sdk.AccAddress, assets sdk.Coin) error {
	_, err := k.MarkerKeeper.SendRestrictionFn(
		markertypes.WithTransferAgents(ctx, vault.GetAddress()),
		vault.PrincipalMarkerAddress(),
		owner,
		sdk.NewCoins(assets),
	)
	if err != nil {
		return fmt.Errorf("failed to pass send restrictions test: %w", err)
	}
	return nil
}

// SwapOut validates a swap-out request, calculates the resulting assets, escrows the user's shares,
// and enqueues a pending withdrawal request to be processed by the EndBlocker.
// It returns the unique ID of the newly queued request.
func (k *Keeper) SwapOut(ctx sdk.Context, vaultAddr, owner sdk.AccAddress, shares sdk.Coin, redeemDenom string) (uint64, error) {
	vault, err := k.GetVault(ctx, vaultAddr)
	if err != nil {
		return 0, fmt.Errorf("failed to get vault: %w", err)
	}
	if vault == nil {
		return 0, fmt.Errorf("vault with address %v not found", vaultAddr.String())
	}

	if vault.Paused {
		return 0, fmt.Errorf("vault %s is paused", vaultAddr.String())
	}

	if !vault.SwapOutEnabled {
		return 0, fmt.Errorf("swaps are not enabled for vault %s", vaultAddr.String())
	}

	if shares.Denom != vault.TotalShares.Denom {
		return 0, fmt.Errorf("swap out denom must be share denom %v : %v", shares.Denom, vault.TotalShares.Denom)
	}

	if redeemDenom == "" {
		redeemDenom = vault.PaymentDenom
	}

	if err := vault.ValidateAcceptedDenom(redeemDenom); err != nil {
		return 0, fmt.Errorf("failed to validate redeem denom: %w", err)
	}

	assets, err := k.ConvertSharesToRedeemCoin(ctx, *vault, shares.Amount, redeemDenom)
	if err != nil {
		return 0, fmt.Errorf("failed to calculate assets from shares: %w", err)
	}

	accept, reason, err := k.AllowSwapOutAmount(ctx, assets, *vault)
	if err != nil {
		return 0, fmt.Errorf("swap out amount not allowed: %w", err)
	}
	if !accept {
		return 0, fmt.Errorf("failed to swap out: swap out amount %s %s for vault %s", assets.String(), reason, vaultAddr.String())
	}

	if err := k.checkPayoutRestrictions(ctx, vault, owner, assets); err != nil {
		return 0, fmt.Errorf("failed to check payout restrictions: %w", err)
	}

	if err := k.BankKeeper.SendCoins(ctx, owner, vault.GetAddress(), sdk.NewCoins(shares)); err != nil {
		return 0, fmt.Errorf("failed to escrow shares: %w", err)
	}

	payoutTime := ctx.BlockTime().Unix() + int64(vault.WithdrawalDelaySeconds)

	pendingReq := types.NewPendingSwapOut(owner, vaultAddr, shares, redeemDenom)
	requestID, err := k.PendingSwapOutQueue.Enqueue(ctx, payoutTime, &pendingReq)
	if err != nil {
		return 0, fmt.Errorf("failed to enqueue pending swap out request: %w", err)
	}

	k.emitEvent(ctx, types.NewEventSwapOutRequested(vaultAddr.String(), owner.String(), redeemDenom, shares, requestID))
	return requestID, nil
}

// SetSwapInEnable updates the SwapInEnabled flag for a given vault. It updates the vault account in the state and
// emits an EventToggleSwapIn event.
func (k *Keeper) SetSwapInEnable(ctx sdk.Context, vault *types.VaultAccount, enabled bool) error {
	vault.SwapInEnabled = enabled
	if err := k.SetVaultAccount(ctx, vault); err != nil {
		return fmt.Errorf("failed to set vault account: %w", err)
	}
	k.emitEvent(ctx, types.NewEventToggleSwapIn(vault.Address, vault.Admin, enabled))
	return nil
}

// SetSwapOutEnable updates the SwapOutEnabled flag for a given vault. It updates the vault account in the state and
// emits an EventToggleSwapOut event.
func (k *Keeper) SetSwapOutEnable(ctx sdk.Context, vault *types.VaultAccount, enabled bool) error {
	vault.SwapOutEnabled = enabled
	if err := k.SetVaultAccount(ctx, vault); err != nil {
		return fmt.Errorf("failed to set vault account: %w", err)
	}
	k.emitEvent(ctx, types.NewEventToggleSwapOut(vault.Address, vault.Admin, enabled))
	return nil
}

// SetMinInterestRate sets the minimum interest rate for a vault.
// An empty string disables the minimum rate check.
func (k *Keeper) SetMinInterestRate(ctx sdk.Context, vault *types.VaultAccount, minRate string) error {
	if err := k.ValidateInterestRateLimits(minRate, vault.MaxInterestRate); err != nil {
		return fmt.Errorf("failed to validate interest rate limits: %w", err)
	}
	if vault.MinInterestRate == minRate {
		return nil
	}
	vault.MinInterestRate = minRate
	if err := k.SetVaultAccount(ctx, vault); err != nil {
		return fmt.Errorf("failed to set vault account: %w", err)
	}
	k.emitEvent(ctx, types.NewEventMinInterestRateUpdated(vault.Address, vault.Admin, minRate))
	return nil
}

// SetMaxInterestRate sets the maximum interest rate for a vault.
// An empty string disables the maximum rate check.
func (k *Keeper) SetMaxInterestRate(ctx sdk.Context, vault *types.VaultAccount, maxRate string) error {
	if err := k.ValidateInterestRateLimits(vault.MinInterestRate, maxRate); err != nil {
		return fmt.Errorf("failed to validate interest rate limits: %w", err)
	}
	if vault.MaxInterestRate == maxRate {
		return nil
	}
	vault.MaxInterestRate = maxRate
	if err := k.SetVaultAccount(ctx, vault); err != nil {
		return fmt.Errorf("failed to set vault account: %w", err)
	}
	k.emitEvent(ctx, types.NewEventMaxInterestRateUpdated(vault.Address, vault.Admin, maxRate))
	return nil
}

// ValidateInterestRateLimits checks that the provided minimum and maximum interest
// rates are valid decimal values and that the minimum rate is not greater than
// the maximum rate. Empty values are treated as unset and pass validation.
func (k Keeper) ValidateInterestRateLimits(minRateStr, maxRateStr string) error {
	var minRate, maxRate sdkmath.LegacyDec
	var err error
	var hasMin, hasMax bool

	if minRateStr != "" {
		minRate, err = sdkmath.LegacyNewDecFromStr(minRateStr)
		if err != nil {
			return fmt.Errorf("invalid min interest rate: %w", err)
		}
		hasMin = true
	}

	if maxRateStr != "" {
		maxRate, err = sdkmath.LegacyNewDecFromStr(maxRateStr)
		if err != nil {
			return fmt.Errorf("invalid max interest rate: %w", err)
		}
		hasMax = true
	}

	if hasMin && hasMax {
		if minRate.GT(maxRate) {
			return fmt.Errorf("minimum interest rate %s cannot be greater than maximum interest rate %s", minRate, maxRate)
		}
	}

	return nil
}

func (k *Keeper) SetWithdrawalDelay(ctx sdk.Context, vault *types.VaultAccount, delaySeconds uint64, authority string) error {
	if vault.WithdrawalDelaySeconds == delaySeconds {
		return nil
	}
	vault.WithdrawalDelaySeconds = delaySeconds
	if err := k.SetVaultAccount(ctx, vault); err != nil {
		return fmt.Errorf("failed to set vault account: %w", err)
	}
	k.emitEvent(ctx, types.NewEventWithdrawalDelayUpdated(vault.Address, authority, delaySeconds))
	return nil
}

// autoPauseVault sets a vault's state to paused, records the reason, persists it to state,
// and emits an EventVaultPaused. This function is intended to be called in response to a
// critical, unrecoverable error for a specific vault. The provided reason should be a stable,
// hard-coded string suitable for persistence and later auditing.
func (k *Keeper) autoPauseVault(ctx sdk.Context, vault *types.VaultAccount, reason string) {
	ctx.Logger().Error(
		"Auto-pausing vault due to critical error",
		"vault_address", vault.GetAddress().String(),
		"reason", reason,
	)

	tvv, err := k.GetTVVInUnderlyingAsset(ctx, *vault)
	if err != nil {
		ctx.Logger().Error("Failed to get TVV in underlying asset", "vault_address", vault.GetAddress().String(), "error", err)
	}

	vault.Paused = true
	vault.PausedReason = reason
	vault.PausedBalance = sdk.Coin{Denom: vault.UnderlyingAsset, Amount: tvv}
	k.AuthKeeper.SetAccount(ctx, vault) // Updating via SetAccount to skip validation since auto-pausing is triggered by invalid state

	k.emitEvent(ctx, types.NewEventVaultPaused(vault.GetAddress().String(), vault.GetAddress().String(), reason, vault.PausedBalance))
}

// UpdateVaultAUMFeeBips reconciles outstanding AUM fees for the provided VaultAccount
// before updating the stored fee rate (in basis points).
//
// This method ensures that all fees accrued under the old rate are accounted for
// before applying the new rate to future periods.
// It returns an error if the new bips value exceeds 10,000 (100%) or if reconciliation fails.
func (k *Keeper) UpdateVaultAUMFeeBips(ctx sdk.Context, vault *types.VaultAccount, bips uint32, authority string) error {
	if bips > 10_000 {
		return fmt.Errorf("invalid AUM fee bips: %d (max 10000)", bips)
	}

	if vault.AumFeeBips == bips {
		return nil
	}

	if err := k.reconcileVault(ctx, vault); err != nil {
		return fmt.Errorf("failed to reconcile before bips change: %w", err)
	}

	vault.AumFeeBips = bips
	if err := k.SetVaultAccount(ctx, vault); err != nil {
		return fmt.Errorf("failed to set vault account: %w", err)
	}

	k.emitEvent(ctx, types.NewEventVaultAUMFeeBipsUpdated(vault.Address, authority, bips))
	return nil
}

// SetMinSwapInValue updates the minimum swap-in value for a vault.
func (k *Keeper) SetMinSwapInValue(ctx sdk.Context, vault *types.VaultAccount, minSwapIn string, authority string) error {
	if err := types.ValidateSwapLimits(minSwapIn, vault.MaxSwapInValue); err != nil {
		return fmt.Errorf("failed to set MinSwapIn: %w", err)
	}
	if vault.MinSwapInValue == minSwapIn {
		return nil
	}
	vault.MinSwapInValue = minSwapIn
	if err := k.SetVaultAccount(ctx, vault); err != nil {
		return fmt.Errorf("failed to set vault account: %w", err)
	}
	k.emitEvent(ctx, types.NewEventMinSwapInValueUpdated(vault.Address, authority, minSwapIn))
	return nil
}

// SetMinSwapOutValue updates the minimum swap-out value for a vault.
func (k *Keeper) SetMinSwapOutValue(ctx sdk.Context, vault *types.VaultAccount, minSwapOut string, authority string) error {
	if err := types.ValidateSwapLimits(minSwapOut, vault.MaxSwapOutValue); err != nil {
		return fmt.Errorf("failed to set MinSwapOut: %w", err)
	}
	if vault.MinSwapOutValue == minSwapOut {
		return nil
	}
	vault.MinSwapOutValue = minSwapOut
	if err := k.SetVaultAccount(ctx, vault); err != nil {
		return fmt.Errorf("failed to set vault account: %w", err)
	}
	k.emitEvent(ctx, types.NewEventMinSwapOutValueUpdated(vault.Address, authority, minSwapOut))
	return nil
}

// SetMaxSwapInValue updates the maximum swap-in value for a vault.
func (k *Keeper) SetMaxSwapInValue(ctx sdk.Context, vault *types.VaultAccount, maxSwapIn string, authority string) error {
	if err := types.ValidateSwapLimits(vault.MinSwapInValue, maxSwapIn); err != nil {
		return fmt.Errorf("failed to set MaxSwapIn: %w", err)
	}
	if vault.MaxSwapInValue == maxSwapIn {
		return nil
	}
	vault.MaxSwapInValue = maxSwapIn
	if err := k.SetVaultAccount(ctx, vault); err != nil {
		return fmt.Errorf("failed to set vault account: %w", err)
	}
	k.emitEvent(ctx, types.NewEventMaxSwapInValueUpdated(vault.Address, authority, maxSwapIn))
	return nil
}

// SetMaxSwapOutValue updates the maximum swap-out value for a vault.
func (k *Keeper) SetMaxSwapOutValue(ctx sdk.Context, vault *types.VaultAccount, maxSwapOut string, authority string) error {
	if err := types.ValidateSwapLimits(vault.MinSwapOutValue, maxSwapOut); err != nil {
		return fmt.Errorf("failed to set MaxSwapOut: %w", err)
	}
	if vault.MaxSwapOutValue == maxSwapOut {
		return nil
	}
	vault.MaxSwapOutValue = maxSwapOut
	if err := k.SetVaultAccount(ctx, vault); err != nil {
		return fmt.Errorf("failed to set vault account: %w", err)
	}
	k.emitEvent(ctx, types.NewEventMaxSwapOutValueUpdated(vault.Address, authority, maxSwapOut))
	return nil
}

// AllowSwapInAmount checks whether a swap-in amount meets the minimum and maximum value requirements for a vault.
func (k *Keeper) AllowSwapInAmount(ctx sdk.Context, swapInAsset sdk.Coin, vault types.VaultAccount) (bool, string, error) {
	assetInUnderlying, err := k.ToUnderlyingAssetAmount(ctx, vault, swapInAsset)
	if err != nil {
		return false, "", fmt.Errorf("failed to convert swap in asset to underlying asset amount: %w", err)
	}

	return checkLimit(assetInUnderlying, vault.MinSwapInValue, vault.MaxSwapInValue)
}

// AllowSwapOutAmount checks whether a swap-out amount meets the minimum and maximum value requirements for a vault.
func (k *Keeper) AllowSwapOutAmount(ctx sdk.Context, assets sdk.Coin, vault types.VaultAccount) (bool, string, error) {
	assetInUnderlying, err := k.ToUnderlyingAssetAmount(ctx, vault, assets)
	if err != nil {
		return false, "", fmt.Errorf("failed to convert assets to underlying asset amount: %w", err)
	}

	return checkLimit(assetInUnderlying, vault.MinSwapOutValue, vault.MaxSwapOutValue)
}

// checkLimit compares an underlying asset amount against the configured min/max limits.
func checkLimit(amount sdkmath.Int, minStr, maxStr string) (bool, string, error) {
	if minStr != "" {
		minVal, ok := sdkmath.NewIntFromString(minStr)
		if !ok {
			return false, "", fmt.Errorf("failed to parse minimum limit: %s", minStr)
		}
		if !minVal.IsZero() && amount.LT(minVal) {
			return false, "is below the minimum required value", nil
		}
	}

	if maxStr != "" {
		maxVal, ok := sdkmath.NewIntFromString(maxStr)
		if !ok {
			return false, "", fmt.Errorf("failed to parse maximum limit: %s", maxStr)
		}
		if !maxVal.IsZero() && amount.GT(maxVal) {
			return false, "is above the maximum allowed value", nil
		}
	}

	return true, "", nil
}
