package documentfs

import (
	"context"
	"errors"
	"testing"
)

func TestNilStorePutRemainsClosedWithoutPanic(t *testing.T) {
	var store *Store
	owner := "11111111-1111-4111-8111-111111111111"
	caseID := "44444444-4444-4444-8444-444444444444"
	fileID := "55555555-5555-4555-8555-555555555555"
	if err := store.PutIdentityDocument(context.Background(), owner, caseID, fileID, []byte("synthetic")); !errors.Is(err, ErrStorage) {
		t.Fatalf("PutIdentityDocument error=%v, want ErrStorage", err)
	}
}
