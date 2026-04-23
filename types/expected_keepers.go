package types

import (
	context "context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	attrtypes "github.com/provenance-io/provenance/x/attribute/types"
	exchangetypes "github.com/provenance-io/provenance/x/exchange"
	markertypes "github.com/provenance-io/provenance/x/marker/types"
)

type MarkerKeeper interface {
	MintCoin(ctx sdk.Context, caller sdk.AccAddress, coin sdk.Coin) error
	BurnCoin(ctx sdk.Context, caller sdk.AccAddress, coin sdk.Coin) error
	AddFinalizeAndActivateMarker(ctx sdk.Context, marker markertypes.MarkerAccountI) error
	TransferCoin(ctx sdk.Context, from, to, admin sdk.AccAddress, amount sdk.Coin) error
	WithdrawCoins(ctx sdk.Context, caller sdk.AccAddress, recipient sdk.AccAddress, denom string, coins sdk.Coins) error
	GetMarker(ctx sdk.Context, address sdk.AccAddress) (markertypes.MarkerAccountI, error)
	GetMarkerByDenom(ctx sdk.Context, denom string) (markertypes.MarkerAccountI, error)
	IsMarkerAccount(ctx sdk.Context, addr sdk.AccAddress) bool
	GetNetAssetValue(ctx sdk.Context, markerDenom, priceDenom string) (*markertypes.NetAssetValue, error)
	SetNetAssetValue(ctx sdk.Context, marker markertypes.MarkerAccountI, netAssetValue markertypes.NetAssetValue, source string) error
	SendRestrictionFn(ctx context.Context, fromAddr, toAddr sdk.AccAddress, amt sdk.Coins) (sdk.AccAddress, error)
	GetUnrestrictedDenomRegex(ctx sdk.Context) (regex string)
}

type AccountKeeper interface {
	NewAccount(context.Context, sdk.AccountI) sdk.AccountI
	NewAccountWithAddress(ctx context.Context, addr sdk.AccAddress) sdk.AccountI

	GetAccount(ctx context.Context, addr sdk.AccAddress) sdk.AccountI
	GetAllAccounts(ctx context.Context) []sdk.AccountI
	HasAccount(ctx context.Context, addr sdk.AccAddress) bool
	SetAccount(ctx context.Context, acc sdk.AccountI)

	GetModuleAddress(moduleName string) sdk.AccAddress
}

type BankKeeper interface {
	SendCoins(context context.Context, fromAddr sdk.AccAddress, toAddr sdk.AccAddress, amt sdk.Coins) error
	GetAllBalances(context context.Context, addr sdk.AccAddress) sdk.Coins
	GetBalance(context context.Context, addr sdk.AccAddress, denom string) sdk.Coin
	GetSupply(context context.Context, denom string) sdk.Coin
	MintCoins(ctx context.Context, moduleName string, amt sdk.Coins) error
	SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
	SetDenomMetaData(context context.Context, denomMetaData banktypes.Metadata)
}

type NameKeeper interface {
	SetNameRecord(ctx sdk.Context, name string, addr sdk.AccAddress, restrict bool) error
	NameExists(ctx sdk.Context, name string) bool
}

type AttributeKeeper interface {
	SetAttribute(ctx sdk.Context, attr attrtypes.Attribute, owner sdk.AccAddress) error
}

// HoldKeeper provides a read-only view of assets that are currently "locked"
// in an account without being moved. The vault module uses this to ensure that
// assets committed to pending P2P trades are still included in Total Vault Value
// (TVV) calculations and liquidity breakdowns.
type HoldKeeper interface {
	// GetHoldCoin returns the amount of a specific denom that is currently on hold for an address.
	GetHoldCoin(ctx sdk.Context, addr sdk.AccAddress, denom string) (sdk.Coin, error)
	// GetAllHolds returns all coins currently on hold for a specific address.
	GetAllHolds(ctx sdk.Context, addr sdk.AccAddress) (sdk.Coins, error)
}

// ExchangeKeeper defines the narrowed x/exchange interface used by the vault
// to coordinate the lifecycle of peer-to-peer (P2P) payments. It exposes both
// message server endpoints for initiating/settling trades and query endpoints
// for retrieving payment details.
type ExchangeKeeper interface {
	// CreatePayment initiates a new P2P payment proposal.
	CreatePayment(ctx context.Context, req *exchangetypes.MsgCreatePaymentRequest) (*exchangetypes.MsgCreatePaymentResponse, error)
	// AcceptPayment finalizes an incoming payment proposal, triggering asset movement.
	AcceptPayment(ctx context.Context, req *exchangetypes.MsgAcceptPaymentRequest) (*exchangetypes.MsgAcceptPaymentResponse, error)
	// RejectPayment declines a pending payment proposal targeting the caller.
	RejectPayment(ctx context.Context, req *exchangetypes.MsgRejectPaymentRequest) (*exchangetypes.MsgRejectPaymentResponse, error)
	// GetPayment retrieves the details of a specific payment by its source and external ID.
	GetPayment(ctx context.Context, req *exchangetypes.QueryGetPaymentRequest) (*exchangetypes.QueryGetPaymentResponse, error)
}
