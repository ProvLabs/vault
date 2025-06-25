package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/provlabs/vault/types"
	"github.com/provlabs/vault/utils"
	"github.com/provlabs/vault/utils/mocks"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestGetVaults(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)

	vaults, err := k.GetVaults(ctx)

	require.NoError(t, err, "expected no error when the vaults map is empty")
	require.Empty(t, vaults, "expected empty vaults map")

	// Generate unique addresses for vault keys
	vault1Addr := utils.TestAddress().Bech32
	vault2Addr := utils.TestAddress().Bech32
	// Ensure they are different for testing distinct entries
	for vault1Addr == vault2Addr {
		vault2Addr = utils.TestAddress().Bech32
	}

	vault1 := types.Vault{
		VaultAddress: vault1Addr, // Use the generated address as the key
		Admin:        utils.TestAddress().Bech32,
	}
	vault2 := types.Vault{
		VaultAddress: vault2Addr, // Use the generated address as the key
		Admin:        utils.TestAddress().Bech32,
	}

	// Set vaults using their Bech32 addresses as keys
	err = k.Vaults.Set(ctx, sdk.MustAccAddressFromBech32(vault1.VaultAddress), vault1)
	require.NoError(t, err, "expected no error setting the first vault")
	err = k.Vaults.Set(ctx, sdk.MustAccAddressFromBech32(vault2.VaultAddress), vault2)
	require.NoError(t, err, "expected no error setting the second vault")

	vaults, err = k.GetVaults(ctx)

	require.NoError(t, err, "expected no error when vaults are present")
	require.Len(t, vaults, 2, "expected two vaults")
	// Assert using the vault addresses as keys
	require.Equal(t, vault1, vaults[vault1.VaultAddress], "expected correct value for the first vault")
	require.Equal(t, vault2, vaults[vault2.VaultAddress], "expected correct value for the second vault")
}
