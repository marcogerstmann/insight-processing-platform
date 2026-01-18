package ingest

import (
	"errors"
	"os"
	"strings"
)

func resolveTenant(payload ReadwiseWebhookDTO) (TenantContext, error) {
	// Today: single-tenant mode, using env secret + fixed tenant id.
	// Later: extract secret from header/query, look up tenant in DB, verify signature, etc.
	tenantID := strings.TrimSpace(os.Getenv("DEFAULT_TENANT_ID"))
	if tenantID == "" {
		return TenantContext{}, errors.New("default tenant ID not configured")
	}

	secretExpected := strings.TrimSpace(os.Getenv("READWISE_WEBHOOK_SECRET"))
	if secretExpected == "" {
		return TenantContext{}, errors.New("webhook secret not configured")
	}

	if strings.TrimSpace(payload.Secret) != secretExpected {
		return TenantContext{}, errors.New("unauthorized")
	}

	return TenantContext{TenantID: tenantID}, nil
}
