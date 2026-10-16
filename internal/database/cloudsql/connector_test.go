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
	config, err := pgx.ParseConfig("postgres://migrator:secret@localhost:5432/app?sslmode=require")
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
	if config.TLSConfig != nil {
		t.Fatal("pgx TLS should be disabled because the connector provides TLS")
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
