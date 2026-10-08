-- name: CreateAccount :exec
INSERT INTO public.usuario (
    id, correo_original, correo_normalizado, hash_clave, estado,
    creado_en, actualizado_en, preferencia_uso
) VALUES (
    sqlc.arg(id), sqlc.arg(email), sqlc.arg(normalized_email), sqlc.arg(password_hash),
    sqlc.arg(state), sqlc.arg(created_at), sqlc.arg(updated_at), sqlc.arg(use_preference)
);

-- name: CreateTenantRole :exec
INSERT INTO public.rol_usuario (usuario_id, rol)
VALUES (sqlc.arg(account_id), 'arrendatario');

-- name: CreateTermsAcceptance :exec
INSERT INTO public.aceptacion_terminos (
    id, usuario_id, version_id, aceptada_en, canal
) VALUES (
    sqlc.arg(id), sqlc.arg(account_id), sqlc.arg(version_id), sqlc.arg(accepted_at), sqlc.arg(channel)
);

-- name: GetAccountByNormalizedEmail :one
SELECT
    id::text AS id,
    correo_original AS email,
    correo_normalizado AS normalized_email,
    hash_clave AS password_hash,
    estado AS state,
    creado_en AS created_at,
    actualizado_en AS updated_at,
    intentos_fallidos_consecutivos AS failed_attempts,
    bloqueado_hasta AS blocked_until, preferencia_uso AS use_preference
FROM public.usuario
WHERE correo_normalizado = sqlc.arg(normalized_email)
LIMIT 1;

-- name: GetAccountByID :one
SELECT
    id::text AS id,
    correo_original AS email,
    correo_normalizado AS normalized_email,
    hash_clave AS password_hash,
    estado AS state,
    creado_en AS created_at,
    actualizado_en AS updated_at,
    intentos_fallidos_consecutivos AS failed_attempts,
    bloqueado_hasta AS blocked_until, preferencia_uso AS use_preference
FROM public.usuario
WHERE id = sqlc.arg(id)
LIMIT 1;

-- name: UpdateLoginState :execrows
UPDATE public.usuario
SET estado = sqlc.arg(state),
    intentos_fallidos_consecutivos = sqlc.arg(failed_attempts),
    bloqueado_hasta = sqlc.arg(blocked_until),
    actualizado_en = now()
WHERE id = sqlc.arg(id);

-- name: CreateSession :exec
INSERT INTO public.sesion (
    id, usuario_id, token_hash, creada_en, ultima_actividad_en,
    expira_en, revocada_en, cliente_resumen
) VALUES (
    sqlc.arg(id), sqlc.arg(account_id), sqlc.arg(token_hash), sqlc.arg(created_at),
    sqlc.arg(last_activity_at), sqlc.arg(expires_at), sqlc.arg(revoked_at), sqlc.arg(client_summary)
);

-- name: GetSessionByTokenHash :one
SELECT
    id::text AS id,
    usuario_id::text AS account_id,
    token_hash,
    creada_en AS created_at,
    ultima_actividad_en AS last_activity_at,
    expira_en AS expires_at,
    revocada_en AS revoked_at,
    COALESCE(cliente_resumen, '') AS client_summary
FROM public.sesion
WHERE token_hash = sqlc.arg(token_hash)
LIMIT 1;

-- name: RevokeSession :execrows
UPDATE public.sesion
SET revocada_en = sqlc.arg(revoked_at)
WHERE id = sqlc.arg(id) AND revocada_en IS NULL;

-- name: TouchSession :execrows
UPDATE public.sesion
SET ultima_actividad_en = sqlc.arg(activity_at)
WHERE id = sqlc.arg(id)
  AND revocada_en IS NULL
  AND ultima_actividad_en <= sqlc.arg(activity_at)
  AND expira_en > sqlc.arg(activity_at)
  AND ultima_actividad_en + interval '30 minutes' > sqlc.arg(activity_at);

-- name: GetTermsVersion :one
SELECT
    id::text AS id,
    codigo AS code,
    tipo AS type,
    hash_sha256 AS sha256,
    publicada_en AS published_at
FROM public.version_terminos
WHERE id = sqlc.arg(id)
LIMIT 1;

