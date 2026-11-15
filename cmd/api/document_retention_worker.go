package main

import (
	"context"
	"time"
)

type documentRetentionRunner interface {
	Run(context.Context, time.Duration, func(error)) error
}

func runDocumentRetentionWorker(ctx context.Context, worker documentRetentionRunner, logMessage func(string)) {
	if err := worker.Run(ctx, time.Second, func(error) {
		logMessage("document verification retention pass failed")
	}); err != nil {
		logMessage("document verification retention worker stopped")
	}
}
