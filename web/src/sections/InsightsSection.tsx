import { NotLoggedIn } from "./NotLoggedIn.tsx";

// Insights list. Wires up to GET /tenants/{tenantID}/insights in IPP-72.
export function InsightsSection() {
  return (
    <section>
      <h2>Insights</h2>
      <NotLoggedIn />
    </section>
  );
}
