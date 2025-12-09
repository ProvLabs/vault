package keeper

import (
	"context"
	"fmt"

	"github.com/provlabs/vault/interest"
	"github.com/provlabs/vault/types"

	"cosmossdk.io/collections"
	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"

	markertypes "github.com/provenance-io/provenance/x/marker/types"
)

const (
	// AutoReconcilePayoutDuration is the time period (in seconds) used to forecast if a vault has
	// sufficient funds to cover future interest payments.
	AutoReconcilePayoutDuration = 24 * interest.SecondsPerHour
)

// reconcileVaultInterest updates interest accounting for a vault if a new interest period has started.
//
// If this is the first time the vault accrues interest, it triggers the start of a new period
// and publishes the initial NAV for the share denom in terms of the underlying asset.
// If the current block time is after PeriodStart, it applies the interest transfer.
// If the current time has not advanced past PeriodStart, it is a no-op.
// This function will do nothing if the vault is paused.
//
// This should be called before any transaction that changes vault principal/reserves or depends on the
// current interest state.
func (k *Keeper) reconcileVaultInterest(ctx sdk.Context, vault *types.VaultAccount) error {
	if vault.Paused {
		return nil
	}
	currentBlockTime := ctx.BlockTime().Unix()

	if vault.PeriodStart != 0 {
		if currentBlockTime <= vault.PeriodStart {
			return nil
		}

		if err := k.PerformVaultInterestTransfer(ctx, vault); err != nil {
			return err
		}
		if err := k.publishShareNav(ctx, vault); err != nil {
			return err
		}
	}

	return k.SafeAddVerification(ctx, vault)
}

// publishShareNav records the Net Asset Value (NAV) for the vault's share denom
// in terms of its underlying asset. It fetches the vault’s principal marker,
// computes the total value of vault assets in underlying units (TVV), and sets
// the NAV as (Price = TVV in underlying, Volume = total shares). If no shares
// exist or the TVV is non-positive, no NAV is published.
func (k *Keeper) publishShareNav(ctx sdk.Context, vault *types.VaultAccount) error {
	vaultMarker, err := k.MarkerKeeper.GetMarker(ctx, vault.PrincipalMarkerAddress())
	if err != nil {
		return fmt.Errorf("failed to get principal marker: %w", err)
	}
	if !vault.TotalShares.IsPositive() {
		return nil
	}
	tvv, err := k.GetTVVInUnderlyingAsset(ctx, *vault)
	if err != nil {
		return fmt.Errorf("failed to get TVV: %w", err)
	}
	if !tvv.IsPositive() {
		return nil
	}

	k.MarkerKeeper.SetNetAssetValue(ctx, vaultMarker, markertypes.NetAssetValue{
		Price:  sdk.NewCoin(vault.UnderlyingAsset, tvv),
		Volume: vault.TotalShares.Amount.Uint64(),
	}, types.ModuleName)
	return nil
}

