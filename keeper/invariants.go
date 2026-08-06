// This file targets the SDK's invariant framework, which is deprecated along with x/crisis but
// is still the only wiring available for module invariants. Delete it once x/crisis is removed.
//
//nolint:staticcheck // SA1019: sdk.Invariant, sdk.InvariantRegistry and sdk.FormatInvariant are all deprecated with x/crisis.
package keeper

import (
	"fmt"
	"strings"

	"github.com/provlabs/vault/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Crisis-module route names for the vault module's invariants.
const (
	TotalValueInvariantRoute     = "total-value"
	ShareSupplyInvariantRoute    = "share-supply"
	EscrowedSharesInvariantRoute = "escrowed-shares"
)

// invariantChecks is the one list of the module's invariants, so registration and AllInvariants
// cannot disagree about which ones exist.
var invariantChecks = []struct {
	route string
	check vaultCheck
}{
	{TotalValueInvariantRoute, checkTotalValue},
	{ShareSupplyInvariantRoute, checkShareSupply},
	{EscrowedSharesInvariantRoute, checkEscrowedShares},
}

// RegisterInvariants registers the vault module's invariants.
func RegisterInvariants(ir sdk.InvariantRegistry, k Keeper) {
	for _, ic := range invariantChecks {
		ir.RegisterRoute(types.ModuleName, ic.route, vaultInvariant(k, ic.route, ic.check))
	}
}

// AllInvariants runs every vault module invariant, stopping at the first one broken.
func AllInvariants(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		for _, ic := range invariantChecks {
			if msg, broken := vaultInvariant(k, ic.route, ic.check)(ctx); broken {
				return msg, true
			}
		}
		return sdk.FormatInvariant(types.ModuleName, "all", "all vault invariants hold"), false
	}
}

// TotalValueInvariant checks each vault's materialized total value against the value derived by
// walking its NAV table. A mismatch means some path moved a priced balance or changed a price
// without reporting it, so share pricing is running off a stale number.
func TotalValueInvariant(k Keeper) sdk.Invariant {
	return vaultInvariant(k, TotalValueInvariantRoute, checkTotalValue)
}

// ShareSupplyInvariant checks that no vault's minted share supply exceeds its TotalShares.
//
// TotalShares is the cross-chain supply-of-record: bridge mints are gated on the headroom between
// it and local supply, and a bridge burn deliberately leaves it untouched because those shares
// still exist on the remote chain. Local supply overtaking it means that headroom is corrupt and
// the bridge can mint shares nothing backs.
func ShareSupplyInvariant(k Keeper) sdk.Invariant {
	return vaultInvariant(k, ShareSupplyInvariantRoute, checkShareSupply)
}

// EscrowedSharesInvariant checks that a vault holds at least the shares its pending swap-outs claim
// to have escrowed. A shortfall means a payout can no longer be honored from escrow.
//
// A surplus is not a violation: share transfers to the vault account are unrestricted, so anyone can
// create one with a bank send, and halting on that would be externally triggerable.
func EscrowedSharesInvariant(k Keeper) sdk.Invariant {
	return vaultInvariant(k, EscrowedSharesInvariantRoute, checkEscrowedShares)
}

// vaultCheck reports how one vault violates an invariant, or an empty violation when it holds. It
// separately reports why a vault could not be checked at all, which is neither: staying silent
// there would let an uncheckable vault read as healthy. Implementations only read state.
type vaultCheck func(ctx sdk.Context, k Keeper, vault types.VaultAccount) (violation, uncheckable string)

// vaultInvariant builds an invariant that runs check against every stored vault and accumulates a
// line per violation, so one run reports every offending vault rather than only the first.
//
// Lookup entries are resolved by resolveVaults, so check only sees live vaults. Unresolvable
// entries and uncheckable vaults land in the message rather than breaking the invariant, so
// neither can halt a chain over state that reporting cannot repair.
func vaultInvariant(k Keeper, route string, check vaultCheck) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		var violations, uncheckable []string

		vaults, skipped, err := k.resolveVaults(ctx, route+" invariant")
		if err != nil {
			return sdk.FormatInvariant(types.ModuleName, route, fmt.Sprintf("failed to list vaults: %v", err)), true
		}

		for _, vault := range vaults {
			violation, reason := check(ctx, k, vault)
			if violation != "" {
				violations = append(violations, violation)
			}
			if reason != "" {
				uncheckable = append(uncheckable, reason)
			}
		}

		return invariantMessage(route, len(vaults), skipped, violations, uncheckable)
	}
}

