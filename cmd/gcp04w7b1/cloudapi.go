package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/gcp04w7b1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/secretmanager/v1"
	"google.golang.org/api/sqladmin/v1beta4"
)

type cloudAPI struct {
	sql *sqladmin.Service
	sm  *secretmanager.Service
}

func apiError(err error) error {
	var e *googleapi.Error
	if errors.As(err, &e) {
		return fmt.Errorf("Cloud API HTTP %d", e.Code)
	}
	return errors.New("Cloud API outcome uncertain")
}
func parent() string               { return "projects/" + gcp04w7b1.ProjectID + "/secrets/" + gcp04w7b1.SecretID }
func parentCanonical() string      { return "projects/447373903153/secrets/" + gcp04w7b1.SecretID }
func secretParentOK(s string) bool { return s == parent() || s == parentCanonical() }
func versionNumber(name string) string {
	i := strings.LastIndex(name, "/versions/")
	if i < 0 {
		return ""
	}
	return name[i+len("/versions/"):]
}

func (a cloudAPI) ReadInstance(ctx context.Context) (gcp04w7b1.InstanceMetadata, error) {
	x, e := a.sql.Instances.Get(gcp04w7b1.ProjectID, gcp04w7b1.Instance).Context(ctx).Do()
	if e != nil {
		return gcp04w7b1.InstanceMetadata{}, apiError(e)
	}
	if x == nil {
		return gcp04w7b1.InstanceMetadata{}, errors.New("instance metadata incomplete")
	}
	ip := ""
	if x.Settings != nil {
		for _, v := range x.IpAddresses {
			if v.Type == "PRIVATE" {
				ip = v.IpAddress
			}
		}
	}
	return gcp04w7b1.InstanceMetadata{Project: x.Project, Name: x.Name, Region: x.Region, Version: x.DatabaseVersion, State: x.State, Connection: x.ConnectionName, IP: ip}, nil
}
func (a cloudAPI) ListUsers(ctx context.Context) ([]gcp04w7b1.UserMetadata, error) {
	call := a.sql.Users.List(gcp04w7b1.ProjectID, gcp04w7b1.Instance)
	x, e := call.Context(ctx).Do()
	if e != nil {
		return nil, apiError(e)
	}
	if x == nil || x.NextPageToken != "" {
		return nil, errors.New("Cloud SQL user inventory incomplete or paginated")
	}
	out := make([]gcp04w7b1.UserMetadata, 0, len(x.Items))
	for _, u := range x.Items {
		if u != nil {
			out = append(out, gcp04w7b1.UserMetadata{Name: u.Name, Type: u.Type, Roles: append([]string(nil), u.DatabaseRoles...)})
		}
	}
	return out, nil
}
func (a cloudAPI) ReadSecret(ctx context.Context) (gcp04w7b1.SecretMetadata, error) {
	x, e := a.sm.Projects.Secrets.Get(parent()).Context(ctx).Do()
	if e != nil {
		var ge *googleapi.Error
		if errors.As(e, &ge) && ge.Code == 404 {
			return gcp04w7b1.SecretMetadata{}, nil
		}
		return gcp04w7b1.SecretMetadata{}, apiError(e)
	}
	if x == nil || !secretParentOK(x.Name) || x.Replication == nil || x.Replication.UserManaged == nil {
		return gcp04w7b1.SecretMetadata{}, errors.New("exact B1 secret metadata incomplete")
	}
	replicas := []string{}
	for _, r := range x.Replication.UserManaged.Replicas {
		if r != nil {
			replicas = append(replicas, r.Location)
		}
	}
	return gcp04w7b1.SecretMetadata{Name: x.Name, ETag: x.Etag, Replication: "USER_MANAGED", Replicas: replicas, Labels: x.Labels}, nil
}
func (a cloudAPI) ListVersions(ctx context.Context) ([]gcp04w7b1.VersionMetadata, error) {
	var out []gcp04w7b1.VersionMetadata
	token := ""
	seen := map[string]bool{}
	for page := 0; page < 50; page++ {
		call := a.sm.Projects.Secrets.Versions.List(parent()).PageSize(100)
		if token != "" {
			call = call.PageToken(token)
		}
		x, e := call.Context(ctx).Do()
		if e != nil {
			var ge *googleapi.Error
			if errors.As(e, &ge) && ge.Code == 404 {
				return []gcp04w7b1.VersionMetadata{}, nil
			}
			return nil, apiError(e)
		}
		if x == nil {
			return nil, errors.New("B1 version inventory incomplete")
		}
		for _, v := range x.Versions {
			if v != nil {
				if !secretParentOK(strings.TrimSuffix(v.Name, "/versions/"+versionNumber(v.Name))) || versionNumber(v.Name) == "" {
					return nil, errors.New("B1 version outside exact resource")
				}
				out = append(out, gcp04w7b1.VersionMetadata{Name: v.Name, Number: versionNumber(v.Name), State: v.State, ETag: v.Etag})
			}
		}
		if x.NextPageToken == "" {
			return out, nil
		}
		if seen[x.NextPageToken] {
			return nil, errors.New("repeated secret version pagination token")
		}
		seen[x.NextPageToken] = true
		token = x.NextPageToken
	}
	return nil, errors.New("B1 version inventory exceeds page bound")
}

