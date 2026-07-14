package keeper_test

import (
	"cosmossdk.io/collections"
	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/types"
)

func (s *TestSuite) TestKeeper_MigrateFlattenMixedDenomVaults() {
	underlying := "ylds"
	payment := "usdc"

	runMigration := func() {
		s.Require().NoError(keeper.NewMigrator(s.simApp.VaultKeeper).Migrate1to2(s.ctx), "flatten migration should succeed")
	}

	getVault := func(addr sdk.AccAddress) *types.VaultAccount {
		acct := s.simApp.AccountKeeper.GetAccount(s.ctx, addr)
		s.Require().NotNil(acct, "vault account should exist after migration")
		vault, ok := acct.(*types.VaultAccount)
		s.Require().True(ok, "account at %s should be a VaultAccount", addr)
		return vault
	}

	s.Run("unpaused mixed vault with outstanding shares flattens in place", func() {
		s.SetupTest()
		legacy := s.createLegacyVaultAccount("vsharemixed", underlying, payment)
		legacy.TotalShares = sdk.NewCoin("vsharemixed", sdkmath.NewInt(999_748_457_017))
		legacy.OutstandingAumFee = sdk.Coin{Denom: "", Amount: sdkmath.ZeroInt()}
		s.simApp.AccountKeeper.SetAccount(s.ctx, legacy)

		nonVaultAddr := sdk.AccAddress([]byte("non-vault-account-addr____"))
		s.simApp.AccountKeeper.SetAccount(s.ctx, s.simApp.AccountKeeper.NewAccountWithAddress(s.ctx, nonVaultAddr))

		runMigration()

		got := getVault(legacy.GetAddress())
		s.Equal(underlying, got.PaymentDenom, "payment denom should collapse onto the underlying asset")
		s.Equal(sdk.NewCoin(underlying, sdkmath.ZeroInt()), got.OutstandingAumFee, "empty outstanding AUM fee should normalize to the zero coin of the underlying")
		s.Equal(sdkmath.NewInt(999_748_457_017), got.TotalShares.Amount, "total shares of record must be preserved by the flatten")
		s.False(got.Paused, "an unpaused vault should stay unpaused through the flatten")

		gotNonVault := s.simApp.AccountKeeper.GetAccount(s.ctx, nonVaultAddr)
		s.Require().NotNil(gotNonVault, "non-vault account should still exist after migration")
		_, isVault := gotNonVault.(*types.VaultAccount)
		s.False(isVault, "non-vault account must not be converted by the migration")

		runMigration()
		got = getVault(legacy.GetAddress())
		s.Equal(underlying, got.PaymentDenom, "second run should be idempotent: payment denom stays flattened")
	})

	s.Run("paused mixed vault keeps pause state and paused balance snapshot", func() {
		s.SetupTest()
		legacy := s.createLegacyVaultAccount("vsharepaused", underlying, payment)
		legacy.Paused = true
		legacy.PausedReason = "withdraw interest funds"
		legacy.PausedBalance = sdk.NewCoin(underlying, sdkmath.NewInt(69_526_395))
		s.simApp.AccountKeeper.SetAccount(s.ctx, legacy)

		runMigration()

		got := getVault(legacy.GetAddress())
		s.Equal(underlying, got.PaymentDenom, "payment denom should collapse onto the underlying asset")
		s.True(got.Paused, "pause state must survive the flatten")
		s.Equal("withdraw interest funds", got.PausedReason, "pause reason must survive the flatten")
		s.Equal(sdkmath.NewInt(69_526_395), got.PausedBalance.Amount, "paused balance snapshot must survive the flatten")
	})

	s.Run("nav authority defaults to admin when unset", func() {
		s.SetupTest()
		legacy := s.createLegacyVaultAccount("vsharenavauth", underlying, payment)
		legacy.NavAuthority = ""
		s.simApp.AccountKeeper.SetAccount(s.ctx, legacy)

		runMigration()

		got := getVault(legacy.GetAddress())
		s.Equal(got.Admin, got.NavAuthority, "unset nav authority should default to the vault admin")
	})

	s.Run("already-single-denom vault is untouched", func() {
		s.SetupTest()
		conforming := s.createLegacyVaultAccount("vsharesingle", underlying, underlying)
		conforming.OutstandingAumFee = sdk.NewCoin(underlying, sdkmath.ZeroInt())
		s.simApp.AccountKeeper.SetAccount(s.ctx, conforming)
		before := *getVault(conforming.GetAddress())

		runMigration()

		after := *getVault(conforming.GetAddress())
		s.Equal(before, after, "a vault already flattened must not be modified by the migration")
	})

	s.Run("non-zero outstanding AUM fee converts through the internal NAV table", func() {
		s.SetupTest()
		s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlying, 2_000_000), s.adminAddr)
		s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(payment, 2_000_000), s.adminAddr)
		legacy := s.createLegacyVaultAccount("vsharefeenav", underlying, payment)
		legacy.OutstandingAumFee = sdk.NewCoin(payment, sdkmath.NewInt(10))
		s.simApp.AccountKeeper.SetAccount(s.ctx, legacy)

		nav := types.NewVaultNAV(payment, sdk.NewInt64Coin(underlying, 2), sdkmath.OneInt(), "test")
		s.Require().NoError(s.simApp.VaultKeeper.SetVaultNAV(s.ctx, legacy, nav, legacy.Admin), "seeding a payment-denom NAV entry must succeed")

		runMigration()

		got := getVault(legacy.GetAddress())
		s.Equal(sdk.NewCoin(underlying, sdkmath.NewInt(20)), got.OutstandingAumFee, "10%s at 2%s each should re-denominate to 20%s", payment, underlying, underlying)
	})

	s.Run("non-zero outstanding AUM fee with no NAV price is zeroed rather than halting", func() {
		s.SetupTest()
		legacy := s.createLegacyVaultAccount("vsharefeezero", underlying, payment)
		legacy.OutstandingAumFee = sdk.NewCoin(payment, sdkmath.NewInt(10))
		s.simApp.AccountKeeper.SetAccount(s.ctx, legacy)

		runMigration()

		got := getVault(legacy.GetAddress())
		s.Equal(sdk.NewCoin(underlying, sdkmath.ZeroInt()), got.OutstandingAumFee, "an unpriceable fee should be zeroed in the underlying, never failing the upgrade")
	})

	s.Run("pending swap-outs in the old payment denom are rewritten to the underlying", func() {
		s.SetupTest()
		legacy := s.createLegacyVaultAccount("vshareswaps", underlying, payment)
		owner := sdk.AccAddress([]byte("swap-out-owner-addr_______"))
		shares := sdk.NewCoin("vshareswaps", sdkmath.NewInt(100))

		mixedReq := types.NewPendingSwapOut(owner, legacy.GetAddress(), shares, payment)
		mixedID, err := s.simApp.VaultKeeper.PendingSwapOutQueue.Enqueue(s.ctx, 1_000, &mixedReq)
		s.Require().NoError(err, "enqueueing a payment-denom swap-out fixture must succeed")

		conformingReq := types.NewPendingSwapOut(owner, legacy.GetAddress(), shares, underlying)
		conformingID, err := s.simApp.VaultKeeper.PendingSwapOutQueue.Enqueue(s.ctx, 2_000, &conformingReq)
		s.Require().NoError(err, "enqueueing an underlying-denom swap-out fixture must succeed")

		runMigration()

		gotTime, gotMixed, err := s.simApp.VaultKeeper.PendingSwapOutQueue.GetByID(s.ctx, mixedID)
		s.Require().NoError(err, "rewritten swap-out %d should still be readable by id", mixedID)
		s.Equal(underlying, gotMixed.RedeemDenom, "payment-denom swap-out should redeem in the underlying after migration")
		s.Equal(int64(1_000), gotTime, "rewritten swap-out must keep its payout timestamp")
		s.Equal(shares, gotMixed.Shares, "rewritten swap-out must keep its escrowed shares")

		_, gotConforming, err := s.simApp.VaultKeeper.PendingSwapOutQueue.GetByID(s.ctx, conformingID)
		s.Require().NoError(err, "conforming swap-out %d should still be readable by id", conformingID)
		s.Equal(underlying, gotConforming.RedeemDenom, "conforming swap-out should be left redeeming the underlying")
	})

	s.Run("pending swap-out with no vault account is skipped instead of failing the upgrade", func() {
		s.SetupTest()
		legacy := s.createLegacyVaultAccount("vshareorphan", underlying, payment)
		owner := sdk.AccAddress([]byte("swap-out-owner-addr_______"))
		orphanVaultAddr := types.GetVaultAddress("vsharemissing")

		orphanReq := types.NewPendingSwapOut(owner, orphanVaultAddr, sdk.NewCoin("vsharemissing", sdkmath.NewInt(50)), payment)
		orphanID, err := s.simApp.VaultKeeper.PendingSwapOutQueue.Enqueue(s.ctx, 1_000, &orphanReq)
		s.Require().NoError(err, "enqueueing a swap-out for a non-existent vault must succeed")

		mixedReq := types.NewPendingSwapOut(owner, legacy.GetAddress(), sdk.NewCoin("vshareorphan", sdkmath.NewInt(100)), payment)
		mixedID, err := s.simApp.VaultKeeper.PendingSwapOutQueue.Enqueue(s.ctx, 2_000, &mixedReq)
		s.Require().NoError(err, "enqueueing a payment-denom swap-out fixture must succeed")

		runMigration()

		_, gotOrphan, err := s.simApp.VaultKeeper.PendingSwapOutQueue.GetByID(s.ctx, orphanID)
		s.Require().NoError(err, "orphaned swap-out %d should still be readable by id", orphanID)
		s.Equal(payment, gotOrphan.RedeemDenom, "orphaned swap-out should be left untouched for queue processing to dequeue")

		_, gotMixed, err := s.simApp.VaultKeeper.PendingSwapOutQueue.GetByID(s.ctx, mixedID)
		s.Require().NoError(err, "rewritten swap-out %d should still be readable by id", mixedID)
		s.Equal(underlying, gotMixed.RedeemDenom, "the walk must continue past the orphan and still rewrite mixed-denom entries")
	})
}

