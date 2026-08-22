import { useEffect, useState } from "react";
import { useAuth } from "../auth/AuthContext.tsx";
import { listInsights, type Insight } from "../api/insights.ts";
import type { RelatedInsight } from "../api/relationships.ts";
import { InsightDetailSection } from "./InsightDetailSection.tsx";
import { Loading } from "../components/Loading.tsx";

interface InsightsSectionProps {
  // Optional tag filter (IPP-109 drill-down). Undefined shows every insight,
  // same as before that ticket.
  tag?: string;
}

// Insights list. On each mount (i.e. every time the section is opened, or the
// tag filter changes) it calls GET /v1/insights and renders the result as a
// plain table. Clicking an insight's text opens its detail view (WEB
// 5/IPP-112), which can itself navigate to a related insight — see
// viewStack below.
// Only ever mounted while signed in (App.tsx gates this), so `token` here is
// always set.
export function InsightsSection({ tag }: InsightsSectionProps = {}) {
  const { token } = useAuth();
  const [insights, setInsights] = useState<Insight[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  // Detail-view navigation stack (WEB 5/IPP-112): pushing lets a related
  // insight open its own detail in turn ("graph walking"), popping is Back.
  // Empty stack = showing the list below.
  const [viewStack, setViewStack] = useState<Insight[]>([]);

  useEffect(() => {
    if (!token) return;

    // Guard against a state update after unmount if the user navigates away
    // mid-request.
    let cancelled = false;
    setLoading(true);
    setError(null);
    setInsights(null);
    setViewStack([]);

    listInsights(token, tag)
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
  }, [token, tag]);

  if (!token) return null;

  function openDetail(insight: Insight) {
    setViewStack((s) => [...s, insight]);
  }

  // The related-insight stub only carries id/text (denormalized server-side,
  // see relationships.ts) — if the full record is already loaded here (the
  // common case), prefer it so tags/notes still show up in its detail view.
  function navigateToRelated(related: RelatedInsight) {
    const known = insights?.find((i) => i.id === related.insight_id);
    openDetail(known ?? { id: related.insight_id, source: "", text: related.text });
  }

  function goBack() {
    setViewStack((s) => s.slice(0, -1));
  }

  if (viewStack.length > 0) {
    const current = viewStack[viewStack.length - 1];
    const previous = viewStack.length > 1 ? viewStack[viewStack.length - 2] : undefined;
    return (
      <InsightDetailSection
        insight={current}
        onNavigate={navigateToRelated}
        onBack={goBack}
        onHome={() => setViewStack([])}
        rootLabel={tag ? `Insights tagged "${tag}"` : "Insights"}
        previousLabel={previous?.text}
      />
    );
  }

  return (
    <section>
      <h2>{tag ? `Insights tagged "${tag}"` : "Insights"}</h2>
      {loading && <Loading />}
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      {!loading && !error && insights && insights.length === 0 && (
        <p className="placeholder">
          {tag ? `No insights tagged "${tag}".` : "No insights yet for this tenant."}
        </p>
      )}
      {!loading && !error && insights && insights.length > 0 && (
        <div className="table-wrap">
          <table className="insights-table">
            <thead>
              <tr>
                <th>Source</th>
                <th>Text</th>
                <th>Tags</th>
              </tr>
            </thead>
            <tbody>
              {insights.map((insight) => (
                <tr key={insight.id}>
                  <td>{insight.source}</td>
                  <td>
                    <button type="button" className="tag-cloud-item" onClick={() => openDetail(insight)}>
                      {insight.text}
                    </button>
                  </td>
                  <td>{insight.enrichment?.tags?.join(", ") ?? ""}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}
