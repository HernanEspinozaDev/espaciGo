package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/dbbootstrap"
	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"github.com/jackc/pgx/v5"
)

const expectedTarget = "espacigo_wizard_20261106"
const expectedHost = "127.0.0.1"
const expectedPort = "15436"
const expectedUser = "postgres"
const privateDir = ".local/testing/wizard-improvements"
const credentialAssignmentQuery = `SELECT format('ALTER ROLE %I PASSWORD %L',$1::text,$2::text)`

var roleFlags = []struct {
	name  string
	login bool
}{
	{"espacigo_schema_owner", false}, {"espacigo_runtime", false},
	{"espacigo_migrator_login", true}, {"espacigo_api_login", true},
	{"espacigo_gadm_bootstrap", false}, {"espacigo_privacy_cleanup_worker", false},
	{"espacigo_gcp06_test_access", false},
}

var expectedMemberships = []membershipEdge{
	{"espacigo_schema_owner", "espacigo_migrator_login", false, false, true},
	{"espacigo_runtime", "espacigo_api_login", false, false, true},
}

var aclExpectedMemberships = []membershipEdge{
	{"espacigo_schema_owner", "espacigo_migrator_login", false, false, true},
	{"espacigo_runtime", "espacigo_api_login", false, false, true},
	{"espacigo_document_verification_fn", "espacigo_schema_owner", false, false, true},
}

var verificationRoleNames = []string{"espacigo_schema_owner", "espacigo_runtime", "espacigo_migrator_login", "espacigo_api_login", "espacigo_gadm_bootstrap", "espacigo_privacy_cleanup_worker", "espacigo_gcp06_test_access", "espacigo_document_verification_fn"}

type membershipEdge struct {
	role, member                          string
	adminOption, inheritOption, setOption bool
}

type target struct{ host, port, user, database string }

type transaction interface {
	pgx.Tx
}

type rowQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func parseConfig(raw string) (string, target, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "postgres" && u.Scheme != "postgresql" || u.User == nil || u.Hostname() == "" {
		return "", target{}, errors.New("invalid DATABASE_URL")
	}
	if _, ok := u.User.Password(); ok {
		return "", target{}, errors.New("DATABASE_URL password forbidden")
	}
	if u.RawQuery == "" {
		return "", target{}, errors.New("DATABASE_URL requires sslmode=disable")
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return "", target{}, errors.New("invalid DATABASE_URL options")
	}
	for k, vs := range q {
		if k != "sslmode" || len(vs) != 1 || vs[0] != "disable" {
			return "", target{}, errors.New("unsupported DATABASE_URL option")
		}
	}
	if len(q) != 1 || u.Fragment != "" || u.Opaque != "" {
		return "", target{}, errors.New("unsupported DATABASE_URL component")
	}
	db, err := url.PathUnescape(strings.TrimPrefix(u.EscapedPath(), "/"))
	if err != nil || db == "" || strings.Contains(db, "/") {
		return "", target{}, errors.New("invalid database name")
	}
	host, port := u.Hostname(), u.Port()
	if port == "" {
		return "", target{}, errors.New("database port required")
	}
	if host != expectedHost || port != expectedPort || u.User.Username() != expectedUser || db != expectedTarget {
		return "", target{}, errors.New("DATABASE_URL target mismatch")
	}
	for _, k := range []string{"PGPASSWORD", "PGOPTIONS", "PGSERVICE", "PGSERVICEFILE"} {
		if os.Getenv(k) != "" {
			return "", target{}, errors.New("libpq override environment forbidden")
		}
	}
	return raw, target{host, port, expectedUser, db}, nil
}

func main() {
	action := flag.String("action", "", "prepare or acl")
	confirm := flag.String("confirm-target", "", "exact database confirmation")
	flag.Parse()
	if (*action != "prepare" && *action != "acl") || *confirm != expectedTarget {
		fail("refusing: require --action=prepare|acl and exact --confirm-target")
	}
	var err error
	if *action == "prepare" {
		err = prepare()
	} else {
		err = applyACL()
	}
	if err != nil {
		fail(*action + " failed: " + err.Error())
	}
	if *action == "prepare" {
		fmt.Println("prepare completed: target identity and role/password readback verified; no migrations or ACL action executed")
	} else {
		fmt.Println("ACL completed: ledger 1..83 verified; repository runtime grants committed and independently read back")
	}
}
func fail(s string) { fmt.Fprintln(os.Stderr, s); os.Exit(1) }

