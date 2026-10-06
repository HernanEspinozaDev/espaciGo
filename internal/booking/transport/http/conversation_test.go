package bookinghttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/booking"
	"github.com/HernanEspinozaDev/espaciGo/internal/conversation"
)

const testReservationID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"

type conversationRepoStub struct {
	sent   conversation.Message
	cursor int64
}

func (r *conversationRepoStub) List(_ context.Context, actor, reservation string, before *int64, limit int) (conversation.Page, error) {
	if actor != renterID || reservation != testReservationID {
		return conversation.Page{}, conversation.ErrNotFound
	}
	return conversation.Page{Items: []conversation.Message{{ID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", ReservationID: reservation, AuthorID: actor, Sequence: 4, Body: "texto <script> como texto", CreatedAt: time.Unix(1, 0)}}, OlderCursor: before}, nil
}
func (r *conversationRepoStub) Send(_ context.Context, actor, reservation, key, body string, _ []byte, id string, now func() time.Time) (conversation.Message, error) {
	if actor != renterID || reservation != testReservationID {
		return conversation.Message{}, conversation.ErrNotFound
	}
	r.sent = conversation.Message{ID: id, ReservationID: reservation, AuthorID: actor, Sequence: 5, Body: body, CreatedAt: now()}
	return r.sent, nil
}
func (r *conversationRepoStub) MarkRead(_ context.Context, actor, reservation string, through int64) (int64, error) {
	if actor != renterID || reservation != testReservationID {
		return 0, conversation.ErrNotFound
	}
	if through < 1 {
		return 0, conversation.ErrInvalid
	}
	if through > r.cursor {
		r.cursor = through
	}
	return r.cursor, nil
}

func TestConversationRoutesAuthenticateValidateAndReturnPlainText(t *testing.T) {
	bookings, err := booking.NewService(repoStub{fixture: booking.Fixture{SpaceID: testReservationID, Title: "Espacio sintético", OwnerID: renterID, RenterID: renterID, RateUnit: "hora", Price: 8000, Currency: "CLP", TimeZone: "America/Santiago"}}, credentials.Generator{}, time.Now, paymentStub{})
	if err != nil {
		t.Fatal(err)
	}
	conversationRepo := &conversationRepoStub{}
	conversations, err := conversation.NewService(conversationRepo, credentials.Generator{}, func() time.Time { return time.Unix(1, 0) })
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(authStub{}, bookings, []string{"http://localhost:8081"}, conversations)

	unauthorized := httptest.NewRecorder()
	h.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/v1/local/booking-trial/reservations/"+testReservationID+"/messages", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized read status=%d", unauthorized.Code)
	}

	badCursor := httptest.NewRecorder()
	badRequest := httptest.NewRequest(http.MethodGet, "/api/v1/local/booking-trial/reservations/"+testReservationID+"/messages?before=-1", nil)
	badRequest.Header.Set("Authorization", "Bearer test-session")
	h.ServeHTTP(badCursor, badRequest)
	if badCursor.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid cursor status=%d body=%s", badCursor.Code, badCursor.Body.String())
	}

	get := httptest.NewRecorder()
	getRequest := httptest.NewRequest(http.MethodGet, "/api/v1/local/booking-trial/reservations/"+testReservationID+"/messages?limit=10", nil)
	getRequest.Header.Set("Authorization", "Bearer test-session")
	h.ServeHTTP(get, getRequest)
	var pageEnvelope struct {
		Data struct {
			Items []conversation.Message `json:"items"`
		} `json:"data"`
	}
	if get.Code != http.StatusOK || json.Unmarshal(get.Body.Bytes(), &pageEnvelope) != nil || len(pageEnvelope.Data.Items) != 1 || pageEnvelope.Data.Items[0].Body != "texto <script> como texto" {
		t.Fatalf("message page=%d %s", get.Code, get.Body.String())
	}

	post := httptest.NewRecorder()
	postRequest := httptest.NewRequest(http.MethodPost, "/api/v1/local/booking-trial/reservations/"+testReservationID+"/messages", strings.NewReader(`{"body":"<img src=x onerror=alert(1)>"}`))
	postRequest.Header.Set("Authorization", "Bearer test-session")
	postRequest.Header.Set("Content-Type", "application/json")
	postRequest.Header.Set("Idempotency-Key", "test-message-key")
	h.ServeHTTP(post, postRequest)
	if post.Code != http.StatusCreated {
		t.Fatalf("message send=%d %s", post.Code, post.Body.String())
	}
	var envelope struct {
		Data   conversation.Message `json:"data"`
		Safety string               `json:"safety_notice"`
	}
	if err = json.Unmarshal(post.Body.Bytes(), &envelope); err != nil || envelope.Data.Body != "<img src=x onerror=alert(1)>" || envelope.Safety != booking.SafetyBanner {
		t.Fatalf("unexpected message envelope: %+v err=%v", envelope, err)
	}

	read := httptest.NewRecorder()
	readRequest := httptest.NewRequest(http.MethodPost, "/api/v1/local/booking-trial/reservations/"+testReservationID+"/messages/read", strings.NewReader(`{"through_sequence":5}`))
	readRequest.Header.Set("Authorization", "Bearer test-session")
	readRequest.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(read, readRequest)
	if read.Code != http.StatusOK || conversationRepo.cursor != 5 || !strings.Contains(read.Body.String(), `"read_through_sequence":5`) {
		t.Fatalf("read cursor status=%d cursor=%d body=%s", read.Code, conversationRepo.cursor, read.Body.String())
	}

	missingKey := httptest.NewRecorder()
	missingKeyRequest := httptest.NewRequest(http.MethodPost, "/api/v1/local/booking-trial/reservations/"+testReservationID+"/messages", strings.NewReader(`{"body":"texto"}`))
	missingKeyRequest.Header.Set("Authorization", "Bearer test-session")
	missingKeyRequest.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(missingKey, missingKeyRequest)
	if missingKey.Code != http.StatusUnprocessableEntity {
		t.Fatalf("missing idempotency key status=%d body=%s", missingKey.Code, missingKey.Body.String())
	}
}
