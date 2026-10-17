package cloudsql

import (
	"context"
	"errors"
	"net"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"

	"cloud.google.com/go/cloudsqlconn"
	"github.com/jackc/pgx/v5"
)

type dialer interface {
	Dial(context.Context, string, ...cloudsqlconn.DialOption) (net.Conn, error)
	Close() error
}

type dialerFactory func(context.Context, ...cloudsqlconn.Option) (dialer, error)

var instanceConnectionNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{4,62}:[a-z][a-z0-9-]+:[a-z][a-z0-9-]+$`)

var pgxEnvironmentKeys = []string{
	"PGHOST", "PGPORT", "PGDATABASE", "PGUSER", "PGPASSWORD", "PGPASSFILE", "PGAPPNAME", "PGCONNECT_TIMEOUT",
	"PGSSLMODE", "PGSSLCERT", "PGSSLKEY", "PGSSLSNI", "PGSSLROOTCERT", "PGSSLPASSWORD", "PGSSLNEGOTIATION",
	"PGTARGETSESSIONATTRS", "PGSERVICE", "PGSERVICEFILE", "PGTZ", "PGOPTIONS", "PGMINPROTOCOLVERSION",
	"PGMAXPROTOCOLVERSION", "PGCHANNELBINDING", "PGREQUIREAUTH",
}

func ValidInstanceConnectionName(instance string) bool {
	return instanceConnectionNamePattern.MatchString(instance)
}

// ValidatePGEnvironment rejects libpq/pgx environment overrides in connector mode.
func ValidatePGEnvironment(getenv func(string) string) error {
	for _, key := range pgxEnvironmentKeys {
		if getenv(key) != "" {
			return errors.New("PostgreSQL connection environment overrides are not allowed in Cloud SQL mode")
		}
	}
	return nil
}

// ValidateConnectionURL admits only the connector mode's fixed URL shape:
// localhost:5432, explicit username/database and sslmode=disable, with no
// URL/query credentials or other pgx parameters.
func ValidateConnectionURL(databaseURL string) error {
	parsed, err := url.Parse(databaseURL)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") || parsed.Opaque != "" || parsed.Fragment != "" || parsed.ForceQuery {
		return errors.New("Cloud SQL DATABASE_URL has an unsupported format")
	}
	if parsed.Hostname() != "localhost" || (parsed.Port() != "" && parsed.Port() != "5432") {
		return errors.New("Cloud SQL DATABASE_URL must target localhost:5432")
	}
	if parsed.User == nil || parsed.User.Username() == "" {
		return errors.New("Cloud SQL DATABASE_URL must include a username")
	}
	if _, hasPassword := parsed.User.Password(); hasPassword {
		return errors.New("Cloud SQL DATABASE_URL must not contain an inline password")
	}
	databaseName := strings.TrimPrefix(parsed.Path, "/")
	if databaseName == "" || strings.Contains(databaseName, "/") {
		return errors.New("Cloud SQL DATABASE_URL must include one database name")
	}
	query, err := url.ParseQuery(parsed.RawQuery)
	if err != nil {
		return errors.New("Cloud SQL DATABASE_URL query is invalid")
	}
	if len(query) != 1 || len(query["sslmode"]) != 1 || query["sslmode"][0] != "disable" {
		return errors.New("Cloud SQL DATABASE_URL only allows sslmode=disable")
	}
	return nil
}

// NewPGXConfig applies credentials from the secret file after parsing a
// restricted URL. A non-secret placeholder prevents pgx from consulting
// passfiles before the final password is assigned.
func NewPGXConfig(databaseURL, password string) (*pgx.ConnConfig, error) {
	if err := ValidateConnectionURL(databaseURL); err != nil {
		return nil, err
	}
	if password == "" {
		return nil, errors.New("database password is required")
	}
	if err := ValidatePGEnvironment(os.Getenv); err != nil {
		return nil, err
	}
	parsed, _ := url.Parse(databaseURL)
	user := parsed.User.Username()
	databaseName := strings.TrimPrefix(parsed.Path, "/")
	parsed.User = url.UserPassword(user, "cloudsql-password-from-secret-file")
	config, err := pgx.ParseConfigWithOptions(parsed.String(), pgx.ParseConfigOptions{
		ConnStringAllowedKeys: []string{"user", "password", "host", "port", "database", "sslmode"},
	})
	if err != nil {
		return nil, errors.New("invalid Cloud SQL connection configuration")
	}
	if config.TLSConfig != nil || len(config.Fallbacks) != 0 {
		return nil, errors.New("Cloud SQL connector requires pgx TLS and fallbacks to be disabled")
	}
	config.User = user
	config.Password = password
	config.Database = databaseName
	config.Host = "localhost"
	config.Port = 5432
	config.RuntimeParams = make(map[string]string)
	config.ValidateConnect = nil
	return config, nil
}

// ConfigurePGX installs the Cloud SQL connector as pgx's network dialer.
// The connector provides the authenticated TLS tunnel. Configs with pgx TLS
// or TLS fallbacks are rejected instead of being rewritten.
func ConfigurePGX(ctx context.Context, config *pgx.ConnConfig, instance string, privateIP bool) (func(), error) {
	return configurePGX(ctx, config, instance, privateIP, func(ctx context.Context, options ...cloudsqlconn.Option) (dialer, error) {
		return cloudsqlconn.NewDialer(ctx, options...)
	})
}

// Connect creates a pgx connection through the shared Cloud SQL connector.
// The caller must invoke closeConnector when the connection is no longer used.
func Connect(ctx context.Context, databaseURL, password, instance string, privateIP bool) (*pgx.Conn, func(), error) {
	config, err := NewPGXConfig(databaseURL, password)
	if err != nil {
		return nil, nil, err
	}
	closeConnector, err := ConfigurePGX(ctx, config, instance, privateIP)
	if err != nil {
		return nil, nil, err
	}
	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		closeConnector()
		return nil, nil, errors.New("could not connect to Cloud SQL")
	}
	return conn, closeConnector, nil
}

// ConnectIAM opens a connector-enforced Cloud SQL connection using automatic
// IAM database authentication. The username is the fixed IAM database user;
// no password or Secret Manager value is accepted by this path.
func ConnectIAM(ctx context.Context, databaseURL, instance string, privateIP bool) (*pgx.Conn, func(), error) {
	if err := ValidateConnectionURL(databaseURL); err != nil {
		return nil, nil, err
	}
	if err := ValidatePGEnvironment(os.Getenv); err != nil {
		return nil, nil, err
	}
	parsed, _ := url.Parse(databaseURL)
	parsed.User = url.UserPassword(parsed.User.Username(), "iam-authentication")
	config, err := pgx.ParseConfigWithOptions(parsed.String(), pgx.ParseConfigOptions{
		ConnStringAllowedKeys: []string{"user", "password", "host", "port", "database", "sslmode"},
	})
	if err != nil || config.TLSConfig != nil || len(config.Fallbacks) != 0 {
		return nil, nil, errors.New("invalid Cloud SQL IAM connection configuration")
	}
	config.Password = ""
	config.Database = strings.TrimPrefix(parsed.Path, "/")
	config.Host = "localhost"
	config.Port = 5432
	config.RuntimeParams = make(map[string]string)
	closeConnector, err := configurePGXWithIAM(ctx, config, instance, privateIP, true, func(ctx context.Context, options ...cloudsqlconn.Option) (dialer, error) {
		return cloudsqlconn.NewDialer(ctx, options...)
	})
	if err != nil {
		return nil, nil, err
	}
	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		closeConnector()
		return nil, nil, errors.New("could not connect to Cloud SQL with IAM database authentication")
	}
	return conn, closeConnector, nil
}

func configurePGX(
	ctx context.Context,
	config *pgx.ConnConfig,
	instance string,
	privateIP bool,
	newDialer dialerFactory,
) (func(), error) {
	return configurePGXWithIAM(ctx, config, instance, privateIP, false, newDialer)
}

func configurePGXWithIAM(
	ctx context.Context,
	config *pgx.ConnConfig,
	instance string,
	privateIP bool,
	iamAuth bool,
	newDialer dialerFactory,
) (func(), error) {
	if config == nil {
		return nil, errors.New("Cloud SQL connector configuration is required")
	}
	if !ValidInstanceConnectionName(instance) {
		return nil, errors.New("Cloud SQL instance connection name is invalid")
	}
	if config.TLSConfig != nil || len(config.Fallbacks) != 0 {
		return nil, errors.New("Cloud SQL connector requires pgx TLS and fallbacks to be disabled")
	}
	options := make([]cloudsqlconn.Option, 0, 2)
	if privateIP {
		options = append(options, cloudsqlconn.WithDefaultDialOptions(cloudsqlconn.WithPrivateIP()))
	}
	if iamAuth {
		options = append(options, cloudsqlconn.WithIAMAuthN())
	}
	connector, err := newDialer(ctx, options...)
	if err != nil {
		return nil, errors.New("Cloud SQL connector initialization failed")
	}
	config.DialFunc = func(dialCtx context.Context, _, _ string) (net.Conn, error) {
		return connector.Dial(dialCtx, instance)
	}
	var once sync.Once
	closeConnector := func() {
		once.Do(func() { _ = connector.Close() })
	}
	return closeConnector, nil
}
