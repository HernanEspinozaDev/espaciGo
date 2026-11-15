package rentalflowhttp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/rentalflow"
)

func FuzzAvailabilityQueryRequiresSingleKnownDateAndDuration(f *testing.F) {
	for _, query := range []string{
		"date=2030-01-02&duration=1",
		"date=2030-01-02&duration=4",
		"date=2030-01-02&date=2030-01-03&duration=1",
		"date=2030-01-02&duration=1&duration=2",
		"date=2030-01-02&duration=one",
		"date=2030-01-02&duration=999999999999999999999999999999",
		"date=2030-02-30&duration=1",
		"date=2030-01-02&duration=1&owner_id=x",
		"date=2030-01-02&duration=%ZZ",
		"date=2030-01-02&duration=1&broken=%",
		"",
	} {
		f.Add(query)
	}

	f.Fuzz(func(t *testing.T, rawQuery string) {
		if len(rawQuery) > 4096 {
			t.Skip()
		}
		availability := &availabilityStub{result: rentalflow.Availability{Items: []rentalflow.Interval{}}}
		handler := newTestHandler(&authStub{}, availability, &quoteStub{}, &previewStub{})
		request := httptest.NewRequest(http.MethodGet, "/api/v1/spaces/"+testSpaceID+"/rental-availability", nil)
		request.URL.RawQuery = rawQuery
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)

		if response.Header().Get("Cache-Control") != "no-store" || !rentalflow.ValidUUID(response.Header().Get("X-Request-ID")) || !json.Valid(response.Body.Bytes()) {
			t.Fatalf("invalid response envelope: status=%d cache=%q requestID=%q body=%q", response.Code, response.Header().Get("Cache-Control"), response.Header().Get("X-Request-ID"), response.Body.String())
		}

		values, parseErr := url.ParseQuery(rawQuery)
		duration := 0
		valid := parseErr == nil && len(values) == 2 && len(values["date"]) == 1 && len(values["duration"]) == 1 && rentalflow.ValidDate(values.Get("date"))
		if valid {
			duration, parseErr = strconv.Atoi(values.Get("duration"))
			valid = parseErr == nil && duration >= 1 && duration <= 4
		}
		if valid {
			if response.Code != http.StatusOK || availability.calls != 1 || availability.space != testSpaceID || availability.date != values.Get("date") || availability.days != duration {
				t.Fatalf("valid query dispatch mismatch: status=%d calls=%d space=%q date=%q duration=%d body=%s", response.Code, availability.calls, availability.space, availability.date, availability.days, response.Body.String())
			}
			return
		}
		if response.Code != http.StatusBadRequest || availability.calls != 0 {
			t.Fatalf("invalid query reached service: status=%d calls=%d rawQuery=%q body=%s", response.Code, availability.calls, rawQuery, response.Body.String())
		}
	})
}

func FuzzAvailabilityQueryDispatchesMappedValidDateAndDuration(f *testing.F) {
	for _, seed := range []struct {
		days     uint16
		rawUnits uint8
	}{{0, 0}, {59, 3}, {366, 1}, {731, 2}} {
		f.Add(seed.days, seed.rawUnits)
	}

	f.Fuzz(func(t *testing.T, rawDays uint16, rawUnits uint8) {
		dayOffset := int(rawDays % 800)
		date := time.Date(2028, time.January, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, dayOffset).Format("2006-01-02")
		duration := int(rawUnits%4) + 1
		query := url.Values{}
		query.Set("date", date)
		query.Set("duration", strconv.Itoa(duration))

		availability := &availabilityStub{result: rentalflow.Availability{Items: []rentalflow.Interval{}}}
		handler := newTestHandler(&authStub{}, availability, &quoteStub{}, &previewStub{})
		request := httptest.NewRequest(http.MethodGet, "/api/v1/spaces/"+testSpaceID+"/rental-availability", nil)
		request.URL.RawQuery = query.Encode()
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)

		if response.Code != http.StatusOK || availability.calls != 1 || availability.space != testSpaceID || availability.date != date || availability.days != duration {
			t.Fatalf("valid mapped query dispatch mismatch: status=%d calls=%d space=%q date=%q duration=%d body=%s", response.Code, availability.calls, availability.space, availability.date, availability.days, response.Body.String())
		}
		if response.Header().Get("Cache-Control") != "no-store" || !rentalflow.ValidUUID(response.Header().Get("X-Request-ID")) || !json.Valid(response.Body.Bytes()) {
			t.Fatalf("invalid response envelope: status=%d cache=%q requestID=%q body=%q", response.Code, response.Header().Get("Cache-Control"), response.Header().Get("X-Request-ID"), response.Body.String())
		}
	})
}
