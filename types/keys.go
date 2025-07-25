package types

import (
	fmt "fmt"

	"github.com/cometbft/cometbft/crypto"

	"cosmossdk.io/collections"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// ModuleName defines the module name
	ModuleName = "vault"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// GovModuleName duplicates the gov module's name to avoid a dependency with x/gov.
	// It should be synced with the gov module's name if it is ever changed.
	// See: https://github.com/cosmos/cosmos-sdk/blob/v0.52.0-beta.2/x/gov/types/keys.go#L9
	GovModuleName = "gov"
)

var (
	// ParamsKeyPrefix is the prefix to retrieve all Params
	ParamsKeyPrefix = collections.NewPrefix(0)
	// ParamsName is a human-readable name for the params collection.
	ParamsName = "params"
	// VaultsKeyPrefix is the prefix to retrieve all Vaults
	VaultsKeyPrefix = collections.NewPrefix(1)
	// VaultsName is a human-readable name for the vaults collection.
	VaultsName = "vaults"
	// VaultInterestDetailsPrefix is the prefix to retrieve all VaultInterestDetails
	VaultInterestDetailsPrefix = collections.NewPrefix(2)
	// VaultInterestDetailsName is a human-readable name for the vault interest details collection
	VaultInterestDetailsName = "vault_interest_details"
)

// GetVaultAddress returns the module account address for the given shareDenom.
func GetVaultAddress(shareDenom string) sdk.AccAddress {
	return sdk.AccAddress(crypto.AddressHash([]byte(fmt.Sprintf("%s/%s", ModuleName, shareDenom))))
}
