package keeper_test

import (
	"fmt"
	"math/big"
	"strings"
	"time"

	"cosmossdk.io/collections"
	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/google/uuid"
	metadatatypes "github.com/provenance-io/provenance/x/metadata/types"

	"github.com/provlabs/vault/types"
)

// TestKeeper_GetVaultNAV_NotFound verifies GetVaultNAV returns collections.ErrNotFound
// when no entry exists for the given vault address and denom.
func (s *TestSuite) TestKeeper_GetVaultNAV_NotFound() {
	underlying := "under"
	share := "vaultshares"
	vaultAddr := types.GetVaultAddress(share)
	s.setupBaseVault(underlying, share)

	tests := []struct {
		name      string
		vaultAddr sdk.AccAddress
		denom     string
	}{
		{
			name:      "denom has no entry for the vault",
			vaultAddr: vaultAddr,
			denom:     "rwa",
		},
		{
			name:      "completely unknown vault address has no entry",
			vaultAddr: types.GetVaultAddress("unknownshare"),
			denom:     "rwa",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			_, err := s.k.GetVaultNAV(s.ctx, tc.vaultAddr, tc.denom)
			s.Require().Error(err, "GetVaultNAV should return an error when no entry exists for vault %s denom %s", tc.vaultAddr, tc.denom)
			s.Assert().ErrorIs(err, collections.ErrNotFound, "GetVaultNAV should return collections.ErrNotFound for vault %s denom %s", tc.vaultAddr, tc.denom)
		})
	}
}

// TestKeeper_SetVaultNAV_OverwriteReStamps verifies that a second SetVaultNAV call
// updates the block height and time on the stored entry rather than preserving the
// values from the first write.
func (s *TestSuite) TestKeeper_SetVaultNAV_OverwriteReStamps() {
	type wantFields struct {
		price  sdk.Coin
		volume sdkmath.Int
		source string
	}
	cases := []struct {
		name         string
		underlying   string
		share        string
		navDenom     string
		firstHeight  int64
		firstNav     types.VaultNAV
		secondHeight int64
		secondNav    types.VaultNAV
		want         wantFields
	}{
		{
			name:        "overwrite re-stamps height, time, price, volume, and source",
			underlying:  "under",
			share:       "vaultshares",
			navDenom:    "rwa",
			firstHeight: 10,
			firstNav: types.VaultNAV{
				Denom:  "rwa",
				Price:  sdk.NewInt64Coin("under", 100),
				Volume: sdkmath.NewInt(5),
				Source: "oracle-first",
			},
			secondHeight: 20,
			secondNav: types.VaultNAV{
				Denom:  "rwa",
				Price:  sdk.NewInt64Coin("under", 200),
				Volume: sdkmath.NewInt(10),
				Source: "oracle-second",
			},
			want: wantFields{
				price:  sdk.NewInt64Coin("under", 200),
				volume: sdkmath.NewInt(10),
				source: "oracle-second",
			},
		},
		{
			name:        "overwrite with same source still re-stamps block metadata",
			underlying:  "usdc",
			share:       "usdcshares",
			navDenom:    "bond",
			firstHeight: 1,
			firstNav: types.VaultNAV{
				Denom:  "bond",
				Price:  sdk.NewInt64Coin("usdc", 1_000),
				Volume: sdkmath.NewInt(1),
				Source: "static-oracle",
			},
			secondHeight: 2,
			secondNav: types.VaultNAV{
				Denom:  "bond",
				Price:  sdk.NewInt64Coin("usdc", 1_050),
				Volume: sdkmath.NewInt(2),
				Source: "static-oracle",
			},
			want: wantFields{
				price:  sdk.NewInt64Coin("usdc", 1_050),
				volume: sdkmath.NewInt(2),
				source: "static-oracle",
			},
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			vault := s.setupBaseVault(tc.underlying, tc.share)
			vaultAddr := types.GetVaultAddress(tc.share)
			s.requireSimpleMarker(tc.navDenom)

			baseCtx := s.ctx
			firstTime := baseCtx.BlockTime().UTC()
			ctx1 := baseCtx.WithBlockHeight(tc.firstHeight)

			s.Require().NoError(s.k.SetVaultNAV(ctx1, vault, tc.firstNav, s.adminAddr.String()), "first SetVaultNAV should succeed")

			storedFirst, err := s.k.GetVaultNAV(ctx1, vaultAddr, tc.navDenom)
			s.Require().NoError(err, "GetVaultNAV should return the first entry")
			s.Assert().Equal(tc.firstHeight, storedFirst.UpdatedBlockHeight, "first write should stamp the block height")
			s.Assert().Equal(firstTime, storedFirst.UpdatedTime, "first write should stamp the block time")

			secondTime := firstTime.Add(time.Second)
			ctx2 := ctx1.WithBlockHeight(tc.secondHeight).WithBlockTime(secondTime)

			s.Require().NoError(s.k.SetVaultNAV(ctx2, vault, tc.secondNav, s.adminAddr.String()), "second SetVaultNAV should succeed")

			storedSecond, err := s.k.GetVaultNAV(ctx2, vaultAddr, tc.navDenom)
			s.Require().NoError(err, "GetVaultNAV should return the overwritten entry")
			s.Assert().Equal(tc.secondHeight, storedSecond.UpdatedBlockHeight, "overwrite should re-stamp the block height")
			s.Assert().Equal(secondTime.UTC(), storedSecond.UpdatedTime, "overwrite should re-stamp the block time")
			s.Assert().Equal(tc.want.price, storedSecond.Price, "overwrite should update the price")
			s.Assert().Equal(tc.want.volume, storedSecond.Volume, "overwrite should update the volume")
			s.Assert().Equal(tc.want.source, storedSecond.Source, "overwrite should update the source")
		})
	}
}

