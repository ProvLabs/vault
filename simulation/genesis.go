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

// RandomizedGenState generates a random GenesisState for the vault module
func RandomizedGenState(simState *module.SimulationState) {
	vaults := randomVaults(simState)
	timeouts := randomTimeouts(simState, vaults)

pendingSwaps := randomPendingSwaps(simState, vaults)

	// TODO: Fund all accounts and markers based on the generated state.

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
	const (
		maxNumVaults            = 5
		chanceOfPaymentDenom    = 2 // 1 in X
		chanceOfNegativeMinRate  = 10 // 1 in X
		maxPositiveMinRate      = 0.1
		maxNegativeMinRate      = 0.05
		maxRateAddition         = 0.2
		chanceOfNewVault        = 3  // 1 in X
		chanceOfPausedVault     = 5  // 1 in X
		maxPausedBalance        = 1_000_000
		minPausedBalance        = 1_000
		chanceOfSwapInEnabled   = 2 // 1 in X
		chanceOfSwapOutEnabled  = 2 // 1 in X
	)

	admin := simState.Accounts[0].Address.String()
	var vaults []types.VaultAccount

	for i := 0; i < simState.Rand.Intn(maxNumVaults)+1; i++ {
		// --- Vault Config ---
		shareDenom := fmt.Sprintf("vaultshare%d", i)
		underlyingDenom := "underlying"
		paymentDenom := ""
		if simState.Rand.Intn(chanceOfPaymentDenom) == 0 {
			paymentDenom = "payment"
		}

		addr := types.GetVaultAddress(shareDenom)

		var minRate float64
		if simState.Rand.Intn(chanceOfNegativeMinRate) == 0 {
			minRate = (simState.Rand.Float64() - 1) * maxNegativeMinRate
		} else {
			minRate = simState.Rand.Float64() * maxPositiveMinRate
		}

		maxRate := minRate + simState.Rand.Float64()*maxRateAddition
		desiredRate := minRate + simState.Rand.Float64()*(maxRate-minRate)
		currentRate := minRate + simState.Rand.Float64()*(maxRate-minRate)
		withdrawalDelaySeconds := uint64(simState.Rand.Int63n(keeper.AutoReconcileTimeout))

		var periodStart, periodTimeout int64
		if simState.Rand.Intn(chanceOfNewVault) == 0 {
			periodStart = 0
			periodTimeout = 0
		} else {
			periodStart = simState.GenTimestamp.Unix()
			periodTimeout = simState.GenTimestamp.Unix() + simState.Rand.Int63n(int64(keeper.AutoReconcileTimeout))
		}

		var paused bool
		var pausedBalance sdk.Coin
		if simState.Rand.Intn(chanceOfPausedVault) == 0 {
			paused = true
			pausedBalance = sdk.NewCoin(underlyingDenom, sdkmath.NewInt(simState.Rand.Int63n(maxPausedBalance)+minPausedBalance))
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
			SwapInEnabled:          simState.Rand.Intn(chanceOfSwapInEnabled) == 0,
			SwapOutEnabled:         simState.Rand.Intn(chanceOfSwapOutEnabled) == 0,
			WithdrawalDelaySeconds: withdrawalDelaySeconds,
			Paused:                 paused,
			PausedBalance:          pausedBalance,
		}
		vaults = append(vaults, vault)
	}

	return vaults
}

func randomTimeouts(simState *module.SimulationState, vaults []types.VaultAccount) []types.QueueEntry {
	const (
		chanceOfTimeoutEntry = 2 // 1 in X
	)

	var timeouts []types.QueueEntry

	for _, vault := range vaults {
		// A vault with no timeout set should not be in the queue.
		if vault.PeriodTimeout == 0 {
			continue
		}

		if simState.Rand.Intn(chanceOfTimeoutEntry) == 0 {
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
	const (
		maxNumPendingSwaps           = 11
		maxSharesAmount              = 1000 // This is the base amount, it will be multiplied by ShareScalar
		chanceOfRedeemDenomIsPayment = 2 // 1 in X
		numPayoutTimeOptions         = 3
	)

	var allPendingSwaps []types.PendingSwapOutQueueEntry
	maxSequence := uint64(0)

	for _, vault := range vaults {
		numPendingSwaps := simState.Rand.Intn(maxNumPendingSwaps)
		for j := 0; j < numPendingSwaps; j++ {
			maxSequence++
			requestID := maxSequence

			owner := simState.Accounts[simState.Rand.Intn(len(simState.Accounts))].Address
			
			// Multiply the base random amount by the ShareScalar to get a realistic, scaled share amount
			baseSharesAmount := sdkmath.NewInt(simState.Rand.Int63n(maxSharesAmount) + 1)
			scaledSharesAmount := baseSharesAmount.Mul(utils.ShareScalar)
			shares := sdk.NewCoin(vault.ShareDenom, scaledSharesAmount)

			redeemDenom := vault.UnderlyingAsset
			if vault.PaymentDenom != "" && simState.Rand.Intn(chanceOfRedeemDenomIsPayment) == 0 {
				redeemDenom = vault.PaymentDenom
			}

			var payoutTime int64
			switch simState.Rand.Intn(numPayoutTimeOptions) {
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
		Entries:            allPendingSwaps,
		LatestSequenceNumber: maxSequence,
	}
}