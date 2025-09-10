package simulation

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/provlabs/vault/types"

	"github.com/cosmos/cosmos-sdk/types/module"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

// RandomizedGenState generates a random GenesisState for the vault module
func RandomizedGenState(simState *module.SimulationState) {
	admin := simState.Accounts[0].Address.String()
	underlying := "underlying"
	payment := ""

	vaults := []types.VaultAccount{}

	for i := 0; i < simState.Rand.Intn(5)+1; i++ {
		denom := fmt.Sprintf("vaultshare%d", i)
		addr := types.GetVaultAddress(denom)

		minRate := simState.Rand.Float64() * 0.1
		maxRate := minRate + simState.Rand.Float64()*0.1
		desiredRate := minRate + simState.Rand.Float64()*(maxRate-minRate)
		currentRate := desiredRate
		minInterestRate := fmt.Sprintf("%f", minRate)
		maxInterestRate := fmt.Sprintf("%f", maxRate)
		desiredInterestRate := fmt.Sprintf("%f", desiredRate)
		currentInterestRate := fmt.Sprintf("%f", currentRate)
		periodStart := simState.GenTimestamp.Unix()
		periodTimeout := time.Duration(simState.Rand.Int63n(int64(time.Hour) * 24 * 7)).
		swapInEnabled := simState.Rand.Intn(2) == 1
		swapOutEnabled := simState.Rand.Intn(2) == 1
		withdrawalDelaySeconds := uint64(simState.Rand.Int63n(int64(time.Hour) * 24 * 7))

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
	}

	vaultGenesis := types.GenesisState{
		Vaults: vaults,
	}

	bz, err := json.MarshalIndent(&vaultGenesis, "", " ")
	if err != nil {
		panic(err)
	}
	fmt.Printf("Selected randomly generated vault parameters:\n%s\n", bz)

	simState.GenState[types.ModuleName] = simState.Cdc.MustMarshalJSON(&vaultGenesis)
}
