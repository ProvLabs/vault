package keeper_test

import (
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"

	markertypes "github.com/provenance-io/provenance/x/marker/types"

	"github.com/provlabs/vault/types"
)

func (s *TestSuite) TestKeeper_ProcessPendingWithdrawals() {
	shareDenom := "vshare"
	underlyingDenom := "ylds"
	vaultAddr := types.GetVaultAddress(shareDenom)

	assets := sdk.NewInt64Coin(underlyingDenom, 50)
	shares := sdk.NewInt64Coin(shareDenom, 100)

	testBlockTime := time.Now().UTC()
	duePayoutTime := testBlockTime.Add(-1 * time.Hour).Unix()

	tests := []struct {
		name          string
		setup         func() (ownerAddr sdk.AccAddress, reqID uint64)
		posthandler   func(ownerAddr sdk.AccAddress, reqID uint64)
		expectedError string
	}{
		{
			name: "successful payout of due request",
			setup: func() (sdk.AccAddress, uint64) {
				ownerAddr := s.CreateAndFundAccount(sdk.Coin{})
				vault := s.setupBaseVault(underlyingDenom, shareDenom)
				s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.PrincipalMarkerAddress(), sdk.NewCoins(assets)), "should fund vault principal with assets for payout")
				s.Require().NoError(s.k.MarkerKeeper.MintCoin(s.ctx, vault.GetAddress(), shares), "should mint escrowed shares to the vault account")

				req := types.PendingWithdrawal{
					Owner:        ownerAddr.String(),
					VaultAddress: vaultAddr.String(),
					Assets:       assets,
					Shares:       shares,
				}
				id, err := s.k.PendingWithdrawalQueue.Enqueue(s.ctx, duePayoutTime, req)
				s.Require().NoError(err, "should successfully enqueue request")
				return ownerAddr, id
			},
			posthandler: func(ownerAddr sdk.AccAddress, reqID uint64) {
				s.assertBalance(ownerAddr, underlyingDenom, assets.Amount)
				supply := s.k.BankKeeper.GetSupply(s.ctx, shareDenom)
				s.Require().True(supply.Amount.IsZero(), "total supply of shares should be zero after burn")

				expectedEvents := sdk.Events{}
				expectedEvents = append(expectedEvents, createSendCoinEvents(markertypes.MustGetMarkerAddress(shareDenom).String(), ownerAddr.String(), assets.String())...)
				expectedEvents = append(expectedEvents, createMarkerBurn(vaultAddr, markertypes.MustGetMarkerAddress(shareDenom), shares)...)
				expectedEvent, err := sdk.TypedEventToEvent(types.NewEventWithdrawalCompleted(vaultAddr.String(), ownerAddr.String(), assets, reqID))
				s.Require().NoError(err, "should not error converting typed EventWithdrawalCompleted")
				expectedEvents = append(expectedEvents, expectedEvent)
				s.Assert().Equal(
					normalizeEvents(expectedEvents),
					normalizeEvents(s.ctx.EventManager().Events()),
					"a single EventWithdrawalCompleted should be emitted",
				)
			},
		},
		// {
		// 	name: "failed payout refunds shares",
		// 	setup: func() (sdk.AccAddress, uint64) {
		// 		ownerAddr := s.CreateAndFundAccount(sdk.Coin{})
		// 		vault := s.setupBaseVault(underlyingDenom, shareDenom)
		// 		s.Require().NoError(s.k.MarkerKeeper.MintCoin(s.ctx, vault.GetAddress(), shares), "should mint escrowed shares to the vault account")

		// 		req := types.PendingWithdrawal{
		// 			Owner:        ownerAddr.String(),
		// 			VaultAddress: vaultAddr.String(),
		// 			Assets:       assets,
		// 			Shares:       shares,
		// 		}
		// 		id, err := s.k.PendingWithdrawalQueue.Enqueue(s.ctx, duePayoutTime, req)
		// 		s.Require().NoError(err)
		// 		return ownerAddr, id
		// 	},
		// 	posthandler: func(ownerAddr sdk.AccAddress, reqID uint64) {
		// 		s.assertBalance(ownerAddr, shareDenom, shares.Amount)
		// 		reason := "insufficient funds"

		// 		expectedEvent, err := sdk.TypedEventToEvent(types.NewEventWithdrawalRefunded(vaultAddr.String(), ownerAddr.String(), shares, reqID, reason))
		// 		s.Require().NoError(err, "should not error converting typed EventWithdrawalRefunded")
		// 		s.Assert().Equal(
		// 			normalizeEvents(sdk.Events{expectedEvent}),
		// 			normalizeEvents(s.ctx.EventManager().Events()),
		// 			"a single EventWithdrawalRefunded should be emitted",
		// 		)
		// 	},
		// },
		// {
		// 	name: "request for non-existent vault is skipped and dequeued",
		// 	setup: func() (sdk.AccAddress, uint64) {
		// 		ownerAddr := s.CreateAndFundAccount(sdk.Coin{})
		// 		req := types.PendingWithdrawal{
		// 			Owner:        ownerAddr.String(),
		// 			VaultAddress: vaultAddr.String(),
		// 			Assets:       assets,
		// 			Shares:       shares,
		// 		}
		// 		id, err := s.k.PendingWithdrawalQueue.Enqueue(s.ctx, duePayoutTime, req)
		// 		s.Require().NoError(err, "should successfully enqueue request for non-existent vault")
		// 		return ownerAddr, id
		// 	},
		// 	posthandler: func(ownerAddr sdk.AccAddress, reqID uint64) {
		// 		var entries []types.PendingWithdrawal
		// 		s.k.PendingWithdrawalQueue.Walk(s.ctx, func(_ int64, _ sdk.AccAddress, _ uint64, req types.PendingWithdrawal) (bool, error) {
		// 			entries = append(entries, req)
		// 			return false, nil
		// 		})
		// 		s.Require().Empty(entries, "queue should be empty after processing the non-existent vault request")
		// 	},
		// },
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			var ownerAddr sdk.AccAddress
			var reqID uint64
			if tc.setup != nil {
				ownerAddr, reqID = tc.setup()
			}
			s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
			s.ctx = s.ctx.WithBlockTime(testBlockTime)

			err := s.k.ProcessPendingWithdrawals(s.ctx)

			if tc.expectedError != "" {
				s.Require().Error(err)
				s.Require().ErrorContains(err, tc.expectedError)
			} else {
				s.Require().NoError(err)
			}

			if tc.posthandler != nil {
				tc.posthandler(ownerAddr, reqID)
			}
		})
	}
}
