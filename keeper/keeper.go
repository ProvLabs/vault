package keeper

import (
	"fmt"

	"github.com/provlabs/vault/types"

	"cosmossdk.io/collections"
	"cosmossdk.io/core/address"
	"cosmossdk.io/core/event"
	"cosmossdk.io/core/store"
	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gogoproto/proto"
)

type Keeper struct {
	schema       collections.Schema
	eventService event.Service
	addressCodec address.Codec
	authority    []byte

	AuthKeeper   types.AccountKeeper
	MarkerKeeper types.MarkerKeeper
	BankKeeper   types.BankKeeper

	Vaults            collections.Map[sdk.AccAddress, []byte]
	VaultStartQueue   collections.Map[collections.Pair[uint64, sdk.AccAddress], collections.NoValue]
	VaultTimeoutQueue collections.Map[collections.Pair[uint64, sdk.AccAddress], collections.NoValue]
}

// NewMsgServer creates a new Keeper for the module.
func NewKeeper(
	cdc codec.Codec,
	storeService store.KVStoreService,
	eventService event.Service,
	addressCodec address.Codec,
	authority []byte,
	authKeeper types.AccountKeeper,
	markerkeeper types.MarkerKeeper,
	bankkeeper types.BankKeeper,
) *Keeper {
	if _, err := addressCodec.BytesToString(authority); err != nil {
		panic(fmt.Sprintf("invalid authority address %s: %s", authority, err))
	}

	builder := collections.NewSchemaBuilder(storeService)
	startKeyCodec := collections.PairKeyCodec(
		collections.Uint64Key,
		sdk.AccAddressKey,
	)
	endKeyCodec := collections.PairKeyCodec(
		collections.Uint64Key,
		sdk.AccAddressKey,
	)

	keeper := &Keeper{
		eventService:      eventService,
		addressCodec:      addressCodec,
		authority:         authority,
		Vaults:            collections.NewMap(builder, types.VaultsKeyPrefix, types.VaultsName, sdk.AccAddressKey, collections.BytesValue),
		VaultStartQueue:   collections.NewMap[collections.Pair[uint64, sdk.AccAddress], collections.NoValue](builder, types.VaultStartQueuePrefix, types.VaultStartQueueName, startKeyCodec, collections.NoValue{}),
		VaultTimeoutQueue: collections.NewMap[collections.Pair[uint64, sdk.AccAddress], collections.NoValue](builder, types.VaultEndQueuePrefix, types.VaultEndQueueName, endKeyCodec, collections.NoValue{}),
		AuthKeeper:        authKeeper,
		MarkerKeeper:      markerkeeper,
		BankKeeper:        bankkeeper,
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

// emitEvent is a helper function to emit an event using the keeper's event service.
// It logs an error if the event emission fails.
func (k Keeper) emitEvent(ctx sdk.Context, event proto.Message) {
	if err := k.eventService.EventManager(ctx).Emit(ctx, event); err != nil {
		k.getLogger(ctx).Error(fmt.Sprintf("error emitting event %#v: %v", event, err))
	}
}
