// Package fakebooking contains the development-only local payment adapter.
// It must only be constructed by cmd/api when LOCAL_BOOKING_TRIAL is enabled.
package fakebooking

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/booking"
)

var ErrUnsupportedOutcome = errors.New("fake booking payment: unsupported local result")

// ResultStore is durable state owned by the local fake adapter, separate from
// the application's payment intent and webhook inbox.
type ResultStore interface {
	SaveFakePaymentResult(context.Context, booking.PaymentEvent, time.Time) (booking.PaymentEvent, bool, error)
	RecordFakePaymentTimeout(context.Context, string, time.Time) error
	FindFakePaymentResult(context.Context, string) (*booking.PaymentEvent, error)
}

type Adapter struct {
	eventKey []byte
	store    ResultStore
	mu       sync.Mutex
	results  map[string]booking.PaymentEvent
}

func New(eventKey []byte) (*Adapter, error) {
	if len(eventKey) < 32 {
		return nil, errors.New("local payment event key must contain at least 32 bytes")
	}
	return &Adapter{eventKey: append([]byte(nil), eventKey...), results: make(map[string]booking.PaymentEvent)}, nil
}

func NewWithStore(eventKey []byte, store ResultStore) (*Adapter, error) {
	adapter, err := New(eventKey)
	if err != nil {
		return nil, err
	}
	if store == nil {
		return nil, errors.New("local fake payment result store is required")
	}
	adapter.store = store
	return adapter, nil
}

func (a *Adapter) StartPayment(ctx context.Context, operationID, requested string) (*booking.PaymentEvent, error) {
	if strings.TrimSpace(operationID) == "" {
		return nil, ErrUnsupportedOutcome
	}
	switch requested {
	case "sin_respuesta":
		if a.store != nil {
			if err := a.store.RecordFakePaymentTimeout(ctx, operationID, time.Now().UTC()); err != nil {
				return nil, err
			}
		}
		return nil, booking.ErrSimulatedNoResponse
	case "exito":
		event, _, err := a.recordResult(ctx, operationID, "exito_simulado")
		if err != nil {
			return nil, err
		}
		return &event, nil
	case "rechazo":
		event, _, err := a.recordResult(ctx, operationID, "rechazo_simulado")
		if err != nil {
			return nil, err
		}
		return &event, nil
	default:
		return nil, ErrUnsupportedOutcome
	}
}

// LookupPayment is a status query keyed by the persisted operation. It never
// starts a second fake charge and lets the service recover an interrupted
// request after the Backend restarts.
func (a *Adapter) LookupPayment(ctx context.Context, operation booking.PaymentOperation) (*booking.PaymentEvent, error) {
	if strings.TrimSpace(operation.ID) == "" {
		return nil, ErrUnsupportedOutcome
	}
	if a.store != nil {
		event, err := a.store.FindFakePaymentResult(ctx, operation.ID)
		if err != nil || event == nil {
			return event, err
		}
		signed := a.SignPaymentEvent(operation.ID, event.Outcome)
		if signed.EventID != event.EventID {
			return nil, ErrUnsupportedOutcome
		}
		return signed, nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	var result *booking.PaymentEvent
	for _, event := range a.results {
		if event.OperationID == operation.ID {
			value := event
			result = &value
			break
		}
	}
	return result, nil
}

func (a *Adapter) recordResult(ctx context.Context, operationID, outcome string) (booking.PaymentEvent, bool, error) {
	event := *a.SignPaymentEvent(operationID, outcome)
	if a.store != nil {
		stored, created, err := a.store.SaveFakePaymentResult(ctx, event, time.Now().UTC())
		if err != nil {
			return booking.PaymentEvent{}, false, err
		}
		stored.Signature = a.SignPaymentEvent(operationID, stored.Outcome).Signature
		return stored, created, nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if existing, ok := a.results[operationID]; ok {
		if existing.Outcome != event.Outcome || existing.EventID != event.EventID {
			return booking.PaymentEvent{}, false, ErrUnsupportedOutcome
		}
		return existing, false, nil
	}
	a.results[operationID] = event
	return event, true, nil
}

// SignPaymentEvent creates a signed callback for the local fake. Its stable
// development key is never reused as a provider credential.
func (a *Adapter) SignPaymentEvent(operationID, outcome string) *booking.PaymentEvent {
	event := &booking.PaymentEvent{
		EventID:     fmt.Sprintf("localfake:%s:%s", operationID, outcome),
		OperationID: operationID,
		Outcome:     outcome,
	}
	mac := hmac.New(sha256.New, a.eventKey)
	_, _ = mac.Write(paymentEventPayload(*event))
	event.Signature = base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return event
}

func (a *Adapter) VerifyPaymentEvent(event booking.PaymentEvent) bool {
	if event.EventID == "" || event.OperationID == "" || (event.Outcome != "exito_simulado" && event.Outcome != "rechazo_simulado") {
		return false
	}
	expected := a.SignPaymentEvent(event.OperationID, event.Outcome)
	if event.EventID != expected.EventID {
		return false
	}
	provided, err := base64.RawURLEncoding.DecodeString(event.Signature)
	if err != nil {
		return false
	}
	return hmac.Equal(provided, mustDecodeSignature(expected.Signature))
}

func paymentEventPayload(event booking.PaymentEvent) []byte {
	return []byte(event.EventID + "\n" + event.OperationID + "\n" + event.Outcome)
}

func mustDecodeSignature(signature string) []byte {
	decoded, _ := base64.RawURLEncoding.DecodeString(signature)
	return decoded
}

func (*Adapter) ProcessRefund(_ context.Context, operationID, requestedKey, requested string) (string, error) {
	if strings.TrimSpace(operationID) == "" || requestedKey != operationID {
		return "", ErrUnsupportedOutcome
	}
	switch requested {
	case "exito":
		return "exito_simulado", nil
	case "fallo":
		return "fallo_simulado", nil
	case "sin_respuesta":
		return "sin_respuesta_simulada", nil
	default:
		return "", ErrUnsupportedOutcome
	}
}
