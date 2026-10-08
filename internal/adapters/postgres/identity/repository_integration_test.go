package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	identitypg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/dbbootstrap"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"github.com/HernanEspinozaDev/espaciGo/internal/privacy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCreateWithTermsRejectsAcceptanceForAnotherAccount(t *testing.T) {
	ctx, pool := newIdentityTestPool(t)
	repo := identitypg.NewIdentityRepository(pool)

	owner := testAccount("00000000-0000-4000-8000-000000000010", "owner@ejemplo.invalid")
	if err := repo.CreateWithTerms(ctx, owner, nil); err != nil {
		t.Fatalf("create owner: %v", err)
	}

	candidate := testAccount("00000000-0000-4000-8000-000000000011", "candidate@ejemplo.invalid")
	acceptance := identity.TermsAcceptance{
		ID: "00000000-0000-4000-8000-000000000051", AccountID: owner.ID,
		VersionID: "00000000-0000-4000-8000-000000000001", Channel: "web",
		AcceptedAt: time.Date(2026, time.October, 4, 12, 0, 0, 0, time.UTC),
	}
	if err := repo.CreateWithTerms(ctx, candidate, []identity.TermsAcceptance{acceptance}); !errors.Is(err, identity.ErrInvalid) {
		t.Fatalf("CreateWithTerms error = %v, want identity.ErrInvalid for cross-account acceptance", err)
	}
	if _, err := repo.AccountByID(ctx, candidate.ID); !errors.Is(err, identity.ErrNotFound) {
		t.Fatalf("candidate account lookup error = %v, want ErrNotFound after rejected registration", err)
	}
	var acceptanceCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM public.aceptacion_terminos WHERE id = $1", acceptance.ID).Scan(&acceptanceCount); err != nil {
		t.Fatal(err)
	}
	if acceptanceCount != 0 {
		t.Fatalf("cross-account terms acceptance persisted %d rows", acceptanceCount)
	}
}