// invariantMessage renders one run's outcome. Only violations break the invariant; unresolvable
// lookup entries and uncheckable vaults ride along so a passing run still says what it missed.
func invariantMessage(route string, checked, skipped int, violations, uncheckable []string) (string, bool) {
	var msg strings.Builder

	if len(violations) == 0 {
		fmt.Fprintf(&msg, "all %d vaults pass the %s check", checked, route)
	} else {
		fmt.Fprintf(&msg, "found %d %s violation(s):\n%s", len(violations), route, strings.Join(violations, "\n"))
	}

	if skipped > 0 {
		fmt.Fprintf(&msg, "\n%d unresolvable lookup entries skipped", skipped)
	}

	if len(uncheckable) > 0 {
		fmt.Fprintf(&msg, "\n%d vault(s) could not be checked:\n%s", len(uncheckable), strings.Join(uncheckable, "\n"))
	}

	return sdk.FormatInvariant(types.ModuleName, route, msg.String()), len(violations) > 0
}

// checkTotalValue compares a vault's stored total value against the walk, reporting rather than
// repairing, because an invariant must not mutate state. A vault whose value cannot be derived is
// uncheckable, not broken: drift cannot be shown either way, and halting would not repair the NAV
// table at fault. The report names what is stored, because nothing stored means no TVV read works.
func checkTotalValue(ctx sdk.Context, k Keeper, vault types.VaultAccount) (string, string) {
	addr := vault.GetAddress()

	stored, found, err := k.storedTotalValue(ctx, vault)
	if err != nil {
		return fmt.Sprintf("\tvault %s: failed to read materialized total value: %v", addr, err), ""
	}

	derived, err := k.WalkTotalValue(ctx, vault)
	if err != nil {
		storedDescription := storedTotalDescription(stored, found)
		k.getLogger(ctx).Error("cannot evaluate the total-value invariant for vault",
			"vault", addr.String(),
			"stored", storedDescription,
			"err", err,
		)
		return "", fmt.Sprintf("\tvault %s: total value cannot be derived, %s: %v", addr, storedDescription, err)
	}

	if !found {
		return fmt.Sprintf("\tvault %s: no materialized total value stored (derived %s)", addr, derived), ""
	}

	if stored.IsNegative() {
		return fmt.Sprintf("\tvault %s: materialized total value %s is negative", addr, stored), ""
	}

	if !stored.Equal(derived) {
		return fmt.Sprintf("\tvault %s: materialized total value %s != derived %s (drift %s)",
			addr, stored, derived, stored.Sub(derived)), ""
	}

	return "", ""
}

// storedTotalDescription renders what a vault has materialized, for reports about a vault whose
// derived value is unavailable to compare it against.
func storedTotalDescription(stored math.Int, found bool) string {
	if !found {
		return "no materialized total value stored"
	}
	return fmt.Sprintf("materialized total value is %s", stored)
}

// checkShareSupply compares a vault's local share supply against its TotalShares ceiling.
func checkShareSupply(ctx sdk.Context, k Keeper, vault types.VaultAccount) (string, string) {
	shareDenom := vault.TotalShares.Denom
	supply := k.BankKeeper.GetSupply(ctx, shareDenom).Amount

	if vault.TotalShares.Amount.IsNegative() {
		return fmt.Sprintf("\tvault %s: total shares %s is negative", vault.GetAddress(), vault.TotalShares), ""
	}

	if supply.GT(vault.TotalShares.Amount) {
		return fmt.Sprintf("\tvault %s: share supply %s%s exceeds total shares %s by %s",
			vault.GetAddress(), supply, shareDenom, vault.TotalShares.Amount, supply.Sub(vault.TotalShares.Amount)), ""
	}

	return "", ""
}

// checkEscrowedShares reports a vault holding fewer shares than its pending swap-outs account for.
func checkEscrowedShares(ctx sdk.Context, k Keeper, vault types.VaultAccount) (string, string) {
	addr := vault.GetAddress()
	shareDenom := vault.TotalShares.Denom

	claimed := math.ZeroInt()
	err := k.PendingSwapOutQueue.WalkByVault(ctx, addr, func(_ int64, id uint64, req types.PendingSwapOut) (bool, error) {
		if req.Shares.Denom != shareDenom {
			return true, fmt.Errorf("request %d escrows %s but the vault's share denom is %s", id, req.Shares, shareDenom)
		}
		claimed = claimed.Add(req.Shares.Amount)
		return false, nil
	})
	if err != nil {
		return fmt.Sprintf("\tvault %s: failed to total pending swap-out shares: %v", addr, err), ""
	}

	held := k.BankKeeper.GetBalance(ctx, addr, shareDenom).Amount
	if held.LT(claimed) {
		return fmt.Sprintf("\tvault %s: holds %s%s in escrow but pending swap-outs account for %s (shortfall %s)",
			addr, held, shareDenom, claimed, claimed.Sub(held)), ""
	}

	return "", ""
}
