package model

import (
	"context"
	"sync"

	"github.com/pdfcpu/pdfcpu/pkg/filter"
)

// DecodeBudget tracks aggregate materialized filter output for one PDF read.
type DecodeBudget struct {
	mu       sync.Mutex
	limit    int64
	used     int64
	reserved int64
	ctx      context.Context
}

func NewDecodeBudget(limit int64) *DecodeBudget {
	if limit == 0 {
		limit = 512 << 20
	}
	return &DecodeBudget{limit: limit}
}

func newDecodeBudget(limit int64) *DecodeBudget { return NewDecodeBudget(limit) }

// SetLimit changes the aggregate limit unless doing so would invalidate output
// already charged or reserved by an in-flight filter stage. Zero selects the
// documented default; a negative value disables the aggregate cap.
func (b *DecodeBudget) SetLimit(limit int64) error {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if limit == 0 {
		limit = 512 << 20
	}
	if limit >= 0 && (b.used > limit || b.reserved > limit-b.used) {
		return filter.ErrDecodeLimitExceeded
	}
	b.limit = limit
	return nil
}

func (b *DecodeBudget) Limit() int64 {
	if b == nil {
		return -1
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.limit
}

func (b *DecodeBudget) Used() int64 {
	if b == nil {
		return 0
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.used
}

func (b *DecodeBudget) SetDecodeContext(ctx context.Context) {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.ctx = ctx
}

func (b *DecodeBudget) DecodeContext() context.Context {
	if b == nil {
		return context.Background()
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.ctx == nil {
		return context.Background()
	}
	return b.ctx
}

func (b *DecodeBudget) Charge(n int64) error { return b.charge(n) }

func (b *DecodeBudget) charge(n int64) error {
	if b == nil || n <= 0 {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.limit >= 0 && (b.used >= b.limit || b.reserved > b.limit-b.used || n > b.limit-b.used-b.reserved) {
		return filter.ErrDecodeLimitExceeded
	}
	if n > int64(^uint64(0)>>1)-b.used {
		return filter.ErrDecodeLimitExceeded
	}
	b.used += n
	return nil
}

// ReserveDecode reserves the maximum output for one filter stage before it
// runs. The returned stage limit is also bounded by the remaining document
// budget, so the decoder cannot materialize beyond the reservation.
func (b *DecodeBudget) ReserveDecode(maxDecodeBytes int64) (int64, int64, error) {
	if b == nil {
		if maxDecodeBytes == 0 {
			maxDecodeBytes = filter.DefaultMaxDecodeBytes
		}
		return maxDecodeBytes, 0, nil
	}
	if maxDecodeBytes == 0 {
		maxDecodeBytes = filter.DefaultMaxDecodeBytes
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.limit < 0 && maxDecodeBytes < 0 {
		return -1, 0, nil
	}
	stageLimit := maxDecodeBytes
	if b.limit >= 0 {
		if b.used >= b.limit || b.reserved >= b.limit-b.used {
			return 0, 0, filter.ErrDecodeLimitExceeded
		}
		available := b.limit - b.used - b.reserved
		if stageLimit < 0 || stageLimit > available {
			stageLimit = available
		}
	}
	if stageLimit < 0 {
		return -1, 0, nil
	}
	if stageLimit == 0 || stageLimit > int64(^uint64(0)>>1)-b.reserved {
		return 0, 0, filter.ErrDecodeLimitExceeded
	}
	b.reserved += stageLimit
	return stageLimit, stageLimit, nil
}

// FinishDecode settles a prior reservation. On decoder error the full
// reservation is charged conservatively because partial output may exist.
func (b *DecodeBudget) FinishDecode(reservation, decodedBytes int64, success bool) error {
	if b == nil || reservation == 0 {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if reservation < 0 || decodedBytes < 0 || reservation > b.reserved {
		return filter.ErrDecodeLimitExceeded
	}
	b.reserved -= reservation
	charge := reservation
	if success {
		if decodedBytes > reservation {
			charge = reservation
		} else {
			charge = decodedBytes
		}
	}
	if charge > int64(^uint64(0)>>1)-b.used {
		b.used = int64(^uint64(0) >> 1)
		return filter.ErrDecodeLimitExceeded
	}
	b.used += charge
	if success && decodedBytes > reservation {
		return filter.ErrDecodeLimitExceeded
	}
	return nil
}
