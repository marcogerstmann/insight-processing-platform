import { apiRequest } from "./client.ts";

// Mirrors the backend ImportResponseDTO in
// internal/adapters/inbound/http/rest/raindrop/dto.go. Enqueued highlights are
// enriched asynchronously by the SQS worker, not by this request.
export interface ImportResult {
  fetched: number;
  enqueued: number;
}

// Trigger a Raindrop highlight import for the caller's tenant. `token`
// overrides the server-configured RAINDROP_API_TOKEN for this call only;
// `limit` <= 0 (or omitted) imports every highlight. No favorites option —
// Raindrop has no equivalent concept. Safe to re-run: highlights already
// imported (via this or the scheduled poll) are skipped server-side.
export async function importRaindropHighlights(
  authToken: string,
  options: { raindropToken?: string; limit?: number } = {},
): Promise<ImportResult> {
  return apiRequest<ImportResult>("/v1/raindrop/import", authToken, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      token: options.raindropToken || undefined,
      limit: options.limit || undefined,
    }),
  });
}
