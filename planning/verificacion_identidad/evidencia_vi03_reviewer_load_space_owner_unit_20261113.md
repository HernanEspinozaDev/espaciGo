# VI-03 — reviewer: owner y grant space en carga de caso

Fecha: 2026-11-13 (America/Santiago). Checkout: `/home/nandev/Documentos/projects/espaciGo-main-sync`; rama `codex/marketplace/fix/kyc-06-save-identity-first`; HEAD `274f37e483a078890e92382a159fa49fd23c2fe8`.

## Alcance

Se añadió `TestLoadReviewerCaseRevalidatesSpaceOwnerAndGrantBeforeReadingFiles` en `internal/adapters/postgres/documentverification/reviewer_case_adapter_read_test.go`. Ejercita `Repository.LoadReviewerCase` —reader usado por `ReviewerAccessService.Load`— con `pgx.Tx` fake y caso/grant space sintéticos:

- Owner exacto y grant del espacio exacto: devuelve el snapshot space, carga metadata de archivos y completa el commit fake.
- Owner discrepante: `ErrNotFound` antes del reread del snapshot, grant o archivos.
- Error sintético al adquirir el owner lock: propaga el error sin releer snapshot, grant ni archivos.
- Owner exacto con grant de otro espacio: `ErrNotFound`, sin lectura de archivos.
- `TestLoadReviewerCaseRejectsSelfReviewBeforeGrantOrFileReads` presenta un caso identity cuyo titular y reviewer coinciden junto con un grant identity sintético exacto; retorna `ErrNotFound` antes de consultar grants, frescura o archivos.

Los escenarios verifican los argumentos `space_id`/`subject_id`, conteos de lecturas, ausencia de writes fake y la invocación del rollback diferido. El helper conserva los fixtures identity previos y también soporta el test de `WithReviewerCaseForRead`.

## Verificación ejecutada

- Selección focal con `go test -race -count=20` para las dos pruebas de `LoadReviewerCase`, ambas pruebas de `WithReviewerCaseForRead`, declaración identity obsoleta y revalidación de grant — PASS.
- Cobertura de `LoadReviewerCase`: 71,4% en esa selección; el perfil confirma que el corte ejercita el error del owner lock, la rama space y las denegaciones por owner/grant, además del rechazo de auto-revisión antes de grants/archivos.
- `go vet ./internal/adapters/postgres/documentverification`, `gofmt -d` y `git diff --check` — PASS.
- Revisión source-only independiente `deleg_94e52a7e`: PASS, sin hallazgos de seguridad, errores lógicos ni sugerencias; no ejecutó pruebas.
- Caracterización test-only, sin cambio productivo ni afirmación RED/GREEN. El primer intento encontró una referencia a un contador ausente en el helper; se corrigió el test harness y las ejecuciones focales posteriores pasaron.

## Límites

Owner, grants, consultas y transacción son sintéticos. No se ejecutó la función SQL de ownership ni se demuestra lock efectivo, grants PostgreSQL, aislamiento, concurrencia, rollback durable o persistencia. Sin SQL/DDL, DB/Cloud, datos reales, commit, push, deploy o integración a `main`.
