package keeper

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Test_partitionReconciledVaults exposes this keeper's partitionReconciledVaults function for unit tests.
func (k Keeper) TestAccessor_partitionReconciledVaults(t *testing.T, ctx sdk.Context, vaults []ReconciledVault) ([]ReconciledVault, []ReconciledVault) {
	t.Helper()
	return k.partitionReconciledVaults(ctx, vaults)
}
