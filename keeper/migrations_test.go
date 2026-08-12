package keeper_test

import (
	"fmt"

	"cosmossdk.io/collections"
	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	markertypes "github.com/provenance-io/provenance/x/marker/types"

	vault "github.com/provlabs/vault"
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

func (s *TestSuite) TestKeeper_MigrateFlattenLegacyVaultFailingCurrentValidation() {
	underlying := "ylds"
	payment := "usdc"

	tests := []struct {
		name               string
		corrupt            func(*types.VaultAccount)
		expForcedErrSubstr string
		expValidAfterPause bool
	}{
		{
			name: "desired interest rate above the magnitude ceiling",
			corrupt: func(v *types.VaultAccount) {
				v.DesiredInterestRate = "200.0"
				v.CurrentInterestRate = "200.0"
			},
			expForcedErrSubstr: "exceeds maximum allowed magnitude",
			expValidAfterPause: false,
		},
		{
			name: "current interest rate diverging from desired",
			corrupt: func(v *types.VaultAccount) {
				v.CurrentInterestRate = "5.0"
				v.DesiredInterestRate = types.ZeroInterestRate
			},
			expForcedErrSubstr: "current interest rate must be zero or equal to desired",
			expValidAfterPause: true,
		},
		{
			name: "withdrawal delay beyond the two-year cap",
			corrupt: func(v *types.VaultAccount) {
				v.WithdrawalDelaySeconds = types.MaxWithdrawalDelay + 1
			},
			expForcedErrSubstr: "withdrawal delay cannot exceed",
			expValidAfterPause: false,
		},
		{
			name: "max swap-in value of zero",
			corrupt: func(v *types.VaultAccount) {
				v.MaxSwapInValue = "0"
			},
			expForcedErrSubstr: "max value cannot be zero",
			expValidAfterPause: false,
		},
		{
			name: "AUM fee bips above the ceiling",
			corrupt: func(v *types.VaultAccount) {
				v.AumFeeBips = 10_001
			},
			expForcedErrSubstr: "AUM fee bips cannot exceed",
			expValidAfterPause: false,
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			legacy := s.createLegacyVaultAccount("vshareinvalid", underlying, payment)
			legacy.NavAuthority = ""
			legacy.PeriodStart = 1_700_000_000
			legacy.PeriodTimeout = 1_700_072_000
			legacy.FeePeriodStart = 1_700_000_000
			legacy.FeePeriodTimeout = 1_700_072_000
			tc.corrupt(legacy)
			s.simApp.AccountKeeper.SetAccount(s.ctx, legacy)

			vaultAddr := legacy.GetAddress()
			s.Require().NoError(s.simApp.VaultKeeper.PayoutTimeoutQueue.Enqueue(s.ctx, legacy.PeriodTimeout, vaultAddr), "seeding a payout timeout for the invalid legacy vault must succeed")
			s.Require().NoError(s.simApp.VaultKeeper.FeeTimeoutQueue.Enqueue(s.ctx, legacy.FeePeriodTimeout, vaultAddr), "seeding a fee timeout for the invalid legacy vault must succeed")
			s.Require().NoError(s.simApp.VaultKeeper.PayoutVerificationSet.Set(s.ctx, vaultAddr), "seeding a payout verification entry for the invalid legacy vault must succeed")

			s.Require().NoError(keeper.NewMigrator(s.simApp.VaultKeeper).Migrate1to2(s.ctx), "a legacy vault failing current validation must never fail the upgrade")

			acct := s.simApp.AccountKeeper.GetAccount(s.ctx, vaultAddr)
			s.Require().NotNil(acct, "the invalid legacy vault must still be persisted after the migration")
			got, ok := acct.(*types.VaultAccount)
			s.Require().True(ok, "account at %s should remain a VaultAccount", vaultAddr)

			s.Equal(underlying, got.PaymentDenom, "the flatten must still be applied to a vault that fails validation")
			s.Equal(got.Admin, got.NavAuthority, "the nav authority default must still be applied to a vault that fails validation")
			s.True(got.Paused, "a vault persisted outside current validation must be paused so it cannot transact on questionable configuration")
			s.Equal(keeper.MigrationInvalidVaultPauseReason, got.PausedReason, "the pause reason must identify the migration as the source")
			s.Equal(types.ZeroInterestRate, got.CurrentInterestRate, "pausing must zero the current interest rate so the interest math never runs on legacy configuration")
			s.Equal(sdk.NewCoin(underlying, sdkmath.ZeroInt()), got.PausedBalance, "the migration must snapshot a zero paused balance rather than value an invalid vault")
			s.Zero(got.PeriodStart, "halting accrual must clear the interest period start")
			s.Zero(got.PeriodTimeout, "halting accrual must clear the interest period timeout")
			s.Zero(got.FeePeriodStart, "halting accrual must clear the fee period start")
			s.Zero(got.FeePeriodTimeout, "halting accrual must clear the fee period timeout")

			s.Equal(tc.expValidAfterPause, got.Validate() == nil, "persisted vault validity should match the expectation for case %q (validate: %v)", tc.name, got.Validate())

			s.Zero(s.countVaultAccrualEntries(vaultAddr), "halting accrual must clear the paused vault from every accrual queue")

			pausedEvent := s.findLastEventVaultPaused()
			s.Require().NotNil(pausedEvent, "the migration must emit EventVaultPaused for the vault it paused")
			s.Equal(vaultAddr.String(), pausedEvent.VaultAddress, "the paused event must name the vault the migration paused")
			s.True(pausedEvent.Forced, "a migration pause is a forced pause")
			s.Contains(pausedEvent.ForcedError, tc.expForcedErrSubstr, "the paused event must surface the tolerated validation error for case %q", tc.name)

			s.Require().NoError(keeper.NewMigrator(s.simApp.VaultKeeper).Migrate1to2(s.ctx), "re-running the migration over an already-paused vault must still succeed")
			after := s.simApp.AccountKeeper.GetAccount(s.ctx, vaultAddr).(*types.VaultAccount)
			s.Equal(*got, *after, "the migration must be idempotent over a vault it already paused")
		})
	}

	s.Run("an already-paused vault keeps its frozen valuation snapshot and original reason", func() {
		s.SetupTest()
		frozenBalance := sdk.NewCoin(underlying, sdkmath.NewInt(69_526_395))
		legacy := s.createLegacyVaultAccount("vsharepausedinvalid", underlying, payment)
		legacy.NavAuthority = ""
		legacy.Paused = true
		legacy.PausedReason = "withdraw interest funds"
		legacy.PausedBalance = frozenBalance
		legacy.DesiredInterestRate = "200.0"
		s.simApp.AccountKeeper.SetAccount(s.ctx, legacy)

		s.Require().NoError(keeper.NewMigrator(s.simApp.VaultKeeper).Migrate1to2(s.ctx), "an already-paused invalid vault must not fail the upgrade")

		got := s.simApp.AccountKeeper.GetAccount(s.ctx, legacy.GetAddress()).(*types.VaultAccount)
		s.True(got.Paused, "an already-paused vault must stay paused")
		s.Equal(frozenBalance, got.PausedBalance, "the frozen valuation GetTVV reports while paused must survive the migration")
		s.Equal("withdraw interest funds", got.PausedReason, "the original pause reason must not be overwritten by the migration reason")

		tvv, err := s.simApp.VaultKeeper.GetTVV(s.ctx, *got)
		s.Require().NoError(err, "valuing a paused vault should not error")
		s.Equal(frozenBalance.Amount, tvv, "the migrated vault must still report its frozen value rather than zero")
	})

	s.Run("a vault failing validation does not stop the rest of the migration", func() {
		s.SetupTest()
		broken := s.createLegacyVaultAccount("vsharebroken", underlying, payment)
		broken.DesiredInterestRate = "200.0"
		s.simApp.AccountKeeper.SetAccount(s.ctx, broken)

		healthy := s.createLegacyVaultAccount("vsharehealthy", underlying, payment)
		s.simApp.AccountKeeper.SetAccount(s.ctx, healthy)

		s.Require().NoError(keeper.NewMigrator(s.simApp.VaultKeeper).Migrate1to2(s.ctx), "one unmigratable vault must not fail the upgrade for every other vault")

		gotBroken := s.simApp.AccountKeeper.GetAccount(s.ctx, broken.GetAddress()).(*types.VaultAccount)
		s.True(gotBroken.Paused, "the vault failing validation should be paused")

		gotHealthy := s.simApp.AccountKeeper.GetAccount(s.ctx, healthy.GetAddress()).(*types.VaultAccount)
		s.Equal(underlying, gotHealthy.PaymentDenom, "a healthy vault must still be flattened alongside one that fails validation")
		s.False(gotHealthy.Paused, "a healthy vault must not be paused by the migration")
	})
}

