package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/marcogerstmann/insight-processing-platform/internal/apperr"
)

// TenantIDKey is the Gin context key the middleware stores the authenticated
// tenant ID under. Handlers must read it instead of the tenantID path param,
// which is untrusted client input.
const TenantIDKey = "tenantID"

// tenantIDClaim is a custom Cognito user pool attribute. It's only present on
// ID tokens (custom attributes aren't included in access tokens), so callers
// must authenticate with an ID token, matching the audience check below.
const tenantIDClaim = "custom:tenant_id"

// CognitoValidator validates Cognito-issued JWTs against the user pool's JWKS
// and extracts the caller's tenant ID from token claims.
type CognitoValidator struct {
	keyfunc  keyfunc.Keyfunc
	issuer   string
	audience string
}

// NewCognitoValidator fetches the user pool's JWKS and keeps it refreshed in
// the background for the lifetime of ctx.
func NewCognitoValidator(ctx context.Context, region, userPoolID, clientID string) (*CognitoValidator, error) {
	issuer := fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/%s", region, userPoolID)

	kf, err := keyfunc.NewDefaultCtx(ctx, []string{issuer + "/.well-known/jwks.json"})
	if err != nil {
		return nil, fmt.Errorf("failed to load Cognito JWKS: %w", err)
	}

	return &CognitoValidator{keyfunc: kf, issuer: issuer, audience: clientID}, nil
}

// Middleware validates the Authorization header on every request and, on
// success, stores the token's tenant ID under TenantIDKey. Requests without a
// valid token are aborted with 401 before reaching the handler.
func (v *CognitoValidator) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, err := v.authenticate(c.Request.Context(), c.GetHeader("Authorization"))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		c.Set(TenantIDKey, tenantID)
		c.Next()
	}
}

func (v *CognitoValidator) authenticate(ctx context.Context, authHeader string) (string, error) {
	tokenStr, ok := strings.CutPrefix(authHeader, "Bearer ")
	tokenStr = strings.TrimSpace(tokenStr)
	if !ok || tokenStr == "" {
		return "", apperr.E(apperr.ErrUnauthorized, errors.New("missing bearer token"))
	}

	token, err := jwt.Parse(tokenStr, v.keyfunc.KeyfuncCtx(ctx),
		jwt.WithIssuer(v.issuer),
		jwt.WithAudience(v.audience),
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithExpirationRequired(),
	)
	if err != nil || !token.Valid {
		return "", apperr.E(apperr.ErrUnauthorized, fmt.Errorf("invalid token: %w", err))
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", apperr.E(apperr.ErrUnauthorized, errors.New("invalid token claims"))
	}

	if use, _ := claims["token_use"].(string); use != "id" {
		return "", apperr.E(apperr.ErrUnauthorized, errors.New("expected a Cognito ID token"))
	}

	tenantID, _ := claims[tenantIDClaim].(string)
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return "", apperr.E(apperr.ErrUnauthorized, errors.New("token missing tenant_id claim"))
	}

	return tenantID, nil
}
