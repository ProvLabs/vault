package vault

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	modulev1 "github.com/provlabs/vault/api/module/v1"
	vaultv1 "github.com/provlabs/vault/api/v1"
	"github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/types"

	autocliv1 "cosmossdk.io/api/cosmos/autocli/v1"
	"cosmossdk.io/core/address"
	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/core/event"
	"cosmossdk.io/core/header"
	"cosmossdk.io/core/store"
	"cosmossdk.io/depinject"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

// ConsensusVersion defines the current x/vault module consensus version.
const ConsensusVersion = 1

var (
	_ module.AppModuleBasic      = AppModule{}
	_ appmodule.AppModule        = AppModule{}
	_ module.HasConsensusVersion = AppModule{}
	_ module.HasGenesis          = AppModule{}
	_ module.HasGenesisBasics    = AppModuleBasic{}
	_ module.HasServices         = AppModule{}
)

// AppModuleBasic implements the basic methods for the vault module.
type AppModuleBasic struct{}

// NewAppModuleBasic creates a new AppModuleBasic.
func NewAppModuleBasic() AppModuleBasic {
	return AppModuleBasic{}
}

// Name returns the vault module name.
func (AppModuleBasic) Name() string { return types.ModuleName }

// RegisterLegacyAminoCodec registers the legacy amino codec.
func (AppModuleBasic) RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	types.RegisterLegacyAminoCodec(cdc)
}

// RegisterInterfaces registers vault interfaces to the interface registry.
func (AppModuleBasic) RegisterInterfaces(reg codectypes.InterfaceRegistry) {
	types.RegisterInterfaces(reg)

	reg.RegisterInterface(
		"vault.v1.VaultAccount",
		(*types.VaultAccountI)(nil),
		&types.VaultAccount{},
	)
	reg.RegisterInterface(
		"vault.v1.VaultAccount",
		(*sdk.AccountI)(nil),
		&types.VaultAccount{},
	)
	reg.RegisterInterface(
		"vault.v1.VaultAccount",
		(*authtypes.GenesisAccount)(nil),
		&types.VaultAccount{},
	)
}

// RegisterGRPCGatewayRoutes sets up gRPC gateway routes.
func (AppModuleBasic) RegisterGRPCGatewayRoutes(clientCtx client.Context, mux *runtime.ServeMux) {
	if err := types.RegisterQueryHandlerClient(context.Background(), mux, types.NewQueryClient(clientCtx)); err != nil {
		panic(err)
	}
}

// DefaultGenesis returns default genesis state as raw bytes.
func (AppModuleBasic) DefaultGenesis(cdc codec.JSONCodec) json.RawMessage {
	return cdc.MustMarshalJSON(types.DefaultGenesisState())
}

// ValidateGenesis validates the vault genesis state.
func (AppModuleBasic) ValidateGenesis(cdc codec.JSONCodec, _ client.TxEncodingConfig, bz json.RawMessage) error {
	var genesis types.GenesisState
	if err := cdc.UnmarshalJSON(bz, &genesis); err != nil {
		return fmt.Errorf("failed to unmarshal %s genesis state: %w", types.ModuleName, err)
	}
	return genesis.Validate()
}

// AppModule implements the core vault module functionality.
type AppModule struct {
	AppModuleBasic
	keeper       *keeper.Keeper
	addressCodec address.Codec
}

// NewAppModule creates a new AppModule instance.
func NewAppModule(keeper *keeper.Keeper, addressCodec address.Codec) AppModule {
	return AppModule{
		AppModuleBasic: NewAppModuleBasic(),
		keeper:         keeper,
		addressCodec:   addressCodec,
	}
}

// IsOnePerModuleType asserts one module per type.
func (AppModule) IsOnePerModuleType() {}

// IsAppModule asserts this is an app module.
func (AppModule) IsAppModule() {}

// ConsensusVersion returns the module consensus version.
func (AppModule) ConsensusVersion() uint64 { return ConsensusVersion }

// InitGenesis initializes the module's state from genesis.
func (m AppModule) InitGenesis(ctx sdk.Context, cdc codec.JSONCodec, bz json.RawMessage) {
	var genesis types.GenesisState
	cdc.MustUnmarshalJSON(bz, &genesis)
	m.keeper.InitGenesis(ctx, &genesis)
}

