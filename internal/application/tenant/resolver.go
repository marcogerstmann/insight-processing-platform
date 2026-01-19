package tenant

import (
	"os"
	"strings"

	"github.com/mgerstmannsf/insight-processing-platform/internal/adapters/inbound/lambda/ingest/dto"
	"github.com/mgerstmannsf/insight-processing-platform/internal/application/errors"
)

type Resolver struct {
}

func NewResolver() *Resolver {
	return &Resolver{}
}

type Context struct {
	TenantID string
}

func (r *Resolver) ResolveFromReadwise(payload dto.ReadwiseWebhookDTO) (Context, error) {
	// Today: single-tenant mode, using env secret + fixed tenant id.
	// Later: extract secret from header/query, look up tenant in DB, verify signature, etc.
	tenantID := strings.TrimSpace(os.Getenv("DEFAULT_TENANT_ID"))
	if tenantID == "" {
		return Context{}, errors.NewError(nil, "default tenant ID not configured")
	}

	secretExpected := strings.TrimSpace(os.Getenv("READWISE_WEBHOOK_SECRET"))
	if secretExpected == "" {
		return Context{}, errors.NewError(nil, "webhook secret not configured")
	}

	if strings.TrimSpace(payload.Secret) != secretExpected {
		return Context{}, errors.NewError(nil, "invalid readwise webhook secret")
	}

	return Context{TenantID: tenantID}, nil
}
