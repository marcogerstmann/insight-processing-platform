package insight

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/marcogerstmann/insight-processing-platform/internal/adapters/inbound/http/rest/auth"
	appinsight "github.com/marcogerstmann/insight-processing-platform/internal/application/insight"
	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
)

// fakeService is a test double for appinsight.Service, only ListByTenantID is
// exercised by the handler tests in this file.
type fakeService struct {
	gotTag        string
	listCalled    bool
	returnInsight []domain.Insight
}

func (f *fakeService) Process(_ context.Context, _ domain.Insight) (appinsight.Result, error) {
	return appinsight.Result{}, nil
}

func (f *fakeService) ListByTenantID(_ context.Context, _, tag string) ([]domain.Insight, error) {
	f.listCalled = true
	f.gotTag = tag
	return f.returnInsight, nil
}

func (f *fakeService) ListTags(_ context.Context, _ string) ([]domain.TagSummary, error) {
	return nil, nil
}

func doListRequest(h *Handler, rawQuery string) (*httptest.ResponseRecorder, ListInsightsResponseDTO) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/insights?"+rawQuery, nil)
	c.Set(auth.TenantIDKey, "t-1")

	h.ListByTenantID(c)

	var body ListInsightsResponseDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec, body
}

func TestHandler_ListByTenantID_NoFilter_PassesEmptyTagToService(t *testing.T) {
	svc := &fakeService{returnInsight: []domain.Insight{{ID: "i-1"}, {ID: "i-2"}}}
	h := NewHandler(svc)

	rec, body := doListRequest(h, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if svc.gotTag != "" {
		t.Fatalf("expected empty tag passed to service, got %q", svc.gotTag)
	}
	if len(body.Items) != 2 {
		t.Fatalf("expected 2 items, got %v", body.Items)
	}
}

func TestHandler_ListByTenantID_WithTag_PassesTagToService_ReturnsMatches(t *testing.T) {
	svc := &fakeService{returnInsight: []domain.Insight{{ID: "i-1", Text: "hello"}}}
	h := NewHandler(svc)

	rec, body := doListRequest(h, "tag=delegation")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if svc.gotTag != "delegation" {
		t.Fatalf("expected tag=delegation passed to service, got %q", svc.gotTag)
	}
	if len(body.Items) != 1 || body.Items[0].ID != "i-1" {
		t.Fatalf("expected single insight i-1, got %v", body.Items)
	}
}

func TestHandler_ListByTenantID_UnknownTag_ReturnsEmptyList(t *testing.T) {
	svc := &fakeService{returnInsight: []domain.Insight{}}
	h := NewHandler(svc)

	rec, body := doListRequest(h, "tag=nonexistent")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(body.Items) != 0 {
		t.Fatalf("expected empty items, got %v", body.Items)
	}
}

func TestHandler_ListByTenantID_DenormalizedTag_ForwardsRawValueToService(t *testing.T) {
	// The handler itself must not normalize - that's the service's job (see
	// service_test.go), so it can reuse the same NormalizeTag used at write
	// time. Here we only assert the raw query value is passed through as-is.
	svc := &fakeService{returnInsight: []domain.Insight{{ID: "i-1"}}}
	h := NewHandler(svc)

	rec, body := doListRequest(h, "tag=Delegation")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if svc.gotTag != "Delegation" {
		t.Fatalf("expected raw tag %q forwarded to service, got %q", "Delegation", svc.gotTag)
	}
	if len(body.Items) != 1 {
		t.Fatalf("expected 1 item, got %v", body.Items)
	}
}

func TestHandler_ListByTenantID_ResponseShapeIdentical_WithAndWithoutFilter(t *testing.T) {
	svc := &fakeService{returnInsight: []domain.Insight{{ID: "i-1", Text: "hello"}}}
	h := NewHandler(svc)

	_, withoutFilter := doListRequest(h, "")
	_, withFilter := doListRequest(h, "tag=a")

	if withoutFilter.TenantID != withFilter.TenantID {
		t.Fatalf("tenant_id differs: %q vs %q", withoutFilter.TenantID, withFilter.TenantID)
	}
	if len(withoutFilter.Items) != len(withFilter.Items) || withoutFilter.Items[0] != withFilter.Items[0] {
		t.Fatalf("item shape differs: %+v vs %+v", withoutFilter.Items, withFilter.Items)
	}
}
