package migrator

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var migrationName = regexp.MustCompile(`^V([0-9]{6})__([a-z0-9]+(?:_[a-z0-9]+)*)\.sql$`)

type Migration struct {
	Version  int64
	Name     string
	Path     string
	SQL      []byte
	Checksum string
}

type HistoryEntry struct {
	Version  int64
	Name     string
	Checksum string
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func DiscoverMigrations(dir string) ([]Migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read migration directory: %w", err)
	}
	var migrations []Migration
	seen := make(map[int64]string)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		matches := migrationName.FindStringSubmatch(entry.Name())
		if matches == nil {
			return nil, fmt.Errorf("invalid migration filename %q", entry.Name())
		}
		version, err := strconv.ParseInt(matches[1], 10, 64)
		if err != nil || version < 1 {
			return nil, fmt.Errorf("invalid migration version in %q", entry.Name())
		}
		if prior, ok := seen[version]; ok {
			return nil, fmt.Errorf("duplicate migration version %d: %s and %s", version, prior, entry.Name())
		}
		seen[version] = entry.Name()
		path := filepath.Join(dir, entry.Name())
		sql, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}
		if len(strings.TrimSpace(string(sql))) == 0 {
			return nil, fmt.Errorf("migration %s is empty", entry.Name())
		}
		migrations = append(migrations, Migration{
			Version:  version,
			Name:     entry.Name(),
			Path:     path,
			SQL:      sql,
			Checksum: sha256Hex(sql),
		})
	}
	sort.Slice(migrations, func(i, j int) bool { return migrations[i].Version < migrations[j].Version })
	for i, migration := range migrations {
		want := int64(i + 1)
		if migration.Version != want {
			return nil, fmt.Errorf("migration sequence is not contiguous from V000001: expected version %06d, found %06d", want, migration.Version)
		}
	}
	return migrations, nil
}

func verifyHistory(migrations []Migration, history []HistoryEntry) error {
	byVersion := make(map[int64]Migration, len(migrations))
	for _, migration := range migrations {
		byVersion[migration.Version] = migration
	}
	for i, entry := range history {
		want := int64(i + 1)
		if entry.Version != want {
			return fmt.Errorf("migration history has a gap: expected version %06d, found %06d", want, entry.Version)
		}
		migration, ok := byVersion[entry.Version]
		if !ok {
			return fmt.Errorf("applied migration V%06d (%s) is missing or renamed", entry.Version, entry.Name)
		}
		if migration.Name != entry.Name {
			return fmt.Errorf("applied migration V%06d was renamed: history=%s file=%s", entry.Version, entry.Name, migration.Name)
		}
		if migration.Checksum != entry.Checksum {
			return fmt.Errorf("checksum drift for applied migration V%06d (%s)", entry.Version, entry.Name)
		}
	}
	return nil
}
