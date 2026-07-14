package types

import (
	fmt "fmt"
	"math"

	gproto "google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/protoadapt"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	proto "github.com/cosmos/gogoproto/proto"

	markertypes "github.com/provenance-io/provenance/x/marker/types"
)

const (
	// ZeroInterestRate is the canonical decimal string for a disabled (zero) interest rate.
	ZeroInterestRate = "0.0"

	// MaxWithdrawalDelay caps the swap-out withdrawal delay in seconds (2 years).
	MaxWithdrawalDelay = 31_536_000 * 2

	// MaxAbsInterestRate is the absolute ceiling on any interest rate's magnitude (100.0 == 10,000% APR),
	// bounding the e^(rt) exponent so an admin-set rate cannot overflow the LegacyDec interest math.
	MaxAbsInterestRate = "100.0"
)

var (
	_ sdk.AccountI             = (*VaultAccount)(nil)
	_ authtypes.GenesisAccount = (*VaultAccount)(nil)

	// maxAbsInterestRateDec is the parsed form of MaxAbsInterestRate, computed once.
	maxAbsInterestRateDec = sdkmath.LegacyMustNewDecFromStr(MaxAbsInterestRate)
)

// ValidateInterestRateMagnitude returns an error if the absolute value of rate
// exceeds the MaxAbsInterestRate ceiling. The check is symmetric: a large negative
// rate overflows the e^(rt) series exactly as a large positive one does.
func ValidateInterestRateMagnitude(rate sdkmath.LegacyDec) error {
	if rate.Abs().GT(maxAbsInterestRateDec) {
		return fmt.Errorf("interest rate %s exceeds maximum allowed magnitude %s", rate, MaxAbsInterestRate)
	}
	return nil
}

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

// NewVaultAccount creates a new single-denom vault: the underlying asset is the only
// denom accepted for deposits, redemptions, fees, and interest. The deprecated
// payment_denom field is populated with the underlying asset purely for wire
// compatibility; it carries no independent meaning.
//
// The vault's operational limits are configured via:
//   - withdrawalDelay: minimum seconds a swap-out must wait in queue.
//   - aumFeeBips: annual management fee in basis points (1/100th of 1%).
//   - minSwapInValue / minSwapOutValue: minimum amount of underlying asset required for swaps.
//     An empty string "" or "0" indicates no minimum limit.
//   - maxSwapInValue / maxSwapOutValue: maximum allowed amount of underlying asset for swaps.
//     An empty string "" indicates no maximum limit. A value of "0" is invalid and rejected
//     by validation (administrators should use Toggle messages to disable operations instead).
//
// All swap limit values are represented as integer strings (cosmos.IntString).
func NewVaultAccount(baseAcc *authtypes.BaseAccount, admin, shareDenom, underlyingAsset string, withdrawalDelay uint64, aumFeeBips uint32, minSwapInValue, minSwapOutValue, maxSwapInValue, maxSwapOutValue string) *VaultAccount {
	return &VaultAccount{
		BaseAccount:            baseAcc,
		Admin:                  admin,
		NavAuthority:           admin,
		TotalShares:            sdk.Coin{Denom: shareDenom, Amount: sdkmath.ZeroInt()},
		UnderlyingAsset:        underlyingAsset,
		PaymentDenom:           underlyingAsset,
		CurrentInterestRate:    ZeroInterestRate,
		DesiredInterestRate:    ZeroInterestRate,
		SwapInEnabled:          true,
		SwapOutEnabled:         true,
		WithdrawalDelaySeconds: withdrawalDelay,
		Paused:                 false,
		PausedBalance:          sdk.Coin{},
		BridgeEnabled:          false,
		BridgeAddress:          "",
		OutstandingAumFee:      sdk.NewCoin(underlyingAsset, sdkmath.ZeroInt()),
		AumFeeBips:             aumFeeBips,
		MinSwapInValue:         minSwapInValue,
		MinSwapOutValue:        minSwapOutValue,
		MaxSwapInValue:         maxSwapInValue,
		MaxSwapOutValue:        maxSwapOutValue,
	}
}

// Clone makes a VaultAccount instance copy.
func (v VaultAccount) Clone() *VaultAccount {
	return protoadapt.MessageV1Of(gproto.Clone(protoadapt.MessageV2Of(&v))).(*VaultAccount)
}

