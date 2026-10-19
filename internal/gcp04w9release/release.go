// Package gcp04w9release prepares the exact W8 login release. Dry-run is the
// only default path; writes require a separate, signed-off release document.
package gcp04w9release

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	ProjectID    = "espacigo-testprod"
	Instance     = "espacigo-testprod-pg"
	Region       = "southamerica-west1"
	Connection   = "espacigo-testprod:southamerica-west1:espacigo-testprod-pg"
	PrivateIP    = "10.189.0.3"
	Database     = "espacigo"
	Login        = "espacigo_gcp04_w8_catalog_probe"
	Role         = "pg_read_all_settings"
	SecretID     = "espacigo-gcp04-w8-catalog-probe-password"
	SecretParent = "projects/espacigo-testprod/secrets/espacigo-gcp04-w8-catalog-probe-password"
	QuerySHA256  = "3fc825934660bbfc965e8d97b2ef7f4648baf2cded6c8871ae6953bd18c70d16"
)

var ErrCollision = errors.New("exact release resource collision; refusing update/reset")

type InstanceMetadata struct {
	Project        string `json:"project"`
	Name           string `json:"name"`
	Region         string `json:"region"`
	Version        string `json:"database_version"`
	State          string `json:"state"`
	Connection     string `json:"connection_name"`
	IP             string `json:"private_ip"`
	Tier           string `json:"tier"`
	DiskType       string `json:"disk_type"`
	DiskSizeGB     int64  `json:"disk_size_gb"`
	Availability   string `json:"availability_type"`
	PrivateNetwork string `json:"private_network"`
	IPv4Enabled    bool   `json:"ipv4_enabled"`
}
type UserMetadata struct {
	Name             string   `json:"name"`
	Type             string   `json:"type"`
	Host             string   `json:"host,omitempty"`
	Instance         string   `json:"instance,omitempty"`
	Project          string   `json:"project,omitempty"`
	Kind             string   `json:"kind,omitempty"`
	ETag             string   `json:"etag,omitempty"`
	DualPasswordType string   `json:"dual_password_type,omitempty"`
	IAMStatus        string   `json:"iam_status,omitempty"`
	DatabaseRoles    []string `json:"database_roles,omitempty"`
}
type VersionMetadata struct {
	Name                           string `json:"name"`
	State                          string `json:"state"`
	CreateTime                     string `json:"create_time,omitempty"`
	DestroyTime                    string `json:"destroy_time,omitempty"`
	ScheduledDestroyTime           string `json:"scheduled_destroy_time,omitempty"`
	ETag                           string `json:"etag,omitempty"`
	ClientSpecifiedPayloadChecksum bool   `json:"client_specified_payload_checksum,omitempty"`
}
type SecretMetadata struct {
	Name        string            `json:"name"`
	Replication string            `json:"replication"`
	Replicas    []string          `json:"replicas"`
	Labels      map[string]string `json:"labels,omitempty"`
	CreateTime  string            `json:"create_time,omitempty"`
	ETag        string            `json:"etag,omitempty"`
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
	CreateSecret(context.Context, string) error
	AddVersion(context.Context, string, []byte) (string, error)
	CreateUser(context.Context, string, []string, []byte) (string, error)
	WaitOperation(context.Context, string) error
	DestroyVersion(context.Context, string) error
	DeleteSecretIfOwned(context.Context, string) error
}
type Plan struct {
	Mode              string       `json:"mode"`
	Inventory         Inventory    `json:"inventory"`
	ProposedUser      UserMetadata `json:"proposed_user"`
	SecretResource    string       `json:"secret_resource"`
	SecretShellAction string       `json:"secret_shell_action"`
	SecretReplication string       `json:"secret_replication"`
	PayloadPolicy     string       `json:"payload_policy"`
	Mutations         string       `json:"mutations"`
	BlockReason       string       `json:"block_reason,omitempty"`
}
type Release struct {
	Decision           string    `json:"decision"`
	ReleaseID          string    `json:"release_id"`
	Reviewer           string    `json:"reviewer"`
	ExpiresAt          time.Time `json:"expires_at"`
	Target             string    `json:"target"`
	Roles              []string  `json:"roles"`
	Experiment         string    `json:"experiment"`
	MaxDurationSeconds int       `json:"max_duration_seconds"`
	Runs               int       `json:"runs"`
	QuerySHA256        string    `json:"query_sha256"`
}
type JournalRecord struct{ ReleaseID, Phase, SecretResource, Version, Operation, UpdatedAt string }
type Journal interface{ Save(JournalRecord) error }
type LocalJournal struct{ Dir string }

