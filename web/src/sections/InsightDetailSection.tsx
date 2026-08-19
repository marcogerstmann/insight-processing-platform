import { useEffect, useState } from "react";
import { useAuth } from "../auth/AuthContext.tsx";
import { listRelationships, type RelatedInsight } from "../api/relationships.ts";
import type { Insight } from "../api/insights.ts";

interface InsightDetailSectionProps {
  insight: Insight;
  onNavigate: (related: RelatedInsight) => void;
  onBack: () => void;
}

// Human-readable label per relation type. Falls back to the raw string for
// anything not in this list, so a future backend enum value degrades
// instead of breaking.
const RELATION_LABELS: Record<string, string> = {
  supports: "Supports",
  contradicts: "Contradicts",
  extends: "Extends",
  example_of: "Example of",
  same_topic: "Same topic",
};

function relationLabel(type: string): string {
  return RELATION_LABELS[type] ?? type;
}

// Insight detail view (WEB 5/IPP-112): the insight's own text/notes/tags,
// plus its "Related insights" list (REL 6/IPP-102) — each showing relation
// type, confidence and the agent's rationale. No graph visualization
// library: clicking a related insight just re-renders this same component
// for that insight instead (see InsightsSection's navigation stack), which
// is enough to walk the graph one hop at a time while keeping the
// rationale text readable.
// Only ever mounted while signed in (InsightsSection gates this), so
// `token` here is always set.
export function InsightDetailSection({ insight, onNavigate, onBack }: InsightDetailSectionProps) {
  const { token } = useAuth();
  const [related, setRelated] = useState<RelatedInsight[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!token) return;

    let cancelled = false;
    setLoading(true);
    setError(null);
    setRelated(null);

    listRelationships(token, insight.id)
      .then((items) => {
        if (!cancelled) setRelated(items);
      })
      .catch((err) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "Failed to load relationships");
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [token, insight.id]);

  if (!token) return null;

  return (
    <section>
      <button type="button" className="tag-cloud-item" onClick={onBack}>
        ← Back
      </button>

      <div className="insight-detail">
        <p className="insight-detail-source">{insight.source}</p>
        <p className="insight-detail-text">{insight.text}</p>
        {insight.notes && <p className="insight-detail-notes">{insight.notes}</p>}
        {insight.enrichment && insight.enrichment.tags.length > 0 && (
          <p className="insight-detail-tags">{insight.enrichment.tags.join(", ")}</p>
        )}
      </div>

      <h3>Related insights</h3>
      {loading && <p>Loading…</p>}
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      {!loading && !error && related && related.length === 0 && (
        <p className="placeholder">No related insights yet.</p>
      )}
      {!loading && !error && related && related.length > 0 && (
        <ul className="related-insights-list">
          {related.map((r) => (
            <li key={r.insight_id} className="related-insight-item">
              <div className="related-insight-header">
                <span className={`relation-badge relation-badge--${r.type}`}>{relationLabel(r.type)}</span>
                <span className="related-insight-confidence">{Math.round(r.confidence * 100)}% confidence</span>
              </div>
              <button type="button" className="related-insight-link" onClick={() => onNavigate(r)}>
                {r.text}
              </button>
              <p className="related-insight-rationale">{r.rationale}</p>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
