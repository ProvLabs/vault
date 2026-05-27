package types

import (
	"errors"
	fmt "fmt"
	"strings"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

const maxDenomMetadataDescriptionLength = 200

// MaxNAVSourceLength bounds the VaultNAV.source attribution string to keep
// per-entry state and emitted events from being inflated by an unbounded value.
const MaxNAVSourceLength = 200

// AllRequestMsgs defines all the Msg*Request messages.
var AllRequestMsgs = []sdk.Msg{
	(*MsgCreateVaultRequest)(nil),
	(*MsgSetShareDenomMetadataRequest)(nil),
	(*MsgSwapInRequest)(nil),
	(*MsgSwapOutRequest)(nil),
	(*MsgUpdateMinInterestRateRequest)(nil),
	(*MsgUpdateMaxInterestRateRequest)(nil),
	(*MsgUpdateInterestRateRequest)(nil),
	(*MsgUpdateWithdrawalDelayRequest)(nil),
	(*MsgToggleSwapInRequest)(nil),
	(*MsgToggleSwapOutRequest)(nil),
	(*MsgDepositInterestFundsRequest)(nil),
	(*MsgWithdrawInterestFundsRequest)(nil),
	(*MsgDepositPrincipalFundsRequest)(nil),
	(*MsgWithdrawPrincipalFundsRequest)(nil),
	(*MsgExpeditePendingSwapOutRequest)(nil),
	(*MsgPauseVaultRequest)(nil),
	(*MsgUnpauseVaultRequest)(nil),
	(*MsgSetBridgeAddressRequest)(nil),
	(*MsgToggleBridgeRequest)(nil),
	(*MsgBridgeMintSharesRequest)(nil),
	(*MsgBridgeBurnSharesRequest)(nil),
	(*MsgSetAssetManagerRequest)(nil),
	(*MsgUpdateParamsRequest)(nil),
	(*MsgUpdateVaultAUMFeeBipsRequest)(nil),
	(*MsgUpdateMinSwapInValueRequest)(nil),
	(*MsgUpdateMinSwapOutValueRequest)(nil),
	(*MsgUpdateMaxSwapInValueRequest)(nil),
	(*MsgUpdateMaxSwapOutValueRequest)(nil),
	(*MsgUpdateVaultNAVRequest)(nil),
	(*MsgUpdateNAVAuthorityRequest)(nil),
}

// ValidateBasic performs stateless validation on MsgCreateVaultRequest.
func (m MsgCreateVaultRequest) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(m.Admin)
	if err != nil {
		return fmt.Errorf("invalid admin address: %q: %w", m.Admin, err)
	}

	if err := sdk.ValidateDenom(m.ShareDenom); err != nil {
		return fmt.Errorf("invalid share denom: %q: %w", m.ShareDenom, err)
	}

	if err := sdk.ValidateDenom(m.UnderlyingAsset); err != nil {
		return fmt.Errorf("invalid underlying asset: %q: %w", m.UnderlyingAsset, err)
	}

	if len(m.PaymentDenom) > 0 {
		if err := sdk.ValidateDenom(m.PaymentDenom); err != nil {
			return fmt.Errorf("invalid payment denom: %q: %w", m.PaymentDenom, err)
		}
	}

	if m.ShareDenom == m.UnderlyingAsset {
		return fmt.Errorf("share denom (%q) cannot equal underlying asset denom (%q)", m.ShareDenom, m.UnderlyingAsset)
	}
	if m.ShareDenom == m.PaymentDenom {
		return fmt.Errorf("share denom (%q) cannot equal payment denom (%q)", m.ShareDenom, m.PaymentDenom)
	}

	if m.WithdrawalDelaySeconds > MaxWithdrawalDelay {
		return fmt.Errorf("withdrawal delay cannot exceed %d seconds", MaxWithdrawalDelay)
	}

	if err := ValidateSwapLimits(m.MinSwapInValue, m.MaxSwapInValue); err != nil {
		return fmt.Errorf("invalid swap-in limits: %w", err)
	}

	if err := ValidateSwapLimits(m.MinSwapOutValue, m.MaxSwapOutValue); err != nil {
		return fmt.Errorf("invalid swap-out limits: %w", err)
	}

	if err := ValidateInitialPaymentNAV(m.PaymentDenom, m.UnderlyingAsset, m.InitialPaymentNav); err != nil {
		return fmt.Errorf("invalid initial payment NAV: %w", err)
	}

	return nil
}

