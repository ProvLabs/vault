package types

import (
	fmt "fmt"
	"slices"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	proto "github.com/cosmos/gogoproto/proto"
	"github.com/provlabs/vault/utils"
)

var (
	_ sdk.AccountI             = (*Vault)(nil)
	_ authtypes.GenesisAccount = (*Vault)(nil)
)

// VaultAccountI defines the interface for a Vault account.
type VaultAccountI interface {
	// proto.Message ensures the vault can be marshaled using protobuf.
	proto.Message

	// sdk.AccountI provides standard Cosmos SDK account behavior.
	sdk.AccountI

	// Clone returns a deep copy of the vault.
	Clone() *Vault

	// Validate verifies the vault’s integrity and internal fields.
	Validate() error

	// GetAdmin returns the vault administrator address as a string.
	GetAdmin() string

	// GetShareDenom returns the denom used for shares in the vault.
	GetShareDenom() string

	// GetUnderlyingAssets returns the list of assets backing the vault.
	GetUnderlyingAssets() sdk.Coins
}

// NewVault creates a new vault.
func NewVault(baseAcc *authtypes.BaseAccount, admin string, shareDenom string, underlyingAssets []string) *Vault {
	coins := utils.Map(underlyingAssets, func(denom string) sdk.Coin {
		return sdk.NewInt64Coin(denom, 0)
	})
	return &Vault{
		BaseAccount:      baseAcc,
		Admin:            admin,
		ShareDenom:       shareDenom,
		UnderlyingAssets: sdk.NewCoins(slices.Collect(coins)...),
	}
}

// Clone makes a MarkerAccount instance copy
func (va Vault) Clone() *Vault {
	return proto.Clone(&va).(*Vault)
}

// Validate performs basic validation on the vault fields.
func (va Vault) Validate() error {
	if _, err := sdk.AccAddressFromBech32(va.Admin); err != nil {
		return fmt.Errorf("invalid admin address: %w", err)
	}
	if err := sdk.ValidateDenom(va.ShareDenom); err != nil {
		return fmt.Errorf("invalid share denom: %w", err)
	}
	if !va.UnderlyingAssets.IsValid() {
		return fmt.Errorf("invalid underlying assets: %s", va.UnderlyingAssets.String())
	}
	return nil
}
