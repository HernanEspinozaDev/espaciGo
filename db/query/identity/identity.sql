-- name: CreateAccount :exec
INSERT INTO public.usuario (
    id, correo_original, correo_normalizado, hash_clave, estado,
    creado_en, actualizado_en
) VALUES (
    sqlc.arg(id), sqlc.arg(email), sqlc.arg(normalized_email), sqlc.arg(password_hash),
    sqlc.arg(state), sqlc.arg(created_at), sqlc.arg(updated_at)
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
    bloqueado_hasta AS blocked_until
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
    bloqueado_hasta AS blocked_until
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
