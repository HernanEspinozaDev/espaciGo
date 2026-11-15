package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type retentionWorkerRunnerFake struct {
	passErr error
	runErr  error
	poll    time.Duration
	called  bool
}

func (f *retentionWorkerRunnerFake) Run(_ context.Context, poll time.Duration, onError func(error)) error {
	f.called = true
	f.poll = poll
	onError(f.passErr)
	return f.runErr
}

func TestDocumentRetentionWorkerLogsOnlyStaticFailureMessages(t *testing.T) {
	const privateFailure = "synthetic private-path=/identity/fixture-logs"
	worker := &retentionWorkerRunnerFake{
		passErr: errors.New(privateFailure),
		runErr:  errors.New(privateFailure),
	}
	var logs []string
	runDocumentRetentionWorker(context.Background(), worker, func(message string) {
		logs = append(logs, message)
	})
	if !worker.called || worker.poll != time.Second {
		t.Fatalf("runner called=%t poll=%s, want one run with 1s polling", worker.called, worker.poll)
	}
	want := []string{
		"document verification retention pass failed",
		"document verification retention worker stopped",
	}
	if len(logs) != len(want) {
		t.Fatalf("logs=%q, want %q", logs, want)
	}
	for i := range want {
		if logs[i] != want[i] {
			t.Fatalf("log[%d]=%q, want %q", i, logs[i], want[i])
		}
		if strings.Contains(logs[i], privateFailure) {
			t.Fatalf("log[%d] contains raw storage error: %q", i, logs[i])
		}
	}
}
