package gameplay

import (
	"time"

	"ChessLI/internal/identity"
)

const drawOfferCooldown = 2 * time.Minute

type drawOfferState struct {
	pending       *DrawOffer
	lastOfferedAt map[identity.ProfileID]time.Time
}

// offer records a pending offer after gameState has checked participant eligibility.
func (d *drawOfferState) offer(profileID identity.ProfileID, now time.Time) error {
	if d.pending != nil {
		return ErrDrawOfferPending
	}
	if lastOfferedAt, ok := d.lastOfferedAt[profileID]; ok && now.Sub(lastOfferedAt) < drawOfferCooldown {
		return ErrDrawOfferCooldown
	}
	d.pending = &DrawOffer{OfferID: identity.NewDrawOfferID(), OfferedBy: profileID}
	d.lastOfferedAt[profileID] = now
	return nil
}

func (d *drawOfferState) validateResponse(command DrawOfferResponseCommand) error {
	offer := d.pending
	if offer == nil {
		return ErrDrawOfferNotFound
	}
	if offer.OfferID != command.OfferID {
		return ErrStaleDrawOffer
	}
	if offer.OfferedBy == command.ProfileID {
		return ErrCannotRespondToOwnDrawOffer
	}
	return nil
}