// ValidateInitialPaymentNAV performs stateless validation on an initial
// payment-denom NAV supplied at vault creation. The NAV must be present when
// paymentDenom is set and differs from underlyingAsset, and must be absent
// otherwise (the identity 1:1 price applies and storing a NAV for the
// underlying asset is rejected by the keeper anyway).
//
// When present, the NAV's price must be denominated in underlyingAsset with a
// positive amount, volume must be positive, and the source string must not
// exceed MaxNAVSourceLength.
func ValidateInitialPaymentNAV(paymentDenom, underlyingAsset string, nav *InitialVaultNAV) error {
	requiresNAV := paymentDenom != "" && paymentDenom != underlyingAsset
	if !requiresNAV {
		if nav != nil {
			return fmt.Errorf("initial_payment_nav must be omitted when payment_denom is empty or equals underlying_asset")
		}
		return nil
	}
	if nav == nil {
		return fmt.Errorf("initial_payment_nav is required when payment_denom %q differs from underlying_asset %q", paymentDenom, underlyingAsset)
	}
	if err := nav.Price.Validate(); err != nil {
		return fmt.Errorf("invalid price coin %v: %w", nav.Price, err)
	}
	if nav.Price.Denom != underlyingAsset {
		return fmt.Errorf("price denom %q must equal underlying_asset %q", nav.Price.Denom, underlyingAsset)
	}
	if !nav.Price.Amount.IsPositive() {
		return fmt.Errorf("price amount must be positive, got %s", nav.Price.Amount)
	}
	if nav.Volume.IsNil() || !nav.Volume.IsPositive() {
		return fmt.Errorf("volume must be positive")
	}
	if len(nav.Source) > MaxNAVSourceLength {
		return fmt.Errorf("source too long (expected <= %d, actual: %d)", MaxNAVSourceLength, len(nav.Source))
	}
	return nil
}

// ValidateBasic performs stateless validation on MsgSetShareDenomMetadataRequest.
func (m MsgSetShareDenomMetadataRequest) ValidateBasic() error {
	if len(m.VaultAddress) == 0 {
		return errors.New("invalid set denom metadata request: vault address cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(m.VaultAddress); err != nil {
		return fmt.Errorf("invalid set denom metadata request: vault address must be a bech32 address string: %w", err)
	}
	if len(m.Admin) == 0 {
		return errors.New("invalid set denom metadata request: administrator cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(m.Admin); err != nil {
		return fmt.Errorf("invalid set denom metadata request: administrator must be a bech32 address string: %w", err)
	}
	if err := ValidateDenomMetadataBasic(m.Metadata); err != nil {
		return fmt.Errorf("invalid set denom metadata request: %w", err)
	}
	return nil
}

// ValidateDenomMetadataBasic performs lightweight, display-oriented validation of
// denomination metadata used for vault share tokens. Unlike the Marker Module it intentionally avoids
// the strict SI-prefix and denom-root checks applied to on-chain currency
// metadata, allowing vault administrators flexibility in naming and formatting.
//
// This function verifies only that:
//   - Base and Display fields are non-empty (after trimming whitespace)
//   - Description length does not exceed maxDenomMetadataDescriptionLength
//
// It does not enforce full denom syntax or unit relationships, since share
// metadata may include arbitrary display names, symbols, or localized text.
func ValidateDenomMetadataBasic(md banktypes.Metadata) error {
	if strings.TrimSpace(md.Base) == "" {
		return errors.New("denom metadata base cannot be empty")
	}
	if strings.TrimSpace(md.Display) == "" {
		return errors.New("denom metadata display cannot be empty")
	}
	if len(md.Description) > maxDenomMetadataDescriptionLength {
		return fmt.Errorf("denom metadata description too long (expected <= %d, actual: %d)", maxDenomMetadataDescriptionLength, len(md.Description))
	}
	return nil
}

// ValidateBasic performs stateless validation on MsgSwapInRequest.
func (m MsgSwapInRequest) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(m.VaultAddress)
	if err != nil {
		return fmt.Errorf("invalid vault address: %q: %w", m.VaultAddress, err)
	}
	_, err = sdk.AccAddressFromBech32(m.Owner)
	if err != nil {
		return fmt.Errorf("invalid owner address: %q: %w", m.Owner, err)
	}

	err = m.Assets.Validate()
	if err != nil {
		return fmt.Errorf("invalid assets coin %v: %w", m.Assets, err)
	}

	if !m.Assets.Amount.GT(sdkmath.NewInt(0)) {
		return fmt.Errorf("invalid amount: assets %s must be greater than zero", m.Assets.Denom)
	}

	return nil
}

// ValidateBasic performs stateless validation on MsgSwapOutRequest.
func (m MsgSwapOutRequest) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(m.VaultAddress)
	if err != nil {
		return fmt.Errorf("invalid vault address: %q: %w", m.VaultAddress, err)
	}
	_, err = sdk.AccAddressFromBech32(m.Owner)
	if err != nil {
		return fmt.Errorf("invalid owner address: %q: %w", m.Owner, err)
	}

	err = m.Assets.Validate()
	if err != nil {
		return fmt.Errorf("invalid assets coin %v: %w", m.Assets, err)
	}

	if !m.Assets.Amount.GT(sdkmath.NewInt(0)) {
		return fmt.Errorf("invalid amount: assets %s must be greater than zero", m.Assets.Denom)
	}

	if m.RedeemDenom != "" {
		if err := sdk.ValidateDenom(m.RedeemDenom); err != nil {
			return fmt.Errorf("invalid redeem_denom: %w", err)
		}
	}

	return nil
}

