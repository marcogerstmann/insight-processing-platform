import { NotLoggedIn } from "./NotLoggedIn.tsx";

// Create insight form. Wires up to POST /tenants/{tenantID}/insights in IPP-73.
export function CreateInsightSection() {
  return (
    <section>
      <h2>Create Insight</h2>
      <NotLoggedIn />
    </section>
  );
}
