package simulation

import (
	"encoding/json"
	"fmt"

	"github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/types"
	"github.com/provlabs/vault/utils"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

const (
	MaxNumVaults                 = 5
	ChanceOfPaymentDenom         = 2  // 1 in X
	ChanceOfNegativeMinRate      = 10 // 1 in X
	MaxPositiveMinRate           = 0.1
	MaxNegativeMinRate           = 0.05
	MaxRateAddition              = 0.2
	ChanceOfNewVault             = 3 // 1 in X
	ChanceOfPausedVault          = 5 // 1 in X
	MaxPausedBalance             = 1_000_000
	MinPausedBalance             = 1_000
	ChanceOfSwapInEnabled        = 2 // 1 in X
	ChanceOfSwapOutEnabled       = 2 // 1 in X
	ChanceOfTimeoutEntry         = 2 // 1 in X
	MaxNumPendingSwaps           = 11
	MaxSharesAmount              = 1000 // This is the base amount, it will be multiplied by ShareScalar
	ChanceOfRedeemDenomIsPayment = 2    // 1 in X
	NumPayoutTimeOptions         = 3
)

// RandomizedGenState generates a random GenesisState for the vault module
func RandomizedGenState(simState *module.SimulationState) {
	vaults := randomVaults(simState)
	timeouts := randomTimeouts(simState, vaults)
	pendingSwaps := randomPendingSwaps(simState, vaults)

	vaultGenesis := types.GenesisState{
		Vaults:              vaults,
		PayoutTimeoutQueue:  timeouts,
		PendingSwapOutQueue: pendingSwaps,
	}

	bz, err := json.MarshalIndent(&vaultGenesis, "", " ")
	if err != nil {
		panic(err)
	}
	fmt.Printf("Selected randomly generated vault parameters: %s\n", bz)

	simState.GenState[types.ModuleName] = simState.Cdc.MustMarshalJSON(&vaultGenesis)
}

func randomVaults(simState *module.SimulationState) []types.VaultAccount {
	admin := simState.Accounts[0].Address.String()
	var vaults []types.VaultAccount

	for i := 0; i < simState.Rand.Intn(MaxNumVaults)+1; i++ {
		// --- Vault Config ---
		shareDenom := fmt.Sprintf("vaultshare%d", i)
		underlyingDenom := "underlying"
		paymentDenom := ""
		if simState.Rand.Intn(ChanceOfPaymentDenom) == 0 {
			paymentDenom = "payment"
		}

		addr := types.GetVaultAddress(shareDenom)

		var minRate float64
		if simState.Rand.Intn(ChanceOfNegativeMinRate) == 0 {
			minRate = (simState.Rand.Float64() - 1) * MaxNegativeMinRate
		} else {
			minRate = simState.Rand.Float64() * MaxPositiveMinRate
		}

		maxRate := minRate + simState.Rand.Float64()*MaxRateAddition
		desiredRate := minRate + simState.Rand.Float64()*(maxRate-minRate)
		currentRate := minRate + simState.Rand.Float64()*(maxRate-minRate)
		withdrawalDelaySeconds := uint64(simState.Rand.Int63n(keeper.AutoReconcileTimeout))

		var periodStart, periodTimeout int64
		if simState.Rand.Intn(ChanceOfNewVault) == 0 {
			periodStart = 0
			periodTimeout = 0
		} else {
			periodStart = simState.GenTimestamp.Unix()
			periodTimeout = simState.GenTimestamp.Unix() + simState.Rand.Int63n(int64(keeper.AutoReconcileTimeout))
		}

		var paused bool
		var pausedBalance sdk.Coin
		if simState.Rand.Intn(ChanceOfPausedVault) == 0 {
			paused = true
			pausedBalance = sdk.NewCoin(underlyingDenom, sdkmath.NewInt(simState.Rand.Int63n(MaxPausedBalance)+MinPausedBalance))
		} else {
			paused = false
			pausedBalance = sdk.Coin{}
		}

		vault := types.VaultAccount{
			BaseAccount:            authtypes.NewBaseAccountWithAddress(addr),
			Admin:                  admin,
			ShareDenom:             shareDenom,
			UnderlyingAsset:        underlyingDenom,
			PaymentDenom:           paymentDenom,
			CurrentInterestRate:    fmt.Sprintf("%f", currentRate),
			DesiredInterestRate:    fmt.Sprintf("%f", desiredRate),
			MinInterestRate:        fmt.Sprintf("%f", minRate),
			MaxInterestRate:        fmt.Sprintf("%f", maxRate),
			PeriodStart:            periodStart,
			PeriodTimeout:          periodTimeout,
			SwapInEnabled:          simState.Rand.Intn(ChanceOfSwapInEnabled) == 0,
			SwapOutEnabled:         simState.Rand.Intn(ChanceOfSwapOutEnabled) == 0,
			WithdrawalDelaySeconds: withdrawalDelaySeconds,
			Paused:                 paused,
			PausedBalance:          pausedBalance,
		}
		vaults = append(vaults, vault)
	}

	return vaults
}

func randomTimeouts(simState *module.SimulationState, vaults []types.VaultAccount) []types.QueueEntry {
	var timeouts []types.QueueEntry

	for _, vault := range vaults {
		// A vault with no timeout set should not be in the queue.
		if vault.PeriodTimeout == 0 {
			continue
		}

		if simState.Rand.Intn(ChanceOfTimeoutEntry) == 0 {
			addr, err := sdk.AccAddressFromBech32(vault.GetAddress().String())
			if err != nil {
				panic(err)
			}
			timeouts = append(timeouts, types.QueueEntry{
				Time: uint64(vault.PeriodTimeout),
				Addr: addr.String(),
			})
		}
	}

	return timeouts
}

func randomPendingSwaps(simState *module.SimulationState, vaults []types.VaultAccount) types.PendingSwapOutQueue {
	var allPendingSwaps []types.PendingSwapOutQueueEntry
	maxSequence := uint64(0)

	for _, vault := range vaults {
		numPendingSwaps := simState.Rand.Intn(MaxNumPendingSwaps)
		for j := 0; j < numPendingSwaps; j++ {
			maxSequence++
			requestID := maxSequence

			owner := simState.Accounts[simState.Rand.Intn(len(simState.Accounts))].Address

			// Multiply the base random amount by the ShareScalar to get a realistic, scaled share amount
			baseSharesAmount := sdkmath.NewInt(simState.Rand.Int63n(MaxSharesAmount) + 1)
			scaledSharesAmount := baseSharesAmount.Mul(utils.ShareScalar)
			shares := sdk.NewCoin(vault.ShareDenom, scaledSharesAmount)

			redeemDenom := vault.UnderlyingAsset
			if vault.PaymentDenom != "" && simState.Rand.Intn(ChanceOfRedeemDenomIsPayment) == 0 {
				redeemDenom = vault.PaymentDenom
			}

			var payoutTime int64
			switch simState.Rand.Intn(NumPayoutTimeOptions) {
			case 0:
				payoutTime = 0
			case 1:
				payoutTime = simState.GenTimestamp.Unix()
			default:
				if vault.WithdrawalDelaySeconds > 0 {
					payoutTime = simState.GenTimestamp.Unix() + simState.Rand.Int63n(int64(vault.WithdrawalDelaySeconds))
				} else {
					payoutTime = simState.GenTimestamp.Unix()
				}
			}

			allPendingSwaps = append(allPendingSwaps, types.PendingSwapOutQueueEntry{
				Time: payoutTime,
				Id:   requestID,
				SwapOut: types.PendingSwapOut{
					VaultAddress: vault.GetAddress().String(),
					Owner:        owner.String(),
					Shares:       shares,
					RedeemDenom:  redeemDenom,
				},
			})
		}
	}

	return types.PendingSwapOutQueue{
		Entries:              allPendingSwaps,
		LatestSequenceNumber: maxSequence,
	}
}
