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
	schema       collections.Schema
	eventService event.Service
	addressCodec address.Codec
	authority    []byte

	AuthKeeper   types.AccountKeeper
	MarkerKeeper types.MarkerKeeper
	BankKeeper   types.BankKeeper
	ExchangeKeeper types.ExchangeKeeper

	AUMFeeAddress         collections.Item[sdk.AccAddress]
	Vaults                collections.Map[sdk.AccAddress, []byte]
	AssetNAV              collections.Map[collections.Pair[sdk.AccAddress, string], types.AssetNAV]
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
	exchangekeeper types.ExchangeKeeper,
) *Keeper {
	if _, err := addressCodec.BytesToString(authority); err != nil {
		panic(fmt.Sprintf("invalid authority address %s: %s", authority, err))
	}

	builder := collections.NewSchemaBuilder(storeService)

	keeper := &Keeper{
		eventService:          eventService,
		addressCodec:          addressCodec,
		authority:             authority,
		AUMFeeAddress:         collections.NewItem(builder, types.AUMFeeAddressKeyPrefix, types.AUMFeeAddressKeyName, collcodec.KeyToValueCodec(sdk.AccAddressKey)),
		Vaults:                collections.NewMap(builder, types.VaultsKeyPrefix, types.VaultsName, sdk.AccAddressKey, collections.BytesValue),
		AssetNAV:              collections.NewMap(builder, types.AssetNAVKeyPrefix, types.AssetNAVName, collections.PairKeyCodec(sdk.AccAddressKey, collections.StringKey), codec.CollValue[types.AssetNAV](cdc)),
		PayoutVerificationSet: collections.NewKeySet(builder, types.VaultPayoutVerificationSetPrefix, types.VaultPayoutVerificationSetName, sdk.AccAddressKey),
		PayoutTimeoutQueue:    queue.NewPayoutTimeoutQueue(builder),
		FeeTimeoutQueue:       queue.NewFeeTimeoutQueue(builder),
		PendingSwapOutQueue:   queue.NewPendingSwapOutQueue(builder, cdc),
		AuthKeeper:            authKeeper,
		MarkerKeeper:          markerkeeper,
		BankKeeper:            bankkeeper,
		ExchangeKeeper:        exchangekeeper,
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

// GetAUMFeeAddress returns the address where AUM fees are collected.
// It prioritizes the address stored in state, falling back to the hardcoded default.
func (k Keeper) GetAUMFeeAddress(ctx sdk.Context) sdk.AccAddress {
	addr, err := k.AUMFeeAddress.Get(ctx)
	if err == nil && len(addr) > 0 {
		return addr
	}
	return types.AUMFeeAddress
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
