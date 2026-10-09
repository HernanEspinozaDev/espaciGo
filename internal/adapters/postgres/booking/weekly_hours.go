package bookingpg

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/accountlock"
	"github.com/HernanEspinozaDev/espaciGo/internal/booking"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) WeeklyHoursForSpace(ctx context.Context, spaceID string) (booking.WeeklyHours, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return booking.WeeklyHours{}, err
	}
	defer tx.Rollback(ctx)
	value, err := r.weeklyHours(ctx, tx, spaceID, "")
	if err != nil {
		return booking.WeeklyHours{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return booking.WeeklyHours{}, err
	}
	return value, nil
}

func (r *Repository) WeeklyHoursForHost(ctx context.Context, hostID, spaceID string) (booking.WeeklyHours, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return booking.WeeklyHours{}, err
	}
	defer tx.Rollback(ctx)
	value, err := r.weeklyHours(ctx, tx, spaceID, hostID)
	if err != nil {
		return booking.WeeklyHours{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return booking.WeeklyHours{}, err
	}
	return value, nil
}

type rowQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func (r *Repository) weeklyHours(ctx context.Context, db rowQuerier, spaceID, hostID string) (booking.WeeklyHours, error) {
	var out booking.WeeklyHours
	var rateUnit string
	var zone sql.NullString
	var err error
	if hostID == "" {
		err = db.QueryRow(ctx, `SELECT e.id::text,e.zona_horaria,COALESCE(h.activo,false),t.modalidad
FROM public.espacio e
LEFT JOIN public.reserva_ensayo_local_fixture f ON f.espacio_id=e.id AND f.anfitrion_id=e.propietario_id
JOIN LATERAL(SELECT modalidad FROM public.tarifa_espacio WHERE espacio_id=e.id ORDER BY version DESC LIMIT 1)t ON true
LEFT JOIN public.espacio_horario_semanal h ON h.espacio_id=e.id
WHERE e.id=$1 AND ((f.habilitada AND e.estado='borrador') OR (NOT COALESCE(f.habilitada,false) AND e.estado='activa' AND EXISTS(
    SELECT 1 FROM public.elegibilidad_verificacion_local eligibility
    JOIN public.verificacion verification ON verification.id=eligibility.verificacion_id AND verification.usuario_id=eligibility.usuario_id AND verification.tipo=eligibility.tipo AND verification.estado='aprobada'
    JOIN public.usuario owner ON owner.id=eligibility.usuario_id AND owner.estado='activo'
    WHERE eligibility.usuario_id=e.propietario_id AND eligibility.tipo='kyc' AND eligibility.estado='elegible')))`, spaceID).Scan(&out.SpaceID, &zone, &out.Enabled, &rateUnit)
	} else {
		err = db.QueryRow(ctx, `SELECT e.id::text,e.zona_horaria,COALESCE(h.activo,false),t.modalidad
FROM public.reserva_ensayo_local_fixture f
JOIN public.espacio e ON e.id=f.espacio_id AND e.propietario_id=f.anfitrion_id AND e.estado='borrador'
JOIN LATERAL(SELECT modalidad FROM public.tarifa_espacio WHERE espacio_id=e.id ORDER BY version DESC LIMIT 1)t ON true
LEFT JOIN public.espacio_horario_semanal h ON h.espacio_id=e.id
		WHERE f.espacio_id=$1 AND f.habilitada AND f.anfitrion_id=$2`, spaceID, hostID).Scan(&out.SpaceID, &zone, &out.Enabled, &rateUnit)
	}
	if err != nil {
		return booking.WeeklyHours{}, mapErr(err)
	}
	if rateUnit != "hora" {
		return booking.WeeklyHours{}, booking.ErrInvalid
	}
	if zone.Valid {
		out.TimeZone = zone.String
	}
	return readWeeklyDays(ctx, db, out)
}

