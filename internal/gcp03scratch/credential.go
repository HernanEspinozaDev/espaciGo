package gcp03scratch

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

const (
	ScratchSecretResource           = "projects/espacigo-testprod/secrets/espacigo-gcp03-pilot01-migrator-password"
	DatabaseOwnerSecretResource     = "projects/espacigo-testprod/secrets/espacigo-gcp03-pilot01-db-owner-password"
	CredentialConfirmation          = "CREATE:" + TargetConfirmation
	ReconcileConfirmation           = "RECONCILE:" + TargetConfirmation
	DatabaseOwnerCreateConfirmation = "ACL-CREATE:" + TargetConfirmation
	DatabaseOwnerRetireConfirmation = "ACL-RETIRE:" + TargetConfirmation
	RevokeConfirmation              = "REVOKE:" + TargetConfirmation
	ProbeRetireConfirmation         = "PROBE-RETIRE:" + TargetConfirmation
)

type SecretVersionStore interface {
	AddVersion(context.Context, string, []byte) (string, error)
	DestroyVersion(context.Context, string) error
}

type CloudSQLUserStore interface {
	CreateBuiltInUser(context.Context, string, []byte) (string, error)
	DeleteBuiltInUser(context.Context, string) (string, error)
	SetDatabaseRoles(context.Context, string, []string, bool) (string, error)
	SetPassword(context.Context, string, []byte) (string, error)
	GetDatabaseRoles(context.Context, string) ([]string, error)
	GetOperation(context.Context, string) (done, failed bool, err error)
}

type CredentialProvisioner struct {
	Secrets SecretVersionStore
	Users   CloudSQLUserStore
	Random  io.Reader
	Poll    func(context.Context, time.Duration) error
}

type CredentialResult struct {
	SecretVersion string
	Reconcile     bool
}

func (p CredentialProvisioner) RevokeSuperuser(ctx context.Context, apply bool, confirmation, secretVersion string) error {
	if !apply || confirmation != RevokeConfirmation || !IsScratchSecretVersion(secretVersion) {
		return errors.New("revoking Cloud SQL scratch roles requires --apply and the exact scratch confirmation")
	}
	if p.Users == nil || p.Secrets == nil {
		return errors.New("Cloud SQL or Secret Manager API client is unavailable")
	}
	operation, err := p.Users.SetDatabaseRoles(ctx, MigratorLogin, nil, true)
	if err != nil {
		return errors.New("could not request removal of temporary Cloud SQL roles")
	}
	if strings.TrimSpace(operation) == "" {
		return errors.New("Cloud SQL returned no role-revocation operation ID; reconcile before continuing")
	}
	poll := p.Poll
	if poll == nil {
		poll = waitPoll
	}
	for {
		done, failed, err := p.Users.GetOperation(ctx, operation)
		if err != nil {
			return errors.New("Cloud SQL role-revocation status is unavailable; reconcile before continuing")
		}
		if done {
			if failed {
				return errors.New("Cloud SQL role revocation failed; scratch login is not ready for retirement")
			}
			roles, err := p.Users.GetDatabaseRoles(ctx, MigratorLogin)
			if err != nil {
				return errors.New("could not verify Cloud SQL role revocation; reconcile before continuing")
			}
			if len(roles) != 0 {
				return errors.New("Cloud SQL still reports database roles for the retired scratch login")
			}
			if err := p.Secrets.DestroyVersion(ctx, secretVersion); err != nil {
				return errors.New("Cloud SQL roles are revoked; exact secret version cleanup needs reconciliation")
			}
			return nil
		}
		if err := poll(ctx, 2*time.Second); err != nil {
			return errors.New("Cloud SQL role revocation did not finish; reconcile before continuing")
		}
	}
}

