import { useState, type FormEvent } from "react";
import { useAuth } from "../auth/AuthContext.tsx";
import { importReadwiseHighlights, type ImportResult } from "../api/readwise.ts";

// Triggers POST /v1/tenants/{tenantID}/readwise/import. Leaving "limit" empty
// imports every highlight; the Readwise token field overrides the server's
// configured READWISE_API_TOKEN for this call only (never stored). Re-running
// this is always safe — highlights already imported, via this or the Readwise
// webhook, are skipped server-side (same idempotency key either way).
// Only ever mounted while signed in (App.tsx gates this), so `token` here is
// always set.
export function ImportReadwiseSection() {
  const { token } = useAuth();
  const [readwiseToken, setReadwiseToken] = useState("");
  const [limit, setLimit] = useState("");
  const [result, setResult] = useState<ImportResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  if (!token) return null;

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    if (!token) return;
    setError(null);
    setResult(null);
    setPending(true);
    try {
      const res = await importReadwiseHighlights(token, {
        readwiseToken: readwiseToken.trim(),
        limit: limit.trim() === "" ? undefined : Number(limit),
      });
      setResult(res);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to import Readwise highlights");
    } finally {
      setPending(false);
    }
  }

  return (
    <section>
      <h2>Import Readwise Highlights</h2>
      <form className="create-form" onSubmit={handleSubmit}>
        <label>
          Readwise API token (optional)
          <input
            type="password"
            value={readwiseToken}
            onChange={(e) => setReadwiseToken(e.target.value)}
            placeholder="Uses the server-configured token if left blank"
            autoComplete="off"
          />
        </label>
        <label>
          Limit (optional)
          <input
            type="number"
            min={1}
            value={limit}
            onChange={(e) => setLimit(e.target.value)}
            placeholder="Leave empty to import all highlights"
          />
        </label>
        <button type="submit" disabled={pending}>
          {pending ? "Importing…" : "Import highlights"}
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
            Fetched {result.fetched} highlight{result.fetched === 1 ? "" : "s"}, enqueued{" "}
            {result.enqueued} for processing.
          </p>
          <p className="placeholder">
            Enrichment runs asynchronously — check the <strong>Insights</strong> tab shortly.
          </p>
        </div>
      )}
    </section>
  );
}
