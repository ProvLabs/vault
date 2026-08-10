package keeper

import (
	"errors"
	"fmt"

	"github.com/provlabs/vault/queue"
	"github.com/provlabs/vault/types"

	"cosmossdk.io/collections"
	collcodec "cosmossdk.io/collections/codec"
	"cosmossdk.io/core/address"
	"cosmossdk.io/core/event"
	"cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"cosmossdk.io/math"

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
	// authorityString is the bech32 form of authority, encoded once with AddressCodec.
	authorityString string

	AuthKeeper          types.AccountKeeper
	MarkerKeeper        types.MarkerKeeper
	MetadataKeeper      types.MetadataKeeper
	BankKeeper          types.BankKeeper
	NameKeeper          types.NameKeeper
	AttrKeeper          types.AttributeKeeper
	ExchangeKeeper      types.ExchangeKeeper
	ExchangeQueryServer types.ExchangeQueryServer

	// Params holds the module-wide parameters.
	Params collections.Item[types.Params]
	// Vaults indexes every vault address; the vault itself lives in the auth account store.
	Vaults collections.Map[sdk.AccAddress, []byte]
	// NAVs prices each denom a vault holds, keyed by vault address and denom.
	NAVs collections.Map[collections.Pair[sdk.AccAddress, string], types.VaultNAV]
	// TotalValues materializes each vault's total value in its underlying asset.
	TotalValues collections.Map[sdk.AccAddress, math.Int]
	// PayoutVerificationSet holds the vaults awaiting a payout verification sweep, each entry doubling
	// as that vault's retry token.
	PayoutVerificationSet collections.KeySet[sdk.AccAddress]
	// PayoutVerificationCursor is the address the next payout verification sweep resumes after.
	PayoutVerificationCursor collections.Item[sdk.AccAddress]
	// PayoutTimeoutQueue schedules the vaults due for an interest payout.
	PayoutTimeoutQueue *queue.PayoutTimeoutQueue
	// FeeTimeoutQueue schedules the vaults due for a fee collection.
	FeeTimeoutQueue *queue.FeeTimeoutQueue
	// PendingSwapOutQueue holds redemptions waiting out their configured delay.
	PendingSwapOutQueue *queue.PendingSwapOutQueue
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
	metadatakeeper types.MetadataKeeper,
	bankkeeper types.BankKeeper,
	namekeeper types.NameKeeper,
	attributekeeper types.AttributeKeeper,
	exchangekeeper types.ExchangeKeeper,
	exchangeQueryServer types.ExchangeQueryServer,
) *Keeper {
	authorityString, err := addressCodec.BytesToString(authority)
	if err != nil {
		panic(fmt.Sprintf("invalid authority address %s: %s", authority, err))
	}

	builder := collections.NewSchemaBuilder(storeService)

	keeper := &Keeper{
		cdc:                      cdc,
		storeService:             storeService,
		eventService:             eventService,
		AddressCodec:             addressCodec,
		authority:                authority,
		authorityString:          authorityString,
		Params:                   collections.NewItem(builder, types.ParamsKeyPrefix, types.ParamsKeyName, codec.CollValue[types.Params](cdc)),
		Vaults:                   collections.NewMap(builder, types.VaultsKeyPrefix, types.VaultsName, sdk.AccAddressKey, collections.BytesValue),
		NAVs:                     collections.NewMap(builder, types.NAVsKeyPrefix, types.NAVsName, collections.PairKeyCodec(sdk.AccAddressKey, collections.StringKey), codec.CollValue[types.VaultNAV](cdc)),
		TotalValues:              collections.NewMap(builder, types.TotalValuesKeyPrefix, types.TotalValuesName, sdk.AccAddressKey, sdk.IntValue),
		PayoutVerificationSet:    collections.NewKeySet(builder, types.VaultPayoutVerificationSetPrefix, types.VaultPayoutVerificationSetName, sdk.AccAddressKey),
		PayoutVerificationCursor: collections.NewItem(builder, types.VaultPayoutVerificationCursorPrefix, types.VaultPayoutVerificationCursorName, collcodec.KeyToValueCodec(sdk.AccAddressKey)),
		PayoutTimeoutQueue:       queue.NewPayoutTimeoutQueue(builder),
		FeeTimeoutQueue:          queue.NewFeeTimeoutQueue(builder),
		PendingSwapOutQueue:      queue.NewPendingSwapOutQueue(builder, cdc),
		AuthKeeper:               authKeeper,
		MarkerKeeper:             markerkeeper,
		MetadataKeeper:           metadatakeeper,
		BankKeeper:               bankkeeper,
		NameKeeper:               namekeeper,
		AttrKeeper:               attributekeeper,
		ExchangeKeeper:           exchangekeeper,
		ExchangeQueryServer:      exchangeQueryServer,
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

// GetAuthorityString returns the module's authority as a bech32 address.
func (k Keeper) GetAuthorityString() string {
	return k.authorityString
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

// IsVaultCreationGovOnly reports whether CreateVault may only be signed by the governance
// module account. Unset params fall back to the module default; any other read failure is
// surfaced so the gate never fails open on an unreadable store.
func (k Keeper) IsVaultCreationGovOnly(ctx sdk.Context) (bool, error) {
	params, err := k.Params.Get(ctx)
	if err != nil {
		if errors.Is(err, collections.ErrNotFound) {
			return types.DefaultParams().GovOnlyVaultCreation, nil
		}
		return false, fmt.Errorf("failed to retrieve params: %w", err)
	}

	return params.GovOnlyVaultCreation, nil
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
