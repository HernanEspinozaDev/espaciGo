package devauth

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLocalQuotaIsAtomicAndExpires(t *testing.T) {
	limiter := NewIPLimiter(3)
	now := time.Now()
	var accepted atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := limiter.AllowVerification(context.Background(), "192.0.2.1", now)
			if err != nil {
				t.Error(err)
			}
			if ok {
				accepted.Add(1)
			}
		}()
	}
	wg.Wait()
	if accepted.Load() != 3 {
		t.Fatal("local quota lost concurrent updates")
	}
	if ok, _ := limiter.AllowVerification(context.Background(), "192.0.2.1", now.Add(time.Hour)); !ok {
		t.Fatal("local quota did not expire")
	}
	if ok, _ := limiter.AllowVerification(context.Background(), "192.0.2.2", now); !ok {
		t.Fatal("different IP shared quota")
	}
}
