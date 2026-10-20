package gcp06sqlaccess

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"regexp"
	"strings"
	"time"
)

const (
	RunID            = "20261020-gcp06a"
	ProjectID        = "espacigo-testprod"
	InstanceName     = "espacigo-gcp06-20261020-gcp06a-pg"
	DatabaseName     = "espacigo_gcp06_candidate_20261020-gcp06a"
	SecretParent     = "projects/espacigo-testprod/secrets/espacigo-test-gcp06-sql-inspector-credential"
	PrepareConfirm   = "GCP06 SQL INSPECT PREPARE 20261020-gcp06a"
	ConnectionName   = "espacigo-testprod:southamerica-west1:espacigo-gcp06-20261020-gcp06a-pg"
	ExpectedRegion   = "southamerica-west1"
	DefaultPollEvery = time.Second
	DefaultMaxPolls  = 90
)

var inspectorLoginPattern = regexp.MustCompile(`^gcp06_inspect_[a-f0-9]{12}$`)

type Version struct {
	Name  string
	State string
}

type SecretStore interface {
	GetSecret(context.Context, string) (exists bool, versionDestroyTTL string, err error)
	ListVersions(context.Context, string) ([]Version, error)
	AddVersion(context.Context, string, []byte) (string, error)
	GetVersion(context.Context, string) (string, error)
	ReadCredentialLogin(context.Context, string) (string, error)
	DestroyVersion(context.Context, string) error
}

type SQLAdmin interface {
	GetInstance(context.Context, string, string) (name, state, databaseVersion, region string, privateIP bool, err error)
	GetDatabase(context.Context, string, string, string) (string, error)
	GetUser(context.Context, string, string, string) (exists bool, userType string, err error)
	InsertUser(context.Context, string, string, string, []byte) (operation string, err error)
	DeleteUser(context.Context, string, string, string) (operation string, err error)
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
	Login         string
	SecretVersion string
	Operation     string
	Reconcile     bool
	UserDeleted   bool
	SecretDeleted bool
}

