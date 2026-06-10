package keeper

import (
	"fmt"

	"github.com/provlabs/vault/types"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/provenance-io/provenance/x/exchange"
)

// settlementLegCoins resolves a payment's legs into the single asset coin and the
// single payment-denom coin for the given settlement direction. The NAV guardrail
// and upsert price exactly one asset coin against one payment coin, so a leg with
// zero or multiple coins is rejected.
func settlementLegCoins(payment *exchange.Payment, direction string) (assetCoin, paymentCoin sdk.Coin, err error) {
	assetLeg, paymentLeg := payment.TargetAmount, payment.SourceAmount
	if direction == types.AssetDirectionInbound {
		assetLeg, paymentLeg = payment.SourceAmount, payment.TargetAmount
	}
	if len(assetLeg) != 1 || len(paymentLeg) != 1 {
		return sdk.Coin{}, sdk.Coin{}, fmt.Errorf("payment legs must each carry exactly one coin to settle against the vault NAV: source_amount=%q target_amount=%q", payment.SourceAmount, payment.TargetAmount)
	}
	return assetLeg[0], paymentLeg[0], nil
}