// ExportGenesis exports the module's state to genesis.
func (m AppModule) ExportGenesis(ctx sdk.Context, cdc codec.JSONCodec) json.RawMessage {
	genesis := m.keeper.ExportGenesis(ctx)
	return cdc.MustMarshalJSON(genesis)
}

// BeginBlock returns the begin blocker for the vault module.
func (m AppModule) BeginBlock(ctx context.Context) error {
	return m.keeper.BeginBlocker(ctx)
}

// EndBlock returns the end blocker for the vault module.
func (m AppModule) EndBlock(ctx context.Context) error {
	return m.keeper.EndBlocker(ctx)
}

// RegisterServices registers gRPC query and message services.
func (m AppModule) RegisterServices(cfg module.Configurator) {
	types.RegisterMsgServer(cfg.MsgServer(), keeper.NewMsgServer(m.keeper))
	types.RegisterQueryServer(cfg.QueryServer(), keeper.NewQueryServer(m.keeper))
}

// AutoCLIOptions defines CLI commands for tx and query.
func (AppModule) AutoCLIOptions() *autocliv1.ModuleOptions {
	return &autocliv1.ModuleOptions{
		Tx: &autocliv1.ServiceCommandDescriptor{
			Service: vaultv1.Msg_ServiceDesc.ServiceName,
			RpcCommandOptions: []*autocliv1.RpcCommandOptions{
				{
					RpcMethod: "CreateVault",
					Use:       "create [admin] [underlying_asset] [share_denom]",
					Alias:     []string{"c", "new"},
					Short:     "Create a new vault",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "admin"},
						{ProtoField: "underlying_asset"},
						{ProtoField: "share_denom"},
					},
				},
				{
					RpcMethod: "SwapIn",
					Use:       "swap-in [owner] [vault_address] [assets]",
					Alias:     []string{"si"},
					Short:     "Deposit underlying assets into a vault to mint shares",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "owner"},
						{ProtoField: "vault_address"},
						{ProtoField: "assets"},
					},
				},
				{
					RpcMethod: "SwapOut",
					Use:       "swap-out [owner] [vault_address] [assets]",
					Alias:     []string{"so"},
					Short:     "Withdraw underlying assets from a vault by burning shares",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "owner"},
						{ProtoField: "vault_address"},
						{ProtoField: "assets"},
					},
				},
				{
					RpcMethod: "UpdateInterestRate",
					Use:       "update-interest-rate [admin] [vault_address] [new_rate]",
					Alias:     []string{"uir"},
					Short:     "Update the current interest rate for a vault",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "admin"},
						{ProtoField: "vault_address"},
						{ProtoField: "new_rate"},
					},
				},
				{
					RpcMethod: "UpdateMinInterestRate",
					Use:       "update-min-interest-rate [admin] [vault_address] [min_rate]",
					Alias:     []string{"umir"},
					Short:     "Set or clear the minimum allowable interest rate for a vault. For example, an interest rate of 90% must be entered as '0.9'.",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "admin"},
						{ProtoField: "vault_address"},
						{ProtoField: "min_rate"},
					},
				},
				{
					RpcMethod: "UpdateMaxInterestRate",
					Use:       "update-max-interest-rate [admin] [vault_address] [max_rate]",
					Alias:     []string{"umaxir"},
					Short:     "Set or clear the maximum allowable interest rate for a vault",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "admin"},
						{ProtoField: "vault_address"},
						{ProtoField: "max_rate"},
					},
				},
				{
					RpcMethod: "DepositInterestFunds",
					Use:       "deposit-interest-funds [admin] [vault_address] [amount]",
					Alias:     []string{"dif"},
					Short:     "Deposit funds into a vault for paying interest",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "admin"},
						{ProtoField: "vault_address"},
						{ProtoField: "amount"},
					},
				},
				{
					RpcMethod: "WithdrawInterestFunds",
					Use:       "withdraw-interest-funds [admin] [vault_address] [amount]",
					Alias:     []string{"wif"},
					Short:     "Withdraw unused interest funds from a vault",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "admin"},
						{ProtoField: "vault_address"},
						{ProtoField: "amount"},
					},
				},
				{
					RpcMethod: "DepositPrincipalFunds",
					Use:       "deposit-principal-funds [admin] [vault_address] [amount]",
					Alias:     []string{"dpf"},
					Short:     "Deposit principal funds into the vault’s marker",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "admin"},
						{ProtoField: "vault_address"},
						{ProtoField: "amount"},
					},
				},
				{
					RpcMethod: "WithdrawPrincipalFunds",
					Use:       "withdraw-principal-funds [admin] [vault_address] [amount]",
					Alias:     []string{"wpf"},
					Short:     "Withdraw principal funds from the vault’s marker",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "admin"},
						{ProtoField: "vault_address"},
						{ProtoField: "amount"},
					},
				},
				{
					RpcMethod: "ToggleSwapIn",
					Use:       "toggle-swap-in [admin] [vault_address] [enabled]",
					Alias:     []string{"tsi"},
					Short:     "Enable or disable swap-in operations for a vault",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "admin"},
						{ProtoField: "vault_address"},
						{ProtoField: "enabled"},
					},
				},
				{
					RpcMethod: "ToggleSwapOut",
					Use:       "toggle-swap-out [admin] [vault_address] [enabled]",
					Alias:     []string{"tso"},
					Short:     "Enable or disable swap-out operations for a vault",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "admin"},
						{ProtoField: "vault_address"},
						{ProtoField: "enabled"},
					},
				},
			},
		},

		Query: &autocliv1.ServiceCommandDescriptor{
			Service: vaultv1.Query_ServiceDesc.ServiceName,
			RpcCommandOptions: []*autocliv1.RpcCommandOptions{
				{
					RpcMethod: "Vaults",
					Use:       "list",
					Alias:     []string{"l", "ls"},
					Short:     "Query all vaults",
				},
				{
					RpcMethod: "Vault",
					Use:       "get [vault_address]",
					Alias:     []string{"g"},
					Short:     "Query a specific vault's configuration and state",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "vault_address"},
					},
				},
				{
					RpcMethod: "EstimateSwapIn",
					Use:       "estimate-swap-in [vault_address] [assets]",
					Alias:     []string{"esi"},
					Short:     "Estimate the number of shares received for a given deposit",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "vault_address"},
						{ProtoField: "assets"},
					},
				},
				{
					RpcMethod: "EstimateSwapOut",
					Use:       "estimate-swap-out [vault_address] [assets]",
					Alias:     []string{"eso"},
					Short:     "Estimate the amount of underlying assets received for a given withdrawal",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "vault_address"},
						{ProtoField: "assets"},
					},
				},
			},
		},
	}
}

