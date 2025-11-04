package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	vaulttypes "github.com/provlabs/vault/types"
)

func TestVaultAccount_UnpackIntoAllInterfaces(t *testing.T) {
	reg := codectypes.NewInterfaceRegistry()
	authtypes.RegisterInterfaces(reg)
	vaulttypes.RegisterInterfaces(reg)

	cdc := codec.NewProtoCodec(reg)
	any, err := codectypes.NewAnyWithValue(&vaulttypes.VaultAccount{})
	require.NoError(t, err, "failed to pack VaultAccount into Any")

	var asVault vaulttypes.VaultAccountI
	require.NoError(t, cdc.UnpackAny(any, &asVault), "failed to unpack Any into VaultAccountI")
	require.NotNil(t, asVault, "unpacked VaultAccountI is nil")

	var asAccount sdk.AccountI
	require.NoError(t, cdc.UnpackAny(any, &asAccount), "failed to unpack Any into AccountI")
	require.NotNil(t, asAccount, "unpacked AccountI is nil")
	_, ok := asAccount.(*vaulttypes.VaultAccount)
	require.True(t, ok, "AccountI did not unwrap to *VaultAccount")

	var asGenesis authtypes.GenesisAccount
	require.NoError(t, cdc.UnpackAny(any, &asGenesis), "failed to unpack Any into GenesisAccount")
	require.NotNil(t, asGenesis, "unpacked GenesisAccount is nil")
	_, ok = asGenesis.(*vaulttypes.VaultAccount)
	require.True(t, ok, "GenesisAccount did not unwrap to *VaultAccount")
}
