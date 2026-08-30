package auth

import (
	"context"
	"errors"

	fbauth "firebase.google.com/go/v4/auth"

	"patungan_app_echo/internal/models"
)

var (
	ErrInvalidToken  = errors.New("invalid token")
	ErrNotRegistered = errors.New("user not registered in the system")
)

// TokenVerifier verifies Firebase ID tokens. *fbauth.Client satisfies it.
type TokenVerifier interface {
	VerifyIDToken(ctx context.Context, token string) (*fbauth.Token, error)
}

type Service struct {
	repo UserRepo
}

func NewService(repo UserRepo) *Service {
	return &Service{repo: repo}
}

// ResolveUser verifies the given Firebase ID token and returns the
// registered user matching the token's email claim. Returns ErrInvalidToken
// when the token cannot be verified and ErrNotRegistered when no user has
// the email.
func (s *Service) ResolveUser(ctx context.Context, idToken string, verifier TokenVerifier) (*models.User, error) {
	decoded, err := verifier.VerifyIDToken(ctx, idToken)
	if err != nil {
		return nil, ErrInvalidToken
	}

	email, _ := decoded.Claims["email"].(string)

	user, err := s.repo.FindByEmail(email)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrNotRegistered
	}
	return user, nil
}
