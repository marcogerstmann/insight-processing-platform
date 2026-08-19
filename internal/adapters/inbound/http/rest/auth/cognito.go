package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/marcogerstmann/insight-processing-platform/internal/apperr"
)

// PrincipalType distinguishes a human user (a Cognito ID token) from the AI
// service authenticating as itself (a Cognito access token minted by the
// client_credentials grant, IPP-94).
type PrincipalType string

const (
	PrincipalUser    PrincipalType = "user"
	PrincipalService PrincipalType = "service"
)

// TenantIDKey is the Gin context key the middleware stores the authenticated
// tenant ID under. Handlers must read it instead of the tenantID path param,
// which is untrusted client input. Only ever set for a PrincipalUser — a
// service principal has no tenant claim to put there (see authenticateService).
const TenantIDKey = "tenantID"

// PrincipalTypeKey is the Gin context key for which kind of caller
// authenticated (PrincipalUser or PrincipalService, stored as a plain string
// so it composes with gin.Context.GetString the same way TenantIDKey does).
// RequireUser and RequireScope read it to enforce the boundary between
// user-only and agent-only routes.
const PrincipalTypeKey = "principalType"

// ScopeKey is the Gin context key for a service principal's granted OAuth
// scopes ([]string). Empty for a user principal — ID tokens carry no
// "scope" claim.
const ScopeKey = "scope"

// ScopeAgentWrite is the OAuth scope granted to the AI service's machine
// client (aws_cognito_user_pool_client.agent, IPP-94), used to gate
// agent-only routes like REL 4/IPP-100's relationship write endpoint.
const ScopeAgentWrite = "ipp/agent.write"

// tenantIDClaim is a custom Cognito user pool attribute. It's only present on
// ID tokens (custom attributes aren't included in access tokens), so callers
// must authenticate with an ID token, matching the audience check below.
const tenantIDClaim = "custom:tenant_id"

// Principal is what a validated token proved about its caller.
type Principal struct {
	Type     PrincipalType
	TenantID string   // set only when Type == PrincipalUser
	Scopes   []string // set only when Type == PrincipalService
}

// CognitoValidator validates Cognito-issued JWTs against the user pool's JWKS
// — either a human's ID token or the AI service's client_credentials access
// token — and extracts what the token proved about its caller.
type CognitoValidator struct {
	keyfunc       keyfunc.Keyfunc
	issuer        string
	userClientID  string
	agentClientID string
}

// NewCognitoValidator fetches the user pool's JWKS and keeps it refreshed in
// the background for the lifetime of ctx. userClientID is the human-facing
// app client (aws_cognito_user_pool_client.rest_api); agentClientID is the
// AI service's machine client (aws_cognito_user_pool_client.agent, IPP-94).
func NewCognitoValidator(ctx context.Context, region, userPoolID, userClientID, agentClientID string) (*CognitoValidator, error) {
	issuer := fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/%s", region, userPoolID)

	kf, err := keyfunc.NewDefaultCtx(ctx, []string{issuer + "/.well-known/jwks.json"})
	if err != nil {
		return nil, fmt.Errorf("failed to load Cognito JWKS: %w", err)
	}

	return &CognitoValidator{keyfunc: kf, issuer: issuer, userClientID: userClientID, agentClientID: agentClientID}, nil
}

// Middleware validates the Authorization header on every request and, on
// success, stores what it learned about the caller under PrincipalTypeKey,
// TenantIDKey and ScopeKey. Requests without a valid token are aborted with
// 401 before reaching the handler. It does not by itself restrict which
// principal type may call a route — see RequireUser / RequireScope for that.
func (v *CognitoValidator) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, err := v.authenticate(c.Request.Context(), c.GetHeader("Authorization"))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		c.Set(PrincipalTypeKey, string(principal.Type))
		c.Set(ScopeKey, principal.Scopes)
		if principal.Type == PrincipalUser {
			c.Set(TenantIDKey, principal.TenantID)
		}
		// No else branch setting TenantIDKey for a service principal: its
		// absence must be a request that never asked for it, not a "" that
		// looks the same as a bug. A handler reading TenantIDKey for a
		// service principal gets Gin's not-set zero value, same as if it
		// forgot to guard the route with RequireUser in the first place —
		// agent-only routes take the tenant as an explicit path/body param.

		c.Next()
	}
}

