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
	// AutoReconcileTimeout is the duration (in seconds) that a vault is considered recently reconciled
	// and is exempt from automatic interest checks in the BeginBlocker.
	AutoReconcileTimeout = 20 * interest.SecondsPerHour

	// AutoReconcilePayoutDuration is the time period (in seconds) used to forecast if a vault has
	// sufficient funds to cover future interest payments.
	AutoReconcilePayoutDuration = 24 * interest.SecondsPerHour
)

// ReconcileVaultInterest updates interest accounting for a vault if a new interest period has started.
//
// This should be called before any transaction that changes vault principal, reserves,
// or performs management actions that depend on current interest state.
// If a new interest period is detected, this will apply interest transfers and reset the period start.
//
// If no prior interest record exists, it initializes the period tracking.
func (k *Keeper) ReconcileVaultInterest(ctx sdk.Context, vault *types.VaultAccount) error {
	currentBlockTime := ctx.BlockTime().Unix()

	if vault.PeriodStart == 0 {
		vault.PeriodStart = currentBlockTime
		err := k.SetVaultAccount(ctx, vault)
		if err != nil {
			return err
		}
	} else {

		if currentBlockTime <= vault.PeriodStart {
			return nil
		}

		if err := k.PerformVaultInterestTransfer(ctx, vault); err != nil {
			return err
		}
	}
	return k.EnqueueVaultStart(ctx, currentBlockTime, vault.GetAddress())
}

// PerformVaultInterestTransfer applies accrued interest between the vault and the marker account
// if the current block time is beyond the start of the interest period.
//
// This function should be used in contexts where the interest period should be evaluated
// but not updated, such as BeginBlock processing. It checks whether the interest period
// has elapsed, calculates the earned or owed interest, performs the necessary transfer
// (including partial liquidation of principal if needed for negative interest),
// and emits a reconciliation event.
//
// This method does not update the PeriodStart timestamp.
func (k *Keeper) PerformVaultInterestTransfer(ctx sdk.Context, vault *types.VaultAccount) error {
	currentBlockTime := ctx.BlockTime().Unix()

	if currentBlockTime <= vault.PeriodStart {
		return nil
	}

	periodDuration := currentBlockTime - vault.PeriodStart
	markerAddress := markertypes.MustGetMarkerAddress(vault.ShareDenom)

	reserves := k.BankKeeper.GetBalance(ctx, vault.GetAddress(), vault.UnderlyingAssets[0])
	principal := k.BankKeeper.GetBalance(ctx, markerAddress, vault.UnderlyingAssets[0])

	interestEarned, err := interest.CalculateInterestEarned(principal, vault.CurrentInterestRate, periodDuration)
	if err != nil {
		return fmt.Errorf("failed to calculate interest: %w", err)
	}

	if interestEarned.IsPositive() {
		if reserves.Amount.LT(interestEarned) {
			return fmt.Errorf("insufficient reserves to pay interest")
		}

		if err := k.BankKeeper.SendCoins(markertypes.WithBypass(ctx),
			vault.GetAddress(),
			markerAddress,
			sdk.NewCoins(sdk.NewCoin(vault.UnderlyingAssets[0], interestEarned)),
		); err != nil {
			return fmt.Errorf("failed to pay interest: %w", err)
		}
	} else if interestEarned.IsNegative() {
		owed := interestEarned.Abs()
		if principal.Amount.LT(owed) {
			owed = principal.Amount
		}

		if err := k.BankKeeper.SendCoins(markertypes.WithBypass(ctx),
			markerAddress,
			vault.GetAddress(),
			sdk.NewCoins(sdk.NewCoin(vault.UnderlyingAssets[0], owed)),
		); err != nil {
			return fmt.Errorf("failed to reclaim negative interest: %w", err)
		}
	}

	principalAfter := k.BankKeeper.GetBalance(ctx, markerAddress, vault.UnderlyingAssets[0])

	k.emitEvent(ctx, types.NewEventVaultReconcile(
		vault.GetAddress().String(),
		principal,
		principalAfter,
		vault.CurrentInterestRate,
		periodDuration,
		interestEarned,
	))

	return nil
}

