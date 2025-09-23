package simapp

// DONTCOVER

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	abci "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"

	"cosmossdk.io/log"
	"cosmossdk.io/store"
	storetypes "cosmossdk.io/store/types"
	"cosmossdk.io/x/feegrant"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/server"
	sdk "github.com/cosmos/cosmos-sdk/types"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/baseapp"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	"github.com/cosmos/cosmos-sdk/types/kv"
	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	"github.com/cosmos/cosmos-sdk/x/simulation"
	simcli "github.com/cosmos/cosmos-sdk/x/simulation/client/cli"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func init() {
	simcli.GetSimulatorFlags()
	cfg := sdk.GetConfig()
	cfg.SetBech32PrefixForAccount("provlabs", "provlabspub")
	cfg.SetBech32PrefixForValidator("provlabsvaloper", "provlabsvaloperpub")
	cfg.SetBech32PrefixForConsensusNode("provlabsvalcons", "provlabsvalconspub")
	cfg.Seal()
}

// fauxMerkleModeOpt returns a BaseApp option to use a dbStoreAdapter instead of
// an IAVLStore for faster simulation speed.
func fauxMerkleModeOpt(bapp *baseapp.BaseApp) {
	bapp.SetFauxMerkleMode()
}

func setupSimulation(dirPrefix string, dbName string) (simtypes.Config, dbm.DB, string, log.Logger, bool, error) {
	config := simcli.NewConfigFromFlags()
	config.ChainID = "simulation-app"
	db, dir, logger, skip, err := simtestutil.SetupSimulation(config, dirPrefix, dbName, simcli.FlagVerboseValue, simcli.FlagEnabledValue)
	return config, db, dir, logger, skip, err
}

// newSimAppOpts creates a new set of AppOptions with a temp dir for home, and the desired invariant check period.
func newSimAppOpts(t testing.TB) simtestutil.AppOptionsMap {
	return simtestutil.AppOptionsMap{
		flags.FlagHome:            t.TempDir(),
		server.FlagInvCheckPeriod: simcli.FlagPeriodValue,
	}
}

// appStateFn wraps the simtypes.AppStateFn and sets the ICA and ICQ GenesisState if isn't yet defined in the appState.
func appStateFn(cdc codec.JSONCodec, simManager *module.SimulationManager, genesisState map[string]json.RawMessage) simtypes.AppStateFn {
	return func(r *rand.Rand, accs []simtypes.Account, config simtypes.Config) (json.RawMessage, []simtypes.Account, string, time.Time) {
		appState, simAccs, chainID, genesisTimestamp := simtestutil.AppStateFn(cdc, simManager, genesisState)(r, accs, config)
		return appState, simAccs, chainID, genesisTimestamp
	}
}

// interBlockCacheOpt returns a BaseApp option function that sets the persistent
// inter-block write-through cache.
func interBlockCacheOpt() func(*baseapp.BaseApp) {
	return baseapp.SetInterBlockCache(store.NewCommitKVStoreCacheManager())
}

