import { NotLoggedIn } from "./NotLoggedIn.tsx";

// Profile / token claims view. Decodes and displays the Cognito ID token
// claims (proving the token carries custom:tenant_id) in IPP-74.
export function ProfileSection() {
  return (
    <section>
      <h2>Profile</h2>
      <NotLoggedIn />
    </section>
  );
}