// RetireRoleCommentProbeLogin deletes only the fixed disposable probe login
// after both probe phases and Cloud SQL role reconciliation have completed.
// The credential version is destroyed only after Cloud SQL confirms the user
// is absent. A retry after successful deletion only reconciles that version.
func (p CredentialProvisioner) RetireRoleCommentProbeLogin(ctx context.Context, apply bool, confirmation, secretVersion string) error {
	if !apply || confirmation != ProbeRetireConfirmation || !IsScratchSecretVersion(secretVersion) {
		return errors.New("retiring the fixed GCP-03 probe login requires --apply and the exact scratch confirmation/version")
	}
	if p.Users == nil || p.Secrets == nil {
		return errors.New("Cloud SQL or Secret Manager API client is unavailable")
	}
	roles, err := p.Users.GetDatabaseRoles(ctx, MigratorLogin)
	if err != nil {
		if !isCloudSQLUserNotFound(err) {
			return errors.New("could not inspect the fixed GCP-03 probe login; retain its secret version")
		}
		return p.Secrets.DestroyVersion(ctx, secretVersion)
	}
	if len(roles) != 1 || roles[0] != "cloudsqlsuperuser" {
		return errors.New("fixed GCP-03 probe login roles differ from the reviewed reconciled state")
	}
	operation, err := p.Users.DeleteBuiltInUser(ctx, MigratorLogin)
	if err != nil || strings.TrimSpace(operation) == "" {
		return errors.New("fixed GCP-03 probe login deletion outcome is uncertain; retain its secret version")
	}
	if err := p.waitOperation(ctx, operation, "fixed GCP-03 probe login deletion"); err != nil {
		return err
	}
	if _, err := p.Users.GetDatabaseRoles(ctx, MigratorLogin); !isCloudSQLUserNotFound(err) {
		return errors.New("Cloud SQL did not confirm fixed GCP-03 probe login absence; retain its secret version")
	}
	if err := p.Secrets.DestroyVersion(ctx, secretVersion); err != nil {
		return errors.New("fixed GCP-03 probe login is absent; exact secret version cleanup needs reconciliation")
	}
	return nil
}

func isCloudSQLUserNotFound(err error) bool {
	type notFound interface{ NotFound() bool }
	var target notFound
	return errors.As(err, &target) && target.NotFound()
}

func IsScratchSecretVersion(name string) bool {
	_, ok := NormalizeSecretVersionName(ScratchSecretResource, name)
	return ok
}

func IsDatabaseOwnerSecretVersion(name string) bool {
	_, ok := NormalizeSecretVersionName(DatabaseOwnerSecretResource, name)
	return ok
}

// NormalizeSecretVersionName validates a version resource against one of the
// two fixed scratch secret namespaces. Secret Manager may return the project
// number even when the request used the project ID; retain that canonical API
// name unchanged after validating its exact secret and positive numeric ID.
func NormalizeSecretVersionName(parent, name string) (string, bool) {
	secret, ok := scratchSecretName(parent)
	if !ok {
		return "", false
	}
	parts := strings.Split(name, "/")
	if len(parts) != 6 || parts[0] != "projects" || parts[2] != "secrets" || parts[3] != secret || parts[4] != "versions" {
		return "", false
	}
	if parts[1] != ProjectID && parts[1] != ProjectNumber {
		return "", false
	}
	version, err := strconv.ParseUint(parts[5], 10, 64)
	if err != nil || version == 0 || strconv.FormatUint(version, 10) != parts[5] {
		return "", false
	}
	return name, true
}

func scratchSecretName(parent string) (string, bool) {
	switch parent {
	case ScratchSecretResource:
		return "espacigo-gcp03-pilot01-migrator-password", true
	case DatabaseOwnerSecretResource:
		return "espacigo-gcp03-pilot01-db-owner-password", true
	default:
		return "", false
	}
}

// SafeSecretVersionReference preserves a canonical API reference for manual
// reconciliation only when its project and secret are in the fixed scratch
// namespace and its final path component is safe to display. It does not make
// aliases or malformed version IDs valid for operations.
func SafeSecretVersionReference(parent, name string) string {
	secret, ok := scratchSecretName(parent)
	if !ok {
		return ""
	}
	parts := strings.Split(name, "/")
	if len(parts) != 6 || parts[0] != "projects" || (parts[1] != ProjectID && parts[1] != ProjectNumber) || parts[2] != "secrets" || parts[3] != secret || parts[4] != "versions" || parts[5] == "" {
		return ""
	}
	for _, char := range parts[5] {
		if !(char >= 'A' && char <= 'Z') && !(char >= 'a' && char <= 'z') && !(char >= '0' && char <= '9') && char != '-' && char != '_' {
			return ""
		}
	}
	return name
}

func invalidSecretVersionResult(parent, kind, name string) (CredentialResult, error) {
	result := CredentialResult{Reconcile: true}
	if reference := SafeSecretVersionReference(parent, name); reference != "" {
		result.SecretVersion = reference
		return result, fmt.Errorf("Secret Manager returned an invalid %s version resource %s; reconcile before retrying", kind, reference)
	}
	return result, fmt.Errorf("Secret Manager returned an invalid %s version resource outside the exact scratch namespace; reconcile before retrying", kind)
}

func withSafeCloudAPIDiagnostic(message string, err error) error {
	type safeDiagnostic interface{ SafeDiagnostic() string }
	var diagnostic safeDiagnostic
	if errors.As(err, &diagnostic) {
		if safe := diagnostic.SafeDiagnostic(); safe != "" {
			return fmt.Errorf("%s (%s)", message, safe)
		}
	}
	return errors.New(message)
}

