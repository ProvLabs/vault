package types

import (
	"errors"
	fmt "fmt"

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
	return errors.New("ValidateBasic not implemented for MsgSwapInRequest")
}

// ValidateBasic returns a not implemented error for MsgSwapOutRequest.
func (m MsgSwapOutRequest) ValidateBasic() error {
	return errors.New("ValidateBasic not implemented for MsgSwapOutRequest")
}

// ValidateBasic returns a not implemented error for MsgRedeemRequest.
func (m MsgRedeemRequest) ValidateBasic() error {
	return errors.New("ValidateBasic not implemented for MsgRedeemRequest")
}

// ValidateBasic returns a not implemented error for MsgRedeemRequest.
func (m MsgUpdateParams) ValidateBasic() error {
	return errors.New("ValidateBasic not implemented for MsgUpdateParams")
}
