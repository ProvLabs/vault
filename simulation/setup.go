package simulation

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/types"
	"github.com/provlabs/vault/utils"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/types/simulation"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	markerkeeper "github.com/provenance-io/provenance/x/marker/keeper"
)

const (
	MaxNumVaults            = 5
	ChanceOfPaymentDenom    = 2  // 1 in X
	ChanceOfNegativeMinRate = 10 // 1 in X
	MaxPositiveMinRate      = 0.1
	MaxNegativeMinRate      = 0.05
	MaxRateAddition         = 0.2

	ChanceOfPausedVault          = 5 // 1 in X
	MaxPausedBalance             = 1_000_000
	MinPausedBalance             = 1_000
	ChanceOfSwapInEnabled        = 2 // 1 in X
	ChanceOfSwapOutEnabled       = 2 // 1 in X
	ChanceOfTimeoutEntry         = 2 // 1 in X
	MaxNumPendingSwaps           = 11
	MaxSharesAmount              = 1000 // This is the base amount, it will be multiplied by ShareScalar
	MaxTotalShares               = 1000 // This is the base amount, it will be multiplied by ShareScalar
	ChanceOfRedeemDenomIsPayment = 2    // 1 in X
	NumPayoutTimeOptions         = 3
)

func randomVaults(simState *module.SimulationState) []types.VaultAccount {
	var vaults []types.VaultAccount

	for i := 0; i < simState.Rand.Intn(MaxNumVaults)+1; i++ {
		vault := CreateRandomVault(simState.Rand, simState.GenTimestamp, simState.Accounts)
		vaults = append(vaults, vault)
	}

	return vaults
}

// CreateRandomVault creates a single random vault for simulation purposes.
func CreateRandomVault(r *rand.Rand, currentTimestamp time.Time, accs []simulation.Account) types.VaultAccount {
	admin := accs[0].Address.String()

	// --- Vault Config ---
	shareDenom := fmt.Sprintf("vaultshare%d", r.Int63n(100_000_000))
	underlyingDenom := "underlying"
	paymentDenom := ""

	if r.Intn(ChanceOfPaymentDenom) == 0 {
		paymentDenom = "payment"
	}

	addr := types.GetVaultAddress(shareDenom)

	var minRate float64
	if r.Intn(ChanceOfNegativeMinRate) == 0 {
		minRate = (r.Float64() - 1) * MaxNegativeMinRate
	} else {
		minRate = r.Float64() * MaxPositiveMinRate
	}

	maxRate := minRate + r.Float64()*MaxRateAddition
	desiredRate := minRate + r.Float64()*(maxRate-minRate)
	currentRate := minRate + r.Float64()*(maxRate-minRate)
	withdrawalDelaySeconds := uint64(r.Int63n(keeper.AutoReconcileTimeout))

	// Calculate a random number of total shares. It should be multiplied by ShareScalar
	baseTotalShares := sdkmath.NewInt(r.Int63n(MaxTotalShares + 1))
	totalShares := sdk.NewCoin(shareDenom, baseTotalShares.Mul(utils.ShareScalar))

	var periodStart, periodTimeout int64
	switch r.Intn(3) {
	case 0: // Inactive
		desiredRate = 0
		currentRate = 0
		periodStart = 0
		periodTimeout = 0
		minRate = 0
		maxRate = 0
	case 1: // Active
		currentRate = desiredRate
		periodStart = currentTimestamp.Unix()
		periodTimeout = currentTimestamp.Unix() + r.Int63n(int64(keeper.AutoReconcileTimeout))
	case 2: // Defaulted
		currentRate = 0
		periodTimeout = currentTimestamp.Unix()
		periodStart = periodTimeout - (r.Int63n(int64(keeper.AutoReconcileTimeout)) + 1)
	}

	var paused bool
	var pausedBalance sdk.Coin
	if r.Intn(ChanceOfPausedVault) == 0 {
		paused = true
		pausedBalance = sdk.NewCoin(underlyingDenom, sdkmath.NewInt(r.Int63n(MaxPausedBalance)+MinPausedBalance))
	} else {
		paused = false
		pausedBalance = sdk.Coin{}
	}

	return types.VaultAccount{
		BaseAccount:            authtypes.NewBaseAccountWithAddress(addr),
		Admin:                  admin,
		TotalShares:            totalShares,
		UnderlyingAsset:        underlyingDenom,
		PaymentDenom:           paymentDenom,
		CurrentInterestRate:    fmt.Sprintf("%f", currentRate),
		DesiredInterestRate:    fmt.Sprintf("%f", desiredRate),
		MinInterestRate:        fmt.Sprintf("%f", minRate),
		MaxInterestRate:        fmt.Sprintf("%f", maxRate),
		PeriodStart:            periodStart,
		PeriodTimeout:          periodTimeout,
		SwapInEnabled:          r.Intn(ChanceOfSwapInEnabled) == 0,
		SwapOutEnabled:         r.Intn(ChanceOfSwapOutEnabled) == 0,
		WithdrawalDelaySeconds: withdrawalDelaySeconds,
		Paused:                 paused,
		PausedBalance:          pausedBalance,
	}
}

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
	underlyingDenom := genRandomDenom(r, denomRegex, "vx")
	paymentDenom := genRandomDenom(r, denomRegex, "vx")
	// restrictedDenom := genRandomDenom(r, denomRegex, "vx")

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
