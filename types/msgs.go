package types

import (
	"errors"
	fmt "fmt"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ValidateBasic returns a not implemented error for MsgCreateVaultRequest.
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

	return nil
}

// ValidateBasic returns a not implemented error for MsgSwapInRequest.
func (m MsgSwapInRequest) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(m.VaultAddress)
	if err != nil {
		return fmt.Errorf("invalid vault address %s : %w", m.VaultAddress, err)
	}
	_, err = sdk.AccAddressFromBech32(m.Owner)
	if err != nil {
		return fmt.Errorf("invalid owner address %s : %w", m.VaultAddress, err)
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

// ValidateBasic performs stateless validation on MsgSwapOutRequest fields.
func (m MsgSwapOutRequest) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(m.VaultAddress)
	if err != nil {
		return fmt.Errorf("invalid vault address %s : %w", m.VaultAddress, err)
	}
	_, err = sdk.AccAddressFromBech32(m.Owner)
	if err != nil {
		return fmt.Errorf("invalid owner address %s : %w", m.Owner, err)
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

// ValidateBasic returns a not implemented error for MsgRedeemRequest.
func (m MsgUpdateParams) ValidateBasic() error {
	return errors.New("ValidateBasic not implemented for MsgUpdateParams")
}

// ValidateBasic performs stateless validation on MsgSetInterestConfigRequest.
func (m MsgSetInterestConfigRequest) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Admin); err != nil {
		return fmt.Errorf("invalid admin address: %q: %w", m.Admin, err)
	}
	if _, err := sdk.AccAddressFromBech32(m.VaultAddress); err != nil {
		return fmt.Errorf("invalid vault address: %q: %w", m.VaultAddress, err)
	}
	if _, err := sdkmath.LegacyNewDecFromStr(m.MinRate); err != nil {
		return fmt.Errorf("invalid min rate: %q: %w", m.MinRate, err)
	}
	if _, err := sdkmath.LegacyNewDecFromStr(m.MaxRate); err != nil {
		return fmt.Errorf("invalid max rate: %q: %w", m.MaxRate, err)
	}
	return nil
}

// ValidateBasic performs stateless validation on MsgUpdateInterestRateRequest.
func (m MsgUpdateInterestRateRequest) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Admin); err != nil {
		return fmt.Errorf("invalid admin address: %q: %w", m.Admin, err)
	}
	if _, err := sdk.AccAddressFromBech32(m.VaultAddress); err != nil {
		return fmt.Errorf("invalid vault address: %q: %w", m.VaultAddress, err)
	}
	if _, err := sdkmath.LegacyNewDecFromStr(m.NewRate); err != nil {
		return fmt.Errorf("invalid interest rate: %q: %w", m.NewRate, err)
	}
	return nil
}

// ValidateBasic performs stateless validation on MsgDepositInterestFundsRequest.
func (m MsgDepositInterestFundsRequest) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Admin); err != nil {
		return fmt.Errorf("invalid admin address: %q: %w", m.Admin, err)
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
	if _, err := sdk.AccAddressFromBech32(m.InterestAdmin); err != nil {
		return fmt.Errorf("invalid interest admin address: %q: %w", m.InterestAdmin, err)
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

// ValidateBasic performs stateless validation on MsgToggleSwapsRequest.
func (m MsgToggleSwapsRequest) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Admin); err != nil {
		return fmt.Errorf("invalid admin address: %q: %w", m.Admin, err)
	}
	if _, err := sdk.AccAddressFromBech32(m.VaultAddress); err != nil {
		return fmt.Errorf("invalid vault address: %q: %w", m.VaultAddress, err)
	}
	return nil
}