-- name: InvalidateActiveActionTokens :execrows
UPDATE public.token_accion
SET invalidado_en = sqlc.arg(invalidated_at)
WHERE usuario_id = sqlc.arg(account_id)
  AND proposito = sqlc.arg(purpose)
  AND consumido_en IS NULL
  AND invalidado_en IS NULL
  AND expira_en > sqlc.arg(invalidated_at)
  AND intentos < 5;

-- name: LockAccountForActionToken :one
SELECT id::text
FROM public.usuario
WHERE id = sqlc.arg(account_id)
FOR UPDATE;

-- name: CreateActionToken :exec
INSERT INTO public.token_accion (
    id, usuario_id, proposito, token_hash, creado_en, expira_en,
    consumido_en, invalidado_en, intentos
) VALUES (
    sqlc.arg(id), sqlc.arg(account_id), sqlc.arg(purpose), sqlc.arg(token_hash),
    sqlc.arg(created_at), sqlc.arg(expires_at), sqlc.arg(consumed_at),
    sqlc.arg(invalidated_at), sqlc.arg(attempts)
);

-- name: GetActionTokenByHash :one
SELECT
    id::text AS id,
    usuario_id::text AS account_id,
    proposito AS purpose,
    token_hash,
    creado_en AS created_at,
    expira_en AS expires_at,
    consumido_en AS consumed_at,
    invalidado_en AS invalidated_at,
    intentos AS attempts
FROM public.token_accion
WHERE token_hash = sqlc.arg(token_hash)
LIMIT 1;

-- name: RecordActionTokenFailure :execrows
UPDATE public.token_accion
SET intentos = intentos + 1
WHERE token_hash = sqlc.arg(token_hash)
  AND expira_en > sqlc.arg(attempted_at)
  AND intentos < 5
  AND consumido_en IS NULL
  AND invalidado_en IS NULL;

-- name: ConsumeActionToken :execrows
UPDATE public.token_accion
SET consumido_en = sqlc.arg(consumed_at)
WHERE token_hash = sqlc.arg(token_hash)
  AND expira_en > sqlc.arg(consumed_at)
  AND intentos < 5
  AND consumido_en IS NULL
  AND invalidado_en IS NULL;

-- name: CountActionTokenEmissions :one
SELECT count(*)::bigint
FROM public.token_accion
WHERE usuario_id = sqlc.arg(account_id)
  AND proposito = sqlc.arg(purpose)
  AND creado_en >= sqlc.arg(since);

-- name: LockAccountByEmail :one
SELECT id::text AS id, correo_original AS email, correo_normalizado AS normalized_email,
    hash_clave AS password_hash, estado AS state, creado_en AS created_at,
    actualizado_en AS updated_at, intentos_fallidos_consecutivos AS failed_attempts,
    bloqueado_hasta AS blocked_until, preferencia_uso AS use_preference
FROM public.usuario
WHERE correo_normalizado = sqlc.arg(normalized_email)
FOR UPDATE;

-- name: LockAccountByVerificationID :one
SELECT u.id::text AS id, u.correo_original AS email, u.correo_normalizado AS normalized_email,
    u.hash_clave AS password_hash, u.estado AS state, u.creado_en AS created_at,
    u.actualizado_en AS updated_at, u.intentos_fallidos_consecutivos AS failed_attempts,
    u.bloqueado_hasta AS blocked_until, preferencia_uso AS use_preference
FROM public.usuario u JOIN public.token_accion t ON t.usuario_id = u.id
WHERE t.id = sqlc.arg(token_id) AND t.proposito = 'verificar_correo'
FOR UPDATE OF u;

-- name: LockAccountByActionTokenID :one
SELECT u.id::text AS id, u.correo_original AS email, u.correo_normalizado AS normalized_email,
    u.hash_clave AS password_hash, u.estado AS state, u.creado_en AS created_at,
    u.actualizado_en AS updated_at, u.intentos_fallidos_consecutivos AS failed_attempts,
    u.bloqueado_hasta AS blocked_until, preferencia_uso AS use_preference
FROM public.usuario u JOIN public.token_accion t ON t.usuario_id = u.id
WHERE t.id = sqlc.arg(token_id)
FOR UPDATE OF u;