// ValidateBasic performs stateless validation on MsgUpdateMinInterestRateRequest.
func (m MsgUpdateMinInterestRateRequest) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Admin); err != nil {
		return fmt.Errorf("invalid admin address: %q: %w", m.Admin, err)
	}
	if _, err := sdk.AccAddressFromBech32(m.VaultAddress); err != nil {
		return fmt.Errorf("invalid vault address: %q: %w", m.VaultAddress, err)
	}
	if m.MinRate != "" {
		if _, err := sdkmath.LegacyNewDecFromStr(m.MinRate); err != nil {
			return fmt.Errorf("invalid min rate: %q: %w", m.MinRate, err)
		}
	}
	return nil
}

// ValidateBasic performs stateless validation on MsgUpdateMaxInterestRateRequest.
func (m MsgUpdateMaxInterestRateRequest) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Admin); err != nil {
		return fmt.Errorf("invalid admin address: %q: %w", m.Admin, err)
	}
	if _, err := sdk.AccAddressFromBech32(m.VaultAddress); err != nil {
		return fmt.Errorf("invalid vault address: %q: %w", m.VaultAddress, err)
	}
	if m.MaxRate != "" {
		if _, err := sdkmath.LegacyNewDecFromStr(m.MaxRate); err != nil {
			return fmt.Errorf("invalid max rate: %q: %w", m.MaxRate, err)
		}
	}
	return nil
}

// ValidateBasic performs stateless validation on MsgUpdateInterestRateRequest.
func (m MsgUpdateInterestRateRequest) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return fmt.Errorf("invalid authority address: %q: %w", m.Authority, err)
	}
	if _, err := sdk.AccAddressFromBech32(m.VaultAddress); err != nil {
		return fmt.Errorf("invalid vault address: %q: %w", m.VaultAddress, err)
	}
	if _, err := sdkmath.LegacyNewDecFromStr(m.NewRate); err != nil {
		return fmt.Errorf("invalid interest rate: %q: %w", m.NewRate, err)
	}
	return nil
}

// ValidateBasic performs stateless validation on MsgUpdateWithdrawalDelayRequest.
func (m MsgUpdateWithdrawalDelayRequest) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return fmt.Errorf("invalid authority address: %q: %w", m.Authority, err)
	}
	if _, err := sdk.AccAddressFromBech32(m.VaultAddress); err != nil {
		return fmt.Errorf("invalid vault address: %q: %w", m.VaultAddress, err)
	}
	if m.WithdrawalDelaySeconds > MaxWithdrawalDelay {
		return fmt.Errorf("withdrawal delay cannot exceed %d seconds", MaxWithdrawalDelay)
	}
	return nil
}

// ValidateBasic performs stateless validation on MsgToggleSwapInRequest.
func (m MsgToggleSwapInRequest) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Admin); err != nil {
		return fmt.Errorf("invalid admin address: %q: %w", m.Admin, err)
	}
	if _, err := sdk.AccAddressFromBech32(m.VaultAddress); err != nil {
		return fmt.Errorf("invalid vault address: %q: %w", m.VaultAddress, err)
	}
	return nil
}

