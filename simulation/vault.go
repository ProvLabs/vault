package simulation

import (
	"fmt"

	"github.com/provlabs/vault/interest"
	"github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/simapp"
	"github.com/provlabs/vault/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
)

// CreateVault creates a new vault with a marker and funds accounts.
func CreateVault(ctx sdk.Context, app *simapp.SimApp, underlying, share string, admin simtypes.Account, accs []simtypes.Account) error {
	if !MarkerExists(ctx, app, underlying) {
		if err := CreateGlobalMarker(ctx, app, sdk.NewInt64Coin(underlying, 1000), accs); err != nil {
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
	msgServer := keeper.NewMsgServer(app.VaultKeeper)
	_, err := msgServer.CreateVault(sdk.WrapSDKContext(ctx), newVault)
	return err
}

// SwapIn performs a swap in for a user.
func SwapIn(ctx sdk.Context, app *simapp.SimApp, user simtypes.Account, shareDenom string, amount sdk.Coin) error {
	vaultAddress := types.GetVaultAddress(shareDenom)
	swapIn := &types.MsgSwapInRequest{
		Owner:        user.Address.String(),
		VaultAddress: vaultAddress.String(),
		Assets:       amount,
	}
	msgServer := keeper.NewMsgServer(app.VaultKeeper)
	_, err := msgServer.SwapIn(ctx, swapIn)
	return err
}

// SwapOut performs a swap out for a user.
func SwapOut(ctx sdk.Context, app *simapp.SimApp, user simtypes.Account, shares sdk.Coin, redeemDenom string) error {
	vaultAddress := types.GetVaultAddress(shares.Denom)
	swapOut := &types.MsgSwapOutRequest{
		Owner:        user.Address.String(),
		VaultAddress: vaultAddress.String(),
		Assets:       shares,
		RedeemDenom:  redeemDenom,
	}
	msgServer := keeper.NewMsgServer(app.VaultKeeper)
	_, err := msgServer.SwapOut(sdk.WrapSDKContext(ctx), swapOut)
	return err
}

// PauseVault pauses a vault.
func PauseVault(ctx sdk.Context, app *simapp.SimApp, shareDenom string) error {
	vaultAddress := types.GetVaultAddress(shareDenom)
	vault, err := app.VaultKeeper.GetVault(ctx, vaultAddress)
	if err != nil {
		return err
	}
	msgServer := keeper.NewMsgServer(app.VaultKeeper)
	_, err = msgServer.PauseVault(ctx, &types.MsgPauseVaultRequest{Admin: vault.Admin, VaultAddress: vault.Address, Reason: "test"})
	return err
}

// DepositInterest deposits interest into a vault.
func DepositInterestFunds(ctx sdk.Context, app *simapp.SimApp, shareDenom string, amount sdk.Coin) (*types.MsgDepositInterestFundsResponse, error) {
	vaultAddress := types.GetVaultAddress(shareDenom)
	vault, err := app.VaultKeeper.GetVault(ctx, vaultAddress)
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
	msgServer := keeper.NewMsgServer(app.VaultKeeper)
	return msgServer.DepositInterestFunds(ctx, deposit)
}

// DepositPrincipal deposits principal into a vault.
func DepositPrincipalFunds(ctx sdk.Context, app *simapp.SimApp, shareDenom string, amount sdk.Coin) (*types.MsgDepositPrincipalFundsResponse, error) {
	vaultAddress := types.GetVaultAddress(shareDenom)
	vault, err := app.VaultKeeper.GetVault(ctx, vaultAddress)
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
	msgServer := keeper.NewMsgServer(app.VaultKeeper)
	return msgServer.DepositPrincipalFunds(ctx, deposit)
}
