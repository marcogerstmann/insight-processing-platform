import { useState, type FormEvent } from "react";
import { useAuth } from "../auth/AuthContext.tsx";

// Login section. Drives the real Cognito USER_PASSWORD_AUTH flow: on success the
// IdToken lands in AuthContext and the other sections can read it; on failure the
// Cognito message is shown inline.
export function LoginSection() {
  const { token, login, logout } = useAuth();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  if (token) {
    return (
      <section>
        <h2>Login</h2>
        <p>You are signed in. The other sections can now use your token.</p>
        <button type="button" onClick={logout}>
          Log out
        </button>
      </section>
    );
  }

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    setError(null);
    setPending(true);
    try {
      await login(email, password);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Login failed");
    } finally {
      setPending(false);
    }
  }

  return (
    <section>
      <h2>Login</h2>
      <form className="login-form" onSubmit={handleSubmit}>
        <label>
          Email
          <input
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            autoComplete="username"
            required
          />
        </label>
        <label>
          Password
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete="current-password"
            required
          />
        </label>
        <button type="submit" disabled={pending}>
          {pending ? "Signing in…" : "Sign in"}
        </button>
        {error && (
          <p className="error" role="alert">
            {error}
          </p>
        )}
      </form>
    </section>
  );
}
