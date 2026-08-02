import { decodeJwt } from "../auth/jwt.ts";
import { apiRequest } from "./client.ts";

// Mirrors the backend ImportResponseDTO in
// internal/adapters/inbound/http/rest/readwise/dto.go. Enqueued highlights are
// enriched asynchronously by the SQS worker, not by this request.
export interface ImportResult {
  fetched: number;
  enqueued: number;
}

// Trigger a Readwise highlight import for the caller's tenant. `token`
// overrides the server-configured READWISE_API_TOKEN for this call only;
// `limit` <= 0 (or omitted) imports every highlight, otherwise only the most
// recently highlighted `limit` of them. Safe to re-run: highlights already
// imported (via this or the Readwise webhook) are skipped server-side.
export async function importReadwiseHighlights(
  authToken: string,
  options: { readwiseToken?: string; limit?: number } = {},
): Promise<ImportResult> {
  const tenantId = decodeJwt(authToken)["custom:tenant_id"];
  return apiRequest<ImportResult>(`/v1/tenants/${tenantId}/readwise/import`, authToken, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      token: options.readwiseToken || undefined,
      limit: options.limit || undefined,
    }),
  });
}
