import {
  createContext,
  useContext,
  useState,
  type ReactNode,
} from "react";
import { initiateAuth } from "./cognito.ts";
import { isExpired } from "./jwt.ts";

// Key under which the IdToken is cached so a page refresh keeps the session.
const STORAGE_KEY = "ipp.idToken";

interface AuthState {
  // The Cognito IdToken, or null when signed out. Sections read this to
  // authorize their REST calls (IPP-72/73) and decode tenant claims (IPP-74).
  token: string | null;
  login: (email: string, password: string) => Promise<void>;
  logout: () => void;
}

const AuthContext = createContext<AuthState | null>(null);

// Restore a cached token on load, but only if it's still valid. Cognito ID
// tokens live one hour and we don't refresh them (out of scope), so an expired
// one is discarded here rather than restored into a "looks logged in but every
// request 401s" state.
function loadStoredToken(): string | null {
  const stored = localStorage.getItem(STORAGE_KEY);
  if (!stored) return null;
  if (isExpired(stored)) {
    localStorage.removeItem(STORAGE_KEY);
    return null;
  }
  return stored;
}

export function AuthProvider({ children }: { children: ReactNode }) {
  // Persisted in localStorage so a refresh keeps the session. Trade-off: any
  // injected script can read it — acceptable for this demo against the dev pool,
  // but this is the line you'd revisit for production (httpOnly cookie + a
  // backed refresh flow).
  const [token, setToken] = useState<string | null>(loadStoredToken);

  async function login(email: string, password: string) {
    const { idToken } = await initiateAuth(email, password);
    localStorage.setItem(STORAGE_KEY, idToken);
    setToken(idToken);
  }

  function logout() {
    localStorage.removeItem(STORAGE_KEY);
    setToken(null);
  }

  return (
    <AuthContext.Provider value={{ token, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

// Access the auth session from any section. Throws if used outside the
// provider, which can only happen through a wiring mistake.
export function useAuth(): AuthState {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error("useAuth must be used within <AuthProvider>");
  }
  return ctx;
}
