package types

import "context"

// DecodeBudget limits aggregate materialized filter output for a single document read.
// Implementations reserve capacity before a filter materializes its output.
type DecodeBudget interface {
	ReserveDecode(maxDecodeBytes int64) (stageLimit, reservation int64, err error)
	FinishDecode(reservation, decodedBytes int64, success bool) error
}

// DecodeContextProvider supplies the request context to filter decoders.
type DecodeContextProvider interface {
	DecodeContext() context.Context
}
