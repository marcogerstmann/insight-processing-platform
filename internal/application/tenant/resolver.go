package tenant

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

type Resolver struct {
	secrets ports.SecretProvider

	// cache across invocations in the same Lambda execution environment (cold start -> warm)
	readwiseSecretOnce sync.Once
	readwiseSecret     string
	readwiseSecretErr  error
}

func NewResolver(secrets ports.SecretProvider) *Resolver {
	return &Resolver{secrets: secrets}
}

type Context struct {
	TenantID string
}

func (r *Resolver) Resolve(in ResolveInput) (Context, error) {
	tenantID := strings.TrimSpace(os.Getenv("DEFAULT_TENANT_ID"))
	if tenantID == "" {
		return Context{}, apperr.E(apperr.ErrServerMisconfigured, errors.New("default tenant ID not configured"))
	}

	if in.Source == "readwise" {
		if _, err := r.expectedReadwiseSecret(); err != nil {
			return Context{}, apperr.E(apperr.ErrServerMisconfigured, err)
		}
	}

	if !r.authorized(in) {
		return Context{}, apperr.E(apperr.ErrUnauthorized, errors.New("invalid webhook secret"))
	}

	return Context{TenantID: tenantID}, nil
}

func (r *Resolver) authorized(in ResolveInput) bool {
	switch in.Source {
	case "readwise":
		return r.authorizedReadwise(in)
	default:
		return false
	}
}

func (r *Resolver) authorizedReadwise(in ResolveInput) bool {
	secretExpected, err := r.expectedReadwiseSecret()
	if err != nil {
		return false
	}
	return strings.TrimSpace(in.Secret) == secretExpected
}

func (r *Resolver) expectedReadwiseSecret() (string, error) {
	r.readwiseSecretOnce.Do(func() {
		if v := strings.TrimSpace(os.Getenv("READWISE_WEBHOOK_SECRET")); v != "" {
			r.readwiseSecret = v
			return
		}

		paramName := strings.TrimSpace(os.Getenv("READWISE_WEBHOOK_SECRET_SSM"))
		if paramName == "" {
			r.readwiseSecretErr = errors.New("neither READWISE_WEBHOOK_SECRET nor READWISE_WEBHOOK_SECRET_SSM set")
			return
		}

		if r.secrets == nil {
			r.readwiseSecretErr = errors.New("secret provider not configured; set READWISE_WEBHOOK_SECRET env var")
			return
		}

		secret, err := r.secrets.Get(context.Background(), paramName)
		if err != nil {
			r.readwiseSecretErr = fmt.Errorf("failed to load readwise webhook secret from SSM (%s): %w", paramName, err)
			return
		}
		if secret == "" {
			r.readwiseSecretErr = fmt.Errorf("SSM parameter resolved to empty secret (%s)", paramName)
			return
		}

		r.readwiseSecret = secret
	})

	if r.readwiseSecretErr != nil {
		return "", r.readwiseSecretErr
	}
	if r.readwiseSecret == "" {
		return "", errors.New("readwise webhook secret resolved to empty")
	}
	return r.readwiseSecret, nil
}
