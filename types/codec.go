package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
)

func RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgUpdateParams{}, "vault/UpdateParams", nil)
	cdc.RegisterConcrete(&MsgCreateVaultRequest{}, "vault/CreateVault", nil)
	cdc.RegisterConcrete(&MsgSwapInRequest{}, "vault/SwapIn", nil)
	cdc.RegisterConcrete(&MsgSwapOutRequest{}, "vault/SwapOut", nil)
	cdc.RegisterConcrete(&MsgRedeemRequest{}, "vault/Redeem", nil)
}

func RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgUpdateParams{},
		&MsgCreateVaultRequest{},
		&MsgSwapInRequest{},
		&MsgSwapOutRequest{},
		&MsgRedeemRequest{})

	msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)
}

var amino = codec.NewLegacyAmino()

func init() {
	RegisterLegacyAminoCodec(amino)
	amino.Seal()
}
