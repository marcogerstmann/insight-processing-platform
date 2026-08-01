import { useState, type FormEvent } from "react";
import { useAuth } from "../auth/AuthContext.tsx";
import { createInsight, type CreateInsightResult } from "../api/insights.ts";

// Create insight form. Posts a single `text` field to
// POST /v1/tenants/{tenantID}/insights and reports whether a new row was stored.
// Only ever mounted while signed in (App.tsx gates this), so `token` here is
// always set.
export function CreateInsightSection() {
  const { token } = useAuth();
  const [text, setText] = useState("");
  const [result, setResult] = useState<CreateInsightResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  if (!token) return null;

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    if (!token) return;
    setError(null);
    setResult(null);

    // Mirror the backend's check (handler.go returns 400 "text is required") so
    // an empty submit is caught client-side without a round-trip.
    if (text.trim() === "") {
      setError("text is required");
      return;
    }

    setPending(true);
    try {
      const res = await createInsight(token, text);
      setResult(res);
      setText("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create insight");
    } finally {
      setPending(false);
    }
  }

  return (
    <section>
      <h2>Create Insight</h2>
      <form className="create-form" onSubmit={handleSubmit}>
        <label>
          Text
          <textarea
            value={text}
            onChange={(e) => setText(e.target.value)}
            rows={4}
          />
        </label>
        <button type="submit" disabled={pending}>
          {pending ? "Creating…" : "Create insight"}
        </button>
        {error && (
          <p className="error" role="alert">
            {error}
          </p>
        )}
      </form>

      {result && (
        <div className="create-result">
          <p>
            {result.inserted
              ? "Created (HTTP 201) — new insight stored."
              : "Already existed (HTTP 200) — no new row."}
          </p>
          <div className="table-wrap">
            <table className="insights-table">
              <tbody>
                <tr>
                  <th>ID</th>
                  <td>{result.insight.id}</td>
                </tr>
                <tr>
                  <th>Source</th>
                  <td>{result.insight.source}</td>
                </tr>
                <tr>
                  <th>Text</th>
                  <td>{result.insight.text}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <p className="placeholder">
            Switch to the <strong>Insights</strong> tab to see it in the list.
          </p>
        </div>
      )}
    </section>
  );
}