func (s *TestSuite) TestVaultModule_RunMigrations() {
	underlying := "ylds"
	payment := "usdc"

	s.Run("vault pinned to v1 runs every registered migration in order", func() {
		s.SetupTest()
		s.ctx = s.ctx.WithChainID(types.MainnetChainID)
		legacy := s.createLegacyVaultAccount("vsharemmv1", underlying, payment)
		s.SetGovOnlyVaultCreation(false)

		fromVM := s.simApp.ModuleManager.GetVersionMap()
		fromVM[types.ModuleName] = 1

		newVM, err := s.simApp.ModuleManager.RunMigrations(s.ctx, s.simApp.Configurator(), fromVM)
		s.Require().NoError(err, "RunMigrations must succeed; the vault v1->v2 and v2->v3 handlers must be registered for vault %s", legacy.Address)
		s.Require().Equal(uint64(vault.ConsensusVersion), newVM[types.ModuleName], "vault module version should advance to the current ConsensusVersion")

		acct := s.simApp.AccountKeeper.GetAccount(s.ctx, legacy.GetAddress())
		got, ok := acct.(*types.VaultAccount)
		s.Require().True(ok, "account at %s should remain a VaultAccount", legacy.Address)
		s.Equal(underlying, got.PaymentDenom, "the v1->v2 migration should flatten the mixed vault %s", legacy.Address)

		_, err = s.simApp.VaultKeeper.NAVs.Get(s.ctx, collections.Join(legacy.GetAddress(), payment))
		s.Require().ErrorIs(err, collections.ErrNotFound, "the flatten migration must not seed internal NAV entries")

		govOnly, err := s.simApp.VaultKeeper.IsVaultCreationGovOnly(s.ctx)
		s.Require().NoError(err, "reading the gate after RunMigrations should not error")
		s.Assert().True(govOnly, "the v2->v3 migration should also run for a chain pinned to v1")
	})

	s.Run("vault pinned to v2 runs only the v2->v3 migration, not the v1->v2 flatten", func() {
		s.SetupTest()
		s.ctx = s.ctx.WithChainID(types.MainnetChainID)
		legacy := s.createLegacyVaultAccount("vsharemmv2", underlying, payment)
		s.SetGovOnlyVaultCreation(false)

		fromVM := s.simApp.ModuleManager.GetVersionMap()
		fromVM[types.ModuleName] = 2

		newVM, err := s.simApp.ModuleManager.RunMigrations(s.ctx, s.simApp.Configurator(), fromVM)
		s.Require().NoError(err, "RunMigrations must succeed; the vault v2->v3 handler must be registered")
		s.Require().Equal(uint64(vault.ConsensusVersion), newVM[types.ModuleName], "vault module version should advance to the current ConsensusVersion")

		govOnly, err := s.simApp.VaultKeeper.IsVaultCreationGovOnly(s.ctx)
		s.Require().NoError(err, "reading the gate after RunMigrations should not error")
		s.Assert().True(govOnly, "the v2->v3 migration should enable the gate for a chain pinned to v2")

		acct := s.simApp.AccountKeeper.GetAccount(s.ctx, legacy.GetAddress())
		got, ok := acct.(*types.VaultAccount)
		s.Require().True(ok, "account at %s should remain a VaultAccount", legacy.Address)
		s.Equal(payment, got.PaymentDenom, "starting at v2 must not re-run the v1->v2 flatten")
	})

	s.Run("version map already at current ConsensusVersion is a no-op", func() {
		s.SetupTest()
		s.ctx = s.ctx.WithChainID(types.MainnetChainID)
		legacy := s.createLegacyVaultAccount("vsharemmnoop", underlying, payment)
		s.SetGovOnlyVaultCreation(false)

		fromVM := s.simApp.ModuleManager.GetVersionMap()
		newVM, err := s.simApp.ModuleManager.RunMigrations(s.ctx, s.simApp.Configurator(), fromVM)
		s.Require().NoError(err, "RunMigrations should succeed when versions match")
		s.Require().Equal(fromVM[types.ModuleName], newVM[types.ModuleName], "vault module version should remain unchanged")

		acct := s.simApp.AccountKeeper.GetAccount(s.ctx, legacy.GetAddress())
		got, ok := acct.(*types.VaultAccount)
		s.Require().True(ok, "account at %s should remain a VaultAccount", legacy.Address)
		s.Equal(payment, got.PaymentDenom, "a matching version map must not trigger the flatten")

		govOnly, err := s.simApp.VaultKeeper.IsVaultCreationGovOnly(s.ctx)
		s.Require().NoError(err, "reading the gate after a no-op RunMigrations should not error")
		s.Assert().False(govOnly, "a matching version map must not trigger the gate migration, even on mainnet")
	})
}

