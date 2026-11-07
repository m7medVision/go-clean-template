// Package command holds the passkey domain's commands, one file each, with their handlers.
package command

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/usecase/aggregate"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// FinishPasskeyRegistration verifies the browser's credential against the ceremony in Token and
// registers it to UserID as a passkey named Name (trimmed; blank gets the default). A token or body
// that does not complete the ceremony is entity.ErrPasskeyCeremonyInvalid, and a credential that is
// registered already entity.ErrPasskeyExists.
type FinishPasskeyRegistration struct {
	cqrs.Command[entity.Written]

	UserID string
	Token  string
	Name   string
	Body   []byte
}

func (c FinishPasskeyRegistration) ActorID() string { return c.UserID }

// FinishPasskeyRegistrationHandler's Now is the clock (time.Now when nil).
type FinishPasskeyRegistrationHandler struct {
	Ceremony repo.PasskeyCeremony
	Store    repo.EventStore
	Now      func() time.Time
}

func (h FinishPasskeyRegistrationHandler) Handle(ctx context.Context, cmd FinishPasskeyRegistration) (entity.Written, error) {
	id, record, err := h.Ceremony.FinishRegistration(cmd.UserID, cmd.Token, cmd.Body)
	if err != nil {
		return entity.Written{}, fmt.Errorf("FinishPasskeyRegistration: %w", err)
	}

	// A credential that was deleted has a stream already, and can be registered again on it.
	passkey := &entity.PasskeyAggregate{}
	if err = h.Store.Load(ctx, entity.PasskeyStreamID(id), passkey); err != nil && !errors.Is(err, entity.ErrStreamNotFound) {
		return entity.Written{}, fmt.Errorf("FinishPasskeyRegistration - h.Store.Load: %w", err)
	}

	if err = passkey.Register(id, cmd.UserID, cmd.Name, record, aggregate.Now(h.Now)); err != nil {
		return entity.Written{}, fmt.Errorf("FinishPasskeyRegistration: %w", err)
	}

	written, err := aggregate.Save(ctx, h.Store, passkey)
	if err != nil {
		return entity.Written{}, fmt.Errorf("FinishPasskeyRegistration - aggregate.Save: %w", err)
	}

	return written, nil
}
