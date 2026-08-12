package keeper_test

import (
	"github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (s *TestSuite) TestTotalValueInvariant() {
	underlyingDenom := "ylds"
	heldDenom := "usdc"
	shareDenom := "vshare"

	tests := []struct {
		name          string
		setup         func() sdk.AccAddress
		expBroken     bool
		expContains   []string
		expNamesVault bool
	}{
		{
			name:      "no vaults exist, nothing to check",
			setup:     func() sdk.AccAddress { return nil },
			expBroken: false,
		},
		{
			name: "freshly created vault holds nothing",
			setup: func() sdk.AccAddress {
				return s.setupBaseVault(underlyingDenom, shareDenom).GetAddress()
			},
			expBroken: false,
		},
		{
			name: "vault holds underlying and a priced held asset",
			setup: func() sdk.AccAddress {
				vault := s.setupHeldAssetVault(underlyingDenom, shareDenom, heldDenom, 1, 2)
				s.fundPrincipal(vault,
					sdk.NewInt64Coin(underlyingDenom, 1_000),
					sdk.NewInt64Coin(heldDenom, 10),
				)
				return vault.GetAddress()
			},
			expBroken: false,
		},
		{
			name: "held asset repriced after the vault acquired it",
			setup: func() sdk.AccAddress {
				vault := s.setupHeldAssetVault(underlyingDenom, shareDenom, heldDenom, 1, 2)
				s.fundPrincipal(vault, sdk.NewInt64Coin(heldDenom, 10))
				s.setVaultNAV(vault, heldDenom, sdk.NewInt64Coin(underlyingDenom, 7), 3)
				return vault.GetAddress()
			},
			expBroken: false,
		},
		{
			name: "paused vault still tracks its balances",
			setup: func() sdk.AccAddress {
				vault := s.setupBaseVault(underlyingDenom, shareDenom)
				s.fundPrincipal(vault, sdk.NewInt64Coin(underlyingDenom, 500))
				s.pauseVault(vault.GetAddress())
				return vault.GetAddress()
			},
			expBroken: false,
		},
		{
			name: "stored total drifted above the derived value",
			setup: func() sdk.AccAddress {
				vault := s.setupBaseVault(underlyingDenom, shareDenom)
				s.fundPrincipal(vault, sdk.NewInt64Coin(underlyingDenom, 1_000))
				s.setStoredTotalValue(vault, sdkmath.NewInt(1_500))
				return vault.GetAddress()
			},
			expBroken:   true,
			expContains: []string{"1500 != derived 1000", "drift 500"},
		},
		{
			name: "stored total drifted below the derived value",
			setup: func() sdk.AccAddress {
				vault := s.setupBaseVault(underlyingDenom, shareDenom)
				s.fundPrincipal(vault, sdk.NewInt64Coin(underlyingDenom, 1_000))
				s.setStoredTotalValue(vault, sdkmath.NewInt(400))
				return vault.GetAddress()
			},
			expBroken:   true,
			expContains: []string{"400 != derived 1000", "drift -600"},
		},
		{
			name: "stored total is missing entirely",
			setup: func() sdk.AccAddress {
				vault := s.setupBaseVault(underlyingDenom, shareDenom)
				s.Require().NoError(s.k.TotalValues.Remove(s.ctx, vault.GetAddress()),
					"dropping the stored total for vault %s should succeed", vault.GetAddress())
				return vault.GetAddress()
			},
			expBroken:   true,
			expContains: []string{"no materialized total value stored"},
		},
		{
			name: "stored total is negative",
			setup: func() sdk.AccAddress {
				vault := s.setupBaseVault(underlyingDenom, shareDenom)
				s.setStoredTotalValue(vault, sdkmath.NewInt(-1))
				return vault.GetAddress()
			},
			expBroken:   true,
			expContains: []string{"is negative"},
		},
		{
			name: "lookup entry with no vault account behind it is counted as skipped, not reported broken",
			setup: func() sdk.AccAddress {
				orphan := sdk.AccAddress("orphanVaultAddress__")
				s.Require().NoError(s.k.Vaults.Set(s.ctx, orphan, []byte{}),
					"seeding an orphaned vault lookup entry should succeed")
				return orphan
			},
			expBroken:   false,
			expContains: []string{"1 unresolvable lookup entries skipped"},
		},
		{
			name: "lookup entry over an account that is not a vault is counted as skipped, not reported broken",
			setup: func() sdk.AccAddress {
				intruder := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1))
				s.Require().NoError(s.k.Vaults.Set(s.ctx, intruder, []byte{}),
					"seeding a lookup entry over a non-vault account should succeed")
				return intruder
			},
			expBroken:   false,
			expContains: []string{"1 unresolvable lookup entries skipped"},
		},
		{
			name: "a healthy vault is still checked alongside an unresolvable lookup entry",
			setup: func() sdk.AccAddress {
				vault := s.setupBaseVault(underlyingDenom, shareDenom)
				s.fundPrincipal(vault, sdk.NewInt64Coin(underlyingDenom, 1_000))
				s.setStoredTotalValue(vault, sdkmath.NewInt(1_500))
				s.Require().NoError(s.k.Vaults.Set(s.ctx, sdk.AccAddress("orphanVaultAddress__"), []byte{}),
					"seeding an orphaned vault lookup entry should succeed")
				return vault.GetAddress()
			},
			expBroken:   true,
			expContains: []string{"found 1 total-value violation(s)", "1 unresolvable lookup entries skipped", "drift 500"},
		},
		{
			name: "an unvaluable vault with a stored total is reported as uncheckable, not broken",
			setup: func() sdk.AccAddress {
				vault, _, underlying, held := s.setupOversizedNAVVault()
				s.seedOversizedNAV(vault, held, underlying, maxValidNAVPrice(), sdkmath.OneInt())
				s.fundPrincipalForBrokenValuation(vault, sdk.NewCoins(
					sdk.NewInt64Coin(held, 1),
					sdk.NewInt64Coin(underlying, 100),
				))
				s.setStoredTotalValue(vault, sdkmath.NewInt(100))
				return vault.GetAddress()
			},
			expBroken: false,
			expContains: []string{
				"1 vault(s) could not be checked",
				"total value cannot be derived, materialized total value is 100",
			},
			expNamesVault: true,
		},
		{
			name: "an unvaluable vault with no stored total is named as uncheckable, so a skipped hydration is visible without halting",
			setup: func() sdk.AccAddress {
				vault, _, underlying, held := s.setupOversizedNAVVault()
				s.seedOversizedNAV(vault, held, underlying, maxValidNAVPrice(), sdkmath.OneInt())
				s.fundPrincipalForBrokenValuation(vault, sdk.NewCoins(
					sdk.NewInt64Coin(held, 1),
					sdk.NewInt64Coin(underlying, 100),
				))
				return vault.GetAddress()
			},
			expBroken: false,
			expContains: []string{
				"1 vault(s) could not be checked",
				"total value cannot be derived, no materialized total value stored",
			},
			expNamesVault: true,
		},
		{
			name: "a drifted vault is still reported alongside an uncheckable one",
			setup: func() sdk.AccAddress {
				drifted := s.setupBaseVault("uusd", "vsharedrifted")
				s.fundPrincipal(drifted, sdk.NewInt64Coin("uusd", 1_000))

				unvaluable, _, unvaluableUnderlying, held := s.setupOversizedNAVVault()
				s.seedOversizedNAV(unvaluable, held, unvaluableUnderlying, maxValidNAVPrice(), sdkmath.OneInt())
				s.fundPrincipalForBrokenValuation(unvaluable, sdk.NewCoins(
					sdk.NewInt64Coin(held, 1),
					sdk.NewInt64Coin(unvaluableUnderlying, 100),
				))

				s.setStoredTotalValue(drifted, sdkmath.NewInt(1_500))
				return drifted.GetAddress()
			},
			expBroken: true,
			expContains: []string{
				"found 1 total-value violation(s)",
				"drift 500",
				"1 vault(s) could not be checked",
			},
			expNamesVault: true,
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			vaultAddr := tc.setup()

			msg, broken := keeper.TotalValueInvariant(s.k)(s.ctx)

			s.Require().Equal(tc.expBroken, broken, "invariant broken state mismatch, message was: %s", msg)
			for _, want := range tc.expContains {
				s.Require().Contains(msg, want, "invariant message should report %q", want)
			}
			if (tc.expBroken || tc.expNamesVault) && vaultAddr != nil {
				s.Require().Contains(msg, vaultAddr.String(), "invariant message should name vault %s", vaultAddr)
			}
		})
	}
}

