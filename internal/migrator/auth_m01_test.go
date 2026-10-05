package migrator

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestM01MigrationPersistsAuthorizedSchemaAndConstraints(t *testing.T) {
	withTestDatabase(t, func(ctx context.Context, databaseURL string) {
		migrationDir := t.TempDir()
		m01, err := os.ReadFile(filepath.Join("..", "..", "db", "migrations", "V000001__m01_identity.sql"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(migrationDir, "V000001__m01_identity.sql"), m01, 0o600); err != nil {
			t.Fatal(err)
		}
		migrations, err := DiscoverMigrations(migrationDir)
		if err != nil {
			t.Fatal(err)
		}
		if len(migrations) != 1 || migrations[0].Name != "V000001__m01_identity.sql" {
			t.Fatalf("unexpected M01 migration set: %#v", migrations)
		}

		first, err := Run(ctx, databaseURL, migrationDir)
		if err != nil {
			t.Fatal(err)
		}
		if len(first.Applied) != 1 || first.Applied[0] != 1 {
			t.Fatalf("first run applied versions %v, want [1]", first.Applied)
		}
		second, err := Run(ctx, databaseURL, migrationDir)
		if err != nil {
			t.Fatal(err)
		}
		if len(second.Applied) != 0 {
			t.Fatalf("repeat run applied versions %v, want no changes", second.Applied)
		}

		conn := connectTestDB(t, ctx, databaseURL)
		defer conn.Close(context.Background())
		var historyRows int
		if err := conn.QueryRow(ctx, "SELECT count(*) FROM public.schema_migrations").Scan(&historyRows); err != nil {
			t.Fatal(err)
		}
		if historyRows != 1 {
			t.Fatalf("history contains %d rows after repeat, want exactly one", historyRows)
		}
		var historyName, migrationChecksum, runnerChecksum string
		if err := conn.QueryRow(ctx, `SELECT name, checksum, runner_checksum FROM public.schema_migrations WHERE version = 1`).Scan(&historyName, &migrationChecksum, &runnerChecksum); err != nil {
			t.Fatal(err)
		}
		if historyName != migrations[0].Name || migrationChecksum != migrations[0].Checksum {
			t.Fatalf("history name/checksum changed: name=%q checksum=%q", historyName, migrationChecksum)
		}
		if len(runnerChecksum) != 64 {
			t.Fatalf("runner checksum length=%d, want 64", len(runnerChecksum))
		}
		assertM01Schema(t, ctx, conn)
		assertM01Constraints(t, ctx, conn, databaseURL)
	})
}

func assertM01Schema(t *testing.T, ctx context.Context, conn *pgx.Conn) {
	t.Helper()
	wantTables := []string{"usuario", "rol_usuario", "sesion", "token_accion", "version_terminos", "aceptacion_terminos"}
	var tableCount int
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM pg_catalog.pg_tables
		WHERE schemaname = 'public' AND tablename = ANY($1)`, wantTables).Scan(&tableCount); err != nil {
		t.Fatal(err)
	}
	if tableCount != len(wantTables) {
		t.Fatalf("found %d of %d M01 tables", tableCount, len(wantTables))
	}

	wantTypes := map[string]map[string]string{
		"usuario": {
			"id": "uuid", "correo_original": "text", "correo_normalizado": "text", "hash_clave": "text",
			"estado": "text", "creado_en": "timestamp with time zone", "actualizado_en": "timestamp with time zone",
			"baja_solicitada_en": "timestamp with time zone", "intentos_fallidos_consecutivos": "integer", "bloqueado_hasta": "timestamp with time zone",
		},
		"rol_usuario": {
			"usuario_id": "uuid", "rol": "text", "concedido_en": "timestamp with time zone", "concedido_por": "uuid",
		},
		"sesion": {
			"id": "uuid", "usuario_id": "uuid", "token_hash": "character", "creada_en": "timestamp with time zone",
			"ultima_actividad_en": "timestamp with time zone", "expira_en": "timestamp with time zone", "revocada_en": "timestamp with time zone", "cliente_resumen": "text",
		},
		"token_accion": {
			"id": "uuid", "usuario_id": "uuid", "proposito": "text", "token_hash": "character",
			"creado_en": "timestamp with time zone", "expira_en": "timestamp with time zone", "consumido_en": "timestamp with time zone",
			"invalidado_en": "timestamp with time zone", "intentos": "integer",
		},
		"version_terminos": {
			"id": "uuid", "codigo": "text", "tipo": "text", "hash_sha256": "character", "publicada_en": "timestamp with time zone",
		},
		"aceptacion_terminos": {
			"id": "uuid", "usuario_id": "uuid", "version_id": "uuid", "aceptada_en": "timestamp with time zone", "canal": "text",
		},
	}
	for table, columns := range wantTypes {
		for column, wantType := range columns {
			var gotType string
			if err := conn.QueryRow(ctx, `SELECT data_type FROM information_schema.columns
				WHERE table_schema = 'public' AND table_name = $1 AND column_name = $2`, table, column).Scan(&gotType); err != nil {
				t.Errorf("missing required column %s.%s: %v", table, column, err)
				continue
			}
			if gotType != wantType {
				t.Errorf("column %s.%s has type %q, want %q", table, column, gotType, wantType)
			}
		}
	}

	var termsCount int
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM public.version_terminos
		WHERE codigo IN ('sintetico-terminos-v0.1', 'sintetico-privacidad-v0.1', 'sintetico-politica-arrendador-v0.1')`).Scan(&termsCount); err != nil {
		t.Fatal(err)
	}
	if termsCount != 3 {
		t.Fatalf("found %d synthetic terms fixtures, want 3", termsCount)
	}

	var outOfScopeTables int
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM pg_catalog.pg_tables
		WHERE schemaname = 'public' AND tablename IN ('perfil_usuario', 'notificacion', 'historial_clave', 'preferencia_uso')`).Scan(&outOfScopeTables); err != nil {
		t.Fatal(err)
	}
	if outOfScopeTables != 0 {
		t.Fatalf("migration created %d tables outside the six-table M01 scope", outOfScopeTables)
	}
}

func assertM01Constraints(t *testing.T, ctx context.Context, conn *pgx.Conn, databaseURL string) {
	t.Helper()

	indexes := map[string]string{}
	rows, err := conn.Query(ctx, `SELECT indexname, indexdef FROM pg_catalog.pg_indexes
		WHERE schemaname = 'public' AND tablename = ANY($1)`, []string{"usuario", "rol_usuario", "sesion", "token_accion", "version_terminos", "aceptacion_terminos"})
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var name, definition string
		if err := rows.Scan(&name, &definition); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		indexes[name] = definition
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		t.Fatal(err)
	}
	rows.Close()
	uniqueIndexes := []string{
		"usuario_correo_normalizado_uk", "rol_usuario_pk", "sesion_pk", "sesion_token_hash_uk",
		"token_accion_pk", "token_accion_token_hash_uk", "version_terminos_pk", "version_terminos_codigo_uk",
		"aceptacion_terminos_pk", "aceptacion_terminos_usuario_version_uk",
	}
	for _, name := range uniqueIndexes {
		definition, ok := indexes[name]
		if !ok {
			t.Errorf("missing required unique index %s", name)
		} else if !strings.Contains(definition, "UNIQUE INDEX") {
			t.Errorf("index %s is not unique: %s", name, definition)
		}
	}
	for _, name := range []string{
		"rol_usuario_concedido_por_idx", "sesion_usuario_activa_idx", "token_accion_usuario_pendiente_idx",
		"token_accion_usuario_emision_idx", "aceptacion_terminos_version_idx",
	} {
		if _, ok := indexes[name]; !ok {
			t.Errorf("missing required index %s", name)
		}
	}
	if definition := indexes["sesion_usuario_activa_idx"]; !strings.Contains(definition, "revocada_en IS NULL") {
		t.Errorf("session lookup index is not scoped to unrevoked sessions: %s", definition)
	}
	if definition := indexes["token_accion_usuario_pendiente_idx"]; !strings.Contains(definition, "consumido_en IS NULL") || !strings.Contains(definition, "invalidado_en IS NULL") {
		t.Errorf("pending token index has unexpected predicate: %s", definition)
	}

	var foreignKeyCount int
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM pg_catalog.pg_constraint
		WHERE connamespace = 'public'::regnamespace AND contype = 'f'
		AND conrelid IN (SELECT oid FROM pg_catalog.pg_class WHERE relname = ANY($1))`,
		[]string{"rol_usuario", "sesion", "token_accion", "aceptacion_terminos"}).Scan(&foreignKeyCount); err != nil {
		t.Fatal(err)
	}
	if foreignKeyCount != 6 {
		t.Fatalf("found %d M01 foreign keys, want 6", foreignKeyCount)
	}
	var invalidDeleteActions int
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM pg_catalog.pg_constraint
		WHERE connamespace = 'public'::regnamespace AND contype = 'f' AND confdeltype NOT IN ('a', 'r')
		AND conrelid IN (SELECT oid FROM pg_catalog.pg_class WHERE relname = ANY($1))`,
		[]string{"rol_usuario", "sesion", "token_accion", "aceptacion_terminos"}).Scan(&invalidDeleteActions); err != nil {
		t.Fatal(err)
	}
	if invalidDeleteActions != 0 {
		t.Fatalf("found %d M01 foreign keys with a non-RESTRICT/NO ACTION delete behavior", invalidDeleteActions)
	}

	const userID = "00000000-0000-4000-8000-000000000010"
	if _, err := conn.Exec(ctx, `INSERT INTO usuario (id, correo_original, correo_normalizado, hash_clave)
		VALUES ($1, 'persona-a@ejemplo.invalid', 'persona-a@ejemplo.invalid', 'hash-sintetico-no-utilizable')`, userID); err != nil {
		t.Fatal(err)
	}
	var originalEmail, normalizedEmail, passwordHash string
	if err := conn.QueryRow(ctx, `SELECT correo_original, correo_normalizado, hash_clave FROM usuario WHERE id = $1`, userID).Scan(&originalEmail, &normalizedEmail, &passwordHash); err != nil {
		t.Fatal(err)
	}
	if originalEmail != "persona-a@ejemplo.invalid" || normalizedEmail != "persona-a@ejemplo.invalid" || passwordHash != "hash-sintetico-no-utilizable" {
		t.Fatalf("account fields did not persist: original=%q normalized=%q hash=%q", originalEmail, normalizedEmail, passwordHash)
	}
	blockedAt := time.Date(2026, time.January, 1, 0, 30, 0, 0, time.UTC)
	if _, err := conn.Exec(ctx, `UPDATE usuario SET estado = 'bloqueado', intentos_fallidos_consecutivos = 4, bloqueado_hasta = $2 WHERE id = $1`, userID, blockedAt); err != nil {
		t.Fatal(err)
	}
	var accountState string
	var failedAttempts int
	var storedBlockedAt time.Time
	if err := conn.QueryRow(ctx, `SELECT estado, intentos_fallidos_consecutivos, bloqueado_hasta FROM usuario WHERE id = $1`, userID).Scan(&accountState, &failedAttempts, &storedBlockedAt); err != nil {
		t.Fatal(err)
	}
	if accountState != "bloqueado" || failedAttempts != 4 || !storedBlockedAt.Equal(blockedAt) {
		t.Fatalf("login lock fields did not persist: state=%q attempts=%d blocked_at=%s", accountState, failedAttempts, storedBlockedAt)
	}
	assertUniqueCorreoConcurrent(t, ctx, databaseURL)

	if _, err := conn.Exec(ctx, `INSERT INTO usuario (id, correo_original, correo_normalizado, hash_clave, estado)
		VALUES ('00000000-0000-4000-8000-000000000020', 'invalido@ejemplo.invalid', 'invalido@ejemplo.invalid', 'hash-sintetico-no-utilizable', 'estado_no_permitido')`); !hasSQLState(err, "23514") {
		t.Errorf("invalid account state error=%v, want check violation 23514", err)
	}
	if _, err := conn.Exec(ctx, `INSERT INTO usuario (id, correo_original, correo_normalizado, hash_clave, intentos_fallidos_consecutivos)
		VALUES ('00000000-0000-4000-8000-000000000021', 'contador@ejemplo.invalid', 'contador@ejemplo.invalid', 'hash-sintetico-no-utilizable', -1)`); !hasSQLState(err, "23514") {
		t.Errorf("negative login attempts error=%v, want check violation 23514", err)
	}

	if _, err := conn.Exec(ctx, `INSERT INTO token_accion (id, usuario_id, proposito, token_hash, creado_en, expira_en)
		VALUES ('00000000-0000-4000-8000-000000000030', $1, 'cambiar_correo', repeat('a', 64), now(), now() + interval '15 minutes')`, userID); !hasSQLState(err, "23514") {
		t.Errorf("unsupported token purpose error=%v, want check violation 23514", err)
	}
	if _, err := conn.Exec(ctx, `INSERT INTO token_accion (id, usuario_id, proposito, token_hash, creado_en, expira_en, intentos)
		VALUES ('00000000-0000-4000-8000-000000000031', $1, 'recuperar_clave', repeat('b', 64), now(), now() + interval '15 minutes', 6)`, userID); !hasSQLState(err, "23514") {
		t.Errorf("sixth token attempt error=%v, want check violation 23514", err)
	}
	if _, err := conn.Exec(ctx, `INSERT INTO token_accion (id, usuario_id, proposito, token_hash, creado_en, expira_en, consumido_en, invalidado_en)
		VALUES ('00000000-0000-4000-8000-000000000032', $1, 'recuperar_clave', repeat('c', 64), now(), now() + interval '15 minutes', now(), now())`, userID); !hasSQLState(err, "23514") {
		t.Errorf("consumed and invalidated token error=%v, want check violation 23514", err)
	}

	if _, err := conn.Exec(ctx, `INSERT INTO sesion (id, usuario_id, token_hash, creada_en, ultima_actividad_en, expira_en)
		VALUES ('00000000-0000-4000-8000-000000000040', $1, repeat('d', 64), '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z', '2026-01-01T08:00:01Z')`, userID); !hasSQLState(err, "23514") {
		t.Errorf("session over absolute lifetime error=%v, want check violation 23514", err)
	}

	tokenCreated := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	tokenExpiry := tokenCreated.Add(15 * time.Minute)
	tokenConsumed := tokenCreated.Add(5 * time.Minute)
	if _, err := conn.Exec(ctx, `INSERT INTO token_accion (id, usuario_id, proposito, token_hash, creado_en, expira_en, consumido_en, intentos)
		VALUES ('00000000-0000-4000-8000-000000000033', $1, 'recuperar_clave', repeat('e', 64), $2, $3, $4, 2)`, userID, tokenCreated, tokenExpiry, tokenConsumed); err != nil {
		t.Fatal(err)
	}
	var storedPurpose, storedTokenHash string
	var storedTokenCreated, storedTokenExpiry, storedTokenConsumed, storedTokenInvalidated pgtype.Timestamptz
	var storedTokenAttempts int
	if err := conn.QueryRow(ctx, `SELECT proposito, token_hash, creado_en, expira_en, consumido_en, invalidado_en, intentos
		FROM token_accion WHERE id = '00000000-0000-4000-8000-000000000033'`).Scan(&storedPurpose, &storedTokenHash, &storedTokenCreated, &storedTokenExpiry, &storedTokenConsumed, &storedTokenInvalidated, &storedTokenAttempts); err != nil {
		t.Fatal(err)
	}
	if storedPurpose != "recuperar_clave" || storedTokenHash != strings.Repeat("e", 64) || !storedTokenCreated.Time.Equal(tokenCreated) || !storedTokenExpiry.Time.Equal(tokenExpiry) || !storedTokenConsumed.Time.Equal(tokenConsumed) || storedTokenInvalidated.Valid || storedTokenAttempts != 2 {
		t.Fatalf("consumed token fields did not persist: purpose=%q attempts=%d consumed_valid=%t invalidated_valid=%t", storedPurpose, storedTokenAttempts, storedTokenConsumed.Valid, storedTokenInvalidated.Valid)
	}
	invalidatedCreated := time.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC)
	invalidatedExpiry := invalidatedCreated.Add(24 * time.Hour)
	invalidatedAt := invalidatedCreated.Add(2 * time.Minute)
	if _, err := conn.Exec(ctx, `INSERT INTO token_accion (id, usuario_id, proposito, token_hash, creado_en, expira_en, invalidado_en)
		VALUES ('00000000-0000-4000-8000-000000000035', $1, 'verificar_correo', repeat('f', 64), $2, $3, $4)`, userID, invalidatedCreated, invalidatedExpiry, invalidatedAt); err != nil {
		t.Fatal(err)
	}
	var storedInvalidatedAt, storedInvalidationCreated pgtype.Timestamptz
	var storedConsumedAt pgtype.Timestamptz
	if err := conn.QueryRow(ctx, `SELECT creado_en, consumido_en, invalidado_en FROM token_accion WHERE id = '00000000-0000-4000-8000-000000000035'`).Scan(&storedInvalidationCreated, &storedConsumedAt, &storedInvalidatedAt); err != nil {
		t.Fatal(err)
	}
	if !storedInvalidationCreated.Time.Equal(invalidatedCreated) || storedConsumedAt.Valid || !storedInvalidatedAt.Valid || !storedInvalidatedAt.Time.Equal(invalidatedAt) {
		t.Fatalf("invalidated token fields did not persist: consumed_valid=%t invalidated_valid=%t", storedConsumedAt.Valid, storedInvalidatedAt.Valid)
	}
	if _, err := conn.Exec(ctx, `INSERT INTO token_accion (id, usuario_id, proposito, token_hash, creado_en, expira_en)
		VALUES ('00000000-0000-4000-8000-000000000034', $1, 'verificar_correo', repeat('e', 64), now(), now() + interval '24 hours')`, userID); !hasSQLState(err, "23505") {
		t.Errorf("duplicate token hash error=%v, want unique violation 23505", err)
	}

	sessionCreated := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	sessionActivity := sessionCreated.Add(time.Hour)
	sessionExpiry := sessionCreated.Add(8 * time.Hour)
	if _, err := conn.Exec(ctx, `INSERT INTO sesion (id, usuario_id, token_hash, creada_en, ultima_actividad_en, expira_en)
		VALUES ('00000000-0000-4000-8000-000000000041', $1, repeat('d', 64), $2, $3, $4)`, userID, sessionCreated, sessionActivity, sessionExpiry); err != nil {
		t.Fatal(err)
	}
	var storedSessionCreated, storedSessionActivity, storedSessionExpiry time.Time
	if err := conn.QueryRow(ctx, `SELECT creada_en, ultima_actividad_en, expira_en FROM sesion WHERE id = '00000000-0000-4000-8000-000000000041'`).Scan(&storedSessionCreated, &storedSessionActivity, &storedSessionExpiry); err != nil {
		t.Fatal(err)
	}
	if !storedSessionCreated.Equal(sessionCreated) || !storedSessionActivity.Equal(sessionActivity) || !storedSessionExpiry.Equal(sessionExpiry) {
		t.Fatalf("session timestamps did not persist: created=%s activity=%s expiry=%s", storedSessionCreated, storedSessionActivity, storedSessionExpiry)
	}
	if _, err := conn.Exec(ctx, `INSERT INTO sesion (id, usuario_id, token_hash, creada_en, ultima_actividad_en, expira_en)
		VALUES ('00000000-0000-4000-8000-000000000042', $1, repeat('d', 64), $2, $2, $3)`, userID, sessionCreated, sessionExpiry); !hasSQLState(err, "23505") {
		t.Errorf("duplicate session token hash error=%v, want unique violation 23505", err)
	}

	if _, err := conn.Exec(ctx, `INSERT INTO rol_usuario (usuario_id, rol) VALUES ($1, 'arrendatario')`, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, `INSERT INTO rol_usuario (usuario_id, rol) VALUES ($1, 'arrendatario')`, userID); !hasSQLState(err, "23505") {
		t.Errorf("duplicate user role error=%v, want unique violation 23505", err)
	}
	if _, err := conn.Exec(ctx, `INSERT INTO rol_usuario (usuario_id, rol) VALUES ($1, 'superusuario')`, userID); !hasSQLState(err, "23514") {
		t.Errorf("unsupported user role error=%v, want check violation 23514", err)
	}

	if _, err := conn.Exec(ctx, `INSERT INTO version_terminos (id, codigo, tipo, hash_sha256, publicada_en)
		VALUES ('00000000-0000-4000-8000-000000000050', 'sintetico-terminos-v0.1', 'terminos', repeat('f', 64), now())`); !hasSQLState(err, "23505") {
		t.Errorf("duplicate terms code error=%v, want unique violation 23505", err)
	}
	const termsVersionID = "00000000-0000-4000-8000-000000000001"
	termsAcceptedAt := time.Date(2026, time.October, 4, 1, 0, 0, 0, time.UTC)
	if _, err := conn.Exec(ctx, `INSERT INTO aceptacion_terminos (id, usuario_id, version_id, aceptada_en, canal)
		VALUES ('00000000-0000-4000-8000-000000000051', $1, $2, $3, 'web')`, userID, termsVersionID, termsAcceptedAt); err != nil {
		t.Fatal(err)
	}
	var storedTermsAcceptance time.Time
	var storedAcceptanceChannel string
	if err := conn.QueryRow(ctx, `SELECT aceptada_en, canal FROM aceptacion_terminos WHERE id = '00000000-0000-4000-8000-000000000051'`).Scan(&storedTermsAcceptance, &storedAcceptanceChannel); err != nil {
		t.Fatal(err)
	}
	if !storedTermsAcceptance.Equal(termsAcceptedAt) || storedAcceptanceChannel != "web" {
		t.Fatalf("terms acceptance fields did not persist: accepted_at=%s channel=%q", storedTermsAcceptance, storedAcceptanceChannel)
	}
	if _, err := conn.Exec(ctx, `INSERT INTO aceptacion_terminos (id, usuario_id, version_id, canal)
		VALUES ('00000000-0000-4000-8000-000000000052', $1, $2, 'web')`, userID, termsVersionID); !hasSQLState(err, "23505") {
		t.Errorf("duplicate terms acceptance error=%v, want unique violation 23505", err)
	}

	if _, err := conn.Exec(ctx, `DELETE FROM usuario WHERE id = $1`, userID); !hasSQLState(err, "23001") && !hasSQLState(err, "23503") {
		t.Errorf("delete referenced account error=%v, want RESTRICT/foreign key violation", err)
	}
}

func assertUniqueCorreoConcurrent(t *testing.T, ctx context.Context, databaseURL string) {
	t.Helper()

	// Separate connections race on one canonical key; the database UNIQUE constraint must decide the winner.
	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make(chan error, 2)
	ids := []string{"00000000-0000-4000-8000-000000000011", "00000000-0000-4000-8000-000000000012"}
	for _, id := range ids {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			conn, err := pgx.Connect(ctx, databaseURL)
			if err != nil {
				results <- err
				return
			}
			defer conn.Close(context.Background())
			<-start
			_, err = conn.Exec(ctx, `INSERT INTO usuario (id, correo_original, correo_normalizado, hash_clave)
				VALUES ($1, $2, 'unicidad@ejemplo.invalid', 'hash-sintetico-no-utilizable')`, id, fmt.Sprintf("%s@ejemplo.invalid", id))
			results <- err
		}(id)
	}
	close(start)
	wg.Wait()
	close(results)
	inserted, rejected := 0, 0
	for err := range results {
		if err == nil {
			inserted++
		} else if hasSQLState(err, "23505") {
			rejected++
		} else {
			t.Errorf("concurrent canonical email insert error=%v, want success or unique violation", err)
		}
	}
	if inserted != 1 || rejected != 1 {
		t.Errorf("concurrent canonical email inserts: inserted=%d rejected=%d, want 1 each", inserted, rejected)
	}
}

func hasSQLState(err error, state string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == state
}

func TestM01FKIntegrityRejectsUnknownUser(t *testing.T) {
	withTestDatabase(t, func(ctx context.Context, databaseURL string) {
		if _, err := Run(ctx, databaseURL, filepath.Join("..", "..", "db", "migrations")); err != nil {
			t.Fatal(err)
		}
		conn := connectTestDB(t, ctx, databaseURL)
		defer conn.Close(context.Background())
		if _, err := conn.Exec(ctx, `INSERT INTO rol_usuario (usuario_id, rol)
			VALUES ('00000000-0000-4000-8000-000000000099', 'arrendatario')`); !hasSQLState(err, "23503") {
			t.Fatalf("unknown user foreign key error=%v, want 23503", err)
		}
	})
}
