package keeper

import (
	"fmt"

	"github.com/provlabs/vault/types"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/provenance-io/provenance/x/exchange"
)

// applySettlementNAV records a settlement's price in the vault's internal NAV table:
// the asset denom's entry is upserted with the single-sided settlement price
// (paymentCoin per assetCoin.Amount units), sourced to the vault itself. When an
// outbound settlement leaves the principal marker holding none of the asset denom,
// the entry is removed so a stale price cannot linger for an asset the vault no
// longer holds.
func (k *Keeper) applySettlementNAV(ctx sdk.Context, vault *types.VaultAccount, assetCoin, paymentCoin sdk.Coin, direction, signer string) error {
	nav := types.NewVaultNAV(assetCoin.Denom, paymentCoin, assetCoin.Amount, vault.Address)
	if err := k.SetVaultNAV(ctx, vault, nav, signer); err != nil {
		return fmt.Errorf("failed to update internal NAV from settlement: %w", err)
	}

	if direction != types.AssetDirectionOutbound {
		return nil
	}
	if !k.BankKeeper.GetBalance(ctx, vault.PrincipalMarkerAddress(), assetCoin.Denom).IsZero() {
		return nil
	}
	if err := k.RemoveVaultNAV(ctx, vault, assetCoin.Denom); err != nil {
		return fmt.Errorf("failed to remove internal NAV for drained denom %q: %w", assetCoin.Denom, err)
	}
	return nil
}

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