-- name: LockAccountBySessionHash :one
SELECT u.id::text AS id, u.correo_original AS email, u.correo_normalizado AS normalized_email,
    u.hash_clave AS password_hash, u.estado AS state, u.creado_en AS created_at,
    u.actualizado_en AS updated_at, u.intentos_fallidos_consecutivos AS failed_attempts,
    u.bloqueado_hasta AS blocked_until, preferencia_uso AS use_preference
FROM public.usuario u JOIN public.sesion s ON s.usuario_id = u.id
WHERE s.token_hash = sqlc.arg(token_hash)
FOR UPDATE OF u;

-- name: GetAccountRoles :many
SELECT rol FROM public.rol_usuario WHERE usuario_id = sqlc.arg(account_id) ORDER BY rol;

-- name: GetVerificationTokenByID :one
SELECT id::text AS id, usuario_id::text AS account_id, proposito AS purpose,
    token_hash, creado_en AS created_at, expira_en AS expires_at,
    consumido_en AS consumed_at, invalidado_en AS invalidated_at, intentos AS attempts
FROM public.token_accion WHERE id = sqlc.arg(id) AND proposito = 'verificar_correo';

-- name: GetActionTokenByID :one
SELECT id::text AS id, usuario_id::text AS account_id, proposito AS purpose,
    token_hash, creado_en AS created_at, expira_en AS expires_at,
    consumido_en AS consumed_at, invalidado_en AS invalidated_at, intentos AS attempts
FROM public.token_accion WHERE id = sqlc.arg(id);

-- name: UpdatePasswordHash :exec
UPDATE public.usuario SET hash_clave = sqlc.arg(password_hash), actualizado_en = now()
WHERE id = sqlc.arg(account_id);

-- name: RevokeActiveSessions :exec
UPDATE public.sesion SET revocada_en = sqlc.arg(revoked_at)
WHERE usuario_id = sqlc.arg(account_id) AND revocada_en IS NULL;

-- M02 profile queries are always scoped to the authenticated account ID.
-- name: GetProfile :one
SELECT usuario_id::text AS account_id, nombre_visible, telefono_normalizado,
       actualizado_en
FROM public.perfil_usuario
WHERE usuario_id = sqlc.arg(account_id);

-- name: UpsertProfile :one
INSERT INTO public.perfil_usuario (usuario_id, nombre_visible, telefono_normalizado, actualizado_en)
VALUES (sqlc.arg(account_id), sqlc.arg(display_name), sqlc.narg(phone), now())
ON CONFLICT (usuario_id) DO UPDATE
SET nombre_visible = EXCLUDED.nombre_visible,
    telefono_normalizado = EXCLUDED.telefono_normalizado,
    actualizado_en = now()
RETURNING usuario_id::text AS account_id, nombre_visible, telefono_normalizado,
          actualizado_en;

-- name: CreateRightsRequest :one
INSERT INTO public.solicitud_titular (id, usuario_id, tipo, canal)
VALUES (sqlc.arg(id), sqlc.arg(account_id), sqlc.arg(kind), sqlc.arg(channel))
RETURNING id::text AS id, tipo AS kind, estado AS state, solicitada_en;

-- name: ListOwnRightsRequests :many
SELECT id::text AS id, tipo AS kind, estado AS state, solicitada_en
FROM public.solicitud_titular
WHERE usuario_id = sqlc.arg(account_id)
ORDER BY solicitada_en DESC, id DESC;

-- name: DeleteExpiredPasswordHistory :exec
DELETE FROM public.historial_clave_local
WHERE usuario_id = sqlc.arg(account_id) AND retirar_en <= sqlc.arg(at);

-- name: PurgeExpiredPasswordHistory :exec
DELETE FROM public.historial_clave_local WHERE retirar_en <= sqlc.arg(at);

-- name: ListPasswordHistory :many
SELECT hash_clave
FROM public.historial_clave_local
WHERE usuario_id = sqlc.arg(account_id) AND retirar_en > sqlc.arg(at)
ORDER BY dejo_de_ser_vigente_en DESC, id;

-- name: StorePreviousPasswordHash :exec
INSERT INTO public.historial_clave_local (
    id, usuario_id, hash_clave, dejo_de_ser_vigente_en, retirar_en, creado_en
) VALUES (
    sqlc.arg(id), sqlc.arg(account_id), sqlc.arg(password_hash),
    sqlc.arg(no_longer_current_at), sqlc.arg(remove_at), sqlc.arg(created_at)
);