func (s *TestSuite) TestKeeper_MigrateEnableMarkerDepositProtection() {
	underlying := "ylds"

	runMigration := func() {
		s.Require().NoError(keeper.NewMigrator(s.simApp.VaultKeeper).Migrate1to2(s.ctx), "1->2 migration should succeed")
	}

	createLegacyShareMarker := func(shareDenom string) sdk.AccAddress {
		vaultAddr := types.GetVaultAddress(shareDenom)
		legacyMarker := markertypes.NewMarkerAccount(
			authtypes.NewBaseAccountWithAddress(markertypes.MustGetMarkerAddress(shareDenom)),
			sdk.NewInt64Coin(shareDenom, 0),
			vaultAddr,
			[]markertypes.AccessGrant{
				{
					Address: vaultAddr.String(),
					Permissions: []markertypes.Access{
						markertypes.Access_Mint,
						markertypes.Access_Burn,
						markertypes.Access_Withdraw,
					},
				},
			},
			markertypes.StatusProposed,
			markertypes.MarkerType_Coin,
			false, false, false, []string{},
		)
		s.Require().NoError(s.simApp.MarkerKeeper.AddFinalizeAndActivateMarker(s.ctx, legacyMarker), "creating legacy share marker %s should succeed", shareDenom)
		return vaultAddr
	}

	getShareMarker := func(shareDenom string) markertypes.MarkerAccountI {
		marker, err := s.simApp.MarkerKeeper.GetMarkerByDenom(s.ctx, shareDenom)
		s.Require().NoError(err, "share marker %s should exist", shareDenom)
		return marker
	}

	s.Run("legacy share marker gains deposit protection and vault deposit access", func() {
		s.SetupTest()
		shareDenom := "vsharelegacy"
		s.createLegacyVaultAccount(shareDenom, underlying, underlying)
		vaultAddr := createLegacyShareMarker(shareDenom)

		runMigration()

		marker := getShareMarker(shareDenom)
		s.True(marker.RequiresDepositAccess(), "migration must enable require_deposit_access on legacy share marker %s", shareDenom)
		s.True(marker.AddressHasAccess(vaultAddr, markertypes.Access_Deposit), "migration must grant the vault deposit access on share marker %s", shareDenom)
		s.True(marker.AddressHasAccess(vaultAddr, markertypes.Access_Mint), "existing mint grant must survive the deposit grant merge")
		s.True(marker.AddressHasAccess(vaultAddr, markertypes.Access_Burn), "existing burn grant must survive the deposit grant merge")
		s.True(marker.AddressHasAccess(vaultAddr, markertypes.Access_Withdraw), "existing withdraw grant must survive the deposit grant merge")
	})

	s.Run("second run is idempotent", func() {
		s.SetupTest()
		shareDenom := "vshareidem"
		s.createLegacyVaultAccount(shareDenom, underlying, underlying)
		createLegacyShareMarker(shareDenom)

		runMigration()
		first := getShareMarker(shareDenom)
		runMigration()
		second := getShareMarker(shareDenom)

		s.Equal(first, second, "an already-protected share marker must not be modified by a second migration run")
	})

	s.Run("vault with missing share marker is skipped without failing", func() {
		s.SetupTest()
		shareDenom := "vsharenomarker"
		s.createLegacyVaultAccount(shareDenom, underlying, underlying)

		runMigration()

		_, err := s.simApp.MarkerKeeper.GetMarkerByDenom(s.ctx, shareDenom)
		s.Require().Error(err, "migration must not conjure a share marker for a vault that has none")
	})

	s.Run("markers created by current CreateVault are left untouched", func() {
		s.SetupTest()
		shareDenom := "vsharecurrent"
		s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlying, 1_000_000), s.adminAddr)
		s.CreateVaultWithParams(shareDenom, underlying)
		before := getShareMarker(shareDenom)

		runMigration()

		after := getShareMarker(shareDenom)
		s.Equal(before, after, "a marker already carrying deposit protection from creation must pass through the migration unchanged")
	})
}

