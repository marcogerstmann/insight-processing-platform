import { apiRequest } from "./client.ts";

// Mirrors TagResponseDTO in
// internal/adapters/inbound/http/rest/insight/dto.go. `score` ranks the tag
// by recency/freshness of usage, not just how many insights carry it — see
// TagRelevanceScore in internal/domain/tag_relevance.go. `score_components`
// only has count/recency/freshness today; relationship density (REL 5)
// isn't computed yet.
export interface TagScoreComponents {
  count: number;
  recency: number;
  freshness: number;
}

export interface Tag {
  tag: string;
  insight_count: number;
  last_insight_at: string;
  score: number;
  score_components: TagScoreComponents;
}

interface ListTagsResponse {
  tenant_id: string;
  items: Tag[];
}

// Fetch the tenant's tags, ranked by relevance score.
export async function listTags(token: string): Promise<Tag[]> {
  const body = await apiRequest<ListTagsResponse>("/v1/tags", token);
  return body.items;
}
