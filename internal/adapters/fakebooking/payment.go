// Package fakebooking contains the development-only local payment adapter.
// It must only be constructed by cmd/api when LOCAL_BOOKING_TRIAL is enabled.
package fakebooking

import (
	"context"
	"errors"
)

var ErrUnsupportedOutcome = errors.New("fake booking payment: unsupported local result")

type Adapter struct{}

func New() *Adapter { return &Adapter{} }
func (*Adapter) Process(_ context.Context, requested string) (string, error) {
	switch requested {
	case "exito", "rechazo", "sin_respuesta":
		return requested, nil
	default:
		return "", ErrUnsupportedOutcome
	}
}
