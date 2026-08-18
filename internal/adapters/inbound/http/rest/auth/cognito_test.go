package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MicahParks/jwkset"
	"github.com/MicahParks/keyfunc/v3"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const (
	testIssuer        = "https://cognito-idp.eu-central-1.amazonaws.com/eu-central-1_test"
	testUserClientID  = "test-client-id"
	testAgentClientID = "test-agent-client-id"
	testKeyID         = "test-key"
)

func newTestValidator(t *testing.T) (*CognitoValidator, *rsa.PrivateKey) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}

	jwk, err := jwkset.NewJWKFromKey(privateKey.Public(), jwkset.JWKOptions{
		Metadata: jwkset.JWKMetadataOptions{KID: testKeyID, ALG: jwkset.AlgRS256},
	})
	if err != nil {
		t.Fatalf("failed to build JWK: %v", err)
	}

	storage := jwkset.NewMemoryStorage()
	if err := storage.KeyWrite(context.Background(), jwk); err != nil {
		t.Fatalf("failed to write JWK: %v", err)
	}

	kf, err := keyfunc.New(keyfunc.Options{Storage: storage})
	if err != nil {
		t.Fatalf("failed to build keyfunc: %v", err)
	}

	return &CognitoValidator{keyfunc: kf, issuer: testIssuer, userClientID: testUserClientID, agentClientID: testAgentClientID}, privateKey
}

type tokenOverrides struct {
	issuer     string
	audience   string // "aud" claim — Cognito sets this only on ID tokens
	clientID   string // "client_id" claim — Cognito sets this only on access tokens
	scope      string // "scope" claim, space-delimited — access tokens only
	tokenUse   string
	tenantID   any
	expiresAt  time.Time
	omitExpiry bool
}