func (p Provisioner) Prepare(ctx context.Context, confirmation string) (Result, error) {
	if confirmation != PrepareConfirm {
		return Result{}, errors.New("candidate SQL inspector preparation requires the exact GCP-06 confirmation")
	}
	if p.Secrets == nil || p.SQL == nil {
		return Result{}, errors.New("Cloud SQL Admin and Secret Manager clients are required")
	}
	exists, destroyTTL, err := p.Secrets.GetSecret(ctx, SecretParent)
	if err != nil {
		return Result{}, safeFailure("could not inspect the exact candidate inspector secret", err)
	}
	if !exists || (destroyTTL != "" && destroyTTL != "0s") {
		return Result{}, errors.New("the exact candidate inspector secret must exist with immediate version destruction")
	}
	versions, err := p.Secrets.ListVersions(ctx, SecretParent)
	if err != nil {
		return Result{}, safeFailure("could not list versions of the exact candidate inspector secret", err)
	}
	if len(versions) != 0 {
		return Result{}, errors.New("candidate inspector secret already has versions or could not be inspected; reconcile before retry")
	}
	instance, state, dbVersion, region, privateIP, err := p.SQL.GetInstance(ctx, ProjectID, InstanceName)
	if err != nil {
		return Result{}, safeFailure("could not verify the exact private candidate instance", err)
	}
	if instance != InstanceName || state != "RUNNABLE" || dbVersion != "POSTGRES_18" || region != ExpectedRegion || !privateIP {
		return Result{}, errors.New("the exact private PostgreSQL 18 candidate is not confirmed RUNNABLE")
	}
	database, err := p.SQL.GetDatabase(ctx, ProjectID, InstanceName, DatabaseName)
	if err != nil {
		return Result{}, safeFailure("could not verify the exact GCP-06 candidate database", err)
	}
	if database != DatabaseName {
		return Result{}, errors.New("the exact GCP-06 candidate database is not confirmed")
	}

	random := p.Random
	if random == nil {
		random = rand.Reader
	}
	material := make([]byte, 38)
	if _, err := io.ReadFull(random, material); err != nil {
		clear(material)
		return Result{}, errors.New("could not generate one-time candidate inspector credentials")
	}
	defer clear(material)
	login := "gcp06_inspect_" + hex.EncodeToString(material[:6])
	password := []byte(base64.RawURLEncoding.EncodeToString(material[6:]))
	defer clear(password)
	if !inspectorLoginPattern.MatchString(login) || len(password) != 43 {
		return Result{}, errors.New("generated candidate inspector credential did not satisfy its fixed contract")
	}
	exists, userType, err := p.SQL.GetUser(ctx, ProjectID, InstanceName, login)
	if err != nil {
		return Result{}, safeFailure("could not verify that the generated inspector login is absent", err)
	}
	if exists || userType != "" {
		return Result{}, errors.New("generated candidate inspector login already exists or its state is uncertain")
	}
	payload := credentialPayload(login, password)
	defer clear(payload)
	version, err := p.Secrets.AddVersion(ctx, SecretParent, payload)
	if err != nil {
		return Result{Login: login, Reconcile: true}, safeFailure("Secret Manager addVersion did not return a confirmed result; do not retry; reconcile version names on the exact candidate secret", err)
	}
	version = strings.TrimSpace(version)
	if !validVersionName(version) {
		return Result{Login: login, Reconcile: true}, errors.New("Secret Manager returned an unexpected version name; do not retry; reconcile the exact secret")
	}
	result := Result{Login: login, SecretVersion: version, Reconcile: true}
	operation, err := p.SQL.InsertUser(ctx, ProjectID, InstanceName, login, password)
	if err != nil {
		return result, safeFailure("Cloud SQL users.insert result is uncertain or rejected; do not retry; use exact login and secret version for reconciliation", err)
	}
	operation = strings.TrimSpace(operation)
	if operation == "" || strings.Contains(operation, "/") {
		return result, errors.New("Cloud SQL returned no safe user-create operation ID; do not retry")
	}
	result.Operation = operation
	done, failed, err := p.waitOperation(ctx, operation)
	if err != nil {
		return result, safeFailure("Cloud SQL user-create operation is incomplete or uncertain; do not retry; retain exact reconciliation IDs", err)
	}
	if !done || failed {
		return result, errors.New("Cloud SQL user-create operation is incomplete or uncertain; do not retry; retain exact reconciliation IDs")
	}
	result.Reconcile = false
	return result, nil
}

func (p Provisioner) Cleanup(ctx context.Context, login, version, confirmation string) (Result, error) {
	if !inspectorLoginPattern.MatchString(login) || !validVersionName(version) || confirmation != CleanupConfirmation(login, version) {
		return Result{}, errors.New("cleanup requires the exact GCP-06 temporary login, secret version, and confirmation")
	}
	if p.Secrets == nil || p.SQL == nil {
		return Result{}, errors.New("Cloud SQL Admin and Secret Manager clients are required")
	}
	state, err := p.Secrets.GetVersion(ctx, version)
	if err != nil {
		return Result{Login: login, SecretVersion: version, Reconcile: true}, safeFailure("could not inspect the exact candidate inspector secret version", err)
	}
	if state != "ENABLED" && state != "DISABLED" && state != "DESTROYED" {
		return Result{Login: login, SecretVersion: version, Reconcile: true}, errors.New("the exact candidate inspector secret version state is unavailable")
	}
	result := Result{Login: login, SecretVersion: version, Reconcile: true}
	exists, userType, err := p.SQL.GetUser(ctx, ProjectID, InstanceName, login)
	if err != nil {
		return result, safeFailure("could not verify the exact candidate inspector SQL user", err)
	}
	if exists && userType != "BUILT_IN" {
		return result, errors.New("the exact candidate inspector SQL user could not be verified")
	}
	if state == "DESTROYED" {
		if exists {
			return result, errors.New("secret version is destroyed while the candidate SQL login still exists; stop for reconciliation")
		}
		result.UserDeleted = true
		result.SecretDeleted = true
		result.Reconcile = false
		return result, nil
	}
	storedLogin, err := p.Secrets.ReadCredentialLogin(ctx, version)
	if err != nil {
		return result, safeFailure("could not validate the exact one-time secret/login binding", err)
	}
	if storedLogin != login {
		return result, errors.New("the exact secret version is not bound to the requested one-time SQL login")
	}
	if exists {
		operation, err := p.SQL.DeleteUser(ctx, ProjectID, InstanceName, login)
		if err != nil {
			return result, safeFailure("Cloud SQL users.delete result is uncertain; do not retry; verify the exact login", err)
		}
		operation = strings.TrimSpace(operation)
		if operation == "" || strings.Contains(operation, "/") {
			return result, errors.New("Cloud SQL returned no safe user-delete operation ID; do not retry")
		}
		result.Operation = operation
		done, failed, err := p.waitOperation(ctx, operation)
		if err != nil {
			return result, safeFailure("Cloud SQL user-delete operation is incomplete or uncertain; verify before retry", err)
		}
		if !done || failed {
			return result, errors.New("Cloud SQL user-delete operation is incomplete or uncertain; verify before retry")
		}
	}
	exists, userType, err = p.SQL.GetUser(ctx, ProjectID, InstanceName, login)
	if err != nil {
		return result, safeFailure("could not verify deletion of the exact one-time inspector login", err)
	}
	if exists || userType != "" {
		return result, errors.New("Cloud SQL did not verify deletion of the exact one-time inspector login")
	}
	result.UserDeleted = true
	if state != "DESTROYED" {
		if err := p.Secrets.DestroyVersion(ctx, version); err != nil {
			return result, safeFailure("Secret Manager destroy result is uncertain; do not retry; verify the exact version", err)
		}
		state, err = p.Secrets.GetVersion(ctx, version)
		if err != nil {
			return result, safeFailure("could not verify destruction of the exact one-time credential version", err)
		}
		if state != "DESTROYED" {
			return result, errors.New("Secret Manager did not verify destruction of the exact one-time credential version")
		}
	}
	result.SecretDeleted = true
	result.Reconcile = false
	return result, nil
}

