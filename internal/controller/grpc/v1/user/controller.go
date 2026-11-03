// Package user serves AuthService (docs/proto/v1/auth.proto) over gRPC.
package user

import (
	"github.com/go-playground/validator/v10"
	v1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	"github.com/m7medvision/go-clean-template/internal/usecase"
	"github.com/m7medvision/go-clean-template/pkg/logger"
)

// Controller -.
type Controller struct {
	v1.UnimplementedAuthServiceServer

	u usecase.User
	l logger.Interface
	v *validator.Validate
}

// New -.
func New(u usecase.User, l logger.Interface) v1.AuthServiceServer {
	return &Controller{u: u, l: l, v: validator.New(validator.WithRequiredStructEnabled())}
}
