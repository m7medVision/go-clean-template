package mail

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/usecase"
)

// UseCase -.
type UseCase struct {
	renderer repo.MailRenderer
	mailer   repo.Mailer
}

// New returns a Mail usecase instrumented with OpenTelemetry tracing spans.
func New(r repo.MailRenderer, m repo.Mailer) usecase.Mail {
	return newTraced(&UseCase{
		renderer: r,
		mailer:   m,
	})
}

// Deliver renders email and sends it. A render error will not go away on a retry; the caller tells
// it apart with errors.Is(err, entity.ErrUnknownEmail) for an email no template exists for.
func (uc *UseCase) Deliver(ctx context.Context, email entity.Email) error {
	msg, err := uc.renderer.Render(ctx, email)
	if err != nil {
		return fmt.Errorf("MailUseCase - Deliver - uc.renderer.Render: %w", err)
	}

	err = uc.mailer.Send(ctx, msg)
	if err != nil {
		return fmt.Errorf("MailUseCase - Deliver - uc.mailer.Send: %w", err)
	}

	return nil
}
