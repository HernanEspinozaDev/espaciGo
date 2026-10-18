package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/cloudstorage"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/evidencefs"
	identitypg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/database/cloudsql"
	"github.com/HernanEspinozaDev/espaciGo/internal/privacy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultPollInterval = 10 * time.Second
	maxDeleteDuration   = 30 * time.Second
)

func main() {
	if err := run(); err != nil {
		log.Print("privacy cleanup worker stopped; inspect service configuration and health")
		os.Exit(1)
	}
}

func run() error {
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		return errors.New("invalid worker configuration")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	poolConfig, err := pgxpool.ParseConfig(cfg.databaseURL())
	if err != nil {
		return errors.New("invalid database connection configuration")
	}
	poolConfig.MaxConns = 2
	if cfg.databaseSetRole != "" {
		poolConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
			_, err := conn.Exec(ctx, `SET ROLE espacigo_privacy_cleanup_worker`)
			return err
		}
	}
	var closeConnector func()
	closeConnector = func() {}
	if cfg.connectionMode == "cloudsql" {
		closeConnector, err = cloudsql.ConfigurePGX(ctx, poolConfig.ConnConfig, cfg.cloudSQLInstance, cfg.cloudSQLPrivateIP)
		if err != nil {
			return errors.New("Cloud SQL connector initialization failed")
		}
	}
	defer closeConnector()
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return errors.New("database pool initialization failed")
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return errors.New("database is not reachable")
	}

	cleaner, err := newCleaner(ctx, cfg)
	if err != nil {
		return errors.New("private evidence store initialization failed")
	}
	if closer, ok := cleaner.(interface{ Close() error }); ok {
		defer closer.Close()
	}
	var repository privacy.Repository = identitypg.NewIdentityRepository(pool)
	if cfg.databaseSetRole != "" {
		repository = identitypg.NewSuppressionCleanupWorkerRepository(pool)
	}
	service, err := privacy.NewService(repository)
	if err != nil {
		return errors.New("privacy cleanup service initialization failed")
	}
	workerID, err := privacy.NewSuppressionCleanupWorkerID()
	if err != nil {
		return errors.New("privacy cleanup worker identity unavailable")
	}

	process := func() {
		if err := service.RunSuppressionCleanupOnceWithWorker(ctx, workerID, time.Now, timedCleaner{store: cleaner}); err != nil {
			log.Print("privacy cleanup cycle failed; pending work remains durable")
		}
	}
	process()
	ticker := time.NewTicker(cfg.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			process()
		}
	}
}

type workerConfig struct {
	connectionMode    string
	dbHost            string
	dbPort            uint16
	dbName            string
	dbUser            string
	dbPassword        string
	dbSSLMode         string
	dbSSLRootCert     string
	dbSSLCert         string
	dbSSLKey          string
	databaseSetRole   string
	cloudSQLInstance  string
	cloudSQLPrivateIP bool
	storageBackend    string
	storageBucket     string
	storagePrefix     string
	evidenceRoot      string
	interval          time.Duration
}

