package mediafs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/HernanEspinozaDev/espaciGo/internal/media"
)

type Store struct{ root string }

func New(root string) (*Store, error) {
	if !filepath.IsAbs(root) {
		return nil, errors.New("media directory must be absolute")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, errors.New("media directory is unavailable")
	}
	if err := os.Chmod(root, 0o700); err != nil {
		return nil, errors.New("media directory is unavailable")
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0o077 != 0 {
		return nil, errors.New("media directory must be private")
	}
	return &Store{root: root}, nil
}

func (s *Store) Put(ctx context.Context, key string, b []byte, mime string) (media.StoredObject, error) {
	if err := ctx.Err(); err != nil {
		return media.StoredObject{}, err
	}
	path, err := s.filePath(key)
	if err != nil || mime != "image/webp" || len(b) == 0 {
		return media.StoredObject{}, media.ErrInvalid
	}
	if err = os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return media.StoredObject{}, errors.New("private media write failed")
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return media.StoredObject{}, err
	}
	if _, err = f.Write(b); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return media.StoredObject{}, err
	}
	if err = f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return media.StoredObject{}, err
	}
	if err = f.Close(); err != nil {
		_ = os.Remove(path)
		return media.StoredObject{}, err
	}
	d := sha256.Sum256(b)
	return media.StoredObject{ObjectKey: key, Provider: "filesystem", Bucket: "local-private-media", Generation: 1, Size: int64(len(b)), SHA256: hex.EncodeToString(d[:])}, nil
}

func (s *Store) Get(ctx context.Context, key string, generation int64) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if generation != 1 {
		return nil, media.ErrNotFound
	}
	path, err := s.filePath(key)
	if err != nil {
		return nil, media.ErrNotFound
	}
	return os.ReadFile(path)
}

func (s *Store) Generation(ctx context.Context, key string) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	path, err := s.filePath(key)
	if err != nil {
		return 0, media.ErrInvalid
	}
	if _, err = os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return 0, media.ErrNotFound
	} else if err != nil {
		return 0, err
	}
	return 1, nil
}

func (s *Store) Delete(ctx context.Context, key string, generation int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if generation != 1 {
		return media.ErrConflict
	}
	path, err := s.filePath(key)
	if err != nil {
		return media.ErrInvalid
	}
	err = os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s *Store) filePath(key string) (string, error) {
	if strings.ContainsAny(key, "\\\x00") || filepath.IsAbs(key) || filepath.ToSlash(filepath.Clean(filepath.FromSlash(key))) != key || !strings.HasPrefix(key, "media/v1/") {
		return "", media.ErrInvalid
	}
	for _, part := range strings.Split(key, "/") {
		if part == "" || part == "." || part == ".." {
			return "", media.ErrInvalid
		}
	}
	path := filepath.Join(s.root, filepath.FromSlash(key))
	rel, err := filepath.Rel(s.root, path)
	if err != nil || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", media.ErrInvalid
	}
	return path, nil
}
