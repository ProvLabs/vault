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

// ReconcileVaultInterest processes any accrued interest for a vault since its last pay period start.
// If interest is due, it transfers funds from the vault to the marker module and resets the pay period.
func (k *Keeper) ReconcileVaultInterest(ctx sdk.Context, vault *types.VaultAccount) error {
	currentBlockTime := ctx.BlockTime().Unix()

	interestDetails, err := k.VaultInterestDetails.Get(ctx, vault.GetAddress())
	if err != nil {
		if errors.Is(err, collections.ErrNotFound) {
			// TODO: Not sure if we should handle this here, perhaps just return nil?
			// Starting of the initial period should be done by management process? Keep for now.
			return k.VaultInterestDetails.Set(ctx, vault.GetAddress(), types.VaultInterestDetails{
				PeriodStart: currentBlockTime,
			})
		}
		return fmt.Errorf("failed to get vault interest details: %w", err)
	}

	if currentBlockTime <= interestDetails.PeriodStart {
		return nil
	}

	duration := currentBlockTime - interestDetails.PeriodStart

	reserves := k.BankKeeper.GetBalance(ctx, vault.GetAddress(), vault.UnderlyingAssets[0])
	principal := k.BankKeeper.GetBalance(ctx, markertypes.MustGetMarkerAddress(vault.ShareDenom), vault.UnderlyingAssets[0])

	interestEarned, err := interest.CalculateInterestEarned(principal, vault.InterestRate, duration)
	if err != nil {
		return fmt.Errorf("failed to calculate interest: %w", err)
	}

	if interestEarned.IsPositive() {
		if reserves.Amount.LT(interestEarned) {
			return fmt.Errorf("insufficient reserves to pay interest")
		}

		if err := k.BankKeeper.SendCoins(markertypes.WithBypass(ctx),
			vault.GetAddress(),
			markertypes.MustGetMarkerAddress(vault.ShareDenom),
			sdk.NewCoins(sdk.NewCoin(vault.UnderlyingAssets[0], interestEarned)),
		); err != nil {
			return fmt.Errorf("failed to pay interest: %w", err)
		}
	} else if interestEarned.IsNegative() {
		owed := interestEarned.Abs()
		from := markertypes.MustGetMarkerAddress(vault.ShareDenom)
		to := vault.GetAddress()

		balance := k.BankKeeper.GetBalance(ctx, from, vault.UnderlyingAssets[0])
		if balance.Amount.LT(owed) {
			return fmt.Errorf("insufficient marker balance to reclaim negative interest")
		}
		if err := k.BankKeeper.SendCoins(markertypes.WithBypass(ctx),
			from,
			to,
			sdk.NewCoins(sdk.NewCoin(vault.UnderlyingAssets[0], owed)),
		); err != nil {
			return fmt.Errorf("failed to reclaim negative interest: %w", err)
		}
	}

	principalAfter := k.BankKeeper.GetBalance(ctx, markertypes.MustGetMarkerAddress(vault.ShareDenom), vault.UnderlyingAssets[0])

	k.emitEvent(ctx, types.NewEventVaultReconcile(
		vault.GetAddress().String(),
		principal,
		principalAfter,
		vault.InterestRate,
		duration,
		interestEarned,
	))

	return k.VaultInterestDetails.Set(ctx, vault.GetAddress(), types.VaultInterestDetails{
		PeriodStart: currentBlockTime,
	})
}

