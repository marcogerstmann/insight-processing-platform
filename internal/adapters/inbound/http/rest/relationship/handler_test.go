package relationship

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/marcogerstmann/insight-processing-platform/internal/adapters/inbound/http/rest/auth"
	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
	"github.com/marcogerstmann/insight-processing-platform/internal/ports"
)

type fakeRepo struct {
	putCalled     bool
	gotRel        domain.Relationship
	err           error
	listRelated   []domain.RelatedInsight
	listTenantID  string
	listInsightID string
}

func (f *fakeRepo) Put(_ context.Context, rel domain.Relationship) error {
	f.putCalled = true
	f.gotRel = rel
	return f.err
}

func (f *fakeRepo) ListByInsightID(_ context.Context, tenantID, insightID string) ([]domain.RelatedInsight, error) {
	f.listTenantID = tenantID
	f.listInsightID = insightID
	return f.listRelated, f.err
}

func doCreateRequest(h *Handler, tenantID, insightID string, body CreateRelationshipRequestDTO) (*httptest.ResponseRecorder, map[string]any) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	raw, _ := json.Marshal(body)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/tenants/"+tenantID+"/insights/"+insightID+"/relationships", bytes.NewReader(raw))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "tenantID", Value: tenantID}, {Key: "insightID", Value: insightID}}

	h.Create(c)

	var respBody map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &respBody)
	return rec, respBody
}

func TestHandler_Create_HappyPath_PersistsAndReturns200(t *testing.T) {
	repo := &fakeRepo{}
	h := NewHandler(repo)

	rec, body := doCreateRequest(h, "t-1", "i-1", CreateRelationshipRequestDTO{
		ToInsightID: "i-2",
		Type:        "supports",
		Confidence:  0.9,
		Rationale:   "because reasons",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%v", rec.Code, http.StatusOK, body)
	}
	if !repo.putCalled {
		t.Fatalf("expected repo.Put to be called")
	}
	if repo.gotRel.TenantID != "t-1" || repo.gotRel.FromInsightID != "i-1" || repo.gotRel.ToInsightID != "i-2" {
		t.Fatalf("gotRel = %+v, want tenant=t-1 from=i-1 to=i-2", repo.gotRel)
	}
	if body["rationale"] != "because reasons" {
		t.Fatalf("response rationale = %v, want unmodified", body["rationale"])
	}
}

func TestHandler_Create_SelfLink_Rejected400_NeverReachesRepo(t *testing.T) {
	repo := &fakeRepo{}
	h := NewHandler(repo)

	rec, _ := doCreateRequest(h, "t-1", "i-1", CreateRelationshipRequestDTO{
		ToInsightID: "i-1",
		Type:        "supports",
		Confidence:  0.9,
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if repo.putCalled {
		t.Fatalf("expected repo.Put not to be called for a self-link")
	}
}

func TestHandler_Create_InvalidType_Rejected400_NeverReachesRepo(t *testing.T) {
	repo := &fakeRepo{}
	h := NewHandler(repo)

	rec, _ := doCreateRequest(h, "t-1", "i-1", CreateRelationshipRequestDTO{
		ToInsightID: "i-2",
		Type:        "not-a-real-type",
		Confidence:  0.9,
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if repo.putCalled {
		t.Fatalf("expected repo.Put not to be called for an invalid type")
	}
}

func TestHandler_Create_UnknownInsight_MapsToBadRequest(t *testing.T) {
	repo := &fakeRepo{err: ports.ErrInsightNotFound}
	h := NewHandler(repo)

	rec, _ := doCreateRequest(h, "t-1", "i-missing", CreateRelationshipRequestDTO{
		ToInsightID: "i-2",
		Type:        "supports",
		Confidence:  0.9,
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func doListRequest(h *Handler, jwtTenantID, urlTenantID, insightID string) (*httptest.ResponseRecorder, ListRelationshipsResponseDTO) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	c.Request = httptest.NewRequest(http.MethodGet, "/v1/tenants/"+urlTenantID+"/insights/"+insightID+"/relationships", nil)
	c.Params = gin.Params{{Key: "tenantID", Value: urlTenantID}, {Key: "insightID", Value: insightID}}
	c.Set(auth.TenantIDKey, jwtTenantID)

	h.ListByInsightID(c)

	var body ListRelationshipsResponseDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec, body
}

func TestHandler_ListByInsightID_HappyPath_ReturnsMappedItems(t *testing.T) {
	repo := &fakeRepo{listRelated: []domain.RelatedInsight{
		{InsightID: "i-2", Text: "hello", Type: domain.RelationSupports, Confidence: 0.9, Rationale: "because reasons"},
	}}
	h := NewHandler(repo)

	rec, body := doListRequest(h, "t-1", "t-1", "i-1")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(body.Items) != 1 || body.Items[0].InsightID != "i-2" || body.Items[0].Rationale != "because reasons" {
		t.Fatalf("body.Items = %+v, want single mapped related insight", body.Items)
	}
}

func TestHandler_ListByInsightID_NoRelationships_Returns200EmptyList(t *testing.T) {
	repo := &fakeRepo{}
	h := NewHandler(repo)

	rec, body := doListRequest(h, "t-1", "t-1", "i-1")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(body.Items) != 0 {
		t.Fatalf("body.Items = %v, want empty", body.Items)
	}
}

func TestHandler_ListByInsightID_UsesJWTTenant_NeverTheURLPathParam(t *testing.T) {
	repo := &fakeRepo{}
	h := NewHandler(repo)

	// URL says "t-attacker", but the JWT (auth.TenantIDKey) says "t-1" —
	// the query must scope to the JWT tenant, never the untrusted path.
	doListRequest(h, "t-1", "t-attacker", "i-1")

	if repo.listTenantID != "t-1" {
		t.Fatalf("repo queried tenant %q, want the JWT tenant t-1 (not the URL's t-attacker)", repo.listTenantID)
	}
}
