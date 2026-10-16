package gcp03restorecredential

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	"google.golang.org/api/secretmanager/v1"
	"google.golang.org/api/sqladmin/v1beta4"
)

const cloudPlatformScope = "https://www.googleapis.com/auth/cloud-platform"

type cloudAPIError struct {
	service string
	method  string
	status  int
	reason  string
}

func (e cloudAPIError) Error() string {
	status := fmt.Sprintf("http_status=%d", e.status)
	if e.status == 0 {
		status = "http_status=none"
	}
	return fmt.Sprintf("Google Cloud API request failed: service=%s method=%s %s reason=%s", e.service, e.method, status, e.reason)
}

func (e cloudAPIError) Definitive() bool {
	return e.status >= 400 && e.status < 500 && e.status != http.StatusRequestTimeout && e.status != http.StatusTooManyRequests && e.status != http.StatusConflict
}

func NewGoogleAPIClients(ctx context.Context) (SQLAdmin, SecretStore, error) {
	return newGoogleAPIClients(ctx)
}

func newGoogleAPIClients(ctx context.Context, extra ...option.ClientOption) (SQLAdmin, SecretStore, error) {
	options := []option.ClientOption{option.WithScopes(cloudPlatformScope)}
	options = append(options, extra...)
	// Keep the quota project last so an endpoint/test option cannot override
	// the fixed project used for both Cloud APIs.
	options = append(options, option.WithQuotaProject(ProjectID))
	sql, err := sqladmin.NewService(ctx, options...)
	if err != nil {
		return nil, nil, errors.New("could not initialize Cloud SQL Admin API client")
	}
	secrets, err := secretmanager.NewService(ctx, options...)
	if err != nil {
		return nil, nil, errors.New("could not initialize Secret Manager API client")
	}
	return sqlAdminAPI{service: sql}, secretManagerAPI{service: secrets}, nil
}

type secretManagerAPI struct{ service *secretmanager.Service }

func (a secretManagerAPI) ListVersions(ctx context.Context, parent string) ([]string, error) {
	if parent != SecretParent {
		return nil, errors.New("secret target is outside the exact GCP-03 restore run")
	}
	var names []string
	page := ""
	for {
		call := a.service.Projects.Secrets.Versions.List(parent).PageSize(1000).Context(ctx)
		if page != "" {
			call.PageToken(page)
		}
		result, err := call.Do()
		if err != nil {
			return nil, apiError("secretmanager.googleapis.com", "projects.secrets.versions.list", err)
		}
		for _, version := range result.Versions {
			if version != nil {
				names = append(names, version.Name)
			}
		}
		page = result.NextPageToken
		if page == "" {
			return names, nil
		}
	}
}

func (a secretManagerAPI) AddVersion(ctx context.Context, parent string, password []byte) (string, error) {
	if parent != SecretParent {
		return "", errors.New("secret target is outside the exact GCP-03 restore run")
	}
	request := &secretmanager.AddSecretVersionRequest{
		Payload: &secretmanager.SecretPayload{Data: encodeSecretPayload(password)},
	}
	version, err := a.service.Projects.Secrets.AddVersion(parent, request).Context(ctx).Do()
	if err != nil {
		return "", apiError("secretmanager.googleapis.com", "projects.secrets.addVersion", err)
	}
	if version == nil {
		return "", errors.New("Secret Manager returned no version resource")
	}
	return strings.TrimSpace(version.Name), nil
}

func (a secretManagerAPI) DestroyVersion(ctx context.Context, name string) error {
	if !isExactVersion(name) {
		return errors.New("secret version cleanup target is outside the exact GCP-03 restore run")
	}
	_, err := a.service.Projects.Secrets.Versions.Destroy(name, &secretmanager.DestroySecretVersionRequest{}).Context(ctx).Do()
	if err != nil {
		return apiError("secretmanager.googleapis.com", "projects.secrets.versions.destroy", err)
	}
	return nil
}

type sqlAdminAPI struct{ service *sqladmin.Service }

func (a sqlAdminAPI) GetInstance(ctx context.Context, project, instance string) (string, string, error) {
	if project != ProjectID || instance != InstanceName {
		return "", "", errors.New("Cloud SQL instance target is outside the exact GCP-03 restore run")
	}
	result, err := a.service.Instances.Get(project, instance).Context(ctx).Do()
	if err != nil {
		return "", "", apiError("sqladmin.googleapis.com", "instances.get", err)
	}
	if result == nil {
		return "", "", errors.New("Cloud SQL returned no instance resource")
	}
	return result.Name, result.State, nil
}

