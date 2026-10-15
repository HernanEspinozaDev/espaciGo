package adminglobal_test

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/devauth"
	password "github.com/HernanEspinozaDev/espaciGo/internal/adapters/password/bcrypt"
	identitypg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/adminglobal"
	"github.com/HernanEspinozaDev/espaciGo/internal/adminlocal"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestGlobalBootstrapDirectoryAndLastMasterConcurrency(t *testing.T) {
	adminURL := os.Getenv("TEST_DATABASE_URL")
	if adminURL == "" {
		t.Skip("TEST_DATABASE_URL is not set; needs a disposable PostgreSQL 18/PostGIS 3.6 instance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(context.Background())
	dbName := fmt.Sprintf("espacigo_gadm01_%08x", uint32(time.Now().UnixNano()))
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{dbName}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, e := admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+pgx.Identifier{dbName}.Sanitize()); e != nil {
			t.Errorf("drop isolated database: %v", e)
		}
	}()
	u, err := url.Parse(adminURL)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + dbName
	_, source, _, _ := runtime.Caller(0)
	migrationDir := filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", "db", "migrations"))
	if _, err = migrator.Run(ctx, u.String(), migrationDir); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	const owner = "00000000-0000-4000-8000-000000000001"
	const other = "00000000-0000-4000-8000-000000000002"
	const legacy = "00000000-0000-4000-8000-000000000003"
	const landlord = "00000000-0000-4000-8000-000000000004"
	const renter = "00000000-0000-4000-8000-000000000005"
	const pending = "00000000-0000-4000-8000-000000000006"
	_, err = pool.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado,creado_en)
		VALUES($1,'owner@example.invalid','owner@example.invalid','synthetic','activo','2026-01-01'),
		($2,'other@example.invalid','other@example.invalid','synthetic','activo','2026-01-01'),
		($3,'legacy@example.invalid','legacy@example.invalid','synthetic','activo','2026-01-01'),
		($4,'host@example.invalid','host@example.invalid','synthetic','activo','2026-01-01'),
		($5,'renter@example.invalid','renter@example.invalid','synthetic','activo','2026-01-01'),
		($6,'pending@example.invalid','pending@example.invalid','synthetic','correo_pendiente','2026-01-01')`, owner, other, legacy, landlord, renter, pending)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct{ id, name, role string }{{owner, "Owner Test", "arrendatario"}, {other, "Second Master", "arrendatario"}, {legacy, "Legacy Admin", "administrador"}, {landlord, "Test Host", "arrendador"}, {renter, "Test Renter", "arrendatario"}, {pending, "Pending User", "arrendatario"}} {
		if _, err = pool.Exec(ctx, `INSERT INTO public.perfil_usuario(usuario_id,nombre_visible) VALUES($1,$2)`, item.id, item.name); err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, `INSERT INTO public.rol_usuario(usuario_id,rol) VALUES($1,$2)`, item.id, item.role); err != nil {
			t.Fatal(err)
		}
	}
	corr := "10000000-0000-4000-8000-000000000001"
	if err = adminglobal.Bootstrap(ctx, pool, "00000000-0000-4000-8000-000000000099", "bootstrap", corr); err == nil {
		t.Fatal("unknown target accepted")
	}
	if err = adminglobal.Bootstrap(ctx, pool, pending, "bootstrap", corr); err == nil {
		t.Fatal("unverified target accepted")
	}
	var bootstrapWG sync.WaitGroup
	bootstrapResults := make(chan error, 2)
	for _, candidate := range []string{owner, other} {
		bootstrapWG.Add(1)
		go func(id string) {
			defer bootstrapWG.Done()
			bootstrapResults <- adminglobal.Bootstrap(ctx, pool, id, "bootstrap", corr)
		}(candidate)
	}
	bootstrapWG.Wait()
	close(bootstrapResults)
	twins, rejectedRaces := 0, 0
	for e := range bootstrapResults {
		if e == nil {
			twins++
		} else {
			rejectedRaces++
		}
	}
	if twins != 1 || rejectedRaces != 1 {
		t.Fatalf("concurrent bootstrap results success=%d rejected=%d", twins, rejectedRaces)
	}
	var master string
	if err = pool.QueryRow(ctx, `SELECT usuario_id::text FROM public.administrador_global_bootstrap WHERE singleton=true`).Scan(&master); err != nil {
		t.Fatal(err)
	}
	loser := owner
	if master == owner {
		loser = other
	}
	if err = adminglobal.Bootstrap(ctx, pool, master, "bootstrap", corr); err != nil {
		t.Fatalf("same-target replay should be idempotent: %v", err)
	}
	if err = adminglobal.Bootstrap(ctx, pool, loser, "bootstrap", corr); err == nil {
		t.Fatal("different-target replay accepted")
	}
	if _, err = pool.Exec(ctx, `DELETE FROM public.administrador_global WHERE usuario_id=$1`, master); err != nil {
		t.Fatal(err)
	}
	credentialFile := makeLocalCLICredentials(t, u.String(), dbName)
	root := repositoryRoot(source)
	if err = runBootstrapCLI(ctx, root, credentialFile, "recover", master); err != nil {
		t.Fatalf("gadm-admin recover failed to restore the bound UUID: %v", err)
	}
	var restored bool
	if err = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.administrador_global WHERE usuario_id=$1)`, master).Scan(&restored); err != nil || !restored {
		t.Fatalf("recover did not restore membership: restored=%v err=%v", restored, err)
	}
	if err = runBootstrapCLI(ctx, root, credentialFile, "recover", loser); err == nil {
		t.Fatal("gadm-admin recover accepted a different UUID")
	}
	var recoverySuccess, recoveryReject int
	if err = pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE resultado='exito'),count(*) FILTER (WHERE resultado='rechazo') FROM public.administrador_global_bootstrap_evento WHERE accion='recuperacion'`).Scan(&recoverySuccess, &recoveryReject); err != nil {
		t.Fatal(err)
	}
	if recoverySuccess != 1 || recoveryReject != 1 {
		t.Fatalf("recover audit counts success=%d rejection=%d; want 1/1", recoverySuccess, recoveryReject)
	}
	var rejected int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.administrador_global_bootstrap_evento WHERE resultado='rechazo'`).Scan(&rejected); err != nil || rejected < 3 {
		t.Fatalf("bootstrap rejects not audited: count=%d err=%v", rejected, err)
	}
	service, err := adminglobal.New(pool, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	auth, err := identity.NewAuthenticationService(identitypg.NewIdentityRepository(pool), password.Hasher{}, devauth.Mailer{}, devauth.NewIPLimiter(50), credentials.Generator{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	globalToken := issueSyntheticSession(t, ctx, pool, master, "synthetic-global-bearer")
	legacyToken := issueSyntheticSession(t, ctx, pool, legacy, "synthetic-legacy-bearer")
	if _, err = auth.AuthorizeCapability(ctx, globalToken, identity.CapabilityAccountsList, identity.UserOperation); err != nil {
		t.Fatalf("global capability denied: %v", err)
	}
	if _, err = auth.AuthorizeCapability(ctx, legacyToken, identity.CapabilityLegacyAdministration, identity.UserOperation); err != nil {
		t.Fatalf("legacy capability changed: %v", err)
	}
	if _, err = auth.AuthorizeCapability(ctx, legacyToken, identity.CapabilityAccountsList, identity.UserOperation); !errors.Is(err, identity.ErrForbidden) {
		t.Fatalf("legacy admin escalated to directory: %v", err)
	}
	if _, err = pool.Exec(ctx, `DELETE FROM public.administrador_global WHERE usuario_id=$1`, master); err != nil {
		t.Fatal(err)
	}
	if _, err = auth.AuthorizeCapability(ctx, globalToken, identity.CapabilityAccountsList, identity.UserOperation); !errors.Is(err, identity.ErrForbidden) {
		t.Fatalf("old bearer kept removed capability: %v", err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO public.administrador_global(usuario_id,concedido_en,motivo_codigo) VALUES($1,now(),'recuperacion_local')`, master); err != nil {
		t.Fatal(err)
	}
	blocker, _ := adminlocal.New(pool, time.Now)
	if _, err = blocker.BlockAccount(ctx, master, renter, "revision_administrativa", corr); err != nil {
		t.Fatal(err)
	}
	restrictedToken := issueSyntheticSession(t, ctx, pool, renter, "synthetic-restricted-bearer")
	if _, err = auth.AuthorizeCapability(ctx, restrictedToken, identity.CapabilityAccountsList, identity.UserOperation); !errors.Is(err, identity.ErrRestrictedMode) {
		t.Fatalf("restricted account gained directory access: %v", err)
	}
	page, err := service.ListAccounts(ctx, master, corr, adminglobal.AccountFilter{PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if page.MatchingTotal != 6 || len(page.Items) != 2 || page.NextCursor == "" {
		t.Fatalf("unexpected first page: %+v", page)
	}
	second, err := service.ListAccounts(ctx, master, corr, adminglobal.AccountFilter{PageSize: 2, Cursor: page.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Items) != 2 || second.Items[0].AccountID == page.Items[1].AccountID {
		t.Fatalf("keyset cursor repeated/omitted tie page: %+v", second)
	}
	filtered, err := service.ListAccounts(ctx, master, corr, adminglobal.AccountFilter{PageSize: 10, Role: identity.RoleAdministrator})
	if err != nil {
		t.Fatal(err)
	}
	if filtered.MatchingTotal != 1 || filtered.Items[0].AccountID != legacy {
		t.Fatalf("role filter leaked global bootstrap owner or missed legacy: %+v", filtered)
	}
	byEmail, err := service.ListAccounts(ctx, master, corr, adminglobal.AccountFilter{PageSize: 10, EmailPrefix: "LEGACY@EXAMPLE"})
	if err != nil || byEmail.MatchingTotal != 1 || byEmail.Items[0].AccountID != legacy {
		t.Fatalf("email prefix filter: %+v %v", byEmail, err)
	}
	byName, err := service.ListAccounts(ctx, master, corr, adminglobal.AccountFilter{PageSize: 10, NamePrefix: "second"})
	if err != nil || byName.MatchingTotal != 1 || byName.Items[0].AccountID != other {
		t.Fatalf("name prefix filter: %+v %v", byName, err)
	}
	blockedFilter, err := service.ListAccounts(ctx, master, corr, adminglobal.AccountFilter{PageSize: 10, AdministrativeBlocked: ptr(true)})
	if err != nil || blockedFilter.MatchingTotal != 1 || blockedFilter.Items[0].AccountID != renter {
		t.Fatalf("administrative blocked filter: %+v %v", blockedFilter, err)
	}
	lookup, err := service.LookupAccount(ctx, master, legacy, corr)
	if err != nil || lookup.AccountID != legacy {
		t.Fatalf("minimal detail lookup: %+v %v", lookup, err)
	}
	if _, err = service.LookupAccount(ctx, master, "00000000-0000-4000-8000-000000000099", corr); !errors.Is(err, adminglobal.ErrNotFound) {
		t.Fatalf("missing detail returned %v", err)
	}
	if _, err = service.ListAccounts(ctx, legacy, corr, adminglobal.AccountFilter{PageSize: 10}); !errors.Is(err, adminglobal.ErrForbidden) {
		t.Fatalf("legacy admin accessed global directory: %v", err)
	}
	if _, err = service.ListAccounts(ctx, renter, corr, adminglobal.AccountFilter{PageSize: 10}); !errors.Is(err, adminglobal.ErrForbidden) {
		t.Fatalf("renter accessed global directory: %v", err)
	}
	if _, err = service.ListAccounts(ctx, master, corr, adminglobal.AccountFilter{PageSize: 10, Role: identity.Role("administrador' OR true --")}); !errors.Is(err, adminglobal.ErrInvalid) {
		t.Fatalf("role filter injection not rejected: %v", err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO public.administrador_global(usuario_id,concedido_en,motivo_codigo) VALUES($1,now(),'concesion_administrativa'),($2,now(),'concesion_administrativa') ON CONFLICT DO NOTHING`, owner, other); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, pair := range [][2]string{{owner, owner}, {other, other}} {
		wg.Add(1)
		go func(actor, target string) {
			defer wg.Done()
			_, e := blocker.BlockAccount(ctx, actor, target, "revision_administrativa", corr)
			errs <- e
		}(pair[0], pair[1])
	}
	wg.Wait()
	close(errs)
	success, conflict := 0, 0
	for e := range errs {
		if e == nil {
			success++
		} else if errors.Is(e, adminlocal.ErrConflict) {
			conflict++
		} else {
			t.Errorf("unexpected concurrent block result: %v", e)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("last-master race results success=%d conflict=%d", success, conflict)
	}
	var active int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.administrador_global g JOIN public.usuario u ON u.id=g.usuario_id WHERE u.estado='activo' AND NOT EXISTS(SELECT 1 FROM public.bloqueo_cuenta_administrativo_local b WHERE b.cuenta_id=u.id)`).Scan(&active); err != nil || active != 1 {
		t.Fatalf("global admins after concurrent blocks=%d err=%v", active, err)
	}
}

func repositoryRoot(source string) string {
	return filepath.Clean(filepath.Join(filepath.Dir(source), "..", ".."))
}

func makeLocalCLICredentials(t *testing.T, databaseURL, databaseName string) string {
	t.Helper()
	root := repositoryRootFromTest(t)
	localTesting := filepath.Join(root, ".local", "testing")
	if err := os.MkdirAll(localTesting, 0o700); err != nil {
		t.Fatal(err)
	}
	directory, err := os.MkdirTemp(localTesting, "gadm01-cli-")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(directory) })
	path := filepath.Join(directory, "credentials.env")
	data := []byte("DATABASE_URL=" + databaseURL + "\nEXPECTED_DATABASE=" + databaseName + "\n")
	if err = os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func repositoryRootFromTest(t *testing.T) string {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve integration test source directory")
	}
	return repositoryRoot(source)
}

func runBootstrapCLI(ctx context.Context, root, credentialFile, command, accountID string) error {
	process := exec.CommandContext(ctx, "go", "run", "./cmd/gadm-admin", command, "--account-id", accountID, "--credentials", credentialFile)
	process.Dir = root
	process.Env = make([]string, 0, len(os.Environ())+1)
	for _, item := range os.Environ() {
		if !strings.HasPrefix(item, "GADM_BOOTSTRAP_ENV=") {
			process.Env = append(process.Env, item)
		}
	}
	process.Env = append(process.Env, "GADM_BOOTSTRAP_ENV=local-test")
	return process.Run()
}

func ptr[T any](value T) *T { return &value }

func issueSyntheticSession(t *testing.T, ctx context.Context, pool *pgxpool.Pool, accountID, token string) identity.Secret {
	t.Helper()
	id, err := (credentials.Generator{}).ID()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	hash := identity.CredentialHash(identity.Secret(token))
	if _, err = pool.Exec(ctx, `INSERT INTO public.sesion(id,usuario_id,token_hash,creada_en,ultima_actividad_en,expira_en) VALUES($1,$2,$3,$4,$4,$5)`, id, accountID, hash, now, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	return identity.Secret(token)
}
