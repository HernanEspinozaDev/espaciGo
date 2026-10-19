package gcp04w7b1

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	ErrCollision = errors.New("B1 exact resource collision; refusing adoption or reset")
	ErrUncertain = errors.New("B1 API outcome uncertain; reconcile; do not retry")
	ErrLease     = errors.New("B1 lease exists; reconcile before any new stage")
)

type InstanceMetadata struct {
	Project    string `json:"project"`
	Name       string `json:"name"`
	Region     string `json:"region"`
	Version    string `json:"database_version"`
	State      string `json:"state"`
	Connection string `json:"connection_name"`
	IP         string `json:"private_ip"`
}
type UserMetadata struct {
	Name  string   `json:"name"`
	Type  string   `json:"type"`
	Roles []string `json:"database_roles"`
}
type SecretMetadata struct {
	Name        string            `json:"name"`
	ETag        string            `json:"etag"`
	Replication string            `json:"replication"`
	Replicas    []string          `json:"replicas"`
	Labels      map[string]string `json:"labels"`
}
type VersionMetadata struct {
	Name   string `json:"name"`
	Number string `json:"number"`
	State  string `json:"state"`
	ETag   string `json:"etag"`
}
type Inventory struct {
	Instance InstanceMetadata  `json:"instance"`
	Users    []UserMetadata    `json:"users"`
	Secret   SecretMetadata    `json:"secret"`
	Versions []VersionMetadata `json:"versions"`
}

type Reader interface {
	ReadInstance(context.Context) (InstanceMetadata, error)
	ListUsers(context.Context) ([]UserMetadata, error)
	ReadSecret(context.Context) (SecretMetadata, error)
	ListVersions(context.Context) ([]VersionMetadata, error)
}
type Mutator interface {
	CreateSecret(context.Context, string) (SecretMetadata, error)
	AddVersion(context.Context, string, []byte) (VersionMetadata, error)
	InsertUser(context.Context, string, string, []string, []byte) (string, error)
	WaitOperation(context.Context, string) error
}
type CleanupMutator interface {
	DeleteUser(context.Context, string) (string, error)
	DestroyVersion(context.Context, string, string) error
	DeleteSecret(context.Context, string, string, string) error
}
type JournalRecord struct {
	ReleaseID   string `json:"release_id"`
	Stage       string `json:"stage"`
	Phase       string `json:"phase"`
	Target      string `json:"target"`
	Login       string `json:"login"`
	Secret      string `json:"secret"`
	SecretETag  string `json:"secret_etag,omitempty"`
	Version     string `json:"version,omitempty"`
	VersionETag string `json:"version_etag,omitempty"`
	Operation   string `json:"operation,omitempty"`
	At          string `json:"at"`
}
type Journal interface{ Save(JournalRecord) error }
type AppendJournal struct{ Dir string }

func (j AppendJournal) Save(r JournalRecord) error {
	if !safeID(r.ReleaseID) || j.Dir == "" {
		return errors.New("private journal path and release ID required")
	}
	if err := os.MkdirAll(j.Dir, 0700); err != nil {
		return err
	}
	if err := os.Chmod(j.Dir, 0700); err != nil {
		return err
	}
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	p := filepath.Join(j.Dir, r.ReleaseID+".jsonl")
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err = f.Chmod(0600); err != nil {
		return err
	}
	if _, err = f.Write(b); err != nil {
		return err
	}
	return f.Sync()
}

type Release struct {
	Decision           string    `json:"decision"`
	Stage              string    `json:"stage"`
	ReleaseID          string    `json:"release_id"`
	Reviewer           string    `json:"reviewer"`
	ExpiresAt          time.Time `json:"expires_at"`
	Target             string    `json:"target"`
	Login              string    `json:"login"`
	Role               string    `json:"database_role"`
	SecretID           string    `json:"secret_id"`
	ResourceReleaseID  string    `json:"resource_release_id,omitempty"`
	QuerySHA256        string    `json:"query_sha256"`
	Runs               int       `json:"runs"`
	MaxDurationSeconds int       `json:"max_duration_seconds"`
	SecretETag         string    `json:"secret_etag,omitempty"`
	Version            string    `json:"secret_version,omitempty"`
	VersionETag        string    `json:"secret_version_etag,omitempty"`
}
type Plan struct {
	Format         string           `json:"format"`
	Mode           string           `json:"mode"`
	Target         InstanceMetadata `json:"target"`
	Login          string           `json:"login"`
	UserRequest    UserMetadata     `json:"user_request_proposal"`
	RoleDecision   string           `json:"role_decision"`
	SecretResource string           `json:"secret_resource"`
	Replication    string           `json:"replication"`
	PayloadPolicy  string           `json:"payload_policy"`
	QuerySHA256    string           `json:"query_sha256"`
	Mutations      string           `json:"mutations"`
}

