package types

import (
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
)

// RegisterInterfaces registers the vault module's message types for protobuf interface compatibility.
func RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	messages := make([]proto.Message, len(AllRequestMsgs))
	copy(messages, AllRequestMsgs)
	registry.RegisterImplementations((*sdk.Msg)(nil), messages...)

	registry.RegisterInterface(
		"vault.v1.VaultAccount",
		(*sdk.AccountI)(nil),
		&VaultAccount{},
	)

	registry.RegisterInterface(
		"vault.v1.VaultAccount",
		(*authtypes.GenesisAccount)(nil),
		&VaultAccount{},
	)

	msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)
}


	msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)
}
