package relationship

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
	"github.com/marcogerstmann/insight-processing-platform/internal/ports"
)

type fakeRepo struct {
	putCalled bool
	gotRel    domain.Relationship
	err       error
}

func (f *fakeRepo) Put(_ context.Context, rel domain.Relationship) error {
	f.putCalled = true
	f.gotRel = rel
	return f.err
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

	rec, _ := doCreateRequest(h, "t-1", "i-1", CreateRelationshipRequestDTO{
		ToInsightID: "i-missing",
		Type:        "supports",
		Confidence:  0.9,
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
