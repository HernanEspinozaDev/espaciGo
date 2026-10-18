// Package cloudstorage implements the private blob-store port with Cloud Storage.
package cloudstorage

import (
	"context"
	"errors"
	"io"
	"path"
	"regexp"
	"strings"

	"cloud.google.com/go/storage"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89a-fA-F][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

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
	if err := validate(ctx, id); err != nil {
		return err
	}
	writer := s.bucket.Object(s.objectName(id)).If(storage.Conditions{DoesNotExist: true}).NewWriter(ctx)
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
	if err := validate(ctx, id); err != nil {
		return nil, err
	}
	reader, err := s.bucket.Object(s.objectName(id)).NewReader(ctx)
	if err != nil {
		return nil, err
	}
	blob, readErr := io.ReadAll(reader)
	closeErr := reader.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return blob, nil
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
