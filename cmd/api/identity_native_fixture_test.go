package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/devauth"
	password "github.com/HernanEspinozaDev/espaciGo/internal/adapters/password/bcrypt"
	identitypg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type nativeFixtureMailbox struct{ delivery identity.VerificationDelivery }

func (m *nativeFixtureMailbox) SendVerification(_ context.Context, d identity.VerificationDelivery) error {
	m.delivery = d
	return nil
}
func (*nativeFixtureMailbox) SendRecovery(context.Context, identity.RecoveryDelivery) error {
	return errors.New("fixture recovery is forbidden")
}
func (*nativeFixtureMailbox) SendPasswordChanged(context.Context, string) error {
	return errors.New("fixture password changes are forbidden")
}
func (*nativeFixtureMailbox) SendLoginAlert(context.Context, string, time.Time) error {
	return errors.New("fixture login alerts are not configured")
}

type nativeFixtureAuth interface {
	Register(context.Context, identity.RegisterInput) (string, error)
	VerifyEmail(context.Context, string, identity.Secret, string) error
}

func registerNativeFixture(ctx context.Context, service nativeFixtureAuth, mailbox *nativeFixtureMailbox, input identity.RegisterInput, persist func(string, bool) error) error {
	id, err := service.Register(ctx, input)
	if id != "" {
		if storeErr := persist(id, false); storeErr != nil {
			return storeErr
		}
	}
	if err != nil {
		return err
	}
	if id == "" || mailbox.delivery.TokenID == "" || mailbox.delivery.Token == "" {
		return errors.New("registration did not produce an explicit verification delivery")
	}
	if err := service.VerifyEmail(ctx, mailbox.delivery.TokenID, mailbox.delivery.Token, input.ClientIP); err != nil {
		return err
	}
	return persist(id, true)
}

func nativeFixtureReceiptBasename(slot string) (string, error) {
	switch slot {
	case "", "1":
		return "fixture.json", nil
	case "2":
		return "fixture-2.json", nil
	default:
		return "", errors.New("native fixture slot must be empty, 1 or 2")
	}
}

type nativeFixtureReceipt struct {
	Email                 string `json:"email"`
	Password              string `json:"password"`
	AccountID             string `json:"account_id"`
	Verified              bool   `json:"verified"`
	RegistrationAttempted bool   `json:"registration_attempted"`
}

func validateNativeFixtureResume(receipt nativeFixtureReceipt, storedID string) error {
	if receipt.AccountID != "" && receipt.AccountID != storedID {
		return errors.New("known fixture is missing or its identity differs")
	}
	if receipt.AccountID == "" && storedID != "" && !receipt.RegistrationAttempted {
		return errors.New("existing account cannot be adopted without a registration intent")
	}
	return nil
}

