// Thin wrapper around the Cognito Identity Provider HTTP API.
//
// USER_PASSWORD_AUTH's InitiateAuth is an unauthenticated action (no SigV4
// request signing), so a single JSON POST does the job — we avoid pulling in
// @aws-sdk/client-cognito-identity-provider, which would dwarf the rest of this
// demo client.
const region = import.meta.env.VITE_AWS_REGION;
const clientId = import.meta.env.VITE_COGNITO_CLIENT_ID;

const endpoint = `https://cognito-idp.${region}.amazonaws.com/`;

export interface AuthTokens {
  idToken: string;
  accessToken: string;
  expiresIn: number;
}

// Authenticate with email + password. Resolves with the Cognito tokens on
// success; rejects with the Cognito error message (e.g. "Incorrect username or
// password.") so the caller can show it inline.
export async function initiateAuth(
  email: string,
  password: string,
): Promise<AuthTokens> {
  const res = await fetch(endpoint, {
    method: "POST",
    headers: {
      "Content-Type": "application/x-amz-json-1.1",
      "X-Amz-Target": "AWSCognitoIdentityProviderService.InitiateAuth",
    },
    body: JSON.stringify({
      AuthFlow: "USER_PASSWORD_AUTH",
      ClientId: clientId,
      AuthParameters: { USERNAME: email, PASSWORD: password },
    }),
  });

  const body = await res.json();

  if (!res.ok) {
    // Cognito errors look like:
    // { "__type": "NotAuthorizedException", "message": "Incorrect username or password." }
    throw new Error(body.message ?? body.__type ?? "Login failed");
  }

  // A permanent-password user logs in straight to AuthenticationResult. Any
  // challenge (NEW_PASSWORD_REQUIRED, MFA, …) is out of scope for this demo, so
  // surface it as an error rather than silently stalling.
  if (!body.AuthenticationResult) {
    throw new Error(
      `Unexpected Cognito challenge: ${body.ChallengeName ?? "unknown"}`,
    );
  }

  const result = body.AuthenticationResult;
  return {
    idToken: result.IdToken,
    accessToken: result.AccessToken,
    expiresIn: result.ExpiresIn,
  };
}