// TestKeeper_SetVaultNAV_RejectsInvalidInput verifies SetVaultNAV rejects every
// invalid input before persisting an entry: the vault share denom, an IBC voucher
// denom, an invalid price coin, a negative price amount, a price denom that is not
// the vault underlying asset, a nil or non-positive volume, a denom that is not a
// registered marker, and an nft/ denom that does not name an existing scope.
func (s *TestSuite) TestKeeper_SetVaultNAV_RejectsInvalidInput() {
	underlying := "under"
	share := "vaultshares"
	registeredDenom := "rwa"
	vault := s.setupBaseVault(underlying, share)
	vaultAddr := types.GetVaultAddress(share)
	s.requireSimpleMarker(registeredDenom)

	tests := []struct {
		name              string
		nav               types.VaultNAV
		expectedErrSubstr string
	}{
		{
			name: "rejects the vault share denom",
			nav: types.VaultNAV{
				Denom:  share,
				Price:  sdk.NewInt64Coin(underlying, 100),
				Volume: sdkmath.NewInt(1),
			},
			expectedErrSubstr: "cannot set NAV for vault share denom",
		},
		{
			name: "rejects an IBC voucher denom",
			nav: types.VaultNAV{
				Denom:  "ibc/27394FB092D2ECCD56123C74F36E4C1F926001CEADA9CA97EA622B25F41E5EB2",
				Price:  sdk.NewInt64Coin(underlying, 100),
				Volume: sdkmath.NewInt(1),
			},
			expectedErrSubstr: "cannot be used by a vault",
		},
		{
			name: "rejects an invalid price coin",
			nav: types.VaultNAV{
				Denom:  registeredDenom,
				Price:  sdk.Coin{Denom: underlying, Amount: sdkmath.NewInt(-1)},
				Volume: sdkmath.NewInt(1),
			},
			expectedErrSubstr: "invalid NAV price",
		},
		{
			name: "rejects a negative price amount",
			nav: types.VaultNAV{
				Denom:  registeredDenom,
				Price:  sdk.Coin{Denom: underlying, Amount: sdkmath.NewInt(-1)},
				Volume: sdkmath.NewInt(1),
			},
			expectedErrSubstr: "invalid NAV price",
		},
		{
			name: "rejects a price denom that is not the vault underlying asset",
			nav: types.VaultNAV{
				Denom:  registeredDenom,
				Price:  sdk.NewInt64Coin("notunderlying", 100),
				Volume: sdkmath.NewInt(1),
			},
			expectedErrSubstr: "must be the vault underlying asset",
		},
		{
			name: "rejects a nil volume",
			nav: types.VaultNAV{
				Denom:  registeredDenom,
				Price:  sdk.NewInt64Coin(underlying, 100),
				Volume: sdkmath.Int{},
			},
			expectedErrSubstr: "NAV volume must be positive",
		},
		{
			name: "rejects a zero volume",
			nav: types.VaultNAV{
				Denom:  registeredDenom,
				Price:  sdk.NewInt64Coin(underlying, 100),
				Volume: sdkmath.ZeroInt(),
			},
			expectedErrSubstr: "NAV volume must be positive",
		},
		{
			name: "rejects a negative volume",
			nav: types.VaultNAV{
				Denom:  registeredDenom,
				Price:  sdk.NewInt64Coin(underlying, 100),
				Volume: sdkmath.NewInt(-1),
			},
			expectedErrSubstr: "NAV volume must be positive",
		},
		{
			name: "rejects a source that exceeds the max length",
			nav: types.VaultNAV{
				Denom:  registeredDenom,
				Price:  sdk.NewInt64Coin(underlying, 100),
				Volume: sdkmath.NewInt(1),
				Source: strings.Repeat("x", types.MaxNAVSourceLength+1),
			},
			expectedErrSubstr: "NAV source too long",
		},
		{
			name: "rejects self-referential price denom matching nav denom",
			nav: types.VaultNAV{
				Denom:  registeredDenom,
				Price:  sdk.NewInt64Coin(registeredDenom, 100),
				Volume: sdkmath.NewInt(1),
			},
			expectedErrSubstr: "price denom must differ",
		},
		{
			name: "rejects a denom that is not a registered marker",
			nav: types.VaultNAV{
				Denom:  "ghostdenom",
				Price:  sdk.NewInt64Coin(underlying, 100),
				Volume: sdkmath.NewInt(1),
			},
			expectedErrSubstr: "is not a registered marker",
		},
		{
			name: "rejects a malformed nft/ denom via the marker check",
			nav: types.VaultNAV{
				Denom:  "nft/notabech32",
				Price:  sdk.NewInt64Coin(underlying, 100),
				Volume: sdkmath.NewInt(1),
			},
			expectedErrSubstr: "is not a registered marker",
		},
		{
			name: "rejects an nft/ denom whose scope does not exist",
			nav: types.VaultNAV{
				Denom:  metadatatypes.ScopeMetadataAddress(uuid.MustParse("00000000-0000-4000-8000-00000000000f")).Denom(),
				Price:  sdk.NewInt64Coin(underlying, 100),
				Volume: sdkmath.NewInt(1),
			},
			expectedErrSubstr: "is not an existing metadata scope",
		},
		{
			name: "rejects an nft/ denom naming a session rather than a scope",
			nav: types.VaultNAV{
				Denom: metadatatypes.SessionMetadataAddress(
					uuid.MustParse("00000000-0000-4000-8000-00000000000e"),
					uuid.MustParse("00000000-0000-4000-8000-00000000000d"),
				).Denom(),
				Price:  sdk.NewInt64Coin(underlying, 100),
				Volume: sdkmath.NewInt(1),
			},
			expectedErrSubstr: "is not an existing metadata scope",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			err := s.k.SetVaultNAV(s.ctx, vault, tc.nav, s.adminAddr.String())
			s.Require().Error(err, "SetVaultNAV should reject input for case %q", tc.name)
			s.Assert().Contains(err.Error(), tc.expectedErrSubstr, "SetVaultNAV error for case %q should mention %q", tc.name, tc.expectedErrSubstr)

			_, getErr := s.k.GetVaultNAV(s.ctx, vaultAddr, tc.nav.Denom)
			s.Assert().ErrorIs(getErr, collections.ErrNotFound, "SetVaultNAV must not persist an entry for rejected input %q", tc.name)
		})
	}
}

