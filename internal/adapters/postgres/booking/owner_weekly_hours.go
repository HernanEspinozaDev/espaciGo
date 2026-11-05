package bookingpg

import (
	"context"
	"database/sql"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/accountlock"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/spaceversion"
	"github.com/HernanEspinozaDev/espaciGo/internal/booking"
)

func (r *Repository) WeeklyHoursForOwner(ctx context.Context, ownerID, spaceID string) (booking.WeeklyHours, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return booking.WeeklyHours{}, err
	}
	defer tx.Rollback(ctx)
	active, err := accountlock.LockActive(ctx, tx, ownerID)
	if err != nil {
		return booking.WeeklyHours{}, err
	}
	if !active {
		return booking.WeeklyHours{}, booking.ErrNotFound
	}
	var value booking.WeeklyHours
	var zone sql.NullString
	var rate string
	err = tx.QueryRow(ctx, `SELECT e.id::text,e.zona_horaria,COALESCE(h.activo,false),t.modalidad,e.actualizado_en
FROM public.espacio e JOIN public.usuario u ON u.id=e.propietario_id AND u.estado='activo'
JOIN LATERAL(SELECT modalidad FROM public.tarifa_espacio WHERE espacio_id=e.id ORDER BY version DESC LIMIT 1)t ON true
LEFT JOIN public.espacio_horario_semanal h ON h.espacio_id=e.id
WHERE e.id=$1 AND e.propietario_id=$2 AND e.estado<>'eliminado'`, spaceID, ownerID).Scan(&value.SpaceID, &zone, &value.Enabled, &rate, &value.UpdatedAt)
	if err != nil {
		return booking.WeeklyHours{}, mapErr(err)
	}
	if rate != "hora" {
		return booking.WeeklyHours{}, booking.ErrNotFound
	}
	if zone.Valid {
		value.TimeZone = zone.String
	}
	value, err = readWeeklyDays(ctx, tx, value)
	if err != nil {
		return booking.WeeklyHours{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return booking.WeeklyHours{}, mapErr(err)
	}
	return value, nil
}

func (r *Repository) SaveWeeklyHoursForOwnerCAS(ctx context.Context, ownerID, spaceID string, value booking.WeeklyHours, expected time.Time) (booking.WeeklyHours, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return booking.WeeklyHours{}, err
	}
	defer tx.Rollback(ctx)
	active, err := accountlock.LockActive(ctx, tx, ownerID)
	if err != nil {
		return booking.WeeklyHours{}, err
	}
	if !active {
		return booking.WeeklyHours{}, booking.ErrNotFound
	}
	var zone sql.NullString
	var rate string
	var actual time.Time
	err = tx.QueryRow(ctx, `SELECT e.zona_horaria,t.modalidad,e.actualizado_en FROM public.espacio e JOIN public.usuario u ON u.id=e.propietario_id AND u.estado='activo'
JOIN LATERAL(SELECT modalidad FROM public.tarifa_espacio WHERE espacio_id=e.id ORDER BY version DESC LIMIT 1)t ON true
WHERE e.id=$1 AND e.propietario_id=$2 AND e.estado<>'eliminado' FOR UPDATE OF e`, spaceID, ownerID).Scan(&zone, &rate, &actual)
	if err != nil {
		return booking.WeeklyHours{}, mapErr(err)
	}
	if !expected.IsZero() && !actual.Equal(expected) {
		return booking.WeeklyHours{}, booking.ErrConflict
	}
	if rate != "hora" || booking.ValidateWeeklyHours(value) != nil {
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
	current, err := r.weeklyHoursTx(ctx, tx, spaceID)
	if err != nil {
		return booking.WeeklyHours{}, err
	}
	if weeklyScheduleEquivalent(current, value) {
		if err = tx.Commit(ctx); err != nil {
			return booking.WeeklyHours{}, mapErr(err)
		}
		return current, nil
	}
	if _, err = tx.Exec(ctx, `INSERT INTO public.espacio_horario_semanal(espacio_id,activo,actualizado_en) VALUES($1,$2,now()) ON CONFLICT(espacio_id) DO UPDATE SET activo=EXCLUDED.activo,actualizado_en=now()`, spaceID, value.Enabled); err != nil {
		return booking.WeeklyHours{}, mapErr(err)
	}
	if _, err = tx.Exec(ctx, `DELETE FROM public.espacio_horario_semanal_tramo WHERE espacio_id=$1`, spaceID); err != nil {
		return booking.WeeklyHours{}, err
	}
	for _, day := range value.Days {
		for i, p := range day.Periods {
			open, _ := parseWeeklyMinute(p.Open, false)
			close, _ := parseWeeklyMinute(p.Close, true)
			if _, err = tx.Exec(ctx, `INSERT INTO public.espacio_horario_semanal_tramo(espacio_id,dia_iso,orden,apertura_minuto,cierre_minuto) VALUES($1,$2,$3,$4,$5)`, spaceID, day.Weekday, i+1, open, close); err != nil {
				return booking.WeeklyHours{}, mapErr(err)
			}
		}
	}
	updated, err := spaceversion.Bump(ctx, tx, spaceID)
	if err != nil {
		return booking.WeeklyHours{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return booking.WeeklyHours{}, mapErr(err)
	}
	value.SpaceID = spaceID
	value.TimeZone = zone.String
	value.UpdatedAt = updated
	return value, nil
}
