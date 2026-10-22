package notify

import (
	"context"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

const _tracerName = "github.com/m7medvision/go-clean-template/internal/repo/notify"

// tracedNotifier wraps a UserNotifier with OpenTelemetry spans, giving a semantic
// "UserNotifier.<Method>" view above the queue spans.
type tracedNotifier struct {
	next repo.UserNotifier
}

func newTraced(next repo.UserNotifier) repo.UserNotifier {
	return &tracedNotifier{next: next}
}

func (n *tracedNotifier) Welcome(ctx context.Context, user *entity.User) error {
	ctx, span := otel.Tracer(_tracerName).Start(ctx, "UserNotifier.Welcome")
	defer span.End()

	err := n.next.Welcome(ctx, user)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	return err
}
