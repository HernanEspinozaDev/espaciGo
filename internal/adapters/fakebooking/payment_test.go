package fakebooking

import (
	"context"
	"errors"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/booking"
)

func TestPaymentEventsAreAuthenticatedAndLookupDoesNotStartAnotherPayment(t *testing.T) {
	key := []byte("fake-provider-authentication-key-at-least-32-bytes")
	adapter, err := New(key)
	if err != nil {
		t.Fatal(err)
	}
	operation := booking.PaymentOperation{ID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", Requested: "exito", State: "pendiente"}
	started, err := adapter.StartPayment(context.Background(), operation.ID, operation.Requested)
	if err != nil || started == nil || !adapter.VerifyPaymentEvent(*started) {
		t.Fatalf("start event=%+v err=%v; expected authenticated fake event", started, err)
	}
	lookedUp, err := adapter.LookupPayment(context.Background(), operation)
	if err != nil || lookedUp == nil || lookedUp.EventID != started.EventID || lookedUp.Signature != started.Signature {
		t.Fatalf("lookup event=%+v err=%v; expected same stable event", lookedUp, err)
	}
	tampered := *started
	tampered.Outcome = "rechazo_simulado"
	if adapter.VerifyPaymentEvent(tampered) {
		t.Fatal("tampered payment event was authenticated")
	}

	operation.Requested = "sin_respuesta"
	if event, callErr := adapter.StartPayment(context.Background(), operation.ID, operation.Requested); event != nil || !errors.Is(callErr, booking.ErrSimulatedNoResponse) {
		t.Fatalf("timeout event=%+v err=%v", event, callErr)
	}
	if event, lookupErr := adapter.LookupPayment(context.Background(), operation); event != nil || lookupErr != nil {
		t.Fatalf("status lookup initiated or invented a charge result: event=%+v err=%v", event, lookupErr)
	}
}