func (s *TestSuite) TestKeeper_MigrateEnableGovOnlyVaultCreation() {
	tests := []struct {
		name     string
		chainID  string
		setup    func()
		expected bool
	}{
		{
			name:     "mainnet params stored without the gate, as an upgrading chain has them",
			chainID:  types.MainnetChainID,
			setup:    func() { s.SetGovOnlyVaultCreation(false) },
			expected: true,
		},
		{
			name:     "mainnet gate already enabled is left enabled",
			chainID:  types.MainnetChainID,
			setup:    func() { s.SetGovOnlyVaultCreation(true) },
			expected: true,
		},
		{
			name:    "mainnet with no params stored at all",
			chainID: types.MainnetChainID,
			setup: func() {
				s.Require().NoError(s.k.Params.Remove(s.ctx), "failed to clear stored params")
			},
			expected: true,
		},
		{
			name:     "testnet is left open",
			chainID:  types.TestnetChainID,
			setup:    func() { s.SetGovOnlyVaultCreation(false) },
			expected: false,
		},
		{
			name:     "local chain is left open",
			chainID:  "vaulty-1",
			setup:    func() { s.SetGovOnlyVaultCreation(false) },
			expected: false,
		},
		{
			name:     "a chain that already enabled the gate itself keeps it regardless of chain ID",
			chainID:  "vaulty-1",
			setup:    func() { s.SetGovOnlyVaultCreation(true) },
			expected: true,
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			s.ctx = s.ctx.WithChainID(tc.chainID)
			tc.setup()

			migrator := keeper.NewMigrator(s.simApp.VaultKeeper)
			s.Require().NoError(migrator.Migrate2to3(s.ctx), "2->3 migration should succeed on chain %s", tc.chainID)

			govOnly, err := s.k.IsVaultCreationGovOnly(s.ctx)
			s.Require().NoError(err, "reading the gate after the 2->3 migration should not error")
			s.Assert().Equal(tc.expected, govOnly, "gov-only vault creation gate after the 2->3 migration on chain %s", tc.chainID)

			s.Require().NoError(migrator.Migrate2to3(s.ctx), "the 2->3 migration should be idempotent across retries")
			govOnly, err = s.k.IsVaultCreationGovOnly(s.ctx)
			s.Require().NoError(err, "reading the gate after a repeated 2->3 migration should not error")
			s.Assert().Equal(tc.expected, govOnly, "a repeated 2->3 migration must not change the gate on chain %s", tc.chainID)
		})
	}
}

