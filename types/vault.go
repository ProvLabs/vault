package types

import (
	fmt "fmt"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	proto "github.com/cosmos/gogoproto/proto"
)

const (
	ZeroInterestRate = "0.0"
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
func NewVaultAccount(baseAcc *authtypes.BaseAccount, admin, shareDenom, underlyingAssets string) *VaultAccount {
	return &VaultAccount{
		BaseAccount:         baseAcc,
		Admin:               admin,
		ShareDenom:          shareDenom,
		UnderlyingAsset:     underlyingAssets,
		CurrentInterestRate: ZeroInterestRate,
		DesiredInterestRate: ZeroInterestRate,
		SwapInEnabled:       true,
		SwapOutEnabled:      true,
	}
}

// Clone makes a MarkerAccount instance copy
func (va VaultAccount) Clone() *VaultAccount {
	return proto.Clone(&va).(*VaultAccount)
}

// Validate performs a series of checks to ensure the VaultAccount is correctly configured.
// It validates the following:
//   - The admin address is a valid bech32 address.
//   - The share denom is valid.
//   - At least one underlying asset is specified, and each has a valid denom.
//   - The current, desired, minimum, and maximum interest rates (if provided) are valid decimals.
//   - The minimum interest rate is not greater than the maximum interest rate.
//   - The desired interest rate falls within the min/max bounds (if set).
//   - The current interest rate is either zero or equal to the desired interest rate.
//
// Returns an error describing the first validation failure encountered, or nil if the VaultAccount is valid.
func (va VaultAccount) Validate() error {
	if _, err := sdk.AccAddressFromBech32(va.Admin); err != nil {
		return fmt.Errorf("invalid admin address: %w", err)
	}
	if err := sdk.ValidateDenom(va.ShareDenom); err != nil {
		return fmt.Errorf("invalid share denom: %w", err)
	}

	if va.PaymentDenom != "" && (va.PaymentDenom == va.ShareDenom || va.PaymentDenom == va.UnderlyingAsset) {
		return fmt.Errorf("payment_denom must not equal share_denom | underlying_asset")
	}

	if err := sdk.ValidateDenom(va.UnderlyingAsset); err != nil {
		return fmt.Errorf("invalid underlying asset denom: %s", va.UnderlyingAsset)
	}

	cur, err := sdkmath.LegacyNewDecFromStr(va.CurrentInterestRate)
	if err != nil {
		return fmt.Errorf("invalid current interest rate: %s", va.CurrentInterestRate)
	}
	des, err := sdkmath.LegacyNewDecFromStr(va.DesiredInterestRate)
	if err != nil {
		return fmt.Errorf("invalid desired interest rate: %s", va.DesiredInterestRate)
	}

	var min, max sdkmath.LegacyDec
	hasMin := va.MinInterestRate != ""
	hasMax := va.MaxInterestRate != ""

	if hasMin {
		min, err = sdkmath.LegacyNewDecFromStr(va.MinInterestRate)
		if err != nil {
			return fmt.Errorf("invalid min interest rate: %s", va.MinInterestRate)
		}
	}
	if hasMax {
		max, err = sdkmath.LegacyNewDecFromStr(va.MaxInterestRate)
		if err != nil {
			return fmt.Errorf("invalid max interest rate: %s", va.MaxInterestRate)
		}
	}

	if hasMin && hasMax && min.GT(max) {
		return fmt.Errorf("minimum interest rate %s cannot be greater than maximum interest rate %s", min, max)
	}

	if hasMin && des.LT(min) {
		return fmt.Errorf("desired interest rate %s is less than minimum interest rate %s", des, min)
	}
	if hasMax && des.GT(max) {
		return fmt.Errorf("desired interest rate %s is greater than maximum interest rate %s", des, max)
	}

	if !cur.IsZero() && !cur.Equal(des) {
		return fmt.Errorf("current interest rate must be zero or equal to desired (current=%s desired=%s)", cur, des)
	}

	return nil
}

func (va VaultAccount) InterestEnabled() bool {
	current, err := sdkmath.LegacyNewDecFromStr(va.CurrentInterestRate)
	if err != nil {
		return false
	}
	return !current.IsZero()
}

// IsInterestRateInRange returns true if the given rate is within the configured min/max bounds.
// If either bound is unset (""), it is treated as unbounded in that direction.
func (v *VaultAccount) IsInterestRateInRange(rate sdkmath.LegacyDec) (bool, error) {
	if v.MinInterestRate != "" {
		minRate, err := sdkmath.LegacyNewDecFromStr(v.MinInterestRate)
		if err != nil {
			return false, fmt.Errorf("invalid min interest rate: %w", err)
		}
		if rate.LT(minRate) {
			return false, nil
		}
	}

	if v.MaxInterestRate != "" {
		maxRate, err := sdkmath.LegacyNewDecFromStr(v.MaxInterestRate)
		if err != nil {
			return false, fmt.Errorf("invalid max interest rate: %w", err)
		}
		if rate.GT(maxRate) {
			return false, nil
		}
	}

	return true, nil
}

func (v *VaultAccount) ValidateAdmin(admin string) error {
	if v.Admin != admin {
		return fmt.Errorf("unauthorized: %s is not the vault admin", admin)
	}
	return nil
}

func (v *VaultAccount) ValuationDenom() string {
	return v.UnderlyingAsset
}

func (v *VaultAccount) IsPaymentOrAssetDenom(denom string) bool {
	return denom == v.UnderlyingAsset || (v.PaymentDenom != "" && denom == v.PaymentDenom)
}

func (v *VaultAccount) ValidateDepositCoin(c sdk.Coin) error {
	if c.IsZero() {
		return fmt.Errorf("zero deposit")
	}
	if !v.IsPaymentOrAssetDenom(c.Denom) {
		return fmt.Errorf("deposit denom %s not accepted", c.Denom)
	}
	return nil
}

func (v *VaultAccount) ValidateRedeemDenom(denom string) error {
	if denom == "" {
		return fmt.Errorf("empty redeem denom")
	}
	if !v.IsPaymentOrAssetDenom(denom) {
		return fmt.Errorf("redeem denom %s not accepted", denom)
	}
	return nil
}
