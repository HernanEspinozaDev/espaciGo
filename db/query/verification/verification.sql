-- name: CreateVerification :one
INSERT INTO public.verificacion (
    id, usuario_id, tipo, estado, proveedor_ref, referencia_evidencia,
    clave_idempotencia, creada_en
) VALUES (
    sqlc.arg(id), sqlc.arg(owner_id), sqlc.arg(type), 'en_revision',
    'local-fixture-v1', sqlc.arg(evidence_ref), sqlc.arg(idempotency_key), sqlc.arg(created_at)
)
ON CONFLICT (usuario_id, clave_idempotencia) DO NOTHING
RETURNING id::text AS id, usuario_id::text AS owner_id, tipo AS type,
    estado AS state, proveedor_ref AS provider, referencia_evidencia AS evidence_ref,
    COALESCE(reintento_de::text, ''::text) AS retry_of,
    COALESCE(motivo_codigo, '') AS reason_code,
    creada_en AS created_at, resuelta_en AS resolved_at,
    clave_idempotencia AS idempotency_key;

-- name: GetVerificationByIdempotency :one
SELECT id::text AS id, usuario_id::text AS owner_id, tipo AS type,
    estado AS state, proveedor_ref AS provider, referencia_evidencia AS evidence_ref,
    COALESCE(reintento_de::text, ''::text) AS retry_of,
    COALESCE(motivo_codigo, '') AS reason_code,
    creada_en AS created_at, resuelta_en AS resolved_at,
    clave_idempotencia AS idempotency_key
FROM public.verificacion
WHERE usuario_id = sqlc.arg(owner_id) AND clave_idempotencia = sqlc.arg(idempotency_key)
LIMIT 1;

-- name: GetOwnVerification :one
SELECT id::text AS id, usuario_id::text AS owner_id, tipo AS type,
    estado AS state, proveedor_ref AS provider, referencia_evidencia AS evidence_ref,
    COALESCE(reintento_de::text, ''::text) AS retry_of,
    COALESCE(motivo_codigo, '') AS reason_code,
    creada_en AS created_at, resuelta_en AS resolved_at,
    clave_idempotencia AS idempotency_key
FROM public.verificacion
WHERE usuario_id = sqlc.arg(owner_id) AND id = sqlc.arg(id)
LIMIT 1;

-- name: ListOwnVerifications :many
SELECT id::text AS id, usuario_id::text AS owner_id, tipo AS type,
    estado AS state, proveedor_ref AS provider, referencia_evidencia AS evidence_ref,
    COALESCE(reintento_de::text, ''::text) AS retry_of,
    COALESCE(motivo_codigo, '') AS reason_code,
    creada_en AS created_at, resuelta_en AS resolved_at,
    clave_idempotencia AS idempotency_key
FROM public.verificacion
WHERE usuario_id = sqlc.arg(owner_id)
ORDER BY creada_en DESC;

-- name: ListPendingVerifications :many
SELECT id::text AS id, usuario_id::text AS owner_id, tipo AS type,
    estado AS state, proveedor_ref AS provider, referencia_evidencia AS evidence_ref,
    COALESCE(reintento_de::text, ''::text) AS retry_of,
    COALESCE(motivo_codigo, '') AS reason_code,
    creada_en AS created_at, resuelta_en AS resolved_at,
    clave_idempotencia AS idempotency_key
FROM public.verificacion
WHERE estado = 'en_revision'
ORDER BY creada_en;

-- name: ReviewVerification :one
UPDATE public.verificacion
SET estado = sqlc.arg(state), revisor_id = sqlc.arg(reviewer_id)::text::uuid,
    motivo_codigo = NULLIF(sqlc.arg(reason_code)::text, ''), resuelta_en = now()
WHERE id = sqlc.arg(id) AND estado = 'en_revision'
RETURNING id::text AS id, usuario_id::text AS owner_id, tipo AS type,
    estado AS state, proveedor_ref AS provider, referencia_evidencia AS evidence_ref,
    COALESCE(reintento_de::text, ''::text) AS retry_of,
    COALESCE(motivo_codigo, '') AS reason_code,
    creada_en AS created_at, resuelta_en AS resolved_at,
    clave_idempotencia AS idempotency_key;

-- name: LockPriorVerificationForRetry :one
SELECT tipo AS type, estado AS state
FROM public.verificacion
WHERE id = sqlc.arg(id) AND usuario_id = sqlc.arg(owner_id)
FOR UPDATE;

-- name: CreateVerificationRetry :one
INSERT INTO public.verificacion (
    id, usuario_id, tipo, estado, proveedor_ref, referencia_evidencia,
    clave_idempotencia, reintento_de, creada_en
) VALUES (
    sqlc.arg(id), sqlc.arg(owner_id), sqlc.arg(type), 'en_revision',
    'local-fixture-v1', sqlc.arg(evidence_ref), sqlc.arg(idempotency_key),
    sqlc.arg(retry_of)::text::uuid, sqlc.arg(created_at)
)
RETURNING id::text AS id, usuario_id::text AS owner_id, tipo AS type,
    estado AS state, proveedor_ref AS provider, referencia_evidencia AS evidence_ref,
    COALESCE(reintento_de::text, ''::text) AS retry_of,
    COALESCE(motivo_codigo, '') AS reason_code,
    creada_en AS created_at, resuelta_en AS resolved_at,
    clave_idempotencia AS idempotency_key;