func StaticPlan() Plan {
	return Plan{Format: "espacigo.gcp04.w7.b1-plan.v1", Mode: "default_off_preview", Target: expectedInstance(), Login: Bootstrap, UserRequest: UserMetadata{Name: Bootstrap, Type: "BUILT_IN", Roles: []string{AdminRole}}, RoleDecision: "explicit_only: cloudsqlsuperuser; no omission/default-role fallback", SecretResource: secretParent(), Replication: "USER_MANAGED: southamerica-west1 only", PayloadPolicy: "generated in process memory; same bytes sent to exact exclusive secret version and user insert; never output, args, journal, Terraform state or logs", QuerySHA256: QuerySHA256, Mutations: "none"}
}

func Collect(ctx context.Context, r Reader) (Inventory, error) {
	if r == nil {
		return Inventory{}, errors.New("read-only B1 metadata reader required")
	}
	var x Inventory
	var err error
	if x.Instance, err = r.ReadInstance(ctx); err != nil {
		return x, errors.New("B1 instance metadata unavailable")
	}
	if x.Users, err = r.ListUsers(ctx); err != nil {
		return x, errors.New("complete B1 user inventory unavailable")
	}
	if x.Secret, err = r.ReadSecret(ctx); err != nil {
		return x, errors.New("B1 secret metadata unavailable")
	}
	if x.Versions, err = r.ListVersions(ctx); err != nil {
		return x, errors.New("B1 secret version inventory unavailable")
	}
	return x, nil
}

func BuildPlan(x Inventory) (Plan, error) {
	if x.Instance != expectedInstance() {
		return Plan{}, errors.New("B1 target metadata drift")
	}
	for _, u := range x.Users {
		if u.Name == Bootstrap || u.Name == Observer || u.Name == "postgres" {
			if u.Name == Bootstrap {
				return Plan{}, ErrCollision
			}
		}
	}
	if x.Secret.Name != "" || x.Secret.ETag != "" || x.Secret.Replication != "" || len(x.Secret.Replicas) > 0 || len(x.Secret.Labels) > 0 || len(x.Versions) > 0 {
		return Plan{}, ErrCollision
	}
	return StaticPlan(), nil
}

func ReconcileInventory(x Inventory, resourceRelease string, version string) (map[string]any, error) {
	if x.Instance != expectedInstance() || !safeID(resourceRelease) || !numericVersion(version) {
		return nil, errors.New("B1 reconciliation target or resource identity invalid")
	}
	userPresent := false
	userExact := false
	for _, u := range x.Users {
		if u.Name == Bootstrap {
			if userPresent {
				return nil, ErrCollision
			}
			userPresent = true
			userExact = u.Type == "BUILT_IN" && len(u.Roles) == 1 && u.Roles[0] == AdminRole
		}
	}
	secretPresent := x.Secret.Name != ""
	secretExact := secretPresent && validSecret(x.Secret, resourceRelease)
	versionFound := false
	versionExact := false
	for _, v := range x.Versions {
		if v.Number == version {
			if versionFound {
				return nil, ErrCollision
			}
			versionFound = true
			versionExact = isSecretVersion(suffixParent(v.Name), version) && v.ETag != ""
		}
	}
	status := "incomplete_or_mixed_state"
	switch {
	case !userPresent && !secretPresent && len(x.Versions) == 0:
		status = "resources_absent"
	case !userPresent && secretExact && len(x.Versions) == 1 && versionFound && versionExact:
		status = "user_absent_secret_version_requires_explicit_cleanup"
	case userExact && secretExact && len(x.Versions) == 1 && versionFound && versionExact && x.Versions[0].State == "ENABLED":
		status = "bootstrap_actor_created_probe_not_accredited"
	case userPresent && !userExact:
		status = "user_collision_do_not_adopt_or_delete"
	case secretPresent && !secretExact:
		status = "secret_collision_do_not_adopt_or_delete"
	}
	return map[string]any{"format": "espacigo.gcp04.w7.b1-reconciliation.v1", "status": status, "target": x.Instance, "bootstrap_login": Bootstrap, "database_role_requested": AdminRole, "secret_resource": x.Secret.Name, "secret_version": version, "mutation_authorized": false}, nil
}

