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

	"github.com/HernanEspinozaDev/espaciGo/internal/booking"
)

var ErrUnsupportedOutcome = errors.New("fake booking payment: unsupported local result")

type Adapter struct{ eventKey []byte }

func New(eventKey []byte) (*Adapter, error) {
	if len(eventKey) < 32 {
		return nil, errors.New("local payment event key must contain at least 32 bytes")
	}
	return &Adapter{eventKey: append([]byte(nil), eventKey...)}, nil
}

func (a *Adapter) StartPayment(_ context.Context, operationID, requested string) (*booking.PaymentEvent, error) {
	if strings.TrimSpace(operationID) == "" {
		return nil, ErrUnsupportedOutcome
	}
	switch requested {
	case "exito":
		return a.SignPaymentEvent(operationID, "exito_simulado"), nil
	case "rechazo":
		return a.SignPaymentEvent(operationID, "rechazo_simulado"), nil
	case "sin_respuesta":
		return nil, booking.ErrSimulatedNoResponse
	default:
		return nil, ErrUnsupportedOutcome
	}
}

// LookupPayment is a status query keyed by the persisted operation. It never
// starts a second fake charge and lets the service recover an interrupted
// request after the Backend restarts.
func (a *Adapter) LookupPayment(_ context.Context, operation booking.PaymentOperation) (*booking.PaymentEvent, error) {
	switch operation.Requested {
	case "exito":
		return a.SignPaymentEvent(operation.ID, "exito_simulado"), nil
	case "rechazo":
		return a.SignPaymentEvent(operation.ID, "rechazo_simulado"), nil
	case "sin_respuesta":
		return nil, nil
	default:
		return nil, ErrUnsupportedOutcome
	}
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