func loadConfig(getenv func(string) string) (workerConfig, error) {
	cfg := workerConfig{connectionMode: strings.TrimSpace(getenv("DATABASE_CONNECTION_MODE")), interval: defaultPollInterval}
	if cfg.connectionMode == "" {
		cfg.connectionMode = "direct"
	}
	if cfg.connectionMode != "direct" && cfg.connectionMode != "cloudsql" {
		return workerConfig{}, errors.New("invalid database connection mode")
	}
	if cfg.connectionMode == "cloudsql" {
		cfg.cloudSQLInstance = strings.TrimSpace(getenv("CLOUD_SQL_INSTANCE_CONNECTION_NAME"))
		if !cloudsql.ValidInstanceConnectionName(cfg.cloudSQLInstance) {
			return workerConfig{}, errors.New("invalid Cloud SQL instance name")
		}
		privateIP := strings.TrimSpace(getenv("CLOUD_SQL_PRIVATE_IP"))
		parsed, err := strconv.ParseBool(privateIP)
		if err != nil && privateIP != "" {
			return workerConfig{}, errors.New("invalid Cloud SQL private IP setting")
		}
		cfg.cloudSQLPrivateIP = parsed
		cfg.dbHost = "localhost"
	} else {
		cfg.dbHost = strings.TrimSpace(getenv("DATABASE_HOST"))
		if cfg.dbHost == "" {
			return workerConfig{}, errors.New("database host is required")
		}
	}
	port := strings.TrimSpace(getenv("DATABASE_PORT"))
	if port == "" {
		cfg.dbPort = 5432
	} else {
		parsed, err := strconv.ParseUint(port, 10, 16)
		if err != nil || parsed == 0 {
			return workerConfig{}, errors.New("invalid database port")
		}
		cfg.dbPort = uint16(parsed)
	}
	cfg.dbName = strings.TrimSpace(getenv("DATABASE_NAME"))
	cfg.dbUser = strings.TrimSpace(getenv("DATABASE_USER"))
	cfg.databaseSetRole = strings.TrimSpace(getenv("DATABASE_SET_ROLE"))
	if cfg.databaseSetRole != "" && cfg.databaseSetRole != "espacigo_privacy_cleanup_worker" {
		return workerConfig{}, errors.New("invalid database role activation")
	}
	passwordPath := strings.TrimSpace(getenv("DATABASE_PASSWORD_FILE"))
	if cfg.dbName == "" || cfg.dbUser == "" || passwordPath == "" {
		return workerConfig{}, errors.New("database name, user and password file are required")
	}
	password, err := os.ReadFile(passwordPath)
	if err != nil {
		return workerConfig{}, errors.New("database password file is unavailable")
	}
	cfg.dbPassword = strings.TrimSpace(string(password))
	for i := range password {
		password[i] = 0
	}
	if cfg.dbPassword == "" {
		return workerConfig{}, errors.New("database password file is empty")
	}
	cfg.dbSSLMode = strings.TrimSpace(getenv("DATABASE_SSLMODE"))
	if cfg.dbSSLMode == "" {
		cfg.dbSSLMode = "disable"
	}
	switch cfg.dbSSLMode {
	case "disable", "allow", "prefer", "require", "verify-ca", "verify-full":
	default:
		return workerConfig{}, errors.New("invalid database SSL mode")
	}
	cfg.dbSSLRootCert = strings.TrimSpace(getenv("DATABASE_SSLROOTCERT"))
	cfg.dbSSLCert = strings.TrimSpace(getenv("DATABASE_SSLCERT"))
	cfg.dbSSLKey = strings.TrimSpace(getenv("DATABASE_SSLKEY"))
	if (cfg.dbSSLCert == "") != (cfg.dbSSLKey == "") {
		return workerConfig{}, errors.New("database client certificate and key must be configured together")
	}
	if cfg.connectionMode == "cloudsql" && (cfg.dbSSLMode != "disable" || cfg.dbSSLRootCert != "" || cfg.dbSSLCert != "" || cfg.dbSSLKey != "") {
		return workerConfig{}, errors.New("Cloud SQL connector cannot be combined with direct TLS settings")
	}
	cfg.storageBackend = strings.TrimSpace(getenv("BLOB_STORE_BACKEND"))
	if cfg.storageBackend == "" {
		cfg.storageBackend = "filesystem"
	}
	cfg.storageBucket = strings.TrimSpace(getenv("CLOUD_STORAGE_BUCKET"))
	cfg.storagePrefix = strings.TrimSpace(getenv("CLOUD_STORAGE_OBJECT_PREFIX"))
	cfg.evidenceRoot = strings.TrimSpace(getenv("M03_EVIDENCE_DIR"))
	if cfg.storageBackend == "gcs" && cfg.storageBucket == "" {
		return workerConfig{}, errors.New("Cloud Storage bucket is required")
	}
	if cfg.storageBackend != "gcs" && cfg.storageBackend != "filesystem" {
		return workerConfig{}, errors.New("invalid private evidence store backend")
	}
	if cfg.storageBackend == "filesystem" && cfg.evidenceRoot == "" {
		return workerConfig{}, errors.New("private evidence directory is required")
	}
	interval := strings.TrimSpace(getenv("PRIVACY_CLEANUP_INTERVAL"))
	if interval != "" {
		cfg.interval, err = time.ParseDuration(interval)
		if err != nil || cfg.interval < time.Second || cfg.interval > time.Hour {
			return workerConfig{}, errors.New("privacy cleanup interval must be between 1s and 1h")
		}
	}
	return cfg, nil
}

func (cfg workerConfig) databaseURL() string {
	endpoint := &url.URL{Scheme: "postgres", User: url.UserPassword(cfg.dbUser, cfg.dbPassword), Host: net.JoinHostPort(cfg.dbHost, strconv.Itoa(int(cfg.dbPort))), Path: "/" + cfg.dbName}
	query := url.Values{}
	query.Set("sslmode", cfg.dbSSLMode)
	if cfg.dbSSLRootCert != "" {
		query.Set("sslrootcert", cfg.dbSSLRootCert)
	}
	if cfg.dbSSLCert != "" {
		query.Set("sslcert", cfg.dbSSLCert)
		query.Set("sslkey", cfg.dbSSLKey)
	}
	endpoint.RawQuery = query.Encode()
	return endpoint.String()
}

func newCleaner(ctx context.Context, cfg workerConfig) (privacy.SyntheticEvidenceCleaner, error) {
	switch cfg.storageBackend {
	case "gcs":
		return cloudstorage.NewWithPrefix(ctx, cfg.storageBucket, cfg.storagePrefix)
	case "filesystem":
		root, err := filepath.Abs(cfg.evidenceRoot)
		if err != nil {
			return nil, err
		}
		return evidencefs.New(root)
	default:
		return nil, errors.New("invalid private evidence store backend")
	}
}

type timedCleaner struct {
	store privacy.SyntheticEvidenceCleaner
}

func (c timedCleaner) Delete(ctx context.Context, id string) error {
	deleteCtx, cancel := context.WithTimeout(ctx, maxDeleteDuration)
	defer cancel()
	return c.store.Delete(deleteCtx, id)
}