func TestCredentialCoreRuntimeLeastPrivilege(t *testing.T) {
	ctx, pool := newIdentityTestPool(t)
	if _, err := pool.Exec(ctx, `DO $$ BEGIN
IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='espacigo_runtime') THEN
    CREATE ROLE espacigo_runtime NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT;
END IF;
END $$`); err != nil {
		t.Fatalf("create disposable runtime role: %v", err)
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = dbbootstrap.GrantRuntimePermissions(ctx, conn.Conn()); err != nil {
		conn.Release()
		t.Fatalf("apply runtime permissions: %v", err)
	}
	conn.Release()
	var historySelect, historyInsert, historyDelete, historyUpdate bool
	var outboxSelect, outboxInsert, outboxUpdate, outboxDelete bool
	var auditSelect, auditInsert, auditUpdate, auditDelete bool
	err = pool.QueryRow(ctx, `SELECT
has_table_privilege('espacigo_runtime','public.historial_clave_local','SELECT'),
has_table_privilege('espacigo_runtime','public.historial_clave_local','INSERT'),
has_table_privilege('espacigo_runtime','public.historial_clave_local','DELETE'),
has_table_privilege('espacigo_runtime','public.historial_clave_local','UPDATE'),
has_table_privilege('espacigo_runtime','public.outbox_evento_local','SELECT'),
has_table_privilege('espacigo_runtime','public.outbox_evento_local','INSERT'),
has_table_privilege('espacigo_runtime','public.outbox_evento_local','UPDATE'),
has_table_privilege('espacigo_runtime','public.outbox_evento_local','DELETE'),
has_table_privilege('espacigo_runtime','public.evento_auditoria_local','SELECT'),
has_table_privilege('espacigo_runtime','public.evento_auditoria_local','INSERT'),
has_table_privilege('espacigo_runtime','public.evento_auditoria_local','UPDATE'),
has_table_privilege('espacigo_runtime','public.evento_auditoria_local','DELETE')`).Scan(
		&historySelect, &historyInsert, &historyDelete, &historyUpdate,
		&outboxSelect, &outboxInsert, &outboxUpdate, &outboxDelete,
		&auditSelect, &auditInsert, &auditUpdate, &auditDelete,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !historySelect || !historyInsert || !historyDelete || historyUpdate ||
		!outboxSelect || !outboxInsert || !outboxUpdate || outboxDelete ||
		!auditSelect || !auditInsert || auditUpdate || auditDelete {
		t.Fatalf("runtime grants history R/I/D/U=%t/%t/%t/%t outbox R/I/U/D=%t/%t/%t/%t audit R/I/U/D=%t/%t/%t/%t",
			historySelect, historyInsert, historyDelete, historyUpdate,
			outboxSelect, outboxInsert, outboxUpdate, outboxDelete,
			auditSelect, auditInsert, auditUpdate, auditDelete)
	}
}

func testAccount(id, email string) identity.Account {
	now := time.Date(2026, time.October, 4, 11, 0, 0, 0, time.UTC)
	return identity.Account{
		ID: id, Email: email, NormalizedEmail: identity.NormalizeEmail(email),
		PasswordHash: identity.Secret("synthetic-hash-not-a-credential"), State: identity.AccountEmailPending,
		CreatedAt: now, UpdatedAt: now,
	}
}

func newIdentityTestPool(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	adminURL := os.Getenv("TEST_DATABASE_URL")
	if adminURL == "" {
		t.Skip("TEST_DATABASE_URL is not set; requires disposable PostgreSQL 18")
	}
	setupCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	admin, err := pgx.Connect(setupCtx, adminURL)
	if err != nil {
		t.Fatalf("connect to disposable PostgreSQL: %v", err)
	}
	if _, err := admin.Exec(setupCtx, `DO $$ BEGIN
IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='espacigo_runtime') THEN
    CREATE ROLE espacigo_runtime NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT;
END IF;
END $$`); err != nil {
		_ = admin.Close(context.Background())
		t.Fatalf("provision runtime role in disposable PostgreSQL: %v", err)
	}
	databaseName := fmt.Sprintf("auth_be01_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(setupCtx, "CREATE DATABASE "+pgx.Identifier{databaseName}.Sanitize()); err != nil {
		_ = admin.Close(context.Background())
		t.Fatalf("create disposable test database: %v", err)
	}
	if err := admin.Close(setupCtx); err != nil {
		t.Fatalf("close PostgreSQL setup connection: %v", err)
	}

	parsedURL, err := url.Parse(adminURL)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	parsedURL.Path = "/" + databaseName
	testURL := parsedURL.String()
	migrationDir := filepath.Join("..", "..", "..", "..", "db", "migrations")

	// Register database cleanup before applying migrations; testing.T runs cleanups LIFO.
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cleanupCancel()
		admin, err := pgx.Connect(cleanupCtx, adminURL)
		if err != nil {
			t.Errorf("reconnect to drop disposable database: %v", err)
			return
		}
		defer admin.Close(context.Background())
		if _, err := admin.Exec(cleanupCtx, "DROP DATABASE IF EXISTS "+pgx.Identifier{databaseName}.Sanitize()); err != nil {
			t.Errorf("drop disposable test database: %v", err)
		}
	})

	if _, err := migrator.Run(setupCtx, testURL, migrationDir); err != nil {
		t.Fatalf("apply M01 to disposable test database: %v", err)
	}

	pool, err := pgxpool.New(context.Background(), testURL)
	if err != nil {
		t.Fatalf("create test pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return context.Background(), pool
}

func TestActionTokenRepositoryReissueAttemptsConsumptionAndEmissionCount(t *testing.T) {
	ctx, pool := newIdentityTestPool(t)
	repo := identitypg.NewIdentityRepository(pool)
	account := testAccount("00000000-0000-4000-8000-000000000020", "tokens@ejemplo.invalid")
	if err := repo.CreateWithTerms(ctx, account, nil); err != nil {
		t.Fatalf("create account: %v", err)
	}

	created := time.Date(2026, time.October, 4, 11, 0, 0, 0, time.UTC)
	first := testActionToken("00000000-0000-4000-8000-000000000030", account.ID, strings.Repeat("a", 64), "verificar_correo", created, created.Add(24*time.Hour))
	second := testActionToken("00000000-0000-4000-8000-000000000031", account.ID, strings.Repeat("b", 64), "verificar_correo", created.Add(time.Minute), created.Add(25*time.Hour))
	if err := repo.ReplaceActiveActionToken(ctx, first); err != nil {
		t.Fatalf("create first verification token: %v", err)
	}
	if err := repo.ReplaceActiveActionToken(ctx, second); err != nil {
		t.Fatalf("reissue verification token: %v", err)
	}

	storedFirst, err := repo.ActionTokenByHash(ctx, first.Hash)
	if err != nil {
		t.Fatalf("read invalidated token: %v", err)
	}
	if storedFirst.InvalidatedAt == nil || !storedFirst.InvalidatedAt.Equal(second.CreatedAt) || storedFirst.ConsumedAt != nil {
		t.Fatal("reissue did not invalidate the prior active token without marking it consumed")
	}
	count, err := repo.CountActionTokenEmissions(ctx, account.ID, "verificar_correo", created)
	if err != nil || count != 2 {
		t.Fatalf("emission count = %d, error = %v; want 2 including invalidated tokens", count, err)
	}

	for attempt := 1; attempt <= 5; attempt++ {
		accepted, err := repo.RecordActionTokenFailure(ctx, second.Hash, second.CreatedAt.Add(time.Duration(attempt)*time.Second))
		if err != nil || !accepted {
			t.Fatalf("record failed token attempt %d: accepted=%t error=%v", attempt, accepted, err)
		}
	}
	if accepted, err := repo.RecordActionTokenFailure(ctx, second.Hash, second.CreatedAt.Add(6*time.Second)); err != nil || accepted {
		t.Fatalf("sixth failed attempt: accepted=%t error=%v, want false without error", accepted, err)
	}
	storedSecond, err := repo.ActionTokenByHash(ctx, second.Hash)
	if err != nil || storedSecond.Attempts != 5 {
		t.Fatalf("stored attempt count = %d, error = %v; want 5", storedSecond.Attempts, err)
	}
	if consumed, err := repo.ConsumeActionToken(ctx, second.Hash, second.CreatedAt.Add(7*time.Second)); err != nil || consumed {
		t.Fatalf("token at failure limit consumed=%t error=%v; want false", consumed, err)
	}

	recovery := testActionToken("00000000-0000-4000-8000-000000000032", account.ID, strings.Repeat("c", 64), "recuperar_clave", created, created.Add(15*time.Minute))
	if err := repo.ReplaceActiveActionToken(ctx, recovery); err != nil {
		t.Fatalf("create recovery token: %v", err)
	}
	if consumed, err := repo.ConsumeActionToken(ctx, recovery.Hash, created.Add(14*time.Minute)); err != nil || !consumed {
		t.Fatalf("consume valid recovery token=%t error=%v; want true", consumed, err)
	}
	if consumed, err := repo.ConsumeActionToken(ctx, recovery.Hash, created.Add(14*time.Minute)); err != nil || consumed {
		t.Fatalf("consume recovery token twice=%t error=%v; want false", consumed, err)
	}
	storedRecovery, err := repo.ActionTokenByHash(ctx, recovery.Hash)
	if err != nil || storedRecovery.ConsumedAt == nil || !storedRecovery.ConsumedAt.Equal(created.Add(14*time.Minute)) {
		t.Fatalf("consumed token state missing: consumed=%t error=%v", storedRecovery.ConsumedAt != nil, err)
	}
}

func TestM02ProfileAndRightsQueriesRemainOwnerScoped(t *testing.T) {
	ctx, pool := newIdentityTestPool(t)
	repo := identitypg.NewIdentityRepository(pool)
	owner := testAccount("00000000-0000-4000-8000-000000000091", "m02-owner@ejemplo.invalid")
	if err := repo.CreateWithTerms(ctx, owner, nil); err != nil {
		t.Fatal(err)
	}
	other := testAccount("00000000-0000-4000-8000-000000000092", "m02-other@ejemplo.invalid")
	if err := repo.CreateWithTerms(ctx, other, nil); err != nil {
		t.Fatal(err)
	}
	phone := "123456789"
	profile, err := repo.SaveProfile(ctx, owner.ID, "Synthetic Owner", &phone)
	if err != nil {
		t.Fatal(err)
	}
	if profile.AccountID != owner.ID || profile.Name != "Synthetic Owner" {
		t.Fatalf("profile=%+v", profile)
	}
	if _, err := repo.GetProfile(ctx, other.ID); !errors.Is(err, privacy.ErrNotFound) {
		t.Fatalf("other account profile read err=%v", err)
	}
	item, err := repo.CreateRightsRequest(ctx, owner.ID, "supresion", "web")
	if err != nil {
		t.Fatal(err)
	}
	if item.State != "en_revision" {
		t.Fatalf("request state=%s", item.State)
	}
	items, err := repo.ListOwnRightsRequests(ctx, other.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("other account received %d requests", len(items))
	}

	export, err := repo.ExportOwnData(ctx, owner.ID)
	if err != nil {
		t.Fatalf("export own data: %v", err)
	}
	if export.Account.Email != owner.Email || export.Account.State != string(owner.State) || len(export.Roles) != 1 || export.Roles[0] != "arrendatario" || export.Profile == nil || export.Profile.Name != "Synthetic Owner" || len(export.Requests) != 1 {
		t.Fatalf("incomplete owner export: %+v", export)
	}
	otherExport, err := repo.ExportOwnData(ctx, other.ID)
	if err != nil {
		t.Fatalf("export other data: %v", err)
	}
	if otherExport.Account.Email != other.Email || otherExport.Account.Email == export.Account.Email || otherExport.Profile != nil || len(otherExport.Requests) != 0 {
		t.Fatalf("export leaked owner data into another account: %+v", otherExport)
	}
	if _, err := repo.ExportOwnData(ctx, "00000000-0000-4000-8000-000000000099"); !errors.Is(err, privacy.ErrNotFound) {
		t.Fatalf("missing account export err=%v", err)
	}
	encoded, err := json.Marshal(export)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{string(owner.PasswordHash), "password_hash", "token_hash", "sesion"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("export contains credential/internal value %q", forbidden)
		}
	}
}

func TestActionTokenReplacementRollsBackIfInsertConflicts(t *testing.T) {
	ctx, pool := newIdentityTestPool(t)
	repo := identitypg.NewIdentityRepository(pool)
	firstAccount := testAccount("00000000-0000-4000-8000-000000000040", "first@ejemplo.invalid")
	secondAccount := testAccount("00000000-0000-4000-8000-000000000041", "second@ejemplo.invalid")
	if err := repo.CreateWithTerms(ctx, firstAccount, nil); err != nil {
		t.Fatalf("create first account: %v", err)
	}
	if err := repo.CreateWithTerms(ctx, secondAccount, nil); err != nil {
		t.Fatalf("create second account: %v", err)
	}
	created := time.Date(2026, time.October, 4, 11, 0, 0, 0, time.UTC)
	occupiedHash := strings.Repeat("d", 64)
	active := testActionToken("00000000-0000-4000-8000-000000000042", firstAccount.ID, strings.Repeat("e", 64), "recuperar_clave", created, created.Add(15*time.Minute))
	occupied := testActionToken("00000000-0000-4000-8000-000000000043", secondAccount.ID, occupiedHash, "recuperar_clave", created, created.Add(15*time.Minute))
	if err := repo.ReplaceActiveActionToken(ctx, active); err != nil {
		t.Fatalf("create active token: %v", err)
	}
	if err := repo.ReplaceActiveActionToken(ctx, occupied); err != nil {
		t.Fatalf("create token owning duplicate hash: %v", err)
	}
	conflicting := testActionToken("00000000-0000-4000-8000-000000000044", firstAccount.ID, occupiedHash, "recuperar_clave", created.Add(time.Minute), created.Add(16*time.Minute))
	if err := repo.ReplaceActiveActionToken(ctx, conflicting); !errors.Is(err, identity.ErrConflict) {
		t.Fatalf("replace with duplicate hash error = %v, want ErrConflict", err)
	}
	stored, err := repo.ActionTokenByHash(ctx, active.Hash)
	if err != nil {
		t.Fatalf("read original token after rollback: %v", err)
	}
	if stored.InvalidatedAt != nil {
		t.Fatal("failed replacement committed prior-token invalidation")
	}
}

func testActionToken(id, accountID, hash, purpose string, createdAt, expiresAt time.Time) identity.ActionToken {
	return identity.ActionToken{
		ID: id, AccountID: accountID, Purpose: purpose, Hash: hash,
		CreatedAt: createdAt, ExpiresAt: expiresAt,
	}
}

func TestCreateWithTermsCommitsAccountRoleAndAcceptanceTogether(t *testing.T) {
	ctx, pool := newIdentityTestPool(t)
	repo := identitypg.NewIdentityRepository(pool)
	account := testAccount("00000000-0000-4000-8000-000000000060", "registration@ejemplo.invalid")
	acceptedAt := account.CreatedAt.Add(time.Minute)
	acceptance := identity.TermsAcceptance{
		ID: "00000000-0000-4000-8000-000000000061", AccountID: account.ID,
		VersionID: "00000000-0000-4000-8000-000000000001", AcceptedAt: acceptedAt, Channel: "web",
	}
	if err := repo.CreateWithTerms(ctx, account, []identity.TermsAcceptance{acceptance}); err != nil {
		t.Fatalf("atomic registration: %v", err)
	}

	byEmail, err := repo.AccountByNormalizedEmail(ctx, account.NormalizedEmail)
	if err != nil || byEmail.ID != account.ID || byEmail.Email != account.Email || byEmail.State != account.State {
		t.Fatalf("account round-trip mismatch or error: %v", err)
	}
	if byEmail.PasswordHash != account.PasswordHash {
		t.Fatal("password hash did not round-trip")
	}
	var tenantRoles, adminRoles, acceptedTerms int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM public.rol_usuario WHERE usuario_id=$1 AND rol='arrendatario'", account.ID).Scan(&tenantRoles); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM public.rol_usuario WHERE usuario_id=$1 AND rol='administrador'", account.ID).Scan(&adminRoles); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM public.aceptacion_terminos WHERE usuario_id=$1 AND version_id=$2", account.ID, acceptance.VersionID).Scan(&acceptedTerms); err != nil {
		t.Fatal(err)
	}
	if tenantRoles != 1 || adminRoles != 0 || acceptedTerms != 1 {
		t.Fatalf("stored registration rows tenant=%d admin=%d terms=%d", tenantRoles, adminRoles, acceptedTerms)
	}
	version, err := repo.TermsVersion(ctx, acceptance.VersionID)
	if err != nil || version.Type != "terminos" || version.Code == "" || len(version.SHA256) != 64 {
		t.Fatalf("terms version lookup failed or incomplete: error=%v", err)
	}
}

