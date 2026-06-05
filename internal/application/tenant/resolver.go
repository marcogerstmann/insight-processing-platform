package tenant

import (
	"errors"
	"os"
	"strings"

	"github.com/marcogerstmann/insight-processing-platform/internal/apperr"
)

type Resolver struct{}

func NewResolver() *Resolver {
	return &Resolver{}
}

type Context struct {
	TenantID string
}

func (r *Resolver) Resolve() (Context, error) {
	tenantID := strings.TrimSpace(os.Getenv("DEFAULT_TENANT_ID"))
	if tenantID == "" {
		return Context{}, apperr.E(apperr.ErrServerMisconfigured, errors.New("default tenant ID not configured"))
	}
	return Context{TenantID: tenantID}, nil
}