// PerformVaultInterestTransfer applies accrued interest between the vault and the marker account
// if the current block time is beyond PeriodStart.
//
// Interest is settled exclusively in the vault's defined UnderlyingAsset.
//   - Positive Interest: Paid from vault reserves to the marker. Fails if reserves are insufficient.
//   - Negative Interest: Refunded from marker principal to the vault. This is bounded by the
//     available balance of the UnderlyingAsset in the marker account.
//
// IMPORTANT: If the vault utilizes composite reserves (holding multiple token types), secondary
// assets are NOT liquidated or transferred to satisfy interest obligations. If the marker owes
// negative interest but lacks sufficient liquidity in the UnderlyingAsset, the transfer is
// capped at the available underlying balance, potentially resulting in a partial payment.
//
// An EventVaultReconcile is emitted upon success. This method does not modify PeriodStart.
func (k *Keeper) PerformVaultInterestTransfer(ctx sdk.Context, vault *types.VaultAccount) error {
	currentBlockTime := ctx.BlockTime().Unix()
	if currentBlockTime <= vault.PeriodStart {
		return nil
	}

	periodDuration := currentBlockTime - vault.PeriodStart
	denom := vault.UnderlyingAsset
	vaultAddr := vault.GetAddress()
	principalAddress := vault.PrincipalMarkerAddress()

	reserves := k.BankKeeper.GetBalance(ctx, vaultAddr, denom)
	principalTvv, err := k.GetTVVInUnderlyingAsset(ctx, *vault)
	if err != nil {
		return err
	}
	principalInTvv := sdk.NewCoin(denom, principalTvv)

	interestEarned, err := interest.CalculateInterestEarned(principalInTvv, vault.CurrentInterestRate, periodDuration)
	if err != nil {
		return fmt.Errorf("failed to calculate interest: %w", err)
	}

	actualInterest := interestEarned

	if interestEarned.IsPositive() {
		if reserves.Amount.LT(interestEarned) {
			return fmt.Errorf("insufficient reserves to pay interest")
		}
		if err := k.BankKeeper.SendCoins(
			markertypes.WithBypass(ctx),
			vaultAddr,
			principalAddress,
			sdk.NewCoins(sdk.NewCoin(denom, interestEarned)),
		); err != nil {
			return fmt.Errorf("failed to pay interest: %w", err)
		}
	} else if interestEarned.IsNegative() {
		principalUnderlying := k.BankKeeper.GetBalance(ctx, principalAddress, denom)
		owed := interestEarned.Abs()
		if principalUnderlying.Amount.LT(owed) {
			owed = principalUnderlying.Amount
		}
		if owed.IsZero() {
			actualInterest = sdkmath.ZeroInt()
		} else {
			if err := k.BankKeeper.SendCoins(
				markertypes.WithBypass(ctx),
				principalAddress,
				vaultAddr,
				sdk.NewCoins(sdk.NewCoin(denom, owed)),
			); err != nil {
				return fmt.Errorf("failed to reclaim negative interest: %w", err)
			}
			actualInterest = owed.Neg()
		}
	}

	principalTvvAfter, err := k.GetTVVInUnderlyingAsset(ctx, *vault)
	if err != nil {
		return err
	}

	k.emitEvent(ctx, types.NewEventVaultReconcile(
		vaultAddr.String(),
		principalInTvv,
		sdk.NewCoin(denom, principalTvvAfter),
		vault.CurrentInterestRate,
		periodDuration,
		actualInterest,
	))

	return nil
}

// CanPayoutDuration determines whether the vault can fulfill the projected
// interest payment or refund over the given duration based on current reserves
// and principal TVV.
//
// It returns true when duration <= 0, when accrued interest is zero, when
// positive interest can be paid from vault reserves, or when negative interest
// can be fully covered by the principal marker's underlying balance.
// Otherwise it returns false.
func (k *Keeper) CanPayoutDuration(ctx sdk.Context, vault *types.VaultAccount, duration int64) (bool, error) {
	if duration <= 0 {
		return true, nil
	}

	underlyingDenom := vault.UnderlyingAsset
	vaultAddr := vault.GetAddress()
	principalAddr := vault.PrincipalMarkerAddress()

	principalTvv, err := k.GetTVVInUnderlyingAsset(ctx, *vault)
	if err != nil {
		return false, err
	}
	if principalTvv.IsZero() {
		return true, nil
	}

	principalCoin := sdk.NewCoin(underlyingDenom, principalTvv)

	interestEarned, err := interest.CalculateInterestEarned(principalCoin, vault.CurrentInterestRate, duration)
	if err != nil {
		return false, fmt.Errorf("failed to calculate interest: %w", err)
	}

	if interestEarned.IsZero() {
		return true, nil
	}

	if interestEarned.IsPositive() {
		reserves := k.BankKeeper.GetBalance(ctx, vaultAddr, underlyingDenom)
		return !reserves.Amount.LT(interestEarned), nil
	}

	principalUnderlying := k.BankKeeper.GetBalance(ctx, principalAddr, underlyingDenom)
	owed := interestEarned.Abs()

	return !principalUnderlying.Amount.LT(owed), nil
}