// Profile with:
// /usr/local/go/bin/go test -benchmem -run=^$ github.com/ProvLabs/vault -bench ^BenchmarkFullAppSimulation$ -Commit=true -cpuprofile cpu.out
func TestAppImportExport(t *testing.T) {
	// uncomment to run in ide without flags.
	simcli.FlagEnabledValue = true
	//tempDir, err := os.MkdirTemp("", "sim-log-*")
	//require.NoError(t, err, "MkdirTemp")
	//t.Logf("tempDir: %s", tempDir)
	//simcli.FlagNumBlocksValue = 30
	//simcli.FlagVerboseValue = true
	//simcli.FlagCommitValue = true
	//simcli.FlagSeedValue = 2
	//simcli.FlagPeriodValue = 3
	//simcli.FlagExportParamsPathValue = filepath.Join(tempDir, fmt.Sprintf("sim_params-%d.json", simcli.FlagSeedValue))
	//simcli.FlagExportStatePathValue = filepath.Join(tempDir, fmt.Sprintf("sim_state-%d.json", simcli.FlagSeedValue))

	config, db, dir, logger, skip, err := setupSimulation("leveldb-app-sim", "Simulation")
	if skip {
		t.Skip("skipping application import/export simulation")
	}
	// printConfig(config)
	require.NoError(t, err, "simulation setup failed")

	defer func() {
		require.NoError(t, db.Close())
		require.NoError(t, os.RemoveAll(dir))
	}()

	appOpts := newSimAppOpts(t)
	baseAppOpts := []func(*baseapp.BaseApp){
		fauxMerkleModeOpt,
		baseapp.SetChainID(config.ChainID),
	}
	app, err := NewSimApp(logger, db, nil, true, appOpts, baseAppOpts...)
	require.NoError(t, err, "NewSimApp failed")
	require.Equal(t, "SimApp", app.Name())
	if !simcli.FlagSigverifyTxValue {
		app.SetNotSigverifyTx()
	}

	fmt.Printf("running provlabs vault test import export\n")

	// Run randomized simulation
	_, lastBlockTime, simParams, simErr := simulation.SimulateFromSeedProv(
		t,
		os.Stdout,
		app.BaseApp,
		appStateFn(app.AppCodec(), app.SimulationManager(), app.DefaultGenesis()),
		simtypes.RandomAccounts, // Replace with own random account function if using keys other than secp256k1
		simtestutil.SimulationOperations(app, app.AppCodec(), config),
		map[string]bool{}, // TODO: add custom module operations if needed
		config,
		app.AppCodec(),
	)

	// export state and simParams before the simulation error is checked
	err = simtestutil.CheckExportSimulation(app, config, simParams)
	require.NoError(t, err, "CheckExportSimulation")
	require.NoError(t, simErr, "SimulateFromSeedProv")

	fmt.Printf("exporting genesis...\n")

	exported, err := app.ExportAppStateAndValidators(false, nil, nil)
	require.NoError(t, err, "ExportAppStateAndValidators")

	fmt.Printf("importing genesis...\n")

	newDB, newDir, newLogger, _, err := simtestutil.SetupSimulation(config, "leveldb-app-sim-2", "Simulation-2", simcli.FlagVerboseValue, simcli.FlagEnabledValue)
	require.NoError(t, err, "simulation setup 2 failed")

	defer func() {
		require.NoError(t, newDB.Close())
		require.NoError(t, os.RemoveAll(newDir))
	}()

	// create a new temp dir for the app to fix wasmvm data dir lockfile contention
	appOpts = newSimAppOpts(t)
	newApp, err := NewSimApp(newLogger, newDB, nil, true, appOpts, baseAppOpts...)
	require.NoError(t, err, "NewSimApp 2 failed")

	var genesisState map[string]json.RawMessage
	err = json.Unmarshal(exported.AppState, &genesisState)
	require.NoError(t, err)

	ctxA := app.NewContextLegacy(true, cmtproto.Header{Height: app.LastBlockHeight(), Time: lastBlockTime})
	ctxB := newApp.NewContextLegacy(true, cmtproto.Header{Height: app.LastBlockHeight(), Time: lastBlockTime})
	_, err = newApp.App.ModuleManager.InitGenesis(ctxB, app.AppCodec(), genesisState)
	if err != nil {
		if strings.Contains(err.Error(), "validator set is empty after InitGenesis") {
			logger.Info("Skipping simulation as all validators have been unbonded")
			logger.Info("err", err, "stacktrace", string(debug.Stack()))
			return
		}
	}
	require.NoError(t, err, "InitGenesis")

	err = newApp.StoreConsensusParams(ctxB, exported.ConsensusParams)
	require.NoError(t, err, "StoreConsensusParams")

	fmt.Printf("comparing stores...\n")

	// skip certain prefixes
	skipPrefixes := map[string][][]byte{
		stakingtypes.StoreKey: {
			stakingtypes.UnbondingQueueKey, stakingtypes.RedelegationQueueKey, stakingtypes.ValidatorQueueKey,
			stakingtypes.HistoricalInfoKey, stakingtypes.UnbondingIDKey, stakingtypes.UnbondingIndexKey,
			stakingtypes.UnbondingTypeKey, stakingtypes.ValidatorUpdatesKey,
		},
		authzkeeper.StoreKey:   {authzkeeper.GrantQueuePrefix},
		feegrant.StoreKey:      {feegrant.FeeAllowanceQueueKeyPrefix},
		slashingtypes.StoreKey: {slashingtypes.ValidatorMissedBlockBitmapKeyPrefix},
	}

	storeKeys := app.GetStoreKeys()
	require.NotEmpty(t, storeKeys, "storeKeys")

	for _, appKeyA := range storeKeys {
		keyName := appKeyA.Name()
		t.Run(keyName, func(t *testing.T) {
			// only compare kvstores
			if _, ok := appKeyA.(*storetypes.KVStoreKey); !ok {
				t.Skipf("Skipping because the key is a %T (not a KVStoreKey)", appKeyA)
				return
			}

			appKeyB := newApp.GetKey(keyName)

			storeA := ctxA.KVStore(appKeyA)
			storeB := ctxB.KVStore(appKeyB)

			failedKVAs, failedKVBs := simtestutil.DiffKVStores(storeA, storeB, skipPrefixes[keyName])
			assert.Equal(t, len(failedKVAs), len(failedKVBs), "unequal sets of key-values to compare: %s", keyName)
			fmt.Printf("compared %d different key/value pairs between %s and %s\n", len(failedKVAs), appKeyA, appKeyB)

			// Make the lists the same length because GetSimulationLog assumes they're that way.
			for len(failedKVBs) < len(failedKVAs) {
				failedKVBs = append(failedKVBs, kv.Pair{Key: []byte{}, Value: []byte{}})
			}
			for len(failedKVBs) > len(failedKVAs) {
				failedKVAs = append(failedKVAs, kv.Pair{Key: []byte{}, Value: []byte{}})
			}

			assert.Equal(t, 0, len(failedKVAs), simtestutil.GetSimulationLog(keyName, app.SimulationManager().StoreDecoders, failedKVAs, failedKVBs))
		})
	}
}

