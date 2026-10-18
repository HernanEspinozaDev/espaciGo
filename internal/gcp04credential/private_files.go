package gcp04credential

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
)

type LocalEvidence struct{}

func (LocalEvidence) VerifyPlan(expected string) error {
	if filepath.Clean(PlanPath) != PlanPath {
		return errors.New("invalid saved plan path")
	}
	data, err := readPrivateFile(PlanPath)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != expected {
		return errors.New("saved plan SHA-256 differs")
	}
	return nil
}

type PrivateJournal struct {
	Path string
}

func (j PrivateJournal) Create(record JournalRecord) error {
	if filepath.Clean(j.Path) != JournalPath || !privateDir(filepath.Dir(j.Path)) {
		return errors.New("journal must use the exact private run directory")
	}
	file, err := os.OpenFile(j.Path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return errors.New("journal already exists or cannot be created")
	}
	if err := writeJournal(file, record); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return errors.New("journal could not be durably closed")
	}
	return syncDirectory(filepath.Dir(j.Path))
}

func (j PrivateJournal) Update(record JournalRecord) error {
	if filepath.Clean(j.Path) != JournalPath || !privateDir(filepath.Dir(j.Path)) {
		return errors.New("journal must use the exact private run directory")
	}
	info, err := os.Lstat(j.Path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0600 {
		return errors.New("journal is not a private regular file")
	}
	tmp, err := os.CreateTemp(filepath.Dir(j.Path), ".credential-journal-*")
	if err != nil {
		return errors.New("journal update could not be staged")
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return errors.New("journal update could not be protected")
	}
	if err := writeJournal(tmp, record); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return errors.New("journal update could not be durably closed")
	}
	if err := os.Rename(tmpName, j.Path); err != nil {
		return errors.New("journal update could not replace the exact journal")
	}
	return syncDirectory(filepath.Dir(j.Path))
}

func writeJournal(file *os.File, record JournalRecord) error {
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(record); err != nil {
		return errors.New("journal could not be serialized")
	}
	if err := file.Sync(); err != nil {
		return errors.New("journal could not be synchronized")
	}
	return nil
}

func privateDir(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0 && info.Mode().Perm() == 0700
}

func syncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return errors.New("private directory could not be opened for synchronization")
	}
	defer dir.Close()
	if err := dir.Sync(); err != nil && !errors.Is(err, io.ErrClosedPipe) {
		return errors.New("private journal directory could not be synchronized")
	}
	return nil
}