func readWeeklyDays(ctx context.Context, db rowQuerier, value booking.WeeklyHours) (booking.WeeklyHours, error) {
	value.Days = make([]booking.WeeklyDay, 7)
	for i := range value.Days {
		value.Days[i] = booking.WeeklyDay{Weekday: i + 1, Periods: []booking.WeeklyPeriod{}}
	}
	rows, err := db.Query(ctx, `SELECT dia_iso,apertura_minuto,cierre_minuto FROM public.espacio_horario_semanal_tramo WHERE espacio_id=$1 ORDER BY dia_iso,orden`, value.SpaceID)
	if err != nil {
		return booking.WeeklyHours{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var weekday, open, close int
		if err = rows.Scan(&weekday, &open, &close); err != nil {
			return booking.WeeklyHours{}, err
		}
		value.Days[weekday-1].Periods = append(value.Days[weekday-1].Periods, booking.WeeklyPeriod{Open: formatMinute(open), Close: formatMinute(close)})
	}
	if err = rows.Err(); err != nil {
		return booking.WeeklyHours{}, err
	}
	return value, nil
}

func (r *Repository) SaveWeeklyHours(ctx context.Context, hostID, spaceID string, value booking.WeeklyHours) (booking.WeeklyHours, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return booking.WeeklyHours{}, err
	}
	defer tx.Rollback(ctx)
	active, err := accountlock.LockActive(ctx, tx, hostID)
	if err != nil {
		return booking.WeeklyHours{}, err
	}
	if !active {
		return booking.WeeklyHours{}, booking.ErrNotFound
	}
	var rateUnit string
	var zone sql.NullString
	err = tx.QueryRow(ctx, `SELECT t.modalidad,e.zona_horaria FROM public.espacio e
JOIN public.reserva_ensayo_local_fixture f ON f.espacio_id=e.id AND f.anfitrion_id=e.propietario_id AND f.habilitada
JOIN LATERAL(SELECT modalidad FROM public.tarifa_espacio WHERE espacio_id=e.id ORDER BY version DESC LIMIT 1)t ON true
WHERE e.id=$1 AND e.propietario_id=$2 AND e.estado='borrador' FOR UPDATE OF e`, spaceID, hostID).Scan(&rateUnit, &zone)
	if err != nil {
		return booking.WeeklyHours{}, mapErr(err)
	}
	if rateUnit != "hora" || booking.ValidateWeeklyHours(value) != nil {
		return booking.WeeklyHours{}, booking.ErrInvalid
	}
	if value.Enabled {
		if !zone.Valid || strings.TrimSpace(zone.String) == "" {
			return booking.WeeklyHours{}, booking.ErrInvalid
		}
		if _, err = time.LoadLocation(zone.String); err != nil {
			return booking.WeeklyHours{}, booking.ErrInvalid
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.espacio_horario_semanal(espacio_id,activo,actualizado_en) VALUES($1,$2,now())
ON CONFLICT (espacio_id) DO UPDATE SET activo=EXCLUDED.activo,actualizado_en=now()`, spaceID, value.Enabled)
	if err != nil {
		return booking.WeeklyHours{}, mapErr(err)
	}
	if _, err = tx.Exec(ctx, `DELETE FROM public.espacio_horario_semanal_tramo WHERE espacio_id=$1`, spaceID); err != nil {
		return booking.WeeklyHours{}, err
	}
	for _, day := range value.Days {
		for index, period := range day.Periods {
			open, _ := parseWeeklyMinute(period.Open, false)
			close, _ := parseWeeklyMinute(period.Close, true)
			if _, err = tx.Exec(ctx, `INSERT INTO public.espacio_horario_semanal_tramo(espacio_id,dia_iso,orden,apertura_minuto,cierre_minuto) VALUES($1,$2,$3,$4,$5)`, spaceID, day.Weekday, index+1, open, close); err != nil {
				return booking.WeeklyHours{}, mapErr(err)
			}
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return booking.WeeklyHours{}, mapErr(err)
	}
	return r.WeeklyHoursForHost(ctx, hostID, spaceID)
}

func (r *Repository) weeklyHoursTx(ctx context.Context, tx pgx.Tx, spaceID string) (booking.WeeklyHours, error) {
	var value booking.WeeklyHours
	var rateUnit string
	var zone sql.NullString
	err := tx.QueryRow(ctx, `SELECT e.id::text,e.zona_horaria,COALESCE(h.activo,false),t.modalidad
FROM public.espacio e JOIN LATERAL(SELECT modalidad FROM public.tarifa_espacio WHERE espacio_id=e.id ORDER BY version DESC LIMIT 1)t ON true
LEFT JOIN public.espacio_horario_semanal h ON h.espacio_id=e.id WHERE e.id=$1`, spaceID).Scan(&value.SpaceID, &zone, &value.Enabled, &rateUnit)
	if err != nil {
		return booking.WeeklyHours{}, mapErr(err)
	}
	if rateUnit != "hora" {
		value.Enabled = false
		return value, nil
	}
	if zone.Valid {
		value.TimeZone = zone.String
	}
	return readWeeklyDays(ctx, tx, value)
}

func formatMinute(value int) string {
	if value == 1440 {
		return "24:00"
	}
	return fmtClock(value/60, value%60)
}

func fmtClock(hour, minute int) string {
	return string([]byte{'0' + byte(hour/10), '0' + byte(hour%10), ':', '0' + byte(minute/10), '0' + byte(minute%10)})
}

func parseWeeklyMinute(value string, closing bool) (int, bool) {
	if closing && value == "24:00" {
		return 1440, true
	}
	if len(value) != 5 || value[2] != ':' || value[0] < '0' || value[0] > '2' || value[1] < '0' || value[1] > '9' || value[3] < '0' || value[3] > '5' || value[4] < '0' || value[4] > '9' {
		return 0, false
	}
	hour := int(value[0]-'0')*10 + int(value[1]-'0')
	minute := int(value[3]-'0')*10 + int(value[4]-'0')
	if hour > 23 {
		return 0, false
	}
	return hour*60 + minute, true
}