// UpdateInterestRates sets the vault's current and desired interest rates and emits
// an EventVaultInterestChange. The modified account is persisted via the auth keeper.
func (k *Keeper) UpdateInterestRates(ctx sdk.Context, vault *types.VaultAccount, currentRate, desiredRate string) error {
	event := types.NewEventVaultInterestChange(vault.GetAddress().String(), currentRate, desiredRate)
	vault.CurrentInterestRate = currentRate
	vault.DesiredInterestRate = desiredRate
	if err := k.SetVaultAccount(ctx, vault); err != nil {
		return err
	}
	k.emitEvent(sdk.UnwrapSDKContext(ctx), event)
	return nil
}

// CalculateVaultTotalAssets returns the total value of the vault's assets, including the interest
// that would have accrued from PeriodStart to the current block time, without mutating state.
//
// If no rate is set or accrual has not started, it returns the provided principal unchanged.
func (k Keeper) CalculateVaultTotalAssets(ctx sdk.Context, vault *types.VaultAccount, principal sdk.Coin) (sdkmath.Int, error) {
	estimated := principal.Amount

	if vault.CurrentInterestRate == "" || vault.PeriodStart == 0 {
		return estimated, nil
	}

	duration := ctx.BlockTime().Unix() - vault.PeriodStart
	if duration <= 0 {
		return estimated, nil
	}

	interestEarned, err := interest.CalculateInterestEarned(principal, vault.CurrentInterestRate, duration)
	if err != nil {
		return sdkmath.Int{}, fmt.Errorf("error calculating interest: %w", err)
	}

	return estimated.Add(interestEarned), nil
}

