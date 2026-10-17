package cloudsql

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"

	"cloud.google.com/go/cloudsqlconn"
	"github.com/jackc/pgx/v5"
)

type fakeDialer struct {
	instance string
	closed   int
}

func (d *fakeDialer) Dial(_ context.Context, instance string, _ ...cloudsqlconn.DialOption) (net.Conn, error) {
	d.instance = instance
	client, server := net.Pipe()
	_ = server.Close()
	return client, nil
}

func (d *fakeDialer) Close() error {
	d.closed++
	return nil
}

func TestConfigurePGXUsesPrivateConnectorAndClosesOnce(t *testing.T) {
	config, err := pgx.ParseConfig("postgres://migrator@localhost:5432/app?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakeDialer{}
	optionCount := -1
	closeConnector, err := configurePGX(context.Background(), config, "project-one:southamerica-west1:db-one", true,
		func(_ context.Context, options ...cloudsqlconn.Option) (dialer, error) {
			optionCount = len(options)
			return fake, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if optionCount != 1 {
		t.Fatalf("private IP option count = %d, want 1", optionCount)
	}
	if config.TLSConfig != nil || len(config.Fallbacks) != 0 {
		t.Fatal("pgx TLS and fallback configurations should remain disabled")
	}
	conn, err := config.DialFunc(context.Background(), "tcp", "ignored-host:5432")
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
	if fake.instance != "project-one:southamerica-west1:db-one" {
		t.Fatalf("dialed instance = %q", fake.instance)
	}
	closeConnector()
	closeConnector()
	if fake.closed != 1 {
		t.Fatalf("connector closed %d times, want once", fake.closed)
	}
}

func TestConfigurePGXWithIAMAddsAutomaticIAMAndPrivateIPOptions(t *testing.T) {
	config, err := pgx.ParseConfig("postgres://observer@localhost:5432/app?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	optionCount := -1
	_, err = configurePGXWithIAM(context.Background(), config, "project-one:southamerica-west1:db-one", true, true,
		func(_ context.Context, options ...cloudsqlconn.Option) (dialer, error) {
			optionCount = len(options)
			return &fakeDialer{}, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if optionCount != 2 || config.Password != "" {
		t.Fatalf("IAM connector options=%d password=%q, want two options and no password", optionCount, config.Password)
	}
}

func TestConfigurePGXRejectsInvalidConfigurationBeforeCreatingDialer(t *testing.T) {
	factoryCalled := false
	factory := func(context.Context, ...cloudsqlconn.Option) (dialer, error) {
		factoryCalled = true
		return &fakeDialer{}, nil
	}
	if _, err := configurePGX(context.Background(), nil, "project-one:region-one:db-one", false, factory); err == nil {
		t.Fatal("nil pgx config should fail")
	}
	config, err := pgx.ParseConfig("postgres://user:secret@localhost/db")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := configurePGX(context.Background(), config, "invalid", false, factory); err == nil {
		t.Fatal("invalid instance name should fail")
	}
	if factoryCalled {
		t.Fatal("dialer factory called for invalid configuration")
	}
}

func TestConfigurePGXSanitizesDialerInitializationError(t *testing.T) {
	config, err := pgx.ParseConfig("postgres://private-user:private-password@localhost/db")
	if err != nil {
		t.Fatal(err)
	}
	_, err = configurePGX(context.Background(), config, "project-one:region-one:db-one", false,
		func(context.Context, ...cloudsqlconn.Option) (dialer, error) {
			return nil, errors.New("ADC failed with private-password")
		})
	if err == nil || strings.Contains(err.Error(), "private-password") {
		t.Fatalf("connector error was not sanitized: %v", err)
	}
}

func TestConfigurePGXRejectsTLSFallbackBeforeCreatingDialer(t *testing.T) {
	config, err := pgx.ParseConfig("postgres://user@localhost/db?sslmode=prefer")
	if err != nil {
		t.Fatal(err)
	}
	factoryCalled := false
	_, err = configurePGX(context.Background(), config, "project-one:region-one:db-one", true,
		func(context.Context, ...cloudsqlconn.Option) (dialer, error) {
			factoryCalled = true
			return &fakeDialer{}, nil
		})
	if err == nil || !strings.Contains(err.Error(), "TLS and fallbacks") {
		t.Fatalf("expected TLS fallback rejection, got %v", err)
	}
	if factoryCalled {
		t.Fatal("connector was created for a pgx config with TLS fallbacks")
	}
}

func TestNewPGXConfigUsesFilePasswordAsEffectiveCredential(t *testing.T) {
	for _, key := range pgxEnvironmentKeys {
		t.Setenv(key, "")
	}
	config, err := NewPGXConfig("postgres://migrator@localhost:5432/scratch?sslmode=disable", "file-password")
	if err != nil {
		t.Fatal(err)
	}
	if config.User != "migrator" || config.Password != "file-password" || config.Database != "scratch" {
		t.Fatal("pgx config did not use the URL identity and file password")
	}
	if config.Host != "localhost" || config.Port != 5432 || config.TLSConfig != nil || len(config.Fallbacks) != 0 || len(config.RuntimeParams) != 0 {
		t.Fatal("pgx config contains a host, TLS fallback, or runtime override outside the Cloud SQL policy")
	}
}

func TestValidateConnectionURLRejectsCredentialAndServiceOverrides(t *testing.T) {
	for _, databaseURL := range []string{
		"postgres://migrator:inline-secret@localhost/app?sslmode=disable",
		"postgres://migrator@localhost/app?sslmode=disable&password=inline-secret",
		"postgres://migrator@localhost/app?sslmode=disable&%70assword=inline-secret",
		"postgres://migrator@localhost/app?sslmode=disable&password=first&password=second",
		"postgres://migrator@localhost/app?sslmode=disable&service=untrusted",
		"postgres://migrator@localhost/app?sslmode=disable&servicefile=%2Ftmp%2Foverride",
		"postgres://migrator@localhost/app?sslmode=disable&sslmode=prefer",
	} {
		if err := ValidateConnectionURL(databaseURL); err == nil || strings.Contains(err.Error(), "inline-secret") || strings.Contains(err.Error(), "/tmp/override") {
			t.Fatalf("expected sanitized rejection for Cloud SQL URL override: %v", err)
		}
	}
}

func TestValidatePGEnvironmentRejectsIdentityOverrides(t *testing.T) {
	err := ValidatePGEnvironment(func(key string) string {
		if key == "PGSERVICE" {
			return "untrusted-service"
		}
		return ""
	})
	if err == nil || strings.Contains(err.Error(), "untrusted-service") {
		t.Fatalf("expected sanitized pgx environment rejection, got %v", err)
	}
}

func TestValidInstanceConnectionName(t *testing.T) {
	for _, test := range []struct {
		name string
		want bool
	}{
		{name: "project-one:southamerica-west1:instance-one", want: true},
		{name: "bad project:southamerica-west1:instance-one", want: false},
		{name: "project-one:region_one:instance", want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := ValidInstanceConnectionName(test.name); got != test.want {
				t.Fatalf("ValidInstanceConnectionName(%q) = %v, want %v", test.name, got, test.want)
			}
		})
	}
}