// ValidateBasic performs stateless validation on MsgToggleSwapOutRequest.
func (m MsgToggleSwapOutRequest) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Admin); err != nil {
		return fmt.Errorf("invalid admin address: %q: %w", m.Admin, err)
	}
	if _, err := sdk.AccAddressFromBech32(m.VaultAddress); err != nil {
		return fmt.Errorf("invalid vault address: %q: %w", m.VaultAddress, err)
	}
	return nil
}

// ValidateBasic performs stateless validation on MsgDepositInterestFundsRequest.
func (m MsgDepositInterestFundsRequest) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return fmt.Errorf("invalid authority address: %q: %w", m.Authority, err)
	}
	if _, err := sdk.AccAddressFromBech32(m.VaultAddress); err != nil {
		return fmt.Errorf("invalid vault address: %q: %w", m.VaultAddress, err)
	}
	if err := m.Amount.Validate(); err != nil {
		return fmt.Errorf("invalid deposit amount: %w", err)
	}
	if !m.Amount.Amount.IsPositive() {
		return fmt.Errorf("deposit amount must be greater than zero")
	}
	return nil
}

// ValidateBasic performs stateless validation on MsgWithdrawInterestFundsRequest.
func (m MsgWithdrawInterestFundsRequest) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return fmt.Errorf("invalid authority address: %q: %w", m.Authority, err)
	}
	if _, err := sdk.AccAddressFromBech32(m.VaultAddress); err != nil {
		return fmt.Errorf("invalid vault address: %q: %w", m.VaultAddress, err)
	}
	if err := m.Amount.Validate(); err != nil {
		return fmt.Errorf("invalid withdrawal amount: %w", err)
	}
	if !m.Amount.Amount.IsPositive() {
		return fmt.Errorf("withdrawal amount must be greater than zero")
	}
	return nil
}

// ValidateBasic performs stateless validation on MsgDepositPrincipalFundsRequest.
func (m MsgDepositPrincipalFundsRequest) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return fmt.Errorf("invalid authority address: %q: %w", m.Authority, err)
	}
	if _, err := sdk.AccAddressFromBech32(m.VaultAddress); err != nil {
		return fmt.Errorf("invalid vault address: %q: %w", m.VaultAddress, err)
	}
	if err := m.Amount.Validate(); err != nil {
		return fmt.Errorf("invalid deposit amount: %w", err)
	}
	if !m.Amount.Amount.IsPositive() {
		return fmt.Errorf("deposit amount must be greater than zero")
	}
	return nil
}

// ValidateBasic performs stateless validation on MsgWithdrawPrincipalFundsRequest.
func (m MsgWithdrawPrincipalFundsRequest) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return fmt.Errorf("invalid authority address: %q: %w", m.Authority, err)
	}
	if _, err := sdk.AccAddressFromBech32(m.VaultAddress); err != nil {
		return fmt.Errorf("invalid vault address: %q: %w", m.VaultAddress, err)
	}
	if err := m.Amount.Validate(); err != nil {
		return fmt.Errorf("invalid withdrawal amount: %w", err)
	}
	if !m.Amount.Amount.IsPositive() {
		return fmt.Errorf("withdrawal amount must be greater than zero")
	}
	return nil
}

// ValidateBasic performs stateless validation on MsgExpeditePendingSwapOutRequest.
func (m MsgExpeditePendingSwapOutRequest) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return fmt.Errorf("invalid authority address: %q: %w", m.Authority, err)
	}
	return nil
}

// ValidateBasic performs stateless validation on MsgPauseVaultRequest.
func (m MsgPauseVaultRequest) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return fmt.Errorf("invalid authority address: %q: %w", m.Authority, err)
	}
	if _, err := sdk.AccAddressFromBech32(m.VaultAddress); err != nil {
		return fmt.Errorf("invalid vault address: %q: %w", m.VaultAddress, err)
	}
	if m.Reason == "" {
		return fmt.Errorf("reason cannot be empty")
	}
	return nil
}

// ValidateBasic performs stateless validation on MsgUnpauseVaultRequest.
func (m MsgUnpauseVaultRequest) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return fmt.Errorf("invalid authority address: %q: %w", m.Authority, err)
	}
	if _, err := sdk.AccAddressFromBech32(m.VaultAddress); err != nil {
		return fmt.Errorf("invalid vault address: %q: %w", m.VaultAddress, err)
	}
	return nil
}