func (s *TestSuite) TestTotalValueInvariant_ReportsEveryOffendingVault() {
	first := s.setupBaseVault("ylds", "vshareone")
	second := s.setupBaseVault("uusd", "vsharetwo")

	s.setStoredTotalValue(first, sdkmath.NewInt(11))
	s.setStoredTotalValue(second, sdkmath.NewInt(22))

	msg, broken := keeper.TotalValueInvariant(s.k)(s.ctx)

	s.Require().True(broken, "two drifted vaults should break the invariant")
	s.Require().Contains(msg, "found 2 total-value violation(s)", "invariant should count both violations")
	s.Require().Contains(msg, first.GetAddress().String(), "invariant should name the first drifted vault")
	s.Require().Contains(msg, second.GetAddress().String(), "invariant should name the second drifted vault")
}

func (s *TestSuite) TestShareSupplyInvariant() {
	underlyingDenom := "ylds"
	shareDenom := "vshare"

	tests := []struct {
		name        string
		setup       func() sdk.AccAddress
		expBroken   bool
		expContains []string
	}{
		{
			name: "vault has issued no shares",
			setup: func() sdk.AccAddress {
				return s.setupBaseVault(underlyingDenom, shareDenom).GetAddress()
			},
			expBroken: false,
		},
		{
			name: "swap-in mints supply and records it in total shares",
			setup: func() sdk.AccAddress {
				vault := s.setupBaseVault(underlyingDenom, shareDenom)
				owner := s.CreateAndFundAccount(sdk.NewInt64Coin(underlyingDenom, 10_000))
				_, err := s.k.SwapIn(s.ctx, vault.GetAddress(), owner, sdk.NewInt64Coin(underlyingDenom, 5_000))
				s.Require().NoError(err, "swap-in should succeed")
				return vault.GetAddress()
			},
			expBroken: false,
		},
		{
			name: "bridged-out shares leave total shares above local supply",
			setup: func() sdk.AccAddress {
				bridge := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1))
				return s.setupBridgeVault("bridgeu", "bridgeshare", bridge, sdkmath.NewInt(1_000_000)).GetAddress()
			},
			expBroken: false,
		},
		{
			name: "minted supply exceeds total shares",
			setup: func() sdk.AccAddress {
				vault := s.setupBaseVault(underlyingDenom, shareDenom)
				s.Require().NoError(
					s.k.MarkerKeeper.MintCoin(s.ctx, vault.GetAddress(), sdk.NewInt64Coin(shareDenom, 1_000)),
					"minting shares outside the vault's accounting should succeed",
				)
				return vault.GetAddress()
			},
			expBroken:   true,
			expContains: []string{"share supply 1000vshare exceeds total shares 0", "by 1000"},
		},
		{
			name: "total shares is negative",
			setup: func() sdk.AccAddress {
				vault := s.setupBaseVault(underlyingDenom, shareDenom)
				vault.TotalShares = sdk.Coin{Denom: shareDenom, Amount: sdkmath.NewInt(-1)}
				s.k.AuthKeeper.SetAccount(s.ctx, vault)
				return vault.GetAddress()
			},
			expBroken:   true,
			expContains: []string{"total shares -1vshare is negative"},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			vaultAddr := tc.setup()

			msg, broken := keeper.ShareSupplyInvariant(s.k)(s.ctx)

			s.Require().Equal(tc.expBroken, broken, "invariant broken state mismatch, message was: %s", msg)
			for _, want := range tc.expContains {
				s.Require().Contains(msg, want, "invariant message should report %q", want)
			}
			if tc.expBroken {
				s.Require().Contains(msg, vaultAddr.String(), "invariant message should name the offending vault")
			}
		})
	}
}

