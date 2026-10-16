package gcp03scratch

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"strings"
	"time"
)

const (
	ScratchSecretResource  = "projects/espacigo-testprod/secrets/espacigo-gcp03-pilot01-migrator-password"
	CredentialConfirmation = "CREATE:" + TargetConfirmation
	RevokeConfirmation     = "REVOKE:" + TargetConfirmation
)

type SecretVersionStore interface {
	AddVersion(context.Context, string, []byte) (string, error)
	DestroyVersion(context.Context, string) error
}

type CloudSQLUserStore interface {
	CreateBuiltInUser(context.Context, string, []byte) (string, error)
	RevokeExistingRoles(context.Context, string) (string, error)
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
	operation, err := p.Users.RevokeExistingRoles(ctx, MigratorLogin)
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

func IsScratchSecretVersion(name string) bool {
	prefix := ScratchSecretResource + "/versions/"
	version := strings.TrimPrefix(name, prefix)
	if version == name || version == "" {
		return false
	}
	for _, char := range version {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func (p CredentialProvisioner) Provision(ctx context.Context, apply bool, confirmation string) (CredentialResult, error) {
	if !apply || confirmation != CredentialConfirmation {
		return CredentialResult{}, errors.New("creating the GCP-03 scratch login requires --apply and the exact scratch confirmation")
	}
	if p.Secrets == nil || p.Users == nil {
		return CredentialResult{}, errors.New("GCP-03 credential API clients are unavailable")
	}
	randomSource := p.Random
	if randomSource == nil {
		randomSource = rand.Reader
	}
	randomBytes := make([]byte, 32)
	defer clear(randomBytes)
	if _, err := io.ReadFull(randomSource, randomBytes); err != nil {
		return CredentialResult{}, errors.New("could not generate scratch password")
	}
	password := []byte(base64.RawURLEncoding.EncodeToString(randomBytes))
	defer clear(password)
	if !scratchPasswordPattern.Match(password) {
		return CredentialResult{}, errors.New("generated scratch password failed format validation")
	}
	version, err := p.Secrets.AddVersion(ctx, ScratchSecretResource, password)
	if err != nil {
		return CredentialResult{Reconcile: true}, errors.New("Secret Manager version creation outcome is uncertain; enumerate versions before retrying")
	}
	version = strings.TrimSpace(version)
	if version == "" || !strings.HasPrefix(version, ScratchSecretResource+"/versions/") {
		return CredentialResult{Reconcile: true}, errors.New("secret API returned an invalid scratch version; reconcile before retrying")
	}
	operation, err := p.Users.CreateBuiltInUser(ctx, MigratorLogin, password)
	if err != nil {
		if isDefinitiveCreateFailure(err) {
			if destroyErr := p.Secrets.DestroyVersion(ctx, version); destroyErr != nil {
				return CredentialResult{SecretVersion: version, Reconcile: true}, errors.New("Cloud SQL rejected user creation and secret cleanup needs reconciliation")
			}
			return CredentialResult{}, errors.New("Cloud SQL rejected scratch login creation; the new secret version was destroyed")
		}
		return CredentialResult{SecretVersion: version, Reconcile: true}, errors.New("Cloud SQL user creation outcome is uncertain; keep the secret version and reconcile before retrying")
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
