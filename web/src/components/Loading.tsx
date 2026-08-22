// Shared loading indicator (spinner + text) so every section's fetch state
// reads the same instead of a bare "Loading…" paragraph.
export function Loading() {
  return (
    <p className="loading" role="status">
      <span className="loading-spinner" aria-hidden="true" />
      Loading…
    </p>
  );
}