// TestKeeper_SetVaultNAV_AllowsZeroPriceForHeldDenom verifies the authority can
// write a held, non-accepted asset down to a zero NAV (e.g. a seized or retired
// asset) and that the zero price is persisted.
func (s *TestSuite) TestKeeper_SetVaultNAV_AllowsZeroPriceForHeldDenom() {
	underlying := "under"
	share := "vaultshares"
	heldDenom := "rwa"
	vault := s.setupBaseVault(underlying, share)
	vaultAddr := types.GetVaultAddress(share)
	s.requireSimpleMarker(heldDenom)

	s.Require().False(vault.IsAcceptedDenom(heldDenom), "%q must be a held, non-accepted denom for this test", heldDenom)

	nav := types.VaultNAV{
		Denom:  heldDenom,
		Price:  sdk.NewInt64Coin(underlying, 0),
		Volume: sdkmath.NewInt(1),
	}
	s.Require().NoError(s.k.SetVaultNAV(s.ctx, vault, nav, s.adminAddr.String()), "SetVaultNAV should accept a zero price for a held denom")

	stored, err := s.k.GetVaultNAV(s.ctx, vaultAddr, heldDenom)
	s.Require().NoError(err, "zero-price NAV for held denom %s should be written", heldDenom)
	s.Assert().True(stored.Price.Amount.IsZero(), "stored NAV price for held denom should be zero, got %s", stored.Price.Amount)
}

func (s *TestSuite) TestKeeper_SetVaultNAV_MetadataDenomChecksScopeInsteadOfMarker() {
	underlying := "under"
	share := "vaultshares"
	vault := s.setupBaseVault(underlying, share)
	vaultAddr := types.GetVaultAddress(share)

	navDenom := s.requireScope("00000000-0000-4000-8000-000000000003")
	nav := types.VaultNAV{
		Denom:  navDenom,
		Price:  sdk.NewInt64Coin(underlying, 100),
		Volume: sdkmath.NewInt(1),
	}

	s.Require().NoError(s.k.SetVaultNAV(s.ctx, vault, nav, s.adminAddr.String()), "SetVaultNAV should accept an nft/ denom without a registered marker")

	stored, err := s.k.GetVaultNAV(s.ctx, vaultAddr, navDenom)
	s.Require().NoError(err, "internal NAV for nft/ denom %s should be written", navDenom)
	s.Assert().Equal(nav.Price, stored.Price, "stored NAV price for nft/ denom")
	s.Assert().Equal(nav.Volume, stored.Volume, "stored NAV volume for nft/ denom")

	_, markerErr := s.simApp.MarkerKeeper.GetMarkerByDenom(s.ctx, navDenom)
	s.Assert().Error(markerErr, "nft/ denom %s must not be a registered marker", navDenom)
}

