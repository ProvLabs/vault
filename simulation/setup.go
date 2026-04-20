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
		return false
	}
	return len(vaults) > 0
}

// Setup ensures the simulation is ready by initializing the AUM fee collector account, binding the
// required marker attribute name, granting that attribute to all simulation accounts, creating global
// markers, and creating an initial vault if none exist.
func Setup(ctx sdk.Context, r *rand.Rand, k keeper.Keeper, ak types.AccountKeeper, bk types.BankKeeper, mk types.MarkerKeeper, accs []simtypes.Account) error {
	if IsSetup(k, ctx) {
		return nil
	}

	denomRegex := mk.GetUnrestrictedDenomRegex(ctx)

	provlabsAddr, err := k.GetAUMFeeAddress(ctx)
	if err != nil {
		return fmt.Errorf("failed to get aum fee address: %w", err)
	}
	feeCollectorAddr := ak.GetModuleAddress("fee_collector")

	// perform bootstrap inside a cache context to ensure atomicity
	cacheCtx, write := ctx.CacheContext()
	if !ak.HasAccount(cacheCtx, provlabsAddr) {
		ak.SetAccount(cacheCtx, ak.NewAccountWithAddress(cacheCtx, provlabsAddr))
	}

	if !ak.HasAccount(cacheCtx, feeCollectorAddr) {
		ak.SetAccount(cacheCtx, ak.NewAccountWithAddress(cacheCtx, feeCollectorAddr))
	}

	if err := BindName(cacheCtx, provlabsAddr, RequiredMarkerAttribute, k.NameKeeper); err != nil {
		return fmt.Errorf("failed to bind name to aum fee address: %w", err)
	}

	if err := AddAttribute(cacheCtx, provlabsAddr, provlabsAddr, RequiredMarkerAttribute, k.NameKeeper, k.AttrKeeper); err != nil {
		return fmt.Errorf("failed to add attribute to aum fee address: %w", err)
	}

	if err := AddAttribute(cacheCtx, provlabsAddr, feeCollectorAddr, RequiredMarkerAttribute, k.NameKeeper, k.AttrKeeper); err != nil {
		return fmt.Errorf("failed to add attribute to fee collector address: %w", err)
	}

	for _, acc := range accs {
		if err := AddAttribute(cacheCtx, provlabsAddr, acc.Address, RequiredMarkerAttribute, k.NameKeeper, k.AttrKeeper); err != nil {
			return fmt.Errorf("failed to add attribute to account %s: %w", acc.Address, err)
		}
	}
	write()

	underlyingDenom := genRandomDenom(r, denomRegex, VaultGlobalDenomSuffix)
	paymentDenom := genRandomDenom(r, denomRegex, VaultGlobalDenomSuffix)

	markerKeeper, ok := mk.(markerkeeper.Keeper)
	if !ok {
		return fmt.Errorf("marker keeper is not of type markerkeeper.Keeper")
	}

	if err := CreateGlobalMarker(ctx, ak, bk, markerKeeper, sdk.NewInt64Coin(underlyingDenom, 1_000_000_000), accs, false, feeCollectorAddr); err != nil {
		return fmt.Errorf("failed to create global marker for underlying %s: %w", underlyingDenom, err)
	}
	if err := CreateGlobalMarker(ctx, ak, bk, markerKeeper, sdk.NewInt64Coin(paymentDenom, 1_000_000_000), accs, false, feeCollectorAddr); err != nil {
		return fmt.Errorf("failed to create global marker for payment %s: %w", paymentDenom, err)
	}

	volume := uint64(r.Intn(4) + 1)
	if err := AddNav(ctx, markerKeeper, paymentDenom, ak.GetModuleAddress("mint"), sdk.NewInt64Coin(underlyingDenom, 1), volume); err != nil {
		return fmt.Errorf("failed to add nav for payment %s: %w", paymentDenom, err)
	}

	admin, _ := simtypes.RandomAcc(r, accs)
	shareDenom := fmt.Sprintf("vaultshare%d", r.Intn(1_000))

	selectedPayment := ""
	if r.Intn(2) == 0 {
		selectedPayment = paymentDenom
	}

	return CreateVault(ctx, &k, ak, bk, markerKeeper, underlyingDenom, selectedPayment, shareDenom, admin, accs)
}
