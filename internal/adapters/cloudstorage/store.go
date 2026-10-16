// Package cloudstorage implements the private blob-store port with Cloud Storage.
package cloudstorage

import (
	"context"
	"errors"
	"io"
	"regexp"

	"cloud.google.com/go/storage"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89a-fA-F][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

// Store uses a private bucket. Authorization remains in the application/API;
// this adapter never creates public ACLs or returns object URLs.
type Store struct {
	client *storage.Client
	bucket *storage.BucketHandle
}

func New(ctx context.Context, bucketName string) (*Store, error) {
	if bucketName == "" {
		return nil, errors.New("cloud storage bucket is required")
	}
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, errors.New("cloud storage client initialization failed")
	}
	return newStoreWithClient(client, bucketName), nil
}

func newStoreWithClient(client *storage.Client, bucketName string) *Store {
	return &Store{client: client, bucket: client.Bucket(bucketName)}
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
	writer := s.bucket.Object(objectName(id)).If(storage.Conditions{DoesNotExist: true}).NewWriter(ctx)
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
	reader, err := s.bucket.Object(objectName(id)).NewReader(ctx)
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
	err := s.bucket.Object(objectName(id)).Delete(ctx)
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

func objectName(id string) string { return id + ".blob" }