func (s *TestSuite) TestKeeper_RemoveVaultNAV() {
	cases := []struct {
		name                string
		underlying          string
		share               string
		navDenom            string
		seedNav             *types.VaultNAV
		signer              string
		expectedErrContains string
		expectedLastPrice   string
		expectedLastVolume  string
		expectedSigner      string
	}{
		{
			name:       "existing entry is deleted and EventNAVRemoved carries the last price",
			underlying: "under",
			share:      "vaultshares",
			navDenom:   "rwa",
			seedNav: &types.VaultNAV{
				Denom:  "rwa",
				Price:  sdk.NewInt64Coin("under", 1_000_000),
				Volume: sdkmath.NewInt(500_000),
				Source: "settlement",
			},
			expectedLastPrice:  "1000000under",
			expectedLastVolume: "500000",
		},
		{
			name:       "removal by the NAV authority records the signer on the event",
			underlying: "under",
			share:      "authorityshares",
			navDenom:   "rwa",
			seedNav: &types.VaultNAV{
				Denom:  "rwa",
				Price:  sdk.NewInt64Coin("under", 250),
				Volume: sdkmath.NewInt(5),
				Source: "oracle",
			},
			signer:             s.adminAddr.String(),
			expectedLastPrice:  "250under",
			expectedLastVolume: "5",
			expectedSigner:     s.adminAddr.String(),
		},
		{
			name:                "denom with no entry returns an error and emits nothing",
			underlying:          "usdc",
			share:               "usdcshares",
			navDenom:            "bond",
			seedNav:             nil,
			expectedErrContains: "failed to get internal NAV for denom \"bond\"",
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			vault := s.setupBaseVault(tc.underlying, tc.share)
			vaultAddr := types.GetVaultAddress(tc.share)

			if tc.seedNav != nil {
				s.requireSimpleMarker(tc.navDenom)
				s.Require().NoError(
					s.k.SetVaultNAV(s.ctx, vault, *tc.seedNav, s.adminAddr.String()),
					"seeding NAV entry for denom %s should succeed", tc.navDenom,
				)
			}

			s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
			err := s.k.RemoveVaultNAV(s.ctx, vault, tc.navDenom, tc.signer)

			var removedEvents []sdk.Event
			for _, ev := range s.ctx.EventManager().Events() {
				if ev.Type == "provlabs.vault.v1.EventNAVRemoved" {
					removedEvents = append(removedEvents, ev)
				}
			}

			if tc.expectedErrContains != "" {
				s.Require().ErrorContains(err, tc.expectedErrContains, "RemoveVaultNAV should fail for denom %s with no entry", tc.navDenom)
				s.Assert().ErrorIs(err, collections.ErrNotFound, "RemoveVaultNAV should wrap collections.ErrNotFound for denom %s", tc.navDenom)
				s.Assert().Empty(removedEvents, "failed RemoveVaultNAV should not emit EventNAVRemoved for denom %s", tc.navDenom)
				return
			}

			s.Require().NoError(err, "RemoveVaultNAV should succeed for seeded denom %s", tc.navDenom)

			_, err = s.k.GetVaultNAV(s.ctx, vaultAddr, tc.navDenom)
			s.Assert().ErrorIs(err, collections.ErrNotFound, "NAV entry for denom %s should be deleted after RemoveVaultNAV", tc.navDenom)

			s.Require().Len(removedEvents, 1, "RemoveVaultNAV should emit exactly one EventNAVRemoved for denom %s", tc.navDenom)
			attrs := map[string]string{}
			for _, a := range removedEvents[0].Attributes {
				attrs[a.Key] = a.Value
			}
			s.Assert().Equal(`"`+vaultAddr.String()+`"`, attrs["vault_address"], "event vault_address attribute should record the vault")
			s.Assert().Equal(`"`+tc.navDenom+`"`, attrs["denom"], "event denom attribute should record the removed denom")
			s.Assert().Equal(`"`+tc.expectedLastPrice+`"`, attrs["last_price"], "event last_price attribute should record the last stored price")
			s.Assert().Equal(`"`+tc.expectedLastVolume+`"`, attrs["last_volume"], "event last_volume attribute should record the last stored volume")
			s.Assert().Equal(`"`+tc.expectedSigner+`"`, attrs["signer"], "event signer attribute should record the NAV authority, and stay empty for a protocol-initiated removal")
		})
	}
}

