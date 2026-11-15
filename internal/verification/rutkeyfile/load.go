// Package rutkeyfile loads the private HMAC keyring without exposing material
// through configuration errors. It never creates, rotates, or rewrites secrets.
package rutkeyfile

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
)

const maxFileBytes = 64 * 1024

var errInvalid = errors.New("invalid private identity fingerprint keyring configuration")

func privateMetadata(info os.FileInfo, dir bool) bool {
	if info == nil || info.Mode()&os.ModeSymlink != 0 {
		return false
	}
	s, ok := info.Sys().(*syscall.Stat_t)
	if !ok || s.Uid != uint32(os.Getuid()) {
		return false
	}
	if dir {
		return info.IsDir() && info.Mode().Perm() == 0700
	}
	return info.Mode().IsRegular() && info.Mode().Perm() == 0600 && info.Size() <= maxFileBytes
}

// Load accepts an absolute, operator-owned 0600 regular file in an owned 0700
// directory. Format: active version and a keys object of version -> hex bytes.
// Duplicate fields/versions are rejected rather than silently overwritten.
func Load(path string) (*verification.RutHMACKeyring, error) {
	if !filepath.IsAbs(path) {
		return nil, errInvalid
	}
	parent, err := os.Lstat(filepath.Dir(path))
	if err != nil || !privateMetadata(parent, true) {
		return nil, errInvalid
	}
	info, err := os.Lstat(path)
	if err != nil || !privateMetadata(info, false) {
		return nil, errInvalid
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, errInvalid
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !privateMetadata(opened, false) || !os.SameFile(info, opened) {
		return nil, errInvalid
	}
	data, err := io.ReadAll(io.LimitReader(f, maxFileBytes+1))
	if err != nil || len(data) > maxFileBytes {
		return nil, errInvalid
	}
	return decode(data)
}

func decode(data []byte) (*verification.RutHMACKeyring, error) {
	d := json.NewDecoder(bytes.NewReader(data))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return nil, errInvalid
	}
	seen := map[string]bool{}
	var active string
	keys := map[string][]byte{}
	for d.More() {
		token, err = d.Token()
		if err != nil {
			return nil, errInvalid
		}
		field, ok := token.(string)
		if !ok || seen[field] {
			return nil, errInvalid
		}
		seen[field] = true
		switch field {
		case "active":
			if d.Decode(&active) != nil {
				return nil, errInvalid
			}
		case "keys":
			token, err = d.Token()
			if err != nil || token != json.Delim('{') {
				return nil, errInvalid
			}
			for d.More() {
				token, err = d.Token()
				if err != nil {
					return nil, errInvalid
				}
				version, ok := token.(string)
				if !ok {
					return nil, errInvalid
				}
				if _, duplicate := keys[version]; duplicate || len(keys) >= 64 {
					return nil, errInvalid
				}
				var encoded string
				if d.Decode(&encoded) != nil {
					return nil, errInvalid
				}
				key, err := hex.DecodeString(encoded)
				if err != nil {
					return nil, errInvalid
				}
				keys[version] = key
			}
			token, err = d.Token()
			if err != nil || token != json.Delim('}') {
				return nil, errInvalid
			}
		default:
			return nil, errInvalid
		}
	}
	token, err = d.Token()
	if err != nil || token != json.Delim('}') || !seen["active"] || !seen["keys"] {
		return nil, errInvalid
	}
	if _, err = d.Token(); err != io.EOF {
		return nil, errInvalid
	}
	ring, err := verification.NewRutHMACKeyring(active, keys)
	if err != nil {
		return nil, errInvalid
	}
	return ring, nil
}
