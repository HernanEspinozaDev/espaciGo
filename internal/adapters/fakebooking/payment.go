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
	now      func() time.Time
	mu       sync.Mutex
	results  map[string]booking.PaymentEvent
	timeouts map[string]bool
}

func New(eventKey []byte) (*Adapter, error) {
	return NewWithClock(eventKey, time.Now)
}

func NewWithClock(eventKey []byte, now func() time.Time) (*Adapter, error) {
	if len(eventKey) < 32 {
		return nil, errors.New("local payment event key must contain at least 32 bytes")
	}
	if now == nil {
		now = time.Now
	}
	return &Adapter{eventKey: append([]byte(nil), eventKey...), now: now, results: make(map[string]booking.PaymentEvent), timeouts: make(map[string]bool)}, nil
}

func NewWithStore(eventKey []byte, store ResultStore) (*Adapter, error) {
	return NewWithStoreAndClock(eventKey, store, time.Now)
}

func NewWithStoreAndClock(eventKey []byte, store ResultStore, now func() time.Time) (*Adapter, error) {
	adapter, err := NewWithClock(eventKey, now)
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
			if err := a.store.RecordFakePaymentTimeout(ctx, operationID, a.now().UTC()); err != nil {
				return nil, err
			}
		} else {
			a.mu.Lock()
			a.timeouts[operationID] = true
			a.mu.Unlock()
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
	if a.timeouts[operation.ID] {
		return nil, booking.ErrSimulatedNoResponse
	}
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
		stored, created, err := a.store.SaveFakePaymentResult(ctx, event, a.now().UTC())
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

// StartTransfer is a development-only, operation-keyed fake transfer. Its
// durable result is intentionally independent from the business transaction;
// the caller persists an intent first and applies the result afterward.
func (a *Adapter) StartTransfer(ctx context.Context, operationID, requested string) (*booking.SettlementEvent, error) {
	if strings.TrimSpace(operationID) == "" {
		return nil, ErrUnsupportedOutcome
	}
	if existing, err := a.lookupTransfer(ctx, operationID); err != nil || existing != nil {
		return existing, err
	}
	switch requested {
	case "sin_respuesta":
		if a.store != nil {
			store, ok := a.store.(booking.SettlementResultStore)
			if !ok {
				return nil, ErrUnsupportedOutcome
			}
			if err := store.RecordFakeSettlementTimeout(ctx, operationID, a.now().UTC()); err != nil {
				return nil, err
			}
		} else {
			a.mu.Lock()
			a.timeouts["settlement:"+operationID] = true
			a.mu.Unlock()
		}
		return nil, booking.ErrSimulatedNoResponse
	case "exito", "rechazo":
		outcome := "exito_simulado"
		if requested == "rechazo" {
			outcome = "rechazo_simulado"
		}
		event := a.SignSettlementEvent(operationID, outcome)
		if a.store != nil {
			store, ok := a.store.(booking.SettlementResultStore)
			if !ok {
				return nil, ErrUnsupportedOutcome
			}
			stored, _, err := store.SaveFakeSettlementResult(ctx, *event, a.now().UTC())
			if err != nil {
				return nil, err
			}
			stored.Signature = a.SignSettlementEvent(operationID, stored.Outcome).Signature
			return &stored, nil
		}
		a.mu.Lock()
		key := "settlement:" + operationID
		if prior, ok := a.results[key]; ok && (prior.EventID != event.EventID || prior.Outcome != event.Outcome) {
			a.mu.Unlock()
			return nil, ErrUnsupportedOutcome
		}
		a.results[key] = booking.PaymentEvent{EventID: event.EventID, OperationID: key, Outcome: event.Outcome}
		a.mu.Unlock()
		return event, nil
	default:
		return nil, ErrUnsupportedOutcome
	}
}

func (a *Adapter) LookupTransfer(ctx context.Context, settlement booking.Settlement) (*booking.SettlementEvent, error) {
	if settlement.OperationID == "" {
		return nil, ErrUnsupportedOutcome
	}
	return a.lookupTransfer(ctx, settlement.OperationID)
}

func (a *Adapter) lookupTransfer(ctx context.Context, operationID string) (*booking.SettlementEvent, error) {
	if a.store != nil {
		store, ok := a.store.(booking.SettlementResultStore)
		if !ok {
			return nil, ErrUnsupportedOutcome
		}
		event, err := store.FindFakeSettlementResult(ctx, operationID)
		if err != nil || event == nil {
			return nil, err
		}
		signed := a.SignSettlementEvent(operationID, event.Outcome)
		if signed.EventID != event.EventID {
			return nil, ErrUnsupportedOutcome
		}
		return signed, nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.timeouts["settlement:"+operationID] {
		return nil, booking.ErrSimulatedNoResponse
	}
	for _, event := range a.results {
		if event.OperationID == "settlement:"+operationID {
			value := booking.SettlementEvent{EventID: event.EventID, OperationID: operationID, Outcome: event.Outcome}
			value.Signature = a.SignSettlementEvent(operationID, value.Outcome).Signature
			return &value, nil
		}
	}
	return nil, nil
}

func (a *Adapter) ResolveTransfer(ctx context.Context, operationID, outcome string) (booking.SettlementEvent, error) {
	if operationID == "" || (outcome != "exito" && outcome != "rechazo") {
		return booking.SettlementEvent{}, ErrUnsupportedOutcome
	}
	wanted := "exito_simulado"
	if outcome == "rechazo" {
		wanted = "rechazo_simulado"
	}
	if a.store != nil {
		store, ok := a.store.(booking.SettlementResultStore)
		if !ok {
			return booking.SettlementEvent{}, ErrUnsupportedOutcome
		}
		event, err := store.ResolveFakeSettlementResult(ctx, operationID, wanted, a.now().UTC())
		if err != nil {
			return booking.SettlementEvent{}, err
		}
		event.Signature = a.SignSettlementEvent(operationID, event.Outcome).Signature
		return event, nil
	}
	a.mu.Lock()
	delete(a.timeouts, "settlement:"+operationID)
	key := "settlement:" + operationID
	a.results[key] = booking.PaymentEvent{EventID: fmt.Sprintf("localfake:settlement:%s:%s", operationID, wanted), OperationID: key, Outcome: wanted}
	a.mu.Unlock()
	event := a.SignSettlementEvent(operationID, wanted)
	return *event, nil
}

func (a *Adapter) SignSettlementEvent(operationID, outcome string) *booking.SettlementEvent {
	event := &booking.SettlementEvent{EventID: fmt.Sprintf("localfake:settlement:%s:%s", operationID, outcome), OperationID: operationID, Outcome: outcome}
	mac := hmac.New(sha256.New, a.eventKey)
	_, _ = mac.Write(settlementEventPayload(*event))
	event.Signature = base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return event
}

func (a *Adapter) VerifyTransferEvent(event booking.SettlementEvent) bool {
	if event.EventID == "" || event.OperationID == "" || (event.Outcome != "exito_simulado" && event.Outcome != "rechazo_simulado") {
		return false
	}
	expected := a.SignSettlementEvent(event.OperationID, event.Outcome)
	provided, err := base64.RawURLEncoding.DecodeString(event.Signature)
	return err == nil && event.EventID == expected.EventID && hmac.Equal(provided, mustDecodeSignature(expected.Signature))
}

func settlementEventPayload(event booking.SettlementEvent) []byte {
	return []byte(event.EventID + "\n" + event.OperationID + "\n" + event.Outcome)
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