func (s *TestSuite) TestEscrowedSharesInvariant() {
	underlyingDenom := "ylds"
	shareDenom := "vshare"
	assets := sdk.NewInt64Coin(underlyingDenom, 5_000)

	tests := []struct {
		name        string
		setup       func() sdk.AccAddress
		expBroken   bool
		expContains []string
	}{
		{
			name: "vault holds nothing and has no pending swap-outs",
			setup: func() sdk.AccAddress {
				return s.setupBaseVault(underlyingDenom, shareDenom).GetAddress()
			},
			expBroken: false,
		},
		{
			name: "escrowed shares match the queued request",
			setup: func() sdk.AccAddress {
				s.enqueueDueSwapOut(underlyingDenom, shareDenom, assets, s.ctx.BlockTime().Unix())
				return types.GetVaultAddress(shareDenom)
			},
			expBroken: false,
		},
		{
			name: "swap-out escrows exactly the shares it queues",
			setup: func() sdk.AccAddress {
				vault := s.setupBaseVault(underlyingDenom, shareDenom)
				owner := s.CreateAndFundAccount(assets)
				minted, err := s.k.SwapIn(s.ctx, vault.GetAddress(), owner, assets)
				s.Require().NoError(err, "swap-in should succeed")

				_, err = s.k.SwapOut(s.ctx, vault.GetAddress(), owner, *minted)
				s.Require().NoError(err, "swap-out of %s should succeed", minted)
				return vault.GetAddress()
			},
			expBroken: false,
		},
		{
			name: "swap-in alone leaves shares with the owner, not the vault",
			setup: func() sdk.AccAddress {
				vault := s.setupBaseVault(underlyingDenom, shareDenom)
				owner := s.CreateAndFundAccount(assets)
				_, err := s.k.SwapIn(s.ctx, vault.GetAddress(), owner, assets)
				s.Require().NoError(err, "swap-in should succeed")
				return vault.GetAddress()
			},
			expBroken: false,
		},
		{
			name: "a surplus anyone could have sent is tolerated, not a violation",
			setup: func() sdk.AccAddress {
				vault := s.setupBaseVault(underlyingDenom, shareDenom)
				owner := s.CreateAndFundAccount(assets)
				minted, err := s.k.SwapIn(s.ctx, vault.GetAddress(), owner, assets)
				s.Require().NoError(err, "swap-in should succeed")
				s.Require().NoError(
					s.k.BankKeeper.SendCoins(s.ctx, owner, vault.GetAddress(), sdk.NewCoins(*minted)),
					"sending shares to the vault without queueing a request should succeed",
				)
				return vault.GetAddress()
			},
			expBroken: false,
		},
		{
			name: "queued request whose escrow was drained",
			setup: func() sdk.AccAddress {
				s.enqueueUnrefundableSwapOut(underlyingDenom, shareDenom, assets, s.ctx.BlockTime().Unix())
				return types.GetVaultAddress(shareDenom)
			},
			expBroken:   true,
			expContains: []string{"holds 0vshare in escrow but pending swap-outs account for", "shortfall"},
		},
		{
			name: "queued request escrows a denom that is not the vault's share denom",
			setup: func() sdk.AccAddress {
				vault := s.setupBaseVault(underlyingDenom, shareDenom)
				req := types.PendingSwapOut{
					Owner:        s.adminAddr.String(),
					VaultAddress: vault.GetAddress().String(),
					RedeemDenom:  underlyingDenom,
					Shares:       sdk.NewInt64Coin("wrongshare", 10),
				}
				_, err := s.k.PendingSwapOutQueue.Enqueue(s.ctx, s.ctx.BlockTime().Unix(), &req)
				s.Require().NoError(err, "enqueueing a mis-denominated request should succeed")
				return vault.GetAddress()
			},
			expBroken:   true,
			expContains: []string{"escrows 10wrongshare but the vault's share denom is vshare"},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			vaultAddr := tc.setup()

			msg, broken := keeper.EscrowedSharesInvariant(s.k)(s.ctx)

			s.Require().Equal(tc.expBroken, broken, "invariant broken state mismatch, message was: %s", msg)
			for _, want := range tc.expContains {
				s.Require().Contains(msg, want, "invariant message should report %q", want)
			}
			if tc.expBroken {
				s.Require().Contains(msg, vaultAddr.String(), "invariant message should name the offending vault")
			}
		})
	}
}

