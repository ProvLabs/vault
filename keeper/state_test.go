package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/provlabs/vault/types"
	"github.com/provlabs/vault/utils"
	"github.com/provlabs/vault/utils/mocks"
)

func TestSetVaultLookup_ErrorsAndSuccess(t *testing.T) {
	ctx, k := mocks.NewVaultKeeper(t)

	err := k.SetVaultLookup(ctx, nil)
	require.Error(t, err, "expected error when vault is nil")

	vBad := &types.VaultAccount{BaseAccount: &authtypes.BaseAccount{Address: "not-bech32"}}
	err = k.SetVaultLookup(ctx, vBad)
	require.Error(t, err, "expected error when vault address is not bech32")

	addr := utils.TestAddress().Bech32
	v := &types.VaultAccount{BaseAccount: authtypes.NewBaseAccountWithAddress(sdk.MustAccAddressFromBech32(addr))}
	err = k.SetVaultLookup(ctx, v)
	require.NoError(t, err, "expected no error when vault is valid")

	got, err := k.GetVaults(ctx)
	require.NoError(t, err, "expected no error retrieving vaults after setting")
	require.Len(t, got, 1, "expected exactly one vault stored")
	assert.True(t, got[0].Equals(sdk.MustAccAddressFromBech32(addr)), "expected stored vault address to match input")
}

func TestSetVaultAccount_ValidateError(t *testing.T) {
	_, k := mocks.NewVaultKeeper(t)

	err := k.SetVaultAccount(sdk.Context{}, &types.VaultAccount{})
	require.Error(t, err, "expected error when vault validation fails")
}