// EstimateVaultTotalAssets returns the estimated total value of the vault's assets,
// including any interest that would have accrued since the last interest period start.
// This is used to simulate the value of earned interest as if a reconciliation had occurred
// at the current block time, without modifying state or transferring funds.
//
// If no interest rate is set or if the vault has not yet begun accruing interest,
// the original total asset amount is returned unmodified.
func (k Keeper) EstimateVaultTotalAssets(ctx sdk.Context, vault *types.VaultAccount, totalAssets sdk.Coin) (sdkmath.Int, error) {
	estimated := totalAssets.Amount

	if vault.InterestRate == "" {
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

	interestEarned, err := interest.CalculateInterestEarned(totalAssets, vault.InterestRate, duration)
	if err != nil {
		return sdkmath.Int{}, fmt.Errorf("error calculating interest: %w", err)
	}

	return estimated.Add(interestEarned), nil
}

func (k *Keeper) HandleVaultInterestTimeouts(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	blockTime := sdkCtx.BlockTime().Unix()

	var toDelete []sdk.AccAddress

	err := k.VaultInterestDetails.Walk(ctx, nil, func(vaultAddr sdk.AccAddress, details types.VaultInterestDetails) (bool, error) {
		if details.ExpireTime > blockTime {
			return false, nil
		}

		vault, err := k.GetVault(sdkCtx, vaultAddr)
		if err != nil || vault == nil {
			toDelete = append(toDelete, vaultAddr)
			return false, nil
		}

		if err := k.ReconcileVaultInterest(sdkCtx, vault); err != nil {
			sdkCtx.Logger().Error("failed to reconcile interest", "vault", vaultAddr.String(), "err", err)
		}

		return false, nil
	})
	if err != nil {
		return fmt.Errorf("walk failed: %w", err)
	}

	for _, addr := range toDelete {
		if err := k.VaultInterestDetails.Remove(ctx, addr); err != nil {
			sdkCtx.Logger().Error("failed to remove VaultInterestDetails for missing vault", "vault", addr.String(), "err", err)
		}
	}

	return nil
}

func (k *Keeper) HandleReconciledVaults(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	blockTime := sdkCtx.BlockTime().Unix()

	reconciled, err := k.GetReconciledVaults(sdkCtx, blockTime)
	if err != nil {
		return fmt.Errorf("failed to get reconciled vaults: %w", err)
	}

	payouts, failedPayouts := k.partitionReconciledVaults(sdkCtx, reconciled)
	k.handlePayouts(ctx, payouts)
	k.handleFailedPayouts(ctx, failedPayouts)

	return nil
}

func (k *Keeper) partitionReconciledVaults(sdkCtx sdk.Context, vaults []ReconciledVault) ([]ReconciledVault, []ReconciledVault) {
	var canPayout, cannotPayout []ReconciledVault
	for _, record := range vaults {
		payout, err := k.canPayout(sdkCtx, record)
		if err != nil {
			sdkCtx.Logger().Error("failed to check if vault can payout", "vault", record.Vault.GetAddress().String(), "err", err)
			continue
		}

		if payout {
			canPayout = append(canPayout, record)
		} else {
			cannotPayout = append(cannotPayout, record)
		}
	}
	return canPayout, cannotPayout
}

func (k *Keeper) handlePayouts(ctx context.Context, payouts []ReconciledVault) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	blockTime := sdkCtx.BlockTime().Unix()

	for _, record := range payouts {
		record.InterestDetails.ExpireTime = blockTime + interest.SecondsPerDay
		if err := k.VaultInterestDetails.Set(ctx, record.Vault.GetAddress(), *record.InterestDetails); err != nil {
			sdkCtx.Logger().Error("failed to set VaultInterestDetails for vault", "vault", record.Vault.GetAddress().String(), "err", err)
		}
	}
}

func (k *Keeper) handleFailedPayouts(ctx context.Context, failedPayouts []ReconciledVault) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	for _, record := range failedPayouts {
		record.Vault.InterestRate = "0"
		k.AuthKeeper.SetAccount(ctx, record.Vault)

		if err := k.VaultInterestDetails.Remove(ctx, record.Vault.GetAddress()); err != nil {
			sdkCtx.Logger().Error("failed to remove VaultInterestDetails for vault", "vault", record.Vault.GetAddress().String(), "err", err)
		}
	}
}

func (k *Keeper) canPayout(ctx context.Context, record ReconciledVault) (bool, error) {
	markerAddr, err := markertypes.MarkerAddress(record.Vault.ShareDenom)
	if err != nil {
		return false, fmt.Errorf("failed to get marker address: %w", err)
	}
	principal := k.BankKeeper.GetBalance(ctx, markerAddr, record.Vault.UnderlyingAssets[0])
	reserves := k.BankKeeper.GetBalance(ctx, record.Vault.GetAddress(), record.Vault.UnderlyingAssets[0])

	periods, _, err := interest.CalculatePeriods(reserves, principal, record.Vault.InterestRate, interest.SecondsPerDay, interest.CalculatePeriodsLimit)
	if err != nil {
		return false, fmt.Errorf("failed to calculate periods: %w", err)
	}

	return periods > 0, nil
}

// ReconciledVault is a helper struct to combine a vault and its interest details.
type ReconciledVault struct {
	Vault           *types.VaultAccount
	InterestDetails *types.VaultInterestDetails
}

// GetReconciledVaults retrieves all vault records where the interest period
// started at the given startTime, indicating they are due for an update.
func (k *Keeper) GetReconciledVaults(ctx context.Context, startTime int64) ([]ReconciledVault, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	var results []ReconciledVault

	err := k.VaultInterestDetails.Walk(sdkCtx, nil, func(vaultAddr sdk.AccAddress, interestDetails types.VaultInterestDetails) (stop bool, err error) {
		if interestDetails.PeriodStart == startTime {
			vault, err := k.GetVault(sdkCtx, vaultAddr)
			if err != nil {
				// TODO Check this, and should it be an error.
				return false, nil
			}
			if vault == nil {
				// TODO Check this, and should it be an error.
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
