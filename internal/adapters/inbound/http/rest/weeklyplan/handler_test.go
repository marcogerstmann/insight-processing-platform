package weeklyplan

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/marcogerstmann/insight-processing-platform/internal/adapters/inbound/http/rest/auth"
	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
	"github.com/marcogerstmann/insight-processing-platform/internal/ports"
)

type fakeService struct {
	submitCalled bool
	gotPlan      domain.WeeklyPlan
	err          error

	getDetail       domain.PlanDetail
	getCalledTenant string
	getCalledPlan   string

	listPlans      []domain.WeeklyPlan
	listCalledWith string

	setReadyCalled  bool
	setReadyActions []domain.Action

	setFailedCalled bool
	setFailedReason string
}

func (f *fakeService) Submit(_ context.Context, plan domain.WeeklyPlan) error {
	f.submitCalled = true
	f.gotPlan = plan
	return f.err
}

func (f *fakeService) Get(_ context.Context, tenantID, planID string) (domain.PlanDetail, error) {
	f.getCalledTenant = tenantID
	f.getCalledPlan = planID
	return f.getDetail, f.err
}

func (f *fakeService) List(_ context.Context, tenantID string) ([]domain.WeeklyPlan, error) {
	f.listCalledWith = tenantID
	return f.listPlans, f.err
}

func (f *fakeService) SetReady(_ context.Context, _, _ string, actions []domain.Action) error {
	f.setReadyCalled = true
	f.setReadyActions = actions
	return f.err
}

func (f *fakeService) SetFailed(_ context.Context, _, _, reason string) error {
	f.setFailedCalled = true
	f.setFailedReason = reason
	return f.err
}

func doCreateRequest(h *Handler, tenantID string, body CreateWeeklyPlanRequestDTO) (*httptest.ResponseRecorder, map[string]any) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	raw, _ := json.Marshal(body)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/tenants/"+tenantID+"/weekly-plans", bytes.NewReader(raw))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "tenantID", Value: tenantID}}
	c.Set(auth.TenantIDKey, tenantID)

	h.Create(c)

	var respBody map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &respBody)
	return rec, respBody
}

func TestHandler_Create_HappyPath_Returns202AndSubmits(t *testing.T) {
	svc := &fakeService{}
	h := NewHandler(svc)

	rec, body := doCreateRequest(h, "t-1", CreateWeeklyPlanRequestDTO{
		Tag:           "golang",
		FocusSentence: "Read more about distributed systems this week.",
	})

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d, body=%v", rec.Code, http.StatusAccepted, body)
	}
	if !svc.submitCalled {
		t.Fatalf("expected svc.Submit to be called")
	}
	if svc.gotPlan.TenantID != "t-1" || svc.gotPlan.Tag != "golang" {
		t.Fatalf("gotPlan = %+v, want tenant=t-1 tag=golang", svc.gotPlan)
	}
	if svc.gotPlan.Status != domain.PlanStatusPending {
		t.Fatalf("gotPlan.Status = %q, want pending", svc.gotPlan.Status)
	}
	if body["id"] == "" || body["status"] != "pending" {
		t.Fatalf("response body = %v, want non-empty id and status=pending", body)
	}
}