// ReconcileCreatorMemberships removes only non-system role memberships added
// while the temporary migrator created the two fixed NOLOGIN group roles. The
// built-in Cloud SQL role is deliberately preserved for V1-V61.
func (p CredentialProvisioner) ReconcileCreatorMemberships(ctx context.Context, apply bool, confirmation string) error {
	if !apply || confirmation != ReconcileConfirmation {
		return errors.New("reconciling temporary role memberships requires --apply and the exact scratch confirmation")
	}
	if p.Users == nil {
		return errors.New("Cloud SQL Admin API client is unavailable")
	}
	operation, err := p.Users.SetDatabaseRoles(ctx, MigratorLogin, []string{"cloudsqlsuperuser"}, true)
	if err != nil || strings.TrimSpace(operation) == "" {
		return errors.New("could not request safe Cloud SQL role membership reconciliation")
	}
	if err := p.waitOperation(ctx, operation, "Cloud SQL role membership reconciliation"); err != nil {
		return err
	}
	roles, err := p.Users.GetDatabaseRoles(ctx, MigratorLogin)
	if err != nil || len(roles) != 1 || roles[0] != "cloudsqlsuperuser" {
		return errors.New("Cloud SQL did not confirm the expected temporary migration role set")
	}
	return nil
}

func (p CredentialProvisioner) CreateDatabaseOwnerCredential(ctx context.Context, apply bool, confirmation string) (CredentialResult, error) {
	if !apply || confirmation != DatabaseOwnerCreateConfirmation {
		return CredentialResult{}, errors.New("temporary database-owner credential requires --apply and the exact ACL confirmation")
	}
	if p.Secrets == nil || p.Users == nil {
		return CredentialResult{}, errors.New("Cloud SQL or Secret Manager API client is unavailable")
	}
	password, err := p.randomPassword()
	if err != nil {
		return CredentialResult{}, err
	}
	defer clear(password)
	version, err := p.Secrets.AddVersion(ctx, DatabaseOwnerSecretResource, password)
	if err != nil {
		if strings.TrimSpace(version) != "" {
			return invalidSecretVersionResult(DatabaseOwnerSecretResource, "database-owner", strings.TrimSpace(version))
		}
		return CredentialResult{Reconcile: true}, withSafeCloudAPIDiagnostic("database-owner secret version outcome is uncertain; enumerate versions before retrying", err)
	}
	version = strings.TrimSpace(version)
	normalized, ok := NormalizeSecretVersionName(DatabaseOwnerSecretResource, version)
	if !ok {
		return invalidSecretVersionResult(DatabaseOwnerSecretResource, "database-owner", version)
	}
	version = normalized
	operation, err := p.Users.SetPassword(ctx, DatabaseOwnerLogin, password)
	if err != nil {
		if isDefinitiveCreateFailure(err) {
			if cleanupErr := p.Secrets.DestroyVersion(ctx, version); cleanupErr != nil {
				return CredentialResult{SecretVersion: version, Reconcile: true}, withSafeCloudAPIDiagnostic("Cloud SQL rejected the owner password and secret cleanup needs reconciliation", err)
			}
			return CredentialResult{}, withSafeCloudAPIDiagnostic("Cloud SQL rejected the owner password; the new secret version was destroyed", err)
		}
		return CredentialResult{SecretVersion: version, Reconcile: true}, withSafeCloudAPIDiagnostic("database-owner password update outcome is uncertain; retain the version and reconcile", err)
	}
	if strings.TrimSpace(operation) == "" {
		return CredentialResult{SecretVersion: version, Reconcile: true}, errors.New("Cloud SQL returned no owner-password operation ID; retain the version and reconcile")
	}
	if err := p.waitOperation(ctx, operation, "database-owner password update"); err != nil {
		return CredentialResult{SecretVersion: version, Reconcile: true}, err
	}
	return CredentialResult{SecretVersion: version}, nil
}

func (p CredentialProvisioner) RetireDatabaseOwnerCredential(ctx context.Context, apply bool, confirmation, secretVersion string) error {
	if !apply || confirmation != DatabaseOwnerRetireConfirmation || !IsDatabaseOwnerSecretVersion(secretVersion) {
		return errors.New("retiring the database-owner credential requires --apply and the exact ACL confirmation/version")
	}
	if p.Users == nil || p.Secrets == nil {
		return errors.New("Cloud SQL or Secret Manager API client is unavailable")
	}
	password, err := p.randomPassword()
	if err != nil {
		return err
	}
	defer clear(password)
	operation, err := p.Users.SetPassword(ctx, DatabaseOwnerLogin, password)
	if err != nil || strings.TrimSpace(operation) == "" {
		return errors.New("database-owner password rotation outcome is uncertain; keep the secret version and reconcile")
	}
	if err := p.waitOperation(ctx, operation, "database-owner password retirement"); err != nil {
		return err
	}
	if err := p.Secrets.DestroyVersion(ctx, secretVersion); err != nil {
		return errors.New("database-owner password was rotated; exact secret version cleanup needs reconciliation")
	}
	return nil
}

