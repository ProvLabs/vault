package keeper

import (
	"fmt"

	"github.com/provlabs/vault/types"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/provenance-io/provenance/x/exchange"
	markertypes "github.com/provenance-io/provenance/x/marker/types"
)

// stageFromPrincipal moves coins from the vault's own principal marker into the vault
// account, for example to fund the asset leg of an outbound settlement before AcceptPayment
// pays it out. A zero amount is a no-op.
//
// Both endpoints are derived from the vault, so the transfer is always an internal hop
// between the vault and its own principal marker, and the marker send restriction is
// bypassed: the vault owns the marker, and restricted-marker rules (transfer access,
// required attributes) are enforced at the vault's boundary by AcceptPayment, which escrows
// assets in (source -> vault) and pays them out (vault -> source) under the send restriction.
// Once a settlement has cleared that boundary, this internal hop should not be restricted again.
func (k *Keeper) stageFromPrincipal(ctx sdk.Context, vault *types.VaultAccount, amt sdk.Coins) error {
	if amt.IsZero() {
		return nil
	}
	return k.BankKeeper.SendCoins(markertypes.WithBypass(ctx), vault.PrincipalMarkerAddress(), vault.GetAddress(), amt)
}

// returnToPrincipal moves coins from the vault account into its own principal marker, for
// example to stow the funds received from a settlement. A zero amount is a no-op. Both
// endpoints are derived from the vault and the marker send restriction is bypassed for the
// same reason as stageFromPrincipal.
func (k *Keeper) returnToPrincipal(ctx sdk.Context, vault *types.VaultAccount, amt sdk.Coins) error {
	if amt.IsZero() {
		return nil
	}
	return k.BankKeeper.SendCoins(markertypes.WithBypass(ctx), vault.GetAddress(), vault.PrincipalMarkerAddress(), amt)
}

// removeDrainedSettlementNAV drops the vault's internal NAV entry for an asset denom
// that an outbound settlement has just emptied out of the principal marker, so a stale
// price cannot linger for an asset the vault no longer holds. Reacquiring the denom
// then requires a fresh price from the NAV authority. The published marker NAV is left
// as-is (publishing simply stops).
//
// Settling never writes the internal NAV table. checkSettlementNAVGuardrail has already
// proven the trade executed at exactly the price the NAV authority recorded, so a
// settlement has no new price to contribute, and leaving the entry untouched keeps the
// price attributed to the authority that set it rather than to the asset manager that
// traded against it.
func (k *Keeper) removeDrainedSettlementNAV(ctx sdk.Context, vault *types.VaultAccount, assetDenom, direction string) error {
	if direction != types.AssetDirectionOutbound {
		return nil
	}
	if !k.BankKeeper.GetBalance(ctx, vault.PrincipalMarkerAddress(), assetDenom).IsZero() {
		return nil
	}
	if err := k.RemoveVaultNAV(ctx, vault, assetDenom, ""); err != nil {
		return fmt.Errorf("failed to remove internal NAV for drained denom %q: %w", assetDenom, err)
	}
	return nil
}

// settlementLegCoins resolves a payment's legs into the single asset coin and the
// single underlying-asset coin for the given settlement direction. The NAV guardrail
// prices exactly one asset coin against one payment coin, so the asset leg with zero
// or multiple coins is rejected. A zero-priced settlement carries no coin on
// the payment leg (the zero coin is stripped); an empty payment leg yields a zero coin
// of underlyingDenom, but a payment leg carrying more than one coin is rejected.
func settlementLegCoins(payment *exchange.Payment, direction, underlyingDenom string) (assetCoin, paymentCoin sdk.Coin, err error) {
	assetLeg, paymentLeg := payment.TargetAmount, payment.SourceAmount
	if direction == types.AssetDirectionInbound {
		assetLeg, paymentLeg = payment.SourceAmount, payment.TargetAmount
	}
	if len(assetLeg) != 1 || len(paymentLeg) > 1 {
		return sdk.Coin{}, sdk.Coin{}, fmt.Errorf("payment legs must carry one asset coin and at most one payment coin to settle against the vault NAV: source_amount=%q target_amount=%q", payment.SourceAmount, payment.TargetAmount)
	}
	paymentCoin = sdk.NewInt64Coin(underlyingDenom, 0)
	if len(paymentLeg) == 1 {
		paymentCoin = paymentLeg[0]
	}
	return assetLeg[0], paymentCoin, nil
}