func (s *TestSuite) TestKeeper_SeedTotalValues() {
	underlyingDenom := "ylds"
	shareDenom := "vshare"
	heldDenom := "usdc"

	seed := func() error {
		return s.simApp.VaultKeeper.HydrateTotalValues(s.ctx)
	}

	requireInvariant := func(expectBroken bool, when string) {
		msg, broken := keeper.TotalValueInvariant(s.k)(s.ctx)
		s.Require().Equal(expectBroken, broken, "total value invariant state %s the seeding run: %s", when, msg)
	}

	tests := []struct {
		name string
		// setup stages chain state and returns the total the run must store for each vault,
		// keyed by bech32 address, plus the vaults that must be left with no stored total.
		setup                 func() (map[string]int64, []sdk.AccAddress)
		runs                  int
		invariantBrokenBefore bool
		invariantBrokenAfter  bool
	}{
		{
			name: "a vault whose total value was never materialized is seeded from the vault lookup",
			setup: func() (map[string]int64, []sdk.AccAddress) {
				v := s.setupHeldAssetVault(underlyingDenom, shareDenom, heldDenom, 1, 2)
				s.Require().NoError(
					FundAccount(s.ctx, s.simApp, v.PrincipalMarkerAddress(), sdk.NewCoins(
						sdk.NewInt64Coin(underlyingDenom, 1_000),
						sdk.NewInt64Coin(heldDenom, 10),
					)),
					"funding the principal should succeed",
				)
				s.dropStoredTotalValue(v.GetAddress())
				return map[string]int64{v.GetAddress().String(): 1_005}, nil
			},
			invariantBrokenBefore: true,
		},
		{
			name: "every vault in the lookup is seeded, restoring the total value invariant",
			setup: func() (map[string]int64, []sdk.AccAddress) {
				first := s.setupHeldAssetVault(underlyingDenom, shareDenom, heldDenom, 1, 2)
				second := s.setupBaseVault("uusd", "vsharetwo")
				s.Require().NoError(
					FundAccount(s.ctx, s.simApp, first.PrincipalMarkerAddress(), sdk.NewCoins(sdk.NewInt64Coin(heldDenom, 10))),
					"funding the first principal should succeed",
				)
				s.dropStoredTotalValue(first.GetAddress())
				s.dropStoredTotalValue(second.GetAddress())
				return map[string]int64{
					first.GetAddress().String():  5,
					second.GetAddress().String(): 0,
				}, nil
			},
			invariantBrokenBefore: true,
		},
		{
			name: "rerunning the seeding recomputes the same total",
			setup: func() (map[string]int64, []sdk.AccAddress) {
				v := s.setupHeldAssetVault(underlyingDenom, shareDenom, heldDenom, 1, 2)
				s.Require().NoError(
					FundAccount(s.ctx, s.simApp, v.PrincipalMarkerAddress(), sdk.NewCoins(sdk.NewInt64Coin(heldDenom, 10))),
					"funding the principal should succeed",
				)
				return map[string]int64{v.GetAddress().String(): 5}, nil
			},
			runs: 2,
		},
		{
			name: "a lookup entry with no vault account is skipped without failing the upgrade or the invariant",
			setup: func() (map[string]int64, []sdk.AccAddress) {
				orphan := sdk.AccAddress("orphanVaultAddress__")
				s.Require().NoError(s.k.Vaults.Set(s.ctx, orphan, []byte{}),
					"seeding an orphaned vault lookup entry should succeed")
				return nil, []sdk.AccAddress{orphan}
			},
			invariantBrokenBefore: false,
			invariantBrokenAfter:  false,
		},
		{
			name: "a vault that cannot be valued is skipped while every healthy vault still gets seeded",
			setup: func() (map[string]int64, []sdk.AccAddress) {
				healthy := s.setupBaseVault("uusd", "vsharetwo")
				s.Require().NoError(
					FundAccount(s.ctx, s.simApp, healthy.PrincipalMarkerAddress(), sdk.NewCoins(sdk.NewInt64Coin("uusd", 2_500))),
					"funding the healthy principal should succeed",
				)

				unvaluable, _, unvaluableUnderlying, unvaluableHeld := s.setupOversizedNAVVault()
				s.seedOversizedNAV(unvaluable, unvaluableHeld, unvaluableUnderlying, maxValidNAVPrice(), sdkmath.OneInt())
				s.fundPrincipalForBrokenValuation(unvaluable, sdk.NewCoins(
					sdk.NewInt64Coin(unvaluableHeld, 1),
					sdk.NewInt64Coin(unvaluableUnderlying, 100),
				))
				s.dropStoredTotalValue(healthy.GetAddress())

				return map[string]int64{healthy.GetAddress().String(): 2_500}, []sdk.AccAddress{unvaluable.GetAddress()}
			},
			invariantBrokenBefore: true,
			invariantBrokenAfter:  false,
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			expectedTotals, expectedUnseeded := tc.setup()

			requireInvariant(tc.invariantBrokenBefore, "before")

			runs := tc.runs
			if runs == 0 {
				runs = 1
			}
			for run := 1; run <= runs; run++ {
				s.Require().NoError(seed(), "seeding run %d must succeed; one unusable vault must never abort the upgrade", run)
			}

			for addr, expected := range expectedTotals {
				vaultAddr := sdk.MustAccAddressFromBech32(addr)
				stored, err := s.k.TotalValues.Get(s.ctx, vaultAddr)
				s.Require().NoError(err, "seeding should have stored a total for vault %s", vaultAddr)
				s.Require().Equal(sdkmath.NewInt(expected).String(), stored.String(),
					"seeded total mismatch for vault %s", vaultAddr)
			}

			for _, vaultAddr := range expectedUnseeded {
				_, err := s.k.TotalValues.Get(s.ctx, vaultAddr)
				s.Require().ErrorIs(err, collections.ErrNotFound,
					"no total should be stored for vault %s, which cannot be loaded or valued", vaultAddr)
			}

			requireInvariant(tc.invariantBrokenAfter, "after")
		})
	}
}

