package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/provlabs/vault/types"
	"github.com/provlabs/vault/utils"
	"github.com/provlabs/vault/utils/mocks"
)

func TestGetVaults(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)

	vaults, err := k.GetVaults(ctx)

	require.NoError(t, err, "expected no error when the vaults map is empty")
	require.Empty(t, vaults, "expected empty vaults map")

	vault1 := types.Vault{
		VaultAddress: utils.TestAddress().Bech32, // Example field
		Admin:        utils.TestAddress().Bech32, // Example field
	}
	vault2 := types.Vault{
		VaultAddress: utils.TestAddress().Bech32,
		Admin:        utils.TestAddress().Bech32,
	}

	err = k.Vaults.Set(ctx, 1, vault1)
	require.NoError(t, err, "expected no error setting the first vault")
	err = k.Vaults.Set(ctx, 2, vault2)
	require.NoError(t, err, "expected no error setting the second vault")

	vaults, err = k.GetVaults(ctx)

	require.NoError(t, err, "expected no error when vaults are present")
	require.Len(t, vaults, 2, "expected two vaults")
	require.Equal(t, vault1, vaults[1], "expected different values for the first vault")
	require.Equal(t, vault2, vaults[2], "expected different values for the second vault")
}
