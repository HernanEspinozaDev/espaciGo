package publiccatalog

import (
	"context"
	"errors"
	"testing"
)

func TestNewRequiresRepositoryAndAtLeast32ByteCursorMasterKey(t *testing.T) {
	key := []byte("01234567890123456789012345678901")
	for _, tc := range []struct {
		name string
		repo Repository
		key  []byte
	}{
		{name: "missing repository", repo: nil, key: key},
		{name: "short cursor key", repo: memoryRepository{}, key: key[:31]},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, err := New(tc.repo, tc.key)
			if service != nil || !errors.Is(err, ErrInvalid) {
				t.Fatalf("New() service=%v error=%v, want nil/ErrInvalid", service, err)
			}
		})
	}

	service, err := New(memoryRepository{}, key)
	if err != nil || service == nil {
		t.Fatalf("New() with exactly 32 key bytes service=%v error=%v", service, err)
	}
}

func TestNewWithImageAndMediaStoreRequiresMediaStore(t *testing.T) {
	key := []byte("01234567890123456789012345678901")
	legacy := PublicImageStoreFunc(func(context.Context, string) ([]byte, error) {
		return nil, ErrNotFound
	})
	service, err := NewWithImageAndMediaStore(memoryRepository{}, key, legacy, nil)
	if service != nil || !errors.Is(err, ErrInvalid) {
		t.Fatalf("NewWithImageAndMediaStore() service=%v error=%v, want nil/ErrInvalid", service, err)
	}
}

func TestNewWithImageStorePropagatesInvalidBaseConfiguration(t *testing.T) {
	key := []byte("0123456789012345678901234567890")
	legacy := PublicImageStoreFunc(func(context.Context, string) ([]byte, error) {
		return nil, ErrNotFound
	})
	service, err := NewWithImageStore(memoryRepository{}, key, legacy)
	if service != nil || !errors.Is(err, ErrInvalid) {
		t.Fatalf("NewWithImageStore() service=%v error=%v, want nil/ErrInvalid", service, err)
	}
}
