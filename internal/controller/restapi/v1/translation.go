package v1

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/evrone/go-clean-template/internal/controller/restapi/middleware"
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/request"
	"github.com/evrone/go-clean-template/internal/entity"
)

type historyInput struct{}

type historyOutput struct {
	Body entity.TranslationHistory
}

type doTranslateInput struct {
	Body request.Translate
}

type doTranslateOutput struct {
	Body entity.Translation
}

func (r *V1) history(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "history",
		Summary:     "Show history",
		Description: "Show all translation history for current user",
		Method:      http.MethodGet,
		Path:        "/v1/translation/history",
		Tags:        []string{_translationTag},
		Security:    []map[string][]string{{_bearerAuthScheme: {}}},
		Errors:      []int{http.StatusUnauthorized, http.StatusInternalServerError},
	}, func(ctx context.Context, _ *historyInput) (*historyOutput, error) {
		userID, ok := middleware.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("unauthorized")
		}

		translationHistory, err := r.t.History(ctx, userID)
		if err != nil {
			r.l.Error(err, "restapi - v1 - history")

			return nil, huma.Error500InternalServerError("database problems")
		}

		return &historyOutput{Body: translationHistory}, nil
	})
}

func (r *V1) doTranslate(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "do-translate",
		Summary:     "Translate",
		Description: "Translate a text",
		Method:      http.MethodPost,
		Path:        "/v1/translation/do-translate",
		Tags:        []string{_translationTag},
		Security:    []map[string][]string{{_bearerAuthScheme: {}}},
		Errors:      []int{http.StatusUnauthorized, http.StatusInternalServerError},
	}, func(ctx context.Context, input *doTranslateInput) (*doTranslateOutput, error) {
		userID, ok := middleware.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("unauthorized")
		}

		translation, err := r.t.Translate(
			ctx,
			userID,
			entity.Translation{
				Source:      input.Body.Source,
				Destination: input.Body.Destination,
				Original:    input.Body.Original,
			},
		)
		if err != nil {
			r.l.Error(err, "restapi - v1 - doTranslate")

			return nil, huma.Error500InternalServerError("translation service problems")
		}

		return &doTranslateOutput{Body: translation}, nil
	})
}
