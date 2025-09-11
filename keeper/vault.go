package keeper

import (
	"context"
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
}

// CreateVault creates the vault based on the provided attributes.
func (k *Keeper) CreateVault(ctx sdk.Context, attributes VaultAttributer) (*types.VaultAccount, error) {
	underlying := attributes.GetUnderlyingAsset()
	payment := attributes.GetPaymentDenom()
	withdrawalDelay := attributes.GetWithdrawalDelaySeconds()

	underlyingAssetAddr, err := markertypes.MarkerAddress(underlying)
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying asset marker address: %w", err)
	}
	if found := k.MarkerKeeper.IsMarkerAccount(ctx, underlyingAssetAddr); !found {
		return nil, fmt.Errorf("underlying asset marker %q not found", underlying)
	}

	vault, err := k.createVaultAccount(ctx, attributes.GetAdmin(), attributes.GetShareDenom(), underlying, payment, withdrawalDelay)
	if err != nil {
		return nil, fmt.Errorf("failed to create vault account: %w", err)
	}

	if _, err := k.createVaultMarker(ctx, vault.GetAddress(), vault.ShareDenom, vault.UnderlyingAsset); err != nil {
		return nil, fmt.Errorf("failed to create vault marker: %w", err)
	}

	k.emitEvent(ctx, types.NewEventVaultCreated(vault))
	return vault, nil
}

// GetVault finds a vault by a given address.
//
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

// createVaultAccount creates and stores a new vault account.
func (k *Keeper) createVaultAccount(ctx sdk.Context, admin, shareDenom, underlyingAsset, paymentDenom string, withdrawalDelay uint64) (*types.VaultAccount, error) {
	vaultAddr := types.GetVaultAddress(shareDenom)

	vault := types.NewVaultAccount(
		authtypes.NewBaseAccountWithAddress(vaultAddr),
		admin,
		shareDenom,
		underlyingAsset,
		paymentDenom,
		withdrawalDelay,
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
		} else if vaultAcc.GetSequence() > 0 {
			return nil, fmt.Errorf("account at %s is not a vault account", vaultAddr.String())
		}
	}
	vaultAcc = k.AuthKeeper.NewAccount(ctx, vault).(types.VaultAccountI)
	k.AuthKeeper.SetAccount(ctx, vaultAcc)

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
//  3. Reconciles any accrued interest from the vault to the marker module (if due).
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
		return nil, err
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
		return nil, err
	}

	if err := k.ReconcileVaultInterest(ctx, vault); err != nil {
		return nil, fmt.Errorf("failed to reconcile vault interest: %w", err)
	}

	principalAddress := vault.PrincipalMarkerAddress()

	shares, err := k.ConvertDepositToSharesInUnderlyingAsset(ctx, *vault, asset)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate shares from assets: %w", err)
	}

	if err := shares.Validate(); err != nil {
		return nil, err
	}

	if err := k.MarkerKeeper.MintCoin(ctx, vault.GetAddress(), shares); err != nil {
		return nil, err
	}

	if err := k.MarkerKeeper.WithdrawCoins(ctx, vault.GetAddress(), recipient, shares.Denom, sdk.NewCoins(shares)); err != nil {
		return nil, err
	}

	if err := k.BankKeeper.SendCoins(markertypes.WithBypass(ctx), recipient, principalAddress, sdk.NewCoins(asset)); err != nil {
		return nil, err
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
		return 0, err
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

	if shares.Denom != vault.ShareDenom {
		return 0, fmt.Errorf("swap out denom must be share denom %v : %v", shares.Denom, vault.ShareDenom)
	}

	if redeemDenom == "" {
		redeemDenom = vault.UnderlyingAsset
	}

	if err := vault.ValidateAcceptedDenom(redeemDenom); err != nil {
		return 0, err
	}

	if err := k.ReconcileVaultInterest(ctx, vault); err != nil {
		return 0, fmt.Errorf("failed to reconcile vault interest: %w", err)
	}

	assets, err := k.ConvertSharesToRedeemCoin(ctx, *vault, shares.Amount, redeemDenom)
	if err != nil {
		return 0, fmt.Errorf("failed to calculate assets from shares: %w", err)
	}
	if assets.Amount.IsZero() && shares.Amount.IsPositive() {
		return 0, fmt.Errorf("redeem amount of %s is too small and results in zero assets", shares.String())
	}

	if err := k.checkPayoutRestrictions(ctx, vault, owner, assets); err != nil {
		return 0, err
	}

	if err := k.BankKeeper.SendCoins(ctx, owner, vault.GetAddress(), sdk.NewCoins(shares)); err != nil {
		return 0, fmt.Errorf("failed to escrow shares: %w", err)
	}

	payoutTime := ctx.BlockTime().Unix() + int64(vault.WithdrawalDelaySeconds)
	pendingReq := types.PendingSwapOut{
		Owner:        owner.String(),
		VaultAddress: vaultAddr.String(),
		Assets:       assets,
		Shares:       shares,
	}
	requestID, err := k.PendingSwapOutQueue.Enqueue(ctx, payoutTime, &pendingReq)
	if err != nil {
		return 0, fmt.Errorf("failed to enqueue pending swap out request: %w", err)
	}

	k.emitEvent(ctx, types.NewEventSwapOutRequested(vaultAddr.String(), owner.String(), assets, shares, requestID))
	return requestID, nil
}

