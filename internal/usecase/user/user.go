package user

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/usecase"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"golang.org/x/crypto/bcrypt"
)

// UseCase -.
type UseCase struct {
	repo     repo.UserRepo
	notifier repo.UserNotifier // scaffold:welcome
	jwt      *jwt.Manager
}

// New returns a User usecase instrumented with OpenTelemetry tracing spans.
func New(
	r repo.UserRepo,
	n repo.UserNotifier, // scaffold:welcome
	j *jwt.Manager,
) usecase.User {
	return newTraced(&UseCase{
		repo:     r,
		notifier: n, // scaffold:welcome
		jwt:      j,
	})
}

// Register -.
func (uc *UseCase) Register(ctx context.Context, username, email, password string) (entity.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return entity.User{}, fmt.Errorf("UserUseCase - Register - bcrypt.GenerateFromPassword: %w", err)
	}

	now := time.Now().UTC()

	user := entity.User{
		ID:           uuid.New().String(),
		Username:     username,
		Email:        email,
		PasswordHash: string(hash),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	err = uc.repo.Store(ctx, &user)
	if err != nil {
		return entity.User{}, fmt.Errorf("UserUseCase - Register - uc.repo.Store: %w", err)
	}

	// scaffold:welcome:begin
	// Store and Welcome are not one transaction: if Welcome fails the user exists but the caller
	// still gets the error, rather than the welcome being lost unseen.
	err = uc.notifier.Welcome(ctx, &user)
	if err != nil {
		return entity.User{}, fmt.Errorf("UserUseCase - Register - uc.notifier.Welcome: %w", err)
	}
	// scaffold:welcome:end

	return user, nil
}

// Login -.
func (uc *UseCase) Login(ctx context.Context, email, password string) (string, error) {
	user, err := uc.repo.GetByEmail(ctx, email)
	if err != nil {
		return "", entity.ErrInvalidCredentials
	}

	// A user stored without a password hash has no password to sign in with.
	if user.PasswordHash == "" {
		return "", entity.ErrInvalidCredentials
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return "", entity.ErrInvalidCredentials
	}

	token, err := uc.jwt.GenerateToken(user.ID, entity.RoleNames(entity.LocalRoles()))
	if err != nil {
		return "", fmt.Errorf("UserUseCase - Login - uc.jwt.GenerateToken: %w", err)
	}

	return token, nil
}

// GetUser -.
func (uc *UseCase) GetUser(ctx context.Context, userID string) (entity.User, error) {
	user, err := uc.repo.GetByID(ctx, userID)
	if err != nil {
		return entity.User{}, fmt.Errorf("UserUseCase - GetUser - uc.repo.GetByID: %w", err)
	}

	return user, nil
}
