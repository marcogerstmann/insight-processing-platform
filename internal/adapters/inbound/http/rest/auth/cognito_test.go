package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/MicahParks/jwkset"
	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

const (
	testIssuer   = "https://cognito-idp.eu-central-1.amazonaws.com/eu-central-1_test"
	testAudience = "test-client-id"
	testKeyID    = "test-key"
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

	return &CognitoValidator{keyfunc: kf, issuer: testIssuer, audience: testAudience}, privateKey
}

type tokenOverrides struct {
	issuer     string
	audience   string
	tokenUse   string
	tenantID   any
	expiresAt  time.Time
	omitExpiry bool
}

func signToken(t *testing.T, key *rsa.PrivateKey, o tokenOverrides) string {
	t.Helper()

	claims := jwt.MapClaims{
		"iss":       o.issuer,
		"aud":       o.audience,
		"sub":       "test-user",
		"token_use": o.tokenUse,
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

func TestAuthenticate_Success(t *testing.T) {
	validator, key := newTestValidator(t)
	token := signToken(t, key, tokenOverrides{
		issuer: testIssuer, audience: testAudience, tokenUse: "id",
		tenantID: "tenant-foo", expiresAt: time.Now().Add(time.Hour),
	})

	tenantID, err := validator.authenticate(context.Background(), "Bearer "+token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tenantID != "tenant-foo" {
		t.Fatalf("tenant id mismatch: got %q want %q", tenantID, "tenant-foo")
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
			overrides: tokenOverrides{issuer: "https://evil.example.com", audience: testAudience, tokenUse: "id", tenantID: "t", expiresAt: time.Now().Add(time.Hour)},
		},
		"wrong audience": {
			overrides: tokenOverrides{issuer: testIssuer, audience: "someone-elses-client", tokenUse: "id", tenantID: "t", expiresAt: time.Now().Add(time.Hour)},
		},
		"expired token": {
			overrides: tokenOverrides{issuer: testIssuer, audience: testAudience, tokenUse: "id", tenantID: "t", expiresAt: time.Now().Add(-time.Hour)},
		},
		"missing expiry": {
			overrides: tokenOverrides{issuer: testIssuer, audience: testAudience, tokenUse: "id", tenantID: "t", omitExpiry: true},
		},
		"access token instead of id token": {
			overrides: tokenOverrides{issuer: testIssuer, audience: testAudience, tokenUse: "access", tenantID: "t", expiresAt: time.Now().Add(time.Hour)},
		},
		"missing tenant claim": {
			overrides: tokenOverrides{issuer: testIssuer, audience: testAudience, tokenUse: "id", expiresAt: time.Now().Add(time.Hour)},
		},
		"blank tenant claim": {
			overrides: tokenOverrides{issuer: testIssuer, audience: testAudience, tokenUse: "id", tenantID: "   ", expiresAt: time.Now().Add(time.Hour)},
		},
		"non-string tenant claim": {
			overrides: tokenOverrides{issuer: testIssuer, audience: testAudience, tokenUse: "id", tenantID: 12345, expiresAt: time.Now().Add(time.Hour)},
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
