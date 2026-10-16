// Package gcp03restorecredential provisions the one-time postgres password
// for the isolated g03r-1016 restore rehearsal. It has no configurable cloud
// target: project, instance, database, user, and secret are fixed below.
package gcp03restorecredential

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
	ProjectID     = "espacigo-testprod"
	ProjectNumber = "447373903153"
	RunID         = "g03r-1016"
	InstanceName  = "espacigo-gcp03-restore-g03r-1016-pg"
	DatabaseName  = "espacigo_scratch"
	UserName      = "postgres"
	SecretParent  = "projects/espacigo-testprod/secrets/espacigo-gcp03-restore-g03r-1016-postgres-password"
	Confirmation  = "SET-POSTGRES-PASSWORD:" + ProjectID + "/" + RunID + "/" + InstanceName + "/" + DatabaseName
)

type SecretStore interface {
	ListVersions(context.Context, string) ([]string, error)
	AddVersion(context.Context, string, []byte) (string, error)
	DestroyVersion(context.Context, string) error
}

type SQLAdmin interface {
	GetInstance(context.Context, string, string) (name, state string, err error)
	GetDatabase(context.Context, string, string, string) (string, error)
	GetUser(context.Context, string, string, string) (name, userType string, err error)
	UpdatePassword(context.Context, string, string, string, []byte) (string, error)
	GetOperation(context.Context, string, string) (done, failed bool, err error)
}

type Provisioner struct {
	Secrets      SecretStore
	SQL          SQLAdmin
	Random       io.Reader
	PollInterval time.Duration
	MaxPolls     int
}

type Result struct {
	SecretVersion string
	Operation     string
	Reconcile     bool
}

// Provision stores one generated password and sets the built-in postgres
// password to the same bytes. An uncertain result always retains the exact
// version for reconciliation; only a definitive SQL request rejection before
// an operation is accepted can trigger deletion of this call's new version.
func (p Provisioner) Provision(ctx context.Context, apply bool, confirmation string) (Result, error) {
	if !apply || confirmation != Confirmation {
		return Result{}, errors.New("setting the isolated restore password requires --apply and the exact run confirmation")
	}
	if p.Secrets == nil || p.SQL == nil {
		return Result{}, errors.New("Cloud SQL Admin or Secret Manager API client is unavailable")
	}
	versions, err := p.Secrets.ListVersions(ctx, SecretParent)
	if err != nil {
		return Result{}, errors.New("could not inspect the exact run-scoped password secret; reconcile before retrying")
	}
	if len(versions) != 0 {
		return Result{}, errors.New("the run-scoped password secret already has versions; reconcile before retrying")
	}
	instance, state, err := p.SQL.GetInstance(ctx, ProjectID, InstanceName)
	if err != nil || instance != InstanceName || state != "RUNNABLE" {
		return Result{}, errors.New("the exact run-scoped Cloud SQL instance is not confirmed RUNNABLE")
	}
	database, err := p.SQL.GetDatabase(ctx, ProjectID, InstanceName, DatabaseName)
	if err != nil || database != DatabaseName {
		return Result{}, errors.New("the exact run-scoped scratch database is not confirmed")
	}
	user, userType, err := p.SQL.GetUser(ctx, ProjectID, InstanceName, UserName)
	if err != nil || user != UserName || userType != "BUILT_IN" {
		return Result{}, errors.New("the exact built-in postgres user is not confirmed on the run-scoped instance")
	}

	random := p.Random
	if random == nil {
		random = rand.Reader
	}
	raw := make([]byte, 32)
	if _, err := io.ReadFull(random, raw); err != nil {
		clear(raw)
		return Result{}, errors.New("could not generate the isolated scratch password")
	}
	password := []byte(base64.RawURLEncoding.EncodeToString(raw))
	clear(raw)
	defer clear(password)
	if !isGeneratedPassword(password) {
		return Result{}, errors.New("generated scratch password did not meet the fixed length")
	}

	version, err := p.Secrets.AddVersion(ctx, SecretParent, password)
	if err != nil {
		if isDefinitive(err) {
			return Result{}, errors.New("Secret Manager definitively rejected the run-scoped version request; no Cloud SQL update was sent")
		}
		return Result{Reconcile: true}, errors.New("Secret Manager version result is uncertain; inspect the exact secret before any retry")
	}
	version = strings.TrimSpace(version)
	if !isExactVersion(version) {
		return Result{Reconcile: true}, errors.New("Secret Manager returned an unexpected version name; reconcile the exact run-scoped secret before retrying")
	}
	result := Result{SecretVersion: version, Reconcile: true}

	operation, err := p.SQL.UpdatePassword(ctx, ProjectID, InstanceName, UserName, password)
	if err != nil {
		if isDefinitive(err) {
			return p.destroyCreatedVersion(ctx, result, "Cloud SQL definitively rejected the postgres password update")
		}
		return result, errors.New("Cloud SQL password update result is uncertain; retain the returned secret version for reconciliation")
	}
	operation = strings.TrimSpace(operation)
	if operation == "" || strings.Contains(operation, "/") {
		return result, errors.New("Cloud SQL returned no safe operation ID; retain the secret version and reconcile")
	}
	result.Operation = operation

	interval := p.PollInterval
	if interval <= 0 {
		interval = 2 * time.Second
	}
	maxPolls := p.MaxPolls
	if maxPolls <= 0 {
		maxPolls = 300
	}
	for range maxPolls {
		if err := wait(ctx, interval); err != nil {
			return result, errors.New("Cloud SQL operation polling ended uncertain; retain the secret version and reconcile")
		}
		done, failed, err := p.SQL.GetOperation(ctx, ProjectID, operation)
		if err != nil {
			return result, errors.New("Cloud SQL operation status is uncertain; retain the secret version and reconcile")
		}
		if !done {
			continue
		}
		if failed {
			// The operation completed with an error, but the API does not prove
			// whether the account update was partially applied. Retain the exact
			// secret version so an operator can reconcile without losing access.
			return result, errors.New("Cloud SQL completed the password update with an error; retain the secret version and reconcile before cleanup or retry")
		}
		result.Reconcile = false
		return result, nil
	}
	return result, errors.New("Cloud SQL operation did not finish within the bounded poll window; retain the secret version and reconcile")
}