func TestHandler_Create_UnknownTag_MapsToBadRequest(t *testing.T) {
	svc := &fakeService{err: ports.ErrUnknownTag}
	h := NewHandler(svc)

	rec, _ := doCreateRequest(h, "t-1", CreateWeeklyPlanRequestDTO{
		Tag:           "nonexistent",
		FocusSentence: "Read more about distributed systems this week.",
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandler_Create_EmptyFocusSentence_Rejected400_NeverReachesService(t *testing.T) {
	svc := &fakeService{}
	h := NewHandler(svc)

	rec, _ := doCreateRequest(h, "t-1", CreateWeeklyPlanRequestDTO{
		Tag:           "golang",
		FocusSentence: "   ",
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if svc.submitCalled {
		t.Fatalf("expected svc.Submit not to be called for an empty focus sentence")
	}
}

func TestHandler_Create_OversizedFocusSentence_Rejected400_NeverReachesService(t *testing.T) {
	svc := &fakeService{}
	h := NewHandler(svc)

	rec, _ := doCreateRequest(h, "t-1", CreateWeeklyPlanRequestDTO{
		Tag:           "golang",
		FocusSentence: strings.Repeat("a", 281),
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if svc.submitCalled {
		t.Fatalf("expected svc.Submit not to be called for an oversized focus sentence")
	}
}

func doSubmitResultRequest(h *Handler, tenantID, planID string, body SubmitPlanResultRequestDTO) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	raw, _ := json.Marshal(body)
	c.Request = httptest.NewRequest(http.MethodPut, "/v1/tenants/"+tenantID+"/weekly-plans/"+planID+"/result", bytes.NewReader(raw))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "tenantID", Value: tenantID}, {Key: "planID", Value: planID}}

	h.SubmitResult(c)
	return rec
}

func TestHandler_SubmitResult_Ready_PersistsActionsAndReturns204(t *testing.T) {
	svc := &fakeService{}
	h := NewHandler(svc)

	rec := doSubmitResultRequest(h, "t-1", "p-1", SubmitPlanResultRequestDTO{
		Status: "ready",
		Actions: []ActionDTO{
			{Title: "Ship it", Why: "why", SupportingInsightIDs: []string{"i-1"}},
		},
	})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if !svc.setReadyCalled {
		t.Fatalf("expected svc.SetReady to be called")
	}
	if len(svc.setReadyActions) != 1 || svc.setReadyActions[0].Title != "Ship it" {
		t.Fatalf("setReadyActions = %+v, want one action titled Ship it", svc.setReadyActions)
	}
	if svc.setFailedCalled {
		t.Fatalf("expected svc.SetFailed not to be called for a ready result")
	}
}

func TestHandler_SubmitResult_Failed_PersistsReasonAndReturns204(t *testing.T) {
	svc := &fakeService{}
	h := NewHandler(svc)

	rec := doSubmitResultRequest(h, "t-1", "p-1", SubmitPlanResultRequestDTO{
		Status:        "failed",
		FailureReason: "llm timed out after 3 attempts",
	})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if !svc.setFailedCalled || svc.setFailedReason != "llm timed out after 3 attempts" {
		t.Fatalf("setFailedCalled=%v setFailedReason=%q", svc.setFailedCalled, svc.setFailedReason)
	}
	if svc.setReadyCalled {
		t.Fatalf("expected svc.SetReady not to be called for a failed result")
	}
}

func TestHandler_SubmitResult_Failed_EmptyReason_Rejected400_NeverReachesService(t *testing.T) {
	svc := &fakeService{}
	h := NewHandler(svc)

	rec := doSubmitResultRequest(h, "t-1", "p-1", SubmitPlanResultRequestDTO{Status: "failed", FailureReason: "  "})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if svc.setFailedCalled {
		t.Fatalf("expected svc.SetFailed not to be called for an empty reason")
	}
}

func TestHandler_SubmitResult_UnknownStatus_Rejected400_NeverReachesService(t *testing.T) {
	svc := &fakeService{}
	h := NewHandler(svc)

	rec := doSubmitResultRequest(h, "t-1", "p-1", SubmitPlanResultRequestDTO{Status: "done"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if svc.setReadyCalled || svc.setFailedCalled {
		t.Fatalf("expected neither SetReady nor SetFailed to be called for an unknown status")
	}
}

func TestHandler_SubmitResult_NotPending_MapsToConflict(t *testing.T) {
	svc := &fakeService{err: ports.ErrPlanNotPending}
	h := NewHandler(svc)

	rec := doSubmitResultRequest(h, "t-1", "p-1", SubmitPlanResultRequestDTO{Status: "ready"})

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func doGetRequest(h *Handler, jwtTenantID, urlTenantID, planID string) (*httptest.ResponseRecorder, PlanDetailDTO) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	c.Request = httptest.NewRequest(http.MethodGet, "/v1/tenants/"+urlTenantID+"/weekly-plans/"+planID, nil)
	c.Params = gin.Params{{Key: "tenantID", Value: urlTenantID}, {Key: "planID", Value: planID}}
	c.Set(auth.TenantIDKey, jwtTenantID)

	h.Get(c)

	var body PlanDetailDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec, body
}

func TestHandler_Get_HappyPath_ReturnsResolvedPlan(t *testing.T) {
	svc := &fakeService{getDetail: domain.PlanDetail{
		Plan: domain.WeeklyPlan{ID: "p-1", Tag: "golang", Status: domain.PlanStatusReady},
		Actions: []domain.ResolvedAction{
			{
				Title: "Ship it",
				Why:   "why",
				SupportingInsights: []domain.ResolvedInsight{
					{InsightID: "i-1", Text: "hello world"},
				},
			},
		},
	}}
	h := NewHandler(svc)

	rec, body := doGetRequest(h, "t-1", "t-1", "p-1")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if body.ID != "p-1" || body.Status != "ready" {
		t.Fatalf("body = %+v, want id=p-1 status=ready", body)
	}
	if len(body.Actions) != 1 || len(body.Actions[0].SupportingInsights) != 1 ||
		body.Actions[0].SupportingInsights[0].Text != "hello world" {
		t.Fatalf("body.Actions = %+v, want one action with one resolved insight", body.Actions)
	}
}

func TestHandler_Get_UsesJWTTenant_NeverTheURLPathParam(t *testing.T) {
	svc := &fakeService{}
	h := NewHandler(svc)

	doGetRequest(h, "t-1", "t-attacker", "p-1")

	if svc.getCalledTenant != "t-1" {
		t.Fatalf("svc queried tenant %q, want the JWT tenant t-1 (not the URL's t-attacker)", svc.getCalledTenant)
	}
}

func TestHandler_Get_UnknownPlan_MapsToNotFound(t *testing.T) {
	svc := &fakeService{err: ports.ErrPlanNotFound}
	h := NewHandler(svc)

	rec, _ := doGetRequest(h, "t-1", "t-1", "p-missing")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func doListRequest(h *Handler, jwtTenantID, urlTenantID string) (*httptest.ResponseRecorder, ListPlansResponseDTO) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	c.Request = httptest.NewRequest(http.MethodGet, "/v1/tenants/"+urlTenantID+"/weekly-plans", nil)
	c.Params = gin.Params{{Key: "tenantID", Value: urlTenantID}}
	c.Set(auth.TenantIDKey, jwtTenantID)

	h.List(c)

	var body ListPlansResponseDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec, body
}

func TestHandler_List_HappyPath_ReturnsMappedItems(t *testing.T) {
	svc := &fakeService{listPlans: []domain.WeeklyPlan{
		{ID: "p-2", Tag: "golang", Status: domain.PlanStatusReady},
		{ID: "p-1", Tag: "golang", Status: domain.PlanStatusPending},
	}}
	h := NewHandler(svc)

	rec, body := doListRequest(h, "t-1", "t-1")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(body.Items) != 2 || body.Items[0].ID != "p-2" || body.Items[1].ID != "p-1" {
		t.Fatalf("body.Items = %+v, want [p-2, p-1] in the order the service returned them", body.Items)
	}
}

func TestHandler_List_UsesJWTTenant_NeverTheURLPathParam(t *testing.T) {
	svc := &fakeService{}
	h := NewHandler(svc)

	doListRequest(h, "t-1", "t-attacker")

	if svc.listCalledWith != "t-1" {
		t.Fatalf("svc queried tenant %q, want the JWT tenant t-1 (not the URL's t-attacker)", svc.listCalledWith)
	}
}
