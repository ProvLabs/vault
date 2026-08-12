package keeper

import (
	"errors"
	"fmt"

	"github.com/provlabs/vault/types"

	"cosmossdk.io/collections"
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"

	markertypes "github.com/provenance-io/provenance/x/marker/types"
)

// migrationInvalidVaultPauseReason is the stable PausedReason recorded for a legacy vault
// the v1->v2 flatten could not persist through validation, so an operator can find every
// vault whose configuration an admin must correct before unpausing.
const migrationInvalidVaultPauseReason = "paused by v1->v2 migration: legacy configuration fails current validation"

// migrateFlattenMixedDenomVaults rewrites all vault state so every vault is
// strictly single-denom on its underlying asset. Mixed-denom vaults (a payment
// denom distinct from the underlying) predate the single-denom restriction; the
// live ones hold zero or dust balances, so no funds are moved — only their
// configuration is flattened.
//
// It is unexported on purpose: the only supported entry point is the
// Migrator.Migrate1to2 handler registered with the SDK module manager via
// module.RegisterServices. Running it directly from a downstream upgrade
// handler would bypass the ConsensusVersion tracking that makes the migration
// fire exactly once per chain.
//
// For each VaultAccount in state the migration:
//  1. Defaults v.NavAuthority to v.Admin when empty, matching how new vaults
//     are created.
//  2. Sets PaymentDenom equal to UnderlyingAsset. The deprecated field remains
//     on the wire for client compatibility, so it is normalized rather than
//     cleared.
//  3. Re-denominates OutstandingAumFee into the underlying asset. A zero or
//     empty fee is normalized directly. A non-zero fee in a foreign denom is
//     converted through the vault's internal NAV table when an entry exists;
//     when no price is available the fee is zeroed with an error-level log
//     rather than failing the upgrade, because a halted chain is strictly
//     worse than forfeiting a fee that live-state analysis shows is zero
//     everywhere today.
//
// It then rewrites any pending swap-out whose redeem denom is not the owning
// vault's underlying asset, so queued payouts settle in the only denom the
// flattened vault can redeem.
//
// No step fails over a single vault, since aborting RunMigrations halts every node
// at the upgrade height: an unpriceable fee is zeroed, an orphaned queue entry is
// skipped, and persistFlattenedVault pauses rather than rejects a vault that no
// longer satisfies the current validation.
//
// The migration is idempotent: already-flattened vaults and conforming queue
// entries are left untouched.
func (k Keeper) migrateFlattenMixedDenomVaults(ctx sdk.Context) error {
	for _, acc := range k.AuthKeeper.GetAllAccounts(ctx) {
		vault, ok := acc.(*types.VaultAccount)
		if !ok {
			continue
		}

		changed := false
		if vault.NavAuthority == "" {
			vault.NavAuthority = vault.Admin
			changed = true
		}

		if vault.PaymentDenom != vault.UnderlyingAsset {
			k.getLogger(ctx).Info("flattening mixed-denom vault to single-denom",
				"vault", vault.Address,
				"underlying_asset", vault.UnderlyingAsset,
				"payment_denom", vault.PaymentDenom,
			)
			vault.PaymentDenom = vault.UnderlyingAsset
			changed = true
		}

		changed = k.normalizeOutstandingAumFee(ctx, vault) || changed

		if changed {
			k.persistFlattenedVault(ctx, vault)
		}
	}

	if err := k.migratePendingSwapOutRedeemDenoms(ctx); err != nil {
		return fmt.Errorf("failed to migrate pending swap-out redeem denoms: %w", err)
	}

	return nil
}

