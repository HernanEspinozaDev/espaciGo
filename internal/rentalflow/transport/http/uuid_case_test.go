package rentalflowhttp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/rentalflow"
)

func TestSyntheticHTTPAvailabilityCanonicalizesUppercaseSpaceUUID(t *testing.T) {
	const spaceID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	now := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.January, 2, 13, 0, 0, 0, time.UTC)
	backend := &syntheticFlowBackend{
		listing: rentalflow.Listing{
			ID: spaceID, OwnerID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", State: "activa",
			Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 12000, Currency: "CLP",
			RateVersion: 2, TimeZone: "America/Santiago",
		},
		interval: rentalflow.Interval{StartAt: start, EndAt: start.Add(time.Hour)}, available: true,
	}
	availability, err := rentalflow.NewAvailabilityService(backend, backend, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	handler := New(nil, availability, nil, nil, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet,
		"/api/v1/spaces/"+strings.ToUpper(spaceID)+"/rental-availability?date=2030-01-02&duration=1", nil))
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("uppercase UUID availability status=%d cache=%q body=%s", response.Code, response.Header().Get("Cache-Control"), response.Body.String())
	}
	var envelope struct {
		Data rentalflow.Availability `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.SpaceID != spaceID || len(envelope.Data.Items) != 1 || backend.listingReads != 1 {
		t.Fatalf("uppercase UUID was not canonicalized at HTTP boundary: data=%+v reads=%d", envelope.Data, backend.listingReads)
	}
}