func signToken(t *testing.T, key *rsa.PrivateKey, o tokenOverrides) string {
	t.Helper()

	claims := jwt.MapClaims{
		"iss":       o.issuer,
		"sub":       "test-user",
		"token_use": o.tokenUse,
	}
	if o.audience != "" {
		claims["aud"] = o.audience
	}
	if o.clientID != "" {
		claims["client_id"] = o.clientID
	}
	if o.scope != "" {
		claims["scope"] = o.scope
	}
	if o.tenantID != nil {
		claims[tenantIDClaim] = o.tenantID
	}
	if !o.omitExpiry {
		claims["exp"] = jwt.NewNumericDate(o.expiresAt)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = testKeyID

	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return signed
}

func TestAuthenticate_ValidUserToken(t *testing.T) {
	validator, key := newTestValidator(t)
	token := signToken(t, key, tokenOverrides{
		issuer: testIssuer, audience: testUserClientID, tokenUse: "id",
		tenantID: "tenant-foo", expiresAt: time.Now().Add(time.Hour),
	})

	principal, err := validator.authenticate(context.Background(), "Bearer "+token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if principal.Type != PrincipalUser {
		t.Fatalf("principal type = %q, want %q", principal.Type, PrincipalUser)
	}
	if principal.TenantID != "tenant-foo" {
		t.Fatalf("tenant id mismatch: got %q want %q", principal.TenantID, "tenant-foo")
	}
}

func TestAuthenticate_ValidMachineToken(t *testing.T) {
	validator, key := newTestValidator(t)
	token := signToken(t, key, tokenOverrides{
		issuer: testIssuer, clientID: testAgentClientID, tokenUse: "access",
		scope: "ipp/agent.write ipp/other.scope", expiresAt: time.Now().Add(time.Hour),
	})

	principal, err := validator.authenticate(context.Background(), "Bearer "+token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if principal.Type != PrincipalService {
		t.Fatalf("principal type = %q, want %q", principal.Type, PrincipalService)
	}
	if principal.TenantID != "" {
		t.Fatalf("machine token must never carry a tenant, got %q", principal.TenantID)
	}
	want := []string{"ipp/agent.write", "ipp/other.scope"}
	if len(principal.Scopes) != len(want) || principal.Scopes[0] != want[0] || principal.Scopes[1] != want[1] {
		t.Fatalf("scopes = %v, want %v", principal.Scopes, want)
	}
}

func TestAuthenticate_MachineTokenIgnoresASpuriousTenantClaim(t *testing.T) {
	// The trap this ticket calls out explicitly: even if a token carried
	// custom:tenant_id alongside token_use=access, authenticateService must
	// never read it. TenantID must stay empty regardless.
	validator, key := newTestValidator(t)
	token := signToken(t, key, tokenOverrides{
		issuer: testIssuer, clientID: testAgentClientID, tokenUse: "access",
		scope: "ipp/agent.write", tenantID: "should-be-ignored", expiresAt: time.Now().Add(time.Hour),
	})

	principal, err := validator.authenticate(context.Background(), "Bearer "+token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if principal.TenantID != "" {
		t.Fatalf("expected no tenant on a machine token, got %q", principal.TenantID)
	}
}

func TestAuthenticate_Rejections(t *testing.T) {
	validator, key := newTestValidator(t)

	tests := map[string]struct {
		useRaw    bool // use rawHeader verbatim instead of building a signed token from overrides
		rawHeader string
		overrides tokenOverrides
	}{
		"missing header": {
			useRaw:    true,
			rawHeader: "",
		},
		"malformed header": {
			useRaw:    true,
			rawHeader: "Basic sometoken",
		},
		"wrong issuer": {
			overrides: tokenOverrides{issuer: "https://evil.example.com", audience: testUserClientID, tokenUse: "id", tenantID: "t", expiresAt: time.Now().Add(time.Hour)},
		},
		"wrong audience": {
			overrides: tokenOverrides{issuer: testIssuer, audience: "someone-elses-client", tokenUse: "id", tenantID: "t", expiresAt: time.Now().Add(time.Hour)},
		},
		"expired token": {
			overrides: tokenOverrides{issuer: testIssuer, audience: testUserClientID, tokenUse: "id", tenantID: "t", expiresAt: time.Now().Add(-time.Hour)},
		},
		"missing expiry": {
			overrides: tokenOverrides{issuer: testIssuer, audience: testUserClientID, tokenUse: "id", tenantID: "t", omitExpiry: true},
		},
		"unrecognized token_use": {
			overrides: tokenOverrides{issuer: testIssuer, audience: testUserClientID, tokenUse: "refresh", tenantID: "t", expiresAt: time.Now().Add(time.Hour)},
		},
		"missing tenant claim": {
			overrides: tokenOverrides{issuer: testIssuer, audience: testUserClientID, tokenUse: "id", expiresAt: time.Now().Add(time.Hour)},
		},
		"blank tenant claim": {
			overrides: tokenOverrides{issuer: testIssuer, audience: testUserClientID, tokenUse: "id", tenantID: "   ", expiresAt: time.Now().Add(time.Hour)},
		},
		"non-string tenant claim": {
			overrides: tokenOverrides{issuer: testIssuer, audience: testUserClientID, tokenUse: "id", tenantID: 12345, expiresAt: time.Now().Add(time.Hour)},
		},
		"machine token with wrong client_id": {
			overrides: tokenOverrides{issuer: testIssuer, clientID: "someone-elses-agent-client", tokenUse: "access", scope: "ipp/agent.write", expiresAt: time.Now().Add(time.Hour)},
		},
		"expired machine token": {
			overrides: tokenOverrides{issuer: testIssuer, clientID: testAgentClientID, tokenUse: "access", scope: "ipp/agent.write", expiresAt: time.Now().Add(-time.Hour)},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			authHeader := tc.rawHeader
			if !tc.useRaw {
				authHeader = "Bearer " + signToken(t, key, tc.overrides)
			}

			_, err := validator.authenticate(context.Background(), authHeader)
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
		})
	}
}

// newTestRouter wires Middleware() in front of one user-only route
// (RequireUser) and one agent-only route (RequireScope), the same shape
// router.go uses — so these tests exercise the full chain a real request
// takes, not just authenticate() in isolation.
func newTestRouter(v *CognitoValidator) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(v.Middleware())
	r.GET("/user-only", RequireUser(), func(c *gin.Context) { c.Status(http.StatusOK) })
	r.POST("/agent-only", RequireScope("ipp/agent.write"), func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func TestMiddleware_RouteAuthorization(t *testing.T) {
	validator, key := newTestValidator(t)
	router := newTestRouter(validator)

	userToken := signToken(t, key, tokenOverrides{
		issuer: testIssuer, audience: testUserClientID, tokenUse: "id",
		tenantID: "tenant-foo", expiresAt: time.Now().Add(time.Hour),
	})
	machineToken := signToken(t, key, tokenOverrides{
		issuer: testIssuer, clientID: testAgentClientID, tokenUse: "access",
		scope: "ipp/agent.write", expiresAt: time.Now().Add(time.Hour),
	})
	wrongScopeToken := signToken(t, key, tokenOverrides{
		issuer: testIssuer, clientID: testAgentClientID, tokenUse: "access",
		scope: "ipp/some.other.scope", expiresAt: time.Now().Add(time.Hour),
	})
	expiredMachineToken := signToken(t, key, tokenOverrides{
		issuer: testIssuer, clientID: testAgentClientID, tokenUse: "access",
		scope: "ipp/agent.write", expiresAt: time.Now().Add(-time.Hour),
	})

	tests := map[string]struct {
		method, path, token string
		wantStatus          int
	}{
		"user token on user-only route":            {http.MethodGet, "/user-only", userToken, http.StatusOK},
		"machine token on user-only route":         {http.MethodGet, "/user-only", machineToken, http.StatusForbidden},
		"machine token on agent-only route":        {http.MethodPost, "/agent-only", machineToken, http.StatusOK},
		"user token on agent-only route":           {http.MethodPost, "/agent-only", userToken, http.StatusForbidden},
		"wrong-scope machine token on agent route": {http.MethodPost, "/agent-only", wrongScopeToken, http.StatusForbidden},
		"expired machine token on agent route":     {http.MethodPost, "/agent-only", expiredMachineToken, http.StatusUnauthorized},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("Authorization", "Bearer "+tc.token)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tc.wantStatus)
			}
		})
	}
}
