package documentfs

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

const MaxDocumentBytes = 10 << 20

const (
	storageEnvelopeMagic = "ESPACIGODOC1"
	maxStoredBytes       = MaxDocumentBytes + 64
)

var (
	ErrInvalid      = errors.New("documentfs: invalid document scope or content")
	ErrTooLarge     = errors.New("documentfs: document exceeds size limit")
	ErrObjectExists = errors.New("documentfs: immutable document object already exists")
	ErrStorage      = errors.New("documentfs: private storage unavailable")
)

type Store struct {
	root string
	aead cipher.AEAD
}

func New(root string, encryptionKey []byte) (*Store, error) {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return nil, ErrInvalid
	}
	if len(encryptionKey) != 32 {
		return nil, ErrInvalid
	}
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return nil, ErrInvalid
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, ErrInvalid
	}
	info, err := os.Lstat(root)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(root, 0o700); err != nil {
			return nil, ErrStorage
		}
		info, err = os.Lstat(root)
	}
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, ErrStorage
	}
	if info.Mode().Perm() != 0o700 {
		if err := os.Chmod(root, 0o700); err != nil {
			return nil, ErrStorage
		}
	}
	info, err = os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o700 {
		return nil, ErrStorage
	}
	return &Store{root: root, aead: aead}, nil
}

func (s *Store) PutIdentityDocument(ctx context.Context, ownerID, caseID, fileID string, content []byte) error {
	return s.put(ctx, identityPath(ownerID, caseID, fileID), content)
}

func (s *Store) PutSpaceDocument(ctx context.Context, ownerID, spaceID, caseID, fileID string, content []byte) error {
	return s.put(ctx, spacePath(ownerID, spaceID, caseID, fileID), content)
}

func (s *Store) ReadIdentityDocument(ctx context.Context, ownerID, caseID, fileID string) ([]byte, error) {
	return s.read(ctx, identityPath(ownerID, caseID, fileID))
}

// ReadSpaceDocument satisfies documentverification.SpaceDocumentStore. The
// caller supplies only IDs loaded from authoritative case state, never a key.
func (s *Store) ReadSpaceDocument(ctx context.Context, ownerID, spaceID, caseID, fileID string) ([]byte, error) {
	return s.read(ctx, spacePath(ownerID, spaceID, caseID, fileID))
}

func (s *Store) DeleteIdentityDocument(ctx context.Context, ownerID, caseID, fileID string) error {
	return s.remove(ctx, identityPath(ownerID, caseID, fileID))
}

func (s *Store) DeleteSpaceDocument(ctx context.Context, ownerID, spaceID, caseID, fileID string) error {
	return s.remove(ctx, spacePath(ownerID, spaceID, caseID, fileID))
}

func (s *Store) put(ctx context.Context, relative string, content []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if relative == "" || len(content) == 0 {
		return ErrInvalid
	}
	if len(content) > MaxDocumentBytes {
		return ErrTooLarge
	}
	sealed, err := s.seal(relative, content)
	if err != nil {
		return ErrStorage
	}
	root, err := s.openRoot()
	if err != nil {
		return err
	}
	defer root.Close()
	if err := ensurePrivateDirectories(root, filepath.Dir(relative)); err != nil {
		return ErrStorage
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	file, err := root.OpenFile(relative, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return ErrObjectExists
		}
		return ErrStorage
	}
	if _, err := file.Write(sealed); err != nil {
		_ = file.Close()
		_ = root.Remove(relative)
		return ErrStorage
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = root.Remove(relative)
		return ErrStorage
	}
	if err := file.Close(); err != nil {
		_ = root.Remove(relative)
		return ErrStorage
	}
	return nil
}

