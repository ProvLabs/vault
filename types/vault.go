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
	ZeroInterestRate   = "0.0"
	MaxWithdrawalDelay = 31536000 * 2 // 2 years in seconds
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
func (v VaultAccount) Clone() *VaultAccount {
	return proto.Clone(&v).(*VaultAccount)
}

// Validate performs a series of checks to ensure the VaultAccount is correctly configured.
func (v VaultAccount) Validate() error {
	if v.BaseAccount == nil {
		return fmt.Errorf("base account cannot be nil")
	}

	if err := v.BaseAccount.Validate(); err != nil {
		return err
	}

	if _, err := sdk.AccAddressFromBech32(v.Admin); err != nil {
		return fmt.Errorf("invalid admin address: %w", err)
	}
	if err := sdk.ValidateDenom(v.TotalShares.Denom); err != nil {
		return fmt.Errorf("invalid share denom: %w", err)
	}
	if v.TotalShares.IsNegative() {
		return fmt.Errorf("total shares cannot be negative: %s", v.TotalShares)
	}
	if err := sdk.ValidateDenom(v.UnderlyingAsset); err != nil {
		return fmt.Errorf("invalid underlying asset denom: %s", v.UnderlyingAsset)
	}

	if v.PaymentDenom != "" {
		if err := sdk.ValidateDenom(v.PaymentDenom); err != nil {
			return fmt.Errorf("invalid payment denom: %q: %w", v.PaymentDenom, err)
		}
		if v.PaymentDenom == v.UnderlyingAsset {
			return fmt.Errorf("payment (%q) denom cannot equal underlying asset denom (%q)", v.PaymentDenom, v.UnderlyingAsset)
		}
	}

	if v.AssetManager != "" {
		if _, err := sdk.AccAddressFromBech32(v.AssetManager); err != nil {
			return fmt.Errorf("invalid asset manager address: %w", err)
		}
	}

	if v.BridgeAddress != "" {
		if _, err := sdk.AccAddressFromBech32(v.BridgeAddress); err != nil {
			return fmt.Errorf("invalid bridge address: %w", err)
		}
	}
	if v.BridgeEnabled && v.BridgeAddress == "" {
		return fmt.Errorf("bridge cannot be enabled without a bridge address")
	}

	cur, err := sdkmath.LegacyNewDecFromStr(v.CurrentInterestRate)
	if err != nil {
		return fmt.Errorf("invalid current interest rate: %s", v.CurrentInterestRate)
	}
	des, err := sdkmath.LegacyNewDecFromStr(v.DesiredInterestRate)
	if err != nil {
		return fmt.Errorf("invalid desired interest rate: %s", v.DesiredInterestRate)
	}

	var min, max sdkmath.LegacyDec
	hasMin := v.MinInterestRate != ""
	hasMax := v.MaxInterestRate != ""

	if hasMin {
		min, err = sdkmath.LegacyNewDecFromStr(v.MinInterestRate)
		if err != nil {
			return fmt.Errorf("invalid min interest rate: %s", v.MinInterestRate)
		}
	}
	if hasMax {
		max, err = sdkmath.LegacyNewDecFromStr(v.MaxInterestRate)
		if err != nil {
			return fmt.Errorf("invalid max interest rate: %s", v.MaxInterestRate)
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

// InterestEnabled returns true if the vault is currently configured to use interest.
func (v VaultAccount) InterestEnabled() bool {
	current, err := sdkmath.LegacyNewDecFromStr(v.CurrentInterestRate)
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
	return markertypes.MustGetMarkerAddress(v.TotalShares.Denom)
}

// ValidateManagementAuthority checks whether the given address is authorized to perform
// vault management actions. Either the vault admin or the asset manager (if set) is allowed.
func (v VaultAccount) ValidateManagementAuthority(authority string) error {
	if authority == v.Admin {
		return nil
	}
	if v.AssetManager != "" && authority == v.AssetManager {
		return nil
	}
	return fmt.Errorf("unauthorized authority: %s", authority)
}

// NewPendingSwapOut creates a new PendingSwapOut object.
func NewPendingSwapOut(owner sdk.AccAddress, vaultAddr sdk.AccAddress, shares sdk.Coin, redeemDenom string) PendingSwapOut {
	return PendingSwapOut{
		Owner:        owner.String(),
		VaultAddress: vaultAddr.String(),
		Shares:       shares,
		RedeemDenom:  redeemDenom,
	}
}

// Validate performs basic checks on a PendingSwapOut request.
func (p PendingSwapOut) Validate() error {
	if _, err := sdk.AccAddressFromBech32(p.Owner); err != nil {
		return fmt.Errorf("invalid owner address %s: %w", p.Owner, err)
	}

	if _, err := sdk.AccAddressFromBech32(p.VaultAddress); err != nil {
		return fmt.Errorf("invalid vault address %s: %w", p.VaultAddress, err)
	}

	if !p.Shares.IsValid() {
		return fmt.Errorf("invalid shares: %s", p.Shares.String())
	}

	if p.Shares.IsZero() {
		return fmt.Errorf("shares cannot be zero")
	}

	if p.RedeemDenom == "" {
		return fmt.Errorf("redeem denom cannot be empty")
	}

	return nil
}
