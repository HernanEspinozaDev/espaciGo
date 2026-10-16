package gcp03scratch

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeSecretVersions struct {
	name       string
	added      []byte
	destroyed  []string
	addErr     error
	destroyErr error
}

func (f *fakeSecretVersions) AddVersion(_ context.Context, parent string, password []byte) (string, error) {
	if parent != ScratchSecretResource {
		return "", errors.New("wrong parent")
	}
	f.added = append([]byte(nil), password...)
	return f.name, f.addErr
}

func (f *fakeSecretVersions) DestroyVersion(_ context.Context, name string) error {
	f.destroyed = append(f.destroyed, name)
	return f.destroyErr
}

type fakeSQLUsers struct {
	name      string
	password  []byte
	operation string
	done      bool
	failed    bool
	createErr error
	getErr    error
	revoked   []string
	roles     []string
}

func (f *fakeSQLUsers) CreateBuiltInUser(_ context.Context, name string, password []byte) (string, error) {
	f.name = name
	f.password = append([]byte(nil), password...)
	return f.operation, f.createErr
}

func (f *fakeSQLUsers) RevokeExistingRoles(_ context.Context, name string) (string, error) {
	f.revoked = append(f.revoked, name)
	return f.operation, f.createErr
}

func (f *fakeSQLUsers) GetDatabaseRoles(context.Context, string) ([]string, error) {
	return append([]string(nil), f.roles...), nil
}

func (f *fakeSQLUsers) GetOperation(_ context.Context, operation string) (bool, bool, error) {
	if operation != f.operation {
		return false, false, errors.New("wrong operation")
	}
	return f.done, f.failed, f.getErr
}

type definitiveFailure struct{ text string }

func (e definitiveFailure) Error() string  { return e.text }
func (definitiveFailure) Definitive() bool { return true }

func successfulProvisioner() (*CredentialProvisioner, *fakeSecretVersions, *fakeSQLUsers) {
	secret := &fakeSecretVersions{name: ScratchSecretResource + "/versions/1"}
	user := &fakeSQLUsers{operation: "op-1", done: true}
	provisioner := &CredentialProvisioner{
		Secrets: secret,
		Users:   user,
		Random:  bytes.NewReader(bytes.Repeat([]byte{0x2a}, 32)),
		Poll:    func(context.Context, time.Duration) error { return nil },
	}
	return provisioner, secret, user
}

func TestCredentialProvisionerRequiresExactConfirmationBeforeAPICalls(t *testing.T) {
	provisioner, secret, user := successfulProvisioner()
	if _, err := provisioner.Provision(context.Background(), false, CredentialConfirmation); err == nil {
		t.Fatal("missing apply gate was accepted")
	}
	if _, err := provisioner.Provision(context.Background(), true, TargetConfirmation); err == nil {
		t.Fatal("non-credential confirmation was accepted")
	}
	if secret.added != nil || user.name != "" {
		t.Fatal("API clients were called before both gates passed")
	}
}

func TestCredentialProvisionerCreatesSecretThenBoundBuiltInUserWithoutPrintingPassword(t *testing.T) {
	provisioner, secret, user := successfulProvisioner()
	result, err := provisioner.Provision(context.Background(), true, CredentialConfirmation)
	if err != nil {
		t.Fatal(err)
	}
	if result.SecretVersion != secret.name || result.Reconcile {
		t.Fatalf("unexpected result: %#v", result)
	}
	if user.name != MigratorLogin || !bytes.Equal(secret.added, user.password) {
		t.Fatal("same generated password was not used for exact secret and SQL user")
	}
	if !scratchPasswordPattern.Match(secret.added) {
		t.Fatal("generated password did not match the password-file format")
	}
	if strings.Contains(errString(err), string(secret.added)) {
		t.Fatal("password appeared in returned error")
	}
	if len(secret.destroyed) != 0 {
		t.Fatalf("successful secret version was destroyed: %v", secret.destroyed)
	}
}

