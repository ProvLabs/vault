package keeper_test

import (
	"cosmossdk.io/collections"
	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	"github.com/provlabs/vault/keeper"
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

// TestKeeper_MigrateInternalNAVSeedFromMarker exercises the migration that seeds
// the Internal NAV table from Marker NAVs for vaults whose payment_denom differs
// from underlying_asset. Each subtest sets up a fresh fleet so cross-test state
// from the suite-level SetupTest does not leak between cases.
func (s *TestSuite) TestKeeper_MigrateInternalNAVSeedFromMarker() {
	s.Run("forward marker NAV seeds internal NAV", func() {
		s.SetupTest()
		underlying := "ylds"
		payment := "usdc"
		share := "vshare1"

		vault := s.setupSinglePaymentDenomVault(underlying, share, payment, 3, 2)

		err := s.simApp.VaultKeeper.MigrateInternalNAVSeedFromMarker(s.ctx)
		s.Require().NoError(err, "migration should succeed when forward marker NAV exists")

		got, err := s.simApp.VaultKeeper.NAVs.Get(s.ctx, collections.Join(vault.GetAddress(), payment))
		s.Require().NoError(err, "internal NAV entry should be seeded for payment denom")
		s.Require().Equal(payment, got.Denom, "internal NAV denom should be payment denom")
		s.Require().Equal(underlying, got.Price.Denom, "internal NAV price denom should be underlying")
		s.Require().Equal(sdkmath.NewInt(3), got.Price.Amount, "internal NAV price amount should be forward marker price amount")
		s.Require().Equal(sdkmath.NewInt(2), got.Volume, "internal NAV volume should be forward marker volume")
		s.Require().Equal(keeper.MigrationNAVSeedSource, got.Source, "internal NAV source should mark this entry as a migration seed")
	})

	s.Run("reverse marker NAV seeds internal NAV when only reverse exists", func() {
		s.SetupTest()
		underlying := "ylds"
		payment := "usdc"
		share := "vshare2"

		s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlying, 2_000_000), s.adminAddr)
		s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(payment, 2_000_000), s.adminAddr)
		vault, err := s.k.CreateVault(s.ctx, vaultAttrs{
			admin:      s.adminAddr.String(),
			share:      share,
			underlying: underlying,
			payment:    payment,
		})
		s.Require().NoError(err, "vault creation should succeed")
		s.setReverseNAV(underlying, payment, 5, 7)

		err = s.simApp.VaultKeeper.MigrateInternalNAVSeedFromMarker(s.ctx)
		s.Require().NoError(err, "migration should succeed when only reverse marker NAV exists")

		got, err := s.simApp.VaultKeeper.NAVs.Get(s.ctx, collections.Join(vault.GetAddress(), payment))
		s.Require().NoError(err, "internal NAV entry should be seeded for payment denom")
		s.Require().Equal(payment, got.Denom, "internal NAV denom should be payment denom")
		s.Require().Equal(underlying, got.Price.Denom, "internal NAV price denom should be underlying")
		s.Require().Equal(sdkmath.NewInt(7), got.Price.Amount, "reverse marker volume should become internal NAV price amount")
		s.Require().Equal(sdkmath.NewInt(5), got.Volume, "reverse marker price amount should become internal NAV volume")
	})

	s.Run("newer reverse NAV wins when both directions exist", func() {
		s.SetupTest()
		underlying := "ylds"
		payment := "usdc"
		share := "vshare3"

		vault := s.setupSinglePaymentDenomVault(underlying, share, payment, 3, 2)
		s.bumpHeight()
		s.setReverseNAV(underlying, payment, 11, 5)

		err := s.simApp.VaultKeeper.MigrateInternalNAVSeedFromMarker(s.ctx)
		s.Require().NoError(err, "migration should succeed when both directions exist")

		got, err := s.simApp.VaultKeeper.NAVs.Get(s.ctx, collections.Join(vault.GetAddress(), payment))
		s.Require().NoError(err, "internal NAV entry should be seeded")
		s.Require().Equal(sdkmath.NewInt(5), got.Price.Amount, "newer reverse volume should become internal NAV price amount")
		s.Require().Equal(sdkmath.NewInt(11), got.Volume, "newer reverse price amount should become internal NAV volume")
	})

	s.Run("payment equals underlying skips seeding but defaults nav_authority", func() {
		s.SetupTest()
		underlying := "ylds"
		share := "vshare4"

		vault := s.setupBaseVault(underlying, share)
		vault.NavAuthority = ""
		s.simApp.AccountKeeper.SetAccount(s.ctx, vault)

		err := s.simApp.VaultKeeper.MigrateInternalNAVSeedFromMarker(s.ctx)
		s.Require().NoError(err, "migration should succeed when payment equals underlying")

		has, err := s.simApp.VaultKeeper.NAVs.Has(s.ctx, collections.Join(vault.GetAddress(), vault.PaymentDenom))
		s.Require().NoError(err, "internal NAV lookup should not error")
		s.Require().False(has, "no internal NAV entry should be created when payment == underlying")

		got, err := s.simApp.VaultKeeper.GetVault(s.ctx, vault.GetAddress())
		s.Require().NoError(err, "should fetch vault after migration")
		s.Require().Equal(vault.Admin, got.NavAuthority, "nav_authority should default to admin when empty")
	})

	s.Run("uylds.fcc payment seeds a 1:1 internal NAV without marker NAV", func() {
		s.SetupTest()
		underlying := "ylds"
		payment := "uylds.fcc"
		share := "vshare5"

		s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlying, 2_000_000), s.adminAddr)
		s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(payment, 2_000_000), s.adminAddr)
		vault, err := s.k.CreateVault(s.ctx, vaultAttrs{
			admin:      s.adminAddr.String(),
			share:      share,
			underlying: underlying,
			payment:    payment,
		})
		s.Require().NoError(err, "vault creation should succeed")

		err = s.simApp.VaultKeeper.MigrateInternalNAVSeedFromMarker(s.ctx)
		s.Require().NoError(err, "migration must not error on uylds.fcc peg vault even without marker NAV")

		got, err := s.simApp.VaultKeeper.NAVs.Get(s.ctx, collections.Join(vault.GetAddress(), payment))
		s.Require().NoError(err, "1:1 internal NAV entry should be seeded for uylds.fcc payment peg vault")
		s.Require().Equal(payment, got.Denom, "1:1 NAV denom should be payment denom")
		s.Require().Equal(underlying, got.Price.Denom, "1:1 NAV price denom should be underlying")
		s.Require().Equal(sdkmath.OneInt(), got.Price.Amount, "1:1 NAV price amount should be one")
		s.Require().Equal(sdkmath.OneInt(), got.Volume, "1:1 NAV volume should be one")
		s.Require().Equal(keeper.MigrationNAVSeedSource, got.Source, "1:1 NAV source should mark this entry as a migration seed")
	})

	s.Run("uylds.fcc underlying seeds a 1:1 internal NAV without marker NAV", func() {
		s.SetupTest()
		underlying := "uylds.fcc"
		payment := "usdc"
		share := "vshare6"

		s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlying, 2_000_000), s.adminAddr)
		s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(payment, 2_000_000), s.adminAddr)
		vault, err := s.k.CreateVault(s.ctx, vaultAttrs{
			admin:      s.adminAddr.String(),
			share:      share,
			underlying: underlying,
			payment:    payment,
		})
		s.Require().NoError(err, "vault creation should succeed")

		err = s.simApp.VaultKeeper.MigrateInternalNAVSeedFromMarker(s.ctx)
		s.Require().NoError(err, "migration must not error on uylds.fcc peg vault even without marker NAV")

		got, err := s.simApp.VaultKeeper.NAVs.Get(s.ctx, collections.Join(vault.GetAddress(), payment))
		s.Require().NoError(err, "1:1 internal NAV entry should be seeded for uylds.fcc underlying peg vault")
		s.Require().Equal(payment, got.Denom, "1:1 NAV denom should be payment denom")
		s.Require().Equal(underlying, got.Price.Denom, "1:1 NAV price denom should be underlying")
		s.Require().Equal(sdkmath.OneInt(), got.Price.Amount, "1:1 NAV price amount should be one")
		s.Require().Equal(sdkmath.OneInt(), got.Volume, "1:1 NAV volume should be one")
	})

	s.Run("missing marker NAV fails migration loudly with vault id", func() {
		s.SetupTest()
		underlying := "ylds"
		payment := "usdc"
		share := "vshare7"

		s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlying, 2_000_000), s.adminAddr)
		s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(payment, 2_000_000), s.adminAddr)
		vault, err := s.k.CreateVault(s.ctx, vaultAttrs{
			admin:      s.adminAddr.String(),
			share:      share,
			underlying: underlying,
			payment:    payment,
		})
		s.Require().NoError(err, "vault creation should succeed")

		err = s.simApp.VaultKeeper.MigrateInternalNAVSeedFromMarker(s.ctx)
		s.Require().Error(err, "migration must fail when no marker NAV exists for a non-peg vault")
		s.Require().Contains(err.Error(), vault.Address, "error message should name the unpriced vault")
		s.Require().Contains(err.Error(), "no marker NAV available", "error message should explain the cause")
	})

	s.Run("preserves pre-existing internal NAV entries", func() {
		s.SetupTest()
		underlying := "ylds"
		payment := "usdc"
		share := "vshare8"

		vault := s.setupSinglePaymentDenomVault(underlying, share, payment, 3, 2)

		existing := types.VaultNAV{
			Denom:  payment,
			Price:  sdk.NewInt64Coin(underlying, 99),
			Volume: sdkmath.NewInt(100),
			Source: "operator-supplied",
		}
		s.Require().NoError(s.simApp.VaultKeeper.SetVaultNAV(s.ctx, vault, existing, vault.Admin), "seeding existing NAV must succeed")

		err := s.simApp.VaultKeeper.MigrateInternalNAVSeedFromMarker(s.ctx)
		s.Require().NoError(err, "migration should succeed when pre-existing NAV entry exists")

		got, err := s.simApp.VaultKeeper.NAVs.Get(s.ctx, collections.Join(vault.GetAddress(), payment))
		s.Require().NoError(err, "internal NAV entry should still be present after re-run")
		s.Require().Equal(sdkmath.NewInt(99), got.Price.Amount, "pre-existing internal NAV price must not be overwritten")
		s.Require().Equal(sdkmath.NewInt(100), got.Volume, "pre-existing internal NAV volume must not be overwritten")
		s.Require().Equal("operator-supplied", got.Source, "pre-existing internal NAV source must not be overwritten")
	})

	s.Run("migration is idempotent across consecutive runs", func() {
		s.SetupTest()
		underlying := "ylds"
		payment := "usdc"
		share := "vshare9"

		vault := s.setupSinglePaymentDenomVault(underlying, share, payment, 3, 2)

		err := s.simApp.VaultKeeper.MigrateInternalNAVSeedFromMarker(s.ctx)
		s.Require().NoError(err, "first migration run should succeed")
		first, err := s.simApp.VaultKeeper.NAVs.Get(s.ctx, collections.Join(vault.GetAddress(), payment))
		s.Require().NoError(err, "seeded NAV should be readable after first run")

		err = s.simApp.VaultKeeper.MigrateInternalNAVSeedFromMarker(s.ctx)
		s.Require().NoError(err, "second migration run should succeed")
		second, err := s.simApp.VaultKeeper.NAVs.Get(s.ctx, collections.Join(vault.GetAddress(), payment))
		s.Require().NoError(err, "seeded NAV should still be readable after second run")
		s.Require().Equal(first, second, "internal NAV should not change between consecutive migration runs")
	})

	s.Run("nav_authority is left intact when already set", func() {
		s.SetupTest()
		underlying := "ylds"
		share := "vshare10"

		vault := s.setupBaseVault(underlying, share)
		other := sdk.AccAddress([]byte("other-nav-authority_______")).String()
		vault.NavAuthority = other
		s.simApp.AccountKeeper.SetAccount(s.ctx, vault)

		err := s.simApp.VaultKeeper.MigrateInternalNAVSeedFromMarker(s.ctx)
		s.Require().NoError(err, "migration should succeed for identity-denom vault with nav_authority set")

		got, err := s.simApp.VaultKeeper.GetVault(s.ctx, vault.GetAddress())
		s.Require().NoError(err, "should fetch vault after migration")
		s.Require().Equal(other, got.NavAuthority, "non-empty nav_authority should not be overwritten")
	})

	s.Run("mixed fleet processes each vault under its own rule", func() {
		s.SetupTest()
		// identity-denom vault: needs only nav_authority default.
		identityUnderlying := "asset0"
		identityShare := "share0"
		identityVault := s.setupBaseVault(identityUnderlying, identityShare)
		identityVault.NavAuthority = ""
		s.simApp.AccountKeeper.SetAccount(s.ctx, identityVault)

		// uylds.fcc peg vault: must receive a 1:1 internal NAV entry.
		pegUnderlying := "asset1"
		pegPayment := "uylds.fcc"
		pegShare := "share1"
		s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(pegUnderlying, 2_000_000), s.adminAddr)
		s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(pegPayment, 2_000_000), s.adminAddr)
		pegVault, err := s.k.CreateVault(s.ctx, vaultAttrs{
			admin:      s.adminAddr.String(),
			share:      pegShare,
			underlying: pegUnderlying,
			payment:    pegPayment,
		})
		s.Require().NoError(err, "peg vault creation should succeed")

		// mismatched-denom vault with forward NAV: must be seeded.
		seededUnderlying := "asset2"
		seededPayment := "asset2pay"
		seededShare := "share2"
		seededVault := s.setupSinglePaymentDenomVault(seededUnderlying, seededShare, seededPayment, 3, 2)

		err = s.simApp.VaultKeeper.MigrateInternalNAVSeedFromMarker(s.ctx)
		s.Require().NoError(err, "mixed-fleet migration should succeed")

		gotIdentity, err := s.simApp.VaultKeeper.GetVault(s.ctx, identityVault.GetAddress())
		s.Require().NoError(err, "identity vault should be readable")
		s.Require().Equal(identityVault.Admin, gotIdentity.NavAuthority, "identity vault nav_authority should default to admin")
		hasIdentityNAV, err := s.simApp.VaultKeeper.NAVs.Has(s.ctx, collections.Join(identityVault.GetAddress(), identityVault.PaymentDenom))
		s.Require().NoError(err, "identity vault NAV lookup should not error")
		s.Require().False(hasIdentityNAV, "identity vault should not have an internal NAV entry")

		gotPegNAV, err := s.simApp.VaultKeeper.NAVs.Get(s.ctx, collections.Join(pegVault.GetAddress(), pegPayment))
		s.Require().NoError(err, "peg vault should have a 1:1 internal NAV entry")
		s.Require().Equal(sdkmath.OneInt(), gotPegNAV.Price.Amount, "peg vault NAV price amount should be one")
		s.Require().Equal(sdkmath.OneInt(), gotPegNAV.Volume, "peg vault NAV volume should be one")
		s.Require().Equal(pegUnderlying, gotPegNAV.Price.Denom, "peg vault NAV price denom should be underlying")

		gotSeeded, err := s.simApp.VaultKeeper.NAVs.Get(s.ctx, collections.Join(seededVault.GetAddress(), seededPayment))
		s.Require().NoError(err, "seeded vault NAV should be readable")
		s.Require().Equal(sdkmath.NewInt(3), gotSeeded.Price.Amount, "seeded vault price amount mismatch")
		s.Require().Equal(sdkmath.NewInt(2), gotSeeded.Volume, "seeded vault volume mismatch")
	})

	s.Run("migration emits EventNAVUpdated for each seeded entry", func() {
		s.SetupTest()
		underlying := "ylds"
		payment := "usdc"
		share := "vshare11"

		vault := s.setupSinglePaymentDenomVault(underlying, share, payment, 3, 2)

		em := sdk.NewEventManager()
		s.ctx = s.ctx.WithEventManager(em)
		err := s.simApp.VaultKeeper.MigrateInternalNAVSeedFromMarker(s.ctx)
		s.Require().NoError(err, "migration should succeed")

		var saw bool
		for _, ev := range normalizeEvents(em.Events()) {
			if ev.Type != "provlabs.vault.v1.EventNAVUpdated" {
				continue
			}
			if getAttribute(ev, "vault_address") == vault.Address &&
				getAttribute(ev, "denom") == payment {
				saw = true
				break
			}
		}
		s.Require().True(saw, "expected EventNAVUpdated for vault=%s denom=%s", vault.Address, payment)
	})

	s.Run("non-vault accounts are skipped", func() {
		s.SetupTest()
		other := sdk.AccAddress([]byte("plain-account_____________"))
		s.simApp.AccountKeeper.SetAccount(s.ctx, s.simApp.AccountKeeper.NewAccountWithAddress(s.ctx, other))

		// Mixed-fleet check that the migration walks past plain accounts.
		underlying := "ylds"
		payment := "usdc"
		share := "vshare12"
		vault := s.setupSinglePaymentDenomVault(underlying, share, payment, 3, 2)

		err := s.simApp.VaultKeeper.MigrateInternalNAVSeedFromMarker(s.ctx)
		s.Require().NoError(err, "non-vault accounts should not break migration")

		has, err := s.simApp.VaultKeeper.NAVs.Has(s.ctx, collections.Join(vault.GetAddress(), payment))
		s.Require().NoError(err, "internal NAV lookup should not error")
		s.Require().True(has, "vault should still be seeded when an unrelated account exists")

		gotOther := s.simApp.AccountKeeper.GetAccount(s.ctx, other)
		s.Require().NotNil(gotOther, "non-vault account should still exist after migration")
		_, ok := gotOther.(*types.VaultAccount)
		s.Require().False(ok, "non-vault account should not be converted by migration")
	})
}

