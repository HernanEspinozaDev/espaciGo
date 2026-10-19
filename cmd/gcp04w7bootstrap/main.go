package main

import (
	"context"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/database/cloudsql"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

//go:embed bootstrap.sql
var bootstrapSQL string

const (
	connectionName = "espacigo-testprod:southamerica-west1:espacigo-testprod-pg"
	databaseName   = "espacigo"
	apiLogin       = "espacigo_api_login"
	migratorLogin  = "espacigo_migrator_login"
)

func main() {
	mode := flag.String("mode", os.Getenv("GCP04_W7_MODE"), "bootstrap or verify-logins")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := run(ctx, *mode, os.Getenv); err != nil {
		fmt.Fprintln(os.Stderr, "W7 runtime bootstrap failed:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, mode string, getenv func(string) string) error {
	switch mode {
	case "bootstrap":
		password, err := readSecret(getenv("GCP04_W7_B1_PASSWORD_FILE"))
		if err != nil {
			return errors.New("B1 password file unavailable")
		}
		defer clear(password)
		conn, closeConnector, err := connect(ctx, "espacigo_gcp04_w7_bootstrap", string(password))
		if err != nil {
			return errors.New("private Cloud SQL connection failed")
		}
		defer closeConnector()
		defer conn.Close(context.Background())
		if _, err := conn.PgConn().Exec(ctx, bootstrapSQL).ReadAll(); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) {
				return fmt.Errorf("bootstrap SQL failed (SQLSTATE %s): %s", pgErr.Code, pgErr.Message)
			}
			return errors.New("fixed bootstrap SQL failed")
		}
		fmt.Println("W7 bootstrap completed")
		return nil
	case "verify-logins":
		for _, item := range []struct{ login, role, env string }{
			{apiLogin, "espacigo_runtime", "GCP04_W7_API_PASSWORD_FILE"},
			{migratorLogin, "espacigo_schema_owner", "GCP04_W7_MIGRATOR_PASSWORD_FILE"},
		} {
			password, err := readSecret(getenv(item.env))
			if err != nil {
				return fmt.Errorf("%s password file unavailable", item.login)
			}
			conn, closeConnector, err := connect(ctx, item.login, string(password))
			clear(password)
			if err != nil {
				return fmt.Errorf("%s Cloud SQL connection failed", item.login)
			}
			var login, membership string
			var canLogin, superuser, createdb, createrole, replication, bypassRLS, inherit, admin, memberInherit, canSet bool
			var membershipCount int
			err = conn.QueryRow(ctx, `SELECT session_user, r.rolcanlogin, r.rolsuper, r.rolcreatedb,
 r.rolcreaterole, r.rolreplication, r.rolbypassrls, r.rolinherit,
 (SELECT count(*) FROM pg_catalog.pg_auth_members m WHERE m.member=r.oid),
 (SELECT x.rolname FROM pg_catalog.pg_auth_members m JOIN pg_catalog.pg_roles x ON x.oid=m.roleid WHERE m.member=r.oid LIMIT 1),
 (SELECT m.admin_option FROM pg_catalog.pg_auth_members m WHERE m.member=r.oid LIMIT 1),
 (SELECT m.inherit_option FROM pg_catalog.pg_auth_members m WHERE m.member=r.oid LIMIT 1),
 (SELECT m.set_option FROM pg_catalog.pg_auth_members m WHERE m.member=r.oid LIMIT 1)
 FROM pg_catalog.pg_roles r WHERE r.rolname=session_user`).Scan(&login, &canLogin, &superuser, &createdb, &createrole, &replication, &bypassRLS, &inherit, &membershipCount, &membership, &admin, &memberInherit, &canSet)
			if err == nil && (login != item.login || !canLogin || superuser || createdb || createrole || replication || bypassRLS || membershipCount != 1 || membership != item.role || admin || !canSet) {
				err = errors.New("login attributes or membership differ from the least-privilege contract")
			}
			if err == nil {
				fmt.Printf("observed %s INHERIT=%t and membership INHERIT=%t with only the required role membership\n", item.login, inherit, memberInherit)
			}
			if err == nil {
				_, err = conn.Exec(ctx, "SET ROLE "+item.role)
			}
			if err == nil {
				var sessionUser, currentUser string
				err = conn.QueryRow(ctx, "SELECT session_user, current_user").Scan(&sessionUser, &currentUser)
				if err == nil && (sessionUser != item.login || currentUser != item.role) {
					err = errors.New("role identity mismatch")
				}
			}
			_ = conn.Close(context.Background())
			closeConnector()
			if err != nil {
				return fmt.Errorf("%s cannot assume its required role", item.login)
			}
			fmt.Printf("verified %s -> %s\n", item.login, item.role)
		}
		return nil
	default:
		return errors.New("mode must be bootstrap or verify-logins")
	}
}

func connect(ctx context.Context, login, password string) (*pgx.Conn, func(), error) {
	if strings.TrimSpace(password) == "" {
		return nil, nil, errors.New("empty password")
	}
	u := &url.URL{Scheme: "postgres", User: url.User(login), Host: "localhost:5432", Path: "/" + databaseName, RawQuery: "sslmode=disable"}
	return cloudsql.Connect(ctx, u.String(), password, connectionName, true)
}

func readSecret(path string) ([]byte, error) {
	if path == "" {
		return nil, errors.New("missing secret path")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0400 || info.Size() == 0 || info.Size() > 4096 {
		return nil, errors.New("secret file must be regular mode 0400 and bounded")
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 || len(data) > 4096 {
		clear(data)
		return nil, errors.New("invalid secret file")
	}
	return data, nil
}
