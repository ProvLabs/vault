package types

import (
	"errors"
	fmt "fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ValidateBasic returns a not implemented error for MsgCreateVaultRequest.
func (m MsgCreateVaultRequest) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(m.MarkerAddress)
	if err != nil {
		return fmt.Errorf("invalid marker address: %q: %w", m.MarkerAddress, err)
	}
	_, err = sdk.AccAddressFromBech32(m.Admin)
	if err != nil {
		return fmt.Errorf("invalid admin address: %q: %w", m.Admin, err)
	}
	return nil
}

// ValidateBasic returns a not implemented error for MsgDepositRequest.
func (m MsgDepositRequest) ValidateBasic() error {
	return errors.New("ValidateBasic not implemented for MsgDepositRequest")
}

// ValidateBasic returns a not implemented error for MsgWithdrawRequest.
func (m MsgWithdrawRequest) ValidateBasic() error {
	return errors.New("ValidateBasic not implemented for MsgWithdrawRequest")
}

// ValidateBasic returns a not implemented error for MsgRedeemRequest.
func (m MsgRedeemRequest) ValidateBasic() error {
	return errors.New("ValidateBasic not implemented for MsgRedeemRequest")
}

// ValidateBasic returns a not implemented error for MsgRedeemRequest.
func (m MsgUpdateParams) ValidateBasic() error {
	return errors.New("ValidateBasic not implemented for MsgUpdateParams")
}
