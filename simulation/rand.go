package simulation

import (
	"fmt"
	"math/rand"

	"github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
)

func getRandomVault(r *rand.Rand, k keeper.Keeper, ctx sdk.Context) (*types.VaultAccount, error) {
	vaults, err := k.GetVaults(ctx)
	if err != nil {
		return nil, err
	}
	if len(vaults) == 0 {
		return nil, fmt.Errorf("no vaults found")
	}
	vaultAddr := vaults[r.Intn(len(vaults))]
	vault, err := k.GetVault(ctx, vaultAddr)
	if err != nil {
		return nil, err
	}
	if vault == nil {
		return nil, fmt.Errorf("received nil vault")
	}
	return vault, nil
}

func getRandomDenom(r *rand.Rand, k keeper.Keeper, ctx sdk.Context, acc simtypes.Account) (string, error) {
	balances := k.BankKeeper.GetAllBalances(ctx, acc.Address)
	if balances.Empty() {
		return "", fmt.Errorf("account has no coins")
	}
	randIndex := r.Intn(len(balances))
	return balances[randIndex].Denom, nil
}

func getRandomInterestRate(r *rand.Rand, k keeper.Keeper, ctx sdk.Context, vaultAddr sdk.AccAddress) (string, error) {
	vault, err := k.GetVault(ctx, vaultAddr)
	if err != nil {
		return "", err
	}
	if vault == nil {
		return "", fmt.Errorf("received nil vault")
	}

	minStr := vault.GetMinInterestRate()
	if minStr == "" {
		minStr = "-1.0"
	}
	min := math.LegacyMustNewDecFromStr(minStr)

	maxStr := vault.GetMaxInterestRate()
	if maxStr == "" {
		maxStr = "2.0"
	}
	max := math.LegacyMustNewDecFromStr(maxStr)

	randomDec := simtypes.RandomDecAmount(r, max.Sub(min))
	resultDec := min.Add(randomDec)

	return resultDec.String(), nil
}

func getRandomMinInterestRate(r *rand.Rand, k keeper.Keeper, ctx sdk.Context, vaultAddr sdk.AccAddress) (string, error) {
	vault, err := k.GetVault(ctx, vaultAddr)
	if err != nil {
		return "", err
	}
	if vault == nil {
		return "", fmt.Errorf("received nil vault")
	}

	// The lower bound for a new min interest rate can be the default -1.0
	lowerBound := math.LegacyMustNewDecFromStr("-1.0")

	// Get max rate
	maxStr := vault.GetMaxInterestRate()
	if maxStr == "" {
		maxStr = "2.0"
	}
	maxRate := math.LegacyMustNewDecFromStr(maxStr)

	// Get current rate
	currentStr := vault.GetCurrentInterestRate()
	if currentStr == "" {
		currentStr = "0.0"
	}
	currentRate := math.LegacyMustNewDecFromStr(currentStr)

	// Get desired rate
	desiredStr := vault.DesiredInterestRate
	if desiredStr == "" {
		desiredStr = "0.0"
	}
	desiredRate := math.LegacyMustNewDecFromStr(desiredStr)

	// The new upper bound is the minimum of current, desired, and max rates.
	upperBound := maxRate
	if currentRate.LT(upperBound) {
		upperBound = currentRate
	}
	if desiredRate.LT(upperBound) {
		upperBound = desiredRate
	}

	// Ensure the upper bound is not less than the lower bound
	if upperBound.LTE(lowerBound) {
		return lowerBound.String(), nil
	}

	// Generate a random rate between lowerBound and upperBound
	randomDec := simtypes.RandomDecAmount(r, upperBound.Sub(lowerBound))
	resultDec := lowerBound.Add(randomDec)

	return resultDec.String(), nil
}

func getRandomMaxInterestRate(r *rand.Rand, k keeper.Keeper, ctx sdk.Context, vaultAddr sdk.AccAddress) (string, error) {
	vault, err := k.GetVault(ctx, vaultAddr)
	if err != nil {
		return "", err
	}
	if vault == nil {
		return "", fmt.Errorf("received nil vault")
	}

	// The upper bound for a new max interest rate can be the default 2.0
	upperBound := math.LegacyMustNewDecFromStr("2.0")

	// Get min rate
	minStr := vault.GetMinInterestRate()
	if minStr == "" {
		minStr = "-1.0"
	}
	minRate := math.LegacyMustNewDecFromStr(minStr)

	// Get current rate
	currentStr := vault.GetCurrentInterestRate()
	if currentStr == "" {
		currentStr = "0.0"
	}
	currentRate := math.LegacyMustNewDecFromStr(currentStr)

	// Get desired rate
	desiredStr := vault.DesiredInterestRate
	if desiredStr == "" {
		desiredStr = "0.0"
	}
	desiredRate := math.LegacyMustNewDecFromStr(desiredStr)

	// The new lower bound is the maximum of current, desired, and min rates.
	lowerBound := minRate
	if currentRate.GT(lowerBound) {
		lowerBound = currentRate
	}
	if desiredRate.GT(lowerBound) {
		lowerBound = desiredRate
	}

	// Ensure the upper bound is not less than the lower bound
	if upperBound.LTE(lowerBound) {
		return upperBound.String(), nil
	}

	// Generate a random rate between lowerBound and upperBound
	randomDec := simtypes.RandomDecAmount(r, upperBound.Sub(lowerBound))
	resultDec := lowerBound.Add(randomDec)

	return resultDec.String(), nil
}

func getRandomPendingSwapOut(r *rand.Rand, k keeper.Keeper, ctx sdk.Context, vaultAddr sdk.AccAddress) (uint64, error) {
	var swapIDs []uint64

	err := k.PendingSwapOutQueue.WalkByVault(sdk.UnwrapSDKContext(ctx), vaultAddr, func(_ int64, id uint64, _ types.PendingSwapOut) (stop bool, err error) {
		swapIDs = append(swapIDs, id)
		return false, nil
	})

	if err != nil {
		return 0, err
	}

	if len(swapIDs) == 0 {
		return 0, fmt.Errorf("no pending swap outs found for vault %s", vaultAddr.String())
	}

	// Pick a random swap ID from the collected list
	randomID := swapIDs[r.Intn(len(swapIDs))]

	return randomID, nil
}

func getRandomVaultAsset(r *rand.Rand, vault *types.VaultAccount) string {
	if vault.PaymentDenom == "" {
		return vault.UnderlyingAsset
	}
	if r.Intn(2) == 0 {
		return vault.UnderlyingAsset
	}
	return vault.PaymentDenom
}

func getRandomAccountWithDenom(r *rand.Rand, k keeper.Keeper, ctx sdk.Context, accs []simtypes.Account, denom string) (simtypes.Account, sdk.Coin, error) {
	r.Shuffle(len(accs), func(i, j int) {
		accs[i], accs[j] = accs[j], accs[i]
	})
	for _, acc := range accs {
		bal := k.BankKeeper.GetBalance(ctx, acc.Address, denom)
		if !bal.IsZero() {
			return acc, bal, nil
		}
	}

	return simtypes.Account{}, sdk.Coin{}, fmt.Errorf("no account has positive %s balance", denom)
}