// persistFlattenedVault writes a vault the flatten step modified. A legacy vault written
// under looser v1 rules may hold a field today's validation rejects, so rather than fail
// the upgrade it is paused — zeroing the current interest rate and clearing its queue
// entries so the questionable configuration never reaches the interest math — and then
// persisted, falling back to the unvalidated SetAccount like forcePauseVault does. The
// tolerated error rides out on EventVaultPaused.forced_error for an admin to correct.
//
// An already-paused vault keeps its frozen PausedBalance and original reason, but the
// pause becomes an unattributed forced one, so resuming it takes a management unpause.
func (k Keeper) persistFlattenedVault(ctx sdk.Context, vault *types.VaultAccount) {
	validationErr := k.SetVaultAccount(ctx, vault)
	if validationErr == nil {
		return
	}

	k.getLogger(ctx).Error("legacy vault fails current validation; pausing it instead of failing the upgrade",
		"vault", vault.Address,
		"err", validationErr,
	)

	reason := migrationInvalidVaultPauseReason
	pausedBalance := sdk.NewCoin(vault.UnderlyingAsset, math.ZeroInt())
	if vault.Paused {
		reason = vault.PausedReason
		if !vault.PausedBalance.Amount.IsNil() {
			pausedBalance = vault.PausedBalance
		}
	}

	k.applyPausedState(ctx, vault, reason, types.NoPauseAuthority, pausedBalance)

	if err := k.haltVaultAccrual(ctx, vault); err != nil {
		k.getLogger(ctx).Error("failed to halt accrual for invalid legacy vault; queue entries may remain",
			"vault", vault.Address,
			"err", err,
		)
	}

	if err := k.SetVaultAccount(ctx, vault); err != nil {
		k.getLogger(ctx).Error("paused legacy vault still fails validation; persisting without validation",
			"vault", vault.Address,
			"err", err,
		)
		k.AuthKeeper.SetAccount(ctx, vault)
	}

	k.emitEvent(ctx, types.NewEventVaultPaused(vault.Address, vault.Address, vault.PausedReason, vault.PausedBalance, true, validationErr.Error()))
}

// normalizeOutstandingAumFee re-denominates a vault's OutstandingAumFee into the
// underlying asset, reporting whether the vault was modified. A zero or empty
// fee becomes the zero coin of the underlying. A non-zero fee in a foreign
// denom is converted through the vault's internal NAV table; when no price is
// available the fee is zeroed with an error-level log so the upgrade cannot
// halt over an unpriceable liability (live-state analysis shows every vault's
// outstanding fee is zero today). By construction no path fails: every outcome
// resolves to a concrete fee value, so the only signal is whether the vault
// was modified.
func (k Keeper) normalizeOutstandingAumFee(ctx sdk.Context, vault *types.VaultAccount) bool {
	fee := vault.OutstandingAumFee
	if fee.Denom == vault.UnderlyingAsset && !fee.Amount.IsNil() {
		return false
	}

	if fee.Amount.IsNil() || fee.Amount.IsZero() {
		vault.OutstandingAumFee = sdk.NewCoin(vault.UnderlyingAsset, math.ZeroInt())
		return true
	}

	converted, err := k.ToUnderlyingAssetAmount(ctx, *vault, fee)
	if err != nil {
		k.getLogger(ctx).Error("zeroing outstanding AUM fee with no internal NAV price",
			"vault", vault.Address,
			"fee", fee.String(),
			"err", err,
		)
		vault.OutstandingAumFee = sdk.NewCoin(vault.UnderlyingAsset, math.ZeroInt())
		return true
	}

	k.getLogger(ctx).Info("re-denominated outstanding AUM fee into underlying asset",
		"vault", vault.Address,
		"old_fee", fee.String(),
		"new_fee", sdk.NewCoin(vault.UnderlyingAsset, converted).String(),
	)
	vault.OutstandingAumFee = sdk.NewCoin(vault.UnderlyingAsset, converted)
	return true
}

