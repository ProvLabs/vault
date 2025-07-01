package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	markertypes "github.com/provenance-io/provenance/x/marker/types"
	"github.com/provlabs/vault/types"
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

	_, err = k.createVaultMarker(ctx, vault.GetAddress(), vault.ShareDenom, vault.UnderlyingAssets[0])
	if err != nil {
		return nil, fmt.Errorf("failed to create vault marker: %w", err)
	}

	k.emitEvent(ctx, types.NewEventVaultCreated(vault))

	return vault, nil
}

// GetVault finds a vault by a given address
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
	vault := types.NewVaultAccount(authtypes.NewBaseAccountWithAddress(vaultAddr), admin, shareDenom, []string{underlyingAsset})

	if err := vault.Validate(); err != nil {
		return nil, fmt.Errorf("failed to validate vault account: %w", err)
	}

	if err := k.SetVault(ctx, vault); err != nil {
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

func (k *Keeper) SwapIn(ctx sdk.Context, vaultAddr sdk.AccAddress, recipient sdk.AccAddress, asset sdk.Coin) (*sdk.Coin, error) {
	vault, err := k.GetVault(ctx, vaultAddr)
	if err != nil {
		return nil, err
	}

	if err := vault.ValidateUnderlyingAssets(asset); err != nil {
		return nil, err
	}

	shares := sdk.NewCoin(vault.ShareDenom, asset.Amount)
	if err := shares.Validate(); err != nil {
		return nil, err
	}

	if err := k.MarkerKeeper.MintCoin(ctx, vault.GetAddress(), shares); err != nil {
		return nil, err
	}

	if err := k.MarkerKeeper.WithdrawCoins(ctx, vault.GetAddress(), recipient, shares.Denom, sdk.NewCoins(shares)); err != nil {
		return nil, err
	}

	return &shares, nil
}
