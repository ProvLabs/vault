package keeper

import sdk "github.com/cosmos/cosmos-sdk/types"

// Test_partitionReconciledVaults exposes this keeper's partitionReconciledVaults function for unit tests.
func (k Keeper) TestAccessor_partitionReconciledVaults(ctx sdk.Context, vaults []ReconciledVault) ([]ReconciledVault, []ReconciledVault) {
	return k.partitionReconciledVaults(ctx, vaults)
}
