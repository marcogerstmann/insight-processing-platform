// Shared placeholder shown by every section that will eventually need an
// authenticated session. Login/token handling arrives in IPP-71; until then
// these sections have nothing real to render.
export function NotLoggedIn() {
  return (
    <p className="placeholder">
      Not logged in. Sign in from the <strong>Login</strong> section first.
    </p>
  );
}
