package adminglobal

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/accountlock"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/jackc/pgx/v5"
)

var ErrConflict = errors.New("adminglobal: version or state conflict")

// AccountProfileInput deliberately excludes email, state, roles and arbitrary JSON.
type AccountProfileInput struct {
	DisplayName string    `json:"display_name"`
	Phone       *string   `json:"phone"`
	ExpectedAt  time.Time `json:"expected_updated_at"`
	Reason      string    `json:"reason"`
}

type AccountProfile struct {
	AccountID string    `json:"account_id"`
	Email     string    `json:"email"`
	Name      string    `json:"display_name"`
	Phone     *string   `json:"phone"`
	UpdatedAt time.Time `json:"updated_at"`
}

type RoleChange struct {
	AccountID string    `json:"account_id"`
	Role      string    `json:"role"`
	Granted   bool      `json:"granted"`
	ChangedAt time.Time `json:"changed_at"`
}

type SessionRevocation struct {
	AccountID string    `json:"account_id"`
	Revoked   int64     `json:"revoked_sessions"`
	RevokedAt time.Time `json:"revoked_at"`
}

var accountReasons = map[string]bool{"correccion_perfil": true, "revision_administrativa": true, "gestion_operativa": true}
var manageableRoles = map[string]bool{"arrendatario": true, "arrendador": true}

type AccountBlockChange struct {
	AccountID string    `json:"account_id"`
	Blocked   bool      `json:"blocked"`
	ChangedAt time.Time `json:"changed_at"`
}

