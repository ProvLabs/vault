package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
)

// RegisterLegacyAminoCodec registers the vault module's concrete message types for Amino.
func RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgUpdateParams{}, "vault/UpdateParams", nil)
	cdc.RegisterConcrete(&MsgCreateVaultRequest{}, "vault/CreateVault", nil)
	cdc.RegisterConcrete(&MsgSwapInRequest{}, "vault/SwapIn", nil)
	cdc.RegisterConcrete(&MsgSwapOutRequest{}, "vault/SwapOut", nil)
	cdc.RegisterConcrete(&MsgSetInterestConfigRequest{}, "vault/SetInterestConfig", nil)
	cdc.RegisterConcrete(&MsgUpdateInterestRateRequest{}, "vault/UpdateInterestRate", nil)
	cdc.RegisterConcrete(&MsgDepositInterestFundsRequest{}, "vault/DepositInterestFunds", nil)
	cdc.RegisterConcrete(&MsgWithdrawInterestFundsRequest{}, "vault/WithdrawInterestFunds", nil)
	cdc.RegisterConcrete(&MsgToggleSwapsRequest{}, "vault/ToggleSwaps", nil)
}

// RegisterInterfaces registers the vault module's message types for protobuf interface compatibility.
func RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgUpdateParams{},
		&MsgCreateVaultRequest{},
		&MsgSwapInRequest{},
		&MsgSwapOutRequest{},
		&MsgSetInterestConfigRequest{},
		&MsgUpdateInterestRateRequest{},
		&MsgDepositInterestFundsRequest{},
		&MsgWithdrawInterestFundsRequest{},
		&MsgToggleSwapsRequest{},
	)

	msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)
}

var amino = codec.NewLegacyAmino()

func init() {
	RegisterLegacyAminoCodec(amino)
	amino.Seal()
}
