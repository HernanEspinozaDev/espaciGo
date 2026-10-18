package gcp04credential

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	ProjectID       = "espacigo-testprod"
	ProjectNumber   = "447373903153"
	Region          = "southamerica-west1"
	RunID           = "g04replay18"
	InstanceName    = "espacigo-gcp04-g04replay18-pg"
	SecretName      = "espacigo-gcp04-g04replay18-db-owner-password"
	SecretParent    = "projects/espacigo-testprod/secrets/espacigo-gcp04-g04replay18-db-owner-password"
	StateBucket     = "espacigo-testprod-tfstate-447373903153-gcp"
	StateObject     = "espacigo/testprod/gcp04-replay-smoke/g04replay18/default.tfstate"
	PlanPath        = ".local/testing/gcp04-g04replay18/base.plan"
	ReleasePath     = ".local/testing/gcp04-g04replay18/credential-release.json"
	JournalPath     = ".local/testing/gcp04-g04replay18/credential-journal.json"
	CredentialOp    = "set-postgres-owner-password-and-create-secret-version"
	DatabaseOwner   = "postgres"
	ExpectedSQLNet  = "https://www.googleapis.com/compute/v1/projects/espacigo-testprod/global/networks/espacigo-testprod"
	ExpectedPlanAdd = 9
)

var digestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

// Release binds one credential operation to the reviewed project, run, target,
// Terraform state object and saved base plan. It contains no credential data.
type Release struct {
	SchemaVersion int          `json:"schema_version"`
	Decision      string       `json:"decision"`
	ReleaseID     string       `json:"release_id"`
	Reviewer      string       `json:"reviewer"`
	IssuedAt      string       `json:"issued_at"`
	ExpiresAt     string       `json:"expires_at"`
	Operation     string       `json:"operation"`
	ProjectID     string       `json:"project_id"`
	ProjectNumber string       `json:"project_number"`
	Region        string       `json:"region"`
	RunID         string       `json:"run_id"`
	Instance      string       `json:"instance_name"`
	Secret        string       `json:"secret_name"`
	State         StateRelease `json:"state"`
	PlanSHA256    string       `json:"plan_sha256"`
}

type StateRelease struct {
	Bucket     string `json:"bucket"`
	Object     string `json:"object"`
	Lineage    string `json:"lineage"`
	Serial     uint64 `json:"serial"`
	Generation int64  `json:"generation"`
	SHA256     string `json:"sha256"`
}

type Preview struct {
	ReleaseSHA256 string `json:"release_sha256"`
	ReleaseID     string `json:"release_id"`
	Operation     string `json:"operation"`
	ProjectID     string `json:"project_id"`
	RunID         string `json:"run_id"`
	Instance      string `json:"instance_name"`
	Secret        string `json:"secret_name"`
	StateLineage  string `json:"state_lineage"`
	StateSerial   uint64 `json:"state_serial"`
	PlanSHA256    string `json:"plan_sha256"`
	CloudCalls    string `json:"cloud_calls"`
	SecretCreated string `json:"secret_generated"`
}

func ParseRelease(data []byte, now time.Time) (Release, string, error) {
	var release Release
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&release); err != nil {
		return Release{}, "", errors.New("credential release file is invalid JSON")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return Release{}, "", errors.New("credential release file has trailing data")
	}
	if err := validateRelease(release, now); err != nil {
		return Release{}, "", err
	}
	sum := sha256.Sum256(data)
	return release, hex.EncodeToString(sum[:]), nil
}

func validateRelease(r Release, now time.Time) error {
	if r.SchemaVersion != 1 || r.Decision != "released" || r.Reviewer != "HernanEspinozaDev" ||
		r.Operation != CredentialOp || r.ProjectID != ProjectID || r.ProjectNumber != ProjectNumber ||
		r.Region != Region || r.RunID != RunID || r.Instance != InstanceName || r.Secret != SecretName ||
		r.State.Bucket != StateBucket || r.State.Object != StateObject {
		return errors.New("credential release does not match the fixed GCP-04 target")
	}
	if strings.TrimSpace(r.ReleaseID) == "" || len(r.ReleaseID) > 128 {
		return errors.New("credential release identifier is invalid")
	}
	issued, err := time.Parse(time.RFC3339, r.IssuedAt)
	if err != nil || issued.After(now.Add(time.Minute)) {
		return errors.New("credential release issue time is invalid")
	}
	expires, err := time.Parse(time.RFC3339, r.ExpiresAt)
	if err != nil || !now.Before(expires) || !expires.After(issued) || expires.Sub(issued) > 24*time.Hour {
		return errors.New("credential release is expired or outside its bounded validity")
	}
	if r.State.Lineage == "" || r.State.Serial < 1 || r.State.Generation < 1 ||
		!digestPattern.MatchString(r.State.SHA256) || !digestPattern.MatchString(r.PlanSHA256) {
		return errors.New("credential release state or plan hash is incomplete")
	}
	return nil
}

func ReadPrivateRelease(path string, now time.Time) (Release, string, []byte, error) {
	if filepath.Clean(path) != ReleasePath {
		return Release{}, "", nil, errors.New("credential release path must be the fixed private run path")
	}
	data, err := readPrivateFile(path)
	if err != nil {
		return Release{}, "", nil, err
	}
	release, digest, err := ParseRelease(data, now)
	if err != nil {
		return Release{}, "", nil, err
	}
	return release, digest, data, nil
}

func VerifyReleaseHash(releaseHash, confirmation string) error {
	if !digestPattern.MatchString(releaseHash) || releaseHash != confirmation {
		return errors.New("execution requires the exact SHA-256 confirmation for this release file")
	}
	return nil
}

func BuildPreview(r Release, digest string) Preview {
	return Preview{
		ReleaseSHA256: digest,
		ReleaseID:     r.ReleaseID,
		Operation:     r.Operation,
		ProjectID:     r.ProjectID,
		RunID:         r.RunID,
		Instance:      r.Instance,
		Secret:        r.Secret,
		StateLineage:  r.State.Lineage,
		StateSerial:   r.State.Serial,
		PlanSHA256:    r.PlanSHA256,
		CloudCalls:    "none",
		SecretCreated: "no",
	}
}

func readPrivateFile(path string) ([]byte, error) {
	dir := filepath.Dir(path)
	dirInfo, err := os.Lstat(dir)
	if err != nil || !dirInfo.IsDir() || dirInfo.Mode()&os.ModeSymlink != 0 || dirInfo.Mode().Perm() != 0700 {
		return nil, errors.New("private evidence directory must exist with mode 0700")
	}
	before, err := os.Lstat(path)
	if err != nil || before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() || before.Mode().Perm() != 0600 {
		return nil, errors.New("private evidence file must be a regular mode-0600 file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.New("could not open private evidence file")
	}
	defer file.Close()
	after, err := file.Stat()
	if err != nil || !os.SameFile(before, after) {
		return nil, errors.New("private evidence file changed while opening")
	}
	data, err := io.ReadAll(io.LimitReader(file, 1<<20))
	if err != nil || len(data) == 1<<20 {
		return nil, errors.New("private evidence file could not be read within its size limit")
	}
	return data, nil
}

func (r Release) String() string {
	return fmt.Sprintf("release=%s project=%s run=%s instance=%s secret=%s", r.ReleaseID, r.ProjectID, r.RunID, r.Instance, r.Secret)
}