func prepare() error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL required")
	}
	_, t, err := parseConfig(dsn)
	if err != nil {
		return err
	}
	if err = requirePrivatePassfile(os.Getenv("PGPASSFILE")); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return errors.New("invalid connection config")
	}
	cfg.ConnectTimeout = 30 * time.Second
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return errors.New("connection failed")
	}
	defer conn.Close(context.Background())
	var db, session, current string
	var major int
	if err = conn.QueryRow(ctx, `SELECT current_database(),session_user,current_user,current_setting('server_version_num')::int/10000`).Scan(&db, &session, &current, &major); err != nil {
		return errors.New("identity query failed")
	}
	if db != t.database || session != t.user || current != t.user || major != 18 {
		return errors.New("connected server identity mismatch")
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		return errors.New("transaction begin failed")
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(context.Background())
		}
	}()
	if err = checkEmpty(ctx, tx); err != nil {
		return err
	}
	source, readErr := os.ReadFile("db/bootstrap/wizard-improvements/001_roles_extensions.sql")
	if readErr != nil {
		return errors.New("bootstrap SQL source unavailable")
	}
	body, stripErr := stripTransaction(source)
	if stripErr != nil {
		return errors.New("bootstrap SQL transaction wrapper invalid")
	}
	secrets := map[string]string{}
	files := []string{}
	rollbackFiles := func() {
		for _, p := range files {
			_ = os.Remove(p)
		}
	}
	for _, r := range roleFlags {
		if r.login {
			b := make([]byte, 32)
			if _, err = rand.Read(b); err != nil {
				rollbackFiles()
				return errors.New("credential generation failed")
			}
			secrets[r.name] = hex.EncodeToString(b)
		}
	}
	if _, err = tx.Exec(ctx, body); err != nil {
		rollbackFiles()
		return errors.New("bootstrap SQL failed")
	}
	for role, secret := range secrets {
		p, e := writePassfile(t, role, secret)
		if e != nil {
			rollbackFiles()
			return e
		}
		files = append(files, p)
	}
	for _, r := range roleFlags {
		var exists bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_catalog.pg_roles WHERE rolname=$1)`, r.name).Scan(&exists)
		if err != nil {
			rollbackFiles()
			return errors.New("role inspection failed")
		}
		if !exists {
			_, err = tx.Exec(ctx, `SELECT format('CREATE ROLE %I %s NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT', $1, $2)`, r.name, loginWord(r.login))
			if err != nil {
				rollbackFiles()
				return errors.New("role creation failed")
			}
			var stmt string
			if err = tx.QueryRow(ctx, `SELECT format('CREATE ROLE %I %s NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT',$1,$2)`, r.name, loginWord(r.login)).Scan(&stmt); err == nil {
				_, err = tx.Exec(ctx, stmt)
			}
			if err != nil {
				rollbackFiles()
				return errors.New("role creation failed")
			}
		} else {
			var login, su, dbcreate, createrole, repl, bypass, inherit bool
			err = tx.QueryRow(ctx, `SELECT rolcanlogin,rolsuper,rolcreatedb,rolcreaterole,rolreplication,rolbypassrls,rolinherit FROM pg_roles WHERE rolname=$1`, r.name).Scan(&login, &su, &dbcreate, &createrole, &repl, &bypass, &inherit)
			if err != nil || login != r.login || su || dbcreate || createrole || repl || bypass || inherit {
				rollbackFiles()
				return errors.New("existing role attributes mismatch")
			}
		}
		if secret, ok := secrets[r.name]; ok {
			var stmt string
			if err = tx.QueryRow(ctx, credentialAssignmentQuery, r.name, secret).Scan(&stmt); err == nil {
				_, err = tx.Exec(ctx, stmt)
			}
			if err != nil {
				rollbackFiles()
				return errors.New("credential assignment failed")
			}
		}
	}
	if err = verifyRoles(ctx, tx); err != nil {
		rollbackFiles()
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return commitOutcomeError()
	}
	committed = true
	if err = conn.QueryRow(ctx, `SELECT current_database(),session_user,current_user,current_setting('server_version_num')::int/10000`).Scan(&db, &session, &current, &major); err != nil || db != t.database || session != t.user || current != t.user || major != 18 {
		return errors.New("post-commit identity verification failed")
	}
	if err = verifyRoles(ctx, conn); err != nil {
		return err
	}
	return nil
}

func applyACL() error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL required")
	}
	_, t, err := parseConfig(dsn)
	if err != nil {
		return err
	}
	if err = requirePrivatePassfile(os.Getenv("PGPASSFILE")); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	plan, err := migrator.Plan(ctx, dsn, "db/migrations", "")
	if err != nil {
		return errors.New("read-only migration plan failed")
	}
	if plan.Database != t.database || plan.ServerVersion == "" || len(plan.Applied) != 83 || len(plan.Pending) != 0 || !plan.HistoryExists {
		return errors.New("migration ledger must match the complete source inventory 1..83 with no pending migrations")
	}
	for i, m := range plan.Applied {
		if m.Version != int64(i+1) || m.Name == "" || m.Checksum == "" {
			return errors.New("migration ledger is not the contiguous verified source prefix 1..83")
		}
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return errors.New("invalid connection config")
	}
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return errors.New("connection failed")
	}
	defer conn.Close(context.Background())
	var db, session, current string
	var major int
	if err = conn.QueryRow(ctx, `SELECT current_database(),session_user,current_user,current_setting('server_version_num')::int/10000`).Scan(&db, &session, &current, &major); err != nil || db != t.database || session != t.user || current != t.user || major != 18 {
		return errors.New("connected server identity mismatch")
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		return errors.New("ACL transaction begin failed")
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(context.Background())
		}
	}()
	if err = verifyRolesForACL(ctx, tx); err != nil {
		return err
	}
	if err = dbbootstrap.GrantRuntimePermissions(ctx, tx); err != nil {
		return errors.New("repository runtime ACL grants failed")
	}
	if err = dbbootstrap.VerifyRuntimePermissionCoreReadbackWithMemberships(ctx, tx, []dbbootstrap.RuntimeMembership{{Role: "espacigo_runtime", Member: "espacigo_api_login", AdminOption: false, InheritOption: false, SetOption: true}}); err != nil {
		return errors.New("pre-commit runtime ACL readback failed")
	}
	if err = verifyRolesForACL(ctx, tx); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return commitOutcomeError()
	}
	committed = true
	if err = conn.QueryRow(ctx, `SELECT current_database(),session_user,current_user,current_setting('server_version_num')::int/10000`).Scan(&db, &session, &current, &major); err != nil || db != t.database || session != t.user || current != t.user || major != 18 {
		return errors.New("post-commit identity verification failed; readback required")
	}
	if err = verifyRolesForACL(ctx, conn); err != nil {
		return err
	}
	if err = dbbootstrap.VerifyRuntimePermissionCoreReadbackWithMemberships(ctx, conn, []dbbootstrap.RuntimeMembership{{Role: "espacigo_runtime", Member: "espacigo_api_login", AdminOption: false, InheritOption: false, SetOption: true}}); err != nil {
		return errors.New("post-commit runtime ACL readback failed; readback required")
	}
	return nil
}

func commitOutcomeError() error {
	return errors.New("commit outcome uncertain; credentials retained; readback required")
}

func requirePrivatePassfile(path string) error {
	if path == "" || !filepath.IsAbs(path) {
		return errors.New("PGPASSFILE must be an absolute private passfile path")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0600 {
		return errors.New("PGPASSFILE must be a regular non-symlink file with mode 0600")
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(st.Uid) != os.Getuid() {
		return errors.New("PGPASSFILE ownership mismatch")
	}
	dir := filepath.Dir(path)
	di, err := os.Lstat(dir)
	if err != nil || !di.IsDir() || di.Mode()&os.ModeSymlink != 0 || di.Mode().Perm() != 0700 {
		return errors.New("PGPASSFILE parent must be a private non-symlink directory with mode 0700")
	}
	dst, ok := di.Sys().(*syscall.Stat_t)
	if !ok || int(dst.Uid) != os.Getuid() {
		return errors.New("PGPASSFILE parent ownership mismatch")
	}
	return nil
}
func stripTransaction(src []byte) (string, error) {
	s := strings.TrimSpace(string(src))
	upper := strings.ToUpper(s)
	if !strings.HasPrefix(upper, "--") || !strings.Contains(upper, "BEGIN;") || !strings.HasSuffix(upper, "COMMIT;") {
		return "", errors.New("transaction wrapper mismatch")
	}
	start := strings.Index(upper, "BEGIN;") + len("BEGIN;")
	end := strings.LastIndex(upper, "COMMIT;")
	if end <= start {
		return "", errors.New("transaction wrapper mismatch")
	}
	return strings.TrimSpace(s[start:end]), nil
}

func loginWord(v bool) string {
	if v {
		return "LOGIN"
	}
	return "NOLOGIN"
}

func checkEmpty(ctx context.Context, tx pgx.Tx) error {
	var bad bool
	q := `SELECT EXISTS(SELECT 1 FROM pg_catalog.pg_namespace n WHERE n.nspname='public' AND n.nspowner<>(SELECT oid FROM pg_catalog.pg_roles WHERE rolname='pg_database_owner')) OR EXISTS(SELECT 1 FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relkind IN ('r','p','v','m','f') AND c.relname<>'spatial_ref_sys' AND NOT EXISTS(SELECT 1 FROM pg_catalog.pg_depend d WHERE d.classid='pg_class'::regclass AND d.objid=c.oid AND d.refclassid='pg_extension'::regclass AND d.deptype='e')) OR EXISTS(SELECT 1 FROM pg_catalog.pg_type t JOIN pg_catalog.pg_namespace n ON n.oid=t.typnamespace WHERE n.nspname='public' AND t.typtype<>'p' AND NOT EXISTS(SELECT 1 FROM pg_catalog.pg_depend d WHERE d.classid='pg_type'::regclass AND d.objid=t.oid AND d.refclassid='pg_extension'::regclass AND d.deptype='e') AND NOT (t.typtype='c' AND EXISTS(SELECT 1 FROM pg_catalog.pg_depend d WHERE d.classid='pg_class'::regclass AND d.objid=t.typrelid AND d.refclassid='pg_extension'::regclass AND d.deptype='e')) AND NOT (t.typrelid=0 AND t.typtype='b' AND t.typelem<>0 AND EXISTS(SELECT 1 FROM pg_catalog.pg_type base WHERE base.oid=t.typelem AND base.typelem=0 AND EXISTS(SELECT 1 FROM pg_catalog.pg_depend d WHERE d.classid='pg_type'::regclass AND d.objid=base.oid AND d.refclassid='pg_extension'::regclass AND d.deptype='e')))) OR to_regclass('public.schema_migrations') IS NOT NULL`
	if err := tx.QueryRow(ctx, q).Scan(&bad); err != nil {
		return errors.New("application emptiness guard failed")
	}
	if bad {
		return errors.New("target is not application-empty")
	}
	var other bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_catalog.pg_namespace n WHERE n.nspname='public' AND n.nspname NOT IN ('information_schema') AND EXISTS(SELECT 1 FROM pg_catalog.pg_class c WHERE c.relnamespace=n.oid AND c.relname='schema_migrations'))`).Scan(&other); err != nil || other {
		return errors.New("migration ledger guard failed")
	}
	return nil
}
func verifyRoles(ctx context.Context, tx rowQuerier) error {
	if err := verifyRoleAttributes(ctx, tx); err != nil {
		return err
	}
	return verifyMemberships(ctx, tx, expectedMemberships)
}