func (s *TestSuite) TestKeeper_RequirePausedHeldReprice() {
	underlying := "under"
	share := "vaultshares"
	heldDenom := "rwa"

	const (
		seedPrice  = 2
		seedVolume = 1
		heldAmount = 1_000
	)

	tests := []struct {
		name              string
		heldAmount        int64
		unpriced          bool
		corruptNav        bool
		paused            bool
		newPrice          int64
		newVolume         int64
		expectedErrSubstr string
	}{
		{
			name:       "paused vault may reprice a held asset",
			heldAmount: heldAmount,
			paused:     true,
			newPrice:   4,
			newVolume:  1,
		},
		{
			name:       "paused vault may price an asset it holds for the first time",
			heldAmount: heldAmount,
			unpriced:   true,
			paused:     true,
			newPrice:   4,
			newVolume:  1,
		},
		{
			name:       "live vault may reprice a denom it does not hold",
			heldAmount: 0,
			newPrice:   4,
			newVolume:  1,
		},
		{
			name:       "live vault may restate a held asset at an identical price and volume",
			heldAmount: heldAmount,
			newPrice:   seedPrice,
			newVolume:  seedVolume,
		},
		{
			name:       "live vault may restate a held asset at the same unit price scaled up",
			heldAmount: heldAmount,
			newPrice:   seedPrice * 4,
			newVolume:  seedVolume * 4,
		},
		{
			name:              "live vault may not mark a held asset up",
			heldAmount:        heldAmount,
			newPrice:          seedPrice + 1,
			newVolume:         seedVolume,
			expectedErrSubstr: "pause the vault to reprice a held asset",
		},
		{
			name:              "live vault may not mark a held asset down",
			heldAmount:        heldAmount,
			newPrice:          seedPrice - 1,
			newVolume:         seedVolume,
			expectedErrSubstr: "pause the vault to reprice a held asset",
		},
		{
			name:              "live vault may not write a held asset down to zero",
			heldAmount:        heldAmount,
			newPrice:          0,
			newVolume:         seedVolume,
			expectedErrSubstr: "pause the vault to reprice a held asset",
		},
		{
			name:              "live vault may not change the unit price by changing only the volume",
			heldAmount:        heldAmount,
			newPrice:          seedPrice,
			newVolume:         seedVolume + 1,
			expectedErrSubstr: "pause the vault to reprice a held asset",
		},
		{
			name:              "live vault may not price an asset it already holds for the first time",
			heldAmount:        heldAmount,
			unpriced:          true,
			newPrice:          seedPrice,
			newVolume:         seedVolume,
			expectedErrSubstr: "pause the vault to reprice a held asset",
		},
		{
			name:              "an unreadable NAV entry for a held denom surfaces the lookup failure",
			heldAmount:        heldAmount,
			corruptNav:        true,
			newPrice:          4,
			newVolume:         1,
			expectedErrSubstr: "failed to get internal NAV for denom",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			origCtx := s.ctx
			defer func() { s.ctx = origCtx }()
			s.ctx, _ = s.ctx.CacheContext()

			vaultAddr := types.GetVaultAddress(share)
			var vault *types.VaultAccount
			if tc.unpriced || tc.corruptNav {
				vault = s.setupBaseVault(underlying, share)
				s.requireSimpleMarker(heldDenom)
				s.fundPrincipal(vault, sdk.NewInt64Coin(heldDenom, tc.heldAmount))
			} else {
				vault = s.setupHeldNAVVault(underlying, share, heldDenom, sdk.NewInt64Coin(underlying, seedPrice), seedVolume, tc.heldAmount)
			}
			if tc.corruptNav {
				s.Require().NoError(s.k.TestAccessor_corruptVaultNAV(s.T(), s.ctx, vaultAddr, heldDenom),
					"failed to corrupt the NAV entry for %s", heldDenom)
			}
			if tc.paused {
				vault = s.pauseVault(vaultAddr)
			}

			nav := types.NewVaultNAV(heldDenom, sdk.NewInt64Coin(underlying, tc.newPrice), sdkmath.NewInt(tc.newVolume), "oracle")
			err := s.k.TestAccessor_requirePausedHeldReprice(s.T(), s.ctx, vault, nav)

			if tc.expectedErrSubstr == "" {
				s.Require().NoError(err, "requirePausedHeldReprice should allow pricing %s at %d per %d units",
					heldDenom, tc.newPrice, tc.newVolume)
				return
			}
			s.Require().ErrorContains(err, tc.expectedErrSubstr,
				"requirePausedHeldReprice should reject pricing %s at %d per %d units while the vault holds %d",
				heldDenom, tc.newPrice, tc.newVolume, tc.heldAmount)
		})
	}
}