func writeNativeFixtureJSON(path string, value any) error {
	if _, err := os.Lstat(path); err == nil {
		if err := validateNativeIdentityPrivateFile(path); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	temp := path + ".pending-" + uuid.NewString()
	f, err := os.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer os.Remove(temp)
	if err := json.NewEncoder(f).Encode(value); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(temp, path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func persistNativeFixtureIdentity(receiptPath, policyPath string, receipt *nativeFixtureReceipt, policy map[string]any, id string, verified bool, write func(string, any) error) error {
	parsed, err := uuid.Parse(id)
	if err != nil || parsed.String() != id || receipt == nil || write == nil {
		return errors.New("invalid fixture persistence identity")
	}
	if receipt.AccountID != "" && receipt.AccountID != id {
		return errors.New("fixture persistence cannot replace a known identity")
	}
	auth, ok := policy["authorization"].(map[string]any)
	if !ok {
		return errors.New("invalid fixture authorization")
	}
	inventory, ok := auth["account_inventory"].([]any)
	if !ok || len(inventory) > 2 {
		return errors.New("invalid private inventory")
	}
	nextInventory := make([]any, 0, len(inventory)+1)
	seen := make(map[string]bool)
	found := false
	for _, entry := range inventory {
		m, ok := entry.(map[string]any)
		if !ok {
			return errors.New("invalid private inventory entry")
		}
		entryID, idOK := m["account_id"].(string)
		entryUUID, parseErr := uuid.Parse(entryID)
		_, verifiedOK := m["verified"].(bool)
		if !idOK || parseErr != nil || entryUUID.String() != entryID || !verifiedOK || seen[entryID] {
			return errors.New("invalid private inventory identity")
		}
		seen[entryID] = true
		nextEntry := make(map[string]any, len(m))
		for k, v := range m {
			nextEntry[k] = v
		}
		if entryID == id {
			nextEntry["verified"] = verified
			found = true
		}
		nextInventory = append(nextInventory, nextEntry)
	}
	if !found {
		if len(inventory) >= 2 {
			return errors.New("synthetic account limit reached")
		}
		nextInventory = append(nextInventory, map[string]any{"account_id": id, "verified": verified})
	}
	// The receipt is a recovery journal, not an atomic two-file transaction.
	next := *receipt
	next.AccountID, next.Verified = id, verified
	if err := write(receiptPath, next); err != nil {
		return err
	}
	*receipt = next
	auth["account_inventory"] = nextInventory
	return write(policyPath, policy)
}

func TestNativeIdentityPrepareAuthorizedFixture(t *testing.T) {
	if os.Getenv("IDENTITY_DECLARATION_NATIVE_PREPARE_FIXTURE") != "1" {
		t.Skip("native fixture registration requires explicit opt-in")
	}
	slot, err := nativeFixtureReceiptBasename(os.Getenv("IDENTITY_DECLARATION_NATIVE_FIXTURE_SLOT"))
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	policyPath := filepath.Join(root, ".local/testing/test-account-policy.json")
	if err := validateNativeIdentityPrivateFile(policyPath); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatal("private account policy could not be read")
	}
	var policy map[string]any
	if json.Unmarshal(raw, &policy) != nil {
		t.Fatal("private account policy is invalid")
	}
	target, targetOK := policy["target"].(map[string]any)
	auth, authOK := policy["authorization"].(map[string]any)
	if !targetOK || !authOK || target["database"] != "espacigo_dms_stage_20261020" || target["host"] != "127.0.0.1" || target["port"] != float64(5432) || target["expected_database_oid"] != float64(16388) || auth["maximum_new_accounts"] != float64(2) || auth["synthetic_email_domain"] != "example.invalid" || auth["synthetic_kyc_rows_write_update_delete"] != true {
		t.Fatal("private fixture policy does not authorize this target")
	}
	inventory, ok := auth["account_inventory"].([]any)
	if !ok || len(inventory) > 2 {
		t.Fatal("private fixture inventory is invalid")
	}
	private := filepath.Join(root, ".local/testing/kyc-native")
	if err := validateNativeIdentityPrivateFile(filepath.Join(private, "api.pgpass")); err != nil {
		t.Fatal(err)
	}
	receiptPath := filepath.Join(private, slot)
	var receipt nativeFixtureReceipt
	if _, err := os.Lstat(receiptPath); err == nil {
		if err := validateNativeIdentityPrivateFile(receiptPath); err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(receiptPath)
		if err != nil || json.Unmarshal(b, &receipt) != nil {
			t.Fatal("private fixture receipt is unreadable")
		}
	} else if errors.Is(err, os.ErrNotExist) {
		if len(inventory) >= 2 {
			t.Fatal("authorized new-account limit reached")
		}
		token, err := (credentials.Generator{}).Token()
		if err != nil {
			t.Fatal("synthetic credential generation failed")
		}
		receipt = nativeFixtureReceipt{Email: "kyc-native-" + uuid.NewString() + "@example.invalid", Password: "A1!" + string(token)}
		if identity.ValidatePassword(identity.Secret(receipt.Password)) != nil {
			t.Fatal("generated synthetic password does not meet policy")
		}
		if err := writeNativeFixtureJSON(receiptPath, receipt); err != nil {
			t.Fatal("private fixture intent could not be persisted")
		}
	} else {
		t.Fatal("private fixture receipt path is unavailable")
	}
	if !strings.HasPrefix(receipt.Email, "kyc-native-") || !strings.HasSuffix(receipt.Email, "@example.invalid") || identity.ValidatePassword(identity.Secret(receipt.Password)) != nil {
		t.Fatal("private fixture identity is not synthetic")
	}
	source, err := migrator.DiscoverMigrations(filepath.Join(root, "db/migrations"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err := readNativeIdentityPreflight(ctx, source, nativeIdentityPreflightVersion); err != nil {
		t.Fatal(err)
	}
	cfg, err := parseNativeIdentityConfig("postgresql://espacigo_api_login@127.0.0.1:5432/espacigo_dms_stage_20261020", filepath.Join(private, "api.pgpass"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		if _, err := conn.Exec(ctx, `SET ROLE espacigo_runtime`); err != nil {
			return err
		}
		var db, version, session, current string
		var oid int64
		if err := conn.QueryRow(ctx, `SELECT current_database(),d.oid::bigint,current_setting('server_version_num'),session_user::text,current_user::text FROM pg_catalog.pg_database d WHERE d.datname=current_database()`).Scan(&db, &oid, &version, &session, &current); err != nil {
			return err
		}
		if db != "espacigo_dms_stage_20261020" || oid != 16388 || version != "180007" || session != "espacigo_api_login" || current != "espacigo_runtime" {
			return errors.New("fixture runtime target or identity mismatch")
		}
		return nil
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal("fixture runtime pool unavailable")
	}
	defer pool.Close()
	persist := func(id string, verified bool) error {
		return persistNativeFixtureIdentity(receiptPath, policyPath, &receipt, policy, id, verified, writeNativeFixtureJSON)
	}
	var existingID, state string
	err = pool.QueryRow(ctx, `SELECT id::text,estado FROM public.usuario WHERE correo_normalizado=$1`, identity.NormalizeEmail(receipt.Email)).Scan(&existingID, &state)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		t.Fatal("own fixture lookup failed before registration")
	}
	if err := validateNativeFixtureResume(receipt, existingID); err != nil {
		t.Fatal(err)
	}
	if err == nil {
		if state == string(identity.AccountActive) {
			if err := persist(existingID, true); err != nil {
				t.Fatal("verified fixture inventory persistence failed")
			}
			return
		}
		if state != string(identity.AccountEmailPending) {
			t.Fatal("own fixture is not eligible for ordinary verification")
		}
		if err := persist(existingID, false); err != nil {
			t.Fatal("pending fixture receipt persistence failed")
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatal("own fixture lookup failed before registration")
	}
	mailbox := &nativeFixtureMailbox{}
	service, err := identity.NewAuthenticationService(identitypg.NewIdentityRepository(pool), password.Hasher{}, mailbox, devauth.NewIPLimiter(3), credentials.Generator{}, time.Now)
	if err != nil {
		t.Fatal("fixture authentication service setup failed")
	}
	if existingID != "" {
		if err := service.ReissueVerification(ctx, receipt.Email, "127.0.0.1"); err != nil {
			t.Fatal("own fixture normal verification reissue failed")
		}
		if err := service.VerifyEmail(ctx, mailbox.delivery.TokenID, mailbox.delivery.Token, "127.0.0.1"); err != nil {
			t.Fatal("own fixture normal verification failed")
		}
		if err := persist(existingID, true); err != nil {
			t.Fatal("verified fixture receipt persistence failed")
		}
	} else {
		docs, err := identity.PublicConsentDocuments()
		if err != nil {
			t.Fatal("public consent documents are unavailable")
		}
		input := identity.RegisterInput{Email: receipt.Email, Password: identity.Secret(receipt.Password), Channel: "api", ClientIP: "127.0.0.1"}
		for _, doc := range docs {
			input.TermsVersionIDs = append(input.TermsVersionIDs, doc.ID)
		}
		receipt.RegistrationAttempted = true
		if err := writeNativeFixtureJSON(receiptPath, receipt); err != nil {
			t.Fatal("registration intent could not be persisted")
		}
		if err := registerNativeFixture(ctx, service, mailbox, input, persist); err != nil {
			t.Fatal("normal synthetic registration/verification failed; private receipt retained, no blind retry")
		}
	}
	var finalState string
	if err := pool.QueryRow(ctx, `SELECT estado FROM public.usuario WHERE id=$1::uuid AND correo_normalizado=$2`, receipt.AccountID, identity.NormalizeEmail(receipt.Email)).Scan(&finalState); err != nil || finalState != string(identity.AccountActive) {
		t.Fatal("registered fixture final active-state readback failed")
	}
}
