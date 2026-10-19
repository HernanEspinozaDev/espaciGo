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
	expectedProject          = "espacigo-testprod"
	expectedConnectionName   = "espacigo-testprod:southamerica-west1:espacigo-testprod-pg"
	expectedPrivateIP        = "10.189.0.3"
	expectedDatabase         = "espacigo"
	expectedDatabaseUser     = "espacigo_gcp04_w7_observer"
	expectedQuerySHA256      = "3e9783799360c4438cdb7399ec78fce6aaf8ee62d404cf70e3dfe90da6f2b869"
	defaultQueryPath         = "/target_preflight.sql"
	maximumTimeout           = 15 * time.Minute
	maximumCredentialsLength = 4096
)

var numericSecretVersion = regexp.MustCompile(`^[1-9][0-9]{0,18}$`)

type config struct {
	ProjectID           string
	ConnectionName      string
	ExpectedPrivateIP   string
	Database            string
	DatabaseUser        string
	PasswordFile        string
	SecretVersion       string
	RequiredConnections int
	Timeout             time.Duration
	Concurrency         int
	QueryFile           string
}

type output struct {
	Format      string          `json:"format"`
	Status      string          `json:"status"`
	Target      targetOutput    `json:"target"`
	QuerySHA256 string          `json:"query_sha256"`
	Report      json.RawMessage `json:"report"`
}

type targetOutput struct {
	ProjectID           string `json:"project_id"`
	InstanceConnection  string `json:"instance_connection_name"`
	PrivateIP           string `json:"private_ip"`
	Database            string `json:"database"`
	DatabaseUser        string `json:"database_user"`
	SecretVersion       string `json:"secret_version"`
	RequiredConnections int    `json:"required_connections"`
}

func main() {
	cfg := configFromEnvironment()
	var timeoutSeconds int
	flag.IntVar(&cfg.RequiredConnections, "required-connections", cfg.RequiredConnections, "approved minimum free connection budget")
	flag.IntVar(&timeoutSeconds, "timeout-seconds", int(cfg.Timeout.Seconds()), "whole-run deadline in seconds")
	flag.IntVar(&cfg.Concurrency, "concurrency", cfg.Concurrency, "must remain 1 for this single-query preflight")
	flag.StringVar(&cfg.QueryFile, "query-file", cfg.QueryFile, "pinned psql source query file")
	flag.Parse()
	cfg.Timeout = time.Duration(timeoutSeconds) * time.Second

	result, gateFailed, err := run(context.Background(), cfg)
	if err != nil {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]string{"format": "espacigo.gcp04.target-preflight.v1", "status": "failed", "error": "preflight did not complete"})
		os.Exit(1)
	}
	_ = json.NewEncoder(os.Stdout).Encode(result)
	if gateFailed {
		os.Exit(2)
	}
}

func configFromEnvironment() config {
	return config{
		ProjectID:           os.Getenv("GCP04_PREFLIGHT_PROJECT_ID"),
		ConnectionName:      os.Getenv("CLOUD_SQL_INSTANCE_CONNECTION_NAME"),
		ExpectedPrivateIP:   os.Getenv("GCP04_PREFLIGHT_EXPECTED_PRIVATE_IP"),
		Database:            os.Getenv("DATABASE_NAME"),
		DatabaseUser:        os.Getenv("DATABASE_USER"),
		PasswordFile:        os.Getenv("DATABASE_PASSWORD_FILE"),
		SecretVersion:       os.Getenv("GCP04_PREFLIGHT_SECRET_VERSION"),
		RequiredConnections: 0,
		Timeout:             120 * time.Second,
		Concurrency:         1,
		QueryFile:           defaultQueryPath,
	}
}

func (c config) validate() error {
	if c.ProjectID != expectedProject || c.ConnectionName != expectedConnectionName || !cloudsql.ValidInstanceConnectionName(c.ConnectionName) {
		return errors.New("Cloud SQL project/instance identity is not the reviewed target")
	}
	parts := strings.Split(c.ConnectionName, ":")
	if len(parts) != 3 || parts[0] != c.ProjectID || parts[1] != "southamerica-west1" || parts[2] != "espacigo-testprod-pg" {
		return errors.New("Cloud SQL connection name does not match the fixed target")
	}
	ip := net.ParseIP(c.ExpectedPrivateIP)
	if ip == nil || !ip.IsPrivate() || c.ExpectedPrivateIP != expectedPrivateIP {
		return errors.New("Cloud SQL private IP does not match the reviewed target")
	}
	if c.Database != expectedDatabase || c.DatabaseUser != expectedDatabaseUser {
		return errors.New("database identity is not the reviewed observer target")
	}
	if c.PasswordFile == "" || c.SecretVersion == "" || !numericSecretVersion.MatchString(c.SecretVersion) {
		return errors.New("numeric credential secret version and mounted file are required")
	}
	if c.RequiredConnections < 1 || c.RequiredConnections > 999999 {
		return errors.New("required connection budget must be from 1 to 999999")
	}
	if c.Timeout < time.Second || c.Timeout > maximumTimeout {
		return errors.New("preflight timeout is outside the allowed range")
	}
	if c.Concurrency != 1 {
		return errors.New("preflight concurrency must be exactly one")
	}
	if c.QueryFile == "" {
		return errors.New("pinned query file is required")
	}
	return nil
}

