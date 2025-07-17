package keeper

import (
	"errors"
	"fmt"

	"cosmossdk.io/collections"
	sdk "github.com/cosmos/cosmos-sdk/types"
	markertypes "github.com/provenance-io/provenance/x/marker/types"
	"github.com/provlabs/vault/interest"
	"github.com/provlabs/vault/types"
)

// reconcileVaultInterest processes any accrued interest for a vault since its last pay period start.
// If interest is due, it transfers funds from the vault to the marker module and resets the pay period.
func (k *Keeper) reconcileVaultInterest(ctx sdk.Context, vault *types.VaultAccount) error {
	now := ctx.BlockTime().Unix()

	interestDetails, err := k.VaultInterestDetails.Get(ctx, vault.GetAddress())
	if err != nil {
		if !errors.Is(err, collections.ErrNotFound) {
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
	if duration <= 0 {
		return nil
	}

	reserves := k.BankKeeper.GetBalance(ctx, vault.GetAddress(), vault.UnderlyingAssets[0])
	principal := k.BankKeeper.GetBalance(ctx, markertypes.MustGetMarkerAddress(vault.ShareDenom), vault.UnderlyingAssets[0])

	interestEarned, err := interest.CalculateInterestEarned(principal, vault.InterestRate, duration)
	if err != nil {
		return fmt.Errorf("failed to calculate interest: %w", err)
	}
	if interestEarned.Amount.IsZero() {
		return nil
	}

	if reserves.Amount.LT(interestEarned.Amount) {
		return fmt.Errorf("insufficient reserves to pay interest")
	}

	if err := k.BankKeeper.SendCoins(markertypes.WithBypass(ctx),
		vault.GetAddress(),
		markertypes.MustGetMarkerAddress(vault.ShareDenom),
		sdk.NewCoins(interestEarned),
	); err != nil {
		return fmt.Errorf("failed to pay interest: %w", err)
	}

	return k.VaultInterestDetails.Set(ctx, vault.GetAddress(), types.VaultInterestDetails{
		PeriodStart: now,
	})
}
