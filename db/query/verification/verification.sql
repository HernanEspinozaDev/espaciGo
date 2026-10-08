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
    clave_idempotencia AS idempotency_key, COALESCE(correccion_codigo,'') AS correction_code,
    revocada_en AS revoked_at, COALESCE(motivo_revocacion_codigo,'') AS revocation_reason;

-- name: GetVerificationByIdempotency :one
SELECT id::text AS id, usuario_id::text AS owner_id, tipo AS type,
    estado AS state, proveedor_ref AS provider, referencia_evidencia AS evidence_ref,
    COALESCE(reintento_de::text, ''::text) AS retry_of,
    COALESCE(motivo_codigo, '') AS reason_code,
    creada_en AS created_at, resuelta_en AS resolved_at,
    clave_idempotencia AS idempotency_key, COALESCE(correccion_codigo,'') AS correction_code,
    revocada_en AS revoked_at, COALESCE(motivo_revocacion_codigo,'') AS revocation_reason
FROM public.verificacion
WHERE usuario_id = sqlc.arg(owner_id) AND clave_idempotencia = sqlc.arg(idempotency_key)
LIMIT 1;

-- name: GetOwnVerification :one
SELECT id::text AS id, usuario_id::text AS owner_id, tipo AS type,
    estado AS state, proveedor_ref AS provider, referencia_evidencia AS evidence_ref,
    COALESCE(reintento_de::text, ''::text) AS retry_of,
    COALESCE(motivo_codigo, '') AS reason_code,
    creada_en AS created_at, resuelta_en AS resolved_at,
    clave_idempotencia AS idempotency_key, COALESCE(correccion_codigo,'') AS correction_code,
    revocada_en AS revoked_at, COALESCE(motivo_revocacion_codigo,'') AS revocation_reason
FROM public.verificacion
WHERE usuario_id = sqlc.arg(owner_id) AND id = sqlc.arg(id)
LIMIT 1;

-- name: ListOwnVerifications :many
SELECT id::text AS id, usuario_id::text AS owner_id, tipo AS type,
    estado AS state, proveedor_ref AS provider, referencia_evidencia AS evidence_ref,
    COALESCE(reintento_de::text, ''::text) AS retry_of,
    COALESCE(motivo_codigo, '') AS reason_code,
    creada_en AS created_at, resuelta_en AS resolved_at,
    clave_idempotencia AS idempotency_key, COALESCE(correccion_codigo,'') AS correction_code,
    revocada_en AS revoked_at, COALESCE(motivo_revocacion_codigo,'') AS revocation_reason
FROM public.verificacion
WHERE usuario_id = sqlc.arg(owner_id)
ORDER BY creada_en DESC;

-- name: ListPendingVerifications :many
SELECT id::text AS id, usuario_id::text AS owner_id, tipo AS type,
    estado AS state, proveedor_ref AS provider, referencia_evidencia AS evidence_ref,
    COALESCE(reintento_de::text, ''::text) AS retry_of,
    COALESCE(motivo_codigo, '') AS reason_code,
    creada_en AS created_at, resuelta_en AS resolved_at,
    clave_idempotencia AS idempotency_key, COALESCE(correccion_codigo,'') AS correction_code,
    revocada_en AS revoked_at, COALESCE(motivo_revocacion_codigo,'') AS revocation_reason
FROM public.verificacion
WHERE estado = 'en_revision'
ORDER BY creada_en;

-- name: ListRejectedVerifications :many
SELECT id::text AS id, usuario_id::text AS owner_id, tipo AS type,
    estado AS state, proveedor_ref AS provider, referencia_evidencia AS evidence_ref,
    COALESCE(reintento_de::text, ''::text) AS retry_of,
    COALESCE(motivo_codigo, '') AS reason_code,
    creada_en AS created_at, resuelta_en AS resolved_at,
    clave_idempotencia AS idempotency_key, COALESCE(correccion_codigo,'') AS correction_code,
    revocada_en AS revoked_at, COALESCE(motivo_revocacion_codigo,'') AS revocation_reason
FROM public.verificacion
WHERE estado = 'rechazada'
ORDER BY resuelta_en DESC, id;

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
    clave_idempotencia AS idempotency_key, COALESCE(correccion_codigo,'') AS correction_code,
    revocada_en AS revoked_at, COALESCE(motivo_revocacion_codigo,'') AS revocation_reason;

-- name: LockPriorVerificationForRetry :one
SELECT tipo AS type, estado AS state, COALESCE(motivo_codigo, '') AS reason_code
FROM public.verificacion
WHERE id = sqlc.arg(id) AND usuario_id = sqlc.arg(owner_id)
FOR UPDATE;

-- name: CreateVerificationRetry :one
INSERT INTO public.verificacion (
    id, usuario_id, tipo, estado, proveedor_ref, referencia_evidencia,
    clave_idempotencia, reintento_de, creada_en, correccion_codigo
) VALUES (
    sqlc.arg(id), sqlc.arg(owner_id), sqlc.arg(type), 'en_revision',
    'local-fixture-v1', sqlc.arg(evidence_ref), sqlc.arg(idempotency_key),
    sqlc.arg(retry_of)::text::uuid, sqlc.arg(created_at), sqlc.arg(correction_code)
)
RETURNING id::text AS id, usuario_id::text AS owner_id, tipo AS type,
    estado AS state, proveedor_ref AS provider, referencia_evidencia AS evidence_ref,
    COALESCE(reintento_de::text, ''::text) AS retry_of,
    COALESCE(motivo_codigo, '') AS reason_code,
    creada_en AS created_at, resuelta_en AS resolved_at,
    clave_idempotencia AS idempotency_key, correccion_codigo AS correction_code,
    revocada_en AS revoked_at, COALESCE(motivo_revocacion_codigo,'') AS revocation_reason;

-- name: ListVerificationHistory :many
SELECT history.id AS sequence, history.accion AS action,
       COALESCE(history.estado_anterior,'') AS from_state,
       history.estado_nuevo AS to_state, COALESCE(history.motivo_codigo,'') AS reason_code,
       history.ocurrida_en AS occurred_at
FROM public.verificacion_historial_local history
JOIN public.verificacion v ON v.id=history.verificacion_id
WHERE v.usuario_id=sqlc.arg(owner_id) AND v.id=sqlc.arg(id)
ORDER BY history.id;

-- name: ListSyntheticEligibility :many
SELECT kinds.tipo AS type,
       COALESCE(e.estado='elegible' AND verification.estado='aprobada' AND owner.estado='activo',false) AS eligible,
       COALESCE(e.verificacion_id::text,'') AS verification_id,
       e.concedida_en AS granted_at, e.revocada_en AS revoked_at,
       COALESCE(e.motivo_revocacion_codigo,'') AS revocation_reason
FROM unnest(ARRAY['kyc','kyb']::text[]) AS kinds(tipo)
LEFT JOIN public.elegibilidad_verificacion_local e
  ON e.usuario_id=sqlc.arg(owner_id) AND e.tipo=kinds.tipo
LEFT JOIN public.verificacion verification
  ON verification.id=e.verificacion_id AND verification.usuario_id=e.usuario_id AND verification.tipo=e.tipo
LEFT JOIN public.usuario owner ON owner.id=e.usuario_id
ORDER BY kinds.tipo;

-- name: LockVerificationForRevoke :one
SELECT usuario_id::text AS owner_id, tipo AS type, estado AS state,
       clave_idempotencia AS idempotency_key
FROM public.verificacion WHERE id=sqlc.arg(id) FOR UPDATE;