func validRelease(r Release, stage string, now time.Time) bool {
	resourceRelease := r.ResourceReleaseID
	if stage == "b1_create" && resourceRelease == "" {
		resourceRelease = r.ReleaseID
	}
	return r.Decision == "approved" && r.Stage == stage && r.Reviewer == "HernanEspinozaDev" && safeID(r.ReleaseID) && safeID(resourceRelease) && r.ExpiresAt.After(now) && r.ExpiresAt.Sub(now) <= 30*time.Minute && r.Target == ProjectID+"/"+Instance && r.Login == Bootstrap && r.Role == AdminRole && r.SecretID == SecretID && r.QuerySHA256 == QuerySHA256 && r.Runs == 1 && r.MaxDurationSeconds > 0 && r.MaxDurationSeconds <= 1800 && (stage != "b1_probe" || (r.Version != "" && r.VersionETag != "" && r.SecretETag != ""))
}

func ValidateRelease(r Release, stage string, now time.Time) error {
	if !validRelease(r, stage, now) {
		return errors.New("no active exact B1 release for requested stage")
	}
	return nil
}

func ParseRelease(data []byte, confirmation, stage string, now time.Time) (Release, error) {
	var r Release
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		return Release{}, errors.New("B1 release file invalid")
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		return Release{}, errors.New("B1 release file has trailing data")
	}
	if ReleaseDigest(data) != strings.ToLower(strings.TrimSpace(confirmation)) {
		return Release{}, errors.New("B1 release file digest confirmation mismatch")
	}
	if err := ValidateRelease(r, stage, now); err != nil {
		return Release{}, err
	}
	return r, nil
}

// AcquireLease is create-exclusive. Expired leases are never stolen; reconcile
// the journal and target first. The returned release function removes only the
// exact lease created by this process.
func AcquireLease(dir, releaseID string, now time.Time) error {
	if !safeID(releaseID) || dir == "" {
		return errors.New("private lease directory and release ID required")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return err
	}
	p := filepath.Join(dir, releaseID+".lease")
	f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return ErrLease
	}
	_, werr := fmt.Fprintf(f, "{\"release_id\":%q,\"acquired_at\":%q,\"expires_at\":%q}\n", releaseID, now.UTC().Format(time.RFC3339), now.Add(30*time.Minute).UTC().Format(time.RFC3339))
	if werr == nil {
		werr = f.Sync()
	}
	cerr := f.Close()
	if werr != nil || cerr != nil {
		_ = os.Remove(p)
		return errors.New("B1 lease could not be durably written")
	}
	return nil
}