// handleVaultInterestTimeouts checks vaults with expired interest periods and reconciles or disables them.
// It uses a safe "collect-then-mutate" pattern to comply with the SDK iterator contract.
// For each due timeout entry:
//   - It collects keys for all non-paused vaults.
//   - It then iterates the collected keys, dequeuing each item before processing it.
//   - Vaults that cannot cover the required interest are marked depleted.
//   - Otherwise, interest is reconciled.
func (k *Keeper) handleVaultInterestTimeouts(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	now := sdkCtx.BlockTime().Unix()

	var keysToProcess []collections.Pair[uint64, sdk.AccAddress]
	var depleted []*types.VaultAccount
	var reconciled []*types.VaultAccount

	err := k.PayoutTimeoutQueue.WalkDue(ctx, now, func(timeout uint64, addr sdk.AccAddress) (bool, error) {
		key := collections.Join(timeout, addr)
		vault, ok := k.tryGetVault(sdkCtx, addr)
		if ok && vault.Paused {
			return false, nil
		}
		keysToProcess = append(keysToProcess, key)
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("walk failed: %w", err)
	}

	for _, key := range keysToProcess {
		timeout := key.K1()
		addr := key.K2()

		if err := k.PayoutTimeoutQueue.Dequeue(ctx, int64(timeout), addr); err != nil {
			sdkCtx.Logger().Error("CRITICAL: failed to dequeue interest timeout, skipping", "vault", addr.String(), "err", err)
			continue
		}

		vault, ok := k.tryGetVault(sdkCtx, addr)
		if !ok {
			continue
		}

		periodDuration := int64(timeout) - vault.PeriodStart
		if periodDuration < 0 {
			periodDuration = now - vault.PeriodStart
		}

		canPay, err := k.CanPayoutDuration(sdkCtx, vault, periodDuration)
		if err != nil {
			sdkCtx.Logger().Error("failed to check payout ability", "vault", addr.String(), "err", err)
			continue
		}

		if !canPay {
			depleted = append(depleted, vault)
			continue
		}

		if err := k.PerformVaultInterestTransfer(sdkCtx, vault); err != nil {
			sdkCtx.Logger().Error("failed to reconcile interest", "vault", addr.String(), "err", err)
			continue
		}

		reconciled = append(reconciled, vault)
	}

	k.resetVaultInterestPeriods(sdkCtx, reconciled)
	k.handleDepletedVaults(sdkCtx, depleted)
	return nil
}

// tryGetVault returns the vault if found, or false if the vault is missing or invalid.
// It should only be used in BeginBlocker/EndBlocker logic where failure is non-critical.
func (k *Keeper) tryGetVault(ctx sdk.Context, addr sdk.AccAddress) (*types.VaultAccount, bool) {
	vault, err := k.GetVault(ctx, addr)
	if err != nil {
		ctx.Logger().Error("failed to get vault", "address", addr.String(), "error", err)
		return nil, false
	}
	if vault == nil {
		ctx.Logger().Error("vault not found", "address", addr.String())
		return nil, false
	}
	return vault, true
}

// handleReconciledVaults processes vaults from the payout verification queue using a safe
// "collect-then-mutate" pattern.
//
// It first collects keys for all non-paused vaults. It then iterates the collected keys, removing
// each from the set before partitioning them into payable vs depleted groups.
func (k *Keeper) handleReconciledVaults(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	var keysToProcess []sdk.AccAddress
	var vaultsToProcess []*types.VaultAccount

	err := k.PayoutVerificationSet.Walk(ctx, nil, func(addr sdk.AccAddress) (bool, error) {
		v, ok := k.tryGetVault(sdkCtx, addr)
		if ok && v.Paused {
			return false, nil
		}
		keysToProcess = append(keysToProcess, addr)
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("walk failed: %w", err)
	}

	for _, addr := range keysToProcess {
		if err := k.PayoutVerificationSet.Remove(ctx, addr); err != nil {
			sdkCtx.Logger().Error("CRITICAL: failed to remove from payout verification set, skipping", "vault", addr.String(), "err", err)
			continue
		}

		v, ok := k.tryGetVault(sdkCtx, addr)
		if ok && !v.Paused {
			vaultsToProcess = append(vaultsToProcess, v)
		}
	}

	payable, depleted := k.partitionVaults(sdkCtx, vaultsToProcess)
	k.handlePayableVaults(sdkCtx, payable)
	k.handleDepletedVaults(sdkCtx, depleted)
	return nil
}

// partitionVaults splits the provided vaults into payable and depleted groups for the
// AutoReconcilePayoutDuration forecast window using CanPayoutDuration.
func (k *Keeper) partitionVaults(sdkCtx sdk.Context, vaults []*types.VaultAccount) ([]*types.VaultAccount, []*types.VaultAccount) {
	var payable []*types.VaultAccount
	var depleted []*types.VaultAccount
	for _, v := range vaults {
		ok, err := k.CanPayoutDuration(sdkCtx, v, AutoReconcilePayoutDuration)
		if err != nil {
			sdkCtx.Logger().Error("failed to check payout ability", "vault", v.GetAddress().String(), "err", err)
			continue
		}
		if ok {
			payable = append(payable, v)
		} else {
			depleted = append(depleted, v)
		}
	}
	return payable, depleted
}

// handlePayableVaults updates timeout tracking for vaults that remain payable after reconciliation.
// It sets PeriodTimeout to now + AutoReconcileTimeout, persists the vault, and enqueues the timeout.
func (k *Keeper) handlePayableVaults(ctx sdk.Context, payouts []*types.VaultAccount) {
	for _, v := range payouts {
		if err := k.SafeEnqueueTimeout(ctx, v); err != nil {
			ctx.Logger().Error("failed to enqueue timeout", "vault", v.GetAddress().String(), "err", err)
		}
	}
}

// handleDepletedVaults disables interest for vaults that cannot cover the forecasted payout window
// by setting the current rate to zero while preserving the desired rate.
func (k *Keeper) handleDepletedVaults(ctx sdk.Context, failedPayouts []*types.VaultAccount) {
	for _, record := range failedPayouts {
		if err := k.UpdateInterestRates(ctx, record, types.ZeroInterestRate, record.DesiredInterestRate); err != nil {
			ctx.Logger().Error("failed to update interest rates", "vault", record.GetAddress().String(), "err", err)
		}
	}
}

// resetVaultInterestPeriods starts a new accrual period for the provided vaults after a successful
// interest reconciliation by calling SafeEnqueueTimeout for each.
//
// This updates PeriodStart and PeriodTimeout, persists the vault, and enqueues the corresponding timeout entry.
func (k *Keeper) resetVaultInterestPeriods(ctx sdk.Context, vaults []*types.VaultAccount) {
	for _, vault := range vaults {
		if err := k.SafeEnqueueTimeout(ctx, vault); err != nil {
			ctx.Logger().Error("failed to enqueue vault timeout", "vault", vault.GetAddress().String(), "err", err)
		}
	}
}
