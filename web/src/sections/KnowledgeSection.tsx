import { useEffect, useState } from "react";
import { useAuth } from "../auth/AuthContext.tsx";
import { listTags, type Tag } from "../api/tags.ts";
import { InsightsSection } from "./InsightsSection.tsx";

// Tag cloud sized by relevance `score`, not `insight_count` — that's the
// point of the underlying ranking (see tags.ts). No charting library: font
// size is a linear scale, but over each tag's position between the lowest
// and highest score *in the current set* rather than the theoretical [0,1]
// range — real scores cluster well under 1.0 (a score of 1.0 needs max
// count, recency and freshness all at once), so scaling against the
// absolute range left every tag near the minimum size.
const MIN_FONT_REM = 0.85;
const MAX_FONT_REM = 2.2;

function fontSizeFor(score: number, minScore: number, maxScore: number): string {
  const spread = maxScore - minScore;
  const t = spread === 0 ? 1 : (score - minScore) / spread;
  return `${MIN_FONT_REM + t * (MAX_FONT_REM - MIN_FONT_REM)}rem`;
}

// Same load-on-mount / loading-error-empty pattern as InsightsSection.
export function KnowledgeSection() {
  const { token } = useAuth();
  const [tags, setTags] = useState<Tag[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [selected, setSelected] = useState<Tag | null>(null);

  useEffect(() => {
    if (!token) return;

    let cancelled = false;
    setLoading(true);
    setError(null);
    setTags(null);
    setSelected(null);

    listTags(token)
      .then((items) => {
        if (!cancelled) setTags(items);
      })
      .catch((err) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "Failed to load tags");
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [token]);

  if (!token) return null;

  // IPP-109: selecting a tag drills into its insights instead of just showing
  // the score breakdown. Tags stay in state, so going back doesn't refetch.
  if (selected) {
    return (
      <section>
        <h2>Knowledge</h2>
        <button type="button" className="tag-cloud-item" onClick={() => setSelected(null)}>
          ← Back to tag cloud
        </button>
        <dl className="tag-score-breakdown">
          <dt>Tag</dt>
          <dd>{selected.tag}</dd>
          <dt>Insight count</dt>
          <dd>{selected.insight_count}</dd>
          <dt>Score</dt>
          <dd>{selected.score.toFixed(3)}</dd>
          <dt>Count component</dt>
          <dd>{selected.score_components.count.toFixed(3)}</dd>
          <dt>Recency component</dt>
          <dd>{selected.score_components.recency.toFixed(3)}</dd>
          <dt>Freshness component</dt>
          <dd>{selected.score_components.freshness.toFixed(3)}</dd>
        </dl>
        <InsightsSection tag={selected.tag} />
      </section>
    );
  }

  const scores = tags?.map((t) => t.score) ?? [];
  const minScore = scores.length ? Math.min(...scores) : 0;
  const maxScore = scores.length ? Math.max(...scores) : 0;

  return (
    <section>
      <h2>Knowledge</h2>
      {loading && <p>Loading…</p>}
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      {!loading && !error && tags && tags.length === 0 && (
        <p className="placeholder">No tags yet for this tenant.</p>
      )}
      {!loading && !error && tags && tags.length > 0 && (
        <div className="tag-cloud">
          {tags.map((tag) => (
            <button
              key={tag.tag}
              type="button"
              className="tag-cloud-item"
              style={{ fontSize: fontSizeFor(tag.score, minScore, maxScore) }}
              title={`score ${tag.score.toFixed(2)} — count ${tag.score_components.count.toFixed(2)}, recency ${tag.score_components.recency.toFixed(2)}, freshness ${tag.score_components.freshness.toFixed(2)}`}
              onClick={() => setSelected(tag)}
            >
              {tag.tag}
            </button>
          ))}
        </div>
      )}
    </section>
  );
}
