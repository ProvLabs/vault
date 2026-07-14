package keeper

import (
	"fmt"

	"github.com/provlabs/vault/types"

	"cosmossdk.io/collections"
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

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
			if err := k.SetVaultAccount(ctx, vault); err != nil {
				return fmt.Errorf("failed to persist flattened vault %s: %w", vault.Address, err)
			}
		}
	}

	if err := k.migratePendingSwapOutRedeemDenoms(ctx); err != nil {
		return fmt.Errorf("failed to migrate pending swap-out redeem denoms: %w", err)
	}

	return nil
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
func (k Keeper) migratePendingSwapOutRedeemDenoms(ctx sdk.Context) error {
	type queuedRewrite struct {
		key collections.Triple[int64, uint64, sdk.AccAddress]
		req types.PendingSwapOut
	}

	var rewrites []queuedRewrite
	err := k.PendingSwapOutQueue.Walk(ctx, func(timestamp int64, id uint64, vaultAddr sdk.AccAddress, req types.PendingSwapOut) (bool, error) {
		vault, err := k.GetVault(ctx, vaultAddr)
		if err != nil || vault == nil {
			return true, fmt.Errorf("failed to load vault %s for pending swap-out %d: %w", vaultAddr, id, err)
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