func (s *TestSuite) TestVaultModule_RunMigrationsSeedsTotalValues() {
	underlyingDenom := "ylds"
	shareDenom := "vshare"

	s.Run("vault pinned to v2 advances to the current version and gets its total seeded", func() {
		s.SetupTest()
		v := s.setupBaseVault(underlyingDenom, shareDenom)
		s.dropStoredTotalValue(v.GetAddress())

		fromVM := s.simApp.ModuleManager.GetVersionMap()
		fromVM[types.ModuleName] = 2

		newVM, err := s.simApp.ModuleManager.RunMigrations(s.ctx, s.simApp.Configurator(), fromVM)
		s.Require().NoError(err, "RunMigrations must succeed; the vault v2->v3 handler must be registered")
		s.Require().Equal(uint64(vault.ConsensusVersion), newVM[types.ModuleName],
			"vault module version should advance to the current ConsensusVersion")

		_, err = s.k.TotalValues.Get(s.ctx, v.GetAddress())
		s.Require().NoError(err, "the v2->v3 migration should have seeded vault %s", v.GetAddress())
	})
}

func (s *TestSuite) TestMigrate2to3_DerivesNAVEntryCounts() {
	underlying := "navcountunder"

	tests := []struct {
		name     string
		share    string
		denoms   []string
		expCount uint64
	}{
		{
			name:     "vault pricing no denoms records nothing",
			share:    "navcountnone",
			denoms:   nil,
			expCount: 0,
		},
		{
			name:     "vault pricing one denom records one",
			share:    "navcountone",
			denoms:   []string{"navcountasseta"},
			expCount: 1,
		},
		{
			name:     "vault pricing several denoms records each of them",
			share:    "navcountmany",
			denoms:   []string{"navcountassetb", "navcountassetc", "navcountassetd"},
			expCount: 3,
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			vault := s.setupBaseVault(underlying, tc.share)

			for _, denom := range tc.denoms {
				s.requireSimpleMarker(denom)
				nav := types.NewVaultNAV(denom, sdk.NewInt64Coin(underlying, 1), sdkmath.NewInt(1), "migrationtest")
				s.Require().NoError(s.k.NAVs.Set(s.ctx, collections.Join(vault.GetAddress(), denom), nav),
					"failed to seed a pre-migration NAV entry for %s", denom)
			}
			s.Require().NoError(s.k.NAVCounts.Remove(s.ctx, vault.GetAddress()),
				"failed to clear the NAV entry count so the vault looks pre-migration")

			migrator := keeper.NewMigrator(s.simApp.VaultKeeper)
			s.Require().NoError(migrator.Migrate2to3(s.ctx), "2->3 migration should succeed")

			count, err := s.k.NAVEntryCount(s.ctx, vault.GetAddress())
			s.Require().NoError(err, "failed to read the derived NAV entry count for vault %s", vault.Address)
			s.Require().Equal(tc.expCount, count,
				"the migration should derive %d priced denoms for vault %s", tc.expCount, vault.Address)

			s.Require().NoError(migrator.Migrate2to3(s.ctx), "the 2->3 migration should be idempotent across retries")
			count, err = s.k.NAVEntryCount(s.ctx, vault.GetAddress())
			s.Require().NoError(err, "failed to re-read the NAV entry count for vault %s", vault.Address)
			s.Require().Equal(tc.expCount, count,
				"a repeated migration must not change the NAV entry count for vault %s", vault.Address)
		})
	}
}

