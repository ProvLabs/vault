package keeper_test

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/types"
)

func (s *TestSuite) TestMinSwapValue() {
	underlying := "undercoin"
	sharedenom := "vshare"
	admin := s.adminAddr
	vaultAddr := types.GetVaultAddress(sharedenom)

	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlying, 1_000_000), admin)

	msgServer := keeper.NewMsgServer(s.simApp.VaultKeeper)

	// 1. Create Vault with MinSwap values
	createMsg := &types.MsgCreateVaultRequest{
		Admin:           admin.String(),
		ShareDenom:      sharedenom,
		UnderlyingAsset: underlying,
		MinSwapInValue:  "100",
		MinSwapOutValue: "50",
	}
	_, err := msgServer.CreateVault(s.ctx, createMsg)
	s.Require().NoError(err, "CreateVault should succeed")

	vault, err := s.k.GetVault(s.ctx, vaultAddr)
	s.Require().NoError(err)
	s.Equal("100", vault.MinSwapInValue)
	s.Equal("50", vault.MinSwapOutValue)

	// 2. Test SwapIn below Min
	user := s.CreateAndFundAccount(sdk.NewInt64Coin(underlying, 1000))
	swapInMsg := &types.MsgSwapInRequest{
		Owner:        user.String(),
		VaultAddress: vaultAddr.String(),
		Assets:       sdk.NewInt64Coin(underlying, 99),
	}
	_, err = msgServer.SwapIn(s.ctx, swapInMsg)
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "below the minimum required value")

	// 3. Test SwapIn at Min
	swapInMsg.Assets = sdk.NewInt64Coin(underlying, 100)
	_, err = msgServer.SwapIn(s.ctx, swapInMsg)
	s.Require().NoError(err, "SwapIn at min should succeed")

	// 4. Test SwapOut below Min
	// First need some shares. We got 100 * 1,000,000 shares from previous step.
	shares := s.simApp.BankKeeper.GetBalance(s.ctx, user, sharedenom)
	s.Require().Equal(int64(100_000_000), shares.Amount.Int64())

	// SwapOut 49 underlying equivalent.
	// Since it's 1:1, 49 underlying = 49 * 1,000,000 shares.
	swapOutMsg := &types.MsgSwapOutRequest{
		Owner:        user.String(),
		VaultAddress: vaultAddr.String(),
		Assets:       sdk.NewInt64Coin(sharedenom, 49_999_999),
	}
	_, err = msgServer.SwapOut(s.ctx, swapOutMsg)
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "below the minimum required value")

	// 5. Test SwapOut at Min
	swapOutMsg.Assets = sdk.NewInt64Coin(sharedenom, 50_000_000)
	_, err = msgServer.SwapOut(s.ctx, swapOutMsg)
	s.Require().NoError(err, "SwapOut at min should succeed")

	// 6. Update MinSwapInValue
	updateInMsg := &types.MsgUpdateMinSwapInValueRequest{
		Admin:           admin.String(),
		VaultAddress:    vaultAddr.String(),
		MinSwapInValue:  "200",
	}
	_, err = msgServer.UpdateMinSwapInValue(s.ctx, updateInMsg)
	s.Require().NoError(err)
	vault, _ = s.k.GetVault(s.ctx, vaultAddr)
	s.Equal("200", vault.MinSwapInValue)

	// 7. Update MinSwapOutValue
	updateOutMsg := &types.MsgUpdateMinSwapOutValueRequest{
		Admin:           admin.String(),
		VaultAddress:    vaultAddr.String(),
		MinSwapOutValue: "150",
	}
	_, err = msgServer.UpdateMinSwapOutValue(s.ctx, updateOutMsg)
	s.Require().NoError(err)
	vault, _ = s.k.GetVault(s.ctx, vaultAddr)
	s.Equal("150", vault.MinSwapOutValue)

	// 8. Test new SwapOut Min
	// 150 underlying = 150 * 1,000,000 shares.
	swapOutMsg.Assets = sdk.NewInt64Coin(sharedenom, 149_999_999)
	_, err = msgServer.SwapOut(s.ctx, swapOutMsg)
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "below the minimum required value")
}