func (v *CognitoValidator) authenticate(ctx context.Context, authHeader string) (Principal, error) {
	tokenStr, ok := strings.CutPrefix(authHeader, "Bearer ")
	tokenStr = strings.TrimSpace(tokenStr)
	if !ok || tokenStr == "" {
		return Principal{}, apperr.E(apperr.ErrUnauthorized, errors.New("missing bearer token"))
	}

	// No jwt.WithAudience here: Cognito puts the app client ID in "aud" only
	// on ID tokens. Access tokens (both a user's and the agent's) carry no
	// "aud" claim at all — the client is identified by "client_id" instead,
	// checked per token type below, the same distinction API Gateway's own
	// Cognito JWT authorizer makes.
	token, err := jwt.Parse(tokenStr, v.keyfunc.KeyfuncCtx(ctx),
		jwt.WithIssuer(v.issuer),
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithExpirationRequired(),
	)
	if err != nil || !token.Valid {
		return Principal{}, apperr.E(apperr.ErrUnauthorized, fmt.Errorf("invalid token: %w", err))
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return Principal{}, apperr.E(apperr.ErrUnauthorized, errors.New("invalid token claims"))
	}

	switch use, _ := claims["token_use"].(string); use {
	case "id":
		return v.authenticateUser(claims)
	case "access":
		return v.authenticateService(claims)
	default:
		return Principal{}, apperr.E(apperr.ErrUnauthorized, fmt.Errorf("unexpected token_use %q", use))
	}
}

// authenticateUser handles a Cognito ID token: audience is the "aud" claim,
// and custom:tenant_id is required — a human always acts within a tenant.
func (v *CognitoValidator) authenticateUser(claims jwt.MapClaims) (Principal, error) {
	aud, _ := claims["aud"].(string)
	if aud != v.userClientID {
		return Principal{}, apperr.E(apperr.ErrUnauthorized, errors.New("unexpected audience"))
	}

	tenantID, _ := claims[tenantIDClaim].(string)
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return Principal{}, apperr.E(apperr.ErrUnauthorized, errors.New("token missing tenant_id claim"))
	}

	return Principal{Type: PrincipalUser, TenantID: tenantID}, nil
}

// authenticateService handles a Cognito access token minted by the
// client_credentials grant for the AI service's own app client (IPP-94).
// The client is identified by "client_id", not "aud" (Cognito omits "aud"
// from access tokens). It deliberately never reads tenantIDClaim: a machine
// token has no tenant — that's the trap this ticket exists to make explicit
// rather than let a caller assume the claim is just usually absent.
func (v *CognitoValidator) authenticateService(claims jwt.MapClaims) (Principal, error) {
	clientID, _ := claims["client_id"].(string)
	if clientID != v.agentClientID {
		return Principal{}, apperr.E(apperr.ErrUnauthorized, errors.New("unexpected client_id"))
	}

	scopeStr, _ := claims["scope"].(string)
	return Principal{Type: PrincipalService, Scopes: strings.Fields(scopeStr)}, nil
}

// RequireUser rejects a request whose token was not a Cognito ID token —
// i.e. the AI service's machine token calling a user-only route. Must run
// after Middleware(), which populates PrincipalTypeKey.
func RequireUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		if PrincipalType(c.GetString(PrincipalTypeKey)) != PrincipalUser {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		c.Next()
	}
}

// RequireScope rejects a request that isn't a service principal carrying the
// given OAuth scope — i.e. a human's ID token, or a machine token minted for
// a different scope, calling an agent-only route. Must run after
// Middleware(), which populates PrincipalTypeKey and ScopeKey.
func RequireScope(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if PrincipalType(c.GetString(PrincipalTypeKey)) != PrincipalService {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}

		scopes, _ := c.Get(ScopeKey)
		granted, _ := scopes.([]string)
		if !slices.Contains(granted, scope) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}

		c.Next()
	}
}
