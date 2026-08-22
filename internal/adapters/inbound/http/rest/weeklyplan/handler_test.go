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
}

func (f *fakeService) Submit(_ context.Context, plan domain.WeeklyPlan) error {
	f.submitCalled = true
	f.gotPlan = plan
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
