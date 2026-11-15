package declarationcrypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const maxDataSize = 64 * 1024

type Keyring struct {
	current string
	keys    map[string][]byte
}

type fileConfig struct {
	Current string            `json:"current"`
	Keys    map[string]string `json:"keys"`
}

func New(current string, keys map[string][]byte) (*Keyring, error) {
	if !validKeyID(current) || len(keys) == 0 {
		return nil, errors.New("invalid keyring configuration")
	}
	copied := make(map[string][]byte, len(keys))
	for id, key := range keys {
		if !validKeyID(id) || len(key) != 32 {
			return nil, errors.New("invalid keyring configuration")
		}
		copied[id] = append([]byte(nil), key...)
	}
	if _, ok := copied[current]; !ok {
		return nil, errors.New("invalid keyring configuration")
	}
	return &Keyring{current: current, keys: copied}, nil
}

func LoadFile(path string) (*Keyring, error) {
	if !filepath.IsAbs(path) {
		return nil, errors.New("invalid keyring file")
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return nil, errors.New("invalid keyring file")
	}
	if info.Size() > maxDataSize {
		return nil, errors.New("invalid keyring file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.New("invalid keyring file")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxDataSize+1))
	if err != nil || len(data) > maxDataSize {
		return nil, errors.New("invalid keyring file")
	}
	var config fileConfig
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return nil, errors.New("invalid keyring file")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, errors.New("invalid keyring file")
	}
	if config.Current == "" || config.Keys == nil {
		return nil, errors.New("invalid keyring file")
	}
	keys := make(map[string][]byte, len(config.Keys))
	for id, encoded := range config.Keys {
		key, err := hex.DecodeString(encoded)
		if err != nil || len(key) != 32 {
			return nil, errors.New("invalid keyring file")
		}
		keys[id] = key
	}
	kr, err := New(config.Current, keys)
	if err != nil {
		return nil, errors.New("invalid keyring file")
	}
	return kr, nil
}

func (k *Keyring) Seal(aad, plaintext []byte) (string, []byte, error) {
	if len(plaintext) == 0 || len(plaintext) > maxDataSize {
		return "", nil, errors.New("invalid plaintext size")
	}
	block, err := aes.NewCipher(k.keys[k.current])
	if err != nil {
		return "", nil, errors.New("encryption failed")
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", nil, errors.New("encryption failed")
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", nil, errors.New("encryption failed")
	}
	return k.current, gcm.Seal(nonce, nonce, plaintext, aad), nil
}

func (k *Keyring) Open(keyVersion string, aad, ciphertext []byte) ([]byte, error) {
	key, ok := k.keys[keyVersion]
	if !ok {
		return nil, errors.New("decryption failed")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.New("decryption failed")
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil || len(ciphertext) < gcm.NonceSize()+gcm.Overhead() {
		return nil, errors.New("decryption failed")
	}
	nonce, sealed := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, sealed, aad)
	if err != nil || len(plaintext) == 0 || len(plaintext) > maxDataSize {
		return nil, errors.New("decryption failed")
	}
	return plaintext, nil
}

func validKeyID(id string) bool {
	if len(id) == 0 || len(id) > 64 {
		return false
	}
	isAlphaNum := func(b byte) bool { return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' }
	if !isAlphaNum(id[0]) {
		return false
	}
	for i := 1; i < len(id); i++ {
		if !isAlphaNum(id[i]) && id[i] != '.' && id[i] != '_' && id[i] != '-' {
			return false
		}
	}
	return true
}
