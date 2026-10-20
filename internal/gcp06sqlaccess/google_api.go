package gcp06sqlaccess

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	"google.golang.org/api/secretmanager/v1"
	"google.golang.org/api/sqladmin/v1beta4"
)

const cloudPlatformScope = "https://www.googleapis.com/auth/cloud-platform"

var credentialLoginPattern = regexp.MustCompile(`^gcp06_inspect_[a-f0-9]{12}$`)

type cloudAPIError struct {
	service    string
	method     string
	status     int
	reason     string
	permission string
}

func (e cloudAPIError) Error() string {
	status := fmt.Sprintf("http_status=%d", e.status)
	if e.status == 0 {
		status = "http_status=none"
	}
	permission := ""
	if e.permission != "" {
		permission = " required_permission=" + e.permission
	}
	return fmt.Sprintf("Google Cloud API request failed: service=%s method=%s %s reason=%s%s", e.service, e.method, status, e.reason, permission)
}

func (e cloudAPIError) SafeCloudError() string { return e.Error() }

func (e cloudAPIError) Definitive() bool {
	return e.status >= 400 && e.status < 500 && e.status != http.StatusRequestTimeout && e.status != http.StatusTooManyRequests && e.status != http.StatusConflict
}

func NewGoogleAPIClients(ctx context.Context) (SQLAdmin, SecretStore, error) {
	options := []option.ClientOption{option.WithScopes(cloudPlatformScope), option.WithQuotaProject(ProjectID)}
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

func (a secretManagerAPI) GetSecret(ctx context.Context, parent string) (bool, string, error) {
	if parent != SecretParent {
		return false, "", errors.New("secret target is outside the exact GCP-06 candidate")
	}
	secret, err := a.service.Projects.Secrets.Get(parent).Context(ctx).Do()
	if err != nil {
		if isNotFound(err) {
			return false, "", nil
		}
		return false, "", apiError("secretmanager.googleapis.com", "projects.secrets.get", err)
	}
	if secret == nil || secret.Name != SecretParent {
		return false, "", errors.New("Secret Manager returned a different candidate secret")
	}
	return true, secret.VersionDestroyTtl, nil
}

func (a secretManagerAPI) ListVersions(ctx context.Context, parent string) ([]Version, error) {
	if parent != SecretParent {
		return nil, errors.New("secret target is outside the exact GCP-06 candidate")
	}
	var versions []Version
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
		for _, item := range result.Versions {
			if item != nil {
				versions = append(versions, Version{Name: strings.TrimSpace(item.Name), State: item.State})
			}
		}
		page = result.NextPageToken
		if page == "" {
			return versions, nil
		}
	}
}

func (a secretManagerAPI) AddVersion(ctx context.Context, parent string, payload []byte) (string, error) {
	if parent != SecretParent || len(payload) == 0 || len(payload) > 4096 {
		return "", errors.New("secret version payload or target is outside the fixed GCP-06 candidate contract")
	}
	request := &secretmanager.AddSecretVersionRequest{Payload: &secretmanager.SecretPayload{Data: base64.StdEncoding.EncodeToString(payload)}}
	version, err := a.service.Projects.Secrets.AddVersion(parent, request).Context(ctx).Do()
	if err != nil {
		return "", apiError("secretmanager.googleapis.com", "projects.secrets.addVersion", err)
	}
	if version == nil {
		return "", errors.New("Secret Manager returned no version resource")
	}
	return strings.TrimSpace(version.Name), nil
}

func (a secretManagerAPI) GetVersion(ctx context.Context, name string) (string, error) {
	if !validVersionName(name) {
		return "", errors.New("secret version target is outside the exact GCP-06 candidate")
	}
	version, err := a.service.Projects.Secrets.Versions.Get(name).Context(ctx).Do()
	if err != nil {
		return "", apiError("secretmanager.googleapis.com", "projects.secrets.versions.get", err)
	}
	if version == nil || version.Name != name {
		return "", errors.New("Secret Manager returned a different version")
	}
	return version.State, nil
}