func (s *TestSuite) TestKeeper_CheckSettlementNAVGuardrail() {
	underlying := "under"
	share := "vaultshares"
	asset := "rwa"
	hugeAmt := sdkmath.NewIntFromBigInt(new(big.Int).Lsh(big.NewInt(1), 130))

	tests := []struct {
		name                string
		seedNav             *types.VaultNAV
		corruptNav          bool
		assetCoin           sdk.Coin
		paymentCoin         sdk.Coin
		expectedErrContains string
	}{
		{
			name:                "no NAV entry for the asset denom rejects the settlement",
			assetCoin:           sdk.NewInt64Coin(asset, 10),
			paymentCoin:         sdk.NewInt64Coin(underlying, 5),
			expectedErrContains: "has no internal NAV entry",
		},
		{
			name:                "undecodable NAV entry propagates a non-NotFound lookup error",
			corruptNav:          true,
			assetCoin:           sdk.NewInt64Coin(asset, 10),
			paymentCoin:         sdk.NewInt64Coin(underlying, 5),
			expectedErrContains: "failed to get internal NAV for denom \"rwa\"",
		},
		{
			name:                "NAV priced in a different denom than the payment coin is rejected",
			seedNav:             &types.VaultNAV{Denom: asset, Price: sdk.NewInt64Coin(underlying, 5), Volume: sdkmath.NewInt(10)},
			assetCoin:           sdk.NewInt64Coin(asset, 10),
			paymentCoin:         sdk.NewInt64Coin("pay", 5),
			expectedErrContains: "is priced in",
		},
		{
			name:                "asset amount times NAV price overflowing 256 bits returns an error",
			seedNav:             &types.VaultNAV{Denom: asset, Price: sdk.NewCoin(underlying, hugeAmt), Volume: sdkmath.NewInt(1)},
			assetCoin:           sdk.NewCoin(asset, hugeAmt),
			paymentCoin:         sdk.NewInt64Coin(underlying, 5),
			expectedErrContains: "failed to multiply settlement asset amount",
		},
		{
			name:                "payment amount times NAV volume overflowing 256 bits returns an error",
			seedNav:             &types.VaultNAV{Denom: asset, Price: sdk.NewInt64Coin(underlying, 1), Volume: hugeAmt},
			assetCoin:           sdk.NewInt64Coin(asset, 1),
			paymentCoin:         sdk.NewCoin(underlying, hugeAmt),
			expectedErrContains: "failed to multiply settlement payment amount",
		},
		{
			name:                "settlement off the NAV price is rejected",
			seedNav:             &types.VaultNAV{Denom: asset, Price: sdk.NewInt64Coin(underlying, 6), Volume: sdkmath.NewInt(10)},
			assetCoin:           sdk.NewInt64Coin(asset, 10),
			paymentCoin:         sdk.NewInt64Coin(underlying, 5),
			expectedErrContains: "does not match internal NAV",
		},
		{
			name:        "settlement at the exact NAV price passes the guardrail even though the vault holds none of the asset yet",
			seedNav:     &types.VaultNAV{Denom: asset, Price: sdk.NewInt64Coin(underlying, 5), Volume: sdkmath.NewInt(10)},
			assetCoin:   sdk.NewInt64Coin(asset, 10),
			paymentCoin: sdk.NewInt64Coin(underlying, 5),
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			origCtx := s.ctx
			defer func() { s.ctx = origCtx }()
			s.ctx, _ = s.ctx.CacheContext()

			vault := s.setupBaseVault(underlying, share)

			if tc.seedNav != nil {
				s.Require().NoError(
					s.k.NAVs.Set(s.ctx, collections.Join(vault.GetAddress(), tc.seedNav.Denom), *tc.seedNav),
					"failed to seed NAV entry for denom %s", tc.seedNav.Denom,
				)
			}
			if tc.corruptNav {
				s.Require().NoError(
					s.k.TestAccessor_corruptVaultNAV(s.T(), s.ctx, vault.GetAddress(), tc.assetCoin.Denom),
					"failed to corrupt NAV entry for denom %s", tc.assetCoin.Denom,
				)
			}

			err := s.k.TestAccessor_checkSettlementNAVGuardrail(s.T(), s.ctx, vault, tc.assetCoin, tc.paymentCoin)

			if tc.expectedErrContains == "" {
				s.Require().NoError(err, "guardrail should pass for asset %s payment %s", tc.assetCoin, tc.paymentCoin)
				return
			}

			s.Require().ErrorContains(err, tc.expectedErrContains, "guardrail error mismatch for asset %s payment %s", tc.assetCoin, tc.paymentCoin)
		})
	}
}

