package keeper_test

import (
	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	"github.com/provlabs/vault/types"
)

func (s *TestSuite) TestKeeper_MigrateVaultAccountPaymentDenomDefaults() {
	createVault := func(shareDenom, underlying, payment string) (sdk.AccAddress, *types.VaultAccount) {
		addr := types.GetVaultAddress(shareDenom)

		vault := &types.VaultAccount{
			BaseAccount:            authtypes.NewBaseAccountWithAddress(addr),
			Admin:                  s.adminAddr.String(),
			TotalShares:            sdk.NewCoin(shareDenom, sdkmath.ZeroInt()),
			UnderlyingAsset:        underlying,
			PaymentDenom:           payment,
			CurrentInterestRate:    types.ZeroInterestRate,
			DesiredInterestRate:    types.ZeroInterestRate,
			SwapInEnabled:          true,
			SwapOutEnabled:         true,
			WithdrawalDelaySeconds: 0,
			Paused:                 false,
			PausedBalance:          sdk.Coin{},
			BridgeEnabled:          false,
			BridgeAddress:          "",
		}

		acct := s.simApp.AccountKeeper.NewAccount(s.ctx, vault)
		vaultAcct, ok := acct.(*types.VaultAccount)
		s.Require().True(ok, "new account should return a *types.VaultAccount for a VaultAccount prototype")

		s.simApp.AccountKeeper.SetAccount(s.ctx, vaultAcct)

		return addr, vaultAcct
	}

	vaultAddrA, _ := createVault("vaultsharesA", "uylds.fcc", "")
	vaultAddrB, _ := createVault("vaultsharesB", "uusdc", "custompay")

	nonVaultAddr := sdk.AccAddress([]byte("non-vault-account-addr____"))
	nonVault := s.simApp.AccountKeeper.NewAccountWithAddress(s.ctx, nonVaultAddr)
	s.simApp.AccountKeeper.SetAccount(s.ctx, nonVault)

	err := s.simApp.VaultKeeper.MigrateVaultAccountPaymentDenomDefaults(s.ctx)
	s.Require().NoError(err, "migration should not return an error")

	gotA := s.simApp.AccountKeeper.GetAccount(s.ctx, vaultAddrA)
	s.Require().NotNil(gotA, "vault A account should exist after migration")
	gotVaultA, ok := gotA.(*types.VaultAccount)
	s.Require().True(ok, "vault A account should be a VaultAccount type")
	s.Require().Equal("uylds.fcc", gotVaultA.PaymentDenom, "vault A PaymentDenom should default to UnderlyingAsset when empty")

	gotB := s.simApp.AccountKeeper.GetAccount(s.ctx, vaultAddrB)
	s.Require().NotNil(gotB, "vault B account should exist after migration")
	gotVaultB, ok := gotB.(*types.VaultAccount)
	s.Require().True(ok, "vault B account should be a VaultAccount type")
	s.Require().Equal("custompay", gotVaultB.PaymentDenom, "vault B PaymentDenom should remain unchanged when already set")

	gotNonVault := s.simApp.AccountKeeper.GetAccount(s.ctx, nonVaultAddr)
	s.Require().NotNil(gotNonVault, "non-vault account should still exist after migration")
	_, ok = gotNonVault.(*types.VaultAccount)
	s.Require().False(ok, "non-vault account should not be converted to a VaultAccount during migration")

	err = s.simApp.VaultKeeper.MigrateVaultAccountPaymentDenomDefaults(s.ctx)
	s.Require().NoError(err, "migration should be idempotent and not error when run a second time")

	gotA2 := s.simApp.AccountKeeper.GetAccount(s.ctx, vaultAddrA)
	s.Require().NotNil(gotA2, "vault A account should still exist after second migration run")
	gotVaultA2, ok := gotA2.(*types.VaultAccount)
	s.Require().True(ok, "vault A account should still be a VaultAccount type after second migration run")
	s.Require().Equal("uylds.fcc", gotVaultA2.PaymentDenom, "vault A PaymentDenom should remain set to UnderlyingAsset after second migration run")
}