func verifyRoleAttributes(ctx context.Context, tx rowQuerier) error {
	for _, r := range roleFlags {
		var login, su, dbcreate, createrole, repl, bypass, inherit bool
		if err := tx.QueryRow(ctx, `SELECT rolcanlogin,rolsuper,rolcreatedb,rolcreaterole,rolreplication,rolbypassrls,rolinherit FROM pg_roles WHERE rolname=$1`, r.name).Scan(&login, &su, &dbcreate, &createrole, &repl, &bypass, &inherit); err != nil || login != r.login || su || dbcreate || createrole || repl || bypass || inherit {
			return errors.New("role readback mismatch")
		}
	}
	return nil
}

func verifyMemberships(ctx context.Context, tx rowQuerier, expected []membershipEdge) error {
	var got []membershipEdge
	rows, err := tx.Query(ctx, `SELECT parent.rolname, member.rolname, m.admin_option, m.inherit_option, m.set_option FROM pg_auth_members m JOIN pg_roles parent ON parent.oid=m.roleid JOIN pg_roles member ON member.oid=m.member WHERE parent.rolname=ANY($1) OR member.rolname=ANY($1)`, verificationRoleNames)
	if err != nil {
		return errors.New("role membership readback failed")
	}
	for rows.Next() {
		var edge membershipEdge
		if err := rows.Scan(&edge.role, &edge.member, &edge.adminOption, &edge.inheritOption, &edge.setOption); err != nil {
			rows.Close()
			return errors.New("role membership readback failed")
		}
		got = append(got, edge)
	}
	rows.Close()
	if rows.Err() != nil || !sameMemberships(got, expected) {
		return errors.New("role membership readback mismatch")
	}
	return nil
}