func TestCreateWithTermsRollsBackWhenVersionForeignKeyFails(t *testing.T) {
	ctx, pool := newIdentityTestPool(t)
	repo := identitypg.NewIdentityRepository(pool)
	account := testAccount("00000000-0000-4000-8000-000000000070", "rollback@ejemplo.invalid")
	acceptance := identity.TermsAcceptance{
		ID: "00000000-0000-4000-8000-000000000071", AccountID: account.ID,
		VersionID: "00000000-0000-4000-8000-000000000099", AcceptedAt: account.CreatedAt.Add(time.Minute), Channel: "web",
	}
	if err := repo.CreateWithTerms(ctx, account, []identity.TermsAcceptance{acceptance}); !errors.Is(err, identity.ErrInvalid) {
		t.Fatalf("registration error = %v, want ErrInvalid for missing terms version", err)
	}
	if _, err := repo.AccountByID(ctx, account.ID); !errors.Is(err, identity.ErrNotFound) {
		t.Fatalf("account after rolled-back registration = %v, want ErrNotFound", err)
	}
	var roles, acceptances int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM public.rol_usuario WHERE usuario_id=$1", account.ID).Scan(&roles); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM public.aceptacion_terminos WHERE usuario_id=$1", account.ID).Scan(&acceptances); err != nil {
		t.Fatal(err)
	}
	if roles != 0 || acceptances != 0 {
		t.Fatalf("failed registration left rows: roles=%d acceptances=%d", roles, acceptances)
	}
}