// ValidateBasic performs stateless validation on MsgSetBridgeAddressRequest.
func (m MsgSetBridgeAddressRequest) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Admin); err != nil {
		return fmt.Errorf("invalid admin address: %q: %w", m.Admin, err)
	}
	if _, err := sdk.AccAddressFromBech32(m.VaultAddress); err != nil {
		return fmt.Errorf("invalid vault address: %q: %w", m.VaultAddress, err)
	}
	if _, err := sdk.AccAddressFromBech32(m.BridgeAddress); err != nil {
		return fmt.Errorf("invalid bridge address: %q: %w", m.BridgeAddress, err)
	}
	return nil
}

// ValidateBasic performs stateless validation on MsgToggleBridgeRequest.
func (m MsgToggleBridgeRequest) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Admin); err != nil {
		return fmt.Errorf("invalid admin address: %q: %w", m.Admin, err)
	}
	if _, err := sdk.AccAddressFromBech32(m.VaultAddress); err != nil {
		return fmt.Errorf("invalid vault address: %q: %w", m.VaultAddress, err)
	}
	return nil
}

// ValidateBasic performs stateless validation on MsgBridgeMintSharesRequest.
func (m MsgBridgeMintSharesRequest) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Bridge); err != nil {
		return fmt.Errorf("invalid bridge address: %q: %w", m.Bridge, err)
	}
	if _, err := sdk.AccAddressFromBech32(m.VaultAddress); err != nil {
		return fmt.Errorf("invalid vault address: %q: %w", m.VaultAddress, err)
	}
	if err := m.Shares.Validate(); err != nil {
		return fmt.Errorf("invalid shares coin %v: %w", m.Shares, err)
	}
	if !m.Shares.Amount.IsPositive() {
		return fmt.Errorf("shares amount must be greater than zero")
	}
	return nil
}

// ValidateBasic performs stateless validation on MsgBridgeBurnSharesRequest.
func (m MsgBridgeBurnSharesRequest) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Bridge); err != nil {
		return fmt.Errorf("invalid bridge address: %q: %w", m.Bridge, err)
	}
	if _, err := sdk.AccAddressFromBech32(m.VaultAddress); err != nil {
		return fmt.Errorf("invalid vault address: %q: %w", m.VaultAddress, err)
	}
	if err := m.Shares.Validate(); err != nil {
		return fmt.Errorf("invalid shares coin %v: %w", m.Shares, err)
	}
	if !m.Shares.Amount.IsPositive() {
		return fmt.Errorf("shares amount must be greater than zero")
	}
	return nil
}

// ValidateBasic performs stateless validation on MsgSetAssetManagerRequest.
func (m MsgSetAssetManagerRequest) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Admin); err != nil {
		return fmt.Errorf("invalid admin address: %q: %w", m.Admin, err)
	}
	if _, err := sdk.AccAddressFromBech32(m.VaultAddress); err != nil {
		return fmt.Errorf("invalid vault address: %q: %w", m.VaultAddress, err)
	}
	if m.AssetManager == "" {
		return nil
	}
	if _, err := sdk.AccAddressFromBech32(m.AssetManager); err != nil {
		return fmt.Errorf("invalid asset manager address: %q: %w", m.AssetManager, err)
	}
	return nil
}

// ValidateBasic performs stateless validation on MsgUpdateParamsRequest.
func (m MsgUpdateParamsRequest) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return fmt.Errorf("invalid authority address: %q: %w", m.Authority, err)
	}
	if err := m.Params.Validate(); err != nil {
		return fmt.Errorf("invalid params for MsgUpdateParamsRequest: %w", err)
	}
	return nil
}

// ValidateBasic performs stateless validation on MsgUpdateVaultAUMFeeBipsRequest.
func (m MsgUpdateVaultAUMFeeBipsRequest) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return fmt.Errorf("invalid authority address: %q: %w", m.Authority, err)
	}
	if _, err := sdk.AccAddressFromBech32(m.VaultAddress); err != nil {
		return fmt.Errorf("invalid vault address: %q: %w", m.VaultAddress, err)
	}
	if m.AumFeeBips > 10_000 {
		return fmt.Errorf("invalid AUM fee bips: %d (max 10000)", m.AumFeeBips)
	}
	return nil
}

