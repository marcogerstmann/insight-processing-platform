import { apiRequest } from "./client.ts";
import { decodeJwt } from "../auth/jwt.ts";

// Mirrors ResponseDTO / PlanDetailDTO in
// internal/adapters/inbound/http/rest/weeklyplan/dto.go.
export interface CreatePlanResult {
  id: string;
  status: string;
}

export interface ResolvedInsight {
  insight_id: string;
  text: string;
}

export interface ResolvedAction {
  title: string;
  why: string;
  supporting_insights: ResolvedInsight[];
}

export interface PlanDetail {
  id: string;
  tag: string;
  focus_sentence: string;
  status: string;
  created_at: string;
  failure_reason?: string;
  actions: ResolvedAction[];
}

// Mirrors PlanListItemDTO — no actions, just enough to list and re-open.
export interface PlanListItem {
  id: string;
  tag: string;
  focus_sentence: string;
  status: string;
  created_at: string;
}

interface ListPlansResponse {
  items: PlanListItem[];
}

// Same "URL needs *a* tenantID but the handler never trusts it" shape as
// relationships.ts — see Handler.Create's doc comment in weeklyplan/handler.go.
function tenantPath(token: string): string {
  const tenantID = decodeJwt(token)["custom:tenant_id"];
  return `/v1/tenants/${encodeURIComponent(String(tenantID))}/weekly-plans`;
}

// POST /v1/tenants/:tenantID/weekly-plans — always returns 202 with status
// "pending"; the plan is generated async by the planning worker.
export async function createWeeklyPlan(
  token: string,
  tag: string,
  focusSentence: string,
): Promise<CreatePlanResult> {
  return apiRequest<CreatePlanResult>(tenantPath(token), token, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ tag, focus_sentence: focusSentence }),
  });
}

// GET .../weekly-plans/:planID — poll until status is "ready" or "failed".
export async function getWeeklyPlan(token: string, planID: string): Promise<PlanDetail> {
  return apiRequest<PlanDetail>(`${tenantPath(token)}/${encodeURIComponent(planID)}`, token);
}

// GET .../weekly-plans — every plan for the tenant, newest first (WEB
// 4/IPP-111), so past plans can be relisted and re-opened.
export async function listWeeklyPlans(token: string): Promise<PlanListItem[]> {
  const body = await apiRequest<ListPlansResponse>(tenantPath(token), token);
  return body.items;
}