func TestAccountSessionAndTermsRepositoryOperations(t *testing.T) {
	ctx, pool := newIdentityTestPool(t)
	repo := identitypg.NewIdentityRepository(pool)
	account := testAccount("00000000-0000-4000-8000-000000000080", "state@ejemplo.invalid")
	if err := repo.CreateWithTerms(ctx, account, nil); err != nil {
		t.Fatalf("create account: %v", err)
	}

	blockedUntil := account.CreatedAt.Add(20 * time.Minute)
	if err := repo.SaveLoginState(ctx, account.ID, identity.AccountBlocked, 4, &blockedUntil); err != nil {
		t.Fatalf("save login state: %v", err)
	}
	storedAccount, err := repo.AccountByID(ctx, account.ID)
	if err != nil || storedAccount.State != identity.AccountBlocked || storedAccount.FailedAttempts != 4 ||
		storedAccount.BlockedUntil == nil || !storedAccount.BlockedUntil.Equal(blockedUntil) {
		t.Fatalf("login state round-trip failed: error=%v", err)
	}
	if err := repo.SaveLoginState(ctx, account.ID, identity.AccountActive, -1, nil); !errors.Is(err, identity.ErrInvalid) {
		t.Fatalf("negative attempts error = %v, want ErrInvalid", err)
	}

	privacy, err := repo.TermsVersion(ctx, "00000000-0000-4000-8000-000000000002")
	if err != nil || privacy.Type != "privacidad" {
		t.Fatalf("privacy terms version lookup error=%v", err)
	}
	acceptance := identity.TermsAcceptance{
		ID: "00000000-0000-4000-8000-000000000081", AccountID: account.ID,
		VersionID: privacy.ID, AcceptedAt: account.CreatedAt.Add(time.Minute), Channel: "api",
	}
	if err := repo.AcceptTerms(ctx, acceptance); err != nil {
		t.Fatalf("accept privacy version: %v", err)
	}
	if err := repo.AcceptTerms(ctx, acceptance); !errors.Is(err, identity.ErrConflict) {
		t.Fatalf("duplicate terms acceptance error = %v, want ErrConflict", err)
	}

	created := account.CreatedAt.Add(time.Hour)
	session := identity.Session{
		ID: "00000000-0000-4000-8000-000000000082", AccountID: account.ID,
		TokenHash: strings.Repeat("f", 64), CreatedAt: created, LastActivityAt: created,
		ExpiresAt: created.Add(8 * time.Hour), ClientSummary: "synthetic-browser",
	}
	if err := repo.CreateSession(ctx, session); err != nil {
		t.Fatalf("create session: %v", err)
	}
	storedSession, err := repo.SessionByTokenHash(ctx, session.TokenHash)
	if err != nil || storedSession.ID != session.ID || storedSession.AccountID != account.ID || storedSession.ClientSummary != session.ClientSummary {
		t.Fatalf("session lookup failed or incomplete: error=%v", err)
	}
	if active, err := repo.TouchSession(ctx, session.ID, created.Add(29*time.Minute)); err != nil || !active {
		t.Fatalf("touch active session=%t error=%v; want true", active, err)
	}
	if active, err := repo.TouchSession(ctx, session.ID, created.Add(59*time.Minute)); err != nil || active {
		t.Fatalf("touch at idle boundary=%t error=%v; want false", active, err)
	}
	if err := repo.RevokeSession(ctx, session.ID, created.Add(time.Hour)); err != nil {
		t.Fatalf("revoke session: %v", err)
	}
	storedSession, err = repo.SessionByTokenHash(ctx, session.TokenHash)
	if err != nil || storedSession.RevokedAt == nil || !storedSession.RevokedAt.Equal(created.Add(time.Hour)) {
		t.Fatalf("revoked session state missing: error=%v", err)
	}
	if active, err := repo.TouchSession(ctx, session.ID, created.Add(2*time.Hour)); err != nil || active {
		t.Fatalf("touch revoked session=%t error=%v; want false", active, err)
	}

	absolute := identity.Session{
		ID: "00000000-0000-4000-8000-000000000083", AccountID: account.ID,
		TokenHash: strings.Repeat("e", 64), CreatedAt: created, LastActivityAt: created,
		ExpiresAt: created.Add(8 * time.Hour),
	}
	if err := repo.CreateSession(ctx, absolute); err != nil {
		t.Fatalf("create absolute-expiry session: %v", err)
	}
	for elapsed := 29 * time.Minute; elapsed < 8*time.Hour; elapsed += 29 * time.Minute {
		if active, err := repo.TouchSession(ctx, absolute.ID, created.Add(elapsed)); err != nil || !active {
			t.Fatalf("refresh before absolute limit at %s: active=%t error=%v", elapsed, active, err)
		}
	}
	if active, err := repo.TouchSession(ctx, absolute.ID, created.Add(8*time.Hour)); err != nil || active {
		t.Fatalf("touch at absolute expiry=%t error=%v; want false", active, err)
	}

	tooLong := identity.Session{
		ID: "00000000-0000-4000-8000-000000000084", AccountID: account.ID,
		TokenHash: strings.Repeat("d", 64), CreatedAt: created, LastActivityAt: created,
		ExpiresAt: created.Add(8*time.Hour + time.Nanosecond),
	}
	if err := repo.CreateSession(ctx, tooLong); !errors.Is(err, identity.ErrInvalid) {
		t.Fatalf("session beyond absolute expiry error = %v, want ErrInvalid", err)
	}
}

