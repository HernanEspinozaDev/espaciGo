// Package cloudstorage implements the private blob-store port with Cloud Storage.
package cloudstorage

import (
	"context"
	"errors"
	"io"
	"path"
	"regexp"
	"strings"
	"unicode/utf8"

	"cloud.google.com/go/storage"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89a-fA-F][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)
var metadataKeyPattern = regexp.MustCompile(`^[a-z0-9_-]+$`)

var ErrObjectTooLarge = errors.New("private blob exceeds configured size limit")

// Store uses a private bucket. Authorization remains in the application/API;
// this adapter never creates public ACLs or returns object URLs.
type Store struct {
	client *storage.Client
	bucket *storage.BucketHandle
	prefix string
}

func New(ctx context.Context, bucketName string) (*Store, error) {
	return NewWithPrefix(ctx, bucketName, "")
}

// NewWithPrefix scopes private blobs under an optional run-specific object prefix.
func NewWithPrefix(ctx context.Context, bucketName, prefix string) (*Store, error) {
	if bucketName == "" {
		return nil, errors.New("cloud storage bucket is required")
	}
	if err := validatePrefix(prefix); err != nil {
		return nil, err
	}
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, errors.New("cloud storage client initialization failed")
	}
	return newStoreWithPrefix(client, bucketName, prefix), nil
}

func newStoreWithClient(client *storage.Client, bucketName string) *Store {
	return newStoreWithPrefix(client, bucketName, "")
}

func newStoreWithPrefix(client *storage.Client, bucketName, prefix string) *Store {
	return &Store{client: client, bucket: client.Bucket(bucketName), prefix: prefix}
}

func (s *Store) Close() error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close()
}

func (s *Store) Put(ctx context.Context, id string, blob []byte) error {
	return s.PutWithMetadata(ctx, id, blob, nil)
}

// PutWithMetadata stores metadata on a private blob using a create-only
// precondition. Metadata is cloned and constrained to safe, bounded values.
func (s *Store) PutWithMetadata(ctx context.Context, id string, blob []byte, metadata map[string]string) error {
	if err := validate(ctx, id); err != nil {
		return err
	}
	metadataCopy, err := validateMetadata(metadata)
	if err != nil {
		return err
	}
	writer := s.bucket.Object(s.objectName(id)).If(storage.Conditions{DoesNotExist: true}).NewWriter(ctx)
	writer.Metadata = metadataCopy
	if _, err := writer.Write(blob); err != nil {
		_ = writer.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return nil
}

func (s *Store) Get(ctx context.Context, id string) ([]byte, error) {
	blob, _, err := s.GetWithMetadata(ctx, id, 0)
	return blob, err
}

// GetWithMetadata retrieves a private blob and its metadata. A positive
// maxBytes bounds both the remote object size and bytes read into memory.
func (s *Store) GetWithMetadata(ctx context.Context, id string, maxBytes int64) ([]byte, map[string]string, error) {
	if err := validate(ctx, id); err != nil {
		return nil, nil, err
	}
	if maxBytes < 0 {
		return nil, nil, errors.New("invalid private blob size limit")
	}
	obj := s.bucket.Object(s.objectName(id))
	attrs, err := obj.Attrs(ctx)
	if err != nil {
		return nil, nil, err
	}
	if maxBytes > 0 && attrs.Size > maxBytes {
		return nil, nil, ErrObjectTooLarge
	}
	reader, err := obj.If(storage.Conditions{GenerationMatch: attrs.Generation, MetagenerationMatch: attrs.Metageneration}).NewReader(ctx)
	if err != nil {
		return nil, nil, err
	}
	var source io.Reader = reader
	if maxBytes > 0 && maxBytes < int64(^uint64(0)>>1) {
		source = io.LimitReader(reader, maxBytes+1)
	}
	blob, readErr := io.ReadAll(source)
	if readErr == nil && maxBytes > 0 && int64(len(blob)) > maxBytes {
		readErr = ErrObjectTooLarge
	}
	metadata := cloneMetadata(attrs.Metadata)
	closeErr := reader.Close()
	if readErr != nil {
		return nil, nil, readErr
	}
	if closeErr != nil {
		return nil, nil, closeErr
	}
	return blob, metadata, nil
}

func (s *Store) Delete(ctx context.Context, id string) error {
	if err := validate(ctx, id); err != nil {
		return err
	}
	obj := s.bucket.Object(s.objectName(id))
	attrs, err := obj.Attrs(ctx)
	if errors.Is(err, storage.ErrObjectNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	// Bind deletion to the generation observed for this key. If an object is
	// replaced between Attrs and Delete, GCS rejects the stale generation.
	err = obj.If(storage.Conditions{GenerationMatch: attrs.Generation}).Delete(ctx)
	if errors.Is(err, storage.ErrObjectNotExist) {
		return nil
	}
	return err
}

func validate(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !uuidPattern.MatchString(id) {
		return errors.New("invalid blob identifier")
	}
	return nil
}

func validateMetadata(metadata map[string]string) (map[string]string, error) {
	if len(metadata) == 0 {
		return nil, nil
	}
	if len(metadata) > 32 {
		return nil, errors.New("too many private blob metadata entries")
	}
	out := make(map[string]string, len(metadata))
	total := 0
	for key, value := range metadata {
		if !metadataKeyPattern.MatchString(key) || len(key) > 128 || !validMetadataValue(value) {
			return nil, errors.New("invalid private blob metadata")
		}
		total += len(key) + len(value)
		if total > 8192 {
			return nil, errors.New("private blob metadata exceeds size limit")
		}
		out[key] = value
	}
	return out, nil
}

func validMetadataValue(value string) bool {
	if value == "" || len(value) > 1024 || !utf8.ValidString(value) {
		return false
	}
	for i := 0; i < len(value); i++ {
		if value[i] < 0x20 || value[i] > 0x7e {
			return false
		}
	}
	return true
}

func cloneMetadata(metadata map[string]string) map[string]string {
	if len(metadata) == 0 {
		return nil
	}
	out := make(map[string]string, len(metadata))
	for key, value := range metadata {
		out[key] = value
	}
	return out
}

func (s *Store) objectName(id string) string {
	if s.prefix == "" {
		return id + ".blob"
	}
	return s.prefix + "/" + id + ".blob"
}

func validatePrefix(prefix string) error {
	if prefix == "" {
		return nil
	}
	if strings.HasPrefix(prefix, "/") || strings.ContainsAny(prefix, "\\\x00") || path.Clean(prefix) != prefix {
		return errors.New("invalid private blob object prefix")
	}
	for _, part := range strings.Split(prefix, "/") {
		if part == "" || part == "." || part == ".." {
			return errors.New("invalid private blob object prefix")
		}
	}
	return nil
}
