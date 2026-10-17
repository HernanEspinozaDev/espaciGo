package gcp03restore

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
)

const ManifestFormat = "espacigo-gcp03-cloud-restore-v1"

var shaPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
var runIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{5,11}$`)

// Manifest binds one non-repeatable Cloud restore run to exact synthetic bytes.
// Its own SHA256 is pinned in the reviewed Terraform inputs.
type Manifest struct {
	Format          string `json:"format"`
	RunID           string `json:"run_id"`
	Database        string `json:"database"`
	DumpObject      string `json:"dump_object"`
	DumpBytes       int64  `json:"dump_bytes"`
	DumpSHA256      string `json:"dump_sha256"`
	TOCObject       string `json:"toc_object"`
	TOCSHA256       string `json:"toc_sha256"`
	AllowlistObject string `json:"allowlist_object"`
	AllowlistSHA256 string `json:"allowlist_sha256"`
	MigrationCount  int    `json:"migration_count"`
	MigrationSHA256 string `json:"migration_inventory_sha256"`
}

func DecodeManifest(data []byte, expectedSHA string) (Manifest, error) {
	var m Manifest
	if !shaPattern.MatchString(expectedSHA) || digest(data) != expectedSHA {
		return m, errors.New("restore manifest hash does not match reviewed input")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return m, errors.New("restore manifest is invalid")
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return m, errors.New("restore manifest contains trailing values")
	}
	if m.Format != ManifestFormat || !runIDPattern.MatchString(m.RunID) || m.Database != "espacigo_scratch" ||
		m.DumpBytes <= 0 || !shaPattern.MatchString(m.DumpSHA256) || !shaPattern.MatchString(m.TOCSHA256) ||
		!shaPattern.MatchString(m.AllowlistSHA256) || m.MigrationCount != 61 || !shaPattern.MatchString(m.MigrationSHA256) {
		return m, errors.New("restore manifest fields do not match the fixed GCP-03 contract")
	}
	for _, object := range []string{m.DumpObject, m.TOCObject, m.AllowlistObject} {
		if object != "runs/"+m.RunID+"/synthetic-v1-v61.dump" && object != "runs/"+m.RunID+"/dump.toc" && object != "runs/"+m.RunID+"/allowlist.toc" {
			return m, errors.New("restore manifest contains an unexpected object path")
		}
	}
	if m.DumpObject != "runs/"+m.RunID+"/synthetic-v1-v61.dump" || m.TOCObject != "runs/"+m.RunID+"/dump.toc" || m.AllowlistObject != "runs/"+m.RunID+"/allowlist.toc" {
		return m, errors.New("restore manifest objects must use the exact run-scoped names")
	}
	return m, nil
}

func VerifyDump(data []byte, m Manifest) error {
	if int64(len(data)) != m.DumpBytes || digest(data) != m.DumpSHA256 {
		return errors.New("synthetic dump bytes do not match the reviewed manifest")
	}
	return nil
}

func VerifyTOC(raw, allowlist []byte, m Manifest) error {
	if digest(raw) != m.TOCSHA256 || digest(allowlist) != m.AllowlistSHA256 {
		return errors.New("dump TOC or restore allowlist hash does not match the reviewed manifest")
	}
	return VerifyTOCEntries(raw, allowlist)
}

// VerifyTOCEntries validates the allowlist against a pg_restore listing. Header
// comments are intentionally ignored because pg_restore embeds environment
// dependent creation time and timezone there. Every non-comment entry remains
// byte-for-byte and order sensitive.
func VerifyTOCEntries(raw, allowlist []byte) error {
	available := make(map[string]struct{})
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, ";") {
			available[line] = struct{}{}
		}
	}
	seen := make(map[string]struct{})
	for _, line := range strings.Split(string(allowlist), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}
		if preinstalledExtensionTOCEntry(line) {
			return errors.New("restore allowlist includes a preinstalled extension object")
		}
		if _, ok := available[line]; !ok {
			return fmt.Errorf("restore allowlist includes an entry absent from dump TOC")
		}
		if _, ok := seen[line]; ok {
			return errors.New("restore allowlist contains a duplicate TOC entry")
		}
		seen[line] = struct{}{}
	}
	if len(seen) == 0 {
		return errors.New("restore allowlist is empty")
	}
	return nil
}

func SameTOCEntries(left, right []byte) bool {
	entries := func(data []byte) []string {
		var result []string
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, ";") {
				result = append(result, line)
			}
		}
		return result
	}
	return equalStrings(entries(left), entries(right))
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func preinstalledExtensionTOCEntry(line string) bool {
	if strings.Contains(line, " EXTENSION - ") || strings.Contains(line, " COMMENT - EXTENSION ") {
		return true
	}
	fields := strings.Fields(line)
	return len(fields) >= 7 && fields[3] == "TABLE" && fields[4] == "DATA" && fields[5] == "public" && fields[6] == "spatial_ref_sys"
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
