package keeper

import (
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

// reconcileVault updates interest accounting and collects AUM fees for a vault if a new period has started.
//
// If this is the first time the vault accrues interest, it triggers the start of a new period
// and publishes the initial NAV for the share denom in terms of the underlying asset.
// If the current block time is after the relevant PeriodStart, it applies the interest and/or fee transfers.
// This function will do nothing if the vault is paused.
//
// This should be called before any transaction that changes vault principal/reserves or depends on the
// current interest state.
//
// The reconciliation is performed atomically using a cache context. If any part of the process fails,
// all state changes (interest transfers, fee transfers, and queue updates) are rolled back.
// To ensure a clean state, the operation works on a cloned copy of the vault and only updates
// the original object upon successful completion.
//
// The process follows a strict mathematical ordering:
// 1. Interest is processed first to ensure the Total Vault Value (TVV) is updated.
// 2. AUM fees are then calculated and collected based on the post-interest TVV.
// 3. Timeouts are rescheduled and the updated NAV is published.
func (k Keeper) reconcileVault(ctx sdk.Context, vault *types.VaultAccount) error {
	if vault == nil {
		return fmt.Errorf("vault account cannot be nil")
	}
	if vault.Paused {
		return nil
	}

	cacheCtx, write := ctx.CacheContext()
	v := vault.Clone()
	currentBlockTime := cacheCtx.BlockTime().Unix()

	if v.PeriodStart != 0 && currentBlockTime > v.PeriodStart {
		if err := k.PerformVaultInterestTransfer(cacheCtx, v); err != nil {
			return fmt.Errorf("perform vault interest transfer: %w", err)
		}
	}

	if v.FeePeriodStart != 0 {
		if err := k.PerformVaultFeeTransfer(cacheCtx, v); err != nil {
			return fmt.Errorf("perform vault fee transfer: %w", err)
		}
		if err := k.SafeEnqueueFeeTimeout(cacheCtx, v); err != nil {
			return fmt.Errorf("enqueue fee timeout after transfer: %w", err)
		}
	}

	if v.PeriodStart != 0 {
		if currentBlockTime > v.PeriodStart {
			if err := k.publishShareNav(cacheCtx, v); err != nil {
				return fmt.Errorf("publish share nav: %w", err)
			}
		}
	} else {
		if v.FeePeriodStart == 0 {
			if err := k.SafeEnqueueFeeTimeout(cacheCtx, v); err != nil {
				return fmt.Errorf("enqueue fee timeout: %w", err)
			}
		}
	}

	if err := k.SafeAddVerification(cacheCtx, v); err != nil {
		return fmt.Errorf("reconcileVault: failed to SafeAddVerification for vault %s: %w", v.GetAddress().String(), err)
	}

	write()
	*vault = *v
	return nil
}

// setShareDenomNAV publishes the Net Asset Value (NAV) for a vault’s share denom
// in terms of the underlying asset.
//
// The NAV price is set to the vault’s total value in underlying units (TVV),
// and the NAV volume is set to the total number of shares. If the total share
// amount cannot be represented as a uint64, this method returns an error and
// does not publish a NAV.
func (k Keeper) setShareDenomNAV(ctx sdk.Context, vault *types.VaultAccount, vaultMarker markertypes.MarkerAccountI, tvv sdkmath.Int) error {
	if !vault.TotalShares.Amount.IsUint64() {
		return fmt.Errorf(
			"vault total shares overflows uint64: %s",
			vault.TotalShares.Amount.String(),
		)
	}

	return k.MarkerKeeper.SetNetAssetValue(
		ctx,
		vaultMarker,
		markertypes.NetAssetValue{
			Price:  sdk.NewCoin(vault.UnderlyingAsset, tvv),
			Volume: vault.TotalShares.Amount.Uint64(),
		},
		types.ModuleName,
	)
}

// publishShareNav records the Net Asset Value (NAV) for the vault's share denom
// in terms of its underlying asset. It fetches the vault’s principal marker,
// computes the total value of vault assets in underlying units (TVV), and
// attempts to set the NAV as (Price = TVV in underlying, Volume = total shares).
// If no shares exist or the TVV is non-positive, no NAV is published.
//
// If NAV publication fails, the error is logged and the operation continues
// without failing the overall vault reconciliation process.
func (k Keeper) publishShareNav(ctx sdk.Context, vault *types.VaultAccount) error {
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

	if err := k.setShareDenomNAV(ctx, vault, vaultMarker, tvv); err != nil {
		ctx.Logger().Error("failed to publish share NAV", "err", err)
	}
	return nil
}

// PerformVaultInterestTransfer applies accrued interest between the vault and the marker account
// if the current block time is beyond PeriodStart.
//
// Interest is settled exclusively in the vault's defined UnderlyingAsset.
// Interest is calculated based on the **Gross TVV** (the literal sum of all assets in the marker).
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
func (k Keeper) PerformVaultInterestTransfer(ctx sdk.Context, vault *types.VaultAccount) error {
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
		return fmt.Errorf("failed to get TVV: %w", err)
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
		return fmt.Errorf("failed to get TVV after reconciliation: %w", err)
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

// PerformVaultFeeTransfer computes and collects the 15 bps technology fee (0.15% annual)
// from the vault's principal marker account.
//
// The fee is calculated based on the **Gross TVV** (the literal sum of all assets in the marker)
// and collected in the vault's configured PaymentDenom.
//
// This method implements a "collect-what-is-available" strategy: it attempts to transfer
// the total outstanding fee (accrued + previously unpaid), but caps the collection at
// the principal marker's current PaymentDenom balance. Any uncollected remainder is
// recorded in OutstandingAumFee to be retried during the next reconciliation.
//
// An EventVaultFeeCollected is emitted upon success.
func (k Keeper) PerformVaultFeeTransfer(ctx sdk.Context, vault *types.VaultAccount) error {
	currentBlockTime := ctx.BlockTime().Unix()
	if currentBlockTime <= vault.FeePeriodStart {
		return nil
	}

	tvv, err := k.GetTVVInUnderlyingAsset(ctx, *vault)
	if err != nil {
		return fmt.Errorf("failed to get TVV: %w", err)
	}

	newFeePayment, err := k.CalculateAccruedAUMFeePayment(ctx, *vault, tvv)
	if err != nil {
		return fmt.Errorf("failed to calculate accrued AUM fee payment: %w", err)
	}

	totalOutstanding := vault.OutstandingAumFee.Add(newFeePayment)
	if totalOutstanding.IsZero() {
		vault.FeePeriodStart = currentBlockTime
		return nil
	}

	provlabsAddr, err := k.GetAUMFeeAddress(ctx)
	if err != nil {
		return fmt.Errorf("failed to get AUM fee address: %w", err)
	}

	principalAddress := vault.PrincipalMarkerAddress()
	balance := k.BankKeeper.GetBalance(ctx, principalAddress, vault.PaymentDenom)

	toCollect := totalOutstanding
	if balance.Amount.LT(totalOutstanding.Amount) {
		toCollect = balance
	}

	if !toCollect.IsZero() {
		if err := k.BankKeeper.SendCoins(
			markertypes.WithTransferAgents(ctx, vault.GetAddress()),
			principalAddress,
			provlabsAddr,
			sdk.NewCoins(toCollect),
		); err != nil {
			return fmt.Errorf("failed to transfer AUM fee: %w", err)
		}
	}

	periodDuration := currentBlockTime - vault.FeePeriodStart
	vault.OutstandingAumFee = totalOutstanding.Sub(toCollect)
	vault.FeePeriodStart = currentBlockTime

	k.emitEvent(ctx, types.NewEventVaultFeeCollected(
		vault.GetAddress().String(),
		toCollect,
		totalOutstanding,
		sdk.NewCoin(vault.UnderlyingAsset, tvv),
		vault.OutstandingAumFee,
		periodDuration,
	))

	return nil
}

// CanPayInterestDuration determines whether the vault can fulfill the projected
// interest payment/refund over the given duration based on current reserves and principal TVV.
//
// Interest is checked against vault reserves (positive interest) or principal
// marker underlying balance (negative interest).
//
// It returns true only if interest checks pass.
func (k Keeper) CanPayInterestDuration(ctx sdk.Context, vault *types.VaultAccount, duration int64) (bool, error) {
	if duration <= 0 {
		return true, nil
	}

	underlyingDenom := vault.UnderlyingAsset
	vaultAddr := vault.GetAddress()
	principalAddr := vault.PrincipalMarkerAddress()

	principalTvv, err := k.GetTVVInUnderlyingAsset(ctx, *vault)
	if err != nil {
		return false, fmt.Errorf("failed to get TVV: %w", err)
	}
	if principalTvv.IsZero() {
		return true, nil
	}

	principalCoin := sdk.NewCoin(underlyingDenom, principalTvv)
	interestEarned, err := interest.CalculateInterestEarned(principalCoin, vault.CurrentInterestRate, duration)
	if err != nil {
		return false, fmt.Errorf("failed to calculate interest: %w", err)
	}

	if !interestEarned.IsZero() {
		if interestEarned.IsPositive() {
			reserves := k.BankKeeper.GetBalance(ctx, vaultAddr, underlyingDenom)
			if reserves.Amount.LT(interestEarned) {
				return false, nil
			}
		} else if interestEarned.IsNegative() {
			principalUnderlying := k.BankKeeper.GetBalance(ctx, principalAddr, underlyingDenom)
			owed := interestEarned.Abs()
			if principalUnderlying.Amount.LT(owed) {
				return false, nil
			}
		}
	}

	return true, nil
}

// UpdateInterestRates sets the vault's current and desired interest rates and emits
// an EventVaultInterestChange. The modified account is persisted via the auth keeper.
func (k Keeper) UpdateInterestRates(ctx sdk.Context, vault *types.VaultAccount, currentRate, desiredRate string) error {
	event := types.NewEventVaultInterestChange(vault.GetAddress().String(), currentRate, desiredRate)
	vault.CurrentInterestRate = currentRate
	vault.DesiredInterestRate = desiredRate
	if err := k.SetVaultAccount(ctx, vault); err != nil {
		return fmt.Errorf("failed to update vault account: %w", err)
	}
	k.emitEvent(ctx, event)
	return nil
}

// CalculateAccruedInterest calculates the interest that would have accrued for the vault
// from its PeriodStart to the current block time, based on the provided principal.
// It returns the interest amount (which can be negative) and does not mutate state.
func (k Keeper) CalculateAccruedInterest(ctx sdk.Context, vault types.VaultAccount, principal sdk.Coin) (sdkmath.Int, error) {
	if vault.CurrentInterestRate == "" || vault.PeriodStart == 0 {
		return sdkmath.ZeroInt(), nil
	}
	duration := ctx.BlockTime().Unix() - vault.PeriodStart
	if duration <= 0 {
		return sdkmath.ZeroInt(), nil
	}
	return interest.CalculateInterestEarned(principal, vault.CurrentInterestRate, duration)
}

// CalculateAccruedAUMFee calculates the AUM fees that would have accrued for the vault
// from its FeePeriodStart to the current block time, based on the provided total assets.
// It returns the fee amount in the underlying asset and does not mutate state.
func (k Keeper) CalculateAccruedAUMFee(ctx sdk.Context, vault types.VaultAccount, totalAssets sdkmath.Int) (sdkmath.Int, error) {
	if vault.FeePeriodStart == 0 {
		return sdkmath.ZeroInt(), nil
	}
	duration := ctx.BlockTime().Unix() - vault.FeePeriodStart
	if duration <= 0 {
		return sdkmath.ZeroInt(), nil
	}
	return interest.CalculateAUMFee(totalAssets, vault.AumFeeBips, duration)
}

// CalculateAccruedAUMFeePayment calculates the AUM fees that would have accrued for the vault
// from its FeePeriodStart to the current block time, converted to the vault's PaymentDenom.
func (k Keeper) CalculateAccruedAUMFeePayment(ctx sdk.Context, vault types.VaultAccount, totalAssets sdkmath.Int) (sdk.Coin, error) {
	feeUnderlying, err := k.CalculateAccruedAUMFee(ctx, vault, totalAssets)
	if err != nil {
		return sdk.Coin{}, fmt.Errorf("failed to calculate accrued AUM fee: %w", err)
	}
	if feeUnderlying.IsZero() {
		return sdk.NewCoin(vault.PaymentDenom, sdkmath.ZeroInt()), nil
	}
	feePayment, err := k.FromUnderlyingAssetAmount(ctx, vault, feeUnderlying, vault.PaymentDenom)
	if err != nil {
		return sdk.Coin{}, fmt.Errorf("failed to convert accrued fee to payment denom: %w", err)
	}
	return sdk.NewCoin(vault.PaymentDenom, feePayment), nil
}

// CalculateOutstandingFeeUnderlying converts the vault's outstanding AUM fees into
// the equivalent amount of the underlying asset.
func (k Keeper) CalculateOutstandingFeeUnderlying(ctx sdk.Context, vault types.VaultAccount) (sdkmath.Int, error) {
	if vault.OutstandingAumFee.IsZero() {
		return sdkmath.ZeroInt(), nil
	}
	return k.ToUnderlyingAssetAmount(ctx, vault, vault.OutstandingAumFee)
}

// CalculateVaultTotalAssets returns the total value of the vault's assets, including the interest
// that would have accrued from PeriodStart to the current block time, and subtracting the
// AUM fees accrued since FeePeriodStart, without mutating state.
//
// VALUATION LOGIC (Net TVV): This method subtracts the **OutstandingAumFee** from the gross total
// to ensure share pricing (NAV) reflects the actual equity owned by shareholders, excluding
// vault liabilities.
//
// If no rate is set or accrual has not started, it returns the provided principal unchanged.
func (k Keeper) CalculateVaultTotalAssets(ctx sdk.Context, vault *types.VaultAccount, principal sdk.Coin) (sdkmath.Int, error) {
	interestEarned, err := k.CalculateAccruedInterest(ctx, *vault, principal)
	if err != nil {
		return sdkmath.Int{}, fmt.Errorf("error calculating interest: %w", err)
	}
	estimated := principal.Amount.Add(interestEarned)

	feeAccrued, err := k.CalculateAccruedAUMFee(ctx, *vault, estimated)
	if err != nil {
		return sdkmath.Int{}, fmt.Errorf("error calculating AUM fee: %w", err)
	}
	estimated = estimated.Sub(feeAccrued)

	outstandingUnderlying, err := k.CalculateOutstandingFeeUnderlying(ctx, *vault)
	if err != nil {
		return sdkmath.Int{}, fmt.Errorf("error converting outstanding fee: %w", err)
	}
	estimated = estimated.Sub(outstandingUnderlying)

	if estimated.IsNegative() {
		estimated = sdkmath.ZeroInt()
	}

	return estimated, nil
}

// handleVaultInterestTimeouts checks vaults with expired interest periods and reconciles or disables them.
// It uses a safe "collect-then-mutate" pattern to comply with the SDK iterator contract.
func (k Keeper) handleVaultInterestTimeouts(ctx sdk.Context) error {
	now := ctx.BlockTime().Unix()

	var keysToProcess []collections.Pair[uint64, sdk.AccAddress]
	var depleted []*types.VaultAccount

	err := k.PayoutTimeoutQueue.WalkDue(ctx, now, func(timeout uint64, addr sdk.AccAddress) (bool, error) {
		key := collections.Join(timeout, addr)
		vault, ok := k.tryGetVault(ctx, addr)
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

		vault, ok := k.tryGetVault(ctx, addr)
		if !ok {
			// clean up queue if vault no longer exists
			_ = k.PayoutTimeoutQueue.Dequeue(ctx, int64(timeout), addr)
			continue
		}

		periodDuration := int64(timeout) - vault.PeriodStart
		if periodDuration < 0 {
			periodDuration = now - vault.PeriodStart
		}

		canPay, err := k.CanPayInterestDuration(ctx, vault, periodDuration)
		if err != nil {
			ctx.Logger().Error("failed to check payout ability, rescheduling", "vault", addr.String(), "err", err)
			k.rescheduleInterestTimeout(ctx, vault, int64(timeout))
			continue
		}

		if !canPay {
			depleted = append(depleted, vault)
			if err := k.PayoutTimeoutQueue.Dequeue(ctx, int64(timeout), addr); err != nil {
				ctx.Logger().Error("CRITICAL: failed to dequeue interest timeout, skipping", "vault", addr.String(), "err", err)
			}
			continue
		}

		if err := k.atomicallyReconcileInterest(ctx, vault, int64(timeout)); err != nil {
			ctx.Logger().Error("failed to reconcile interest atomically, rescheduling", "vault", addr.String(), "err", err)
			k.rescheduleInterestTimeout(ctx, vault, int64(timeout))
			continue
		}
	}

	k.handleDepletedVaults(ctx, depleted)
	return nil
}

// atomicallyReconcileInterest performs the interest transfer, dequeues the current
// timeout, and enqueues the next period timeout within a single atomic cache context.
// If any step fails, the entire operation is reverted.
func (k Keeper) atomicallyReconcileInterest(ctx sdk.Context, vault *types.VaultAccount, timeout int64) error {
	cacheCtx, write := ctx.CacheContext()
	v := vault.Clone()

	if err := k.PerformVaultInterestTransfer(cacheCtx, v); err != nil {
		return fmt.Errorf("failed to perform vault interest transfer: %w", err)
	}

	if err := k.SafeEnqueueTimeout(cacheCtx, v); err != nil {
		return fmt.Errorf("failed to reset interest period: %w", err)
	}

	write()
	*vault = *v
	return nil
}

// rescheduleInterestTimeout pushes a vault's interest timeout to the next window.
// This is used when a reconciliation attempt fails due to transient errors, preventing
// block-to-block retries.
//
// NOTE: This resets the PeriodStart to the current block time via SafeEnqueueTimeout.
func (k Keeper) rescheduleInterestTimeout(ctx sdk.Context, vault *types.VaultAccount, oldTimeout int64) {
	// Dequeue on the main context first to ensure it's removed even if the atomic part fails.
	if err := k.PayoutTimeoutQueue.Dequeue(ctx, oldTimeout, vault.GetAddress()); err != nil {
		ctx.Logger().Error("failed to dequeue old interest timeout", "vault", vault.GetAddress().String(), "err", err)
		return
	}

	cacheCtx, write := ctx.CacheContext()
	v := vault.Clone()
	if err := k.SafeEnqueueTimeout(cacheCtx, v); err != nil {
		ctx.Logger().Error("failed to reschedule interest timeout", "vault", vault.GetAddress().String(), "err", err)
		return
	}
	write()
	*vault = *v
}

// tryGetVault returns the vault if found, or false if the vault is missing or invalid.
// It should only be used in BeginBlocker/EndBlocker logic where failure is non-critical.
func (k Keeper) tryGetVault(ctx sdk.Context, addr sdk.AccAddress) (*types.VaultAccount, bool) {
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
func (k Keeper) handleReconciledVaults(ctx sdk.Context) error {
	var keysToProcess []sdk.AccAddress
	var vaultsToProcess []*types.VaultAccount

	err := k.PayoutVerificationSet.Walk(ctx, nil, func(addr sdk.AccAddress) (bool, error) {
		v, ok := k.tryGetVault(ctx, addr)
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
			ctx.Logger().Error("CRITICAL: failed to remove from payout verification set, skipping", "vault", addr.String(), "err", err)
			continue
		}

		v, ok := k.tryGetVault(ctx, addr)
		if ok && !v.Paused {
			vaultsToProcess = append(vaultsToProcess, v)
		}
	}

	payable, depleted := k.partitionVaults(ctx, vaultsToProcess)
	k.handlePayableVaults(ctx, payable)
	k.handleDepletedVaults(ctx, depleted)
	return nil
}

// partitionVaults splits the provided vaults into payable and depleted groups for the
// AutoReconcilePayoutDuration forecast window using CanPayInterestDuration.
func (k Keeper) partitionVaults(ctx sdk.Context, vaults []*types.VaultAccount) ([]*types.VaultAccount, []*types.VaultAccount) {
	var payable []*types.VaultAccount
	var depleted []*types.VaultAccount
	for _, v := range vaults {
		ok, err := k.CanPayInterestDuration(ctx, v, AutoReconcilePayoutDuration)
		if err != nil {
			ctx.Logger().Error("failed to check payout ability", "vault", v.GetAddress().String(), "err", err)
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
func (k Keeper) handlePayableVaults(ctx sdk.Context, payouts []*types.VaultAccount) {
	for _, v := range payouts {
		if err := k.SafeEnqueueTimeout(ctx, v); err != nil {
			ctx.Logger().Error("failed to enqueue timeout", "vault", v.GetAddress().String(), "err", err)
		}
	}
}

// handleDepletedVaults disables interest for vaults that cannot cover the forecasted payout window
// by setting the current rate to zero while preserving the desired rate.
func (k Keeper) handleDepletedVaults(ctx sdk.Context, failedPayouts []*types.VaultAccount) {
	for _, record := range failedPayouts {
		if err := k.UpdateInterestRates(ctx, record, types.ZeroInterestRate, record.DesiredInterestRate); err != nil {
			ctx.Logger().Error("failed to update interest rates", "vault", record.GetAddress().String(), "err", err)
		}
	}
}

// handleVaultFeeTimeouts checks vaults with expired fee periods and reconciles them.
// It uses a safe "collect-then-mutate" pattern to comply with the SDK iterator contract.
func (k Keeper) handleVaultFeeTimeouts(ctx sdk.Context) error {
	now := ctx.BlockTime().Unix()

	var keysToProcess []collections.Pair[uint64, sdk.AccAddress]

	err := k.FeeTimeoutQueue.WalkDue(ctx, now, func(timeout uint64, addr sdk.AccAddress) (bool, error) {
		key := collections.Join(timeout, addr)
		vault, ok := k.tryGetVault(ctx, addr)
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

		vault, ok := k.tryGetVault(ctx, addr)
		if !ok {
			// clean up queue if vault no longer exists
			_ = k.FeeTimeoutQueue.Dequeue(ctx, int64(timeout), addr)
			continue
		}

		if err := k.atomicallyReconcileFee(ctx, vault, int64(timeout)); err != nil {
			ctx.Logger().Error("failed to collect AUM fee atomically, rescheduling", "vault", addr.String(), "err", err)
			k.rescheduleFeeTimeout(ctx, vault, int64(timeout))
			continue
		}
	}

	return nil
}

// atomicallyReconcileFee performs the AUM fee collection, dequeues the current
// fee timeout, and enqueues the next fee period timeout within a single atomic
// cache context. If any step fails, the entire operation is reverted.
func (k Keeper) atomicallyReconcileFee(ctx sdk.Context, vault *types.VaultAccount, timeout int64) error {
	cacheCtx, write := ctx.CacheContext()
	v := vault.Clone()

	if err := k.PerformVaultFeeTransfer(cacheCtx, v); err != nil {
		return fmt.Errorf("failed to perform vault fee transfer: %w", err)
	}

	if err := k.SafeEnqueueFeeTimeout(cacheCtx, v); err != nil {
		return fmt.Errorf("failed to reset fee period: %w", err)
	}

	write()
	*vault = *v
	return nil
}

// rescheduleFeeTimeout pushes a vault's fee timeout to the next window.
// This is used when a fee collection attempt fails due to transient errors
// (e.g. missing NAVs), preventing block-to-block retries.
//
// NOTE: This resets the FeePeriodStart to the current block time via SafeEnqueueFeeTimeout.
func (k Keeper) rescheduleFeeTimeout(ctx sdk.Context, vault *types.VaultAccount, oldTimeout int64) {
	// Dequeue on the main context first to ensure it's removed even if the atomic part fails.
	if err := k.FeeTimeoutQueue.Dequeue(ctx, oldTimeout, vault.GetAddress()); err != nil {
		ctx.Logger().Error("failed to dequeue old fee timeout", "vault", vault.GetAddress().String(), "err", err)
		return
	}

	cacheCtx, write := ctx.CacheContext()
	v := vault.Clone()
	if err := k.SafeEnqueueFeeTimeout(cacheCtx, v); err != nil {
		ctx.Logger().Error("failed to reschedule fee timeout", "vault", vault.GetAddress().String(), "err", err)
		return
	}
	write()
	*vault = *v
}
