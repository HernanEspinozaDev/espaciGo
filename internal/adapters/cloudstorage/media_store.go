package cloudstorage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"regexp"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/HernanEspinozaDev/espaciGo/internal/media"
)

var mediaUUID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89a-fA-F][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

// MediaStore confines uploads to generated names inside the versioned
// media/v1 namespace and always uses create-only writes and exact generations.
type MediaStore struct {
	client     *storage.Client
	bucket     *storage.BucketHandle
	bucketName string
}

func NewMediaStore(ctx context.Context, bucketName string) (*MediaStore, error) {
	if strings.TrimSpace(bucketName) == "" {
		return nil, errors.New("media bucket is required")
	}
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, errors.New("media storage client initialization failed")
	}
	return &MediaStore{client: client, bucket: client.Bucket(bucketName), bucketName: bucketName}, nil
}

func (s *MediaStore) Close() error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close()
}

func (s *MediaStore) Put(ctx context.Context, key string, b []byte, mime string) (media.StoredObject, error) {
	if err := validateMediaKey(key); err != nil || mime != "image/webp" || len(b) == 0 {
		return media.StoredObject{}, media.ErrInvalid
	}
	obj := s.bucket.Object(key).If(storage.Conditions{DoesNotExist: true})
	w := obj.NewWriter(ctx)
	w.ContentType = mime
	w.CacheControl = "private, no-store"
	w.Metadata = map[string]string{"sha256": hashMedia(b)}
	if _, err := w.Write(b); err != nil {
		_ = w.Close()
		return media.StoredObject{}, err
	}
	if err := w.Close(); err != nil {
		return media.StoredObject{}, err
	}
	attrs := w.Attrs()
	if attrs == nil || attrs.Generation <= 0 {
		return media.StoredObject{}, errors.New("media object generation unavailable")
	}
	return media.StoredObject{ObjectKey: key, Provider: "gcs", Bucket: s.bucketName, Generation: attrs.Generation, Size: attrs.Size, SHA256: hashMedia(b)}, nil
}

func (s *MediaStore) Get(ctx context.Context, key string, generation int64) ([]byte, error) {
	if err := validateMediaKey(key); err != nil || generation <= 0 {
		return nil, media.ErrInvalid
	}
	r, err := s.bucket.Object(key).Generation(generation).NewReader(ctx)
	if err != nil {
		return nil, err
	}
	b, readErr := io.ReadAll(io.LimitReader(r, 10<<20+1))
	closeErr := r.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if len(b) > 10<<20 {
		return nil, media.ErrInvalid
	}
	return b, nil
}

func (s *MediaStore) Generation(ctx context.Context, key string) (int64, error) {
	if err := validateMediaKey(key); err != nil {
		return 0, media.ErrInvalid
	}
	attrs, err := s.bucket.Object(key).Attrs(ctx)
	if errors.Is(err, storage.ErrObjectNotExist) {
		return 0, media.ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	return attrs.Generation, nil
}

func (s *MediaStore) Delete(ctx context.Context, key string, generation int64) error {
	if err := validateMediaKey(key); err != nil || generation <= 0 {
		return media.ErrInvalid
	}
	err := s.bucket.Object(key).If(storage.Conditions{GenerationMatch: generation}).Delete(ctx)
	if errors.Is(err, storage.ErrObjectNotExist) {
		return nil
	}
	return err
}

func validateMediaKey(key string) error {
	parts := strings.Split(key, "/")
	if len(parts) == 7 && parts[0] == "media" && parts[1] == "v1" {
		if parts[2] == "spaces" && mediaUUID.MatchString(parts[3]) && mediaUUID.MatchString(parts[4]) && validToken(parts[5]) && (parts[6] == "display.webp" || parts[6] == "thumbnail.webp") {
			return nil
		}
		if parts[2] == "profiles" && mediaUUID.MatchString(parts[3]) && mediaUUID.MatchString(parts[4]) && validToken(parts[5]) && parts[6] == "avatar.webp" {
			return nil
		}
	}
	if len(parts) == 8 && parts[0] == "users" && mediaUUID.MatchString(parts[1]) && parts[2] == "spaces" && mediaUUID.MatchString(parts[3]) && parts[4] == "images" && parts[5] == "v2" && mediaUUID.MatchString(parts[6]) {
		bits := strings.Split(parts[7], "_")
		if len(bits) == 3 && validMediaTimestamp(bits[0]) && validToken(bits[1]) && (bits[2] == "display.webp" || bits[2] == "thumbnail.webp") {
			return nil
		}
	}
	return media.ErrInvalid
}

func validMediaTimestamp(v string) bool {
	const layout = "20060102T150405.000000000Z"
	parsed, err := time.Parse(layout, v)
	return err == nil && parsed.UTC().Format(layout) == v
}

func validToken(v string) bool {
	if v == "" || v[0] == '0' {
		return false
	}
	for _, r := range v {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
func hashMedia(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
