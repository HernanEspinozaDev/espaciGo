package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/gcp04w9release"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	"google.golang.org/api/secretmanager/v1"
	"google.golang.org/api/sqladmin/v1beta4"
)

const scope = "https://www.googleapis.com/auth/cloud-platform"

func quotaProjectOption() option.ClientOption {
	return option.WithQuotaProject(gcp04w9release.ProjectID)
}

type api struct {
	sql *sqladmin.Service
	sm  *secretmanager.Service
}

func safe(err error) error {
	var g *googleapi.Error
	if errors.As(err, &g) {
		return fmt.Errorf("Cloud API HTTP %d", g.Code)
	}
	return errors.New("Cloud API outcome unavailable or uncertain")
}
func (a api) ReadInstance(ctx context.Context) (gcp04w9release.InstanceMetadata, error) {
	x, e := a.sql.Instances.Get(gcp04w9release.ProjectID, gcp04w9release.Instance).Context(ctx).Do()
	if e != nil {
		return gcp04w9release.InstanceMetadata{}, safe(e)
	}
	if x == nil || x.Settings == nil || x.Settings.IpConfiguration == nil {
		return gcp04w9release.InstanceMetadata{}, errors.New("instance metadata incomplete")
	}
	ip := ""
	for _, v := range x.IpAddresses {
		if v.Type == "PRIVATE" {
			ip = v.IpAddress
		}
	}
	ipcfg := x.Settings.IpConfiguration
	privateNetwork := ""
	ipv4 := false
	if ipcfg != nil {
		privateNetwork = ipcfg.PrivateNetwork
		ipv4 = ipcfg.Ipv4Enabled
	}
	return gcp04w9release.InstanceMetadata{Project: x.Project, Name: x.Name, Region: x.Region, Version: x.DatabaseVersion, State: x.State, Connection: x.ConnectionName, IP: ip, Tier: x.Settings.Tier, DiskType: x.Settings.DataDiskType, DiskSizeGB: x.Settings.DataDiskSizeGb, Availability: x.Settings.AvailabilityType, PrivateNetwork: privateNetwork, IPv4Enabled: ipv4}, nil
}
func (a api) ListUsers(ctx context.Context) ([]gcp04w9release.UserMetadata, error) {
	x, e := a.sql.Users.List(gcp04w9release.ProjectID, gcp04w9release.Instance).Context(ctx).Do()
	if e != nil {
		return nil, safe(e)
	}
	if x == nil {
		return nil, errors.New("user inventory missing")
	}
	if x.NextPageToken != "" {
		return nil, errors.New("unexpected Cloud SQL users pagination token")
	}
	out := make([]gcp04w9release.UserMetadata, 0, len(x.Items))
	for _, u := range x.Items {
		if u != nil {
			out = append(out, gcp04w9release.UserMetadata{Name: u.Name, Type: u.Type, Host: u.Host, Instance: u.Instance, Project: u.Project, Kind: u.Kind, ETag: u.Etag, DualPasswordType: u.DualPasswordType, IAMStatus: u.IamStatus, DatabaseRoles: append([]string(nil), u.DatabaseRoles...)})
		}
	}
	return out, nil
}
func (a api) ReadSecret(ctx context.Context) (gcp04w9release.SecretMetadata, error) {
	x, e := a.sm.Projects.Secrets.Get(gcp04w9release.SecretParent).Context(ctx).Do()
	if e != nil {
		var ge *googleapi.Error
		if errors.As(e, &ge) && ge.Code == 404 {
			return gcp04w9release.SecretMetadata{}, nil
		}
		return gcp04w9release.SecretMetadata{}, safe(e)
	}
	if x == nil {
		return gcp04w9release.SecretMetadata{}, errors.New("global secret replication metadata incomplete")
	}
	if !gcp04w9release.IsSecretParent(x.Name) {
		return gcp04w9release.SecretMetadata{}, errors.New("global secret resource name is outside the exact W9 aliases")
	}
	if x.Replication == nil || x.Replication.UserManaged == nil {
		return gcp04w9release.SecretMetadata{}, errors.New("global secret replication metadata incomplete")
	}
	ls := []string{}
	for _, r := range x.Replication.UserManaged.Replicas {
		if r != nil {
			ls = append(ls, r.Location)
		}
	}
	return gcp04w9release.SecretMetadata{Name: x.Name, Replication: "USER_MANAGED", Replicas: ls, Labels: x.Labels, CreateTime: x.CreateTime, ETag: x.Etag}, nil
}
func (a api) ListVersions(ctx context.Context) ([]gcp04w9release.VersionMetadata, error) {
	var out []gcp04w9release.VersionMetadata
	token := ""
	seen := map[string]bool{}
	for page := 0; page < 1000; page++ {
		call := a.sm.Projects.Secrets.Versions.List(gcp04w9release.SecretParent).PageSize(100)
		if token != "" {
			call = call.PageToken(token)
		}
		x, e := call.Context(ctx).Do()
		if e != nil {
			var ge *googleapi.Error
			if errors.As(e, &ge) && ge.Code == 404 {
				return []gcp04w9release.VersionMetadata{}, nil
			}
			return nil, safe(e)
		}
		if x == nil {
			return nil, errors.New("version inventory incomplete")
		}
		for _, v := range x.Versions {
			if v != nil {
				if !gcp04w9release.IsSecretVersion(v.Name) {
					return nil, errors.New("version inventory contains a resource outside the exact W9 secret")
				}
				out = append(out, gcp04w9release.VersionMetadata{Name: v.Name, State: v.State, CreateTime: v.CreateTime, DestroyTime: v.DestroyTime, ScheduledDestroyTime: v.ScheduledDestroyTime, ETag: v.Etag, ClientSpecifiedPayloadChecksum: v.ClientSpecifiedPayloadChecksum})
			}
		}
		if x.NextPageToken == "" {
			return out, nil
		}
		if seen[x.NextPageToken] {
			return nil, errors.New("repeated Secret Manager page token")
		}
		seen[x.NextPageToken] = true
		token = x.NextPageToken
	}
	return nil, errors.New("version inventory exceeds page bound")
}