func readPasswordFile(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0022 != 0 {
		return "", errors.New("mounted credential file is not a protected regular file")
	}
	if info.Size() == 0 || info.Size() > maximumCredentialsLength {
		return "", errors.New("mounted credential file size is invalid")
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 || len(data) > maximumCredentialsLength {
		return "", errors.New("mounted credential file could not be read within its limit")
	}
	password := strings.TrimSuffix(strings.TrimSuffix(string(data), "\n"), "\r")
	if password == "" || strings.ContainsAny(password, "\x00\r\n") {
		return "", errors.New("mounted credential file has an invalid shape")
	}
	return password, nil
}

func compileQuery(source []byte) (string, string, error) {
	digest := sha256.Sum256(source)
	digestHex := hex.EncodeToString(digest[:])
	if digestHex != expectedQuerySHA256 {
		return "", "", errors.New("target preflight SQL digest differs from the reviewed source")
	}
	marker := "BEGIN READ ONLY;\n"
	start := strings.Index(string(source), marker)
	if start < 0 {
		return "", "", errors.New("psql read-only wrapper was not found")
	}
	body := string(source[start+len(marker):])
	if !strings.HasSuffix(body, "COMMIT;\n") {
		return "", "", errors.New("psql transaction terminator differs from the reviewed source")
	}
	body = strings.TrimSuffix(body, "COMMIT;\n")
	if strings.Contains(body, "\\") || strings.Contains(body, ":{") || strings.Contains(body, ":required_connections") {
		return "", "", errors.New("unsupported psql meta command remains in SQL body")
	}
	if strings.Count(body, ":'required_connections'") != 3 {
		return "", "", errors.New("required connection parameter shape differs from the reviewed query")
	}
	body = strings.ReplaceAll(body, ":'required_connections'", "$1")
	return strings.TrimSpace(body), digestHex, nil
}

func run(ctx context.Context, c config) (output, bool, error) {
	if err := c.validate(); err != nil {
		return output{}, false, err
	}
	source, err := os.ReadFile(c.QueryFile)
	if err != nil {
		return output{}, false, errors.New("target preflight SQL could not be read")
	}
	query, queryDigest, err := compileQuery(source)
	if err != nil {
		return output{}, false, err
	}
	password, err := readPasswordFile(c.PasswordFile)
	if err != nil {
		return output{}, false, err
	}
	deadline, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	databaseURL := "postgres://" + url.PathEscape(c.DatabaseUser) + "@localhost:5432/" + url.PathEscape(c.Database) + "?sslmode=disable"
	conn, closeConnector, err := cloudsql.Connect(deadline, databaseURL, password, c.ConnectionName, true)
	if err != nil {
		return output{}, false, errors.New("Cloud SQL private connector session failed")
	}
	defer closeConnector()
	defer conn.Close(context.Background())

	tx, err := conn.BeginTx(deadline, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return output{}, false, errors.New("read-only target transaction could not begin")
	}
	var report []byte
	if err := tx.QueryRow(deadline, query, c.RequiredConnections).Scan(&report); err != nil {
		_ = tx.Rollback(context.Background())
		return output{}, false, errors.New("target inventory query failed")
	}
	if err := tx.Commit(deadline); err != nil {
		return output{}, false, errors.New("read-only target transaction could not commit")
	}
	if len(report) == 0 || len(report) > 8<<20 || !json.Valid(report) {
		return output{}, false, errors.New("target inventory report is missing or invalid")
	}
	gateFailed, err := validateReport(report, c)
	if err != nil {
		return output{}, false, err
	}
	status := "passed"
	if gateFailed {
		status = "gate_failed"
	}
	return output{
		Format: "espacigo.gcp04.target-preflight.v1", Status: status,
		Target: targetOutput{ProjectID: c.ProjectID, InstanceConnection: c.ConnectionName,
			PrivateIP: c.ExpectedPrivateIP, Database: c.Database, DatabaseUser: c.DatabaseUser,
			SecretVersion: c.SecretVersion, RequiredConnections: c.RequiredConnections},
		QuerySHA256: queryDigest, Report: json.RawMessage(report),
	}, gateFailed, nil
}

func validateReport(raw []byte, c config) (bool, error) {
	var report struct {
		Database struct {
			Name                string `json:"name"`
			User                string `json:"user"`
			TransactionReadOnly bool   `json:"transaction_read_only"`
		} `json:"database"`
		Capacity struct {
			RequiredConnections int  `json:"required_connections"`
			Passes              bool `json:"passes"`
		} `json:"capacity"`
	}
	if err := json.Unmarshal(raw, &report); err != nil {
		return false, errors.New("target inventory report schema is invalid")
	}
	if report.Database.Name != c.Database || report.Database.User != c.DatabaseUser || !report.Database.TransactionReadOnly || report.Capacity.RequiredConnections != c.RequiredConnections {
		return false, errors.New("target SQL session identity or read-only invariant failed")
	}
	return !report.Capacity.Passes, nil
}

func runForLocalIntegration(ctx context.Context, conn *pgx.Conn, query string, required int, expectedDatabase, expectedUser string) (json.RawMessage, bool, error) {
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, false, err
	}
	var report []byte
	if err := tx.QueryRow(ctx, query, required).Scan(&report); err != nil {
		_ = tx.Rollback(context.Background())
		return nil, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, false, err
	}
	cfg := config{Database: expectedDatabase, DatabaseUser: expectedUser, RequiredConnections: required}
	gateFailed, err := validateReport(report, cfg)
	return report, gateFailed, err
}
