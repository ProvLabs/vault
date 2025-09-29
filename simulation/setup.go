package simulation

import (
	"fmt"
	"math/rand"

	"github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"

	markerkeeper "github.com/provenance-io/provenance/x/marker/keeper"
)

// IsSetup checks if the simulation has been set up with any vaults.
func IsSetup(k keeper.Keeper, ctx sdk.Context) bool {
	vaults, err := k.GetVaults(ctx)
	if err != nil {
		// If there's an error getting vaults, we can consider it not set up.
		return false
	}
	return len(vaults) > 0
}

// Setup ensures the simulation is ready by creating global markers and an initial vault if none exist.
func Setup(ctx sdk.Context, r *rand.Rand, k keeper.Keeper, ak types.AccountKeeper, bk types.BankKeeper, mk types.MarkerKeeper, accs []simtypes.Account) error {
	if IsSetup(k, ctx) {
		return nil
	}

	denomRegex := mk.GetUnrestrictedDenomRegex(ctx)

	// Create global markers for underlying and payment denoms.
	underlyingDenom := genRandomDenom(r, denomRegex, VaultGlobalDenomSuffix)
	paymentDenom := genRandomDenom(r, denomRegex, VaultGlobalDenomSuffix)
	restrictedDenom := genRandomDenom(r, denomRegex, VaultGlobalDenomSuffix)

	// Need to manually cast here
	markerKeeper, ok := mk.(markerkeeper.Keeper)
	if !ok {
		return fmt.Errorf("marker keeper is not of type markerkeeper.Keeper")
	}

	if err := CreateGlobalMarker(ctx, ak, bk, markerKeeper, sdk.NewInt64Coin(underlyingDenom, 100_000_000), accs, false); err != nil {
		return fmt.Errorf("failed to create global marker for underlying: %w", err)
	}
	if err := CreateGlobalMarker(ctx, ak, bk, markerKeeper, sdk.NewInt64Coin(paymentDenom, 100_000_000), accs, false); err != nil {
		return fmt.Errorf("failed to create global marker for payment: %w", err)
	}

	// Create a restricted marker and distribute to only 2 accounts
	/*
		if len(accs) < 2 {
			return fmt.Errorf("not enough accounts to create restricted marker, need at least 2")
		}
		restrictedAccs := accs[:2]
		if err := CreateGlobalMarker(ctx, ak, bk, markerKeeper, sdk.NewInt64Coin(restrictedDenom, 100_000_000), restrictedAccs, true); err != nil {
			return fmt.Errorf("failed to create global marker for restricted: %w", err)
		}
	*/

	// Add required attribute to the accounts that received the restricted asset
	/*
		for _, acc := range restrictedAccs {
			if err := AddAttribute(ctx, acc.Address, RequiredMarkerAttribute, nk, attrk); err != nil {
				return fmt.Errorf("failed to add attribute to account %s: %w", acc.Address, err)
			}
		}
	*/

	if err := AddNav(ctx, markerKeeper, paymentDenom, ak.GetModuleAddress("mint"), sdk.NewInt64Coin(underlyingDenom, 1), 1); err != nil {
		return fmt.Errorf("failed to add nav for payment: %w", err)
	}

	/*
		if err := AddNav(ctx, markerKeeper, paymentDenom, ak.GetModuleAddress("mint"), sdk.NewInt64Coin(restrictedDenom, 1), 1); err != nil {
			return fmt.Errorf("failed to add nav for payment: %w", err)
		}
	*/

	if err := AddNav(ctx, markerKeeper, underlyingDenom, ak.GetModuleAddress("mint"), sdk.NewInt64Coin(paymentDenom, 1), 1); err != nil {
		return fmt.Errorf("failed to add nav for payment: %w", err)
	}

	/*
		if err := AddNav(ctx, markerKeeper, underlyingDenom, ak.GetModuleAddress("mint"), sdk.NewInt64Coin(restrictedDenom, 1), 1); err != nil {
			return fmt.Errorf("failed to add nav for payment: %w", err)
		}
	*/

	/*
		if err := AddNav(ctx, markerKeeper, restrictedDenom, ak.GetModuleAddress("mint"), sdk.NewInt64Coin(underlyingDenom, 2), 1); err != nil {
			return fmt.Errorf("failed to add nav for restricted: %w", err)
		}

		if err := AddNav(ctx, markerKeeper, restrictedDenom, ak.GetModuleAddress("mint"), sdk.NewInt64Coin(paymentDenom, 2), 1); err != nil {
			return fmt.Errorf("failed to add nav for restricted: %w", err)
		}
	*/

	// Create an initial vault.
	admin, _ := simtypes.RandomAcc(r, accs)
	shareDenom := fmt.Sprintf("vaultshare%d", r.Intn(1000))

	selectedPayment := ""
	if r.Intn(2) == 0 {
		selectedPayment = paymentDenom
	}

	return CreateVault(ctx, &k, ak, bk, markerKeeper, underlyingDenom, selectedPayment, shareDenom, admin, accs)
}