func (s *Service) SetAccountBlocked(ctx context.Context, actor, target, reason, correlation string, blocked bool) (AccountBlockChange, error) {
	reasonAllowed := blocked && (reason == "riesgo_seguridad" || reason == "uso_indebido" || reason == "revision_administrativa") || !blocked && (reason == "revision_concluida" || reason == "error_bloqueo" || reason == "riesgo_resuelto")
	if !canonicalUUID.MatchString(actor) || !canonicalUUID.MatchString(target) || actor == target || !canonicalUUID.MatchString(correlation) || !reasonAllowed {
		return AccountBlockChange{}, ErrInvalid
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return AccountBlockChange{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err = accountlock.LockGlobalAdministratorSet(ctx, tx); err != nil {
		return AccountBlockChange{}, err
	}
	if err = lockManagedAccounts(ctx, tx, actor, target); err != nil {
		return AccountBlockChange{}, err
	}
	if err = requireGlobalActor(ctx, tx, actor); err != nil {
		return AccountBlockChange{}, err
	}
	var targetActive bool
	if err = tx.QueryRow(ctx, `SELECT estado='activo' FROM public.usuario WHERE id=$1`, target).Scan(&targetActive); errors.Is(err, pgx.ErrNoRows) {
		return AccountBlockChange{}, ErrNotFound
	} else if err != nil {
		return AccountBlockChange{}, err
	} else if !targetActive {
		return AccountBlockChange{}, ErrConflict
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.bloqueo_cuenta_administrativo_local WHERE cuenta_id=$1)`, target).Scan(&exists); err != nil {
		return AccountBlockChange{}, err
	}
	if exists == blocked {
		return AccountBlockChange{}, ErrConflict
	}
	at := s.now().UTC()
	if blocked {
		var masters int64
		if err = tx.QueryRow(ctx, `SELECT count(*) FROM public.administrador_global ag JOIN public.usuario u ON u.id=ag.usuario_id WHERE u.estado='activo' AND (u.bloqueado_hasta IS NULL OR u.bloqueado_hasta<=$1) AND NOT EXISTS(SELECT 1 FROM public.bloqueo_cuenta_administrativo_local b WHERE b.cuenta_id=u.id)`, at).Scan(&masters); err != nil {
			return AccountBlockChange{}, err
		}
		var isMaster bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.administrador_global WHERE usuario_id=$1)`, target).Scan(&isMaster); err != nil {
			return AccountBlockChange{}, err
		}
		if isMaster && masters <= 1 {
			return AccountBlockChange{}, ErrConflict
		}
		if _, err = tx.Exec(ctx, `INSERT INTO public.bloqueo_cuenta_administrativo_local(cuenta_id,bloqueada_por,motivo_codigo,bloqueada_en,correlacion_id) VALUES($1,$2,$3,$4,$5)`, target, actor, reason, at, correlation); err != nil {
			return AccountBlockChange{}, err
		}
	} else {
		if _, err = tx.Exec(ctx, `DELETE FROM public.bloqueo_cuenta_administrativo_local WHERE cuenta_id=$1`, target); err != nil {
			return AccountBlockChange{}, err
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE public.sesion SET revocada_en=$2 WHERE usuario_id=$1 AND revocada_en IS NULL`, target, at); err != nil {
		return AccountBlockChange{}, err
	}
	if blocked {
		if _, err = tx.Exec(ctx, `UPDATE public.token_accion SET invalidado_en=$2 WHERE usuario_id=$1 AND invalidado_en IS NULL AND consumido_en IS NULL`, target, at); err != nil {
			return AccountBlockChange{}, err
		}
	}
	changeID, e := s.ids.ID()
	if e != nil {
		return AccountBlockChange{}, e
	}
	action := "bloquear"
	if !blocked {
		action = "desbloquear"
	}
	if _, err = tx.Exec(ctx, `INSERT INTO public.bloqueo_cuenta_historial_local(id,cuenta_id,actor_id,accion,motivo_codigo,ocurrida_en,correlacion_id) VALUES($1,$2,$3,$4,$5,$6,$7)`, changeID, target, actor, action, reason, at, correlation); err != nil {
		return AccountBlockChange{}, err
	}
	auditAction := "admin.global.accounts.block"
	if !blocked {
		auditAction = "admin.global.accounts.unblock"
	}
	if err = s.auditWithTarget(ctx, tx, actor, target, auditAction, "exito", reason, correlation, at, target); err != nil {
		return AccountBlockChange{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return AccountBlockChange{}, err
	}
	return AccountBlockChange{AccountID: target, Blocked: blocked, ChangedAt: at}, nil
}

func (s *Service) UpdateAccountProfile(ctx context.Context, actor, target, correlation string, in AccountProfileInput) (AccountProfile, error) {
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	if !canonicalUUID.MatchString(actor) || !canonicalUUID.MatchString(target) || actor == target || !canonicalUUID.MatchString(correlation) || !accountReasons[in.Reason] || in.ExpectedAt.IsZero() || len([]rune(in.DisplayName)) < 1 || len([]rune(in.DisplayName)) > 120 {
		return AccountProfile{}, ErrInvalid
	}
	if in.Phone != nil {
		phone := strings.TrimSpace(*in.Phone)
		if phone != "" && (len(phone) != 9 || strings.Trim(phone, "0123456789") != "") {
			return AccountProfile{}, ErrInvalid
		}
		if phone == "" {
			in.Phone = nil
		} else {
			in.Phone = &phone
		}
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return AccountProfile{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err = lockManagedAccounts(ctx, tx, actor, target); err != nil {
		return AccountProfile{}, err
	}
	if err = requireGlobalActor(ctx, tx, actor); err != nil {
		return AccountProfile{}, err
	}
	var out AccountProfile
	err = tx.QueryRow(ctx, `UPDATE public.perfil_usuario p SET nombre_visible=$3,telefono_normalizado=$4,actualizado_en=$5
		FROM public.usuario u WHERE p.usuario_id=u.id AND u.id=$1 AND p.usuario_id=$1 AND p.actualizado_en=$2 AND u.estado='activo'
		AND NOT EXISTS(SELECT 1 FROM public.bloqueo_cuenta_administrativo_local b WHERE b.cuenta_id=u.id)
		RETURNING u.id::text,u.correo_original,p.nombre_visible,p.telefono_normalizado,p.actualizado_en`, target, in.ExpectedAt.UTC(), in.DisplayName, in.Phone, s.now().UTC()).Scan(&out.AccountID, &out.Email, &out.Name, &out.Phone, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		var exists bool
		if e := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.perfil_usuario WHERE usuario_id=$1)`, target).Scan(&exists); e != nil {
			return AccountProfile{}, e
		}
		if exists {
			return AccountProfile{}, ErrConflict
		}
		return AccountProfile{}, ErrNotFound
	}
	if err != nil {
		return AccountProfile{}, err
	}
	if err = s.auditWithTarget(ctx, tx, actor, target, "admin.global.accounts.update", "exito", in.Reason, correlation, out.UpdatedAt, target); err != nil {
		return AccountProfile{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return AccountProfile{}, err
	}
	return out, nil
}

// SetAccountRole is an idempotent desired-state operation. It only manages ordinary
// marketplace roles; administrator and global membership require separate reviewed flows.
func (s *Service) SetAccountRole(ctx context.Context, actor, target, role, reason, correlation string, granted bool) (RoleChange, error) {
	if !canonicalUUID.MatchString(actor) || !canonicalUUID.MatchString(target) || actor == target || !canonicalUUID.MatchString(correlation) || !manageableRoles[role] || !accountReasons[reason] {
		return RoleChange{}, ErrInvalid
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return RoleChange{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err = lockManagedAccounts(ctx, tx, actor, target); err != nil {
		return RoleChange{}, err
	}
	if err = requireGlobalActor(ctx, tx, actor); err != nil {
		return RoleChange{}, err
	}
	var active bool
	if err = tx.QueryRow(ctx, `SELECT estado='activo' AND NOT EXISTS(SELECT 1 FROM public.bloqueo_cuenta_administrativo_local b WHERE b.cuenta_id=u.id) FROM public.usuario u WHERE id=$1`, target).Scan(&active); errors.Is(err, pgx.ErrNoRows) {
		return RoleChange{}, ErrNotFound
	} else if err != nil {
		return RoleChange{}, err
	} else if !active {
		return RoleChange{}, ErrConflict
	}
	var changed bool
	if granted {
		tag, e := tx.Exec(ctx, `INSERT INTO public.rol_usuario(usuario_id,rol,concedido_por) VALUES($1,$2,$3) ON CONFLICT(usuario_id,rol) DO NOTHING`, target, role, actor)
		if e != nil {
			return RoleChange{}, e
		}
		changed = tag.RowsAffected() > 0
	} else {
		tag, e := tx.Exec(ctx, `DELETE FROM public.rol_usuario WHERE usuario_id=$1 AND rol=$2`, target, role)
		if e != nil {
			return RoleChange{}, e
		}
		changed = tag.RowsAffected() > 0
	}
	at := s.now().UTC()
	result := "exito"
	auditReason := reason
	if !changed {
		auditReason = "estado_idempotente"
	}
	if err = s.auditWithTarget(ctx, tx, actor, target, "admin.global.accounts.roles."+map[bool]string{true: "grant", false: "revoke"}[granted]+"."+role, result, auditReason, correlation, at, target); err != nil {
		return RoleChange{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return RoleChange{}, err
	}
	return RoleChange{AccountID: target, Role: role, Granted: granted, ChangedAt: at}, nil
}

func (s *Service) RevokeAccountSessions(ctx context.Context, actor, target, reason, correlation string) (SessionRevocation, error) {
	if !canonicalUUID.MatchString(actor) || !canonicalUUID.MatchString(target) || actor == target || !canonicalUUID.MatchString(correlation) || !accountReasons[reason] {
		return SessionRevocation{}, ErrInvalid
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return SessionRevocation{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err = lockManagedAccounts(ctx, tx, actor, target); err != nil {
		return SessionRevocation{}, err
	}
	if err = requireGlobalActor(ctx, tx, actor); err != nil {
		return SessionRevocation{}, err
	}
	var targetActive bool
	if err = tx.QueryRow(ctx, `SELECT estado='activo' AND NOT EXISTS(SELECT 1 FROM public.bloqueo_cuenta_administrativo_local b WHERE b.cuenta_id=u.id) FROM public.usuario u WHERE id=$1`, target).Scan(&targetActive); errors.Is(err, pgx.ErrNoRows) {
		return SessionRevocation{}, ErrNotFound
	} else if err != nil {
		return SessionRevocation{}, err
	} else if !targetActive {
		return SessionRevocation{}, ErrConflict
	}
	at := s.now().UTC()
	tag, err := tx.Exec(ctx, `UPDATE public.sesion SET revocada_en=$2 WHERE usuario_id=$1 AND revocada_en IS NULL`, target, at)
	if err != nil {
		return SessionRevocation{}, err
	}
	if err = s.auditWithTarget(ctx, tx, actor, target, "admin.global.accounts.sessions.revoke", "exito", reason, correlation, at, target); err != nil {
		return SessionRevocation{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return SessionRevocation{}, err
	}
	return SessionRevocation{AccountID: target, Revoked: tag.RowsAffected(), RevokedAt: at}, nil
}

func lockManagedAccounts(ctx context.Context, tx pgx.Tx, actor, target string) error {
	rows, err := tx.Query(ctx, `SELECT id FROM public.usuario WHERE id=$1 OR id=$2 ORDER BY id FOR UPDATE`, actor, target)
	if err != nil {
		return err
	}
	defer rows.Close()
	found := 0
	for rows.Next() {
		found++
	}
	if err = rows.Err(); err != nil {
		return err
	}
	if found != 2 {
		return ErrNotFound
	}
	return nil
}

// Keep a compile-time reference to the explicit capability contract used by HTTP.
var _ = identity.CapabilityAccountsUpdate
