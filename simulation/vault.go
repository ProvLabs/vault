package simulation

import (
	"fmt"

	"github.com/provlabs/vault/interest"
	"github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"

	markerkeeper "github.com/provenance-io/provenance/x/marker/keeper"
)

// CreateVault creates a new vault with a marker and funds accounts.
func CreateVault(ctx sdk.Context, vk *keeper.Keeper, ak types.AccountKeeper, bk types.BankKeeper, mk markerkeeper.Keeper, underlying, share string, admin simtypes.Account, accs []simtypes.Account) error {
	if !MarkerExists(ctx, mk, underlying) {
		if err := CreateGlobalMarker(ctx, ak, bk, mk, sdk.NewInt64Coin(underlying, 1000), accs); err != nil {
			return fmt.Errorf("unable to create global marker: %w", err)
		}
	}

	// Create a vault that uses the marker as an underlying asset
	newVault := &types.MsgCreateVaultRequest{
		Admin:                  admin.Address.String(),
		ShareDenom:             share,
		UnderlyingAsset:        underlying,
		PaymentDenom:           "",
		WithdrawalDelaySeconds: interest.SecondsPerDay,
	}
	msgServer := keeper.NewMsgServer(vk)
	_, err := msgServer.CreateVault(sdk.WrapSDKContext(ctx), newVault)
	return err
}

// SwapIn performs a swap in for a user.
func SwapIn(ctx sdk.Context, vk *keeper.Keeper, user simtypes.Account, shareDenom string, amount sdk.Coin) error {
	vaultAddress := types.GetVaultAddress(shareDenom)
	swapIn := &types.MsgSwapInRequest{
		Owner:        user.Address.String(),
		VaultAddress: vaultAddress.String(),
		Assets:       amount,
	}
	msgServer := keeper.NewMsgServer(vk)
	_, err := msgServer.SwapIn(ctx, swapIn)
	return err
}

// SwapOut performs a swap out for a user.
func SwapOut(ctx sdk.Context, vk *keeper.Keeper, user simtypes.Account, shares sdk.Coin, redeemDenom string) error {
	vaultAddress := types.GetVaultAddress(shares.Denom)
	swapOut := &types.MsgSwapOutRequest{
		Owner:        user.Address.String(),
		VaultAddress: vaultAddress.String(),
		Assets:       shares,
		RedeemDenom:  redeemDenom,
	}
	msgServer := keeper.NewMsgServer(vk)
	_, err := msgServer.SwapOut(sdk.WrapSDKContext(ctx), swapOut)
	return err
}

// PauseVault pauses a vault.
func PauseVault(ctx sdk.Context, vk *keeper.Keeper, shareDenom string) error {
	vaultAddress := types.GetVaultAddress(shareDenom)
	vault, err := vk.GetVault(ctx, vaultAddress)
	if err != nil {
		return err
	}
	msgServer := keeper.NewMsgServer(vk)
	_, err = msgServer.PauseVault(ctx, &types.MsgPauseVaultRequest{Admin: vault.Admin, VaultAddress: vault.Address, Reason: "test"})
	return err
}

// DepositInterest deposits interest into a vault.
func DepositInterestFunds(ctx sdk.Context, vk *keeper.Keeper, shareDenom string, amount sdk.Coin) (*types.MsgDepositInterestFundsResponse, error) {
	vaultAddress := types.GetVaultAddress(shareDenom)
	vault, err := vk.GetVault(ctx, vaultAddress)
	if vault == nil {
		return nil, fmt.Errorf("vault not found")
	}
	if err != nil {
		return nil, err
	}
	deposit := &types.MsgDepositInterestFundsRequest{
		Admin:        vault.Admin,
		VaultAddress: vault.Address,
		Amount:       amount,
	}
	msgServer := keeper.NewMsgServer(vk)
	return msgServer.DepositInterestFunds(ctx, deposit)
}

// DepositPrincipal deposits principal into a vault.
func DepositPrincipalFunds(ctx sdk.Context, vk *keeper.Keeper, shareDenom string, amount sdk.Coin) (*types.MsgDepositPrincipalFundsResponse, error) {
	vaultAddress := types.GetVaultAddress(shareDenom)
	vault, err := vk.GetVault(ctx, vaultAddress)
	if vault == nil {
		return nil, fmt.Errorf("vault not found")
	}
	if err != nil {
		return nil, err
	}
	deposit := &types.MsgDepositPrincipalFundsRequest{
		Admin:        vault.Admin,
		VaultAddress: vaultAddress.String(),
		Amount:       amount,
	}
	msgServer := keeper.NewMsgServer(vk)
	return msgServer.DepositPrincipalFunds(ctx, deposit)
}

// SetVaultBridge sets the bridge address and enabled flag for a vault.
func SetVaultBridge(ctx sdk.Context, vk *keeper.Keeper, shareDenom string, bridgeAddr sdk.AccAddress, enabled bool) error {
	marker, err := vk.MarkerKeeper.GetMarkerByDenom(ctx, shareDenom)
	if err != nil {
		return err
	}
	vaultAddr := marker.GetAddress()

	vault, err := vk.GetVault(ctx, vaultAddr)
	if err != nil {
		return err
	}
	if vault == nil {
		return fmt.Errorf("vault with share denom %s not found", shareDenom)
	}

	vault.BridgeAddress = bridgeAddr.String()
	vault.BridgeEnabled = enabled

	vk.AuthKeeper.SetAccount(ctx, vault)
	return nil
}