func TestCredentialProvisionerDestroysSecretOnlyOnDefinitiveSQLRejection(t *testing.T) {
	provisioner, secret, _ := successfulProvisioner()
	provisioner.Users.(*fakeSQLUsers).createErr = definitiveFailure{text: "private-password"}
	result, err := provisioner.Provision(context.Background(), true, CredentialConfirmation)
	if err == nil || result.Reconcile || !strings.Contains(err.Error(), "destroyed") {
		t.Fatalf("definitive failure was not handled: %#v %v", result, err)
	}
	if len(secret.destroyed) != 1 || secret.destroyed[0] != secret.name {
		t.Fatalf("unexpected secret cleanup: %v", secret.destroyed)
	}
	if strings.Contains(err.Error(), "private-password") {
		t.Fatal("password leaked through API error")
	}
}

func TestCredentialProvisionerRetainsSecretForUncertainOperations(t *testing.T) {
	provisioner, secret, user := successfulProvisioner()
	user.createErr = errors.New("transport failed after sending private-password")
	result, err := provisioner.Provision(context.Background(), true, CredentialConfirmation)
	if err == nil || !result.Reconcile || result.SecretVersion != secret.name {
		t.Fatalf("uncertain create did not require reconciliation: %#v %v", result, err)
	}
	if len(secret.destroyed) != 0 || strings.Contains(err.Error(), "private-password") {
		t.Fatalf("uncertain create destroyed the secret or leaked a password: %v", err)
	}
}

func TestCredentialProvisionerKeepsSecretWhenOperationFinishesWithError(t *testing.T) {
	provisioner, secret, user := successfulProvisioner()
	user.failed = true
	result, err := provisioner.Provision(context.Background(), true, CredentialConfirmation)
	if err == nil || !result.Reconcile || result.SecretVersion != secret.name || len(secret.destroyed) != 0 {
		t.Fatalf("uncertain operation cleanup was not preserved for reconciliation: %#v %v", result, err)
	}
}

func TestRevokeSuperuserRequiresExactGateAndWaitsForAPIOperation(t *testing.T) {
	provisioner, secret, user := successfulProvisioner()
	if err := provisioner.RevokeSuperuser(context.Background(), true, CredentialConfirmation, secret.name); err == nil {
		t.Fatal("creation confirmation was accepted for role revocation")
	}
	if len(user.revoked) != 0 {
		t.Fatal("role API was called before the exact revocation confirmation")
	}
	if err := provisioner.RevokeSuperuser(context.Background(), true, RevokeConfirmation, secret.name); err != nil {
		t.Fatal(err)
	}
	if len(user.revoked) != 1 || user.revoked[0] != MigratorLogin {
		t.Fatalf("unexpected login role revocation target: %v", user.revoked)
	}
	if len(secret.destroyed) != 1 || secret.destroyed[0] != secret.name {
		t.Fatalf("exact secret version was not destroyed after successful revocation: %v", secret.destroyed)
	}
}

func TestScratchSecretVersionGuard(t *testing.T) {
	if !IsScratchSecretVersion(ScratchSecretResource + "/versions/17") {
		t.Fatal("exact scratch secret version rejected")
	}
	for _, candidate := range []string{
		ScratchSecretResource,
		ScratchSecretResource + "/versions/latest",
		"projects/other/secrets/name/versions/17",
	} {
		if IsScratchSecretVersion(candidate) {
			t.Fatalf("unexpected secret version accepted: %q", candidate)
		}
	}
}

func TestRevokeSuperuserRequiresAdminAPIToReportNoRemainingRoles(t *testing.T) {
	provisioner, secret, user := successfulProvisioner()
	user.roles = []string{"cloudsqlsuperuser"}
	if err := provisioner.RevokeSuperuser(context.Background(), true, RevokeConfirmation, secret.name); err == nil {
		t.Fatal("revocation succeeded while Cloud SQL still reported role membership")
	}
	if len(secret.destroyed) != 0 {
		t.Fatal("secret version destroyed while Cloud SQL roles remain")
	}
}

func TestRevokeSuperuserReportsSecretCleanupFailureForReconciliation(t *testing.T) {
	provisioner, secret, _ := successfulProvisioner()
	secret.destroyErr = errors.New("secret API failed")
	if err := provisioner.RevokeSuperuser(context.Background(), true, RevokeConfirmation, secret.name); err == nil || !strings.Contains(err.Error(), "reconciliation") {
		t.Fatalf("secret cleanup failure was not surfaced: %v", err)
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
