// Package businessclock provides the optional, local-demo business clock.
// With no configured file it is exactly the real system clock.
package businessclock

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"time"
)

var ErrInvalid = errors.New("invalid local business clock configuration")

type state struct {
	mu   sync.RWMutex
	path string
	last time.Time
}

// FromEnv enables the manual clock only when the local booking trial is
// explicitly enabled and a clock file is configured. File contents are
// reread so a host-side command can advance a running API without restart.
func FromEnv(getenv func(string) string) (func() time.Time, error) {
	path := strings.TrimSpace(getenv("LOCAL_DEMO_BUSINESS_CLOCK_FILE"))
	if path == "" {
		return time.Now, nil
	}
	if getenv("LOCAL_BOOKING_TRIAL") != "1" || path != "/run/fe14-demo/business-clock.json" {
		return nil, ErrInvalid
	}
	c, err := newState(path)
	if err != nil {
		return nil, ErrInvalid
	}
	return c.now, nil
}

func newState(path string) (*state, error) {
	c := &state{path: path}
	value, err := c.read()
	if err != nil {
		return nil, err
	}
	c.last = value
	return c, nil
}

func (c *state) read() (time.Time, error) {
	data, err := os.ReadFile(c.path)
	if err != nil {
		return time.Time{}, err
	}
	var value struct {
		Now string `json:"now"`
	}
	if json.Unmarshal(data, &value) != nil {
		return time.Time{}, ErrInvalid
	}
	at, err := time.Parse(time.RFC3339Nano, value.Now)
	if err != nil {
		return time.Time{}, ErrInvalid
	}
	return at.UTC(), nil
}

func (c *state) now() time.Time {
	if value, err := c.read(); err == nil {
		c.mu.Lock()
		c.last = value
		c.mu.Unlock()
		return value
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.last
}