func (j LocalJournal) Save(r JournalRecord) error {
	if j.Dir == "" || !safeID(r.ReleaseID) {
		return errors.New("private journal directory required")
	}
	if err := os.MkdirAll(j.Dir, 0700); err != nil {
		return err
	}
	if err := os.Chmod(j.Dir, 0700); err != nil {
		return err
	}
	p := filepath.Join(j.Dir, r.ReleaseID+".json")
	b := []byte(fmt.Sprintf("{\"release_id\":%q,\"phase\":%q,\"secret_resource\":%q,\"version\":%q,\"operation\":%q,\"updated_at\":%q}\n", r.ReleaseID, r.Phase, r.SecretResource, r.Version, r.Operation, r.UpdatedAt))
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := f.Chmod(0600); err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		return err
	}
	return f.Sync()
}
func ReadRelease(path string) (Release, error) {
	var r Release
	b, e := os.ReadFile(path)
	if e != nil {
		return r, e
	} // Deliberately use strict JSON and reject permissive/trailing content.
	d := jsonDecoder(b)
	if e = d(&r); e != nil {
		return r, e
	}
	return r, nil
}
func Collect(ctx context.Context, r Reader) (Inventory, error) {
	if r == nil {
		return Inventory{}, errors.New("metadata reader unavailable")
	}
	var x Inventory
	var e error
	if x.Instance, e = r.ReadInstance(ctx); e != nil {
		return x, fmt.Errorf("instance metadata unavailable: %w", e)
	}
	if x.Users, e = r.ListUsers(ctx); e != nil {
		return x, fmt.Errorf("complete SQL user inventory unavailable: %w", e)
	}
	if x.Secret, e = r.ReadSecret(ctx); e != nil {
		return x, fmt.Errorf("global secret metadata unavailable: %w", e)
	}
	if x.Versions, e = r.ListVersions(ctx); e != nil {
		return x, fmt.Errorf("complete secret version inventory unavailable: %w", e)
	}
	return x, nil
}
func BuildPlan(x Inventory) (Plan, error) {
	want := InstanceMetadata{Project: ProjectID, Name: Instance, Region: Region, Version: "POSTGRES_18", State: "RUNNABLE", Connection: Connection, IP: PrivateIP, Tier: "db-n4-standard-2", DiskType: "PD_SSD", DiskSizeGB: 20, Availability: "ZONAL", PrivateNetwork: "projects/espacigo-testprod/global/networks/espacigo-testprod", IPv4Enabled: false}
	if x.Instance.Project != want.Project || x.Instance.Name != want.Name || x.Instance.Region != want.Region || x.Instance.Version != want.Version || x.Instance.State != want.State || x.Instance.Connection != want.Connection || x.Instance.IP != want.IP || x.Instance.Tier != want.Tier || x.Instance.DiskSizeGB != want.DiskSizeGB || x.Instance.PrivateNetwork != want.PrivateNetwork || x.Instance.IPv4Enabled != want.IPv4Enabled {
		return Plan{}, errors.New("pinned Cloud SQL metadata drift")
	}
	for _, u := range x.Users {
		if u.Name == Login {
			return Plan{}, ErrCollision
		}
	}
	secretMissing := x.Secret.Name == "" && x.Secret.Replication == "" && len(x.Secret.Replicas) == 0
	secretExact := x.Secret.Name == SecretParent && x.Secret.Replication == "USER_MANAGED" && len(x.Secret.Replicas) == 1 && x.Secret.Replicas[0] == Region
	if !secretMissing && !secretExact {
		return Plan{}, errors.New("global secret metadata or replica policy mismatch")
	}
	if len(x.Versions) != 0 {
		return Plan{}, errors.New("secret version collision; refusing rotation")
	}
	return Plan{Mode: "dry_run", Inventory: x, ProposedUser: UserMetadata{Name: Login, Type: "BUILT_IN", DatabaseRoles: []string{Role}}, SecretResource: SecretParent, SecretShellAction: "create only if absent; pre-existing resource blocks execute", SecretReplication: "USER_MANAGED: southamerica-west1 only", PayloadPolicy: "random bytes generated only in process memory; never output, journal, Terraform state, args or logs", Mutations: "none"}, nil
}
func Execute(ctx context.Context, inv Inventory, release Release, m Mutator, j Journal, now time.Time, random io.Reader) (string, error) {
	if release.Decision != "approved" || release.Reviewer != "HernanEspinozaDev" || !safeID(release.ReleaseID) || release.Target != ProjectID+"/"+Instance || (!now.Before(release.ExpiresAt) || release.ExpiresAt.Sub(now) > 30*time.Minute) || len(release.Roles) != 1 || release.Roles[0] != Role || release.Experiment != "one_temporary_cloudsql_role_and_one_w8_probe" || release.MaxDurationSeconds != 1800 || release.Runs != 1 || release.QuerySHA256 != QuerySHA256 {
		return "", errors.New("release document is not an active exact reviewer approval")
	}
	if m == nil || j == nil {
		return "", errors.New("mutator and private journal required")
	}
	if _, e := BuildPlan(inv); e != nil {
		return "", e
	}
	if inv.Secret.Name != "" {
		return "", ErrCollision
	}
	if random == nil {
		random = rand.Reader
	}
	raw := make([]byte, 32)
	if _, e := io.ReadFull(random, raw); e != nil {
		return "", errors.New("secure password generation failed")
	}
	defer zero(raw)
	password := []byte(base64.RawURLEncoding.EncodeToString(raw))
	defer zero(password)
	stamp := func(phase, ver, op string) error {
		return j.Save(JournalRecord{ReleaseID: release.ReleaseID, Phase: phase, SecretResource: SecretParent, Version: ver, Operation: op, UpdatedAt: time.Now().UTC().Format(time.RFC3339)})
	}
	createdSecret := inv.Secret.Name == ""
	if createdSecret {
		if e := stamp("secret_shell_create_started", "", ""); e != nil {
			return "", e
		}
		if e := m.CreateSecret(ctx, release.ReleaseID); e != nil {
			_ = stamp("secret_shell_create_uncertain", "", "")
			return "", errors.New("secret shell creation failed or uncertain; reconcile exact resource before retry")
		}
		if e := stamp("secret_shell_created", "", ""); e != nil {
			return "", e
		}
	} else if e := stamp("secret_shell_preexisting_exact", "", ""); e != nil {
		return "", e
	}
	if e := stamp("secret_version_add_started", "", ""); e != nil {
		return "", e
	}
	ver, e := m.AddVersion(ctx, SecretParent, password)
	if e != nil {
		_ = stamp("secret_version_add_uncertain", "", "")
		return "", errors.New("secret version result uncertain; retain release journal and reconcile before retry")
	}
	if !numericVersion(ver) {
		return "", errors.New("Secret Manager returned nonnumeric version; reconcile without retry")
	}
	if e = stamp("secret_version_created", ver, ""); e != nil {
		return "", e
	}
	if e = stamp("user_insert_started", ver, ""); e != nil {
		return "", e
	}
	op, e := m.CreateUser(ctx, Login, []string{Role}, password)
	if e != nil {
		if errors.Is(e, ErrDefinitive) {
			if ce := compensateSecret(ctx, m, release.ReleaseID, ver, createdSecret); ce != nil {
				_ = stamp("compensation_uncertain", ver, "")
				return "", ce
			}
			_ = stamp("compensated", ver, "")
			return "", errors.New("user creation definitively failed; exact newly-created secret resources compensated")
		}
		_ = stamp("user_creation_uncertain", ver, "")
		return "", errors.New("user creation uncertain; retain exact resources and reconcile; no retry")
	}
	if strings.TrimSpace(op) == "" {
		_ = stamp("user_creation_uncertain", ver, "")
		return "", errors.New("user operation identity absent; reconcile without retry")
	}
	if e = stamp("user_operation_pending", ver, op); e != nil {
		return "", e
	}
	if e = m.WaitOperation(ctx, op); e != nil {
		if errors.Is(e, ErrDefinitive) {
			if ce := compensateSecret(ctx, m, release.ReleaseID, ver, createdSecret); ce != nil {
				_ = stamp("operation_compensation_uncertain", ver, op)
				return "", ce
			}
			_ = stamp("operation_rejected_compensated", ver, op)
			return "", errors.New("Cloud SQL rejected the user operation; exact newly-created secret resources compensated")
		}
		_ = stamp("user_operation_uncertain", ver, op)
		return "", errors.New("user operation incomplete; reconcile before retry or compensation")
	}
	if e = stamp("complete", ver, op); e != nil {
		return "", e
	}
	return ver, nil
}

var ErrDefinitive = errors.New("definitive API rejection")

func compensateSecret(ctx context.Context, m Mutator, releaseID, version string, createdShell bool) error {
	if err := m.DestroyVersion(ctx, version); err != nil {
		return errors.New("exact created secret version compensation uncertain")
	}
	if createdShell {
		if err := m.DeleteSecretIfOwned(ctx, releaseID); err != nil {
			return errors.New("version compensated; run-owned secret shell cleanup uncertain")
		}
	}
	return nil
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
func numericVersion(v string) bool {
	if v == "" {
		return false
	}
	for _, c := range v {
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

// Keep release parsing strict without exposing any values on errors.
func jsonDecoder(b []byte) func(any) error {
	return func(v any) error {
		d := json.NewDecoder(strings.NewReader(string(b)))
		d.DisallowUnknownFields()
		if e := d.Decode(v); e != nil {
			return e
		}
		var tail any
		if e := d.Decode(&tail); !errors.Is(e, io.EOF) {
			return errors.New("trailing or invalid JSON content")
		}
		return nil
	}
}