// ValidateBasic performs stateless validation on MsgUpdateMinSwapInValueRequest.
func (m MsgUpdateMinSwapInValueRequest) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return fmt.Errorf("invalid authority address: %q: %w", m.Authority, err)
	}
	if _, err := sdk.AccAddressFromBech32(m.VaultAddress); err != nil {
		return fmt.Errorf("invalid vault address: %q: %w", m.VaultAddress, err)
	}
	if err := ValidateSwapLimits(m.MinSwapInValue, ""); err != nil {
		return fmt.Errorf("invalid swap-in limits: %w", err)
	}
	return nil
}

// ValidateBasic performs stateless validation on MsgUpdateMinSwapOutValueRequest.
func (m MsgUpdateMinSwapOutValueRequest) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return fmt.Errorf("invalid authority address: %q: %w", m.Authority, err)
	}
	if _, err := sdk.AccAddressFromBech32(m.VaultAddress); err != nil {
		return fmt.Errorf("invalid vault address: %q: %w", m.VaultAddress, err)
	}
	if err := ValidateSwapLimits(m.MinSwapOutValue, ""); err != nil {
		return fmt.Errorf("invalid swap-out limits: %w", err)
	}
	return nil
}

// ValidateBasic performs stateless validation on MsgUpdateMaxSwapInValueRequest.
func (m MsgUpdateMaxSwapInValueRequest) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return fmt.Errorf("invalid authority address: %q: %w", m.Authority, err)
	}
	if _, err := sdk.AccAddressFromBech32(m.VaultAddress); err != nil {
		return fmt.Errorf("invalid vault address: %q: %w", m.VaultAddress, err)
	}
	if err := ValidateSwapLimits("", m.MaxSwapInValue); err != nil {
		return fmt.Errorf("invalid swap-in limits: %w", err)
	}
	return nil
}

// ValidateBasic performs stateless validation on MsgUpdateMaxSwapOutValueRequest.
func (m MsgUpdateMaxSwapOutValueRequest) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return fmt.Errorf("invalid authority address: %q: %w", m.Authority, err)
	}
	if _, err := sdk.AccAddressFromBech32(m.VaultAddress); err != nil {
		return fmt.Errorf("invalid vault address: %q: %w", m.VaultAddress, err)
	}
	if err := ValidateSwapLimits("", m.MaxSwapOutValue); err != nil {
		return fmt.Errorf("invalid swap-out limits: %w", err)
	}
	return nil
}

// ValidateBasic performs stateless validation on MsgUpdateVaultNAVRequest.
func (m MsgUpdateVaultNAVRequest) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Signer); err != nil {
		return fmt.Errorf("invalid signer address: %q: %w", m.Signer, err)
	}
	if _, err := sdk.AccAddressFromBech32(m.VaultAddress); err != nil {
		return fmt.Errorf("invalid vault address: %q: %w", m.VaultAddress, err)
	}
	if err := sdk.ValidateDenom(m.Denom); err != nil {
		return fmt.Errorf("invalid denom: %q: %w", m.Denom, err)
	}
	if err := m.Price.Validate(); err != nil {
		return fmt.Errorf("invalid price coin %v: %w", m.Price, err)
	}
	if m.Denom == m.Price.Denom {
		return fmt.Errorf("NAV denom %q and price denom must differ", m.Denom)
	}
	if !m.Price.Amount.IsPositive() {
		return fmt.Errorf("price amount must be positive, got %s", m.Price.Amount)
	}
	if m.Volume.IsNil() || !m.Volume.IsPositive() {
		return fmt.Errorf("volume must be positive")
	}
	if len(m.Source) > MaxNAVSourceLength {
		return fmt.Errorf("source too long (expected <= %d, actual: %d)", MaxNAVSourceLength, len(m.Source))
	}
	return nil
}

// ValidateBasic performs stateless validation on MsgUpdateNAVAuthorityRequest.
func (m MsgUpdateNAVAuthorityRequest) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Signer); err != nil {
		return fmt.Errorf("invalid signer address: %q: %w", m.Signer, err)
	}
	if _, err := sdk.AccAddressFromBech32(m.VaultAddress); err != nil {
		return fmt.Errorf("invalid vault address: %q: %w", m.VaultAddress, err)
	}
	if m.NewAuthority != "" {
		if _, err := sdk.AccAddressFromBech32(m.NewAuthority); err != nil {
			return fmt.Errorf("invalid new authority address: %q: %w", m.NewAuthority, err)
		}
	}
	return nil
}
