package keeper

import (
	"context"
	"errors"
	"fmt"

	"github.com/provlabs/vault/interest"
	"github.com/provlabs/vault/types"

	sdk "github.com/cosmos/cosmos-sdk/types"

	markertypes "github.com/provenance-io/provenance/x/marker/types"
)

func (k *Keeper) BeginBlocker(ctx context.Context) error {
	return k.HandleVaultInterestTimeouts(ctx)
}

func (k *Keeper) EndBlocker(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	blockTime := sdkCtx.BlockTime().Unix()

	reconciled, err := k.GetReconciledVaults(sdkCtx, blockTime)
	if err != nil {
		return fmt.Errorf("failed to get reconciled vaults: %w", err)
	}

	var canPayout, cannotPayout []ReconciledVault
	for _, record := range reconciled {
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

	for _, record := range canPayout {
		record.InterestDetails.ExpireTime = blockTime + interest.SecondsPerDay
		if err := k.VaultInterestDetails.Set(ctx, record.Vault.GetAddress(), *record.InterestDetails); err != nil {
			sdkCtx.Logger().Error("failed to set VaultInterestDetails for vault", "vault", record.Vault.GetAddress().String(), "err", err)
		}
	}

	for _, record := range cannotPayout {
		// TODO Do we want to wrap this because it involves the AuthKeeper
		record.Vault.InterestRate = "0"
		k.AuthKeeper.SetAccount(ctx, record.Vault)

		if err := k.VaultInterestDetails.Remove(ctx, record.Vault.GetAddress()); err != nil {
			sdkCtx.Logger().Error("failed to remove VaultInterestDetails for vault", "vault", record.Vault.GetAddress().String(), "err", err)
		}
	}

	return nil
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

// GetVaultsForUpdate retrieves all vault records where the interest period
// started at the given startTime, indicating they are due for an update.
func (k *Keeper) GetReconciledVaults(ctx context.Context, startTime int64) ([]ReconciledVault, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	var results []ReconciledVault

	err := k.VaultInterestDetails.Walk(sdkCtx, nil, func(vaultAddr sdk.AccAddress, interestDetails types.VaultInterestDetails) (stop bool, err error) {
		if interestDetails.PeriodStart == startTime {
			vault, err := k.GetVault(sdkCtx, vaultAddr)
			if err != nil {
				return true, fmt.Errorf("failed to get vault account for %s: %w", vaultAddr.String(), err)
			}
			if vault == nil {
				return true, errors.New("vault not found for existing interest details")
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
