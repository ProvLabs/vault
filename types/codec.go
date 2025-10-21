package types

import (
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	proto "github.com/cosmos/gogoproto/proto"
)

var (
	_ VaultAccountI            = (*VaultAccount)(nil)
	_ sdk.AccountI             = (*VaultAccount)(nil)
	_ authtypes.GenesisAccount = (*VaultAccount)(nil)
)

func RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	messages := make([]proto.Message, len(AllRequestMsgs))
	copy(messages, AllRequestMsgs)
	registry.RegisterImplementations((*sdk.Msg)(nil), messages...)

	registry.RegisterInterface(
		"provlabs.vault.v1.VaultAccountI",
		(*VaultAccountI)(nil),
	)

	registry.RegisterImplementations(
		(*VaultAccountI)(nil),
		&VaultAccount{},
	)

	registry.RegisterImplementations(
		(*sdk.AccountI)(nil),
		&VaultAccount{},
	)

	registry.RegisterImplementations(
		(*authtypes.GenesisAccount)(nil),
		&VaultAccount{},
	)

	msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)
}
