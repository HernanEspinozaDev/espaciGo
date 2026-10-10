# LOCAL-ADMIN-01B — consulta y exportación de auditoría local

Issue de subentrega: [#228](https://github.com/HernanEspinozaDev/espaciGo/issues/228). PR de implementación: [#229](https://github.com/HernanEspinozaDev/espaciGo/pull/229), commit inicial `76621b1` (rama `codex/local-admin-01b`), abierto para revisión de HernanEspinozaDev.

## Alcance y decisiones conservadas

Este corte reutiliza `public.evento_auditoria_local` y su rol de ejecución existente. No añade una migración, no cambia la retención de cinco años, no concede permisos `UPDATE`/`DELETE` y no amplía los productores de auditoría. #111–#118, #185/#40 y LOCAL-CORE-02 permanecen abiertos por sus criterios generales.

- Consultas exclusivas de administrador activo, autorizadas por el Backend y el autenticador común.
- `from` inclusivo y `until` exclusivo, RFC3339 aceptando offset y normalizando a UTC; intervalo obligatorio de hasta 31 días.
- Filtros exactos opcionales por actor, tipo/ID de recurso, acción y resultado; sin búsqueda de texto libre.
- Lista ordenada por `(ocurrido_en DESC, id DESC)`, tamaño 25 por defecto / 100 máximo, sin conteo ni `OFFSET`. El cursor URL-safe incluye versión, cuenta, hash de filtros, tamaño y tupla de corte/continuación.
- Export JSON v1, campos `id`, `occurred_at`, `actor_id|null`, `resource_type`, `resource_id`, `action`, `result`, `reason_code`, `correlation_id`; incluye filtros y `generated_at`. Más de 10.000 filas produce 422 y no entrega una exportación parcial.
- Lista y exportación capturan el timestamp de corte dentro de una transacción PostgreSQL antes de insertar el evento de acceso; solo recorren eventos anteriores al corte. Así las lecturas administrativas generadas por las páginas posteriores tampoco entran en el recorrido iniciado. Se registra un evento por petición administrativa autenticada, no por fila; filtros inválidos y rol denegado con sesión activa se registran con motivos estructurados. Una petición sin identidad autenticable no se registra; ante una caída de almacenamiento no es posible garantizar que el propio intento se persista. Los cursores son AEAD opacos URL-safe, ligados a cuenta/filtros/tamaño y a la instancia del Backend; un reinicio invalida cursores pendientes y se reinicia la consulta.

## Contraste de productores con #111–#114

La inspección de `INSERT INTO evento_auditoria_local` y los métodos que centralizan esa escritura encontró productores específicos, no una capa transversal:

| Acción registrada actualmente | Productor/archivo | Límite observado |
|---|---|---|
| `identidad.credencial_cambiar` | sqlc `RecordCredentialChangeAudit`, `db/query/identity/identity.sql` | Cambio de credencial; no registra todas las operaciones de identidad. |
| `identity.credential_notice.reopen` | sqlc `RecordCredentialNoticeRecoveryAudit`, mismo archivo | Reapertura administrativa de un evento terminal del outbox de credenciales; no es outbox general. |
| `privacy.suppression.review`, `privacy.suppression.execute` | `internal/adapters/postgres/identity/suppression.go` | Evaluación y ejecución local de baja. |
| `kyc.synthetic.review`, `kyc.eligibility.revoke` | `internal/adapters/postgres/verification/repository.go` vía `recordVerificationAudit` | Revisión sintética y revocación de elegibilidad. |
| `booking.admin.reservations.list`, `booking.admin.reservations.read` | `internal/adapters/postgres/booking/admin_read.go` | Lecturas de consulta administrativa; la colección ahora usa `coleccion_reservas_ensayo_local` y UUID cero estable, no el ID del administrador como reserva. Registros históricos no se reescriben. |
| `reputation.review.moderate` | `internal/adapters/postgres/reputation/repository.go` | Moderación local de un reporte. |
| `local.damage_claim.resolve` | `internal/adapters/postgres/damageclaim/repository.go` | Resolución histórica local, separada de efectos financieros. |
| `admin.audit.events.list`, `admin.audit.events.export` | Nuevo corte en `internal/adapters/postgres/booking/admin_audit.go` | Consulta/exportación; una fila por petición, incluyendo filtros inválidos autenticados y rol denegado con sesión activa. |

Reservas/transiciones, contrato/firma, operación de arriendo, conversación, inbox fake de pagos y avisos/outboxes conservan historiales o registros propios. No todos escriben un evento central. Persisten brechas de cobertura transversal, contrato de auditoría/outbox general y continuidad definidas por #111–#114; este corte no las presenta como resueltas.

## Implementación

- Rutas `GET /api/v1/admin/local/audit-events` y `/export`, registradas sobre el handler que aplica CORS, request ID, no-store, autenticación activa y rol administrador. Errores usan `ErrorEnvelope`; 401 usa `WWW-Authenticate: Bearer`.
- SQL lee exclusivamente los nueve campos aprobados; no selecciona `detalle_codigos`, payloads, texto libre ni expiración. Se conserva compatibilidad con `actor_id` retirado y se serializa como `null`.
- No se usa tabla nueva ni se muta/elimina el ledger. La lectura no inicia conciliaciones ni vencimientos.
- El mock muestra filtros, páginas, descarga JSON y aviso de alcance. Cambiar filtros o sesión limpia la vista y hace obsoletos cursores/respuestas; la descarga revalida cuenta, token y generación después de recibir el Blob.
- OpenAPI actualizado en `planning/openapi.yaml`.

## Validación

Validación completada en la rama del PR. El harness PostgreSQL creó y retiró una base desechable migrada desde cero; no apuntó a `espacigo_pgdata`.

```sh
go test ./internal/booking/transport/http ./internal/adapters/postgres/booking ./cmd/api
go vet ./internal/booking/transport/http ./internal/adapters/postgres/booking ./cmd/api
GO_TEST_RUN='^TestLocalBookingTrialPostgresLifecycleAndConcurrentRetry$' bash scripts/test-m06-local-booking-postgres.sh ./internal/adapters/postgres/booking
cd mock && npm run build && node --test test/admin-audit-state.test.mjs
git diff --check
```

- `go test ./internal/booking/transport/http ./internal/adapters/postgres/booking ./cmd/api`: pasó.
- Integración PostgreSQL desechable `TestLocalBookingTrialPostgresLifecycleAndConcurrentRetry`: pasó. Cubre filtros (incluidos actor/recurso/resultado), periodo, páginas empatadas con UUID como desempate, cursor ligado, exclusión del evento de lectura actual, exportación exacta/minimizada, límite 10.000 con auditoría `rechazo`, rol no autorizado y filtros inválidos registrados; mantiene la comprobación previa de que leer reservas no modifica sus datos de negocio.
- `cd mock && npm run build && node --test test/admin-audit-state.test.mjs`: build pasó; 6/6 pruebas pasaron (cambio de criterios, logout/relogin, exportación tardía, 401 vigente con limpieza común, 401 obsoleto tras relogin y 403 con limpieza administrativa conservando sesión).
- Parseo YAML de `planning/openapi.yaml`: pasó como OpenAPI 3.1.0.
- `scripts/dev-env.sh up backend mock-frontend`: reconstruyó API/mock y esperó salud; base/volumen y secretos se conservaron, sin migración nueva. `scripts/dev-env.sh verify-http`: todas las comprobaciones HTTP/CORS/readiness/mock pasaron.
- Recorrido HTTP local con la cuenta administradora sintética ya guardada fuera del repo: login → consulta de 98 eventos en 21 páginas de 5 → exportación JSON v1 de 103 eventos. Se comprobaron los nueve campos permitidos, el encabezado de descarga y la exclusión del evento de la exportación actual. El recorrido registró las lecturas en auditoría; no alteró estados/importes de negocio. La diferencia de filas refleja eventos de acceso generados entre el listado paginado y la exportación posterior.
- `go vet ./internal/booking/transport/http ./internal/adapters/postgres/booking ./cmd/api`: pasó.
- `git diff --check`: pasó.

## Pendientes

- El mock permite filtros, páginas y descarga; su estado asíncrono tiene pruebas enfocadas. El recorrido con administrador se ejecutó contra las APIs reales desde el cliente local; la automatización no valida una descarga visual del archivo en el navegador.
- No hay exportación de toda la actividad de cuenta ni reconstrucción de productores que no escriben al ledger.
- #111–#118 continúan abiertos; no se cierra M11 ni LOCAL-CORE-02. Sin proveedor real, GCP o inmutabilidad productiva.
