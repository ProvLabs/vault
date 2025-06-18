package keeper

import (
	"fmt"

	"cosmossdk.io/collections"
	"cosmossdk.io/core/address"
	"cosmossdk.io/core/event"
	"cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/provlabs/vault/types"
)

type Keeper struct {
	schema       collections.Schema
	eventService event.Service
	addressCodec address.Codec
	authority    []byte

	Params collections.Item[types.Params]
	Vaults collections.Map[uint32, types.Vault]
}

func NewKeeper(
	cdc codec.Codec,
	storeService store.KVStoreService,
	eventService event.Service,
	addressCodec address.Codec,
	authority []byte,
) *Keeper {
	if _, err := addressCodec.BytesToString(authority); err != nil {
		panic(fmt.Sprintf("invalid authority address %s: %s", authority, err))
	}

	builder := collections.NewSchemaBuilder(storeService)

	keeper := &Keeper{
		eventService: eventService,
		addressCodec: addressCodec,
		authority:    authority,
		Params:       collections.NewItem(builder, types.ParamsKey, "params", codec.CollValue[types.Params](cdc)),
		Vaults:       collections.NewMap(builder, collections.Prefix(types.VaultsKey), "vaults", collections.Uint32Key, codec.CollValue[types.Vault](cdc)),
	}

	schema, err := builder.Build()
	if err != nil {
		panic(err)
	}

	keeper.schema = schema
	return keeper
}

// GetAuthority returns the module's authority.
func (k Keeper) GetAuthority() []byte {
	return k.authority
}

// getLogger returns a logger with vault module context.
func (k Keeper) getLogger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", "x/"+types.ModuleName)
}