func roleNames() []string {
	names := make([]string, 0, len(roleFlags))
	for _, r := range roleFlags {
		names = append(names, r.name)
	}
	return names
}

func verifyRolesForACL(ctx context.Context, tx rowQuerier) error {
	if err := verifyRoleAttributes(ctx, tx); err != nil {
		return err
	}
	if err := verifyMemberships(ctx, tx, aclExpectedMemberships); err != nil {
		return err
	}
	var login, su, dbcreate, createrole, repl, bypass, inherit bool
	if err := tx.QueryRow(ctx, `SELECT rolcanlogin,rolsuper,rolcreatedb,rolcreaterole,rolreplication,rolbypassrls,rolinherit FROM pg_roles WHERE rolname=$1`, "espacigo_document_verification_fn").Scan(&login, &su, &dbcreate, &createrole, &repl, &bypass, &inherit); err != nil || login || su || dbcreate || createrole || repl || bypass || inherit {
		return errors.New("helper role attributes mismatch")
	}
	return nil
}

func sameMemberships(got, want []membershipEdge) bool {
	if len(got) != len(want) {
		return false
	}
	set := make(map[membershipEdge]struct{}, len(want))
	for _, edge := range want {
		if _, ok := set[edge]; ok {
			return false
		}
		set[edge] = struct{}{}
	}
	for _, edge := range got {
		if _, ok := set[edge]; !ok {
			return false
		}
		delete(set, edge)
	}
	return len(set) == 0
}

func writePassfile(t target, role, secret string) (string, error) {
	dir := filepath.Join(".local", "testing", "wizard-improvements")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", errors.New("credential directory creation failed")
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return "", errors.New("credential directory permission failed")
	}
	name := filepath.Join(dir, role+".pgpass")
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", errors.New("credential file already exists or cannot be created")
	}
	line := net.JoinHostPort(t.host, t.port) + ":" + t.database + ":" + role + ":" + strings.NewReplacer("\\", "\\\\", ":", "\\:").Replace(secret) + "\n"
	_, err = f.WriteString(line)
	if err == nil {
		err = f.Sync()
	}
	ce := f.Close()
	if err != nil || ce != nil {
		_ = os.Remove(name)
		return "", errors.New("credential file write failed")
	}
	return name, nil
}
