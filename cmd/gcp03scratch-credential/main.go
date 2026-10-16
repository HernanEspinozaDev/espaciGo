package main

import (
	"context"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/gcp03scratch"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	"google.golang.org/api/secretmanager/v1"
	"google.golang.org/api/sqladmin/v1beta4"
)

const cloudPlatformScope = "https://www.googleapis.com/auth/cloud-platform"

const (
	cloudSQLAdminAPIService = "sqladmin.googleapis.com"
	secretManagerAPIService = "secretmanager.googleapis.com"
)

type cloudAPIError struct {
	service string
	method  string
	status  int
	reason  string
}

func newSQLAdminService(ctx context.Context, extra ...option.ClientOption) (*sqladmin.Service, error) {
	return sqladmin.NewService(ctx, cloudAPIClientOptions(extra...)...)
}

func newSecretManagerService(ctx context.Context, extra ...option.ClientOption) (*secretmanager.Service, error) {
	return secretmanager.NewService(ctx, cloudAPIClientOptions(extra...)...)
}

func cloudAPIClientOptions(extra ...option.ClientOption) []option.ClientOption {
	options := []option.ClientOption{
		option.WithScopes(cloudPlatformScope),
		option.WithQuotaProject(gcp03scratch.ProjectID),
	}
	return append(options, extra...)
}

func (e cloudAPIError) Error() string {
	status := fmt.Sprintf("http_status=%d", e.status)
	if e.status == 0 {
		status = "http_status=none"
	}
	return fmt.Sprintf("Google Cloud API request failed: service=%s method=%s %s reason=%s", e.service, e.method, status, e.reason)
}
func (e cloudAPIError) SafeDiagnostic() string { return e.Error() }
func (e cloudAPIError) Definitive() bool {
	return e.status >= 400 && e.status < 500 && e.status != http.StatusRequestTimeout && e.status != http.StatusTooManyRequests && e.status != http.StatusConflict
}
func (e cloudAPIError) NotFound() bool { return e.status == http.StatusNotFound }

type secretVersionAPI struct{ service *secretmanager.Service }

func (a secretVersionAPI) AddVersion(ctx context.Context, parent string, password []byte) (string, error) {
	if _, ok := gcp03scratch.NormalizeSecretVersionName(parent, parent+"/versions/1"); !ok {
		return "", errors.New("Secret Manager parent is outside the exact scratch namespace")
	}
	request := &secretmanager.AddSecretVersionRequest{
		Payload: &secretmanager.SecretPayload{Data: base64.StdEncoding.EncodeToString(password)},
	}
	version, err := a.service.Projects.Secrets.AddVersion(parent, request).Context(ctx).Do()
	if err != nil {
		return "", cloudAPIErrorFrom(secretManagerAPIService, "projects.secrets.addVersion", err)
	}
	if version == nil {
		return "", errors.New("Secret Manager returned no version resource; reconcile before retrying")
	}
	name := strings.TrimSpace(version.Name)
	if normalized, ok := gcp03scratch.NormalizeSecretVersionName(parent, name); !ok {
		return name, errors.New("Secret Manager returned a version outside the exact scratch namespace")
	} else {
		return normalized, nil
	}
}

func (a secretVersionAPI) DestroyVersion(ctx context.Context, name string) error {
	_, err := a.service.Projects.Secrets.Versions.Destroy(name, &secretmanager.DestroySecretVersionRequest{}).Context(ctx).Do()
	if err != nil {
		return cloudAPIErrorFrom(secretManagerAPIService, "projects.secrets.versions.destroy", err)
	}
	return nil
}

type sqlUserAPI struct{ service *sqladmin.Service }

func (a sqlUserAPI) CreateBuiltInUser(ctx context.Context, name string, password []byte) (string, error) {
	user := &sqladmin.User{Name: name, Password: string(password), Type: "BUILT_IN"}
	operation, err := a.service.Users.Insert(gcp03scratch.ProjectID, gcp03scratch.InstanceName, user).Context(ctx).Do()
	if err != nil {
		return "", cloudAPIErrorFrom(cloudSQLAdminAPIService, "users.insert", err)
	}
	return operation.Name, nil
}

func (a sqlUserAPI) DeleteBuiltInUser(ctx context.Context, name string) (string, error) {
	if name != gcp03scratch.MigratorLogin {
		return "", errors.New("only the fixed GCP-03 probe login can be deleted")
	}
	operation, err := a.service.Users.Delete(gcp03scratch.ProjectID, gcp03scratch.InstanceName).Name(name).Context(ctx).Do()
	if err != nil {
		return "", cloudAPIErrorFrom(cloudSQLAdminAPIService, "users.delete", err)
	}
	return operation.Name, nil
}