func (p CredentialProvisioner) randomPassword() ([]byte, error) {
	randomSource := p.Random
	if randomSource == nil {
		randomSource = rand.Reader
	}
	randomBytes := make([]byte, 32)
	if _, err := io.ReadFull(randomSource, randomBytes); err != nil {
		clear(randomBytes)
		return nil, errors.New("could not generate scratch password")
	}
	password := []byte(base64.RawURLEncoding.EncodeToString(randomBytes))
	clear(randomBytes)
	if !scratchPasswordPattern.Match(password) {
		clear(password)
		return nil, errors.New("generated scratch password failed format validation")
	}
	return password, nil
}

func (p CredentialProvisioner) waitOperation(ctx context.Context, operation, action string) error {
	poll := p.Poll
	if poll == nil {
		poll = waitPoll
	}
	for {
		done, failed, err := p.Users.GetOperation(ctx, operation)
		if err != nil {
			return fmt.Errorf("%s status is unavailable; reconcile before continuing", action)
		}
		if done {
			if failed {
				return fmt.Errorf("%s failed; reconcile before continuing", action)
			}
			return nil
		}
		if err := poll(ctx, 2*time.Second); err != nil {
			return fmt.Errorf("%s did not finish; reconcile before continuing", action)
		}
	}
}

func (p CredentialProvisioner) Provision(ctx context.Context, apply bool, confirmation string) (CredentialResult, error) {
	if !apply || confirmation != CredentialConfirmation {
		return CredentialResult{}, errors.New("creating the GCP-03 scratch login requires --apply and the exact scratch confirmation")
	}
	if p.Secrets == nil || p.Users == nil {
		return CredentialResult{}, errors.New("GCP-03 credential API clients are unavailable")
	}
	password, err := p.randomPassword()
	if err != nil {
		return CredentialResult{}, err
	}
	defer clear(password)
	version, err := p.Secrets.AddVersion(ctx, ScratchSecretResource, password)
	if err != nil {
		if strings.TrimSpace(version) != "" {
			return invalidSecretVersionResult(ScratchSecretResource, "scratch", strings.TrimSpace(version))
		}
		return CredentialResult{Reconcile: true}, withSafeCloudAPIDiagnostic("Secret Manager version creation outcome is uncertain; enumerate versions before retrying", err)
	}
	version = strings.TrimSpace(version)
	normalized, ok := NormalizeSecretVersionName(ScratchSecretResource, version)
	if !ok {
		return invalidSecretVersionResult(ScratchSecretResource, "scratch", version)
	}
	version = normalized
	operation, err := p.Users.CreateBuiltInUser(ctx, MigratorLogin, password)
	if err != nil {
		if isDefinitiveCreateFailure(err) {
			if destroyErr := p.Secrets.DestroyVersion(ctx, version); destroyErr != nil {
				return CredentialResult{SecretVersion: version, Reconcile: true}, withSafeCloudAPIDiagnostic("Cloud SQL rejected user creation and secret cleanup needs reconciliation", err)
			}
			return CredentialResult{}, withSafeCloudAPIDiagnostic("Cloud SQL rejected scratch login creation; the new secret version was destroyed", err)
		}
		return CredentialResult{SecretVersion: version, Reconcile: true}, withSafeCloudAPIDiagnostic("Cloud SQL user creation outcome is uncertain; keep the secret version and reconcile before retrying", err)
	}
	if strings.TrimSpace(operation) == "" {
		return CredentialResult{SecretVersion: version, Reconcile: true}, errors.New("Cloud SQL returned no operation ID; keep the secret version and reconcile")
	}
	poll := p.Poll
	if poll == nil {
		poll = waitPoll
	}
	for {
		done, failed, err := p.Users.GetOperation(ctx, operation)
		if err != nil {
			return CredentialResult{SecretVersion: version, Reconcile: true}, errors.New("Cloud SQL operation status is unavailable; keep the secret version and reconcile")
		}
		if done {
			if failed {
				return CredentialResult{SecretVersion: version, Reconcile: true}, errors.New("Cloud SQL user operation finished with errors; keep the secret version and reconcile before retrying")
			}
			return CredentialResult{SecretVersion: version}, nil
		}
		if err := poll(ctx, 2*time.Second); err != nil {
			return CredentialResult{SecretVersion: version, Reconcile: true}, errors.New("Cloud SQL user operation did not finish; keep the secret version and reconcile")
		}
	}
}

func isDefinitiveCreateFailure(err error) bool {
	type definitive interface{ Definitive() bool }
	var result definitive
	return errors.As(err, &result) && result.Definitive()
}

func waitPoll(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
