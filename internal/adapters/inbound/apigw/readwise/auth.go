package readwise

import (
	"context"
	"errors"
	"sync"

	"github.com/marcogerstmann/insight-processing-platform/internal/apperr"
	"github.com/marcogerstmann/insight-processing-platform/internal/envutil"
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
	if incoming != expected {
		return apperr.E(apperr.ErrUnauthorized, errors.New("invalid webhook secret"))
	}
	return nil
}

func (a *webhookAuthenticator) expectedSecret() (string, error) {
	a.once.Do(func() {
		secret, err := envutil.ResolveSecret(context.Background(), "READWISE_WEBHOOK_SECRET", a.secrets)
		if err != nil {
			a.err = err
			return
		}
		if secret == "" {
			a.err = errors.New("READWISE_WEBHOOK_SECRET is not set")
			return
		}
		a.secret = secret
	})

	return a.secret, a.err
}
