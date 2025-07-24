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
	return nil
}

func (k *Keeper) EndBlocker(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	blockTime := sdkCtx.BlockTime().Unix()

	records, err := k.GetVaultRecords(sdkCtx, blockTime)
	if err != nil {
		// TODO Something is wrong with store if there is an error
		return fmt.Errorf("failed to get vault records: %w", err)
	}

	for _, v := range records {
		canPayout, err := k.CanPayout(sdkCtx, v)
		if err != nil {
			// TODO Do we just want to continue?
			// TODO Possibly the marker doesn't exist
			// TODO It can't calculate interest
			// TODO Or something is not setup in vault correctly
			return fmt.Errorf("failed to check if vault can payout: %w", err)
		}

		// TODO Is this all or nothing?
		if canPayout {
			v.InterestDetails.ExpireTime = blockTime + interest.SecondsPerDay
			if err := k.VaultInterestDetails.Set(ctx, v.Vault.GetAddress(), v.InterestDetails); err != nil {
				// TODO Something is wrong with the store
				// TODO Do we want to error or continue?
				return fmt.Errorf("failed to set vault interest details: %w", err)
			}
		} else {
			// TODO We want to wrap this because it involves the AuthKeeper
			v.Vault.InterestRate = "0"
			k.AuthKeeper.SetAccount(ctx, v.Vault)

			if err := k.VaultInterestDetails.Remove(ctx, v.Vault.GetAddress()); err != nil {
				// TODO Something is wrong with the store
				// TODO Do we want to error or continue?
				return fmt.Errorf("failed to remove vault interest details: %w", err)
			}
		}
	}
	return nil
}

func (k *Keeper) CanPayout(ctx context.Context, record VaultRecord) (bool, error) {
	markerAddr, err := markertypes.MarkerAddress(record.Vault.ShareDenom)
	if err != nil {
		return false, fmt.Errorf("failed to get marker address: %w", err)
	}
	principal := k.BankKeeper.GetBalance(ctx, markerAddr, record.Vault.UnderlyingAssets[0])
	reserves := k.BankKeeper.GetBalance(ctx, record.Vault.GetAddress(), record.Vault.UnderlyingAssets[0])

	periods, _, err := interest.CalculatePeriods(reserves, principal, record.Vault.InterestRate, interest.SecondsPerDay, interest.CalculatePeriodsNoLimit)
	if err != nil {
		return false, fmt.Errorf("failed to calculate periods: %w", err)
	}

	return periods > 0, nil
}

// Vault is a helper struct to combine a vault and its interest details.
// TODO What's a better name for this?
// TODO Can VaultInterestDetails be a pointer?
type VaultRecord struct {
	Vault           *types.VaultAccount
	InterestDetails types.VaultInterestDetails
}

// GetVaultsWithInterestDetailsByStartTime retrieves all vaults and their interest details
// where the interest period started at the given startTime.
// TODO We could possibly move this into state.go
func (k *Keeper) GetVaultRecords(ctx context.Context, startTime int64) ([]VaultRecord, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	var results []VaultRecord

	err := k.VaultInterestDetails.Walk(sdkCtx, nil, func(vaultAddr sdk.AccAddress, interestDetails types.VaultInterestDetails) (stop bool, err error) {
		if interestDetails.PeriodStart == startTime {
			vault, err := k.GetVault(sdkCtx, vaultAddr)
			if err != nil {
				return true, fmt.Errorf("failed to get vault account for %s: %w", vaultAddr.String(), err)
			}
			if vault == nil {
				return true, errors.New("vault not found for existing interest details")
			}

			results = append(results, VaultRecord{
				Vault:           vault,
				InterestDetails: interestDetails,
			})
		}
		return false, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk vault interest details: %w", err)
	}

	return results, nil
}
