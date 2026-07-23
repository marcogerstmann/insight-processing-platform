import { useAuth } from "../auth/AuthContext.tsx";
import { decodeJwt, type JwtClaims } from "../auth/jwt.ts";
import { NotLoggedIn } from "./NotLoggedIn.tsx";

// The two claims the backend middleware (auth/cognito.go) actually checks — we
// surface them first and highlight them.
const HIGHLIGHT_CLAIMS = ["custom:tenant_id", "token_use"];

// Unix-timestamp claims (seconds) worth rendering as a readable date too.
const TIME_CLAIMS = new Set(["exp", "iat", "auth_time"]);

function formatValue(key: string, value: unknown): string {
  if (TIME_CLAIMS.has(key) && typeof value === "number") {
    return `${value} (${new Date(value * 1000).toLocaleString()})`;
  }
  if (Array.isArray(value)) return value.join(", ");
  if (value !== null && typeof value === "object") return JSON.stringify(value);
  return String(value);
}

// Highlighted claims first (in the order above), then the rest as decoded.
function orderedEntries(claims: JwtClaims): [string, unknown][] {
  const highlighted = HIGHLIGHT_CLAIMS.filter((k) => k in claims).map(
    (k) => [k, claims[k]] as [string, unknown],
  );
  const rest = Object.entries(claims).filter(
    ([k]) => !HIGHLIGHT_CLAIMS.includes(k),
  );
  return [...highlighted, ...rest];
}

// Profile / token claims view. Decodes the logged-in user's Cognito ID token
// client-side and lists its claims — no backend call. This makes it obvious the
// JWT really carries custom:tenant_id, token_use, exp, etc.
export function ProfileSection() {
  const { token } = useAuth();

  if (!token) {
    return (
      <section>
        <h2>Profile</h2>
        <NotLoggedIn />
      </section>
    );
  }

  let claims: JwtClaims;
  try {
    claims = decodeJwt(token);
  } catch {
    return (
      <section>
        <h2>Profile</h2>
        <p className="error" role="alert">
          Could not decode the ID token.
        </p>
      </section>
    );
  }

  return (
    <section>
      <h2>Profile</h2>
      <p className="placeholder">
        Claims decoded from your Cognito ID token, client-side. They are not
        verified here — the API re-validates the signature server-side.
      </p>
      <table className="insights-table claims-table">
        <tbody>
          {orderedEntries(claims).map(([key, value]) => (
            <tr
              key={key}
              className={HIGHLIGHT_CLAIMS.includes(key) ? "highlight" : undefined}
            >
              <th>{key}</th>
              <td>{formatValue(key, value)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}
