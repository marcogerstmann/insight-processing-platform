package tenant

import (
	"errors"
	"os"
	"strings"

	"github.com/mgerstmannsf/insight-processing-platform/internal/application/apperr"
)

type Resolver struct {
}

func NewResolver() *Resolver {
	return &Resolver{}
}

type Context struct {
	TenantID string
}

func (r *Resolver) Resolve(in ResolveInput) (Context, error) {
	// Today: single-tenant mode, using env secret + fixed tenant id.
	// Later: extract secret from header/query, look up tenant in DB, verify signature, etc.
	tenantID := strings.TrimSpace(os.Getenv("DEFAULT_TENANT_ID"))
	if tenantID == "" {
		return Context{}, apperr.E(apperr.ErrServerMisconfigured, errors.New("default tenant ID not configured"))
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
	secretExpected := strings.TrimSpace(os.Getenv("READWISE_WEBHOOK_SECRET"))
	if secretExpected == "" {
		return false
	}

	if strings.TrimSpace(in.Secret) != secretExpected {
		return false
	}

	return true
}