func (a sqlAdminAPI) GetDatabase(ctx context.Context, project, instance, database string) (string, error) {
	if project != ProjectID || instance != InstanceName || database != DatabaseName {
		return "", errors.New("Cloud SQL database target is outside the exact GCP-03 restore run")
	}
	result, err := a.service.Databases.Get(project, instance, database).Context(ctx).Do()
	if err != nil {
		return "", apiError("sqladmin.googleapis.com", "databases.get", err)
	}
	if result == nil {
		return "", errors.New("Cloud SQL returned no database resource")
	}
	return result.Name, nil
}

func (a sqlAdminAPI) GetUser(ctx context.Context, project, instance, user string) (string, string, error) {
	if project != ProjectID || instance != InstanceName || user != UserName {
		return "", "", errors.New("Cloud SQL user target is outside the exact GCP-03 restore run")
	}
	result, err := a.service.Users.Get(project, instance, user).Context(ctx).Do()
	if err != nil {
		return "", "", apiError("sqladmin.googleapis.com", "users.get", err)
	}
	if result == nil {
		return "", "", errors.New("Cloud SQL returned no user resource")
	}
	return result.Name, result.Type, nil
}

func (a sqlAdminAPI) UpdatePassword(ctx context.Context, project, instance, user string, password []byte) (string, error) {
	if project != ProjectID || instance != InstanceName || user != UserName || !isGeneratedPassword(password) {
		return "", errors.New("Cloud SQL password update target or generated password is outside the exact GCP-03 restore contract")
	}
	request := &sqladmin.User{Name: UserName, Type: "BUILT_IN", Password: string(password)}
	operation, err := a.service.Users.Update(ProjectID, InstanceName, request).Name(UserName).Context(ctx).Do()
	if err != nil {
		return "", apiError("sqladmin.googleapis.com", "users.update", err)
	}
	if operation == nil {
		return "", errors.New("Cloud SQL returned no password update operation")
	}
	return operation.Name, nil
}

func (a sqlAdminAPI) GetOperation(ctx context.Context, project, name string) (bool, bool, error) {
	if project != ProjectID || name == "" || strings.Contains(name, "/") {
		return false, false, errors.New("Cloud SQL operation target is invalid for the exact GCP-03 restore run")
	}
	operation, err := a.service.Operations.Get(ProjectID, name).Context(ctx).Do()
	if err != nil {
		return false, false, apiError("sqladmin.googleapis.com", "operations.get", err)
	}
	if operation == nil {
		return false, false, errors.New("Cloud SQL returned no operation resource")
	}
	if operation.Status != "DONE" {
		return false, false, nil
	}
	return true, operation.Error != nil, nil
}

func apiError(service, method string, err error) error {
	var apiErr *googleapi.Error
	if !errors.As(err, &apiErr) {
		return cloudAPIError{service: service, method: method, reason: "TRANSPORT_UNCERTAIN"}
	}
	reason := "UNCLASSIFIED"
	for _, detail := range apiErr.Details {
		item, ok := detail.(map[string]interface{})
		if !ok || item["@type"] != "type.googleapis.com/google.rpc.ErrorInfo" {
			continue
		}
		if value, ok := item["reason"].(string); ok {
			reason = safeReason(value)
			break
		}
	}
	return cloudAPIError{service: service, method: method, status: apiErr.Code, reason: reason}
}

func safeReason(reason string) string {
	safe := map[string]string{
		"ACCESS_NOT_CONFIGURED": "ACCESS_NOT_CONFIGURED", "API_DISABLED": "API_DISABLED",
		"FAILED_PRECONDITION": "FAILED_PRECONDITION", "INVALID_ARGUMENT": "INVALID_ARGUMENT",
		"NOT_FOUND": "NOT_FOUND", "PERMISSION_DENIED": "PERMISSION_DENIED",
		"RESOURCE_EXHAUSTED": "RESOURCE_EXHAUSTED", "SERVICE_DISABLED": "SERVICE_DISABLED",
		"UNAUTHENTICATED": "UNAUTHENTICATED", "UNAVAILABLE": "UNAVAILABLE",
		"USER_PROJECT_DENIED": "USER_PROJECT_DENIED", "quotaExceeded": "RESOURCE_EXHAUSTED",
		"userProjectDenied": "USER_PROJECT_DENIED", "serviceDisabled": "SERVICE_DISABLED",
		"invalid": "INVALID_ARGUMENT", "notFound": "NOT_FOUND", "forbidden": "PERMISSION_DENIED",
	}
	if value := safe[reason]; value != "" {
		return value
	}
	return "UNCLASSIFIED"
}

// encodeSecretPayload is separate so the Secret Manager API's transport
// encoding cannot be confused with the URL-safe encoding of the password.
func encodeSecretPayload(password []byte) string {
	return base64.StdEncoding.EncodeToString(password)
}