func TestTouchSessionDoesNotMoveActivityBackwards(t *testing.T) {
	ctx, pool := newIdentityTestPool(t)
	repo := identitypg.NewIdentityRepository(pool)
	account := testAccount("00000000-0000-4000-8000-000000000090", "ordered@ejemplo.invalid")
	if err := repo.CreateWithTerms(ctx, account, nil); err != nil {
		t.Fatal(err)
	}
	session := identity.Session{
		ID: "00000000-0000-4000-8000-000000000091", AccountID: account.ID,
		TokenHash: strings.Repeat("9", 64), CreatedAt: account.CreatedAt,
		LastActivityAt: account.CreatedAt, ExpiresAt: account.CreatedAt.Add(8 * time.Hour),
	}
	if err := repo.CreateSession(ctx, session); err != nil {
		t.Fatal(err)
	}
	latest := session.CreatedAt.Add(20 * time.Minute)
	if updated, err := repo.TouchSession(ctx, session.ID, latest); err != nil || !updated {
		t.Fatalf("current activity: updated=%t error=%v", updated, err)
	}
	for _, stale := range []time.Time{session.CreatedAt.Add(10 * time.Minute), session.CreatedAt.Add(-time.Minute)} {
		if updated, err := repo.TouchSession(ctx, session.ID, stale); err != nil || updated {
			t.Fatalf("stale activity: updated=%t error=%v; want false and nil", updated, err)
		}
	}
	stored, err := repo.SessionByTokenHash(ctx, session.TokenHash)
	if err != nil || !stored.LastActivityAt.Equal(latest) || !stored.ExpiresAt.Equal(session.ExpiresAt) {
		t.Fatalf("activity or absolute expiry changed: error=%v", err)
	}
	if updated, err := repo.TouchSession(ctx, session.ID, session.CreatedAt.Add(45*time.Minute)); err != nil || !updated {
		t.Fatalf("activity within preserved idle window: updated=%t error=%v", updated, err)
	}
}

