package operationhttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/operation"
)

type returnsRepo struct {
	operation.Repository
	actor, space string
	deny         bool
}

func (r *returnsRepo) ListSpaceReturns(_ context.Context, actor, space string) ([]operation.SpaceReturn, error) {
	r.actor, r.space = actor, space
	if r.deny {
		return nil, operation.ErrNotFound
	}
	return []operation.SpaceReturn{{ReservationID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", State: "finalizada", StartAt: time.Unix(0, 0).UTC(), EndAt: time.Unix(3600, 0).UTC(), CheckOutRecorded: true}}, nil
}

type returnsFiles struct{ operation.FileStore }
type returnsIDs struct{ operation.IDGenerator }

func TestCurrentOwnerReturnsProjectionIsPrivateMinimalAndScoped(t *testing.T) {
	repo := &returnsRepo{}
	service, err := operation.NewService(repo, returnsFiles{}, returnsIDs{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(testAuthenticator{}, service, nil)
	path := "/api/v1/local/booking-trial/spaces/aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa/returns"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
	if response.Code != 200 || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("private readback failed", response.Code)
	}
	var envelope struct {
		Data struct {
			SpaceID string           `json:"space_id"`
			Items   []map[string]any `json:"items"`
		}
	}
	if err = json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if repo.actor != "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb" || repo.space != envelope.Data.SpaceID || len(envelope.Data.Items) != 1 {
		t.Fatal("owner scope lost")
	}
	item := envelope.Data.Items[0]
	if len(item) != 6 {
		t.Fatal("private projection has unexpected fields")
	}
	for _, key := range []string{"reservation_id", "state", "start_at", "end_at", "check_out_recorded", "vacancy_confirmed"} {
		if _, ok := item[key]; !ok {
			t.Fatal("projection key missing", key)
		}
	}
	repo.deny = true
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
	if response.Code != 404 {
		t.Fatal("non-owner read allowed", response.Code)
	}
}