// SetSwapInEnable updates the SwapInEnabled flag for a given vault. It updates the vault account in the state and
// emits an EventToggleSwapIn event.
func (k *Keeper) SetSwapInEnable(ctx context.Context, vault *types.VaultAccount, enabled bool) {
	vault.SwapInEnabled = enabled
	k.AuthKeeper.SetAccount(ctx, vault)
	k.emitEvent(sdk.UnwrapSDKContext(ctx), types.NewEventToggleSwapIn(vault.Address, vault.Admin, enabled))
}

// SetSwapOutEnable updates the SwapOutEnabled flag for a given vault. It updates the vault account in the state and
// emits an EventToggleSwapOut event.
func (k *Keeper) SetSwapOutEnable(ctx context.Context, vault *types.VaultAccount, enabled bool) {
	vault.SwapOutEnabled = enabled
	k.AuthKeeper.SetAccount(ctx, vault)
	k.emitEvent(sdk.UnwrapSDKContext(ctx), types.NewEventToggleSwapOut(vault.Address, vault.Admin, enabled))
}

// SetMinInterestRate sets the minimum interest rate for a vault.
// An empty string disables the minimum rate check.
func (k *Keeper) SetMinInterestRate(ctx sdk.Context, vault *types.VaultAccount, minRate string) error {
	if err := k.ValidateInterestRateLimits(minRate, vault.MaxInterestRate); err != nil {
		return err
	}
	if vault.MinInterestRate == minRate {
		return nil
	}
	vault.MinInterestRate = minRate
	if err := k.SetVaultAccount(ctx, vault); err != nil {
		return err
	}
	k.emitEvent(ctx, types.NewEventMinInterestRateUpdated(vault.Address, vault.Admin, minRate))
	return nil
}

// SetMaxInterestRate sets the maximum interest rate for a vault.
// An empty string disables the maximum rate check.
func (k *Keeper) SetMaxInterestRate(ctx sdk.Context, vault *types.VaultAccount, maxRate string) error {
	if err := k.ValidateInterestRateLimits(vault.MinInterestRate, maxRate); err != nil {
		return err
	}
	if vault.MaxInterestRate == maxRate {
		return nil
	}
	vault.MaxInterestRate = maxRate
	if err := k.SetVaultAccount(ctx, vault); err != nil {
		return err
	}
	k.emitEvent(ctx, types.NewEventMaxInterestRateUpdated(vault.Address, vault.Admin, maxRate))
	return nil
}

// ValidateInterestRateLimits checks that the provided minimum and maximum interest
// rates are valid decimal values and that the minimum rate is not greater than
// the maximum rate. Empty values are treated as unset and pass validation.
func (k Keeper) ValidateInterestRateLimits(minRateStr, maxRateStr string) error {
	if minRateStr == "" || maxRateStr == "" {
		return nil
	}

	minRate, err := sdkmath.LegacyNewDecFromStr(minRateStr)
	if err != nil {
		return fmt.Errorf("invalid min interest rate: %w", err)
	}
	maxRate, err := sdkmath.LegacyNewDecFromStr(maxRateStr)
	if err != nil {
		return fmt.Errorf("invalid max interest rate: %w", err)
	}

	if minRate.GT(maxRate) {
		return fmt.Errorf("minimum interest rate %s cannot be greater than maximum interest rate %s", minRate, maxRate)
	}

	return nil
}
