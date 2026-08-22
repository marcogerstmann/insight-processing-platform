import { useEffect, useState, type FormEvent } from "react";
import { useAuth } from "../auth/AuthContext.tsx";
import { listTags, type Tag } from "../api/tags.ts";
import {
  createWeeklyPlan,
  getWeeklyPlan,
  listWeeklyPlans,
  type PlanDetail,
  type PlanListItem,
} from "../api/weeklyPlans.ts";
import type { RelatedInsight } from "../api/relationships.ts";
import type { Insight } from "../api/insights.ts";
import { InsightDetailSection } from "./InsightDetailSection.tsx";

const POLL_INTERVAL_MS = 3000;
// The planning worker chains a few LLM calls (context gathering + action
// generation + citation validation); 2 minutes gives that room without
// spinning forever on a stuck plan (IPP-110's AC).
const POLL_TIMEOUT_MS = 2 * 60 * 1000;

// Weekly focus form + plan view (WEB 3/IPP-110, WEB 4/IPP-111): pick a tag,
// write one sentence, submit to POST /weekly-plans, then poll GET
// /weekly-plans/:id until the planning worker (PLAN 4/IPP-106) resolves it
// to ready or failed. Past plans (GET /weekly-plans) are listed below and
// re-openable the same way.
export function WeeklyPlanSection() {
  const { token } = useAuth();
  const [tags, setTags] = useState<Tag[]>([]);
  const [tag, setTag] = useState("");
  const [focusSentence, setFocusSentence] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [planId, setPlanId] = useState<string | null>(null);
  const [plan, setPlan] = useState<PlanDetail | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [timedOut, setTimedOut] = useState(false);

  const [pastPlans, setPastPlans] = useState<PlanListItem[] | null>(null);
  const [pastPlansError, setPastPlansError] = useState<string | null>(null);
  const [openError, setOpenError] = useState<string | null>(null);

  // Insight-detail navigation stack (WEB 5/IPP-112's pattern, reused here
  // per IPP-111's "clickable links into the insight detail" AC): opening a
  // supporting insight pushes a stub record — the plan only ever carries
  // insight_id/text, same shape InsightsSection falls back to for a related
  // insight it hasn't already loaded in full.
  const [viewStack, setViewStack] = useState<Insight[]>([]);

  function refreshPastPlans() {
    if (!token) return;
    listWeeklyPlans(token)
      .then((items) => setPastPlans(items))
      .catch((err) => {
        setPastPlansError(err instanceof Error ? err.message : "Failed to load past plans");
      });
  }

  useEffect(() => {
    if (!token) return;
    listTags(token)
      .then((items) => {
        setTags(items);
        setTag((current) => current || items[0]?.tag || "");
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : "Failed to load tags");
      });
    refreshPastPlans();
  }, [token]);

  // Polls while a plan is pending. setInterval in an effect, cleared on
  // ready/failed/timeout/unmount — never more than one interval alive per
  // planId change, per IPP-110's implementation notes.
  useEffect(() => {
    if (!token || !planId) return;

    const startedAt = Date.now();
    const interval = setInterval(() => {
      if (Date.now() - startedAt > POLL_TIMEOUT_MS) {
        clearInterval(interval);
        setTimedOut(true);
        setPlanId(null);
        return;
      }
      getWeeklyPlan(token, planId)
        .then((detail) => {
          if (detail.status !== "pending") {
            clearInterval(interval);
            setPlan(detail);
            setPlanId(null);
            refreshPastPlans();
          }
        })
        .catch((err) => {
          clearInterval(interval);
          setError(err instanceof Error ? err.message : "Failed to check plan status");
          setPlanId(null);
        });
    }, POLL_INTERVAL_MS);

    return () => clearInterval(interval);
  }, [token, planId]);

  if (!token) return null;

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    if (!token || !tag || focusSentence.trim() === "") return;

    setError(null);
    setOpenError(null);
    setPlan(null);
    setTimedOut(false);
    setSubmitting(true);
    try {
      const result = await createWeeklyPlan(token, tag, focusSentence);
      setPlanId(result.id);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to submit weekly plan");
    } finally {
      setSubmitting(false);
    }
  }

  const pending = submitting || planId !== null;

  function openPastPlan(id: string) {
    if (!token || pending) return;
    setOpenError(null);
    setPlan(null);
    setTimedOut(false);
    getWeeklyPlan(token, id)
      .then((detail) => setPlan(detail))
      .catch((err) => {
        setOpenError(err instanceof Error ? err.message : "Failed to open plan");
      });
  }

  function openInsight(insightID: string, text: string) {
    setViewStack((s) => [...s, { id: insightID, source: "", text }]);
  }

  function navigateToRelated(related: RelatedInsight) {
    openInsight(related.insight_id, related.text);
  }

  function goBackFromInsight() {
    setViewStack((s) => s.slice(0, -1));
  }

  if (viewStack.length > 0) {
    const current = viewStack[viewStack.length - 1];
    return <InsightDetailSection insight={current} onNavigate={navigateToRelated} onBack={goBackFromInsight} />;
  }

  return (
    <section>
      <h2>Weekly Plan</h2>
      <form className="create-form" onSubmit={handleSubmit}>
        <label>
          Tag
          <select value={tag} onChange={(e) => setTag(e.target.value)} disabled={pending}>
            {tags.length === 0 && <option value="">No tags yet</option>}
            {tags.map((t) => (
              <option key={t.tag} value={t.tag}>
                {t.tag}
              </option>
            ))}
          </select>
        </label>
        <label>
          Focus
          <input
            type="text"
            value={focusSentence}
            onChange={(e) => setFocusSentence(e.target.value)}
            placeholder="This week I want to become better at delegation."
            disabled={pending}
          />
        </label>
        <button type="submit" disabled={pending || !tag}>
          {pending ? "Working…" : "Submit"}
        </button>
        {error && (
          <p className="error" role="alert">
            {error}
          </p>
        )}
      </form>

      {planId && (
        <p className="placeholder">
          An agent is drafting your plan — this usually takes a minute or two.
        </p>
      )}

      {timedOut && (
        <p className="error" role="alert">
          Still not ready after {POLL_TIMEOUT_MS / 1000}s. Check the Weekly Plan again later.
        </p>
      )}

      {openError && (
        <p className="error" role="alert">
          {openError}
        </p>
      )}

      {plan && plan.status === "failed" && (
        <p className="error" role="alert">
          Plan failed: {plan.failure_reason}
        </p>
      )}

      {plan && plan.status === "ready" && (
        <>
          <dl className="tag-score-breakdown">
            <dt>Tag</dt>
            <dd>{plan.tag}</dd>
            <dt>Focus</dt>
            <dd>{plan.focus_sentence}</dd>
          </dl>
          <ul className="related-insights-list">
            {plan.actions.map((action, i) => (
              <li key={i} className="related-insight-item">
                <h4>{action.title}</h4>
                <p className="plan-action-why">{action.why}</p>
                {action.supporting_insights.length > 0 && (
                  <p className="plan-action-citations">
                    Supporting:{" "}
                    {action.supporting_insights.map((si, j) => (
                      <span key={si.insight_id}>
                        {j > 0 && ", "}
                        <button
                          type="button"
                          className="tag-cloud-item"
                          onClick={() => openInsight(si.insight_id, si.text)}
                        >
                          {si.text}
                        </button>
                      </span>
                    ))}
                  </p>
                )}
              </li>
            ))}
          </ul>
        </>
      )}

      <h3>Past plans</h3>
      {pastPlansError && (
        <p className="error" role="alert">
          {pastPlansError}
        </p>
      )}
      {pastPlans && pastPlans.length === 0 && <p className="placeholder">No plans yet.</p>}
      {pastPlans && pastPlans.length > 0 && (
        <div className="table-wrap">
          <table className="insights-table">
            <thead>
              <tr>
                <th>Tag</th>
                <th>Focus</th>
                <th>Status</th>
                <th>Created</th>
              </tr>
            </thead>
            <tbody>
              {pastPlans.map((p) => (
                <tr key={p.id}>
                  <td>{p.tag}</td>
                  <td>
                    <button
                      type="button"
                      className="tag-cloud-item"
                      disabled={pending}
                      onClick={() => openPastPlan(p.id)}
                    >
                      {p.focus_sentence}
                    </button>
                  </td>
                  <td>{p.status}</td>
                  <td>{new Date(p.created_at).toLocaleString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}
