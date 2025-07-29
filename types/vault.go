package types

import (
	fmt "fmt"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	proto "github.com/cosmos/gogoproto/proto"
)

var (
	_ sdk.AccountI             = (*VaultAccount)(nil)
	_ authtypes.GenesisAccount = (*VaultAccount)(nil)
)

// VaultAccountI defines the interface for a Vault account.
type VaultAccountI interface {
	// proto.Message ensures the vault can be marshaled using protobuf.
	proto.Message

	// sdk.AccountI provides standard Cosmos SDK account behavior.
	sdk.AccountI

	// Clone returns a deep copy of the vault.
	Clone() *VaultAccount

	// Validate verifies the vault’s integrity and internal fields.
	Validate() error

	// GetAdmin returns the vault administrator address as a string.
	GetAdmin() string

	// GetShareDenom returns the denom used for shares in the vault.
	GetShareDenom() string

	// GetUnderlyingAssets returns the list of assets backing the vault.
	GetUnderlyingAssets() []string
}

// NewVaultAccount creates a new vault.
func NewVaultAccount(baseAcc *authtypes.BaseAccount, admin string, shareDenom string, underlyingAssets []string) *VaultAccount {
	return &VaultAccount{
		BaseAccount:      baseAcc,
		Admin:            admin,
		ShareDenom:       shareDenom,
		UnderlyingAssets: underlyingAssets,
	}
}

// Clone makes a MarkerAccount instance copy
func (va VaultAccount) Clone() *VaultAccount {
	return proto.Clone(&va).(*VaultAccount)
}

// Validate performs basic validation on the vault fields.
func (va VaultAccount) Validate() error {
	if _, err := sdk.AccAddressFromBech32(va.Admin); err != nil {
		return fmt.Errorf("invalid admin address: %w", err)
	}
	if err := sdk.ValidateDenom(va.ShareDenom); err != nil {
		return fmt.Errorf("invalid share denom: %w", err)
	}

	if len(va.UnderlyingAssets) < 1 {
		return fmt.Errorf("at least one underlying asset is required")
	}

	for _, denom := range va.UnderlyingAssets {
		if err := sdk.ValidateDenom(denom); err != nil {
			return fmt.Errorf("invalid underlying asset denom: %s", denom)
		}
	}

	if len(va.CurrentInterestRate) > 0 {
		_, err := sdkmath.LegacyNewDecFromStr(va.CurrentInterestRate)
		if err != nil {
			return fmt.Errorf("invalid current interest rate: %s", va.CurrentInterestRate)
		}
	}

	if len(va.DesiredInterestRate) > 0 {
		_, err := sdkmath.LegacyNewDecFromStr(va.DesiredInterestRate)
		if err != nil {
			return fmt.Errorf("invalid desired interest rate: %s", va.DesiredInterestRate)
		}
	}

	return nil
}

// ValidateUnderlyingAssets checks if the given asset's denomination is supported by the vault.
func (va VaultAccount) ValidateUnderlyingAssets(asset sdk.Coin) error {
	for _, denom := range va.UnderlyingAssets {
		if asset.Denom == denom {
			return nil
		}
	}
	return fmt.Errorf("%s asset denom not supported for vault, expected one of %v", asset.Denom, va.UnderlyingAssets)
}
