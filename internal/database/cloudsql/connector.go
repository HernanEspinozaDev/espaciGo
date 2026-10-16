package cloudsql

import (
	"context"
	"errors"
	"net"
	"regexp"
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

func ValidInstanceConnectionName(instance string) bool {
	return instanceConnectionNamePattern.MatchString(instance)
}

// ConfigurePGX installs the Cloud SQL connector as pgx's network dialer.
// The connector provides the authenticated TLS tunnel; pgx TLS is disabled so
// the client does not try to negotiate a second TLS layer over that tunnel.
func ConfigurePGX(ctx context.Context, config *pgx.ConnConfig, instance string, privateIP bool) (func(), error) {
	return configurePGX(ctx, config, instance, privateIP, func(ctx context.Context, options ...cloudsqlconn.Option) (dialer, error) {
		return cloudsqlconn.NewDialer(ctx, options...)
	})
}

func configurePGX(
	ctx context.Context,
	config *pgx.ConnConfig,
	instance string,
	privateIP bool,
	newDialer dialerFactory,
) (func(), error) {
	if config == nil {
		return nil, errors.New("Cloud SQL connector configuration is required")
	}
	if !ValidInstanceConnectionName(instance) {
		return nil, errors.New("Cloud SQL instance connection name is invalid")
	}
	options := make([]cloudsqlconn.Option, 0, 1)
	if privateIP {
		options = append(options, cloudsqlconn.WithDefaultDialOptions(cloudsqlconn.WithPrivateIP()))
	}
	connector, err := newDialer(ctx, options...)
	if err != nil {
		return nil, errors.New("Cloud SQL connector initialization failed")
	}
	config.TLSConfig = nil
	config.DialFunc = func(dialCtx context.Context, _, _ string) (net.Conn, error) {
		return connector.Dial(dialCtx, instance)
	}
	var once sync.Once
	closeConnector := func() {
		once.Do(func() { _ = connector.Close() })
	}
	return closeConnector, nil
}