func (a sqlUserAPI) SetDatabaseRoles(ctx context.Context, name string, roles []string, revokeExisting bool) (string, error) {
	if name != gcp03scratch.MigratorLogin && name != gcp03scratch.DatabaseOwnerLogin {
		return "", errors.New("only fixed GCP-03 scratch logins are supported")
	}
	user := &sqladmin.User{
		Name: name,
		Type: "BUILT_IN",
	}
	call := a.service.Users.Update(gcp03scratch.ProjectID, gcp03scratch.InstanceName, user)
	if len(roles) == 0 {
		// The API encodes roles in query parameters for users.update. An empty
		// repeated value represents the documented zero-role replacement.
		call.DatabaseRoles("")
	} else {
		call.DatabaseRoles(roles...)
	}
	operation, err := call.Name(name).RevokeExistingRoles(revokeExisting).Context(ctx).Do()
	if err != nil {
		return "", cloudAPIErrorFrom(cloudSQLAdminAPIService, "users.update", err)
	}
	return operation.Name, nil
}

func (a sqlUserAPI) SetPassword(ctx context.Context, name string, password []byte) (string, error) {
	if name != gcp03scratch.DatabaseOwnerLogin {
		return "", errors.New("only the fixed GCP-03 database-owner login is supported")
	}
	user := &sqladmin.User{Name: name, Type: "BUILT_IN", Password: string(password)}
	operation, err := a.service.Users.Update(gcp03scratch.ProjectID, gcp03scratch.InstanceName, user).Name(name).Context(ctx).Do()
	if err != nil {
		return "", cloudAPIErrorFrom(cloudSQLAdminAPIService, "users.update", err)
	}
	return operation.Name, nil
}

func (a sqlUserAPI) GetDatabaseRoles(ctx context.Context, name string) ([]string, error) {
	if name != gcp03scratch.MigratorLogin && name != gcp03scratch.DatabaseOwnerLogin {
		return nil, errors.New("only fixed GCP-03 scratch logins are supported")
	}
	user, err := a.service.Users.Get(gcp03scratch.ProjectID, gcp03scratch.InstanceName, name).Context(ctx).Do()
	if err != nil {
		return nil, cloudAPIErrorFrom(cloudSQLAdminAPIService, "users.get", err)
	}
	return append([]string(nil), user.DatabaseRoles...), nil
}

func (a sqlUserAPI) GetOperation(ctx context.Context, name string) (bool, bool, error) {
	operation, err := a.service.Operations.Get(gcp03scratch.ProjectID, name).Context(ctx).Do()
	if err != nil {
		return false, false, cloudAPIErrorFrom(cloudSQLAdminAPIService, "operations.get", err)
	}
	if operation.Status != "DONE" {
		return false, false, nil
	}
	return true, operation.Error != nil && len(operation.Error.Errors) != 0, nil
}

func cloudAPIErrorFrom(service, method string, err error) error {
	var apiErr *googleapi.Error
	if errors.As(err, &apiErr) {
		return cloudAPIError{service: service, method: method, status: apiErr.Code, reason: safeGoogleAPIReason(apiErr)}
	}
	return cloudAPIError{service: service, method: method, reason: "TRANSPORT_UNCERTAIN"}
}

