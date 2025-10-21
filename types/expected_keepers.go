package types

import (
	context "context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	attrtypes "github.com/provenance-io/provenance/x/attribute/types"
	"github.com/provenance-io/provenance/x/marker/types"
)

type MarkerKeeper interface {
	MintCoin(ctx sdk.Context, caller sdk.AccAddress, coin sdk.Coin) error
	BurnCoin(ctx sdk.Context, caller sdk.AccAddress, coin sdk.Coin) error
	AddFinalizeAndActivateMarker(ctx sdk.Context, marker types.MarkerAccountI) error
	TransferCoin(ctx sdk.Context, from, to, admin sdk.AccAddress, amount sdk.Coin) error
	WithdrawCoins(ctx sdk.Context, caller sdk.AccAddress, recipient sdk.AccAddress, denom string, coins sdk.Coins) error
	GetMarker(ctx sdk.Context, address sdk.AccAddress) (types.MarkerAccountI, error)
	GetMarkerByDenom(ctx sdk.Context, denom string) (types.MarkerAccountI, error)
	IsMarkerAccount(ctx sdk.Context, addr sdk.AccAddress) bool
	GetNetAssetValue(ctx sdk.Context, markerDenom, priceDenom string) (*types.NetAssetValue, error)
	SetNetAssetValue(ctx sdk.Context, marker types.MarkerAccountI, netAssetValue types.NetAssetValue, source string) error
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
}

type NameKeeper interface {
	SetNameRecord(ctx sdk.Context, name string, addr sdk.AccAddress, restrict bool) error
}

type AttributeKeeper interface {
	SetAttribute(ctx sdk.Context, attr attrtypes.Attribute, owner sdk.AccAddress) error
}

