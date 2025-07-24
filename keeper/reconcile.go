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