func TestAppSimulationAfterImport(t *testing.T) {
	config, db, dir, logger, skip, err := setupSimulation("leveldb-app-sim", "Simulation")
	if skip {
		t.Skip("skipping application simulation after import")
	}
	require.NoError(t, err, "simulation setup failed")

	defer func() {
		require.NoError(t, db.Close())
		require.NoError(t, os.RemoveAll(dir))
	}()

	appOpts := newSimAppOpts(t)
	baseAppOpts := []func(*baseapp.BaseApp){
		fauxMerkleModeOpt,
		baseapp.SetChainID(config.ChainID),
	}

	app, err := NewSimApp(logger, db, nil, true, appOpts, baseAppOpts...)
	require.NoError(t, err, "NewSimApp failed")
	require.Equal(t, "SimApp", app.Name())
	if !simcli.FlagSigverifyTxValue {
		app.SetNotSigverifyTx()
	}

	// Run randomized simulation
	stopEarly, lastBlockTime, simParams, simErr := simulation.SimulateFromSeedProv(
		t,
		os.Stdout,
		app.BaseApp,
		appStateFn(app.AppCodec(), app.SimulationManager(), app.DefaultGenesis()),
		simtypes.RandomAccounts, // Replace with own random account function if using keys other than secp256k1
		simtestutil.SimulationOperations(app, app.AppCodec(), config),
		map[string]bool{}, // TODO: add custom module operations if needed,
		config,
		app.AppCodec(),
	)

	// export state and simParams before the simulation error is checked
	err = simtestutil.CheckExportSimulation(app, config, simParams)
	require.NoError(t, err, "CheckExportSimulation")
	require.NoError(t, simErr, "SimulateFromSeedProv")

	if stopEarly {
		fmt.Println("can't export or import a zero-validator genesis, exiting test...")
		return
	}

	fmt.Printf("exporting genesis...\n")

	exported, err := app.ExportAppStateAndValidators(false, nil, nil)
	require.NoError(t, err, "ExportAppStateAndValidators")

	fmt.Printf("importing genesis...\n")

	newDB, newDir, newLogger, _, err := simtestutil.SetupSimulation(config, "leveldb-app-sim-2", "Simulation-2", simcli.FlagVerboseValue, simcli.FlagEnabledValue)
	require.NoError(t, err, "simulation setup 2 failed")

	defer func() {
		require.NoError(t, newDB.Close())
		require.NoError(t, os.RemoveAll(newDir))
	}()

	// create a new temp dir for the app to fix wasmvm data dir lockfile contention
	appOpts = newSimAppOpts(t)
	newApp, err := NewSimApp(newLogger, newDB, nil, true, appOpts, baseAppOpts...)
	require.NoError(t, err, "NewSimApp 2 failed")

	_, err = newApp.InitChain(&abci.RequestInitChain{
		AppStateBytes: exported.AppState,
		ChainId:       config.ChainID,
		Time:          lastBlockTime,
	})
	require.NoError(t, err, "InitChain")

	simcli.FlagGenesisTimeValue = lastBlockTime.Unix()
	_, _, err = simulation.SimulateFromSeed(
		t,
		os.Stdout,
		newApp.BaseApp,
		appStateFn(app.AppCodec(), app.SimulationManager(), app.DefaultGenesis()),
		simtypes.RandomAccounts, // Replace with own random account function if using keys other than secp256k1
		simtestutil.SimulationOperations(newApp, newApp.AppCodec(), config),
		map[string]bool{}, // TODO: add custom module operations if needed,
		config,
		app.AppCodec(),
	)
	require.NoError(t, err)
}

