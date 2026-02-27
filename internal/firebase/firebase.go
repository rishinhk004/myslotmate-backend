package firebase

import (
	"context"
	"fmt"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

type Config struct {
	CredentialsFile string
	ProjectID       string
}

type App struct {
	Auth *auth.Client
}

// NewApp initializes a Firebase Admin app (Auth client only for now).
// It expects a path to the service account JSON file via Config.CredentialsFile.
// If no credentials file is provided, it falls back to Application Default Credentials.
func NewApp(ctx context.Context, cfg Config) (*App, error) {
	var opts []option.ClientOption

	if cfg.CredentialsFile != "" {
		opts = append(opts, option.WithCredentialsFile(cfg.CredentialsFile))
	}

	app, err := firebase.NewApp(ctx, &firebase.Config{
		ProjectID: cfg.ProjectID,
	}, opts...)
	if err != nil {
		return nil, fmt.Errorf("firebase.NewApp: %w", err)
	}

	authClient, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("app.Auth: %w", err)
	}

	return &App{
		Auth: authClient,
	}, nil
}