// CanPayoutDuration determines whether the vault can fulfill the interest payment
// or refund over the given duration, based on the current reserves and principal.
//
// It calculates the interest accrued (positive or negative) over the specified duration
// using the vault's interest rate. The result determines whether the vault can
// successfully execute the interest transfer:
//
//   - Returns true if the duration is zero or less (no accrual needed).
//   - Returns true if the interest is zero (no transfer needed).
//   - Returns true if the interest is positive and the vault's reserves are sufficient to pay it.
//   - Returns true if the interest is negative and the marker holds any principal (able to refund).
//   - Returns false otherwise.
//
// This function is typically used prior to reconciling interest or scheduling expiration.
func (k *Keeper) CanPayoutDuration(ctx sdk.Context, vault *types.VaultAccount, duration int64) (bool, error) {
	if duration <= 0 {
		return true, nil
	}

	denom := vault.UnderlyingAssets[0]
	vaultAddr := vault.GetAddress()
	markerAddr := markertypes.MustGetMarkerAddress(vault.ShareDenom)

	reserves := k.BankKeeper.GetBalance(ctx, vaultAddr, denom)
	principal := k.BankKeeper.GetBalance(ctx, markerAddr, denom)

	interestEarned, err := interest.CalculateInterestEarned(principal, vault.CurrentInterestRate, duration)
	if err != nil {
		return false, fmt.Errorf("failed to calculate interest: %w", err)
	}

	switch {
	case interestEarned.IsZero():
		return true, nil
	case interestEarned.IsPositive():
		return !reserves.Amount.LT(interestEarned), nil
	case interestEarned.IsNegative():
		return principal.Amount.IsPositive(), nil
	default:
		return false, fmt.Errorf("unexpected interest value: %s", interestEarned.String())
	}
}

// UpdateInterestRates updates the current and desired interest rates for a vault.
// This function will emit the NewEventVaultInterestChange event.
func (k *Keeper) UpdateInterestRates(ctx context.Context, vault *types.VaultAccount, currentRate, desiredRate string) {
	event := types.NewEventVaultInterestChange(vault.GetAddress().String(), currentRate, desiredRate)
	vault.CurrentInterestRate = currentRate
	vault.DesiredInterestRate = desiredRate
	k.AuthKeeper.SetAccount(ctx, vault)
	k.emitEvent(sdk.UnwrapSDKContext(ctx), event)
}

