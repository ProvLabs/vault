package types

import (
	"errors"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/provenance-io/provenance/x/exchange"
)

// NewPaymentFromExchange maps an exchange-module payment into the vault module's Payment
// view, which is both what the vault's payment queries return and what an asset manager
// submits as the approved terms of a settlement.
func NewPaymentFromExchange(payment *exchange.Payment) Payment {
	return Payment{
		Source:       payment.Source,
		SourceAmount: payment.SourceAmount,
		Target:       payment.Target,
		TargetAmount: payment.TargetAmount,
		ExternalId:   payment.ExternalId,
	}
}

// Validate performs stateless validation on a Payment, mirroring the exchange module's
// own payment validation so terms approved by an asset manager are rejected here rather
// than deep in the settlement path.
func (p Payment) Validate() error {
	if _, err := sdk.AccAddressFromBech32(p.Source); err != nil {
		return fmt.Errorf("invalid source address: %q: %w", p.Source, err)
	}
	if _, err := sdk.AccAddressFromBech32(p.Target); err != nil {
		return fmt.Errorf("invalid target address: %q: %w", p.Target, err)
	}
	if err := p.SourceAmount.Validate(); err != nil {
		return fmt.Errorf("invalid source amount %q: %w", p.SourceAmount, err)
	}
	if err := p.TargetAmount.Validate(); err != nil {
		return fmt.Errorf("invalid target amount %q: %w", p.TargetAmount, err)
	}
	if p.SourceAmount.IsZero() && p.TargetAmount.IsZero() {
		return errors.New("source amount and target amount cannot both be zero")
	}
	if err := exchange.ValidateExternalID(p.ExternalId); err != nil {
		return fmt.Errorf("invalid external id: %w", err)
	}
	return nil
}

// ValidateMatches requires stored to carry exactly the terms recorded in p, so an approval
// signed over p authorizes that deal alone. A (source, external_id) pair is a reusable
// label rather than a commitment, so it cannot stand in for the terms themselves.
func (p Payment) ValidateMatches(stored *exchange.Payment) error {
	if stored == nil {
		return errors.New("no stored payment to match against")
	}
	switch {
	case p.Source != stored.Source:
		return fmt.Errorf("approved source %s does not match stored source %s", p.Source, stored.Source)
	case p.ExternalId != stored.ExternalId:
		return fmt.Errorf("approved external id %q does not match stored external id %q", p.ExternalId, stored.ExternalId)
	case p.Target != stored.Target:
		return fmt.Errorf("approved target %s does not match stored target %s", p.Target, stored.Target)
	case !p.SourceAmount.Equal(stored.SourceAmount):
		return fmt.Errorf("approved source amount %q does not match stored source amount %q", p.SourceAmount, stored.SourceAmount)
	case !p.TargetAmount.Equal(stored.TargetAmount):
		return fmt.Errorf("approved target amount %q does not match stored target amount %q", p.TargetAmount, stored.TargetAmount)
	}
	return nil
}
