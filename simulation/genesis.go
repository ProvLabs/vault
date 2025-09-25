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
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	markertypes "github.com/provenance-io/provenance/x/marker/types"
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
	ChanceOfRedeemDenomIsPayment = 2    // 1 in X
	NumPayoutTimeOptions         = 3
)

// RandomizedGenState generates a random GenesisState for the vault module
func RandomizedGenState(simState *module.SimulationState) *types.GenesisState {
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
	return &vaultGenesis
}

func randomVaults(simState *module.SimulationState) []types.VaultAccount {
	var vaults []types.VaultAccount
	admin := simState.Accounts[0].Address.String()

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
		switch simState.Rand.Intn(3) {
		case 0: // Inactive
			desiredRate = 0
			currentRate = 0
			periodStart = 0
			periodTimeout = 0
			minRate = 0
			maxRate = 0
		case 1: // Active
			currentRate = desiredRate
			periodStart = simState.GenTimestamp.Unix()
			periodTimeout = simState.GenTimestamp.Unix() + simState.Rand.Int63n(int64(keeper.AutoReconcileTimeout))
		case 2: // Defaulted
			currentRate = 0
			periodTimeout = simState.GenTimestamp.Unix()
			periodStart = periodTimeout - (simState.Rand.Int63n(int64(keeper.AutoReconcileTimeout)) + 1)
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
		LatestSequenceNumber: maxSequence + 1,
	}
}

// GenerateMarkerGenesis creates the underlying and payment markers for simulation.
func GenerateMarkerGenesis(simState *module.SimulationState) {
	var markerGenesis markertypes.GenesisState
	var bankGenesis banktypes.GenesisState
	simState.Cdc.MustUnmarshalJSON(simState.GenState[markertypes.ModuleName], &markerGenesis)
	simState.Cdc.MustUnmarshalJSON(simState.GenState[banktypes.ModuleName], &bankGenesis)

	underlying := sdk.NewInt64Coin("underlyingvault", 1_000_000_000_000)
	payment := sdk.NewInt64Coin("paymentvault", 1_000_000_000_000)

	for _, coin := range []sdk.Coin{underlying, payment} {
		modAddr := authtypes.NewModuleAddress(types.ModuleName)
		marker := markertypes.NewMarkerAccount(
			authtypes.NewBaseAccountWithAddress(modAddr),
			coin,
			modAddr,
			[]markertypes.AccessGrant{
				{
					Address: modAddr.String(),
					Permissions: markertypes.AccessList{
						markertypes.Access_Mint,
						markertypes.Access_Burn,
						markertypes.Access_Withdraw,
					},
				},
			},
			markertypes.StatusActive,
			markertypes.MarkerType_Coin,
			false, // supply not fixed
			true,  // allow gov control
			true,  // allow forced transfer
			[]string{},
		)
		markerGenesis.Markers = append(markerGenesis.Markers, *marker)
	}

	// We need to set a NAV for the payment denom so that it can be used in swaps
	nav := markertypes.NewNetAssetValue(sdk.NewInt64Coin(underlying.Denom, 1), 2)
	markerGenesis.NetAssetValues = append(markerGenesis.NetAssetValues, markertypes.MarkerNetAssetValues{
		Address:        string(markertypes.MustGetMarkerAddress(payment.Denom)),
		NetAssetValues: []markertypes.NetAssetValue{nav},
	})

	markerGenesisBz, err := simState.Cdc.MarshalJSON(&markerGenesis)
	if err != nil {
		panic(err)
	}
	simState.GenState[markertypes.ModuleName] = markerGenesisBz
}

// DistributeGeneratedMarkers distributes the generated markers to the simulation accounts.
func DistributeGeneratedMarkers(simState *module.SimulationState) {
	var bankGenesis banktypes.GenesisState
	simState.Cdc.MustUnmarshalJSON(simState.GenState[banktypes.ModuleName], &bankGenesis)

	underlying := sdk.NewInt64Coin("underlyingvault", 1_000_000_000_000)
	payment := sdk.NewInt64Coin("paymentvault", 1_000_000_000_000)

	// Update bankGenesis to evenly distribute all the marker denoms to the the simState accounts
	// Update bankGenesis supply to have each of the markers
	for _, coin := range []sdk.Coin{underlying, payment} {
		numAccounts := int64(len(simState.Accounts))
		if numAccounts == 0 {
			continue
		}

		// Add the new coin to the total supply
		bankGenesis.Supply = bankGenesis.Supply.Add(coin)

		// Distribute the coins, adding the remainder to the last account
		amountPerAccount := coin.Amount.QuoRaw(numAccounts)
		remainder := coin.Amount.ModRaw(numAccounts)

		for i, acc := range simState.Accounts {
			distAmount := amountPerAccount
			if i == len(simState.Accounts)-1 {
				distAmount = distAmount.Add(remainder)
			}

			bankGenesis.Balances = append(bankGenesis.Balances, banktypes.Balance{Address: acc.Address.String(), Coins: sdk.NewCoins(sdk.NewCoin(coin.Denom, distAmount))})
		}
	}

	bankGenesisBz, err := simState.Cdc.MarshalJSON(&bankGenesis)
	if err != nil {
		panic(err)
	}
	simState.GenState[banktypes.ModuleName] = bankGenesisBz
}
