package documentgcs

import (
	"context"
	"errors"
	"sync"
)

type memoryObject struct {
	blob     []byte
	metadata map[string]string
}
type memoryStore struct {
	mu      sync.Mutex
	objects map[string]memoryObject
}

func newMemoryStore() *memoryStore { return &memoryStore{objects: make(map[string]memoryObject)} }
func (m *memoryStore) PutWithMetadata(_ context.Context, id string, blob []byte, metadata map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.objects[id]; ok {
		return errors.New("exists")
	}
	m.objects[id] = memoryObject{append([]byte(nil), blob...), clone(metadata)}
	return nil
}
func (m *memoryStore) GetWithMetadata(_ context.Context, id string, max int64) ([]byte, map[string]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	x, ok := m.objects[id]
	if !ok {
		return nil, nil, errors.New("not found")
	}
	if max > 0 && int64(len(x.blob)) > max {
		return nil, nil, errors.New("too large")
	}
	return append([]byte(nil), x.blob...), clone(x.metadata), nil
}
func (m *memoryStore) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.objects, id)
	return nil
}
func clone(in map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		out[k] = v
	}
	return out
}
