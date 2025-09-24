package types

import (
	fmt "fmt"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	proto "github.com/cosmos/gogoproto/proto"

	markertypes "github.com/provenance-io/provenance/x/marker/types"
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

	// Clone returns a deep copy of the underlying VaultAccount implementation.
	// The returned value shares no state with the receiver.
	Clone() *VaultAccount

	// Validate performs stateless validation of the vault's fields and invariants.
	// It should return an error if any field is malformed or inconsistent.
	Validate() error

	// GetAdmin returns the bech32-encoded address string of the vault administrator.
	GetAdmin() string
	// GetTotalShares returns the total shares issued by the vault.
	GetTotalShares() sdk.Coin

	// GetShareDenom returns the share token denom that the vault mints/burns to
	// represent proportional ownership in the vault's underlying assets.
	GetShareDenom() string

	// GetUnderlyingAsset returns the denom of the asset the vault actually holds.
	GetUnderlyingAsset() string

	// GetPaymentDenom returns the denom used for fees/interest payments, if any.
	GetPaymentDenom() string

	// GetWithdrawalDelaySeconds returns the number of seconds that
	// withdrawals from this vault are delayed after a swap-out request.
	// A value of 0 means withdrawals are processed immediately in the end blocker.
	GetWithdrawalDelaySeconds() uint64

	// GetPaused reports whether the vault is currently paused.
	// A paused vault disables user operations such as swaps or reconciles.
	GetPaused() bool

	// GetPausedBalance returns the balance snapshot held when the vault
	// was paused. This is used to track the reserves or assets that are
	// locked during the pause state.
	GetPausedBalance() sdk.Coin
}

// NewVaultAccount creates a new vault with an optional payment denom allowed for I/O alongside the underlying asset.
func NewVaultAccount(baseAcc *authtypes.BaseAccount, admin, shareDenom, underlyingAsset, paymentDenom string, withdrawalDelay uint64) *VaultAccount {
	return &VaultAccount{
		BaseAccount:            baseAcc,
		Admin:                  admin,
		ShareDenom:             shareDenom,
		TotalShares:            sdk.Coin{Denom: shareDenom, Amount: sdkmath.ZeroInt()},
		UnderlyingAsset:        underlyingAsset,
		PaymentDenom:           paymentDenom,
		CurrentInterestRate:    ZeroInterestRate,
		DesiredInterestRate:    ZeroInterestRate,
		SwapInEnabled:          true,
		SwapOutEnabled:         true,
		WithdrawalDelaySeconds: withdrawalDelay,
		Paused:                 false,
		PausedBalance:          sdk.Coin{},
		BridgeEnabled:          false,
		BridgeAddress:          "",
	}
}

// Clone makes a MarkerAccount instance copy.
func (va VaultAccount) Clone() *VaultAccount {
	return proto.Clone(&va).(*VaultAccount)
}

// Validate performs a series of checks to ensure the VaultAccount is correctly configured.
func (va VaultAccount) Validate() error {
	if va.BaseAccount == nil {
		return fmt.Errorf("base account cannot be nil")
	}

	if err := va.BaseAccount.Validate(); err != nil {
		return err
	}

	if _, err := sdk.AccAddressFromBech32(va.Admin); err != nil {
		return fmt.Errorf("invalid admin address: %w", err)
	}
	if err := sdk.ValidateDenom(va.ShareDenom); err != nil {
		return fmt.Errorf("invalid share denom: %w", err)
	}
	if va.TotalShares.Denom != va.ShareDenom {
		return fmt.Errorf("total shares denom (%s) must match share denom (%s)", va.TotalShares.Denom, va.ShareDenom)
	}
	if va.TotalShares.IsNegative() {
		return fmt.Errorf("total shares cannot be negative: %s", va.TotalShares)
	}
	if err := sdk.ValidateDenom(va.UnderlyingAsset); err != nil {
		return fmt.Errorf("invalid underlying asset denom: %s", va.UnderlyingAsset)
	}

	if va.PaymentDenom != "" {
		if err := sdk.ValidateDenom(va.PaymentDenom); err != nil {
			return fmt.Errorf("invalid payment denom: %q: %w", va.PaymentDenom, err)
		}
		if va.PaymentDenom == va.UnderlyingAsset {
			return fmt.Errorf("payment (%q) denom cannot equal underlying asset denom (%q)", va.PaymentDenom, va.UnderlyingAsset)
		}
	}

	if va.BridgeAddress != "" {
		if _, err := sdk.AccAddressFromBech32(va.BridgeAddress); err != nil {
			return fmt.Errorf("invalid bridge address: %w", err)
		}
	}
	if va.BridgeEnabled && va.BridgeAddress == "" {
		return fmt.Errorf("bridge cannot be enabled without a bridge address")
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

// AcceptedDenoms returns the list of coin denoms accepted for I/O.
// Always includes the underlying asset; includes payment_denom only if set and distinct.
func (v *VaultAccount) AcceptedDenoms() []string {
	if v.PaymentDenom != "" && v.PaymentDenom != v.UnderlyingAsset {
		return []string{v.UnderlyingAsset, v.PaymentDenom}
	}
	return []string{v.UnderlyingAsset}
}

// IsAcceptedDenom reports whether denom is allowed by the vault configuration.
func (v *VaultAccount) IsAcceptedDenom(denom string) bool {
	if denom == v.UnderlyingAsset {
		return true
	}
	return v.PaymentDenom != "" && denom == v.PaymentDenom
}

// ValidateAcceptedDenom returns an error if denom is not supported by the vault.
func (v *VaultAccount) ValidateAcceptedDenom(denom string) error {
	if v.IsAcceptedDenom(denom) {
		return nil
	}
	allowed := v.AcceptedDenoms()
	if len(allowed) == 1 {
		return fmt.Errorf(`denom not supported for vault; must be "%s": got "%s"`, allowed[0], denom)
	}
	return fmt.Errorf(`denom not supported for vault; must be one of "%s" or "%s": got "%s"`, allowed[0], allowed[1], denom)
}

// ValidateAcceptedCoin returns an error if the coin amount is zero or its denom is not supported.
func (v *VaultAccount) ValidateAcceptedCoin(c sdk.Coin) error {
	if c.IsZero() {
		return fmt.Errorf("amount must be greater than zero")
	}
	return v.ValidateAcceptedDenom(c.Denom)
}

// PrincipalMarkerAddress returns the share-denom marker address that holds the
// vault’s principal (i.e., the marker account backing the vault’s shares).
func (v VaultAccount) PrincipalMarkerAddress() sdk.AccAddress {
	return markertypes.MustGetMarkerAddress(v.ShareDenom)
}
