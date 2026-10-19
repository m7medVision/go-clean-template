package user

import (
	"context"
	"fmt"
	"time"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/evrone/go-clean-template/pkg/jwt"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// UseCase -.
type UseCase struct {
	repo     repo.UserRepo
	queue    repo.QueueRepo
	jwt      *jwt.Manager
	loginURL string
}

// New returns a User usecase instrumented with OpenTelemetry tracing spans. loginURL is the sign-in
// page the welcome email links to.
func New(r repo.UserRepo, q repo.QueueRepo, j *jwt.Manager, loginURL string) usecase.User {
	return newTraced(&UseCase{
		repo:     r,
		queue:    q,
		jwt:      j,
		loginURL: loginURL,
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

	// The worker sends it. Store and Send are not one transaction: if Send fails the user exists but
	// the caller still gets the error, rather than the email being lost unseen.
	welcome, err := entity.EncodeEmail(entity.WelcomeEmail{
		To:       user.Email,
		Username: user.Username,
		LoginURL: uc.loginURL,
	})
	if err != nil {
		return entity.User{}, fmt.Errorf("UserUseCase - Register - entity.EncodeEmail: %w", err)
	}

	_, err = uc.queue.Send(ctx, entity.EmailQueue, welcome)
	if err != nil {
		return entity.User{}, fmt.Errorf("UserUseCase - Register - uc.queue.Send: %w", err)
	}

	return user, nil
}

// Login -.
func (uc *UseCase) Login(ctx context.Context, email, password string) (string, error) {
	user, err := uc.repo.GetByEmail(ctx, email)
	if err != nil {
		return "", entity.ErrInvalidCredentials
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return "", entity.ErrInvalidCredentials
	}

	token, err := uc.jwt.GenerateToken(user.ID)
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
