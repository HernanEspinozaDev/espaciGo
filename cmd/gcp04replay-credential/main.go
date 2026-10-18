package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/HernanEspinozaDev/espaciGo/internal/gcp04credential"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	"google.golang.org/api/secretmanager/v1"
	"google.golang.org/api/sqladmin/v1beta4"
)

const cloudPlatformScope = "https://www.googleapis.com/auth/cloud-platform"

type cloudAPIError struct {
	status int
}

func (e cloudAPIError) Error() string {
	if e.status == 0 {
		return "Cloud API transport outcome is uncertain"
	}
	return fmt.Sprintf("Cloud API returned HTTP %d", e.status)
}

func safeAPIError(err error) error {
	var apiErr *googleapi.Error
	if errors.As(err, &apiErr) {
		return cloudAPIError{status: apiErr.Code}
	}
	return cloudAPIError{}
}

type cloudStateReader struct{ client *storage.Client }

func (r cloudStateReader) ReadState(ctx context.Context, bucket, object string) ([]byte, int64, error) {
	if bucket != gcp04credential.StateBucket || object != gcp04credential.StateObject {
		return nil, 0, errors.New("state target outside fixed GCP-04 scope")
	}
	attrs, err := r.client.Bucket(bucket).Object(object).Attrs(ctx)
	if err != nil {
		return nil, 0, safeAPIError(err)
	}
	reader, err := r.client.Bucket(bucket).Object(object).Generation(attrs.Generation).NewReader(ctx)
	if err != nil {
		return nil, 0, safeAPIError(err)
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, 16<<20))
	if err != nil || len(data) == 16<<20 {
		return nil, 0, errors.New("Terraform state object could not be read within its size limit")
	}
	return data, attrs.Generation, nil
}

type cloudTargets struct {
	sql     *sqladmin.Service
	secrets *secretmanager.Service
}

func canonicalSecret(name string) bool {
	return name == gcp04credential.SecretParent || name == "projects/"+gcp04credential.ProjectNumber+"/secrets/"+gcp04credential.SecretName
}

func (c cloudTargets) ReadSQLInstance(ctx context.Context, project, instance string) (gcp04credential.SQLInstance, error) {
	if project != gcp04credential.ProjectID || instance != gcp04credential.InstanceName {
		return gcp04credential.SQLInstance{}, errors.New("SQL target outside fixed GCP-04 scope")
	}
	got, err := c.sql.Instances.Get(project, instance).Context(ctx).Do()
	if err != nil || got == nil {
		return gcp04credential.SQLInstance{}, safeAPIError(err)
	}
	if got.Settings == nil {
		return gcp04credential.SQLInstance{}, errors.New("Cloud SQL settings were absent")
	}
	ip := got.Settings.IpConfiguration
	labels := got.Settings.UserLabels
	privateNetwork := ""
	ipv4 := true
	if ip != nil {
		privateNetwork = ip.PrivateNetwork
		ipv4 = ip.Ipv4Enabled
	}
	return gcp04credential.SQLInstance{
		Name: got.Name, Project: got.Project, Region: got.Region, Version: got.DatabaseVersion,
		State: got.State, PrivateNetwork: privateNetwork, IPv4Enabled: ipv4, RunID: labels["run_id"],
	}, nil
}

func (c cloudTargets) ReadSecret(ctx context.Context, project, secret string) (string, error) {
	if project != gcp04credential.ProjectID || secret != gcp04credential.SecretName {
		return "", errors.New("secret target outside fixed GCP-04 scope")
	}
	got, err := c.secrets.Projects.Secrets.Get(gcp04credential.SecretParent).Context(ctx).Do()
	if err != nil || got == nil || !canonicalSecret(got.Name) {
		return "", safeAPIError(err)
	}
	return got.Name, nil
}

func (c cloudTargets) ListSecretVersions(ctx context.Context, project, secret string) ([]string, error) {
	if project != gcp04credential.ProjectID || secret != gcp04credential.SecretName {
		return nil, errors.New("secret target outside fixed GCP-04 scope")
	}
	var versions []string
	call := c.secrets.Projects.Secrets.Versions.List(gcp04credential.SecretParent).PageSize(100)
	for {
		page, err := call.Context(ctx).Do()
		if err != nil {
			return nil, safeAPIError(err)
		}
		for _, item := range page.Versions {
			if item != nil {
				versions = append(versions, item.Name)
			}
		}
		if page.NextPageToken == "" {
			return versions, nil
		}
		call = call.PageToken(page.NextPageToken)
	}
}

type secretVersionWriter struct{ service *secretmanager.Service }