-- name: EnqueueCredentialChanged :exec
INSERT INTO public.outbox_evento_local (
    id, agregado_tipo, agregado_id, tipo, clave_deduplicacion, version, creada_en, disponible_en
) VALUES (
    sqlc.arg(id), 'usuario', sqlc.arg(account_id), 'identidad.credencial_cambiada',
    sqlc.arg(dedupe_key), 1, sqlc.arg(created_at), sqlc.arg(created_at)
);

-- name: StartCredentialNoticeCycle :exec
INSERT INTO public.outbox_evento_ciclo_local(evento_id,numero_ciclo,estado,iniciada_en)
VALUES (sqlc.arg(event_id),1,'pendiente',sqlc.arg(started_at));

-- name: RecordCredentialChangeAudit :exec
INSERT INTO public.evento_auditoria_local (
    id, actor_id, recurso_tipo, recurso_id, accion, resultado, motivo_codigo,
    correlacion_id, ocurrido_en, retirar_en
) VALUES (
    sqlc.arg(id), sqlc.arg(actor_id), 'usuario', sqlc.arg(resource_id),
    'identidad.credencial_cambiar', 'exito', 'clave_actualizada',
    sqlc.arg(correlation_id), sqlc.arg(at), sqlc.arg(remove_at)
);

-- name: ClaimCredentialChangedNotice :one
WITH exhausted AS (
    UPDATE public.outbox_evento_local
       SET fallo_terminal_en=sqlc.arg(at), codigo_fallo_terminal='worker_interrupted',
           retirar_en=sqlc.arg(at)::timestamptz + interval '30 days', lease_hasta=NULL,
           ultimo_error='worker_interrupted'
     WHERE tipo='identidad.credencial_cambiada' AND entregada_en IS NULL AND cancelada_en IS NULL
       AND fallo_terminal_en IS NULL AND intentos_ciclo >= 8
       AND lease_hasta IS NOT NULL AND lease_hasta <= sqlc.arg(at)
     RETURNING id,ciclo_actual,intentos_ciclo,fallo_terminal_en,codigo_fallo_terminal
), closed_cycles AS (
    UPDATE public.outbox_evento_ciclo_local AS cycle
       SET estado='fallo_terminal', finalizada_en=exhausted.fallo_terminal_en,
           intentos=exhausted.intentos_ciclo, codigo_resultado=exhausted.codigo_fallo_terminal
      FROM exhausted
     WHERE cycle.evento_id=exhausted.id AND cycle.numero_ciclo=exhausted.ciclo_actual
     RETURNING cycle.evento_id
), candidate AS (
    SELECT event.id FROM public.outbox_evento_local AS event
    WHERE event.tipo = 'identidad.credencial_cambiada' AND event.entregada_en IS NULL AND event.cancelada_en IS NULL
      AND event.fallo_terminal_en IS NULL AND event.intentos_ciclo < 8
      AND event.disponible_en <= sqlc.arg(at) AND (event.lease_hasta IS NULL OR event.lease_hasta <= sqlc.arg(at))
      AND EXISTS (SELECT 1 FROM public.usuario account WHERE account.id=event.agregado_id AND account.estado='activo')
    ORDER BY event.disponible_en, event.creada_en, event.id
    FOR UPDATE SKIP LOCKED LIMIT 1
), claimed AS (
    UPDATE public.outbox_evento_local AS event
       SET lease_hasta=sqlc.arg(lease_until), intentos=event.intentos+1,
           intentos_ciclo=event.intentos_ciclo+1
      FROM candidate
     WHERE event.id=candidate.id
     RETURNING event.id,event.agregado_id,event.intentos,event.intentos_ciclo,
               event.ciclo_actual,event.lease_hasta
), claimed_cycle AS (
    UPDATE public.outbox_evento_ciclo_local AS cycle
       SET intentos=claimed.intentos_ciclo
      FROM claimed
     WHERE cycle.evento_id=claimed.id AND cycle.numero_ciclo=claimed.ciclo_actual
     RETURNING cycle.evento_id
)
SELECT claimed.id::text AS id, claimed.agregado_id::text AS account_id,
       claimed.intentos AS total_attempts, claimed.intentos_ciclo AS cycle_attempts,
       claimed.ciclo_actual AS cycle_number, claimed.lease_hasta AS lease_until