// Exact mutators are invoked only by the separately approved release path.
type writer struct{ a api }

func (w writer) CreateSecret(ctx context.Context, release string) error {
	req := &secretmanager.Secret{Replication: &secretmanager.Replication{UserManaged: &secretmanager.UserManaged{Replicas: []*secretmanager.Replica{{Location: gcp04w9release.Region}}}}, Labels: map[string]string{"release_id": release, "purpose": "gcp04-w9-catalog-probe"}}
	x, e := w.a.sm.Projects.Secrets.Create("projects/"+gcp04w9release.ProjectID, req).SecretId(gcp04w9release.SecretID).Context(ctx).Do()
	if e != nil {
		return safe(e)
	}
	if x == nil || !gcp04w9release.IsSecretParent(x.Name) || x.Etag == "" || x.Labels["release_id"] != release || x.Labels["purpose"] != "gcp04-w9-catalog-probe" || x.Replication == nil || x.Replication.UserManaged == nil || len(x.Replication.UserManaged.Replicas) != 1 || x.Replication.UserManaged.Replicas[0].Location != gcp04w9release.Region {
		return errors.New("created secret shell response does not match exact release")
	}
	return nil
}
func (w writer) AddVersion(ctx context.Context, parent string, p []byte) (string, error) {
	if !gcp04w9release.IsSecretParent(parent) {
		return "", errors.New("secret resource mismatch")
	}
	x, e := w.a.sm.Projects.Secrets.AddVersion(parent, &secretmanager.AddSecretVersionRequest{Payload: &secretmanager.SecretPayload{Data: base64.StdEncoding.EncodeToString(p)}}).Context(ctx).Do()
	if e != nil {
		return "", safe(e)
	}
	if x == nil || !gcp04w9release.IsSecretVersion(x.Name) {
		return "", errors.New("version response missing")
	}
	q := strings.Split(x.Name, "/")
	version := q[len(q)-1]
	if !gcp04w9release.IsSecretVersionName(x.Name, version) {
		return "", errors.New("version response resource mismatch")
	}
	return version, nil
}
func (w writer) CreateUser(ctx context.Context, name string, roles []string, p []byte) (string, error) {
	if name != gcp04w9release.Login || len(roles) != 1 || roles[0] != gcp04w9release.Role || len(p) == 0 {
		return "", errors.New("exact user request required")
	}
	op, e := w.a.sql.Users.Insert(gcp04w9release.ProjectID, gcp04w9release.Instance, &sqladmin.User{Name: name, Type: "BUILT_IN", DatabaseRoles: roles, Password: string(p)}).Context(ctx).Do()
	if e != nil {
		var ge *googleapi.Error
		if errors.As(e, &ge) && ge.Code >= 400 && ge.Code < 500 && ge.Code != 408 && ge.Code != 429 {
			return "", fmt.Errorf("%w: Cloud SQL rejected exact user request", gcp04w9release.ErrDefinitive)
		}
		return "", safe(e)
	}
	if op == nil {
		return "", errors.New("operation response missing")
	}
	return op.Name, nil
}
func (w writer) WaitOperation(ctx context.Context, name string) error {
	for i := 0; i < 60; i++ {
		op, e := w.a.sql.Operations.Get(gcp04w9release.ProjectID, name).Context(ctx).Do()
		if e != nil {
			return safe(e)
		}
		if op == nil {
			return errors.New("operation response missing")
		}
		if op.Status == "DONE" {
			if op.Error != nil && len(op.Error.Errors) > 0 {
				return fmt.Errorf("%w: Cloud SQL operation failed", gcp04w9release.ErrDefinitive)
			}
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return errors.New("operation deadline exceeded; reconcile")
}
func (w writer) DestroyVersion(ctx context.Context, v string) error {
	name, ok := gcp04w9release.SecretVersionResource(v)
	if !ok {
		return errors.New("numeric secret version required")
	}
	current, e := w.a.sm.Projects.Secrets.Versions.Get(name).Context(ctx).Do()
	if e != nil {
		return safe(e)
	}
	if current == nil || !gcp04w9release.IsSecretVersionName(current.Name, v) || current.Etag == "" || (current.State != "ENABLED" && current.State != "DISABLED") {
		return errors.New("exact destroyable version metadata unavailable")
	}
	x, e := w.a.sm.Projects.Secrets.Versions.Destroy(name, &secretmanager.DestroySecretVersionRequest{Etag: current.Etag}).Context(ctx).Do()
	if e != nil {
		return safe(e)
	}
	if x == nil || !gcp04w9release.IsSecretVersionName(x.Name, v) || x.State != "DESTROYED" {
		return errors.New("destroyed version response mismatch")
	}
	return nil
}
func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
func run(args []string, out, errOut io.Writer) int {
	f := flag.NewFlagSet("gcp04w9release", flag.ContinueOnError)
	f.SetOutput(errOut)
	execute := f.Bool("execute", false, "requires exact, active experiment-stage approval")
	prepareShell := f.Bool("prepare-secret-shell", false, "creates only an approved empty Secret shell")
	reconcileShell := f.Bool("reconcile-secret-shell", false, "read-only verification of an existing ETag-pinned empty shell")
	relpath := f.String("release-file", "", "private release file under .local/testing/gcp04-w9")
	relsha := f.String("confirm-release-sha256", "", "exact SHA-256 of reviewed release file")
	if e := f.Parse(args); e != nil {
		return 2
	}
	selected := 0
	for _, enabled := range []bool{*execute, *prepareShell, *reconcileShell} {
		if enabled {
			selected++
		}
	}
	if selected > 1 {
		fmt.Fprintln(errOut, "choose exactly one release stage")
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cred, e := google.FindDefaultCredentials(ctx, scope)
	if e != nil {
		fmt.Fprintln(errOut, "authorized application default credentials unavailable")
		return 1
	}
	opts := []option.ClientOption{option.WithCredentials(cred), quotaProjectOption()}
	sql, e := sqladmin.NewService(ctx, opts...)
	if e != nil {
		fmt.Fprintln(errOut, "Cloud SQL metadata client unavailable")
		return 1
	}
	sm, e := secretmanager.NewService(ctx, opts...)
	if e != nil {
		fmt.Fprintln(errOut, "Secret Manager metadata client unavailable")
		return 1
	}
	inv, e := gcp04w9release.Collect(ctx, api{sql, sm})
	if e != nil {
		fmt.Fprintln(errOut, e)
		return 1
	}
	plan, planErr := gcp04w9release.BuildPlan(inv)
	if *execute || *prepareShell || *reconcileShell {
		if planErr != nil {
			fmt.Fprintln(errOut, planErr)
			return 1
		}
		expectedPath := ".local/testing/gcp04-w9/release.json"
		if *prepareShell {
			expectedPath = ".local/testing/gcp04-w9/shell-release.json"
		} else if *reconcileShell {
			expectedPath = ".local/testing/gcp04-w9/reconcile-shell-release.json"
		}
		if filepath.Clean(*relpath) != expectedPath || len(*relsha) != 64 {
			fmt.Fprintln(errOut, "selected release stage requires its exact private W9 release file and reviewed SHA-256")
			return 2
		}
		info, se := os.Lstat(*relpath)
		if se != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
			fmt.Fprintln(errOut, "release file must be a regular owner-only file under .local/testing/gcp04-w9")
			return 2
		}
		dirInfo, de := os.Stat(filepath.Dir(*relpath))
		if de != nil || !dirInfo.IsDir() || dirInfo.Mode().Perm()&0077 != 0 {
			fmt.Fprintln(errOut, "release directory must be private to its owner")
			return 2
		}
		body, se := os.ReadFile(*relpath)
		if se != nil {
			fmt.Fprintln(errOut, "reviewed release file could not be read")
			return 2
		}
		sum := sha256.Sum256(body)
		if hex.EncodeToString(sum[:]) != strings.ToLower(*relsha) {
			fmt.Fprintln(errOut, "release file SHA-256 does not match reviewed value")
			return 2
		}
		release, se := gcp04w9release.ReadRelease(*relpath)
		if se != nil {
			fmt.Fprintln(errOut, "reviewed release file is invalid")
			return 2
		}
		if *prepareShell {
			if release.Stage != "secret_shell" {
				fmt.Fprintln(errOut, "secret-shell flag requires a secret_shell stage approval")
				return 2
			}
			if se = gcp04w9release.PrepareSecretShell(ctx, inv, release, writer{a: api{sql, sm}}, gcp04w9release.LocalJournal{Dir: ".local/testing/gcp04-w9/journal"}, time.Now()); se != nil {
				fmt.Fprintln(errOut, se)
				return 1
			}
			secret, re := (api{sql, sm}).ReadSecret(ctx)
			versions, ve := (api{sql, sm}).ListVersions(ctx)
			if re != nil || ve != nil || !gcp04w9release.IsSecretParent(secret.Name) || secret.ETag == "" || secret.Replication != "USER_MANAGED" || len(secret.Replicas) != 1 || secret.Replicas[0] != gcp04w9release.Region || secret.Labels["release_id"] != release.ReleaseID || secret.Labels["purpose"] != "gcp04-w9-catalog-probe" || len(versions) != 0 {
				fmt.Fprintln(errOut, "created shell metadata or empty version inventory could not be verified; reconcile before retry")
				return 1
			}
			j := gcp04w9release.LocalJournal{Dir: ".local/testing/gcp04-w9/journal"}
			if se = j.Save(gcp04w9release.JournalRecord{ReleaseID: release.ReleaseID, Phase: "secret_shell_verified", SecretResource: gcp04w9release.SecretParent, ObservedSecretName: secret.Name, ETag: secret.ETag, UpdatedAt: time.Now().UTC().Format(time.RFC3339)}); se != nil {
				fmt.Fprintln(errOut, "shell verified; private journal confirmation failed")
				return 1
			}
			fmt.Fprintf(out, "{\"mode\":\"prepare_secret_shell\",\"secret_resource\":%q,\"secret_etag\":%q,\"versions\":0,\"sql_user_created\":false,\"payload_created\":false}\n", secret.Name, secret.ETag)
			return 0
		}
		if *reconcileShell {
			if release.Stage != "secret_shell" || release.SecretShellETag == "" {
				fmt.Fprintln(errOut, "read-only reconciliation requires a secret_shell release with the exact observed ETag")
				return 2
			}
			if se = gcp04w9release.ReconcileSecretShell(inv, release, gcp04w9release.LocalJournal{Dir: ".local/testing/gcp04-w9/journal"}, time.Now()); se != nil {
				fmt.Fprintln(errOut, se)
				return 1
			}
			fmt.Fprintf(out, "{\"mode\":\"reconcile_secret_shell_read_only\",\"secret_resource\":%q,\"secret_etag\":%q,\"versions\":0,\"cloud_mutations\":false}\n", inv.Secret.Name, inv.Secret.ETag)
			return 0
		}
		if release.Stage != "experiment" {
			fmt.Fprintln(errOut, "execute flag requires an experiment stage approval")
			return 2
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		version, se := gcp04w9release.Execute(ctx, inv, release, writer{a: api{sql, sm}}, gcp04w9release.LocalJournal{Dir: ".local/testing/gcp04-w9/journal"}, time.Now(), nil)
		if se != nil {
			fmt.Fprintln(errOut, se)
			return 1
		}
		fmt.Fprintf(out, "{\"mode\":\"execute\",\"secret_version\":%q,\"payload_written_to_output\":false}\n", version)
		return 0
	}
	if planErr != nil {
		plan = gcp04w9release.Plan{Mode: "dry_run_blocked", Inventory: inv, ProposedUser: gcp04w9release.UserMetadata{Name: gcp04w9release.Login, Type: "BUILT_IN", DatabaseRoles: []string{gcp04w9release.Role}}, SecretResource: gcp04w9release.SecretParent, SecretShellAction: "create only if absent; pre-existing resource blocks execute", SecretReplication: "USER_MANAGED: southamerica-west1 only", PayloadPolicy: "not generated", Mutations: "none", BlockReason: planErr.Error()}
	}
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(true)
	if err := enc.Encode(plan); err != nil {
		fmt.Fprintln(errOut, "plan serialization failed")
		return 1
	}
	return 0
}
func (w writer) DeleteSecretIfOwned(ctx context.Context, release, version string) error {
	if release == "" || version == "" || strings.Trim(version, "0123456789") != "" {
		return errors.New("release identity and numeric version required for secret cleanup")
	}
	got, err := w.a.sm.Projects.Secrets.Get(gcp04w9release.SecretParent).Context(ctx).Do()
	if err != nil {
		return safe(err)
	}
	if got == nil || !gcp04w9release.IsSecretParent(got.Name) || got.Labels["release_id"] != release || got.Labels["purpose"] != "gcp04-w9-catalog-probe" {
		return errors.New("secret shell ownership labels do not match this release")
	}
	versions, err := w.a.ListVersions(ctx)
	if err != nil {
		return err
	}
	if len(versions) != 1 || !gcp04w9release.IsSecretVersionName(versions[0].Name, version) || versions[0].State != "DESTROYED" {
		return errors.New("secret version inventory is not the sole exact destroyed run-owned version; refusing delete")
	}
	if got.Etag == "" {
		return errors.New("secret ETag unavailable for conditional owned-shell cleanup")
	}
	_, err = w.a.sm.Projects.Secrets.Delete(gcp04w9release.SecretParent).Etag(got.Etag).Context(ctx).Do()
	if err != nil {
		return safe(err)
	}
	return nil
}
