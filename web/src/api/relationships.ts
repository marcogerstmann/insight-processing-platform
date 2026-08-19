import { apiRequest } from "./client.ts";
import { decodeJwt } from "../auth/jwt.ts";

// Mirrors RelatedInsightDTO / ListRelationshipsResponseDTO in
// internal/adapters/inbound/http/rest/relationship/dto.go. `text` is the
// related insight's text, denormalized onto the edge server-side so this
// list needs no per-item fetch (REL 6/IPP-102).
export interface RelatedInsight {
  insight_id: string;
  text: string;
  type: string;
  confidence: number;
  rationale: string;
}

interface ListRelationshipsResponse {
  insight_id: string;
  items: RelatedInsight[];
}

// GET /v1/tenants/:tenantID/insights/:insightID/relationships, sorted by
// confidence descending (server-side). The URL's :tenantID segment is never
// trusted by the handler — it always scopes to the JWT's own tenant claim —
// but the route still needs *a* value there, so we fill it from that same
// claim rather than inventing a placeholder.
export async function listRelationships(token: string, insightID: string): Promise<RelatedInsight[]> {
  const tenantID = decodeJwt(token)["custom:tenant_id"];
  const body = await apiRequest<ListRelationshipsResponse>(
    `/v1/tenants/${encodeURIComponent(String(tenantID))}/insights/${encodeURIComponent(insightID)}/relationships`,
    token,
  );
  return body.items;
}