func CreateB1(ctx context.Context, x Inventory, r Release, reader Reader, m Mutator, j Journal, leaseDir string, now time.Time, random io.Reader) (string, error) {
	if !validRelease(r, "b1_create", now) {
		return "", errors.New("no active exact B1-create release")
	}
	if reader == nil || m == nil || j == nil {
		return "", errors.New("B1 create dependencies unavailable")
	}
	if _, err := BuildPlan(x); err != nil {
		return "", err
	}
	err := AcquireLease(leaseDir, leaseKey(r.ReleaseID, "create"), now)
	if err != nil {
		return "", err
	}
	deadline, cancel := context.WithTimeout(ctx, time.Duration(r.MaxDurationSeconds)*time.Second)
	defer cancel()
	stamp := func(phase string, secret SecretMetadata, version VersionMetadata, op string) error {
		return j.Save(JournalRecord{ReleaseID: r.ReleaseID, Stage: r.Stage, Phase: phase, Target: r.Target, Login: r.Login, Secret: secret.Name, SecretETag: secret.ETag, Version: version.Number, VersionETag: version.ETag, Operation: op, At: time.Now().UTC().Format(time.RFC3339)})
	}
	if err = stamp("create_started", SecretMetadata{}, VersionMetadata{}, ""); err != nil {
		return "", err
	}
	secret, err := m.CreateSecret(deadline, r.ReleaseID)
	if err != nil {
		_ = stamp("secret_shell_outcome_uncertain", SecretMetadata{}, VersionMetadata{}, "")
		return "", ErrUncertain
	}
	if !validSecret(secret, r.ReleaseID) || secret.ETag == "" {
		_ = stamp("secret_shell_identity_uncertain", secret, VersionMetadata{}, "")
		return "", ErrUncertain
	}
	if err = stamp("secret_shell_created", secret, VersionMetadata{}, ""); err != nil {
		return "", ErrUncertain
	}
	if random == nil {
		random = rand.Reader
	}
	raw := make([]byte, 32)
	if _, err = io.ReadFull(random, raw); err != nil {
		return "", errors.New("B1 credential generation failed")
	}
	password := []byte(base64.RawURLEncoding.EncodeToString(raw))
	zero(raw)
	defer zero(password)
	version, err := m.AddVersion(deadline, secret.Name, password)
	if err != nil {
		_ = stamp("secret_version_outcome_uncertain", secret, VersionMetadata{}, "")
		return "", ErrUncertain
	}
	if !validVersion(version) || version.ETag == "" {
		_ = stamp("secret_version_identity_uncertain", secret, version, "")
		return "", ErrUncertain
	}
	if err = stamp("secret_version_created", secret, version, ""); err != nil {
		return "", ErrUncertain
	}
	// The administrative role is explicit in every request; empty roles are rejected.
	op, err := m.InsertUser(deadline, Bootstrap, "BUILT_IN", []string{AdminRole}, password)
	if err != nil || strings.TrimSpace(op) == "" {
		_ = stamp("user_insert_outcome_uncertain", secret, version, op)
		return "", ErrUncertain
	}
	if err = stamp("user_operation_pending", secret, version, op); err != nil {
		return "", ErrUncertain
	}
	if err = m.WaitOperation(deadline, op); err != nil {
		_ = stamp("user_operation_uncertain", secret, version, op)
		return "", ErrUncertain
	}
	users, err := reader.ListUsers(deadline)
	if err != nil {
		_ = stamp("user_presence_uncertain", secret, version, op)
		return "", ErrUncertain
	}
	if !containsExactUser(users) {
		_ = stamp("user_presence_mismatch", secret, version, op)
		return "", ErrUncertain
	}
	if err = stamp("create_complete", secret, version, op); err != nil {
		return "", ErrUncertain
	}
	return version.Number, nil
}

func containsExactUser(users []UserMetadata) bool {
	count := 0
	for _, u := range users {
		if u.Name == Bootstrap {
			count++
			if u.Type != "BUILT_IN" || len(u.Roles) != 1 || u.Roles[0] != AdminRole {
				return false
			}
		}
	}
	return count == 1
}

func inspectExactUser(users []UserMetadata) (present, exact bool) {
	for _, u := range users {
		if u.Name == Bootstrap {
			if present {
				return true, false
			}
			present = true
			exact = u.Type == "BUILT_IN" && len(u.Roles) == 1 && u.Roles[0] == AdminRole
		}
	}
	return present, exact
}

