package v1

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/evrone/go-clean-template/internal/controller/restapi/middleware"
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/request"
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
)

type registerInput struct {
	Body request.Register
}

type registerOutput struct {
	Body entity.User
}

type loginInput struct {
	Body request.Login
}

type loginOutput struct {
	Body response.Token
}

type profileInput struct{}

type profileOutput struct {
	Body entity.User
}

func (r *V1) register(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID:   "register",
		Summary:       "Register",
		Description:   "Register a new user",
		Method:        http.MethodPost,
		Path:          "/v1/auth/register",
		Tags:          []string{_authTag},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusConflict, http.StatusInternalServerError},
	}, func(ctx context.Context, input *registerInput) (*registerOutput, error) {
		user, err := r.u.Register(ctx, input.Body.Username, input.Body.Email, input.Body.Password)
		if err != nil {
			r.l.Error(err, "restapi - v1 - register")

			if errors.Is(err, entity.ErrUserAlreadyExists) {
				return nil, huma.Error409Conflict("user already exists")
			}

			return nil, huma.Error500InternalServerError("internal server error")
		}

		return &registerOutput{Body: user}, nil
	})
}

func (r *V1) login(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "login",
		Summary:     "Login",
		Description: "Authenticate user and get JWT token",
		Method:      http.MethodPost,
		Path:        "/v1/auth/login",
		Tags:        []string{_authTag},
		Errors:      []int{http.StatusUnauthorized, http.StatusInternalServerError},
	}, func(ctx context.Context, input *loginInput) (*loginOutput, error) {
		token, err := r.u.Login(ctx, input.Body.Email, input.Body.Password)
		if err != nil {
			r.l.Error(err, "restapi - v1 - login")

			if errors.Is(err, entity.ErrInvalidCredentials) {
				return nil, huma.Error401Unauthorized("invalid credentials")
			}

			return nil, huma.Error500InternalServerError("internal server error")
		}

		return &loginOutput{Body: response.Token{Token: token}}, nil
	})
}

func (r *V1) profile(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "profile",
		Summary:     "Get profile",
		Description: "Get current user profile",
		Method:      http.MethodGet,
		Path:        "/v1/user/profile",
		Tags:        []string{_usersTag},
		Security:    []map[string][]string{{_bearerAuthScheme: {}}},
		Errors:      []int{http.StatusUnauthorized, http.StatusNotFound, http.StatusInternalServerError},
	}, func(ctx context.Context, _ *profileInput) (*profileOutput, error) {
		userID, ok := middleware.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("unauthorized")
		}

		user, err := r.u.GetUser(ctx, userID)
		if err != nil {
			r.l.Error(err, "restapi - v1 - profile")

			if errors.Is(err, entity.ErrUserNotFound) {
				return nil, huma.Error404NotFound("user not found")
			}

			return nil, huma.Error500InternalServerError("internal server error")
		}

		return &profileOutput{Body: user}, nil
	})
}