// init registers the vault module with the appmodule framework.
func init() {
	appmodule.Register(&modulev1.Module{}, appmodule.Provide(ProvideModule))
}

// ModuleInputs defines the inputs required to initialize the vault module.
type ModuleInputs struct {
	depinject.In
	Config        *modulev1.Module
	StoreService  store.KVStoreService
	HeaderService header.Service
	EventService  event.Service
	Codec         codec.Codec
	AddressCodec  address.Codec
	AuthKeeper    types.AccountKeeper
	MarkerKeeper  types.MarkerKeeper
	BankKeeper    types.BankKeeper
}

// ModuleOutputs defines the outputs of the vault module provider.
type ModuleOutputs struct {
	depinject.Out
	Keeper *keeper.Keeper
	Module appmodule.AppModule
}

// ProvideModule wires up the vault module and its keeper.
func ProvideModule(in ModuleInputs) ModuleOutputs {
	authority := authtypes.NewModuleAddress(types.GovModuleName)
	if in.Config.Authority != "" {
		authority = authtypes.NewModuleAddressOrBech32Address(in.Config.Authority)
	}

	k := keeper.NewKeeper(
		in.Codec,
		in.StoreService,
		in.EventService,
		in.AddressCodec,
		authority,
		in.AuthKeeper,
		in.MarkerKeeper,
		in.BankKeeper,
	)
	m := NewAppModule(k, in.AddressCodec)
	return ModuleOutputs{Keeper: k, Module: m}
}