func TestReplacingActionTokensSerializesPerAccount(t *testing.T) {
	ctx, pool := newIdentityTestPool(t)
	repo := identitypg.NewIdentityRepository(pool)
	account := testAccount("00000000-0000-4000-8000-000000000090", "concurrent@ejemplo.invalid")
	if err := repo.CreateWithTerms(ctx, account, nil); err != nil {
		t.Fatalf("create account: %v", err)
	}
	if _, err := pool.Exec(ctx, `CREATE FUNCTION public.test_delay_auth_be01_token_insert() RETURNS trigger
		LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_sleep(0.05); RETURN NEW; END $$`); err != nil {
		t.Fatalf("create insert delay trigger function: %v", err)
	}
	if _, err := pool.Exec(ctx, `CREATE TRIGGER test_delay_auth_be01_token_insert
		BEFORE INSERT ON public.token_accion FOR EACH ROW
		EXECUTE FUNCTION public.test_delay_auth_be01_token_insert()`); err != nil {
		t.Fatalf("create insert delay trigger: %v", err)
	}

	created := account.CreatedAt.Add(time.Minute)
	start := make(chan struct{})
	errCh := make(chan error, 8)
	var wg sync.WaitGroup
	for i := 0; i < cap(errCh); i++ {
		token := testActionToken(
			fmt.Sprintf("00000000-0000-4000-8000-%012x", 100+i), account.ID,
			fmt.Sprintf("%064x", i+1), "recuperar_clave", created, created.Add(15*time.Minute),
		)
		wg.Add(1)
		go func(token identity.ActionToken) {
			defer wg.Done()
			<-start
			errCh <- repo.ReplaceActiveActionToken(ctx, token)
		}(token)
	}
	close(start)
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent token replacement: %v", err)
		}
	}

	var active int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.token_accion
		WHERE usuario_id=$1 AND proposito='recuperar_clave'
		AND consumido_en IS NULL AND invalidado_en IS NULL
		AND expira_en > $2 AND intentos < 5`, account.ID, created).Scan(&active); err != nil {
		t.Fatal(err)
	}
	if active != 1 {
		t.Fatalf("active recovery tokens after concurrent reissues = %d, want exactly one", active)
	}
}
