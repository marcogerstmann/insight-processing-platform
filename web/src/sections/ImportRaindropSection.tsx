import { useState, type FormEvent } from "react";
import { useAuth } from "../auth/AuthContext.tsx";
import { importRaindropHighlights, type ImportResult } from "../api/raindrop.ts";

// Triggers POST /v1/raindrop/import. Leaving "limit" empty imports every
// highlight; the Raindrop token field overrides the server's configured
// RAINDROP_API_TOKEN for this call only (never stored). Re-running this is
// always safe — highlights already imported, via this or the scheduled poll,
// are skipped server-side (same idempotency key either way). No "only
// favorites" control: Raindrop has no favorites concept.
// Only ever mounted while signed in (App.tsx gates this), so `token` here is
// always set.
export function ImportRaindropSection() {
  const { token } = useAuth();
  const [raindropToken, setRaindropToken] = useState("");
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
      const res = await importRaindropHighlights(token, {
        raindropToken: raindropToken.trim(),
        limit: limit.trim() === "" ? undefined : Number(limit),
      });
      setResult(res);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to import Raindrop highlights");
    } finally {
      setPending(false);
    }
  }

  return (
    <section>
      <h2>Import Raindrop Highlights</h2>
      <form className="create-form" onSubmit={handleSubmit}>
        <label>
          Raindrop API token (optional)
          <input
            type="password"
            value={raindropToken}
            onChange={(e) => setRaindropToken(e.target.value)}
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