// CalculateVaultTotalAssets returns the total value of the vault's assets,
// including any interest that would have accrued since the last interest period start.
// This is used to simulate the value of earned interest as if a reconciliation had occurred
// at the current block time, without modifying state or transferring funds.
//
// If no interest rate is set or if the vault has not yet begun accruing interest,
// the original principal amount is returned unmodified.
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
//
// For each vault with an expired interest period:
// - If the vault is missing, it is skipped.
// - If the vault can't pay the required interest, it is marked as depleted.
// - Otherwise, interest is reconciled for the vault.
//
// After processing, any depleted vaults are handled to disable interest or take corrective action.
//
// This is intended to run during BeginBlock and ignores individual vault errors.
func (k *Keeper) handleVaultInterestTimeouts(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	now := sdkCtx.BlockTime().Unix()

	var processed []collections.Pair[uint64, sdk.AccAddress]
	var depleted []*types.VaultAccount
	var reconciled []sdk.AccAddress

	err := k.VaultTimeoutQueue.Walk(ctx, nil, func(key collections.Pair[uint64, sdk.AccAddress], _ collections.NoValue) (bool, error) {
		t := int64(key.K1())
		addr := key.K2()

		if t > now {
			return true, nil
		}

		vault, ok := k.tryGetVault(sdkCtx, addr)
		if !ok {
			processed = append(processed, key)
			return false, nil
		}

		periodDuration := t - vault.PeriodStart
		if periodDuration < 0 {
			periodDuration = now - vault.PeriodStart
		}

		canPay, err := k.CanPayoutDuration(sdkCtx, vault, periodDuration)
		if err != nil {
			sdkCtx.Logger().Error("failed to check payout ability", "vault", addr.String(), "err", err)
			processed = append(processed, key)
			return false, nil
		}

		if !canPay {
			depleted = append(depleted, vault)
			processed = append(processed, key)
			return false, nil
		}

		if err := k.PerformVaultInterestTransfer(sdkCtx, vault); err != nil {
			sdkCtx.Logger().Error("failed to reconcile interest", "vault", addr.String(), "err", err)
			processed = append(processed, key)
			return false, nil
		}

		reconciled = append(reconciled, addr)
		processed = append(processed, key)
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("walk failed: %w", err)
	}

	for _, key := range processed {
		if err := k.VaultTimeoutQueue.Remove(ctx, key); err != nil {
			sdkCtx.Logger().Error("failed to remove processed timeout", "err", err)
		}
	}

	k.resetVaultInterestPeriods(ctx, reconciled, now)
	k.handleDepletedVaults(ctx, depleted)
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

func (k *Keeper) handleReconciledVaults(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	now := sdkCtx.BlockTime().Unix()

	var processed []collections.Pair[uint64, sdk.AccAddress]
	var vaults []*types.VaultAccount

	err := k.WalkDueStarts(ctx, now, func(t uint64, addr sdk.AccAddress) (bool, error) {
		v, ok := k.tryGetVault(sdkCtx, addr)
		if ok {
			vaults = append(vaults, v)
		}
		processed = append(processed, collections.Join(t, addr))
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("walk failed: %w", err)
	}

	for _, key := range processed {
		_ = k.VaultStartQueue.Remove(ctx, key)
	}

	payable, depleted := k.partitionVaults(sdkCtx, vaults)
	k.handlePayableVaults(ctx, payable)
	k.handleDepletedVaults(ctx, depleted)
	return nil
}

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

// handlePayableVaults handles the logic for payable vaults that have been reonciled.
func (k *Keeper) handlePayableVaults(ctx context.Context, payouts []*types.VaultAccount) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	now := sdkCtx.BlockTime().Unix()

	for _, v := range payouts {
		v.PeriodTimeout = now + AutoReconcileTimeout

		if err := k.SetVault(sdkCtx, v); err != nil {
			sdkCtx.Logger().Error("failed to persist vault", "vault", v.GetAddress().String(), "err", err)
			continue
		}
		if err := k.EnqueueVaultTimeout(ctx, v.PeriodTimeout, v.GetAddress()); err != nil {
			sdkCtx.Logger().Error("failed to enqueue timeout", "vault", v.GetAddress().String(), "err", err)
		}
	}
}

// handleDepletedVaults handles the logic for depleted vaults that have been reconciled.
func (k *Keeper) handleDepletedVaults(ctx context.Context, failedPayouts []*types.VaultAccount) {
	for _, record := range failedPayouts {
		k.UpdateInterestRates(ctx, record, types.ZeroInterestRate, record.DesiredInterestRate)
	}
}

// resetVaultInterestPeriods sets a new PeriodStart for the given vaults in VaultInterestDetails.
func (k *Keeper) resetVaultInterestPeriods(ctx context.Context, vaultAddrs []sdk.AccAddress, periodStart int64) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	for _, addr := range vaultAddrs {
		expire := periodStart + AutoReconcileTimeout
		if err := k.EnqueueVaultTimeout(ctx, expire, addr); err != nil {
			sdkCtx.Logger().Error("failed to enqueue vault timeout", "vault", addr.String(), "err", err)
		}
	}
}