func (w secretVersionWriter) AddVersion(ctx context.Context, parent string, password []byte) (string, error) {
	if parent != gcp04credential.SecretParent || len(password) == 0 {
		return "", errors.New("Secret Manager addVersion target or payload is invalid")
	}
	request := &secretmanager.AddSecretVersionRequest{
		Payload: &secretmanager.SecretPayload{Data: base64.StdEncoding.EncodeToString(password)},
	}
	version, err := w.service.Projects.Secrets.AddVersion(parent, request).Context(ctx).Do()
	if err != nil {
		return "", safeAPIError(err)
	}
	if version == nil {
		return "", errors.New("Secret Manager returned no version identity")
	}
	return version.Name, nil
}

type sqlPasswordWriter struct{ service *sqladmin.Service }

func (w sqlPasswordWriter) SetPassword(ctx context.Context, project, instance, user string, password []byte) (string, error) {
	if project != gcp04credential.ProjectID || instance != gcp04credential.InstanceName || user != gcp04credential.DatabaseOwner || len(password) == 0 {
		return "", errors.New("Cloud SQL update target or payload is invalid")
	}
	body := &sqladmin.User{Name: user, Type: "BUILT_IN", Password: string(password)}
	operation, err := w.service.Users.Update(project, instance, body).Name(user).Context(ctx).Do()
	if err != nil {
		return "", safeAPIError(err)
	}
	if operation == nil {
		return "", errors.New("Cloud SQL returned no operation identity")
	}
	return operation.Name, nil
}

func (w sqlPasswordWriter) ReadOperation(ctx context.Context, project, operationName string) (bool, bool, error) {
	if project != gcp04credential.ProjectID || strings.TrimSpace(operationName) == "" {
		return false, false, errors.New("Cloud SQL operation target is invalid")
	}
	operation, err := w.service.Operations.Get(project, operationName).Context(ctx).Do()
	if err != nil || operation == nil {
		return false, false, safeAPIError(err)
	}
	if operation.Status != "DONE" {
		return false, false, nil
	}
	return true, operation.Error != nil && len(operation.Error.Errors) > 0, nil
}

func newCloudRunner(ctx context.Context) (gcp04credential.CredentialRunner, error) {
	options := []option.ClientOption{option.WithScopes(cloudPlatformScope), option.WithQuotaProject(gcp04credential.ProjectID)}
	sqlService, err := sqladmin.NewService(ctx, options...)
	if err != nil {
		return gcp04credential.CredentialRunner{}, errors.New("Cloud SQL API client could not be initialized")
	}
	secretService, err := secretmanager.NewService(ctx, options...)
	if err != nil {
		return gcp04credential.CredentialRunner{}, errors.New("Secret Manager API client could not be initialized")
	}
	storageClient, err := storage.NewClient(ctx, options...)
	if err != nil {
		return gcp04credential.CredentialRunner{}, errors.New("Cloud Storage API client could not be initialized")
	}
	return gcp04credential.CredentialRunner{
		States:    cloudStateReader{client: storageClient},
		Inspector: cloudTargets{sql: sqlService, secrets: secretService},
		Secrets:   secretVersionWriter{service: secretService},
		SQL:       sqlPasswordWriter{service: sqlService},
		Journal:   gcp04credential.PrivateJournal{Path: gcp04credential.JournalPath},
		Evidence:  gcp04credential.LocalEvidence{},
	}, nil
}

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("gcp04replay-credential", flag.ContinueOnError)
	flags.SetOutput(stderr)
	preview := flags.Bool("preview", false, "validate a private release without Cloud calls or secret generation")
	execute := flags.Bool("execute", false, "execute only with an exact released SHA-256 confirmation")
	releaseFile := flags.String("release-file", gcp04credential.ReleasePath, "fixed private release JSON path")
	confirm := flags.String("confirm-release-sha256", "", "exact SHA-256 of the reviewed release file")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *preview == *execute {
		fmt.Fprintf(stderr, "preview: gcp04replay-credential --preview --release-file=%s\n", gcp04credential.ReleasePath)
		fmt.Fprintf(stderr, "execute: gcp04replay-credential --execute --release-file=%s --confirm-release-sha256=<exact-release-sha256>\n", gcp04credential.ReleasePath)
		return 2
	}
	now := time.Now().UTC()
	_, releaseHash, data, err := gcp04credential.ReadPrivateRelease(*releaseFile, now)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if *preview {
		previewData, err := (gcp04credential.CredentialRunner{Evidence: gcp04credential.LocalEvidence{}}).Preview(data, now)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		if err := json.NewEncoder(stdout).Encode(previewData); err != nil {
			fmt.Fprintln(stderr, "preview could not be printed")
			return 1
		}
		return 0
	}
	if err := gcp04credential.VerifyReleaseHash(releaseHash, *confirm); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	runner, err := newCloudRunner(ctx)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	result, err := runner.Execute(ctx, data, *confirm)
	if err != nil {
		_ = json.NewEncoder(stdout).Encode(result)
		fmt.Fprintln(stderr, err)
		return 1
	}
	_ = json.NewEncoder(stdout).Encode(result)
	return 0
}
