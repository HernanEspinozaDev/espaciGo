package fakebooking

import (
	"context"
	"errors"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/booking"
)

func TestUnknownLookupDoesNotFabricateResultAndStartIsIdempotent(t *testing.T) {
	key := []byte("fake-provider-authentication-key-at-least-32-bytes")
	adapter, err := New(key)
	if err != nil {
		t.Fatal(err)
	}
	operation := booking.PaymentOperation{ID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", Requested: "exito", State: "pendiente"}
	if event, err := adapter.LookupPayment(context.Background(), operation); err != nil || event != nil {
		t.Fatalf("unknown fake operation lookup=%+v err=%v; want no fabricated result", event, err)
	}
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

	operation2 := booking.PaymentOperation{ID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", Requested: "sin_respuesta", State: "pendiente"}
	if event, err := adapter.LookupPayment(context.Background(), operation2); err != nil || event != nil {
		t.Fatalf("unknown timeout operation lookup=%+v err=%v", event, err)
	}
	if event, callErr := adapter.StartPayment(context.Background(), operation2.ID, operation2.Requested); event != nil || !errors.Is(callErr, booking.ErrSimulatedNoResponse) {
		t.Fatalf("timeout event=%+v err=%v", event, callErr)
	}
	if event, lookupErr := adapter.LookupPayment(context.Background(), operation2); lookupErr != nil || event != nil {
		t.Fatalf("timeout lookup invented a result: event=%+v err=%v", event, lookupErr)
	}
	if event, callErr := adapter.StartPayment(context.Background(), operation2.ID, operation2.Requested); event != nil || !errors.Is(callErr, booking.ErrSimulatedNoResponse) {
		t.Fatalf("repeated timeout start event=%+v err=%v", event, callErr)
	}
	operation3 := booking.PaymentOperation{ID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", Requested: "exito", State: "pendiente"}
	started3, err := adapter.StartPayment(context.Background(), operation3.ID, operation3.Requested)
	if err != nil || started3 == nil {
		t.Fatalf("start before simulated crash=%+v err=%v", started3, err)
	}
	lookup3, err := adapter.LookupPayment(context.Background(), operation3)
	if err != nil || lookup3 == nil || lookup3.EventID != started3.EventID || !adapter.VerifyPaymentEvent(*lookup3) {
		t.Fatalf("lookup did not recover actually recorded fake result: event=%+v err=%v", lookup3, err)
	}
}
