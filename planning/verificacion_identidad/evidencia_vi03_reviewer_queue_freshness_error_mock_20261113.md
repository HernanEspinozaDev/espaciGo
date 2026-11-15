# Evidencia VI-03 — cola cerrada ante error de frescura

**Fecha:** 2026-11-13
**Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`
**Rama / HEAD observado:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`

## Caracterización

Se añadió `TestReviewerQueueFailsClosedWhenDeclarationFreshnessLookupErrors` sobre `Repository.ListReviewerCases` real con un `pgx.Tx` sintético. Tras entregar un candidato y un grant exacto sintéticos, el fake inyecta error en la consulta de revisión vigente. La lista propaga el error original y devuelve `nil`, no consulta metadata de archivos, no confirma ni ejecuta escrituras y llama al rollback fake una vez. El test pasó en su primera ejecución válida: caracteriza un guard ya presente, no se afirma RED/GREEN ni cambio productivo.

## Verificación local

- Focal `-race -count=1`: PASS.
- Focal `-race -count=20`: PASS bajo `env -i`.
- Suites `-race`: `./internal/adapters/postgres/documentverification ./internal/documentverification ./internal/documentverification/transport/http` — PASS bajo `env -i`.
- `go vet` de los tres paquetes, `go build -o /dev/null ./cmd/api`, `gofmt` y `git diff --check` — PASS.

## Límites

El candidato, grant, lookup y transacción son fakes. El test no ejecuta ni valida SQL, PostgreSQL, semántica de rollback, grants persistidos, concurrencia, autorización efectiva o runtime. Comprueba el control flow del adapter ante un error sintético, no la persistencia.

## Revisión independiente

`deleg_ee5c6cb0` realizó revisión source-only con `passed=true`, sin hallazgos de seguridad/lógica; no ejecutó pruebas. Confirmó que el adapter devuelve nil/error en la falla del lookup antes de cargar archivos o hacer commit y que el rollback contado es fake. Sugirió considerar, en otro corte, un error después de haber acumulado candidatos válidos: este test de un candidato no lo cubre. La observación queda como límite; no amplía los claims de este subcorte.

No se ejecutó SQL/DDL, migración, consulta ni acceso a DB persistente o Cloud. Worktree compartido y cambios ajenos preservados. Sin commit, push, deploy o integración a `main`.