var googleAPIReasonAllowlist = map[string]string{
	"ABORTED":                         "ABORTED",
	"ACCESS_NOT_CONFIGURED":           "ACCESS_NOT_CONFIGURED",
	"ACCESS_TOKEN_SCOPE_INSUFFICIENT": "ACCESS_TOKEN_SCOPE_INSUFFICIENT",
	"ALREADY_EXISTS":                  "ALREADY_EXISTS",
	"API_DISABLED":                    "API_DISABLED",
	"BAD_REQUEST":                     "BAD_REQUEST",
	"backendError":                    "INTERNAL",
	"badRequest":                      "BAD_REQUEST",
	"conditionNotMet":                 "FAILED_PRECONDITION",
	"DEADLINE_EXCEEDED":               "DEADLINE_EXCEEDED",
	"FAILED_PRECONDITION":             "FAILED_PRECONDITION",
	"INTERNAL":                        "INTERNAL",
	"INVALID_ARGUMENT":                "INVALID_ARGUMENT",
	"NOT_FOUND":                       "NOT_FOUND",
	"ORG_RESTRICTION_VIOLATION":       "ORG_RESTRICTION_VIOLATION",
	"PERMISSION_DENIED":               "PERMISSION_DENIED",
	"RATE_LIMIT_EXCEEDED":             "RATE_LIMIT_EXCEEDED",
	"RESOURCE_EXHAUSTED":              "RESOURCE_EXHAUSTED",
	"SERVICE_DISABLED":                "SERVICE_DISABLED",
	"UNAUTHENTICATED":                 "UNAUTHENTICATED",
	"UNAVAILABLE":                     "UNAVAILABLE",
	"UNKNOWN":                         "UNKNOWN",
	"USER_PROJECT_DENIED":             "USER_PROJECT_DENIED",
	"accessNotConfigured":             "ACCESS_NOT_CONFIGURED",
	"forbidden":                       "PERMISSION_DENIED",
	"invalid":                         "INVALID_ARGUMENT",
	"invalidParameter":                "INVALID_ARGUMENT",
	"notFound":                        "NOT_FOUND",
	"quotaExceeded":                   "RESOURCE_EXHAUSTED",
	"rateLimitExceeded":               "RATE_LIMIT_EXCEEDED",
	"serviceDisabled":                 "SERVICE_DISABLED",
	"userRateLimitExceeded":           "RATE_LIMIT_EXCEEDED",
	"userProjectDenied":               "USER_PROJECT_DENIED",
}

func safeGoogleAPIReason(apiErr *googleapi.Error) string {
	for _, detail := range apiErr.Details {
		item, ok := detail.(map[string]interface{})
		if !ok || item["@type"] != "type.googleapis.com/google.rpc.ErrorInfo" {
			continue
		}
		if reason, ok := item["reason"].(string); ok {
			if safe := googleAPIReasonAllowlist[reason]; safe != "" {
				return safe
			}
		}
	}
	for _, item := range apiErr.Errors {
		if safe := googleAPIReasonAllowlist[item.Reason]; safe != "" {
			return safe
		}
	}
	return "UNCLASSIFIED"
}

func main() {
	os.Exit(run())
}

