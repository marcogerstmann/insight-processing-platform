import { decodeJwt } from "../auth/jwt.ts";
import { apiRequest } from "./client.ts";

// Mirrors the backend ResponseDTO in
// internal/adapters/inbound/http/rest/insight/dto.go. `enrichment` is optional:
// an insight the worker hasn't enriched yet has none.
export interface Enrichment {
  summary: string;
  tags: string[];
  key_question: string;
}

export interface Insight {
  id: string;
  source: string;
  text: string;
  enrichment?: Enrichment;
}

interface ListInsightsResponse {
  tenant_id: string;
  items: Insight[];
}

// Fetch the tenant's insights. The tenant ID comes from the token's
// custom:tenant_id claim — the same value the backend trusts — never from user
// input, so the path segment can't diverge from what the server authorizes.
export async function listInsights(token: string): Promise<Insight[]> {
  const tenantId = decodeJwt(token)["custom:tenant_id"];
  const body = await apiRequest<ListInsightsResponse>(
    `/v1/tenants/${tenantId}/insights`,
    token,
  );
  return body.items;
}

// Mirrors the backend CreateInsightResponseDTO. `inserted` is false when the
// insight already existed (HTTP 200) and true when a new row was stored (201).
export interface CreateInsightResult {
  inserted: boolean;
  insight: Insight;
}

// Create an insight for the caller's tenant. Matches CreateInsightRequestDTO
// (a single `text` field).
export async function createInsight(
  token: string,
  text: string,
): Promise<CreateInsightResult> {
  const tenantId = decodeJwt(token)["custom:tenant_id"];
  return apiRequest<CreateInsightResult>(`/v1/tenants/${tenantId}/insights`, token, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ text }),
  });
}
