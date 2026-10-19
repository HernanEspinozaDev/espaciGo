// Package gcp04w8release builds a read-only, dry-run release plan for the
// separate W8 catalog probe. It intentionally has no Cloud mutation client.
package gcp04w8release

import (
	"context"
	"errors"
	"fmt"
	"sort"
)

const (
	ProjectID      = "espacigo-testprod"
	InstanceName   = "espacigo-testprod-pg"
	ConnectionName = "espacigo-testprod:southamerica-west1:espacigo-testprod-pg"
	Region         = "southamerica-west1"
	PrivateIP      = "10.189.0.3"
	Database       = "espacigo"
	Login          = "espacigo_gcp04_w8_catalog_probe"
	SecretID       = "espacigo-gcp04-w8-catalog-probe-password"
	JobName        = "espacigo-gcp04-w8-catalog-probe"
	ServiceAccount = "espacigo-gcp04-w8-catalog-probe"
	CandidateRole  = "pg_read_all_settings"
)

var ErrCollision = errors.New("W8 exact resource already exists; abort without update or reset")

type Instance struct {
	Project, Name, Region, Version, State, ConnectionName, PrivateIP string
}

type User struct{ Name string }

// AdminReader is intentionally read-only. A production Cloud SQL Admin API
// mutator is deliberately absent from this package.
type AdminReader interface {
	ReadInstance(context.Context, string, string) (Instance, error)
	ListUsers(context.Context, string, string) ([]User, error)
	ReadRegionalSecret(context.Context, string, string, string) (bool, error)
	ListRegionalSecretVersions(context.Context, string, string, string) ([]string, error)
}

type CreateUserRequest struct {
	Name          string   `json:"name"`
	Type          string   `json:"type"`
	DatabaseRoles []string `json:"databaseRoles"`
}

type Plan struct {
	Format         string            `json:"format"`
	Mode           string            `json:"mode"`
	Target         Instance          `json:"target"`
	Login          string            `json:"login"`
	UserRequest    CreateUserRequest `json:"user_request_proposal"`
	RoleDecision   string            `json:"role_decision"`
	RegionalSecret string            `json:"regional_secret"`
	SecretPayload  string            `json:"secret_payload"`
	Image          string            `json:"image"`
	Job            string            `json:"job"`
	ServiceAccount string            `json:"service_account"`
	RequiredIAM    []string          `json:"required_iam"`
	Sequence       []string          `json:"sequence"`
	Teardown       []string          `json:"teardown"`
	CloudMutations string            `json:"cloud_mutations"`
}

func Build(ctx context.Context, api AdminReader) (Plan, error) {
	if api == nil {
		return Plan{}, errors.New("read-only Admin API is unavailable")
	}
	instance, err := api.ReadInstance(ctx, ProjectID, InstanceName)
	if err != nil || instance != expectedInstance() {
		return Plan{}, errors.New("Cloud SQL metadata differs from the pinned W8 target")
	}
	users, err := api.ListUsers(ctx, ProjectID, InstanceName)
	if err != nil {
		return Plan{}, errors.New("Cloud SQL user inventory unavailable")
	}
	for _, user := range users {
		if user.Name == Login {
			return Plan{}, ErrCollision
		}
	}
	secretExists, err := api.ReadRegionalSecret(ctx, ProjectID, Region, SecretID)
	if err != nil {
		return Plan{}, errors.New("regional Secret Manager metadata unavailable")
	}
	if !secretExists {
		return Plan{}, errors.New("regional secret must be separately reviewed and precreated; no payload is written by this plan")
	}
	versions, err := api.ListRegionalSecretVersions(ctx, ProjectID, Region, SecretID)
	if err != nil {
		return Plan{}, errors.New("regional secret version inventory unavailable")
	}
	if len(versions) != 0 {
		return Plan{}, errors.New("regional secret version collision; abort without rotation or deletion")
	}
	roles := []string{CandidateRole}
	if err := validateRoles(roles); err != nil {
		return Plan{}, err
	}
	return Plan{
		Format: "espacigo.gcp04.w8-release-plan.v1", Mode: "dry_run_only", Target: instance,
		Login: Login, UserRequest: CreateUserRequest{Name: Login, Type: "BUILT_IN", DatabaseRoles: roles},
		RoleDecision:   "candidate_unresolved: pg_read_all_settings is broad and Cloud SQL users.insert acceptance is undocumented; no create is authorized",
		RegionalSecret: fmt.Sprintf("projects/%s/locations/%s/secrets/%s", ProjectID, Region, SecretID),
		SecretPayload:  "not_generated_or_serialized", Image: "southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod/gcp04-w8-catalog-probe@sha256:<reviewed-immutable-digest-required>",
		Job: JobName, ServiceAccount: ServiceAccount,
		RequiredIAM:    []string{"roles/cloudsql.client (condition: exact SQL instance)", "roles/secretmanager.secretAccessor (exact W8 regional secret)", "roles/artifactregistry.reader (exact repository service agent)"},
		Sequence:       []string{"read exact instance/user/secret metadata", "abort on login or secret-version collision; never reset or update", "review candidate role and Admin API databaseRoles support", "generate password in memory only", "add regional secret version from memory; record numeric version only", "CREATE_USER for new login only with explicit databaseRoles=[pg_read_all_settings]", "create one isolated Job pinned to immutable image digest", "run first catalog probe once; emit allowlisted report only"},
		Teardown:       []string{"delete only exact W8 Job and dedicated service account/IAM bindings created by this release", "delete only W8 login if ownership is proven from this release journal", "destroy only exact numeric secret version created by this release", "preserve SQL instance, database, network, repository, preexisting secret metadata and all unrelated resources"},
		CloudMutations: "none; this output is a proposal",
	}, nil
}

func validateRoles(roles []string) error {
	if len(roles) != 1 || roles[0] != CandidateRole {
		return errors.New("explicit nonempty W8 databaseRoles list is required; no default-role fallback")
	}
	return nil
}

func expectedInstance() Instance {
	return Instance{Project: ProjectID, Name: InstanceName, Region: Region, Version: "POSTGRES_18", State: "RUNNABLE", ConnectionName: ConnectionName, PrivateIP: PrivateIP}
}

// SortedUsers is useful for deterministic mock/API inventory diagnostics and
// never includes credential fields.
func SortedUsers(users []User) []string {
	names := make([]string, 0, len(users))
	for _, user := range users {
		names = append(names, user.Name)
	}
	sort.Strings(names)
	return names
}
