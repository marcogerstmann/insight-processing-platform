// Thin fetch wrapper for the REST API — no client library. Every call attaches
// the Cognito ID token as a bearer credential, matching the API Gateway JWT
// authorizer + the Gin middleware in
// internal/adapters/inbound/http/rest/auth/cognito.go.
const baseUrl = import.meta.env.VITE_REST_API_ENDPOINT;

// Raised for any non-2xx response so sections can render the message inline.
// Carries the HTTP status so a 401 can be worded as a session problem.
export class ApiError extends Error {
  constructor(
    public readonly status: number,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

export async function apiRequest<T>(
  path: string,
  token: string,
  init: RequestInit = {},
): Promise<T> {
  const res = await fetch(`${baseUrl}${path}`, {
    ...init,
    headers: {
      ...init.headers,
      Authorization: `Bearer ${token}`,
    },
  });

  if (res.status === 401) {
    // The token is expired or otherwise rejected by the authorizer. Surface it
    // as a clear, actionable message instead of a blank screen.
    throw new ApiError(401, "Session expired or invalid — please log in again.");
  }
  if (!res.ok) {
    throw new ApiError(res.status, `Request failed (HTTP ${res.status})`);
  }

  return res.json() as Promise<T>;
}