// CleanupB1 is a separate, explicitly released stage. It deletes only this
// exact login, waits for DONE, confirms absence, then destroys one ETag-pinned
// version and removes the exact run-labelled secret shell by its ETag.
func CleanupB1(ctx context.Context, x Inventory, r Release, reader Reader, m CleanupMutator, waiter interface {
	WaitOperation(context.Context, string) error
}, j Journal, leaseDir string, now time.Time) error {
	if !validRelease(r, "b1_cleanup", now) || r.Version == "" || r.VersionETag == "" || r.SecretETag == "" {
		return errors.New("no active exact B1-cleanup release with pinned version/ETags")
	}
	if reader == nil || m == nil || waiter == nil || j == nil {
		return errors.New("B1 cleanup dependencies unavailable")
	}
	if x.Instance != expectedInstance() {
		return errors.New("B1 cleanup target metadata drift")
	}
	userPresent, userExact := inspectExactUser(x.Users)
	if userPresent && !userExact {
		return errors.New("B1 cleanup exact user identity or explicit role mismatch")
	}
	if !validSecret(x.Secret, r.ResourceReleaseID) || x.Secret.ETag != r.SecretETag {
		return errors.New("B1 cleanup run-owned secret identity mismatch")
	}
	if len(x.Versions) != 1 || x.Versions[0].Number != r.Version || x.Versions[0].ETag != r.VersionETag || !validVersion(x.Versions[0]) {
		return ErrCollision
	}
	err := AcquireLease(leaseDir, leaseKey(r.ReleaseID, "cleanup"), now)
	if err != nil {
		return err
	}
	deadline, cancel := context.WithTimeout(ctx, time.Duration(r.MaxDurationSeconds)*time.Second)
	defer cancel()
	stamp := func(phase, op string) error {
		return j.Save(JournalRecord{ReleaseID: r.ReleaseID, Stage: r.Stage, Phase: phase, Target: r.Target, Login: r.Login, Secret: x.Secret.Name, SecretETag: x.Secret.ETag, Version: r.Version, VersionETag: r.VersionETag, Operation: op, At: time.Now().UTC().Format(time.RFC3339)})
	}
	op := ""
	if userPresent {
		if err = stamp("user_delete_started", ""); err != nil {
			return err
		}
		op, err = m.DeleteUser(deadline, Bootstrap)
		if err != nil || strings.TrimSpace(op) == "" {
			_ = stamp("user_delete_outcome_uncertain", op)
			return ErrUncertain
		}
		if err = waiter.WaitOperation(deadline, op); err != nil {
			_ = stamp("user_delete_operation_uncertain", op)
			return ErrUncertain
		}
		users, listErr := reader.ListUsers(deadline)
		if listErr != nil {
			_ = stamp("user_absence_uncertain", op)
			return ErrUncertain
		}
		for _, u := range users {
			if u.Name == Bootstrap {
				_ = stamp("user_absence_failed", op)
				return ErrUncertain
			}
		}
		if err = stamp("user_delete_done_absent", op); err != nil {
			return ErrUncertain
		}
	} else {
		if err = stamp("user_absent_reconciled", ""); err != nil {
			return ErrUncertain
		}
	}
	if err = m.DestroyVersion(deadline, x.Secret.Name+"/versions/"+r.Version, r.VersionETag); err != nil {
		_ = stamp("version_destroy_uncertain", op)
		return ErrUncertain
	}
	versions, err := reader.ListVersions(deadline)
	if err != nil || !containsDestroyedVersion(versions, r.Version) {
		_ = stamp("version_destroy_reconcile", op)
		return ErrUncertain
	}
	if err = stamp("version_destroyed", op); err != nil {
		return ErrUncertain
	}
	if err = m.DeleteSecret(deadline, x.Secret.Name, r.ResourceReleaseID, r.SecretETag); err != nil {
		_ = stamp("secret_shell_delete_uncertain", op)
		return ErrUncertain
	}
	secret, err := reader.ReadSecret(deadline)
	if err != nil || secret.Name != "" {
		_ = stamp("secret_shell_absence_uncertain", op)
		return ErrUncertain
	}
	if err = stamp("cleanup_complete", op); err != nil {
		return ErrUncertain
	}
	return nil
}

func containsDestroyedVersion(versions []VersionMetadata, number string) bool {
	if len(versions) != 1 {
		return false
	}
	v := versions[0]
	return v.Number == number && isSecretVersion(suffixParent(v.Name), number) && v.State == "DESTROYED" && v.ETag != ""
}

func leaseKey(releaseID, stage string) string {
	if len(releaseID) > 32 {
		releaseID = releaseID[:32]
	}
	return releaseID + "-" + stage
}

func validSecret(s SecretMetadata, run string) bool {
	return isSecretParent(s.Name) && s.Replication == "USER_MANAGED" && len(s.Replicas) == 1 && s.Replicas[0] == Region && s.Labels["release_id"] == run && s.Labels["purpose"] == "gcp04-w7-b1-bootstrap"
}
func validVersion(v VersionMetadata) bool {
	return v.Number != "" && len(v.Number) <= 19 && v.Number[0] >= '1' && v.Number[0] <= '9' && allDigits(v.Number[1:]) && isSecretVersion(suffixParent(v.Name), v.Number) && v.State == "ENABLED"
}
func secretParent() string          { return "projects/" + ProjectID + "/secrets/" + SecretID }
func secretParentCanonical() string { return "projects/447373903153/secrets/" + SecretID }
func isSecretParent(name string) bool {
	return name == secretParent() || name == secretParentCanonical()
}
func suffixParent(name string) string {
	i := strings.LastIndex(name, "/versions/")
	if i < 0 {
		return ""
	}
	return name[:i]
}
func isSecretVersion(parent, number string) bool {
	return (parent == secretParent() || parent == secretParentCanonical()) && number != "" && allDigits(number)
}
func expectedInstance() InstanceMetadata {
	return InstanceMetadata{Project: ProjectID, Name: Instance, Region: Region, Version: "POSTGRES_18", State: "RUNNABLE", Connection: Connection, IP: PrivateIP}
}
func safeID(s string) bool {
	if s == "" || len(s) > 64 {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
			return false
		}
	}
	return true
}
func allDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
func ReleaseDigest(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
