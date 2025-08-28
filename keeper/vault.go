package keeper

import (
	"context"
	"fmt"

	"github.com/provlabs/vault/types"
	"github.com/provlabs/vault/utils"

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
}

// CreateVault creates the vault based on the provided attributes.
func (k *Keeper) CreateVault(ctx sdk.Context, attributes VaultAttributer) (*types.VaultAccount, error) {
	underlyingAssetAddr, err := markertypes.MarkerAddress(attributes.GetUnderlyingAsset())
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying asset marker address: %w", err)
	}

	if found := k.MarkerKeeper.IsMarkerAccount(ctx, underlyingAssetAddr); !found {
		return nil, fmt.Errorf("underlying asset marker %q not found", attributes.GetUnderlyingAsset())
	}

	vault, err := k.createVaultAccount(ctx, attributes.GetAdmin(), attributes.GetShareDenom(), attributes.GetUnderlyingAsset())
	if err != nil {
		return nil, fmt.Errorf("failed to create vault account: %w", err)
	}

	_, err = k.createVaultMarker(ctx, vault.GetAddress(), vault.ShareDenom, vault.UnderlyingAsset)
	if err != nil {
		return nil, fmt.Errorf("failed to create vault marker: %w", err)
	}

	k.emitEvent(ctx, types.NewEventVaultCreated(vault))

	return vault, nil
}

// GetVault finds a vault by a given address
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
func (k *Keeper) createVaultAccount(ctx sdk.Context, admin, shareDenom, underlyingAsset string) (*types.VaultAccount, error) {
	vaultAddr := types.GetVaultAddress(shareDenom)

	vault := types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(vaultAddr), admin, shareDenom, underlyingAsset)

	if err := vault.Validate(); err != nil {
		return nil, fmt.Errorf("failed to validate vault account: %w", err)
	}

	if err := k.SetVaultLookup(ctx, vault); err != nil {
		return nil, fmt.Errorf("failed to store new vault: %w", err)
	}

	vaultAcc := k.AuthKeeper.GetAccount(ctx, vault.GetAddress())
	if vaultAcc != nil {
		_, ok := vaultAcc.(types.VaultAccountI)
		if ok {
			return nil, fmt.Errorf("vault address already exists for %s", vaultAddr.String())
		} else if vaultAcc.GetSequence() > 0 {
			// account exists, is not a vault, and has been signed for
			return nil, fmt.Errorf("account at %s is not a vault account", vaultAddr.String())
		}
	}
	vaultAcc = k.AuthKeeper.NewAccount(ctx, vault).(types.VaultAccountI)
	k.AuthKeeper.SetAccount(ctx, vaultAcc)

	return vault, nil
}

// createVaultMarker creates, finalizes, and activates a new restricted marker for the vault's share denomination.
// TODO: https://github.com/ProvLabs/vault/issues/2 discussion of marker configuration
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

// SwapIn handles the process of depositing underlying assets into a vault in exchange for newly minted vault shares.
// It performs the following steps:
//  1. Retrieves the vault configuration for the given vault address.
//  2. Reconciles any accrued interest from the vault to the marker module if due.
//  3. Validates that the provided underlying asset is supported by the vault.
//  4. Constructs the vault share amount based on the asset value.
//  5. Mints the equivalent amount of shares to the vault account.
//  6. Withdraws the minted shares from the vault to the recipient address.
//  7. Sends the underlying asset from the recipient to the vault’s marker account.
//  8. Emits a SwapIn event with metadata for indexing and audit.
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

	if !vault.SwapInEnabled {
		return nil, fmt.Errorf("swaps are not enabled for vault %s", vaultAddr.String())
	}

	if err := k.ReconcileVaultInterest(ctx, vault); err != nil {
		return nil, fmt.Errorf("failed to reconcile vault interest: %w", err)
	}

	markerAddr := markertypes.MustGetMarkerAddress(vault.ShareDenom)

	// if err := vault.ValidateUnderlyingAssets(asset); err != nil {
	// 	return nil, err
	// }

	totalShares := k.BankKeeper.GetSupply(ctx, vault.ShareDenom).Amount
	totalAssets := k.BankKeeper.GetBalance(ctx, markerAddr, vault.UnderlyingAsset).Amount

	shares, err := utils.CalculateSharesFromAssets(asset.Amount, totalAssets, totalShares, vault.ShareDenom)
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

	if err := k.BankKeeper.SendCoins(markertypes.WithBypass(ctx), recipient, markerAddr, sdk.NewCoins(asset)); err != nil {
		return nil, err
	}

	k.emitEvent(ctx, types.NewEventSwapIn(vaultAddr.String(), recipient.String(), asset, shares))

	return &shares, nil
}

// SwapOut handles the process of redeeming vault shares in exchange for underlying assets.
// It performs the following steps:
//  1. Retrieves the vault configuration for the given vault address.
//  2. Reconciles any accrued interest from the vault to the marker module if due.
//  3. Validates that the provided share denomination matches the vault's configured share denom.
//  4. Calculates the amount of underlying assets to return based on the share amount.
//  5. Transfers the shares from the owner to the vault's marker account.
//  6. Burns the received shares from the vault account.
//  7. Sends the equivalent amount of underlying assets from the marker to the owner.
//  8. Emits a SwapOut event with metadata for indexing and audit.
//
// Returns the burned share amount on success, or an error if any step fails.
func (k *Keeper) SwapOut(ctx sdk.Context, vaultAddr, owner sdk.AccAddress, shares sdk.Coin) (*sdk.Coin, error) {
	vault, err := k.GetVault(ctx, vaultAddr)
	if err != nil {
		return nil, err
	}
	if vault == nil {
		return nil, fmt.Errorf("vault with address %v not found", vaultAddr.String())
	}

	if !vault.SwapOutEnabled {
		return nil, fmt.Errorf("swaps are not enabled for vault %s", vaultAddr.String())
	}

	if shares.Denom != vault.ShareDenom {
		return nil, fmt.Errorf("swap out denom must be share denom %v : %v", shares.Denom, vault.ShareDenom)
	}

	if err := k.ReconcileVaultInterest(ctx, vault); err != nil {
		return nil, fmt.Errorf("failed to reconcile vault interest: %w", err)
	}

	markerAddr := markertypes.MustGetMarkerAddress(vault.ShareDenom)

	totalShares := k.BankKeeper.GetSupply(ctx, vault.ShareDenom).Amount
	totalAssets := k.BankKeeper.GetBalance(ctx, markerAddr, vault.UnderlyingAsset).Amount

	assets, err := utils.CalculateAssetsFromShares(shares.Amount, totalShares, totalAssets, vault.UnderlyingAsset)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate assets from shares: %w", err)
	}

	if err := assets.Validate(); err != nil {
		return nil, err
	}

	if err := k.BankKeeper.SendCoins(ctx, owner, markerAddr, sdk.NewCoins(shares)); err != nil {
		return nil, fmt.Errorf("failed to send shares to marker: %w", err)
	}

	if err := k.MarkerKeeper.BurnCoin(ctx, vault.GetAddress(), shares); err != nil {
		return nil, fmt.Errorf("failed to burn shares: %w", err)
	}

	if err := k.BankKeeper.SendCoins(markertypes.WithTransferAgents(ctx, vaultAddr), markerAddr, owner, sdk.NewCoins(assets)); err != nil {
		return nil, fmt.Errorf("failed to send underlying asset: %w", err)
	}

	k.emitEvent(ctx, types.NewEventSwapOut(vaultAddr.String(), owner.String(), assets, shares))

	return &shares, nil
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
