package simulation

import (
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"

	"github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
)

// genRandomDenom generates a random denominator string with a given suffix.
func genRandomDenom(r *rand.Rand, regex, suffix string) string {
	denom := randomUnrestrictedDenom(r, regex) + suffix
	denom = denom[:len(denom)-len(suffix)] + suffix
	return denom
}

// randomInt63 generates a random int64 between 0 and maxVal.
func randomInt63(r *rand.Rand, maxVal int64) (result int64) {
	if maxVal == 0 {
		return 0
	}
	return r.Int63n(maxVal)
}

// randomUnrestrictedDenom generates a random string for a denom based on the length constraints in the expression.
func randomUnrestrictedDenom(r *rand.Rand, unrestrictedDenomExp string) string {
	exp := regexp.MustCompile(`\{(\d+),(\d+)\}`)
	matches := exp.FindStringSubmatch(unrestrictedDenomExp)
	if len(matches) != 3 {
		panic("expected two number as range expression in unrestricted denom expression")
	}
	minLen, _ := strconv.ParseInt(matches[1], 10, 32)
	maxLen, _ := strconv.ParseInt(matches[2], 10, 32)

	return simtypes.RandStringOfLength(r, int(randomInt63(r, maxLen-minLen)+minLen))
}

// getRandomVault selects a random vault from all existing vaults.
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

// getRandomVaultWithCondition gets a random vault that satisfies a given condition.
func getRandomVaultWithCondition(r *rand.Rand, k keeper.Keeper, ctx sdk.Context, condition func(vault types.VaultAccount) bool) (*types.VaultAccount, error) {
	allVaultAddrs, err := k.GetVaults(ctx)
	if err != nil {
		return nil, err
	}

	var matchingVaults []*types.VaultAccount
	for _, vaultAddr := range allVaultAddrs {
		vault, err := k.GetVault(ctx, vaultAddr)
		if err != nil || vault == nil {
			continue
		}

		if condition(*vault) {
			matchingVaults = append(matchingVaults, vault)
		}
	}

	if len(matchingVaults) == 0 {
		return nil, fmt.Errorf("no vaults found matching condition")
	}

	return matchingVaults[r.Intn(len(matchingVaults))], nil
}

// getRandomBridgedVault finds a random vault that has bridging enabled and the bridge address is a sim account.
func getRandomBridgedVault(r *rand.Rand, k keeper.Keeper, ctx sdk.Context, accs []simtypes.Account, checkBalance bool) (types.VaultAccount, error) {
	addrs, err := k.GetVaults(sdk.UnwrapSDKContext(ctx))
	if err != nil {
		return types.VaultAccount{}, err
	}

	vaults := []types.VaultAccount{}
	for _, addr := range addrs {
		vault, err := k.GetVault(ctx, addr)
		if err != nil {
			continue
		}
		if vault == nil {
			continue
		}

		if vault.BridgeAddress != "" {
			addr, err := sdk.AccAddressFromBech32(vault.BridgeAddress)
			if err == nil {
				if _, found := simtypes.FindAccount(accs, addr); found {
					if checkBalance {
						balance := k.BankKeeper.GetBalance(ctx, addr, vault.GetTotalShares().Denom)
						if balance.IsPositive() {
							vaults = append(vaults, *vault)
						}
					} else {
						vaults = append(vaults, *vault)
					}
				}
			}
		}
	}

	if len(vaults) == 0 {
		return types.VaultAccount{}, fmt.Errorf("no vaults with bridge enabled and bridge account found")
	}

	return vaults[r.Intn(len(vaults))], nil
}

// getRandomDenom gets a random denomination from an account's balances that has a 'vx' suffix.
func getRandomDenom(r *rand.Rand, k keeper.Keeper, ctx sdk.Context, acc simtypes.Account) (string, error) {
	balances := k.BankKeeper.GetAllBalances(sdk.UnwrapSDKContext(ctx), acc.Address)
	if balances.Empty() {
		return "", fmt.Errorf("account has no coins")
	}

	r.Shuffle(len(balances), func(i, j int) {
		balances[i], balances[j] = balances[j], balances[i]
	})

	for _, coin := range balances {
		if strings.HasSuffix(coin.Denom, VaultGlobalDenomSuffix) {
			return coin.Denom, nil
		}
	}

	return "", fmt.Errorf("account has no coins with a 'vx' suffix")
}

// getRandomInterestRate generates a random valid interest rate for a given vault.
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

// getRandomMinInterestRate generates a random valid minimum interest rate for a given vault.
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

// getRandomMaxInterestRate generates a random valid maximum interest rate for a given vault.
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

// getRandomPendingSwapOut gets a random pending swap out request ID for a given vault.
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

// getRandomVaultAsset gets a random asset (either underlying or payment) accepted by a vault.
func getRandomVaultAsset(r *rand.Rand, vault *types.VaultAccount) string {
	if vault.PaymentDenom == "" {
		return vault.UnderlyingAsset
	}
	if r.Intn(2) == 0 {
		return vault.UnderlyingAsset
	}
	return vault.PaymentDenom
}

// getRandomAccountWithDenom finds a random account from a list that has a positive balance of a given denomination.
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