func (s *TestSuite) TestKeeper_SetNAVAuthority_PersistsAndEmits() {
	cases := []struct {
		name                string
		underlying          string
		share               string
		seedAuthority       string
		newAuthorityIsEmpty bool
	}{
		{
			name:                "rotate from default (empty) to explicit address",
			underlying:          "undera",
			share:               "vaultsharesa",
			seedAuthority:       "",
			newAuthorityIsEmpty: false,
		},
		{
			name:                "rotate from one explicit address to another",
			underlying:          "underb",
			share:               "vaultsharesb",
			seedAuthority:       "preexisting",
			newAuthorityIsEmpty: false,
		},
		{
			name:                "reset explicit authority back to empty",
			underlying:          "underc",
			share:               "vaultsharesc",
			seedAuthority:       "preexisting",
			newAuthorityIsEmpty: true,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			vault := s.setupBaseVault(tc.underlying, tc.share)
			vaultAddr := types.GetVaultAddress(tc.share)
			oracle := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1_000))

			seedAuthority := ""
			if tc.seedAuthority != "" {
				seedAuthority = s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1_000)).String()
				vault.NavAuthority = seedAuthority
				s.Require().NoError(s.k.SetVaultAccount(s.ctx, vault), "seeding initial NavAuthority should succeed")
			}

			newAuthority := oracle.String()
			if tc.newAuthorityIsEmpty {
				newAuthority = ""
			}

			s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
			err := s.k.SetNAVAuthority(s.ctx, vault, newAuthority, s.adminAddr.String())
			s.Require().NoError(err, "SetNAVAuthority should succeed")

			stored, err := s.k.GetVault(s.ctx, vaultAddr)
			s.Require().NoError(err, "GetVault after SetNAVAuthority should succeed")
			s.Assert().Equal(newAuthority, stored.NavAuthority, "NavAuthority should be persisted as %q", newAuthority)

			var matches []sdk.Event
			for _, ev := range s.ctx.EventManager().Events() {
				if ev.Type == "provlabs.vault.v1.EventNAVAuthorityUpdated" {
					matches = append(matches, ev)
				}
			}
			s.Require().Len(matches, 1, "SetNAVAuthority should emit exactly one EventNAVAuthorityUpdated")

			attrs := map[string]string{}
			for _, a := range matches[0].Attributes {
				attrs[a.Key] = a.Value
			}
			s.Assert().Equal(`"`+s.adminAddr.String()+`"`, attrs["admin"], "event admin attribute should record the signer")
			s.Assert().Equal(`"`+newAuthority+`"`, attrs["new_authority"], "event new_authority attribute should record the new authority")
			s.Assert().Equal(`"`+vaultAddr.String()+`"`, attrs["vault_address"], "event vault_address attribute should record the vault")
		})
	}
}

// TestKeeper_SetNAVAuthority_NoOpWhenUnchanged verifies that calling
// SetNAVAuthority with the value already stored on the vault is a no-op: the
// call succeeds, the vault account is not rewritten with an event, and no
// EventNAVAuthorityUpdated event is emitted. This guards future keeper callers
// (sims, direct invocations) from emitting spurious authority-rotation events.
func (s *TestSuite) TestKeeper_SetNAVAuthority_NoOpWhenUnchanged() {
	underlying := "under"
	share := "vaultshares"
	vault := s.setupBaseVault(underlying, share)
	vaultAddr := types.GetVaultAddress(share)
	oracle := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1_000))

	s.Require().NoError(
		s.k.SetNAVAuthority(s.ctx, vault, oracle.String(), s.adminAddr.String()),
		"initial rotation should succeed",
	)

	before, err := s.k.GetVault(s.ctx, vaultAddr)
	s.Require().NoError(err, "GetVault after initial rotation should succeed")

	s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	s.Require().NoError(
		s.k.SetNAVAuthority(s.ctx, vault, oracle.String(), s.adminAddr.String()),
		"re-setting NAV authority to its current value should be a no-op",
	)

	for _, ev := range s.ctx.EventManager().Events() {
		s.Assert().NotEqualf(
			"provlabs.vault.v1.EventNAVAuthorityUpdated", ev.Type,
			"no-op SetNAVAuthority should not emit EventNAVAuthorityUpdated",
		)
	}

	after, err := s.k.GetVault(s.ctx, vaultAddr)
	s.Require().NoError(err, "GetVault after no-op SetNAVAuthority should succeed")
	s.Assert().Equal(before.NavAuthority, after.NavAuthority, "no-op should leave NavAuthority untouched")
}