// migratePendingSwapOutRedeemDenoms rewrites every pending swap-out whose
// redeem denom differs from the owning vault's underlying asset. Entries are
// collected during the walk and written afterwards so the underlying iterator
// is never invalidated by a concurrent Set.
//
// An entry whose vault cannot be loaded is skipped with an error-level log
// rather than failing the upgrade: the runtime queue processing tolerates the
// same state by dequeuing such entries, so halting the chain over one orphaned
// entry would be strictly worse.
func (k Keeper) migratePendingSwapOutRedeemDenoms(ctx sdk.Context) error {
	type queuedRewrite struct {
		key collections.Triple[int64, uint64, sdk.AccAddress]
		req types.PendingSwapOut
	}

	var rewrites []queuedRewrite
	err := k.PendingSwapOutQueue.Walk(ctx, func(timestamp int64, id uint64, vaultAddr sdk.AccAddress, req types.PendingSwapOut) (bool, error) {
		vault, ok := k.tryGetVault(ctx, vaultAddr)
		if !ok {
			k.getLogger(ctx).Error("skipping pending swap-out with no usable vault; queue processing dequeues such entries",
				"vault", vaultAddr.String(),
				"swap_out_id", id,
			)
			return false, nil
		}
		if req.RedeemDenom == vault.UnderlyingAsset {
			return false, nil
		}
		k.getLogger(ctx).Info("rewriting pending swap-out redeem denom to underlying asset",
			"vault", vault.Address,
			"swap_out_id", id,
			"redeem_denom", req.RedeemDenom,
			"underlying_asset", vault.UnderlyingAsset,
		)
		req.RedeemDenom = vault.UnderlyingAsset
		rewrites = append(rewrites, queuedRewrite{key: collections.Join3(timestamp, id, vaultAddr), req: req})
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("failed to walk pending swap-out queue: %w", err)
	}

	for _, rewrite := range rewrites {
		if err := k.PendingSwapOutQueue.IndexedMap.Set(ctx, rewrite.key, rewrite.req); err != nil {
			return fmt.Errorf("failed to rewrite pending swap-out %d: %w", rewrite.key.K2(), err)
		}
	}

	return nil
}

// migrateEnableMarkerDepositProtection enables require_deposit_access on every
// vault's share marker and grants the vault address deposit access, skipping
// (with an error log) any vault whose marker cannot be loaded. Idempotent.
func (k Keeper) migrateEnableMarkerDepositProtection(ctx sdk.Context) error {
	for _, acc := range k.AuthKeeper.GetAllAccounts(ctx) {
		vault, ok := acc.(*types.VaultAccount)
		if !ok {
			continue
		}

		marker, err := k.MarkerKeeper.GetMarkerByDenom(ctx, vault.TotalShares.Denom)
		if err != nil {
			k.getLogger(ctx).Error("skipping deposit protection for vault with no loadable share marker",
				"vault", vault.Address,
				"share_denom", vault.TotalShares.Denom,
				"err", err,
			)
			continue
		}

		changed := false
		if !marker.RequiresDepositAccess() {
			marker.SetRequireDepositAccess(true)
			changed = true
		}

		vaultAddr := vault.GetAddress()
		if !marker.AddressHasAccess(vaultAddr, markertypes.Access_Deposit) {
			grant := markertypes.NewAccessGrant(vaultAddr, markertypes.AccessList{markertypes.Access_Deposit})
			if err := marker.GrantAccess(grant); err != nil {
				return fmt.Errorf("failed to grant deposit access to vault %s on share marker %s: %w", vault.Address, vault.TotalShares.Denom, err)
			}
			changed = true
		}

		if changed {
			k.getLogger(ctx).Info("enabled deposit protection on vault share marker",
				"vault", vault.Address,
				"share_denom", vault.TotalShares.Denom,
			)
			if err := k.MarkerKeeper.SetMarker(ctx, marker); err != nil {
				return fmt.Errorf("failed to persist deposit protection on share marker %s: %w", vault.TotalShares.Denom, err)
			}
		}
	}

	return nil
}

// migrateEnableGovOnlyVaultCreation turns the gov_only_vault_creation param on for mainnet,
// which enforced the governance gate before the param existed. Idempotent.
func (k Keeper) migrateEnableGovOnlyVaultCreation(ctx sdk.Context) error {
	if !types.GetDefaultGovOnlyVaultCreation(ctx.ChainID()) {
		return nil
	}

	params, err := k.Params.Get(ctx)
	if err != nil {
		if !errors.Is(err, collections.ErrNotFound) {
			return fmt.Errorf("failed to retrieve params: %w", err)
		}
		params = types.DefaultParams()
	}

	if params.GovOnlyVaultCreation {
		return nil
	}

	params.GovOnlyVaultCreation = true
	if err := k.Params.Set(ctx, params); err != nil {
		return fmt.Errorf("failed to persist gov-only vault creation param: %w", err)
	}

	k.getLogger(ctx).Info("enabled gov-only vault creation", "chain_id", ctx.ChainID())

	return nil
}
