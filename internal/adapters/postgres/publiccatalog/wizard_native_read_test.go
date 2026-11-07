package publiccatalogpg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"syscall"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/publiccatalog"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This opt-in probe reads the verified day draft and performs no fixture writes.
func TestWizardNativeDetailReadOnlyDraftExcludedFromPublicCatalog(t *testing.T) {
	if os.Getenv("ESPACIGO_WIZARD_PUBLIC_SQL_READ") != "1" {
		t.Skip("set ESPACIGO_WIZARD_PUBLIC_SQL_READ=1 to run against the authorized native test DB")
	}
	root, pool, ctx := wizardNativeReadPool(t)
	dayID := wizardNativeIntentID(t, root, "native-wizard-intent-dia.json")
	if _, err := New(pool).Detail(ctx, dayID); !errors.Is(err, publiccatalog.ErrNotFound) {
		t.Fatalf("day draft Detail error=%v; want publiccatalog.ErrNotFound", err)
	}
}

// Run only after the explicitly authorized hourly publication has completed.
func TestWizardNativeDetailReadOnlyPublishedHourVisibleWithAudit(t *testing.T) {
	if os.Getenv("ESPACIGO_WIZARD_PUBLIC_PUBLISHED_READ") != "1" {
		t.Skip("set ESPACIGO_WIZARD_PUBLIC_PUBLISHED_READ=1 after authorized hourly publication")
	}
	root, pool, ctx := wizardNativeReadPool(t)
	hourID := wizardNativeIntentID(t, root, "native-wizard-intent-hora.json")

	var state string
	if err := pool.QueryRow(ctx, `SELECT estado FROM public.espacio WHERE id=$1`, hourID).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != "activa" {
		t.Fatalf("hour draft state=%q; want activa", state)
	}
	var auditCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.espacio_publicacion_historial_local h JOIN public.espacio e ON e.id=h.espacio_id WHERE h.espacio_id=$1 AND h.actor_id=e.propietario_id AND h.estado_anterior='borrador' AND h.estado_nuevo='activa' AND length(trim(h.correlacion_id))>0`, hourID).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 {
		t.Fatalf("matching owner publication audit rows=%d; want 1", auditCount)
	}

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly, IsoLevel: pgx.RepeatableRead})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	var scheduleJSON []byte
	projectionSQL := `SELECT ` + weeklyScheduleProjectionSQL + ` FROM public.espacio e JOIN public.tarifa_espacio t ON t.espacio_id=e.id WHERE e.id=$1 AND t.modalidad='hora'`
	if err := tx.QueryRow(ctx, projectionSQL, hourID).Scan(&scheduleJSON); err != nil {
		t.Fatalf("read published-hour schedule projection: %v", err)
	}
	var schedule publiccatalog.PublicWeeklySchedule
	if err := json.Unmarshal(scheduleJSON, &schedule); err != nil {
		t.Fatal(err)
	}
	want := wizardNativeStoredSchedule(t, ctx, tx, hourID)
	if !reflect.DeepEqual(schedule, want) {
		t.Fatalf("public schedule projection differs from stored weekly segments: got=%+v want=%+v", schedule, want)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	got, err := New(pool).Detail(ctx, hourID)
	if err != nil {
		t.Fatalf("published hour Detail: %v", err)
	}
	if got.WeeklySchedule == nil || !reflect.DeepEqual(*got.WeeklySchedule, want) {
		t.Fatalf("published Detail schedule differs from stored weekly segments: got=%+v want=%+v", got.WeeklySchedule, want)
	}
}

func wizardNativeReadPool(t *testing.T) (string, *pgxpool.Pool, context.Context) {
	t.Helper()
	root, err := findRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	passfile := filepath.Join(root, ".local/testing/wizard-improvements/espacigo_api_login.pgpass")
	if info, err := os.Stat(passfile); err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("approved API passfile must exist with mode 0600: %v", err)
	}
	if os.Getenv("PGPASSFILE") != passfile {
		t.Fatalf("PGPASSFILE must name the approved absolute passfile %q", passfile)
	}
	const targetURL = "postgres://espacigo_api_login@127.0.0.1:15436/espacigo_wizard_20261106?sslmode=disable"
	cfg, err := pgxpool.ParseConfig(targetURL)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ConnConfig.Host != "127.0.0.1" || cfg.ConnConfig.Port != 15436 || cfg.ConnConfig.Database != "espacigo_wizard_20261106" || cfg.ConnConfig.User != "espacigo_api_login" {
		t.Fatal("refusing unexpected PostgreSQL target")
	}
	cfg.ConnConfig.RuntimeParams["default_transaction_read_only"] = "on"
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, `SET ROLE espacigo_runtime`)
		return err
	}
	ctx := context.Background()
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	var database, sessionUser, currentUser, readOnly string
	if err := pool.QueryRow(ctx, `SELECT current_database(),session_user,current_user,current_setting('default_transaction_read_only')`).Scan(&database, &sessionUser, &currentUser, &readOnly); err != nil {
		t.Fatal(err)
	}
	if database != "espacigo_wizard_20261106" || sessionUser != "espacigo_api_login" || currentUser != "espacigo_runtime" || readOnly != "on" {
		t.Fatalf("unexpected DB identity/read-only state: database=%q session_user=%q current_user=%q read_only=%q", database, sessionUser, currentUser, readOnly)
	}
	return root, pool, ctx
}

func wizardNativeIntentID(t *testing.T, root, filename string) string {
	t.Helper()
	intentPath := filepath.Join(root, ".local/testing/wizard-improvements", filename)
	intentInfo, err := os.Lstat(intentPath)
	if err != nil {
		t.Fatalf("private verified intent unavailable: %v", err)
	}
	stat, ok := intentInfo.Sys().(*syscall.Stat_t)
	if !ok || !intentInfo.Mode().IsRegular() || intentInfo.Mode().Perm() != 0600 || stat.Uid != uint32(os.Geteuid()) {
		t.Fatal("private verified intent must be a regular owner-owned 0600 file")
	}
	intentBytes, err := os.ReadFile(intentPath)
	if err != nil {
		t.Fatal(err)
	}
	var intent struct {
		SpaceID string `json:"spaceId"`
		Status  string `json:"status"`
	}
	if err := json.Unmarshal(intentBytes, &intent); err != nil {
		t.Fatal(err)
	}
	if intent.Status != "verified" || !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`).MatchString(intent.SpaceID) {
		t.Fatal("private intent must contain a verified canonical UUID")
	}
	return intent.SpaceID
}