func TestFullAppSimulation(t *testing.T) {
	config, db, dir, logger, skip, err := setupSimulation("leveldb-app-sim", "Simulation")
	if skip {
		t.Skip("skipping provlabs vault application simulation")
	}
	require.NoError(t, err, "provlabs vault simulation setup failed")

	defer func() {
		require.NoError(t, db.Close())
		require.NoError(t, os.RemoveAll(dir))
	}()

	appOpts := newSimAppOpts(t)
	baseAppOpts := []func(*baseapp.BaseApp){
		fauxMerkleModeOpt,
		baseapp.SetChainID(config.ChainID),
	}

	app, err := NewSimApp(logger, db, nil, true, appOpts, baseAppOpts...)
	require.NoError(t, err, "NewSimApp failed")
	require.Equal(t, "SimApp", app.Name())
	if !simcli.FlagSigverifyTxValue {
		app.SetNotSigverifyTx()
	}

	fmt.Printf("running provlabs vault full app simulation\n")

	// run randomized simulation
	_, simParams, simErr := simulation.SimulateFromSeed(
		t,
		os.Stdout,
		app.BaseApp,
		appStateFn(app.AppCodec(), app.SimulationManager(), app.DefaultGenesis()),
		simtypes.RandomAccounts, // Replace with own random account function if using keys other than secp256k1
		simtestutil.SimulationOperations(app, app.AppCodec(), config),
		map[string]bool{}, // TODO: add custom module operations if needed,
		config,
		app.AppCodec(),
	)

	// export state and simParams before the simulation error is checked
	err = simtestutil.CheckExportSimulation(app, config, simParams)
	require.NoError(t, err, "CheckExportSimulation")
	require.NoError(t, simErr, "SimulateFromSeed")
}

