package keeper

import (
	"errors"
	"fmt"

	"github.com/provlabs/vault/interest"
	"github.com/provlabs/vault/types"

	"cosmossdk.io/collections"

	sdk "github.com/cosmos/cosmos-sdk/types"

	markertypes "github.com/provenance-io/provenance/x/marker/types"
)

// ReconcileVaultInterest processes any accrued interest for a vault since its last pay period start.
// If interest is due, it transfers funds from the vault to the marker module and resets the pay period.
func (k *Keeper) ReconcileVaultInterest(ctx sdk.Context, vault *types.VaultAccount) error {
	blocktime := ctx.BlockTime()
	now := blocktime.Unix()

	interestDetails, err := k.VaultInterestDetails.Get(ctx, vault.GetAddress())
	if err != nil {
		if errors.Is(err, collections.ErrNotFound) {
			// TODO: Not sure if we should handle this here, perhaps just return nil?
			// Starting of the initial period should be done by management process? Keep for now.
			return k.VaultInterestDetails.Set(ctx, vault.GetAddress(), types.VaultInterestDetails{
				PeriodStart: now,
			})
		}
		return fmt.Errorf("failed to get vault interest details: %w", err)
	}

	if now <= interestDetails.PeriodStart {
		return nil
	}

	duration := now - interestDetails.PeriodStart

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
		PeriodStart: now,
	})
}
