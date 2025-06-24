package simapp

import (
	_ "embed"
	"io"
	"os"
	"path/filepath"

	"cosmossdk.io/core/appconfig"
	"cosmossdk.io/depinject"
	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	_ "cosmossdk.io/x/feegrant/module"
	"cosmossdk.io/x/tx/signing"
	_ "cosmossdk.io/x/upgrade"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	_ "github.com/cosmos/cosmos-sdk/x/auth"
	_ "github.com/cosmos/cosmos-sdk/x/auth/tx/config"
	_ "github.com/cosmos/cosmos-sdk/x/authz/module"
	_ "github.com/cosmos/cosmos-sdk/x/bank"
	_ "github.com/cosmos/cosmos-sdk/x/consensus"
	_ "github.com/cosmos/cosmos-sdk/x/group/module"
	_ "github.com/cosmos/cosmos-sdk/x/params"
	_ "github.com/cosmos/cosmos-sdk/x/staking"
	"google.golang.org/protobuf/proto"

	// Cosmos Modules
	feegrantkeeper "cosmossdk.io/x/feegrant/keeper"
	upgradekeeper "cosmossdk.io/x/upgrade/keeper"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	consensuskeeper "github.com/cosmos/cosmos-sdk/x/consensus/keeper"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	groupkeeper "github.com/cosmos/cosmos-sdk/x/group/keeper"
	paramskeeper "github.com/cosmos/cosmos-sdk/x/params/keeper"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"

	// IBC Modules
	capabilitykeeper "github.com/cosmos/ibc-go/modules/capability/keeper"
	ibckeeper "github.com/cosmos/ibc-go/v8/modules/core/keeper"

	// Provenance Modules
	attributekeeper "github.com/provenance-io/provenance/x/attribute/keeper"
	markerkeeper "github.com/provenance-io/provenance/x/marker/keeper"
	namekeeper "github.com/provenance-io/provenance/x/name/keeper"

	// Custom Modules
	_ "github.com/provlabs/vault"
	vaultkeeper "github.com/provlabs/vault/keeper"
)

var DefaultNodeHome string

//go:embed app.yaml
var AppConfigYAML []byte

var (
	_ runtime.AppI            = (*SimApp)(nil)
	_ servertypes.Application = (*SimApp)(nil)
)

// SimApp extends an ABCI application, but with most of its parameters exported.
// They are exported for convenience in creating helper functions, as object
// capabilities aren't needed for testing.
type SimApp struct {
	*runtime.App
	legacyAmino       *codec.LegacyAmino
	appCodec          codec.Codec
	txConfig          client.TxConfig
	interfaceRegistry codectypes.InterfaceRegistry

	// Cosmos Modules
	AccountKeeper   authkeeper.AccountKeeper
	BankKeeper      bankkeeper.Keeper
	ConsensusKeeper consensuskeeper.Keeper
	ParamsKeeper    paramskeeper.Keeper
	StakingKeeper   *stakingkeeper.Keeper
	UpgradeKeeper   *upgradekeeper.Keeper

	AuthzKeeper    authzkeeper.Keeper
	FeegrantKeeper feegrantkeeper.Keeper
	GroupKeeper    groupkeeper.Keeper
	// TODO We want to set this up
	GovKeeper govkeeper.Keeper

	// IBC Modules
	CapabilityKeeper *capabilitykeeper.Keeper
	IBCKeeper        *ibckeeper.Keeper
	// Provenance Modules
	NameKeeper      namekeeper.Keeper
	AttributeKeeper attributekeeper.Keeper
	MarkerKeeper    markerkeeper.Keeper
	// ExchangeKeeper  exchangekeeper.Keeper
	// Custom Modules
	VaultKeeper *vaultkeeper.Keeper
}

func init() {
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	DefaultNodeHome = filepath.Join(userHomeDir, ".simapp")
}

// ProvideExchangeDummyCustomSigners returns a slice of dummy CustomGetSigner functions
// for the exchange module. These are used to register placeholder signer functions for
// specific message types (e.g., MsgAcceptPaymentRequest, MsgCreatePaymentRequest) to
// satisfy the signing context validation during app initialization.
func ProvideExchangeDummyCustomSigners() []signing.CustomGetSigner {
	return []signing.CustomGetSigner{
		signing.CustomGetSigner{
			MsgType: "provenance.exchange.v1.MsgAcceptPaymentRequest",
			Fn:      func(proto.Message) ([][]byte, error) { return [][]byte{}, nil },
		},
		signing.CustomGetSigner{
			MsgType: "provenance.exchange.v1.MsgCreatePaymentRequest",
			Fn:      func(proto.Message) ([][]byte, error) { return [][]byte{}, nil },
		},
	}
}