func TestSimple(t *testing.T) {
	simcli.FlagEnabledValue = true
	config, db, dir, logger, skip, err := setupSimulation("leveldb-app-sim", "Simulation")
	if skip {
		t.Skip("skipping provlabs vault application simulation")
	}
	require.NoError(t, err, "provlabs vault simulation setup failed")

	defer func() {
		require.NoError(t, db.Close())
		require.NoError(t, os.RemoveAll(dir))
	}()

	appOpts := newSimAppOpts(t)
	baseAppOpts := []func(*baseapp.BaseApp){
		fauxMerkleModeOpt,
		baseapp.SetChainID(config.ChainID),
	}

	app, err := NewSimApp(logger, db, nil, true, appOpts, baseAppOpts...)
	require.NoError(t, err, "NewSimApp failed")
	require.Equal(t, "SimApp", app.Name())
	if !simcli.FlagSigverifyTxValue {
		app.SetNotSigverifyTx()
	}

	// run randomized simulation
	_, _, simErr := simulation.SimulateFromSeed(
		t,
		os.Stdout,
		app.BaseApp,
		appStateFn(app.AppCodec(), app.SimulationManager(), app.DefaultGenesis()),
		simtypes.RandomAccounts, // Replace with own random account function if using keys other than secp256k1
		simtestutil.SimulationOperations(app, app.AppCodec(), config),
		map[string]bool{}, // TODO: add custom module operations if needed,
		config,
		app.AppCodec(),
	)

	require.NoError(t, simErr, "SimulateFromSeed")
}

func TestAppStateDeterminism(t *testing.T) {
	// uncomment these to run in ide without flags.
	simcli.FlagEnabledValue = true
	//simcli.FlagBlockSizeValue = 100
	//simcli.FlagNumBlocksValue = 50

	if !simcli.FlagEnabledValue {
		t.Skip("skipping application simulation")
	}

	config := simcli.NewConfigFromFlags()
	config.InitialBlockHeight = 1
	config.ExportParamsPath = ""
	config.OnOperation = false
	config.AllInvariants = false
	config.ChainID = "simulation-app"
	config.DBBackend = "memdb"
	config.Commit = true

	numSeeds := 3
	numTimesToRunPerSeed := 5
	appHashList := make([]json.RawMessage, numTimesToRunPerSeed)

	var seeds []int64
	if config.Seed != simcli.DefaultSeedValue {
		// If a seed was provided, just do that one.
		numSeeds = 1
		seeds = append(seeds, config.Seed)
	} else {
		// Otherwise, pick random seeds to use.
		seeds = make([]int64, numSeeds)
		for i := range seeds {
			seeds[i] = rand.Int63()
		}
	}

	for i, seed := range seeds {
		config.Seed = seed

		for j := 0; j < numTimesToRunPerSeed; j++ {
			var logger log.Logger
			if simcli.FlagVerboseValue {
				logger = log.NewTestLogger(t)
			} else {
				logger = log.NewNopLogger()
			}

			// create a new temp dir for the app to fix wasmvm data dir lockfile contention
			appOpts := newSimAppOpts(t)
			if simcli.FlagVerboseValue {
				appOpts[flags.FlagLogLevel] = "debug"
			}

			db := dbm.NewMemDB()
			app, err := NewSimApp(logger, db, nil, true, appOpts, interBlockCacheOpt(), baseapp.SetChainID(config.ChainID))
			require.NoError(t, err, "NewSimApp failed")
			require.Equal(t, "SimApp", app.Name())
			if !simcli.FlagSigverifyTxValue {
				app.SetNotSigverifyTx()
			}

			fmt.Printf(
				"running provlabs vault non-determinism simulation; seed %d: %d/%d, attempt: %d/%d\n",
				config.Seed, i+1, numSeeds, j+1, numTimesToRunPerSeed,
			)

			_, _, err = simulation.SimulateFromSeed(
				t,
				os.Stdout,
				app.BaseApp,
				appStateFn(app.AppCodec(), app.SimulationManager(), app.DefaultGenesis()),
				simtypes.RandomAccounts, // Replace with own random account function if using keys other than secp256k1
				simtestutil.SimulationOperations(app, app.AppCodec(), config),
				map[string]bool{}, // TODO: add custom module operations if needed,
				config,
				app.AppCodec(),
			)
			require.NoError(t, err)

			appHash := app.LastCommitID().Hash
			appHashList[j] = appHash

			if j != 0 {
				require.Equal(
					t, string(appHashList[0]), string(appHashList[j]),
					"non-determinism in seed %d: %d/%d, attempt: %d/%d\n", config.Seed, i+1, numSeeds, j+1, numTimesToRunPerSeed,
				)
			}
		}
	}
}