func (p Provisioner) waitOperation(ctx context.Context, operation string) (bool, bool, error) {
	interval := p.PollInterval
	if interval <= 0 {
		interval = DefaultPollEvery
	}
	maxPolls := p.MaxPolls
	if maxPolls <= 0 {
		maxPolls = DefaultMaxPolls
	}
	for range maxPolls {
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return false, false, ctx.Err()
		case <-timer.C:
		}
		done, failed, err := p.SQL.GetOperation(ctx, ProjectID, operation)
		if err != nil {
			return false, false, err
		}
		if done {
			return true, failed, nil
		}
	}
	return false, false, errors.New("Cloud SQL operation poll limit reached")
}

func CleanupConfirmation(login, version string) string {
	return "GCP06 SQL INSPECT CLEANUP " + RunID + " " + login + " " + VersionNumber(CanonicalVersion(version))
}

func credentialPayload(login string, password []byte) []byte {
	payload := make([]byte, 0, len(login)+len(password)+27)
	payload = append(payload, `{"username":"`...)
	payload = append(payload, login...)
	payload = append(payload, `","password":"`...)
	payload = append(payload, password...)
	payload = append(payload, '"', '}')
	return payload
}

func validVersionName(name string) bool {
	parts := strings.Split(name, "/")
	if len(parts) != 6 || parts[0] != "projects" || parts[1] != ProjectID || parts[2] != "secrets" || parts[3] != "espacigo-test-gcp06-sql-inspector-credential" || parts[4] != "versions" {
		return false
	}
	version := parts[5]
	if version == "" || version == "0" || version[0] == '0' {
		return false
	}
	for _, digit := range version {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}

func VersionNumber(name string) string {
	if !validVersionName(name) {
		return ""
	}
	return strings.TrimPrefix(name, SecretParent+"/versions/")
}

func CanonicalVersion(value string) string {
	if validVersionName(value) {
		return value
	}
	if value == "" || value == "0" || value[0] == '0' {
		return ""
	}
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return ""
		}
	}
	return SecretParent + "/versions/" + value
}

func IsInspectorLogin(value string) bool { return inspectorLoginPattern.MatchString(value) }

func safeFailure(message string, err error) error {
	var safe interface{ SafeCloudError() string }
	if errors.As(err, &safe) {
		return errors.New(message + ": " + safe.SafeCloudError())
	}
	return errors.New(message)
}