func (s *TestSuite) TestMigrate2to3_GrandfathersATableOverTheCap() {
	underlying := "grandunder"
	share := "grandshares"
	vault := s.setupBaseVault(underlying, share)

	existingDenom := "grandheld"
	s.requireSimpleMarker(existingDenom)
	existing := types.NewVaultNAV(existingDenom, sdk.NewInt64Coin(underlying, 1), sdkmath.NewInt(1), "grandtest")
	s.Require().NoError(s.k.NAVs.Set(s.ctx, collections.Join(vault.GetAddress(), existingDenom), existing),
		"failed to seed an existing NAV entry on the over-cap vault")

	overCap := uint64(types.MaxVaultNAVEntries + 1)
	s.Require().NoError(s.k.NAVCounts.Set(s.ctx, vault.GetAddress(), overCap),
		"failed to seed a recorded count above the cap")

	newDenom := "grandasset"
	s.requireSimpleMarker(newDenom)
	nav := types.NewVaultNAV(newDenom, sdk.NewInt64Coin(underlying, 1), sdkmath.NewInt(1), "grandtest")
	err := s.k.SetVaultNAV(s.ctx, vault, nav, s.adminAddr.String())
	s.Require().ErrorContains(err, fmt.Sprintf("max %d", types.MaxVaultNAVEntries),
		"a vault over the cap must not be able to price another denom")

	s.Require().NoError(s.k.RemoveVaultNAV(s.ctx, vault, existingDenom, s.adminAddr.String()),
		"an over-cap vault must still be able to prune entries")

	count, err := s.k.NAVEntryCount(s.ctx, vault.GetAddress())
	s.Require().NoError(err, "failed to read the NAV entry count for vault %s", vault.Address)
	s.Require().Equal(overCap-1, count,
		"pruning an entry must walk an over-cap vault back toward the cap")
}
