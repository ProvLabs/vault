package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/provenance-io/provenance/x/marker/types"
)

type MarkerKeeper interface {
	MintCoin(ctx sdk.Context, caller sdk.AccAddress, coin sdk.Coin) error
	BurnCoin(ctx sdk.Context, caller sdk.AccAddress, coin sdk.Coin) error
	AddFinalizeAndActivateMarker(ctx sdk.Context, marker types.MarkerAccountI) error
	TransferCoin(ctx sdk.Context, from, to, admin sdk.AccAddress, amount sdk.Coin) error
	WithdrawCoins(ctx sdk.Context, caller sdk.AccAddress, recipient sdk.AccAddress, denom string, coins sdk.Coins) error
	GetMarker(ctx sdk.Context, address sdk.AccAddress) (types.MarkerAccountI, error)
}
