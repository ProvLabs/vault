package simulation

import (
	"encoding/json"
	"fmt"

	"github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

// RandomizedGenState generates a random GenesisState for the vault module
func RandomizedGenState(simState *module.SimulationState) {
	admin := simState.Accounts[0].Address.String()
	underlying := "underlying"
	payment := ""

	vaults := []types.VaultAccount{}
	timeouts := []types.QueueEntry{}
	pendingSwaps := []types.PendingSwapOutQueueEntry{}
	pendingSwapOutQueue := types.PendingSwapOutQueue{}
	pendingSwapOutQueue.Entries = pendingSwaps
	maxSequence := uint64(0)

	// TODO Make sure the vault account has the shares to burn
	// TODO Make sure the MarkerAccount has enough of the asset
	// TODO Make sure there is a Marker for the shares and has the sum of all shares + more
	// TODO Make sure the vault account has enough of underlying asset for paying interest

	for i := 0; i < simState.Rand.Intn(5)+1; i++ {
		denom := fmt.Sprintf("vaultshare%d", i)
		addr := types.GetVaultAddress(denom)

		minRate := simState.Rand.Float64() * 0.1
		maxRate := minRate + simState.Rand.Float64()*0.1
		desiredRate := minRate + simState.Rand.Float64()*(maxRate-minRate)
		currentRate := minRate + simState.Rand.Float64()*(maxRate-minRate)
		minInterestRate := fmt.Sprintf("%f", minRate)
		maxInterestRate := fmt.Sprintf("%f", maxRate)
		desiredInterestRate := fmt.Sprintf("%f", desiredRate)
		currentInterestRate := fmt.Sprintf("%f", currentRate)
		periodStart := simState.GenTimestamp.Unix()
		periodTimeout := simState.GenTimestamp.Unix() + simState.Rand.Int63n(int64(keeper.AutoReconcileTimeout))
		swapInEnabled := simState.Rand.Intn(2) == 1
		swapOutEnabled := simState.Rand.Intn(2) == 1
		withdrawalDelaySeconds := uint64(simState.Rand.Int63n(keeper.AutoReconcileTimeout))

		vaults = append(vaults, types.VaultAccount{
			BaseAccount:            authtypes.NewBaseAccountWithAddress(addr),
			Admin:                  admin,
			ShareDenom:             denom,
			UnderlyingAsset:        underlying,
			PaymentDenom:           payment,
			CurrentInterestRate:    currentInterestRate,
			DesiredInterestRate:    desiredInterestRate,
			MinInterestRate:        minInterestRate,
			MaxInterestRate:        maxInterestRate,
			PeriodStart:            periodStart,
			PeriodTimeout:          periodTimeout,
			SwapInEnabled:          swapInEnabled,
			SwapOutEnabled:         swapOutEnabled,
			WithdrawalDelaySeconds: withdrawalDelaySeconds,
		})

		timeouts = append(timeouts, types.QueueEntry{
			Time: uint64(periodTimeout),
			Addr: addr.String(),
		})

		// Create a random number of pending swaps for the vault
		for j := 0; j < simState.Rand.Intn(11); j++ {
			owner := simState.Accounts[simState.Rand.Intn(len(simState.Accounts))].Address
			shares := sdk.NewCoin(denom, sdkmath.NewInt(simState.Rand.Int63n(1000)+1))
			assets := sdk.NewCoin(underlying, sdkmath.NewInt(simState.Rand.Int63n(1000)+1))
			requestID := uint64(i*10 + j)
			payoutTime := simState.GenTimestamp.Unix() + int64(withdrawalDelaySeconds)

			pendingSwaps = append(pendingSwaps, types.PendingSwapOutQueueEntry{
				Time: payoutTime,
				Id:   requestID,
				SwapOut: types.PendingSwapOut{
					VaultAddress: addr.String(),
					Owner:        owner.String(),
					Shares:       shares,
					Assets:       assets,
				}},
			)
			if requestID > maxSequence {
				maxSequence = requestID
			}
		}
	}

	pendingSwapOutQueue.LatestSequenceNumber = maxSequence

	vaultGenesis := types.GenesisState{
		Vaults:              vaults,
		PayoutTimeoutQueue:  timeouts,
		PendingSwapOutQueue: pendingSwapOutQueue,
	}

	bz, err := json.MarshalIndent(&vaultGenesis, "", " ")
	if err != nil {
		panic(err)
	}
	fmt.Printf("Selected randomly generated vault parameters: %s", bz)

	simState.GenState[types.ModuleName] = simState.Cdc.MustMarshalJSON(&vaultGenesis)
}