func (s *TestSuite) TestAllInvariants() {
	underlyingDenom := "ylds"
	shareDenom := "vshare"

	tests := []struct {
		name       string
		setup      func()
		expBroken  bool
		expSurface func() string
	}{
		{
			name:      "every invariant holds",
			setup:     func() { s.setupBaseVault(underlyingDenom, shareDenom) },
			expBroken: false,
		},
		{
			name: "surfaces a broken total value",
			setup: func() {
				s.setStoredTotalValue(s.setupBaseVault(underlyingDenom, shareDenom), sdkmath.NewInt(99))
			},
			expBroken:  true,
			expSurface: func() string { return keeper.TotalValueInvariantRoute },
		},
		{
			name: "surfaces a broken share supply when total value is fine",
			setup: func() {
				vault := s.setupBaseVault(underlyingDenom, shareDenom)
				s.Require().NoError(
					s.k.MarkerKeeper.MintCoin(s.ctx, vault.GetAddress(), sdk.NewInt64Coin(shareDenom, 1_000)),
					"minting shares outside the vault's accounting should succeed",
				)
			},
			expBroken:  true,
			expSurface: func() string { return keeper.ShareSupplyInvariantRoute },
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			tc.setup()

			msg, broken := keeper.AllInvariants(s.k)(s.ctx)

			s.Require().Equal(tc.expBroken, broken, "AllInvariants broken state mismatch, message was: %s", msg)
			if tc.expSurface != nil {
				s.Require().Contains(msg, tc.expSurface(), "AllInvariants should surface the %s route", tc.expSurface())
			}
		})
	}
}

