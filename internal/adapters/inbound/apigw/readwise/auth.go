package readwise

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/marcogerstmann/insight-processing-platform/internal/apperr"
	"github.com/marcogerstmann/insight-processing-platform/internal/ports"
)

type webhookAuthenticator struct {
	secrets ports.SecretProvider
	once    sync.Once
	secret  string
	err     error
}

func newWebhookAuthenticator(secrets ports.SecretProvider) *webhookAuthenticator {
	return &webhookAuthenticator{secrets: secrets}
}

func (a *webhookAuthenticator) Authenticate(incoming string) error {
	expected, err := a.expectedSecret()
	if err != nil {
		return apperr.E(apperr.ErrServerMisconfigured, err)
	}
	if strings.TrimSpace(incoming) != expected {
		return apperr.E(apperr.ErrUnauthorized, errors.New("invalid webhook secret"))
	}
	return nil
}

func (a *webhookAuthenticator) expectedSecret() (string, error) {
	a.once.Do(func() {
		if v := strings.TrimSpace(os.Getenv("READWISE_WEBHOOK_SECRET")); v != "" {
			a.secret = v
			return
		}

		paramName := strings.TrimSpace(os.Getenv("READWISE_WEBHOOK_SECRET_SSM"))
		if paramName == "" {
			a.err = errors.New("neither READWISE_WEBHOOK_SECRET nor READWISE_WEBHOOK_SECRET_SSM set")
			return
		}

		if a.secrets == nil {
			a.err = errors.New("secret provider not configured; set READWISE_WEBHOOK_SECRET env var")
			return
		}

		secret, err := a.secrets.Get(context.Background(), paramName)
		if err != nil {
			a.err = fmt.Errorf("failed to load readwise webhook secret from SSM (%s): %w", paramName, err)
			return
		}
		if secret == "" {
			a.err = fmt.Errorf("SSM parameter resolved to empty secret (%s)", paramName)
			return
		}

		a.secret = secret
	})

	if a.err != nil {
		return "", a.err
	}
	if a.secret == "" {
		return "", errors.New("readwise webhook secret resolved to empty")
	}
	return a.secret, nil
}