// TestVaultModule_RunMigrations exercises the registered v1->v2 migration
// end-to-end through the SDK module manager, the same path an upstream upgrade
// handler uses via runModuleMigrations / app.mm.RunMigrations. The first
// subtest fails if module.go stops calling cfg.RegisterMigration for vault
// v1->v2: in that case RunMigrations returns "no migrations found for module
// vault" and the version map advance never happens.
func (s *TestSuite) TestVaultModule_RunMigrations() {
	s.Run("vault pinned to v1 runs the registered v1->v2 migration and seeds internal NAV", func() {
		s.SetupTest()
		underlying := "ylds"
		payment := "usdc"
		share := "vshare-mm-v1"
		vault := s.setupSinglePaymentDenomVault(underlying, share, payment, 3, 2)

		fromVM := s.simApp.ModuleManager.GetVersionMap()
		fromVM[types.ModuleName] = 1

		newVM, err := s.simApp.ModuleManager.RunMigrations(s.ctx, s.simApp.Configurator(), fromVM)
		s.Require().NoError(err, "RunMigrations must succeed; vault v1->v2 handler must be registered for vault %s", vault.Address)
		s.Require().Equal(uint64(2), newVM[types.ModuleName], "vault module version should advance to ConsensusVersion 2")

		got, err := s.simApp.VaultKeeper.NAVs.Get(s.ctx, collections.Join(vault.GetAddress(), payment))
		s.Require().NoError(err, "internal NAV should be seeded for vault %s denom %s by the v1->v2 migration", vault.Address, payment)
		s.Equal(payment, got.Denom, "seeded NAV denom should be payment denom %s", payment)
		s.Equal(underlying, got.Price.Denom, "seeded NAV price denom should be underlying %s", underlying)
		s.Equal(sdkmath.NewInt(3), got.Price.Amount, "seeded NAV price amount should match forward marker price for vault %s", vault.Address)
		s.Equal(sdkmath.NewInt(2), got.Volume, "seeded NAV volume should match forward marker volume for vault %s", vault.Address)
		s.Equal(keeper.MigrationNAVSeedSource, got.Source, "seeded NAV source should mark this entry as a migration seed for vault %s", vault.Address)
	})

	s.Run("version map already at current ConsensusVersion is a no-op and preserves operator-supplied NAV", func() {
		s.SetupTest()
		underlying := "ylds"
		payment := "usdc"
		share := "vshare-mm-noop"
		vault := s.setupSinglePaymentDenomVault(underlying, share, payment, 3, 2)

		existing := types.VaultNAV{
			Denom:  payment,
			Price:  sdk.NewInt64Coin(underlying, 99),
			Volume: sdkmath.NewInt(100),
			Source: "operator-supplied",
		}
		s.Require().NoError(s.simApp.VaultKeeper.SetVaultNAV(s.ctx, vault, existing, vault.Admin), "seeding operator-supplied NAV must succeed for vault %s", vault.Address)

		fromVM := s.simApp.ModuleManager.GetVersionMap()
		newVM, err := s.simApp.ModuleManager.RunMigrations(s.ctx, s.simApp.Configurator(), fromVM)
		s.Require().NoError(err, "RunMigrations should succeed when versions match for vault %s", vault.Address)
		s.Require().Equal(fromVM[types.ModuleName], newVM[types.ModuleName], "vault module version should remain unchanged")

		got, err := s.simApp.VaultKeeper.NAVs.Get(s.ctx, collections.Join(vault.GetAddress(), payment))
		s.Require().NoError(err, "operator-supplied NAV should still be readable for vault %s denom %s", vault.Address, payment)
		s.Equal(sdkmath.NewInt(99), got.Price.Amount, "operator-supplied NAV price must not be overwritten for vault %s", vault.Address)
		s.Equal(sdkmath.NewInt(100), got.Volume, "operator-supplied NAV volume must not be overwritten for vault %s", vault.Address)
		s.Equal("operator-supplied", got.Source, "operator-supplied NAV source must not be overwritten for vault %s", vault.Address)
	})

	s.Run("RunMigrations surfaces the inner migration error when a vault has no marker NAV", func() {
		s.SetupTest()
		underlying := "ylds"
		payment := "usdc"
		share := "vshare-mm-err"

		s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlying, 2_000_000), s.adminAddr)
		s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(payment, 2_000_000), s.adminAddr)
		vault, err := s.k.CreateVault(s.ctx, vaultAttrs{
			admin:      s.adminAddr.String(),
			share:      share,
			underlying: underlying,
			payment:    payment,
		})
		s.Require().NoError(err, "vault creation should succeed for share %s", share)

		fromVM := s.simApp.ModuleManager.GetVersionMap()
		fromVM[types.ModuleName] = 1

		_, err = s.simApp.ModuleManager.RunMigrations(s.ctx, s.simApp.Configurator(), fromVM)
		s.Require().Error(err, "RunMigrations should propagate the inner migration error for unpriceable vault %s", vault.Address)
		s.ErrorContains(err, vault.Address, "propagated error should name the unpriceable vault")
		s.ErrorContains(err, "no marker NAV available", "propagated error should explain the cause")
	})
}
