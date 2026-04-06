package keeper

import (
	"fmt"

	"github.com/provlabs/vault/queue"
	"github.com/provlabs/vault/types"

	"cosmossdk.io/collections"
	collcodec "cosmossdk.io/collections/codec"
	"cosmossdk.io/core/address"
	"cosmossdk.io/core/event"
	"cosmossdk.io/core/store"
	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gogoproto/proto"
)

type Keeper struct {
	cdc          codec.Codec
	storeService store.KVStoreService
	schema       collections.Schema
	eventService event.Service
	AddressCodec address.Codec
	authority    []byte

	AuthKeeper   types.AccountKeeper
	MarkerKeeper types.MarkerKeeper
	BankKeeper   types.BankKeeper
	NameKeeper   types.NameKeeper
	AttrKeeper   types.AttributeKeeper

	Params                collections.Item[types.Params]
	Vaults                collections.Map[sdk.AccAddress, []byte]
	PayoutVerificationSet collections.KeySet[sdk.AccAddress]
	PayoutTimeoutQueue    *queue.PayoutTimeoutQueue
	FeeTimeoutQueue       *queue.FeeTimeoutQueue
	PendingSwapOutQueue   *queue.PendingSwapOutQueue
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
	namekeeper types.NameKeeper,
	attributekeeper types.AttributeKeeper,
) *Keeper {
	if _, err := addressCodec.BytesToString(authority); err != nil {
		panic(fmt.Sprintf("invalid authority address %s: %s", authority, err))
	}

	builder := collections.NewSchemaBuilder(storeService)

	keeper := &Keeper{
		cdc:                   cdc,
		storeService:          storeService,
		eventService:          eventService,
		AddressCodec:          addressCodec,
		authority:             authority,
		Params:                collections.NewItem(builder, types.ParamsKeyPrefix, types.ParamsKeyName, codec.CollValue[types.Params](cdc)),
		Vaults:                collections.NewMap(builder, types.VaultsKeyPrefix, types.VaultsName, sdk.AccAddressKey, collections.BytesValue),
		PayoutVerificationSet: collections.NewKeySet(builder, types.VaultPayoutVerificationSetPrefix, types.VaultPayoutVerificationSetName, sdk.AccAddressKey),
		PayoutTimeoutQueue:    queue.NewPayoutTimeoutQueue(builder),
		FeeTimeoutQueue:       queue.NewFeeTimeoutQueue(builder),
		PendingSwapOutQueue:   queue.NewPendingSwapOutQueue(builder, cdc),
		AuthKeeper:            authKeeper,
		MarkerKeeper:          markerkeeper,
		BankKeeper:            bankkeeper,
		NameKeeper:            namekeeper,
		AttrKeeper:            attributekeeper,
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

// OpenKVStore returns a KVStore for the module.
func (k Keeper) OpenKVStore(ctx sdk.Context) store.KVStore {
	return k.storeService.OpenKVStore(ctx)
}

// GetAUMFeeAddress returns the address where AUM fees are collected.
func (k Keeper) GetAUMFeeAddress(ctx sdk.Context) (sdk.AccAddress, error) {
	params, err := k.Params.Get(ctx)
	if err != nil || len(params.TechFeeAddress) == 0 {
		return types.GetDefaultTechFeeAddress(ctx.ChainID()), err
	}
	addr, parseErr := sdk.AccAddressFromBech32(params.TechFeeAddress)
	if parseErr != nil {
		return types.GetDefaultTechFeeAddress(ctx.ChainID()), fmt.Errorf("invalid AUM fee address in params %q: %w", params.TechFeeAddress, parseErr)
	}
	return addr, nil
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