func (s *Store) read(ctx context.Context, relative string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if relative == "" {
		return nil, ErrInvalid
	}
	root, err := s.openRoot()
	if err != nil {
		return nil, err
	}
	defer root.Close()
	if err := checkPrivateDirectories(root, filepath.Dir(relative)); err != nil {
		return nil, os.ErrNotExist
	}
	file, err := root.OpenFile(relative, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, os.ErrNotExist
		}
		return nil, ErrStorage
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || info.Size() <= int64(len(storageEnvelopeMagic)+12+16) || info.Size() > maxStoredBytes {
		return nil, ErrStorage
	}
	sealed, err := io.ReadAll(io.LimitReader(file, maxStoredBytes+1))
	if err != nil || len(sealed) > maxStoredBytes {
		return nil, ErrStorage
	}
	content, err := s.open(relative, sealed)
	if err != nil || len(content) == 0 || len(content) > MaxDocumentBytes {
		return nil, ErrStorage
	}
	return content, nil
}

func (s *Store) remove(ctx context.Context, relative string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if relative == "" {
		return ErrInvalid
	}
	root, err := s.openRoot()
	if err != nil {
		return err
	}
	defer root.Close()
	if err := checkPrivateDirectories(root, filepath.Dir(relative)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return ErrStorage
	}
	if err := root.Remove(relative); err != nil && !errors.Is(err, os.ErrNotExist) {
		return ErrStorage
	}
	return nil
}

func (s *Store) openRoot() (*os.Root, error) {
	if s == nil || s.root == "" {
		return nil, ErrStorage
	}
	root, err := os.OpenRoot(s.root)
	if err != nil {
		return nil, ErrStorage
	}
	info, err := root.Stat(".")
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
		_ = root.Close()
		return nil, ErrStorage
	}
	return root, nil
}

func ensurePrivateDirectories(root *os.Root, relative string) error {
	current := ""
	for _, part := range splitPath(relative) {
		if current == "" {
			current = part
		} else {
			current = filepath.Join(current, part)
		}
		if err := root.Mkdir(current, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
			return err
		}
		info, err := root.Lstat(current)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return ErrStorage
		}
		if err := root.Chmod(current, 0o700); err != nil {
			return err
		}
	}
	return nil
}

func checkPrivateDirectories(root *os.Root, relative string) error {
	current := ""
	for _, part := range splitPath(relative) {
		if current == "" {
			current = part
		} else {
			current = filepath.Join(current, part)
		}
		info, err := root.Lstat(current)
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o700 {
			return ErrStorage
		}
	}
	return nil
}

func splitPath(path string) []string {
	parts := strings.Split(filepath.ToSlash(path), "/")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" && part != "." {
			result = append(result, part)
		}
	}
	return result
}

func (s *Store) seal(relative string, content []byte) ([]byte, error) {
	if s == nil || s.aead == nil {
		return nil, ErrStorage
	}
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	sealed := make([]byte, 0, len(storageEnvelopeMagic)+len(nonce)+len(content)+s.aead.Overhead())
	sealed = append(sealed, storageEnvelopeMagic...)
	sealed = append(sealed, nonce...)
	sealed = s.aead.Seal(sealed, nonce, content, []byte(relative))
	return sealed, nil
}

func (s *Store) open(relative string, sealed []byte) ([]byte, error) {
	if s == nil || s.aead == nil || len(sealed) < len(storageEnvelopeMagic)+s.aead.NonceSize()+s.aead.Overhead() || string(sealed[:len(storageEnvelopeMagic)]) != storageEnvelopeMagic {
		return nil, ErrStorage
	}
	nonceStart := len(storageEnvelopeMagic)
	nonceEnd := nonceStart + s.aead.NonceSize()
	return s.aead.Open(nil, sealed[nonceStart:nonceEnd], sealed[nonceEnd:], []byte(relative))
}

func identityPath(ownerID, caseID, fileID string) string {
	if documentverification.ValidateID(ownerID) != nil || documentverification.ValidateID(caseID) != nil || documentverification.ValidateID(fileID) != nil {
		return ""
	}
	return filepath.Join("users", ownerID, "identity", caseID, fileID+".bin")
}

func spacePath(ownerID, spaceID, caseID, fileID string) string {
	if documentverification.ValidateID(ownerID) != nil || documentverification.ValidateID(spaceID) != nil || documentverification.ValidateID(caseID) != nil || documentverification.ValidateID(fileID) != nil {
		return ""
	}
	return filepath.Join("users", ownerID, "spaces", spaceID, "documents", caseID, fileID+".bin")
}

var _ documentverification.SpaceDocumentStore = (*Store)(nil)