func (s *TestSuite) TestKeeper_SetVaultNAV_EnforcesEntryCap() {
	underlying := "capunder"
	share := "capshares"

	tests := []struct {
		name          string
		recordedCount uint64
		denom         string
		prePrice      bool
		expErr        string
		expCount      uint64
	}{
		{
			name:          "well under the cap, new denom is priced",
			recordedCount: 0,
			denom:         "capasseta",
			expCount:      1,
		},
		{
			name:          "one slot left, new denom is priced",
			recordedCount: types.MaxVaultNAVEntries - 1,
			denom:         "capassetb",
			expCount:      types.MaxVaultNAVEntries,
		},
		{
			name:          "at the cap, a new denom is rejected",
			recordedCount: types.MaxVaultNAVEntries,
			denom:         "capassetc",
			expErr:        fmt.Sprintf("already prices %d denoms (max %d)", types.MaxVaultNAVEntries, types.MaxVaultNAVEntries),
			expCount:      types.MaxVaultNAVEntries,
		},
		{
			name:          "over the cap, a new denom is rejected",
			recordedCount: types.MaxVaultNAVEntries + 5,
			denom:         "capassetd",
			expErr:        fmt.Sprintf("already prices %d denoms (max %d)", types.MaxVaultNAVEntries+5, types.MaxVaultNAVEntries),
			expCount:      types.MaxVaultNAVEntries + 5,
		},
		{
			name:          "at the cap, repricing an already-priced denom is allowed",
			recordedCount: types.MaxVaultNAVEntries,
			denom:         "capassete",
			prePrice:      true,
			expCount:      types.MaxVaultNAVEntries,
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			vault := s.setupBaseVault(underlying, share)
			s.requireSimpleMarker(tc.denom)

			if tc.prePrice {
				s.setVaultNAV(vault, tc.denom, sdk.NewInt64Coin(underlying, 1), 1)
			}
			s.Require().NoError(s.k.NAVCounts.Set(s.ctx, vault.GetAddress(), tc.recordedCount),
				"failed to seed a recorded NAV entry count of %d", tc.recordedCount)

			nav := types.NewVaultNAV(tc.denom, sdk.NewInt64Coin(underlying, 2), sdkmath.NewInt(1), "captest")
			err := s.k.SetVaultNAV(s.ctx, vault, nav, s.adminAddr.String())

			if tc.expErr != "" {
				s.Require().ErrorContains(err, tc.expErr,
					"SetVaultNAV should reject %q once the vault records %d priced denoms", tc.denom, tc.recordedCount)
				_, getErr := s.k.GetVaultNAV(s.ctx, vault.GetAddress(), tc.denom)
				s.Require().ErrorIs(getErr, collections.ErrNotFound,
					"a rejected NAV must not be written for denom %q", tc.denom)
			} else {
				s.Require().NoError(err, "SetVaultNAV should price %q when the vault records %d priced denoms", tc.denom, tc.recordedCount)
			}

			count, err := s.k.NAVEntryCount(s.ctx, vault.GetAddress())
			s.Require().NoError(err, "failed to read the NAV entry count for vault %s", vault.Address)
			s.Require().Equal(tc.expCount, count,
				"NAV entry count mismatch for vault %s after setting %q", vault.Address, tc.denom)
		})
	}
}

func (s *TestSuite) TestKeeper_NAVEntryCount_TracksTheTable() {
	underlying := "trackunder"
	share := "trackshares"
	denoms := []string{"trackasseta", "trackassetb", "trackassetc"}

	vault := s.setupBaseVault(underlying, share)
	for _, denom := range denoms {
		s.requireSimpleMarker(denom)
	}

	assertCount := func(expected uint64, stage string) {
		count, err := s.k.NAVEntryCount(s.ctx, vault.GetAddress())
		s.Require().NoError(err, "failed to read the NAV entry count %s", stage)
		s.Require().Equal(expected, count, "NAV entry count mismatch %s", stage)
	}

	assertCount(0, "before any denom is priced")

	for i, denom := range denoms {
		s.setVaultNAV(vault, denom, sdk.NewInt64Coin(underlying, 1), 1)
		assertCount(uint64(i+1), "after pricing "+denom)
	}

	s.setVaultNAV(vault, denoms[0], sdk.NewInt64Coin(underlying, 7), 1)
	assertCount(uint64(len(denoms)), "after repricing an already-priced denom")

	s.Require().NoError(s.k.RemoveVaultNAV(s.ctx, vault, denoms[0], s.adminAddr.String()),
		"failed to remove the NAV for %s", denoms[0])
	assertCount(uint64(len(denoms))-1, "after removing one denom")

	s.Require().NoError(s.k.NAVCounts.Set(s.ctx, vault.GetAddress(), 0),
		"failed to force the recorded count to zero")
	s.Require().NoError(s.k.RemoveVaultNAV(s.ctx, vault, denoms[1], s.adminAddr.String()),
		"a NAV removal must still succeed when the recorded count already reads zero")
	assertCount(0, "after removing a denom while the recorded count read zero")
}