// AppConfig returns the default app config.
func AppConfig() depinject.Config {
	return depinject.Configs(
		appconfig.LoadYAML(AppConfigYAML),
		depinject.Provide(ProvideExchangeDummyCustomSigners, ProvideMarkerKeeperStub),
		depinject.Supply(
			map[string]module.AppModuleBasic{
				genutiltypes.ModuleName: genutil.NewAppModuleBasic(genutiltypes.DefaultMessageValidator),
			},
		),
	)
}

// NewSimApp returns a reference to an initialized SimApp.
func NewSimApp(
	logger log.Logger,
	db dbm.DB,
	traceStore io.Writer,
	loadLatest bool,
	appOpts servertypes.AppOptions,
	baseAppOptions ...func(*baseapp.BaseApp),
) (*SimApp, error) {
	var (
		app        = &SimApp{}
		appBuilder *runtime.AppBuilder
	)

	if err := depinject.Inject(
		depinject.Configs(
			AppConfig(),
			depinject.Supply(
				logger,
				appOpts,
			),
		),
		&appBuilder,
		&app.appCodec,
		&app.legacyAmino,
		&app.txConfig,
		&app.interfaceRegistry,
		// Cosmos Modules
		&app.AccountKeeper,
		&app.BankKeeper,
		&app.ConsensusKeeper,
		&app.ParamsKeeper,
		&app.StakingKeeper,
		&app.UpgradeKeeper,
		&app.AuthzKeeper,
		&app.FeegrantKeeper,
		&app.GroupKeeper,

		// ProvLabs Vault Module
		&app.VaultKeeper,
	); err != nil {
		return nil, err
	}

	app.App = appBuilder.Build(db, traceStore, baseAppOptions...)

	if err := app.RegisterLegacyModules(); err != nil {
		return nil, err
	}

	if err := app.RegisterProvenanceModules(); err != nil {
		return nil, err
	}

	if err := app.RegisterStreamingServices(appOpts, app.kvStoreKeys()); err != nil {
		return nil, err
	}

	if err := app.Load(loadLatest); err != nil {
		return nil, err
	}

	return app, nil
}

func (app *SimApp) LegacyAmino() *codec.LegacyAmino {
	return app.legacyAmino
}

func (app *SimApp) SimulationManager() *module.SimulationManager {
	return nil
}

func (app *SimApp) GetKey(storeKey string) *storetypes.KVStoreKey {
	key, _ := app.UnsafeFindStoreKey(storeKey).(*storetypes.KVStoreKey)
	return key
}

func (app *SimApp) GetMemKey(memKey string) *storetypes.MemoryStoreKey {
	key, _ := app.UnsafeFindStoreKey(memKey).(*storetypes.MemoryStoreKey)
	return key
}

func (app *SimApp) GetSubspace(moduleName string) paramstypes.Subspace {
	subspace, _ := app.ParamsKeeper.GetSubspace(moduleName)
	return subspace
}

func (app *SimApp) kvStoreKeys() map[string]*storetypes.KVStoreKey {
	keys := make(map[string]*storetypes.KVStoreKey)
	for _, k := range app.GetStoreKeys() {
		if kv, ok := k.(*storetypes.KVStoreKey); ok {
			keys[kv.Name()] = kv
		}
	}
	return keys
}

// ProvideMarkerKeeperStub returns an empty markerkeeper.Keeper instance.
//
// This stub is used to satisfy dependency injection requirements when wiring
// the Vault module in the app module configuration. It allows the Vault module
// to be included in the dependency graph even though the actual MarkerKeeper
// is initialized separately using the legacy Provenance wiring in SimApp.
//
// This function should only be used during app setup and should not be relied
// on at runtime, as the returned keeper is not fully configured and will panic
// if used.
func ProvideMarkerKeeperStub() *markerkeeper.Keeper {
	return &markerkeeper.Keeper{}
}

func (app *SimApp) AppCodec() codec.Codec {
	return app.appCodec
}