func (a cloudAPI) CreateSecret(ctx context.Context, release string) (gcp04w7b1.SecretMetadata, error) {
	if release == "" {
		return gcp04w7b1.SecretMetadata{}, errors.New("B1 release ID required")
	}
	x, e := a.sm.Projects.Secrets.Create("projects/"+gcp04w7b1.ProjectID, &secretmanager.Secret{Replication: &secretmanager.Replication{UserManaged: &secretmanager.UserManaged{Replicas: []*secretmanager.Replica{{Location: gcp04w7b1.Region}}}}, Labels: map[string]string{"release_id": release, "purpose": "gcp04-w7-b1-bootstrap"}}).SecretId(gcp04w7b1.SecretID).Context(ctx).Do()
	if e != nil {
		return gcp04w7b1.SecretMetadata{}, apiError(e)
	}
	if x == nil || !secretParentOK(x.Name) || x.Etag == "" || x.Labels["release_id"] != release || x.Labels["purpose"] != "gcp04-w7-b1-bootstrap" || x.Replication == nil || x.Replication.UserManaged == nil {
		return gcp04w7b1.SecretMetadata{}, errors.New("B1 secret create response mismatch")
	}
	locs := []string{}
	for _, v := range x.Replication.UserManaged.Replicas {
		if v != nil {
			locs = append(locs, v.Location)
		}
	}
	return gcp04w7b1.SecretMetadata{Name: x.Name, ETag: x.Etag, Replication: "USER_MANAGED", Replicas: locs, Labels: x.Labels}, nil
}
func (a cloudAPI) AddVersion(ctx context.Context, name string, payload []byte) (gcp04w7b1.VersionMetadata, error) {
	if !secretParentOK(name) || len(payload) == 0 {
		return gcp04w7b1.VersionMetadata{}, errors.New("exact B1 secret and in-memory payload required")
	}
	x, e := a.sm.Projects.Secrets.AddVersion(name, &secretmanager.AddSecretVersionRequest{Payload: &secretmanager.SecretPayload{Data: base64.StdEncoding.EncodeToString(payload)}}).Context(ctx).Do()
	if e != nil {
		return gcp04w7b1.VersionMetadata{}, apiError(e)
	}
	if x == nil || x.Etag == "" {
		return gcp04w7b1.VersionMetadata{}, errors.New("B1 secret version response incomplete")
	}
	num := versionNumber(x.Name)
	return gcp04w7b1.VersionMetadata{Name: x.Name, Number: num, State: "ENABLED", ETag: x.Etag}, nil
}
func (a cloudAPI) InsertUser(ctx context.Context, name, typ string, roles []string, password []byte) (string, error) {
	if name != gcp04w7b1.Bootstrap || typ != "BUILT_IN" || len(roles) != 1 || roles[0] != gcp04w7b1.AdminRole || len(password) == 0 {
		return "", errors.New("B1 insert requires exact login and explicit cloudsqlsuperuser role")
	}
	x, e := a.sql.Users.Insert(gcp04w7b1.ProjectID, gcp04w7b1.Instance, &sqladmin.User{Name: name, Type: typ, DatabaseRoles: append([]string(nil), roles...), Password: string(password)}).Context(ctx).Do()
	if e != nil {
		return "", apiError(e)
	}
	if x == nil || x.Name == "" {
		return "", errors.New("B1 user insert operation missing")
	}
	return x.Name, nil
}
func (a cloudAPI) WaitOperation(ctx context.Context, name string) error {
	if name == "" {
		return errors.New("B1 SQL operation name required")
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		op, e := a.sql.Operations.Get(gcp04w7b1.ProjectID, name).Context(ctx).Do()
		if e != nil {
			return apiError(e)
		}
		if op == nil {
			return errors.New("B1 SQL operation response missing")
		}
		if op.Status == "DONE" {
			if op.Error != nil && len(op.Error.Errors) > 0 {
				return errors.New("B1 Cloud SQL operation completed with error")
			}
			return nil
		}
		select {
		case <-ctx.Done():
			return errors.New("B1 operation deadline; reconcile, do not retry")
		case <-ticker.C:
		}
	}
}
func (a cloudAPI) ReadOperation(ctx context.Context, name string) (gcp04w7b1.OperationMetadata, error) {
	if name == "" {
		return gcp04w7b1.OperationMetadata{}, errors.New("B1 operation ID required")
	}
	op, e := a.sql.Operations.Get(gcp04w7b1.ProjectID, name).Context(ctx).Do()
	if e != nil {
		return gcp04w7b1.OperationMetadata{}, apiError(e)
	}
	if op == nil {
		return gcp04w7b1.OperationMetadata{}, errors.New("B1 operation metadata missing")
	}
	failed := op.Error != nil && len(op.Error.Errors) > 0
	return gcp04w7b1.OperationMetadata{ID: op.Name, Type: op.OperationType, TargetID: op.TargetId, TargetLink: op.TargetLink, TargetProject: op.TargetProject, Status: op.Status, HasErrors: failed}, nil
}
func (a cloudAPI) DeleteUser(ctx context.Context, name string) (string, error) {
	if name != gcp04w7b1.Bootstrap {
		return "", errors.New("B1 cleanup user identity mismatch")
	}
	x, e := a.sql.Users.Delete(gcp04w7b1.ProjectID, gcp04w7b1.Instance).Name(name).Context(ctx).Do()
	if e != nil {
		return "", apiError(e)
	}
	if x == nil || x.Name == "" {
		return "", errors.New("B1 delete operation missing")
	}
	return x.Name, nil
}
func (a cloudAPI) DestroyVersion(ctx context.Context, name, etag string) error {
	if !secretParentOK(strings.TrimSuffix(name, "/versions/"+versionNumber(name))) || versionNumber(name) == "" || etag == "" {
		return errors.New("B1 exact version and ETag required")
	}
	v, e := a.sm.Projects.Secrets.Versions.Get(name).Context(ctx).Do()
	if e != nil {
		return apiError(e)
	}
	if v == nil || v.Name != name || v.Etag != etag || (v.State != "ENABLED" && v.State != "DISABLED") {
		return errors.New("B1 version state or ETag changed")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	x, e := a.sm.Projects.Secrets.Versions.Destroy(name, &secretmanager.DestroySecretVersionRequest{Etag: etag}).Context(ctx).Do()
	if e != nil {
		return apiError(e)
	}
	if x == nil || x.Name != name || x.State != "DESTROYED" {
		return errors.New("B1 version destruction not confirmed")
	}
	if x.Etag == "" {
		return errors.New("B1 destroyed version ETag missing")
	}
	return nil
}
func (a cloudAPI) DeleteSecret(ctx context.Context, name, release, etag string) error {
	if !secretParentOK(name) || release == "" || etag == "" {
		return errors.New("B1 exact shell, release, and ETag required")
	}
	x, e := a.sm.Projects.Secrets.Get(name).Context(ctx).Do()
	if e != nil {
		return apiError(e)
	}
	if x == nil || x.Name != name || x.Etag != etag || x.Labels["release_id"] != release || x.Labels["purpose"] != "gcp04-w7-b1-bootstrap" {
		return errors.New("B1 secret ownership or ETag mismatch")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	_, e = a.sm.Projects.Secrets.Delete(name).Etag(etag).Context(ctx).Do()
	if e != nil {
		return apiError(e)
	}
	return nil
}
