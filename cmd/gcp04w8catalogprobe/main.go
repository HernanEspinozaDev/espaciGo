package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"net"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/database/cloudsql"
	"github.com/jackc/pgx/v5"
)

const (
	projectID       = "espacigo-testprod"
	connectionName  = "espacigo-testprod:southamerica-west1:espacigo-testprod-pg"
	privateIP       = "10.189.0.3"
	databaseName    = "espacigo"
	databaseUser    = "espacigo_gcp04_w8_catalog_probe"
	queryPath       = "/gcp04_w8_catalog_probe.sql"
	querySHA256     = "3fc825934660bbfc965e8d97b2ef7f4648baf2cded6c8871ae6953bd18c70d16"
	maxTimeout      = 2 * time.Minute
	maxPasswordSize = 4096
)

var secretVersionPattern = regexp.MustCompile(`^[1-9][0-9]{0,18}$`)

type config struct {
	project, connection, ip, database, user, passwordFile, secretVersion, queryFile string
	timeout                                                                         time.Duration
}

type report struct {
	Format string `json:"format"`
	Target struct {
		Database      string `json:"database"`
		Login         string `json:"login"`
		ServerVersion int    `json:"server_version_num"`
		ReadOnly      bool   `json:"transaction_read_only"`
	} `json:"target"`
}

func main() {
	cfg := config{project: os.Getenv("GCP04_W8_PROJECT_ID"), connection: os.Getenv("CLOUD_SQL_INSTANCE_CONNECTION_NAME"),
		ip: os.Getenv("GCP04_W8_EXPECTED_PRIVATE_IP"), database: os.Getenv("DATABASE_NAME"),
		user: os.Getenv("DATABASE_USER"), passwordFile: os.Getenv("DATABASE_PASSWORD_FILE"),
		secretVersion: os.Getenv("GCP04_W8_SECRET_VERSION"), queryFile: queryPath, timeout: 90 * time.Second}
	var timeoutSeconds int
	flag.IntVar(&timeoutSeconds, "timeout-seconds", int(cfg.timeout.Seconds()), "single-query deadline (maximum 120 seconds)")
	flag.StringVar(&cfg.queryFile, "query-file", cfg.queryFile, "pinned W8 SQL source file")
	flag.Parse()
	cfg.timeout = time.Duration(timeoutSeconds) * time.Second
	result, err := run(context.Background(), cfg)
	if err != nil {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]string{"format": "espacigo.gcp04.w8.catalog-probe.v1", "status": "failed", "error": "catalog probe did not complete"})
		os.Exit(1)
	}
	_ = json.NewEncoder(os.Stdout).Encode(result)
}

func (c config) validate() error {
	if c.project != projectID || c.connection != connectionName || c.ip != privateIP || c.database != databaseName || c.user != databaseUser {
		return errors.New("target identity differs from the pinned W8 target")
	}
	if c.passwordFile == "" || !secretVersionPattern.MatchString(c.secretVersion) {
		return errors.New("protected password file and numeric secret version are required")
	}
	if c.timeout < time.Second || c.timeout > maxTimeout || c.queryFile == "" {
		return errors.New("query deadline or SQL source is invalid")
	}
	return nil
}

func compileQuery(source []byte) (string, error) {
	sum := sha256.Sum256(source)
	if hex.EncodeToString(sum[:]) != querySHA256 {
		return "", errors.New("W8 SQL source digest mismatch")
	}
	body := string(source)
	beginMarker := "BEGIN READ ONLY;\n"
	commitMarker := "\nCOMMIT;\n"
	if !strings.HasPrefix(body, "-- W8 first observation") || strings.Count(body, beginMarker) != 1 || !strings.HasSuffix(body, commitMarker) {
		return "", errors.New("W8 SQL wrapper differs from reviewed form")
	}
	start := strings.Index(body, beginMarker) + len(beginMarker)
	statement := strings.TrimSpace(strings.TrimSuffix(body[start:], commitMarker))
	statement = strings.TrimSuffix(statement, ";")
	if !strings.HasPrefix(statement, "WITH RECURSIVE") || strings.Contains(statement, ";") {
		return "", errors.New("W8 SQL must contain exactly one catalog SELECT statement")
	}
	for _, forbidden := range []string{"pg_settings", "pg_stat_", "pg_authid", "pg_shadow", "INSERT ", "UPDATE ", "DELETE ", "CREATE ", "ALTER ", "DROP "} {
		if strings.Contains(strings.ToLower(statement), strings.ToLower(forbidden)) {
			return "", errors.New("W8 SQL contains a forbidden catalog or mutation token")
		}
	}
	return statement, nil
}

func readPassword(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0022 != 0 || info.Size() == 0 || info.Size() > maxPasswordSize {
		return "", errors.New("mounted credential file is not protected")
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 || len(data) > maxPasswordSize {
		return "", errors.New("mounted credential file is unavailable")
	}
	password := strings.TrimSuffix(strings.TrimSuffix(string(data), "\n"), "\r")
	if password == "" || strings.ContainsAny(password, "\x00\r\n") {
		return "", errors.New("mounted credential file has invalid shape")
	}
	return password, nil
}

func run(ctx context.Context, c config) (map[string]any, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}
	source, err := os.ReadFile(c.queryFile)
	if err != nil {
		return nil, errors.New("W8 SQL source unavailable")
	}
	query, err := compileQuery(source)
	if err != nil {
		return nil, err
	}
	password, err := readPassword(c.passwordFile)
	if err != nil {
		return nil, err
	}
	deadline, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	databaseURL := "postgres://" + url.PathEscape(c.user) + "@localhost:5432/" + url.PathEscape(c.database) + "?sslmode=disable"
	conn, closeConnector, err := cloudsql.Connect(deadline, databaseURL, password, c.connection, true)
	if err != nil {
		return nil, errors.New("private Cloud SQL connector failed")
	}
	defer closeConnector()
	defer conn.Close(context.Background())
	remote, ok := conn.PgConn().Conn().RemoteAddr().(*net.TCPAddr)
	if !ok || remote.IP.String() != c.ip {
		return nil, errors.New("Cloud SQL socket did not reach the pinned private IP")
	}
	tx, err := conn.BeginTx(deadline, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, errors.New("read-only catalog transaction failed")
	}
	raw, err := queryReport(deadline, tx, query)
	if err != nil {
		_ = tx.Rollback(context.Background())
		return nil, errors.New("allowlisted catalog query failed")
	}
	if err := tx.Commit(deadline); err != nil {
		return nil, errors.New("read-only catalog transaction did not commit")
	}
	var decoded report
	if err := json.Unmarshal(raw, &decoded); err != nil || decoded.Format != "espacigo.gcp04.w8.catalog-probe.v1" || decoded.Target.Database != databaseName || decoded.Target.Login != databaseUser || decoded.Target.ServerVersion < 180000 || decoded.Target.ServerVersion >= 190000 || !decoded.Target.ReadOnly {
		return nil, errors.New("catalog report identity or read-only contract failed")
	}
	return map[string]any{"format": decoded.Format, "status": "observed", "target": decoded.Target, "report": json.RawMessage(raw), "query_sha256": querySHA256, "secret_version": c.secretVersion}, nil
}

func queryReport(ctx context.Context, tx pgx.Tx, query string) ([]byte, error) {
	var raw []byte
	err := tx.QueryRow(ctx, query).Scan(&raw)
	return raw, err
}