func (p Provisioner) destroyCreatedVersion(ctx context.Context, result Result, message string) (Result, error) {
	if err := p.Secrets.DestroyVersion(ctx, result.SecretVersion); err != nil {
		return result, fmt.Errorf("%s; exact newly-created secret version cleanup is uncertain, reconcile numeric version %s", message, SecretVersionNumber(result.SecretVersion))
	}
	return Result{}, errors.New(message + "; the exact newly-created secret version was destroyed")
}

type definitive interface{ Definitive() bool }

func isDefinitive(err error) bool {
	var classified definitive
	return errors.As(err, &classified) && classified.Definitive()
}

func isExactVersion(name string) bool {
	parts := strings.Split(name, "/")
	if len(parts) != 6 || parts[0] != "projects" || (parts[1] != ProjectID && parts[1] != ProjectNumber) || parts[2] != "secrets" || parts[3] != strings.TrimPrefix(SecretParent, "projects/"+ProjectID+"/secrets/") || parts[4] != "versions" {
		return false
	}
	n, err := strconv.ParseUint(parts[5], 10, 64)
	return err == nil && n > 0 && strconv.FormatUint(n, 10) == parts[5]
}

// SecretVersionNumber returns only the positive numeric version for the
// package's single fixed Secret Manager resource.
func SecretVersionNumber(name string) string {
	if !isExactVersion(name) {
		return ""
	}
	parts := strings.Split(name, "/")
	return parts[5]
}

func isGeneratedPassword(password []byte) bool {
	if len(password) != 43 {
		return false
	}
	decoded := make([]byte, 32)
	n, err := base64.RawURLEncoding.Strict().Decode(decoded, password)
	if err != nil || n != 32 {
		clear(decoded)
		return false
	}
	clear(decoded)
	return true
}

func wait(ctx context.Context, interval time.Duration) error {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