func (a secretManagerAPI) ReadCredentialLogin(ctx context.Context, name string) (string, error) {
	if !validVersionName(name) {
		return "", errors.New("secret credential target is outside the exact GCP-06 candidate")
	}
	response, err := a.service.Projects.Secrets.Versions.Access(name).Context(ctx).Do()
	if err != nil {
		return "", apiError("secretmanager.googleapis.com", "projects.secrets.versions.access", err)
	}
	if response == nil || response.Payload == nil {
		return "", errors.New("Secret Manager returned no exact credential payload")
	}
	payload, err := base64.StdEncoding.DecodeString(response.Payload.Data)
	if err != nil || len(payload) == 0 || len(payload) > 4096 {
		clear(payload)
		return "", errors.New("Secret Manager credential payload is invalid")
	}
	defer clear(payload)
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var record struct {
		Username string          `json:"username"`
		Password json.RawMessage `json:"password"`
	}
	if err := decoder.Decode(&record); err != nil || !credentialLoginPattern.MatchString(record.Username) {
		clear(record.Password)
		return "", errors.New("Secret Manager credential record is invalid")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		clear(record.Password)
		return "", errors.New("Secret Manager credential record has trailing data")
	}
	if len(record.Password) != 45 || record.Password[0] != '"' || record.Password[len(record.Password)-1] != '"' {
		clear(record.Password)
		return "", errors.New("Secret Manager credential password shape is invalid")
	}
	for _, ch := range record.Password[1 : len(record.Password)-1] {
		if !((ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-') {
			clear(record.Password)
			return "", errors.New("Secret Manager credential password shape is invalid")
		}
	}
	encodedPassword := record.Password[1 : len(record.Password)-1]
	decodedPassword := make([]byte, 32)
	decodedLength, decodeErr := base64.RawURLEncoding.Strict().Decode(decodedPassword, encodedPassword)
	clear(decodedPassword)
	if decodeErr != nil || decodedLength != 32 {
		clear(record.Password)
		return "", errors.New("Secret Manager credential password shape is invalid")
	}
	clear(record.Password)
	return record.Username, nil
}

func (a secretManagerAPI) DestroyVersion(ctx context.Context, name string) error {
	if !validVersionName(name) {
		return errors.New("secret version cleanup target is outside the exact GCP-06 candidate")
	}
	_, err := a.service.Projects.Secrets.Versions.Destroy(name, &secretmanager.DestroySecretVersionRequest{}).Context(ctx).Do()
	if err != nil {
		return apiError("secretmanager.googleapis.com", "projects.secrets.versions.destroy", err)
	}
	return nil
}

type sqlAdminAPI struct{ service *sqladmin.Service }

func (a sqlAdminAPI) GetInstance(ctx context.Context, project, instance string) (string, string, string, string, bool, error) {
	if project != ProjectID || instance != InstanceName {
		return "", "", "", "", false, errors.New("Cloud SQL target is outside the exact GCP-06 candidate")
	}
	resource, err := a.service.Instances.Get(project, instance).Context(ctx).Do()
	if err != nil {
		return "", "", "", "", false, apiError("sqladmin.googleapis.com", "instances.get", err)
	}
	if resource == nil {
		return "", "", "", "", false, errors.New("Cloud SQL returned no candidate instance")
	}
	privateIP := false
	for _, address := range resource.IpAddresses {
		if address != nil && address.Type == "PRIVATE" && address.IpAddress != "" {
			privateIP = true
		}
		if address != nil && address.Type == "PRIMARY" {
			return "", "", "", "", false, errors.New("Cloud SQL candidate unexpectedly exposes a public address")
		}
	}
	return resource.Name, resource.State, resource.DatabaseVersion, resource.Region, privateIP, nil
}

func (a sqlAdminAPI) GetDatabase(ctx context.Context, project, instance, database string) (string, error) {
	if project != ProjectID || instance != InstanceName || database != DatabaseName {
		return "", errors.New("Cloud SQL database target is outside the exact GCP-06 candidate")
	}
	resource, err := a.service.Databases.Get(project, instance, database).Context(ctx).Do()
	if err != nil {
		return "", apiError("sqladmin.googleapis.com", "databases.get", err)
	}
	if resource == nil {
		return "", errors.New("Cloud SQL returned no candidate database")
	}
	return resource.Name, nil
}

func (a sqlAdminAPI) GetUser(ctx context.Context, project, instance, username string) (bool, string, error) {
	if project != ProjectID || instance != InstanceName || !inspectorLoginPattern.MatchString(username) {
		return false, "", errors.New("Cloud SQL user target is outside the exact GCP-06 inspector contract")
	}
	resource, err := a.service.Users.Get(project, instance, username).Context(ctx).Do()
	if err != nil {
		if isNotFound(err) {
			return false, "", nil
		}
		return false, "", apiError("sqladmin.googleapis.com", "users.get", err)
	}
	if resource == nil || resource.Name != username {
		return false, "", errors.New("Cloud SQL returned a different candidate SQL user")
	}
	userType := resource.Type
	if userType == "" {
		userType = "BUILT_IN"
	}
	return true, userType, nil
}

func (a sqlAdminAPI) InsertUser(ctx context.Context, project, instance, username string, password []byte) (string, error) {
	if project != ProjectID || instance != InstanceName || !inspectorLoginPattern.MatchString(username) || len(password) != 43 {
		return "", errors.New("Cloud SQL user creation is outside the exact GCP-06 inspector contract")
	}
	request := &sqladmin.User{Name: username, Type: "BUILT_IN", Password: string(password)}
	operation, err := a.service.Users.Insert(project, instance, request).Context(ctx).Do()
	if err != nil {
		return "", apiError("sqladmin.googleapis.com", "users.insert", err)
	}
	if operation == nil {
		return "", errors.New("Cloud SQL returned no user-create operation")
	}
	return operation.Name, nil
}

func (a sqlAdminAPI) DeleteUser(ctx context.Context, project, instance, username string) (string, error) {
	if project != ProjectID || instance != InstanceName || !inspectorLoginPattern.MatchString(username) {
		return "", errors.New("Cloud SQL user deletion is outside the exact GCP-06 inspector contract")
	}
	operation, err := a.service.Users.Delete(project, instance).Name(username).Context(ctx).Do()
	if err != nil {
		return "", apiError("sqladmin.googleapis.com", "users.delete", err)
	}
	if operation == nil {
		return "", errors.New("Cloud SQL returned no user-delete operation")
	}
	return operation.Name, nil
}

func (a sqlAdminAPI) GetOperation(ctx context.Context, project, operationID string) (bool, bool, error) {
	if project != ProjectID || operationID == "" || strings.Contains(operationID, "/") {
		return false, false, errors.New("Cloud SQL operation target is invalid for the fixed GCP-06 candidate")
	}
	operation, err := a.service.Operations.Get(project, operationID).Context(ctx).Do()
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
	permission := ""
	for _, detail := range apiErr.Details {
		item, ok := detail.(map[string]interface{})
		if !ok || item["@type"] != "type.googleapis.com/google.rpc.ErrorInfo" {
			continue
		}
		if value, ok := item["reason"].(string); ok {
			reason = safeReason(value)
		}
		if metadata, ok := item["metadata"].(map[string]interface{}); ok {
			if value, ok := metadata["permission"].(string); ok && safePermission(value) {
				permission = value
			}
		}
	}
	return cloudAPIError{service: service, method: method, status: apiErr.Code, reason: reason, permission: permission}
}

func safeReason(reason string) string {
	allowed := map[string]string{
		"ACCESS_NOT_CONFIGURED": "ACCESS_NOT_CONFIGURED", "API_DISABLED": "API_DISABLED",
		"FAILED_PRECONDITION": "FAILED_PRECONDITION", "INVALID_ARGUMENT": "INVALID_ARGUMENT",
		"NOT_FOUND": "NOT_FOUND", "PERMISSION_DENIED": "PERMISSION_DENIED",
		"RESOURCE_EXHAUSTED": "RESOURCE_EXHAUSTED", "SERVICE_DISABLED": "SERVICE_DISABLED",
		"UNAUTHENTICATED": "UNAUTHENTICATED", "UNAVAILABLE": "UNAVAILABLE",
		"USER_PROJECT_DENIED": "USER_PROJECT_DENIED", "quotaExceeded": "RESOURCE_EXHAUSTED",
		"userProjectDenied": "USER_PROJECT_DENIED", "serviceDisabled": "SERVICE_DISABLED",
		"invalid": "INVALID_ARGUMENT", "notFound": "NOT_FOUND", "forbidden": "PERMISSION_DENIED",
	}
	if value := allowed[reason]; value != "" {
		return value
	}
	return "UNCLASSIFIED"
}

func safePermission(permission string) bool {
	for _, prefix := range []string{"cloudsql.", "secretmanager."} {
		if strings.HasPrefix(permission, prefix) && !strings.ContainsAny(permission, " \t\r\n/@") {
			return true
		}
	}
	return false
}

func isNotFound(err error) bool {
	var apiErr *googleapi.Error
	return errors.As(err, &apiErr) && apiErr.Code == http.StatusNotFound
}
