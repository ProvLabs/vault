package keeper

import (
	"errors"
	"fmt"

	"github.com/provlabs/vault/queue"
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
	cdc          codec.Codec
	storeService store.KVStoreService
	schema       collections.Schema
	eventService event.Service
	AddressCodec address.Codec
	authority    []byte

	AuthKeeper     types.AccountKeeper
	MarkerKeeper   types.MarkerKeeper
	BankKeeper     types.BankKeeper
	NameKeeper     types.NameKeeper
	AttrKeeper     types.AttributeKeeper
	ExchangeKeeper types.ExchangeKeeper
	HoldKeeper     types.HoldKeeper
	MetadataKeeper types.MetadataKeeper

	Params                collections.Item[types.Params]
	Vaults                collections.Map[sdk.AccAddress, []byte]
	AssetNAVs             collections.Map[collections.Pair[sdk.AccAddress, string], types.AssetNAV]
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
	exchangekeeper types.ExchangeKeeper,
	holdkeeper types.HoldKeeper,
	metadatakeeper types.MetadataKeeper,
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
		AssetNAVs:             collections.NewMap(builder, types.VaultAssetNAVPrefix, types.VaultAssetNAVName, collections.PairKeyCodec(sdk.AccAddressKey, collections.StringKey), codec.CollValue[types.AssetNAV](cdc)),
		PayoutVerificationSet: collections.NewKeySet(builder, types.VaultPayoutVerificationSetPrefix, types.VaultPayoutVerificationSetName, sdk.AccAddressKey),
		PayoutTimeoutQueue:    queue.NewPayoutTimeoutQueue(builder),
		FeeTimeoutQueue:       queue.NewFeeTimeoutQueue(builder),
		PendingSwapOutQueue:   queue.NewPendingSwapOutQueue(builder, cdc),
		AuthKeeper:            authKeeper,
		MarkerKeeper:          markerkeeper,
		BankKeeper:            bankkeeper,
		NameKeeper:            namekeeper,
		AttrKeeper:            attributekeeper,
		ExchangeKeeper:        exchangekeeper,
		HoldKeeper:            holdkeeper,
		MetadataKeeper:        metadatakeeper,
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
	if err != nil {
		if errors.Is(err, collections.ErrNotFound) {
			return types.GetDefaultTechFeeAddress(ctx.ChainID()), nil
		}
		return nil, fmt.Errorf("failed to retrieve params: %w", err)
	}

	if len(params.TechFeeAddress) == 0 {
		return types.GetDefaultTechFeeAddress(ctx.ChainID()), nil
	}

	addr, parseErr := k.AddressCodec.StringToBytes(params.TechFeeAddress)
	if parseErr != nil {
		return nil, fmt.Errorf("failed to parse AUM fee address from params %q: %w", params.TechFeeAddress, parseErr)
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