func wizardNativeStoredSchedule(t *testing.T, ctx context.Context, tx pgx.Tx, id string) publiccatalog.PublicWeeklySchedule {
	t.Helper()
	rows, err := tx.Query(ctx, `SELECT dia_iso, apertura_minuto, cierre_minuto FROM public.espacio_horario_semanal_tramo WHERE espacio_id=$1 ORDER BY dia_iso, orden`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	days := make([]publiccatalog.PublicWeeklyDay, 7)
	for i := range days {
		days[i] = publiccatalog.PublicWeeklyDay{Weekday: i + 1, Periods: []publiccatalog.PublicWeeklyPeriod{}}
	}
	for rows.Next() {
		var day, open, close int
		if err := rows.Scan(&day, &open, &close); err != nil {
			t.Fatal(err)
		}
		if day < 1 || day > 7 {
			t.Fatalf("invalid stored weekday %d", day)
		}
		days[day-1].Periods = append(days[day-1].Periods, publiccatalog.PublicWeeklyPeriod{Open: formatNativeMinute(open), Close: formatNativeMinute(close)})
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return publiccatalog.PublicWeeklySchedule{Enabled: true, TimeZone: "America/Santiago", Days: days}
}

func formatNativeMinute(value int) string {
	if value == 1440 {
		return "24:00"
	}
	return fmt.Sprintf("%02d:%02d", value/60, value%60)
}

func findRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for dir := cwd; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("could not locate repository go.mod")
		}
	}
}