FROM claimed;

-- name: CompleteCredentialChangedNotice :execrows
WITH completed AS (
UPDATE public.outbox_evento_local
SET entregada_en = sqlc.arg(at)::timestamptz, retirar_en = sqlc.arg(at)::timestamptz + interval '30 days', lease_hasta = NULL, ultimo_error = NULL
WHERE id = sqlc.arg(id) AND entregada_en IS NULL AND lease_hasta = sqlc.arg(lease_until)
RETURNING id,ciclo_actual
)
UPDATE public.outbox_evento_ciclo_local AS cycle
SET estado='entregada', finalizada_en=sqlc.arg(at), codigo_resultado=NULL
FROM completed
WHERE cycle.evento_id=completed.id AND cycle.numero_ciclo=completed.ciclo_actual;

-- name: RetryCredentialChangedNotice :execrows
WITH changed AS (
UPDATE public.outbox_evento_local AS event
SET disponible_en = sqlc.arg(retry_at),
    lease_hasta=NULL,
    ultimo_error=sqlc.arg(error_code),
    fallo_terminal_en=CASE WHEN event.intentos_ciclo >= 8 THEN sqlc.arg(at)::timestamptz ELSE NULL::timestamptz END,
    codigo_fallo_terminal=CASE WHEN event.intentos_ciclo >= 8 THEN sqlc.arg(error_code) ELSE NULL END,
    retirar_en=CASE WHEN event.intentos_ciclo >= 8 THEN sqlc.arg(at)::timestamptz + interval '30 days' ELSE NULL END
WHERE event.id = sqlc.arg(id) AND event.entregada_en IS NULL AND event.lease_hasta = sqlc.arg(lease_until)
RETURNING event.id,event.ciclo_actual,event.intentos_ciclo,event.fallo_terminal_en,event.codigo_fallo_terminal
)
UPDATE public.outbox_evento_ciclo_local AS cycle
SET estado=CASE WHEN changed.intentos_ciclo >= 8 THEN 'fallo_terminal' ELSE 'pendiente' END,
    finalizada_en=CASE WHEN changed.intentos_ciclo >= 8 THEN sqlc.arg(at)::timestamptz ELSE NULL::timestamptz END,
    intentos=changed.intentos_ciclo,
    codigo_resultado=CASE WHEN changed.intentos_ciclo >= 8 THEN sqlc.arg(error_code) ELSE NULL END
FROM changed
WHERE cycle.evento_id=changed.id AND cycle.numero_ciclo=changed.ciclo_actual;

-- name: CancelInactiveCredentialNotice :execrows
WITH cancelled AS (
UPDATE public.outbox_evento_local
SET cancelada_en=sqlc.arg(at), motivo_cancelacion_codigo='destinatario_inactivo',
    retirar_en=sqlc.arg(at)::timestamptz + interval '30 days', lease_hasta=NULL,
    ultimo_error='recipient_inactive'
WHERE id=sqlc.arg(id) AND entregada_en IS NULL AND cancelada_en IS NULL
  AND lease_hasta=sqlc.arg(lease_until)
RETURNING id,ciclo_actual,cancelada_en
)
UPDATE public.outbox_evento_ciclo_local AS cycle
SET estado='cancelada',finalizada_en=cancelled.cancelada_en,codigo_resultado='recipient_inactive'
FROM cancelled
WHERE cycle.evento_id=cancelled.id AND cycle.numero_ciclo=cancelled.ciclo_actual;

