package user

import (
	v1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	"github.com/m7medvision/go-clean-template/internal/controller/grpc/v1/response"
	"github.com/m7medvision/go-clean-template/internal/entity"
)

// NewUserResponse is user as Register and GetProfile answer it. Their two messages have the same
// fields, so one converts to the other; the proto keeps both names, which clients compile against.
func NewUserResponse[T v1.RegisterResponse | v1.GetProfileResponse](user *entity.User) *T {
	resp := T(v1.RegisterResponse{
		Id:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		CreatedAt: response.Time(user.CreatedAt),
		UpdatedAt: response.Time(user.UpdatedAt),
	})

	return &resp
}
