package keeper_test

import (
	"math/big"
	"strings"
	"time"

	"cosmossdk.io/collections"
	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/google/uuid"
	markertypes "github.com/provenance-io/provenance/x/marker/types"
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
// invalid input before persisting an entry: the vault share denom, an invalid
// price coin, a non-positive price amount, a price denom outside the vault's
// accepted denoms, a nil or non-positive volume, and a denom that is not a
// registered marker.
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
			name: "rejects an invalid price coin",
			nav: types.VaultNAV{
				Denom:  registeredDenom,
				Price:  sdk.Coin{Denom: underlying, Amount: sdkmath.NewInt(-1)},
				Volume: sdkmath.NewInt(1),
			},
			expectedErrSubstr: "invalid NAV price",
		},
		{
			name: "rejects a zero price amount",
			nav: types.VaultNAV{
				Denom:  registeredDenom,
				Price:  sdk.NewInt64Coin(underlying, 0),
				Volume: sdkmath.NewInt(1),
			},
			expectedErrSubstr: "NAV price amount must be positive",
		},
		{
			name: "rejects a price denom outside the vault accepted denoms",
			nav: types.VaultNAV{
				Denom:  registeredDenom,
				Price:  sdk.NewInt64Coin("notaccepted", 100),
				Volume: sdkmath.NewInt(1),
			},
			expectedErrSubstr: "must be an accepted vault denom",
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

func (s *TestSuite) TestKeeper_SetVaultNAV_MetadataDenomSkipsMarkerCheck() {
	underlying := "under"
	share := "vaultshares"
	vault := s.setupBaseVault(underlying, share)
	vaultAddr := types.GetVaultAddress(share)

	navDenom := metadatatypes.ScopeMetadataAddress(uuid.MustParse("00000000-0000-4000-8000-000000000003")).Denom()
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

// TestKeeper_SetNAVAuthority_PersistsAndEmits verifies that a SetNAVAuthority
// call that changes the value assigns the new authority on the vault account,
// persists the change via SetVaultAccount, and emits a single
// EventNAVAuthorityUpdated with the supplied signer recorded for attribution.
// Both a rotation to an explicit address and a reset to the empty string
// (fall-back-to-admin) flow through the same write path.
func (s *TestSuite) TestKeeper_RemoveVaultNAV() {
	cases := []struct {
		name                string
		underlying          string
		share               string
		navDenom            string
		seedNav             *types.VaultNAV
		expectedErrContains string
		expectedLastPrice   string
		expectedLastVolume  string
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
			err := s.k.RemoveVaultNAV(s.ctx, vault, tc.navDenom)

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
			name:        "no NAV entry for the asset denom skips the guardrail",
			assetCoin:   sdk.NewInt64Coin(asset, 10),
			paymentCoin: sdk.NewInt64Coin(underlying, 5),
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
			name:        "settlement at the exact NAV price passes the guardrail",
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

func (s *TestSuite) TestKeeper_PublishAssetNAVToMarker() {
	underlying := "under"
	share := "vshare"
	paymentDenom := "pay"
	asset := "rwacoin"

	tests := []struct {
		name                string
		registerAssetMarker bool
		volume              sdkmath.Int
		expectedErrContains string
	}{
		{
			name:                "registered marker denom publishes the marker NAV attributed to the vault address",
			registerAssetMarker: true,
			volume:              sdkmath.NewInt(10),
		},
		{
			name:                "volume that does not fit in uint64 is rejected before the marker lookup",
			volume:              sdkmath.NewIntFromBigInt(new(big.Int).Lsh(big.NewInt(1), 70)),
			expectedErrContains: "overflows the marker NAV volume",
		},
		{
			name:                "NAV denom without a registered marker fails the lookup",
			volume:              sdkmath.NewInt(10),
			expectedErrContains: "failed to get marker for NAV denom",
		},
		{
			name:                "marker NAV the marker module rejects propagates the error",
			registerAssetMarker: true,
			volume:              sdkmath.ZeroInt(),
			expectedErrContains: "failed to set marker NAV for denom",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			origCtx := s.ctx
			defer func() { s.ctx = origCtx }()
			s.ctx, _ = s.ctx.CacheContext()

			vault, _ := s.setupAssetSettlementVault(underlying, share, paymentDenom)
			if tc.registerAssetMarker {
				s.requireSimpleMarker(asset)
			}
			nav := types.VaultNAV{
				Denom:  asset,
				Price:  sdk.NewInt64Coin(paymentDenom, 5),
				Volume: tc.volume,
				Source: vault.Address,
			}

			s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
			err := s.k.TestAccessor_publishAssetNAVToMarker(s.T(), s.ctx, vault, nav)

			if tc.expectedErrContains != "" {
				s.Require().ErrorContains(err, tc.expectedErrContains, "publishAssetNAVToMarker should fail for denom %s volume %s", nav.Denom, nav.Volume)
				return
			}
			s.Require().NoError(err, "publishAssetNAVToMarker should succeed for denom %s volume %s", nav.Denom, nav.Volume)

			stored, err := s.k.MarkerKeeper.GetNetAssetValue(s.ctx, asset, paymentDenom)
			s.Require().NoError(err, "failed to read marker NAV for %s priced in %s", asset, paymentDenom)
			s.Require().NotNil(stored, "marker NAV for %s priced in %s should exist after publish", asset, paymentDenom)
			s.Assert().Equal(nav.Price, stored.Price, "marker NAV price should match the internal NAV price for %s", asset)
			s.Assert().Equal(nav.Volume.Uint64(), stored.Volume, "marker NAV volume should match the internal NAV volume for %s", asset)
			s.requireTypedEventEmitted(markertypes.NewEventSetNetAssetValue(asset, nav.Price, nav.Volume.Uint64(), vault.Address))
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