func (s *TestSuite) TestRegisterInvariants_RegistersEveryRoute() {
	registry := &recordingInvariantRegistry{}

	keeper.RegisterInvariants(registry, s.k)

	expectedRoutes := []string{
		keeper.TotalValueInvariantRoute,
		keeper.ShareSupplyInvariantRoute,
		keeper.EscrowedSharesInvariantRoute,
	}

	s.Require().Len(registry.routes, len(expectedRoutes), "every vault invariant should be registered")
	for i, expected := range expectedRoutes {
		s.Assert().Equal(types.ModuleName, registry.routes[i].module, "route %q should be under the vault module", expected)
		s.Assert().Equal(expected, registry.routes[i].route, "invariant route name mismatch at position %d", i)
		s.Assert().NotNil(registry.routes[i].invariant, "registered invariant %q should not be nil", expected)
	}
}

func (s *TestSuite) TestRegisterInvariants_RoutesReachTheCrisisKeeper() {
	registered := map[string]sdk.Invariant{}
	for _, route := range s.simApp.CrisisKeeper.Routes() {
		if route.ModuleName == types.ModuleName {
			registered[route.Route] = route.Invar
		}
	}

	for _, expected := range []string{
		keeper.TotalValueInvariantRoute,
		keeper.ShareSupplyInvariantRoute,
		keeper.EscrowedSharesInvariantRoute,
	} {
		invariant, ok := registered[expected]
		s.Require().True(ok,
			"route %q must be registered on the crisis keeper; module.Manager.RegisterInvariants is a no-op, so the app has to register directly", expected)

		_, broken := invariant(s.ctx)
		s.Assert().False(broken, "route %q should hold on a freshly initialized chain", expected)
	}
}

// setStoredTotalValue overwrites a vault's materialized total value without touching balances,
// putting state in the drifted condition the invariant exists to catch.
func (s *TestSuite) setStoredTotalValue(vault *types.VaultAccount, total sdkmath.Int) {
	s.Require().NoError(
		s.k.TotalValues.Set(s.ctx, vault.GetAddress(), total),
		"overwriting the stored total for vault %s with %s should succeed", vault.GetAddress(), total,
	)
}

// recordingInvariantRegistry captures the routes registered against it so tests can assert what
// the module wires up.
type recordingInvariantRegistry struct {
	routes []registeredInvariant
}

// registeredInvariant is one route captured by recordingInvariantRegistry.
type registeredInvariant struct {
	module    string
	route     string
	invariant sdk.Invariant
}

// RegisterRoute implements sdk.InvariantRegistry by recording the route.
func (r *recordingInvariantRegistry) RegisterRoute(module, route string, invariant sdk.Invariant) {
	r.routes = append(r.routes, registeredInvariant{module: module, route: route, invariant: invariant})
}

var _ sdk.InvariantRegistry = (*recordingInvariantRegistry)(nil)