func (s *TestSuite) TestVaultModule_RunMigrations() {
	underlying := "ylds"
	payment := "usdc"

	s.Run("vault pinned to v1 runs the registered v1->v2 flatten migration", func() {
		s.SetupTest()
		legacy := s.createLegacyVaultAccount("vsharemmv1", underlying, payment)

		fromVM := s.simApp.ModuleManager.GetVersionMap()
		fromVM[types.ModuleName] = 1

		newVM, err := s.simApp.ModuleManager.RunMigrations(s.ctx, s.simApp.Configurator(), fromVM)
		s.Require().NoError(err, "RunMigrations must succeed; vault v1->v2 handler must be registered for vault %s", legacy.Address)
		s.Require().Equal(uint64(2), newVM[types.ModuleName], "vault module version should advance to ConsensusVersion 2")

		acct := s.simApp.AccountKeeper.GetAccount(s.ctx, legacy.GetAddress())
		got, ok := acct.(*types.VaultAccount)
		s.Require().True(ok, "account at %s should remain a VaultAccount", legacy.Address)
		s.Equal(underlying, got.PaymentDenom, "the v1->v2 migration should flatten the mixed vault %s", legacy.Address)

		_, err = s.simApp.VaultKeeper.NAVs.Get(s.ctx, collections.Join(legacy.GetAddress(), payment))
		s.Require().ErrorIs(err, collections.ErrNotFound, "the flatten migration must not seed internal NAV entries")
	})

	s.Run("version map already at current ConsensusVersion is a no-op", func() {
		s.SetupTest()
		legacy := s.createLegacyVaultAccount("vsharemmnoop", underlying, payment)

		fromVM := s.simApp.ModuleManager.GetVersionMap()
		newVM, err := s.simApp.ModuleManager.RunMigrations(s.ctx, s.simApp.Configurator(), fromVM)
		s.Require().NoError(err, "RunMigrations should succeed when versions match")
		s.Require().Equal(fromVM[types.ModuleName], newVM[types.ModuleName], "vault module version should remain unchanged")

		acct := s.simApp.AccountKeeper.GetAccount(s.ctx, legacy.GetAddress())
		got, ok := acct.(*types.VaultAccount)
		s.Require().True(ok, "account at %s should remain a VaultAccount", legacy.Address)
		s.Equal(payment, got.PaymentDenom, "a matching version map must not trigger the flatten")
	})
}