func run() int {
	apply := flag.Bool("apply", false, "create the one-time scratch login and secret version")
	revoke := flag.Bool("revoke-superuser", false, "remove all database-role memberships from the temporary scratch login")
	reconcileRoles := flag.Bool("reconcile-creator-memberships", false, "preserve only cloudsqlsuperuser and remove PostgreSQL 18 creator memberships")
	createOwner := flag.Bool("create-database-owner-credential", false, "set a temporary password for the scratch database owner")
	retireOwner := flag.Bool("retire-database-owner-credential", false, "rotate the scratch database owner password and destroy its exact secret version")
	retireProbeLogin := flag.Bool("retire-role-comment-probe-login", false, "delete the fixed GCP-03 probe login and destroy its exact secret version")
	confirm := flag.String("confirm-target", "", "exact reviewed scratch target confirmation")
	secretVersion := flag.String("secret-version", "", "exact Secret Manager version to destroy after successful role revocation")
	flag.Parse()
	modeCount := 0
	for _, enabled := range []bool{*revoke, *reconcileRoles, *createOwner, *retireOwner, *retireProbeLogin} {
		if enabled {
			modeCount++
		}
	}
	if modeCount == 0 {
		modeCount = 1 // Default mode creates the migration login.
	}
	valid := flag.NArg() == 0 && *apply && modeCount == 1
	switch {
	case *revoke:
		valid = valid && *confirm == gcp03scratch.RevokeConfirmation && gcp03scratch.IsScratchSecretVersion(*secretVersion)
	case *reconcileRoles:
		valid = valid && *confirm == gcp03scratch.ReconcileConfirmation && *secretVersion == ""
	case *createOwner:
		valid = valid && *confirm == gcp03scratch.DatabaseOwnerCreateConfirmation && *secretVersion == ""
	case *retireOwner:
		valid = valid && *confirm == gcp03scratch.DatabaseOwnerRetireConfirmation && gcp03scratch.IsDatabaseOwnerSecretVersion(*secretVersion)
	case *retireProbeLogin:
		valid = valid && *confirm == gcp03scratch.ProbeRetireConfirmation && gcp03scratch.IsScratchSecretVersion(*secretVersion)
	default:
		valid = valid && *confirm == gcp03scratch.CredentialConfirmation && *secretVersion == ""
	}
	if !valid {
		fmt.Fprintf(flag.CommandLine.Output(), "create: gcp03scratch-credential --apply --confirm-target=%s\n", gcp03scratch.CredentialConfirmation)
		fmt.Fprintf(flag.CommandLine.Output(), "revoke: gcp03scratch-credential --apply --revoke-superuser --confirm-target=%s --secret-version=projects/espacigo-testprod/secrets/espacigo-gcp03-pilot01-migrator-password/versions/N\n", gcp03scratch.RevokeConfirmation)
		fmt.Fprintf(flag.CommandLine.Output(), "reconcile: gcp03scratch-credential --apply --reconcile-creator-memberships --confirm-target=%s\n", gcp03scratch.ReconcileConfirmation)
		fmt.Fprintf(flag.CommandLine.Output(), "owner-create: gcp03scratch-credential --apply --create-database-owner-credential --confirm-target=%s\n", gcp03scratch.DatabaseOwnerCreateConfirmation)
		fmt.Fprintf(flag.CommandLine.Output(), "owner-retire: gcp03scratch-credential --apply --retire-database-owner-credential --confirm-target=%s --secret-version=projects/espacigo-testprod/secrets/espacigo-gcp03-pilot01-db-owner-password/versions/N\n", gcp03scratch.DatabaseOwnerRetireConfirmation)
		fmt.Fprintf(flag.CommandLine.Output(), "probe-retire: gcp03scratch-credential --apply --retire-role-comment-probe-login --confirm-target=%s --secret-version=projects/espacigo-testprod/secrets/espacigo-gcp03-pilot01-migrator-password/versions/N\n", gcp03scratch.ProbeRetireConfirmation)
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	sqlService, err := newSQLAdminService(ctx)
	if err != nil {
		fmt.Fprintln(flag.CommandLine.Output(), "could not initialize Cloud SQL Admin API client")
		return 1
	}
	provisioner := gcp03scratch.CredentialProvisioner{Users: sqlUserAPI{service: sqlService}}
	secretService, err := newSecretManagerService(ctx)
	if err != nil {
		fmt.Fprintln(flag.CommandLine.Output(), "could not initialize Secret Manager API client")
		return 1
	}
	provisioner.Secrets = secretVersionAPI{service: secretService}
	if *retireProbeLogin {
		if err := provisioner.RetireRoleCommentProbeLogin(ctx, *apply, *confirm, *secretVersion); err != nil {
			fmt.Fprintln(flag.CommandLine.Output(), err)
			return 1
		}
		fmt.Fprintln(flag.CommandLine.Output(), "fixed GCP-03 probe login deleted and exact secret version destroyed")
		return 0
	}
	if *revoke {
		if err := provisioner.RevokeSuperuser(ctx, *apply, *confirm, *secretVersion); err != nil {
			fmt.Fprintln(flag.CommandLine.Output(), err)
			return 1
		}
		fmt.Fprintln(flag.CommandLine.Output(), "temporary Cloud SQL roles revoked and exact secret version destroyed")
		return 0
	}
	if *reconcileRoles {
		if err := provisioner.ReconcileCreatorMemberships(ctx, *apply, *confirm); err != nil {
			fmt.Fprintln(flag.CommandLine.Output(), err)
			return 1
		}
		fmt.Fprintln(flag.CommandLine.Output(), "temporary creator memberships removed; cloudsqlsuperuser preserved for V1-V61")
		return 0
	}
	if *createOwner {
		result, err := provisioner.CreateDatabaseOwnerCredential(ctx, *apply, *confirm)
		if err != nil {
			fmt.Fprintln(flag.CommandLine.Output(), err)
			if result.SecretVersion != "" {
				fmt.Fprintf(flag.CommandLine.Output(), "reconciliation_required=true secret_version=%s\n", result.SecretVersion)
			}
			return 1
		}
		fmt.Fprintf(flag.CommandLine.Output(), "temporary database-owner credential created; secret version=%s\n", result.SecretVersion)
		return 0
	}
	if *retireOwner {
		if err := provisioner.RetireDatabaseOwnerCredential(ctx, *apply, *confirm, *secretVersion); err != nil {
			fmt.Fprintln(flag.CommandLine.Output(), err)
			return 1
		}
		fmt.Fprintln(flag.CommandLine.Output(), "database-owner password rotated and exact secret version destroyed")
		return 0
	}
	result, err := provisioner.Provision(ctx, *apply, *confirm)
	if err != nil {
		fmt.Fprintln(flag.CommandLine.Output(), err)
		if result.SecretVersion != "" {
			fmt.Fprintf(flag.CommandLine.Output(), "reconciliation_required=true secret_version=%s\n", result.SecretVersion)
		}
		return 1
	}
	fmt.Fprintf(flag.CommandLine.Output(), "scratch login creation completed; secret version=%s\n", result.SecretVersion)
	return 0
}
