import { useEffect, useState, type FormEvent } from "react";
import { useAuth } from "../auth/AuthContext.tsx";
import { listTags, type Tag } from "../api/tags.ts";
import { createWeeklyPlan, getWeeklyPlan, type PlanDetail } from "../api/weeklyPlans.ts";

const POLL_INTERVAL_MS = 3000;
// The planning worker chains a few LLM calls (context gathering + action
// generation + citation validation); 2 minutes gives that room without
// spinning forever on a stuck plan (IPP-110's AC).
const POLL_TIMEOUT_MS = 2 * 60 * 1000;

// Weekly focus form (WEB 3/IPP-110): pick a tag, write one sentence, submit
// to POST /weekly-plans, then poll GET /weekly-plans/:id until the planning
// worker (PLAN 4/IPP-106) resolves it to ready or failed.
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
  }, [token]);

  // Polls while a plan is pending. setInterval in an effect, cleared on
  // ready/failed/timeout/unmount — never more than one interval alive per
  // planId change, per the ticket's implementation notes.
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

      {plan && plan.status === "failed" && (
        <p className="error" role="alert">
          Plan failed: {plan.failure_reason}
        </p>
      )}

      {plan && plan.status === "ready" && (
        <ul className="related-insights-list">
          {plan.actions.map((action, i) => (
            <li key={i} className="related-insight-item">
              <h4>{action.title}</h4>
              <p className="related-insight-rationale">{action.why}</p>
              {action.supporting_insights.length > 0 && (
                <p className="related-insight-confidence">
                  Supporting: {action.supporting_insights.map((si) => si.text).join("; ")}
                </p>
              )}
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