-- name: CancelInactiveCredentialNotices :execrows
WITH candidate AS (
    SELECT event.id FROM public.outbox_evento_local AS event
    JOIN public.usuario AS account ON account.id=event.agregado_id
    WHERE event.tipo='identidad.credencial_cambiada' AND account.estado<>'activo'
      AND event.entregada_en IS NULL AND event.cancelada_en IS NULL
      AND event.fallo_terminal_en IS NULL AND (event.lease_hasta IS NULL OR event.lease_hasta<=sqlc.arg(at))
    ORDER BY event.creada_en,event.id FOR UPDATE OF event SKIP LOCKED LIMIT sqlc.arg(limit_rows)
), cancelled AS (
    UPDATE public.outbox_evento_local AS event
       SET cancelada_en=sqlc.arg(at),motivo_cancelacion_codigo='destinatario_inactivo',
           retirar_en=sqlc.arg(at)::timestamptz + interval '30 days',lease_hasta=NULL,
           ultimo_error='recipient_inactive'
      FROM candidate WHERE event.id=candidate.id
      RETURNING event.id,event.ciclo_actual,event.cancelada_en
)
UPDATE public.outbox_evento_ciclo_local AS cycle
SET estado='cancelada',finalizada_en=cancelled.cancelada_en,codigo_resultado='recipient_inactive'
FROM cancelled
WHERE cycle.evento_id=cancelled.id AND cycle.numero_ciclo=cancelled.ciclo_actual;

-- name: ListTerminalCredentialNotices :many
SELECT event.id::text AS id,event.agregado_id::text AS account_id,
       event.intentos AS total_attempts,event.ciclo_actual AS cycle_number,
       event.intentos_ciclo AS cycle_attempts,event.codigo_fallo_terminal AS error_code,
       event.fallo_terminal_en AS failed_at,event.retirar_en AS remove_at
FROM public.outbox_evento_local AS event
WHERE event.tipo='identidad.credencial_cambiada' AND event.fallo_terminal_en IS NOT NULL
ORDER BY event.fallo_terminal_en,event.id;

-- name: LockCredentialNoticeForRecovery :one
SELECT id::text AS id, agregado_id::text AS account_id, entregada_en,
       cancelada_en, fallo_terminal_en, intentos, ciclo_actual
FROM public.outbox_evento_local WHERE id=sqlc.arg(id) FOR UPDATE;

-- name: FindCredentialNoticeCycleByKey :one
SELECT numero_ciclo, estado, iniciada_en, finalizada_en, intentos, codigo_resultado,
       actor_reapertura_id::text AS actor_id, motivo_reapertura_codigo AS reason_code,
       correlacion_id, clave_idempotencia
FROM public.outbox_evento_ciclo_local
WHERE evento_id=sqlc.arg(event_id) AND clave_idempotencia=sqlc.arg(idempotency_key);

-- name: CreateCredentialNoticeRecoveryCycle :exec
INSERT INTO public.outbox_evento_ciclo_local(
    evento_id,numero_ciclo,estado,iniciada_en,actor_reapertura_id,
    motivo_reapertura_codigo,correlacion_id,clave_idempotencia
) VALUES (
    sqlc.arg(event_id),sqlc.arg(cycle_number),'pendiente',sqlc.arg(at),sqlc.arg(actor_id),
    sqlc.arg(reason_code),sqlc.arg(correlation_id),sqlc.arg(idempotency_key)
);

-- name: ReopenCredentialNotice :execrows
UPDATE public.outbox_evento_local
SET ciclo_actual=sqlc.arg(cycle_number),intentos_ciclo=0,fallo_terminal_en=NULL,
    codigo_fallo_terminal=NULL,disponible_en=sqlc.arg(at),lease_hasta=NULL,
    retirar_en=NULL,ultimo_error=NULL
WHERE id=sqlc.arg(id) AND fallo_terminal_en IS NOT NULL
  AND entregada_en IS NULL AND cancelada_en IS NULL;

-- name: RecordCredentialNoticeRecoveryAudit :exec
INSERT INTO public.evento_auditoria_local(
    id,actor_id,recurso_tipo,recurso_id,accion,resultado,motivo_codigo,
    correlacion_id,ocurrido_en,retirar_en,clave_idempotencia
) VALUES (
    sqlc.arg(audit_id),sqlc.arg(actor_id),'outbox_evento',sqlc.arg(event_id),
    'identity.credential_notice.reopen','exito',sqlc.arg(reason_code),
    sqlc.arg(correlation_id),sqlc.arg(at),sqlc.arg(remove_at),sqlc.arg(idempotency_key)
);

-- name: PurgeExpiredCredentialNotices :one
SELECT public.purge_expired_local_credential_notices(sqlc.arg(at),sqlc.arg(limit_rows)) AS purged;

-- name: ActiveCredentialNoticeRecipientEmail :one
SELECT correo_original FROM public.usuario
WHERE id=sqlc.arg(account_id) AND estado='activo';
