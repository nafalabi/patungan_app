package services

import (
	"context"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

// InitFirebase initializes the Firebase Admin SDK and returns an auth client
func InitFirebase(credPath string) (*auth.Client, error) {
	opt := option.WithCredentialsFile(credPath)
	app, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		return nil, err
	}
	return app.Auth(context.Background())
}
