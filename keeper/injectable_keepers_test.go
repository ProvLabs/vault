package keeper_test

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/provlabs/vault/types"
)

// faultBankKeeper wraps a real types.BankKeeper and lets a test force a targeted
// SendCoins call to fail. All other methods delegate to the embedded keeper. It
// exists so tests can deterministically trigger the critical-failure branches in
// the swap-out payout and refund paths without manipulating real chain state.
type faultBankKeeper struct {
	types.BankKeeper
	failSendCoins func(from, to sdk.AccAddress, amt sdk.Coins) error
}

// SendCoins returns the configured failure when the predicate matches the call;
// otherwise it delegates to the wrapped keeper.
func (f faultBankKeeper) SendCoins(ctx context.Context, from, to sdk.AccAddress, amt sdk.Coins) error {
	if f.failSendCoins != nil {
		if err := f.failSendCoins(from, to, amt); err != nil {
			return err
		}
	}
	return f.BankKeeper.SendCoins(ctx, from, to, amt)
}

// faultMarkerKeeper wraps a real types.MarkerKeeper and lets a test force BurnCoin
// to fail. All other methods delegate to the embedded keeper. It exists so tests
// can trigger the critical burn-failure branch of processSingleWithdrawal.
type faultMarkerKeeper struct {
	types.MarkerKeeper
	failBurnCoin func(caller sdk.AccAddress, coin sdk.Coin) error
}

// BurnCoin returns the configured failure when the predicate matches the call;
// otherwise it delegates to the wrapped keeper.
func (f faultMarkerKeeper) BurnCoin(ctx sdk.Context, caller sdk.AccAddress, coin sdk.Coin) error {
	if f.failBurnCoin != nil {
		if err := f.failBurnCoin(caller, coin); err != nil {
			return err
		}
	}
	return f.MarkerKeeper.BurnCoin(ctx, caller, coin)
}

// installBankSendFault replaces the suite keeper's BankKeeper with a fault wrapper
// that fails any SendCoins for which fail returns a non-nil error.
func (s *TestSuite) installBankSendFault(fail func(from, to sdk.AccAddress, amt sdk.Coins) error) {
	s.k.BankKeeper = faultBankKeeper{BankKeeper: s.k.BankKeeper, failSendCoins: fail}
}

// installMarkerBurnFault replaces the suite keeper's MarkerKeeper with a fault
// wrapper that fails any BurnCoin for which fail returns a non-nil error.
func (s *TestSuite) installMarkerBurnFault(fail func(caller sdk.AccAddress, coin sdk.Coin) error) {
	s.k.MarkerKeeper = faultMarkerKeeper{MarkerKeeper: s.k.MarkerKeeper, failBurnCoin: fail}
}
