import { useEffect, useState } from "react";
import { useAuth } from "../auth/AuthContext.tsx";
import { listInsights, type Insight } from "../api/insights.ts";
import { NotLoggedIn } from "./NotLoggedIn.tsx";

// Insights list. On each mount (i.e. every time the section is opened) it calls
// GET /v1/tenants/{tenantID}/insights and renders the result as a plain table.
export function InsightsSection() {
  const { token } = useAuth();
  const [insights, setInsights] = useState<Insight[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!token) return;

    // Guard against a state update after unmount if the user navigates away
    // mid-request.
    let cancelled = false;
    setLoading(true);
    setError(null);
    setInsights(null);

    listInsights(token)
      .then((items) => {
        if (!cancelled) setInsights(items);
      })
      .catch((err) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "Failed to load insights");
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [token]);

  if (!token) {
    return (
      <section>
        <h2>Insights</h2>
        <NotLoggedIn />
      </section>
    );
  }

  return (
    <section>
      <h2>Insights</h2>
      {loading && <p>Loading…</p>}
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      {!loading && !error && insights && insights.length === 0 && (
        <p className="placeholder">No insights yet for this tenant.</p>
      )}
      {!loading && !error && insights && insights.length > 0 && (
        <table className="insights-table">
          <thead>
            <tr>
              <th>ID</th>
              <th>Source</th>
              <th>Text</th>
              <th>Tags</th>
            </tr>
          </thead>
          <tbody>
            {insights.map((insight) => (
              <tr key={insight.id}>
                <td>{insight.id}</td>
                <td>{insight.source}</td>
                <td>{insight.text}</td>
                <td>{insight.enrichment?.tags?.join(", ") ?? ""}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </section>
  );
}
