package auth_test

import (
	"context"
	"errors"
	"testing"

	fbauth "firebase.google.com/go/v4/auth"

	"patungan_app_echo/internal/models"
	auth "patungan_app_echo/internal/modules/auth"
)

type fakeVerifier struct {
	token *fbauth.Token
	err   error
}

func (f fakeVerifier) VerifyIDToken(ctx context.Context, token string) (*fbauth.Token, error) {
	return f.token, f.err
}

type fakeUserRepo struct {
	user     *models.User
	err      error
	gotEmail string
}

func (f *fakeUserRepo) FindByEmail(email string) (*models.User, error) {
	f.gotEmail = email
	return f.user, f.err
}

func TestResolveUser_RejectsUnregistered(t *testing.T) {
	svc := auth.NewService(&fakeUserRepo{}) // no user registered
	verifier := fakeVerifier{token: &fbauth.Token{Claims: map[string]interface{}{"email": "a@b.c"}}}

	_, err := svc.ResolveUser(context.Background(), "id-token", verifier)
	if !errors.Is(err, auth.ErrNotRegistered) {
		t.Fatalf("err = %v, want ErrNotRegistered", err)
	}
}

func TestResolveUser_InvalidToken(t *testing.T) {
	svc := auth.NewService(&fakeUserRepo{user: &models.User{}})
	verifier := fakeVerifier{err: errors.New("token verification failed")}

	_, err := svc.ResolveUser(context.Background(), "id-token", verifier)
	if !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("err = %v, want ErrInvalidToken", err)
	}
}

func TestResolveUser_ReturnsRegisteredUser(t *testing.T) {
	repo := &fakeUserRepo{user: &models.User{Email: "a@b.c"}}
	svc := auth.NewService(repo)
	verifier := fakeVerifier{token: &fbauth.Token{Claims: map[string]interface{}{"email": "a@b.c"}}}

	user, err := svc.ResolveUser(context.Background(), "id-token", verifier)
	if err != nil {
		t.Fatalf("ResolveUser returned error: %v", err)
	}
	if user != repo.user {
		t.Fatalf("user = %+v, want repo user", user)
	}
	if repo.gotEmail != "a@b.c" {
		t.Fatalf("repo lookup email = %q, want %q (email claim)", repo.gotEmail, "a@b.c")
	}
}
