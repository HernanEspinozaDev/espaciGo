# VI-03 — fecha civil de vencimiento observada

- Fecha de verificación local: 2026-11-12.
- Checkout: `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- Rama/HEAD comprobados: `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- Estado: cambios locales sin commit; el worktree compartido ya contenía muchos cambios ajenos a este subcorte, preservados.

## Alcance y cambios

- El contrato HTTP de `document_expires_at_observed` es una fecha civil `YYYY-MM-DD` (`format: date`) en request y DTO de revisor. El parser rechaza timestamps y fechas civiles malformadas.
- La fecha es válida durante todo el día indicado en `America/Santiago`. Go calcula el límite exclusivo con el primer instante de la fecha civil siguiente mediante `calendar.FirstInstantOfDate`; las pruebas incluyen una transición DST con hora local inexistente. Se incluye `time/tzdata` para que la zona esté disponible en entornos mínimos.
- El DTO vuelve a emitir la fecha civil cuando el instante interno representa exactamente el inicio local del día siguiente. La conversión interna conserva el esquema/tipo de instante usado por el servicio y el evento de revisión; no se modificó ni aplicó V96.
- OpenAPI y el plan registran la decisión ratificada. La generación a un archivo temporal produjo `Format: date` en las dos apariciones correspondientes.

## Verificación ejecutada

- RED/GREEN previo del handler: `TestTypedIdentityReviewUsesSantiagoCivilExpiryDate` falló antes de incorporar tzdata local y pasó tras la corrección; la prueba confirma el comportamiento de fecha civil/DST.
- `env -u DATABASE_URL -u DATABASE_DSN -u DB_DSN -u TEST_DATABASE_URL -u PGHOST -u PGPORT -u PGDATABASE -u PGUSER -u PGPASSFILE go test -race ./internal/documentverification ./internal/documentverification/transport/http ./internal/documentverification/inspection -count=1` — PASS (tres paquetes).
- El mismo aislamiento de variables con `go vet` en los tres paquetes — PASS.
- El mismo aislamiento con `go build -o /dev/null ./cmd/api` — PASS.
- `node --test tooling/document-verification-openapi.test.mjs` — 2/2 PASS. Es cobertura del harness OpenAPI existente, no prueba de integración UI.
- `node tooling/api-generate.mjs --output "$TMPDIR/openapi-date-draft.d.ts"` — PASS; lectura del artefacto temporal confirmó `Format: date` en request y DTO.
- `node tooling/api-check.mjs` — FAIL: compara el archivo generado completo y detecta drift; el artefacto versionado todavía declara `date-time` para este campo y presenta otras diferencias. No se editaron archivos generados: `frontend/AGENTS.md` reserva `frontend/` al worktree/branch frontend autorizados.
- `git diff --check` en los archivos de este subcorte — PASS.

## Límites y siguiente gate

- La UI/adaptador y el cliente OpenAPI versionado no se actualizaron. Antes de integrar este contrato hay que alinear/regenerar el cliente y enviar `YYYY-MM-DD` desde el formulario, en el worktree frontend autorizado, y volver a ejecutar su gate.
- No se ejecutó SQL/DDL, migración, PostgreSQL, Cloud, proveedor, datos reales, despliegue, publicación ni integración a `main`.
- Revisión independiente read-only: sin hallazgos concretos de lógica o seguridad en el slice de fecha civil. El reporte mencionó la llamada `Complete()` con checklist posiblemente nulo, pero también constató que `Complete` devuelve `false` si el receptor es nil; verificado en `internal/documentverification/service.go`, no es un defecto ni produce panic. La revisión confirmó que la regeneración del cliente/UI y la evidencia de persistencia permanecen fuera de alcance y pendientes.
