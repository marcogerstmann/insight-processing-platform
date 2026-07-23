// Minimal browser-side JWT decoding.
//
// We only ever READ claims from a token Cognito already signed and handed back
// over TLS — never to verify it. Signature verification is the API Gateway JWT
// authorizer's job (server-side), so a plain base64url decode of the payload is
// all we need. Do not trust these claims for any security decision.

export interface JwtClaims {
  // Expiry, in seconds since the epoch.
  exp: number;
  // Tenant the REST API scopes requests to (see the Cognito pool's custom
  // attribute in terraform/envs/dev/rest-api.tf).
  "custom:tenant_id"?: string;
  [claim: string]: unknown;
}

export function decodeJwt(token: string): JwtClaims {
  const payload = token.split(".")[1];
  if (!payload) {
    throw new Error("Malformed JWT: missing payload segment");
  }
  // JWT uses base64url (`-`/`_`, no padding); atob expects standard base64.
  const base64 = payload.replace(/-/g, "+").replace(/_/g, "/");
  return JSON.parse(atob(base64)) as JwtClaims;
}

// True when the token has expired or can't be decoded — either way it's unusable.
export function isExpired(token: string): boolean {
  try {
    return decodeJwt(token).exp * 1000 < Date.now();
  } catch {
    return true;
  }
}