// ValidateSwapLimits ensures that the provided min and max swap values are valid
// integers, non-negative, and consistent (min <= max).
// An empty string represents no limit. For maximums, "0" blocks all operations.
func ValidateSwapLimits(minStr, maxStr string) error {
	var minVal sdkmath.Int
	if minStr != "" {
		var ok bool
		minVal, ok = sdkmath.NewIntFromString(minStr)
		if !ok {
			return fmt.Errorf("invalid min value: %s", minStr)
		}
		if minVal.IsNegative() {
			return fmt.Errorf("min value must be non-negative: %s", minStr)
		}
	} else {
		minVal = sdkmath.ZeroInt()
	}

	if maxStr != "" {
		maxVal, ok := sdkmath.NewIntFromString(maxStr)
		if !ok {
			return fmt.Errorf("invalid max value: %s", maxStr)
		}
		if maxVal.IsNegative() {
			return fmt.Errorf("max value must be non-negative: %s", maxStr)
		}
		if maxVal.IsZero() {
			return fmt.Errorf("max value cannot be zero; use toggle messages to disable swaps")
		}
		if minVal.GT(maxVal) {
			return fmt.Errorf("min value %s cannot be greater than max value %s", minStr, maxStr)
		}
	}

	return nil
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

	if v.PaymentDenom != "" && v.PaymentDenom != v.UnderlyingAsset {
		return fmt.Errorf("payment denom %q must be empty or equal underlying asset %q; vaults are single-denom", v.PaymentDenom, v.UnderlyingAsset)
	}

	if v.AssetManager != "" {
		if _, err := sdk.AccAddressFromBech32(v.AssetManager); err != nil {
			return fmt.Errorf("invalid asset manager address: %w", err)
		}
	}

	if v.NavAuthority != "" {
		if _, err := sdk.AccAddressFromBech32(v.NavAuthority); err != nil {
			return fmt.Errorf("invalid nav authority address: %w", err)
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
	if err = ValidateInterestRateMagnitude(cur); err != nil {
		return fmt.Errorf("invalid current interest rate: %w", err)
	}
	des, err := sdkmath.LegacyNewDecFromStr(v.DesiredInterestRate)
	if err != nil {
		return fmt.Errorf("invalid desired interest rate: %s", v.DesiredInterestRate)
	}
	if err = ValidateInterestRateMagnitude(des); err != nil {
		return fmt.Errorf("invalid desired interest rate: %w", err)
	}

	var minRate, maxRate sdkmath.LegacyDec
	hasMin := v.MinInterestRate != ""
	hasMax := v.MaxInterestRate != ""

	if hasMin {
		minRate, err = sdkmath.LegacyNewDecFromStr(v.MinInterestRate)
		if err != nil {
			return fmt.Errorf("invalid min interest rate: %s", v.MinInterestRate)
		}
		if err = ValidateInterestRateMagnitude(minRate); err != nil {
			return fmt.Errorf("invalid min interest rate: %w", err)
		}
	}
	if hasMax {
		maxRate, err = sdkmath.LegacyNewDecFromStr(v.MaxInterestRate)
		if err != nil {
			return fmt.Errorf("invalid max interest rate: %s", v.MaxInterestRate)
		}
		if err := ValidateInterestRateMagnitude(maxRate); err != nil {
			return fmt.Errorf("invalid max interest rate: %w", err)
		}
	}

	if hasMin && hasMax && minRate.GT(maxRate) {
		return fmt.Errorf("minimum interest rate %s cannot be greater than maximum interest rate %s", minRate, maxRate)
	}
	if hasMin && des.LT(minRate) {
		return fmt.Errorf("desired interest rate %s is less than minimum interest rate %s", des, minRate)
	}
	if hasMax && des.GT(maxRate) {
		return fmt.Errorf("desired interest rate %s is greater than maximum interest rate %s", des, maxRate)
	}
	if !cur.IsZero() && !cur.Equal(des) {
		return fmt.Errorf("current interest rate must be zero or equal to desired (current=%s desired=%s)", cur, des)
	}

	if v.PeriodStart < 0 {
		return fmt.Errorf("period start cannot be negative: %d", v.PeriodStart)
	}
	if v.PeriodTimeout < 0 && v.PeriodTimeout != math.MinInt64 {
		return fmt.Errorf("period timeout cannot be negative: %d", v.PeriodTimeout)
	}

	if v.FeePeriodStart < 0 {
		return fmt.Errorf("fee period start cannot be negative: %d", v.FeePeriodStart)
	}
	if v.FeePeriodTimeout < 0 && v.FeePeriodTimeout != math.MinInt64 {
		return fmt.Errorf("fee period timeout cannot be negative: %d", v.FeePeriodTimeout)
	}
	if v.AumFeeBips > 10_000 {
		return fmt.Errorf("AUM fee bips cannot exceed 10,000: %d", v.AumFeeBips)
	}

	if v.OutstandingAumFee.Amount.IsNil() {
		return fmt.Errorf("outstanding AUM fee amount cannot be nil; use a zero coin of the underlying asset %q", v.UnderlyingAsset)
	}
	if v.OutstandingAumFee.IsNegative() {
		return fmt.Errorf("outstanding AUM fee cannot be negative: %s", v.OutstandingAumFee)
	}
	if (v.OutstandingAumFee.Denom != "" || !v.OutstandingAumFee.Amount.IsZero()) && v.OutstandingAumFee.Denom != v.UnderlyingAsset {
		return fmt.Errorf("outstanding AUM fee denom %s does not match underlying asset %s", v.OutstandingAumFee.Denom, v.UnderlyingAsset)
	}

	if err := ValidateSwapLimits(v.MinSwapInValue, v.MaxSwapInValue); err != nil {
		return fmt.Errorf("failed to validate swap-in limits: %w", err)
	}

	if err := ValidateSwapLimits(v.MinSwapOutValue, v.MaxSwapOutValue); err != nil {
		return fmt.Errorf("failed to validate swap-out limits: %w", err)
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

// GetNAVAuthority returns the address authorized to mutate the vault's internal
// NAV table. When nav_authority is unset, the vault admin is treated as the NAV
// authority.
func (v *VaultAccount) GetNAVAuthority() string {
	if v.NavAuthority != "" {
		return v.NavAuthority
	}
	return v.Admin
}

// ValidateNAVAuthority returns an error if the given address is not the vault's
// NAV authority.
func (v *VaultAccount) ValidateNAVAuthority(signer string) error {
	if signer != v.GetNAVAuthority() {
		return fmt.Errorf("unauthorized: %s is not the vault NAV authority", signer)
	}
	return nil
}

// IsAcceptedDenom reports whether denom is allowed for vault I/O. Vaults are
// single-denom: only the underlying asset is accepted.
func (v *VaultAccount) IsAcceptedDenom(denom string) bool {
	return denom == v.UnderlyingAsset
}

// ValidateAcceptedDenom returns an error if denom is not the vault's underlying asset.
func (v *VaultAccount) ValidateAcceptedDenom(denom string) error {
	if v.IsAcceptedDenom(denom) {
		return nil
	}
	return fmt.Errorf(`denom not supported for vault; must be "%s": got "%s"`, v.UnderlyingAsset, denom)
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

// ValidateAssetManagerAuthority checks whether the given address is the vault's asset
// manager. It guards actions reserved for the asset manager alone (e.g. P2P settlement),
// where the admin is deliberately excluded: the field is a role, not a person, so a vault
// owner who wants a composite approval workflow (e.g. admin and manager both sign) points
// the asset manager at a group address rather than the module offering per-vault
// configurability. A vault with no asset manager cannot perform these actions at all.
func (v VaultAccount) ValidateAssetManagerAuthority(authority string) error {
	if v.AssetManager == "" {
		return fmt.Errorf("no asset manager set")
	}
	if authority != v.AssetManager {
		return fmt.Errorf("unauthorized authority: %s", authority)
	}
	return nil
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

// NewVaultNAV creates a new VaultNAV entry for the given denom, pricing
// volume units at price and attributing the entry to source.
func NewVaultNAV(denom string, price sdk.Coin, volume sdkmath.Int, source string) VaultNAV {
	return VaultNAV{
		Denom:  denom,
		Price:  price,
		Volume: volume,
		Source: source,
	}
}
