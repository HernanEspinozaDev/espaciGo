package adminglobal

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/jackc/pgx/v5"
)

var testDatabaseName = regexp.MustCompile(`^espacigo_gadm01_[a-f0-9]{8,12}$`)

type TransactionBeginner interface {
	BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error)
}

// ValidateBootstrapTarget intentionally accepts only a named loopback test DB.
func ValidateBootstrapTarget(environment, databaseURL, expectedDatabase string) error {
	if environment != "local-test" || !testDatabaseName.MatchString(expectedDatabase) {
		return errors.New("bootstrap is restricted to an explicitly named local test database")
	}
	parsed, err := url.Parse(databaseURL)
	if err != nil || parsed.Scheme != "postgres" && parsed.Scheme != "postgresql" {
		return errors.New("invalid local database URL")
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "localhost" && host != "127.0.0.1" && host != "::1" {
		return errors.New("bootstrap requires a loopback database")
	}
	if parsed.Path != "/"+expectedDatabase {
		return errors.New("database name does not match the explicit test target")
	}
	return nil
}

// ValidateLocalActivationTarget allows the owner bootstrap only against an explicitly
// named ordinary local database. URL query parameters are rejected to prevent libpq
// host/service redirection. Docker callers must join the database container's network
// namespace so both effective PostgreSQL endpoints remain loopback.
func ValidateLocalActivationTarget(environment, databaseURL, expectedDatabase string) error {
	if environment != "local-activation" || expectedDatabase != "espacigo_local" {
		return errors.New("activation is restricted to the explicit espacigo_local database")
	}
	parsed, err := url.Parse(databaseURL)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("invalid local activation database URL")
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "localhost" && host != "127.0.0.1" && host != "::1" {
		return errors.New("activation requires an explicit loopback database URL")
	}
	if parsed.Path != "/"+expectedDatabase {
		return errors.New("database name does not match the explicit activation target")
	}
	return nil
}

// IsLocalDatabaseAddress accepts only loopback database endpoints.
func IsLocalDatabaseAddress(address net.IP) bool {
	return address != nil && address.IsLoopback()
}

// Bootstrap binds one verified, active account as the initial global administrator.
// Recovery only restores the account recorded by the first successful bootstrap.
func Bootstrap(ctx context.Context, pool TransactionBeginner, target, action, correlation string) error {
	action, validAction := canonicalBootstrapAction(action)
	if pool == nil || !canonicalUUID.MatchString(target) || !canonicalUUID.MatchString(correlation) || !validAction {
		return ErrInvalid
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	// This lock is shared with every operation that can change global membership.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(1195469901,1397056561)`); err != nil {
		return err
	}
	result, reason := "rechazo", "destino_no_elegible"
	var bound string
	boundErr := tx.QueryRow(ctx, `SELECT usuario_id::text FROM public.administrador_global_bootstrap WHERE singleton=true FOR UPDATE`).Scan(&bound)
	if action == "bootstrap" && boundErr == pgx.ErrNoRows {
		bound = ""
	} else if boundErr != nil && boundErr != pgx.ErrNoRows {
		return boundErr
	}
	var eligible bool
	err = tx.QueryRow(ctx, `SELECT u.estado='activo' AND (u.bloqueado_hasta IS NULL OR u.bloqueado_hasta<=now())
		AND NOT EXISTS(SELECT 1 FROM public.bloqueo_cuenta_administrativo_local b WHERE b.cuenta_id=u.id)
		FROM public.usuario u WHERE u.id=$1 FOR UPDATE`, target).Scan(&eligible)
	if err == nil && eligible {
		switch {
		case action == "bootstrap" && bound == "":
			_, err = tx.Exec(ctx, `INSERT INTO public.administrador_global_bootstrap(singleton,usuario_id,creado_en) VALUES(true,$1,now())`, target)
			if err == nil {
				_, err = tx.Exec(ctx, `INSERT INTO public.administrador_global(usuario_id,concedido_en,motivo_codigo) VALUES($1,now(),'bootstrap_propietario') ON CONFLICT(usuario_id) DO NOTHING`, target)
			}
		case bound == target:
			_, err = tx.Exec(ctx, `INSERT INTO public.administrador_global(usuario_id,concedido_en,motivo_codigo) VALUES($1,now(),'recuperacion_local') ON CONFLICT(usuario_id) DO NOTHING`, target)
		default:
			err = ErrForbidden
		}
		if err == nil {
			result, reason = "exito", map[bool]string{true: "recuperacion_local", false: "bootstrap_inicial"}[action == "recuperacion"]
		}
	} else if err != nil && err != pgx.ErrNoRows {
		return err
	}
	if err != nil && !errors.Is(err, ErrForbidden) {
		return err
	}
	if action == "recuperacion" && bound == "" {
		reason = "sin_bootstrap_registrado"
	}
	if action == "bootstrap" && bound != "" && bound != target {
		reason = "destino_distinto_al_bootstrap"
	}
	id, err := (credentials.Generator{}).ID()
	if err != nil {
		return err
	}
	_, auditErr := tx.Exec(ctx, `INSERT INTO public.administrador_global_bootstrap_evento(id,usuario_objetivo_id,actor_tipo,accion,resultado,motivo_codigo,correlacion_id,ocurrida_en)
		VALUES($1,$2,'cli_local',$3,$4,$5,$6,$7)`, id, target, action, result, reason, correlation, time.Now().UTC())
	if auditErr != nil {
		return auditErr
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	if result != "exito" {
		return fmt.Errorf("bootstrap rejected: %s", reason)
	}
	return nil
}

func canonicalBootstrapAction(action string) (string, bool) {
	switch action {
	case "bootstrap":
		return "bootstrap", true
	case "recover", "recuperacion":
		return "recuperacion", true
	default:
		return "", false
	}
}
