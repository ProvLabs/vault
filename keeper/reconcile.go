package keeper

import (
	"context"
	"errors"
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

	interestDetails, err := k.VaultInterestDetails.Get(ctx, vault.GetAddress())
	if err != nil {
		if errors.Is(err, collections.ErrNotFound) {
			return k.VaultInterestDetails.Set(ctx, vault.GetAddress(), types.VaultInterestDetails{
				PeriodStart: currentBlockTime,
			})
		}
		return fmt.Errorf("failed to get vault interest details: %w", err)
	}

	if currentBlockTime <= interestDetails.PeriodStart {
		return nil
	}

	if err := k.PerformVaultInterestTransfer(ctx, vault, interestDetails); err != nil {
		return err
	}

	return k.VaultInterestDetails.Set(ctx, vault.GetAddress(), types.VaultInterestDetails{
		PeriodStart: currentBlockTime,
	})
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
func (k *Keeper) PerformVaultInterestTransfer(ctx sdk.Context, vault *types.VaultAccount, interestDetails types.VaultInterestDetails) error {
	currentBlockTime := ctx.BlockTime().Unix()

	if currentBlockTime <= interestDetails.PeriodStart {
		return nil
	}

	periodDuration := currentBlockTime - interestDetails.PeriodStart
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

// SetInterestRate updates the current interest rate for a given vault. If the new rate is
// different from the currently set rate, it updates the vault account in the state and
// emits an EventVaultInterestChange event. No action is taken if the new rate is the same as the existing one.
func (k *Keeper) SetInterestRate(ctx context.Context, vault *types.VaultAccount, rate string) {
	if rate == vault.CurrentInterestRate {
		return
	}
	event := types.NewEventVaultInterestChange(vault.GetAddress().String(), vault.CurrentInterestRate, rate)
	vault.CurrentInterestRate = rate
	k.AuthKeeper.SetAccount(ctx, vault)
	k.emitEvent(sdk.UnwrapSDKContext(ctx), event)
}

func (k *Keeper) UpdateInterestRates(ctx context.Context, vault *types.VaultAccount, currentRate, desiredRate string) {
	if currentRate == vault.CurrentInterestRate && vault.DesiredInterestRate == currentRate {
		return
	}
	// event := types.NewEventVaultInterestChange(vault.GetAddress().String(), vault.CurrentInterestRate, cu)
	vault.CurrentInterestRate = currentRate
	vault.DesiredInterestRate = desiredRate
	k.AuthKeeper.SetAccount(ctx, vault)
	// k.emitEvent(sdk.UnwrapSDKContext(ctx), event)
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

	if vault.CurrentInterestRate == "" {
		return estimated, nil
	}

	interestDetails, err := k.VaultInterestDetails.Get(ctx, vault.GetAddress())
	if err != nil {
		if !errors.Is(err, collections.ErrNotFound) {
			return sdkmath.Int{}, fmt.Errorf("error getting interest details: %w", err)
		}
		return estimated, nil
	}

	duration := ctx.BlockTime().Unix() - interestDetails.PeriodStart
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
	currentBlockTime := sdkCtx.BlockTime().Unix()

	var depletedVaults []ReconciledVault
	var reconciledVaults []sdk.AccAddress
	err := k.VaultInterestDetails.Walk(ctx, nil, func(vaultAddr sdk.AccAddress, details types.VaultInterestDetails) (bool, error) {
		if details.ExpireTime > currentBlockTime {
			return false, nil
		}

		vault, ok := k.tryGetVault(sdkCtx, vaultAddr)
		if !ok {
			return false, nil
		}

		periodDuration := currentBlockTime - details.PeriodStart
		canPay, err := k.CanPayoutDuration(sdkCtx, vault, periodDuration)
		if err != nil {
			sdkCtx.Logger().Error("failed to check payout ability", "vault", vaultAddr.String(), "err", err)
			return false, nil
		}
		if !canPay {
			// PR Review TODO: Due to possible drift from endblocker to begin blocker.  The account might not be able to pay the funds.
			// Should we just take the remainder of funds or just disable current interest of vault?
			depletedVaults = append(depletedVaults, ReconciledVault{Vault: vault, InterestDetails: &details})
			return false, nil
		}

		if err := k.PerformVaultInterestTransfer(sdkCtx, vault, details); err != nil {
			sdkCtx.Logger().Error("failed to reconcile interest", "vault", vaultAddr.String(), "err", err)
		}
		reconciledVaults = append(reconciledVaults, vault.GetAddress())
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("walk failed: %w", err)
	}

	k.resetVaultInterestPeriods(ctx, reconciledVaults, currentBlockTime)
	k.handleDepletedVaults(ctx, depletedVaults)
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

// handleReconciledVaults processes vaults that have been reconciled in the current block.
// It partitions them into active (able to pay interest) and depleted (unable to pay interest) vaults.
// Active vaults have their expiration time extended, while depleted vaults have their interest rate set to zero.
func (k *Keeper) handleReconciledVaults(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	blockTime := sdkCtx.BlockTime().Unix()

	reconciled, err := k.GetReconciledVaults(sdkCtx, blockTime)
	if err != nil {
		return fmt.Errorf("failed to get reconciled vaults: %w", err)
	}

	payable, depleted := k.partitionReconciledVaults(sdkCtx, reconciled)
	k.handlePayableVaults(ctx, payable)
	k.handleDepletedVaults(ctx, depleted)

	return nil
}

// partitionReconciledVaults groups the vaults into payable and depleted vaults.
func (k *Keeper) partitionReconciledVaults(sdkCtx sdk.Context, vaults []ReconciledVault) ([]ReconciledVault, []ReconciledVault) {
	var payable, depleted []ReconciledVault
	for _, record := range vaults {
		payout, err := k.CanPayoutDuration(sdkCtx, record.Vault, AutoReconcilePayoutDuration)
		if err != nil {
			sdkCtx.Logger().Error("failed to check if vault can payout", "vault", record.Vault.GetAddress().String(), "err", err)
			continue
		}

		if payout {
			payable = append(payable, record)
		} else {
			depleted = append(depleted, record)
		}
	}
	return payable, depleted
}

// handlePayableVaults handles the logic for payable vaults that have been reonciled.
func (k *Keeper) handlePayableVaults(ctx context.Context, payouts []ReconciledVault) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	blockTime := sdkCtx.BlockTime().Unix()

	for _, record := range payouts {
		record.InterestDetails.ExpireTime = blockTime + AutoReconcileTimeout
		if err := k.VaultInterestDetails.Set(ctx, record.Vault.GetAddress(), *record.InterestDetails); err != nil {
			sdkCtx.Logger().Error("failed to set VaultInterestDetails for vault", "vault", record.Vault.GetAddress().String(), "err", err)
		}
	}
}

// handleDepletedVaults handles the logic for depleted vaults that have been reconciled.
func (k *Keeper) handleDepletedVaults(ctx context.Context, failedPayouts []ReconciledVault) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	for _, record := range failedPayouts {
		k.SetInterestRate(ctx, record.Vault, types.ZeroInterestRate)

		if err := k.VaultInterestDetails.Remove(ctx, record.Vault.GetAddress()); err != nil {
			sdkCtx.Logger().Error("failed to remove VaultInterestDetails for vault", "vault", record.Vault.GetAddress().String(), "err", err)
		}
	}
}

// resetVaultInterestPeriods sets a new PeriodStart for the given vaults in VaultInterestDetails.
func (k *Keeper) resetVaultInterestPeriods(ctx context.Context, vaultAddrs []sdk.AccAddress, periodStart int64) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	for _, addr := range vaultAddrs {
		err := k.VaultInterestDetails.Set(ctx, addr, types.VaultInterestDetails{
			PeriodStart: periodStart,
		})
		if err != nil {
			sdkCtx.Logger().Error("failed to reset VaultInterestDetails period", "vault", addr.String(), "err", err)
		}
	}
}

// ReconciledVault is a helper struct to combine a vault and its interest details.
type ReconciledVault struct {
	Vault           *types.VaultAccount
	InterestDetails *types.VaultInterestDetails
}

// GetReconciledVaults retrieves all vault records where the interest period
// started at the given currentBlockTime, indicating they have been reconciled.
func (k *Keeper) GetReconciledVaults(ctx context.Context, currentBlockTime int64) ([]ReconciledVault, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	var results []ReconciledVault

	err := k.VaultInterestDetails.Walk(sdkCtx, nil, func(vaultAddr sdk.AccAddress, interestDetails types.VaultInterestDetails) (stop bool, err error) {
		if interestDetails.PeriodStart == currentBlockTime {
			vault, ok := k.tryGetVault(sdkCtx, vaultAddr)
			if !ok {
				return false, nil
			}

			results = append(results, ReconciledVault{
				Vault:           vault,
				InterestDetails: &interestDetails,
			})
		}
		return false, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk vault interest details: %w", err)
	}

	return results, nil
}
