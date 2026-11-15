# Evidencia VI-03 — declaración vigente antes de enviar identity

**Fecha:** 2026-11-13
**Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`
**Rama / HEAD:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`

## Cambio

Se amplió la comprobación de frescura en `Repository.Transition` para que también cubra el envío owner: propósito `identity`, acción `case_transitioned` y resultado `CaseSubmitted`. Si el caso conserva una declaración anterior a la revisión más reciente visible para el titular, el adapter devuelve `dv.ErrConflict` antes de cargar metadata de archivos o invocar la transición. El guard de decisión reviewer permanece igual; las demás transiciones no activan la consulta.

`TestIdentitySubmissionRejectsStaleDeclarationBeforeReadingFiles` usa un `pgx.Tx` fake con un control de declaración vigente y un caso cuya declaración ligada es anterior. Se observó RED: sin ampliar el guard, el caso stale alcanzó el callback y no se consultó la revisión actual. Tras el cambio, GREEN confirma `ErrConflict`, cero callback y cero lectura de archivos para el caso stale; el control vigente sí consulta, lee metadata y alcanza el callback. Ambos escenarios verifican cero escrituras/commit y rollback fake.

## Verificación

- Focales de frescura reviewer + submit: `go test -race -count=20 ./internal/adapters/postgres/documentverification -run '^(TestIdentityReviewRejectsCaseLinkedToOlderDeclarationRevision|TestIdentitySubmissionRejectsStaleDeclarationBeforeReadingFiles)$'` — PASS.
- Suites `-race`: `./internal/adapters/postgres/documentverification ./internal/documentverification ./internal/documentverification/transport/http` — PASS.
- `go vet` de esos tres paquetes y `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt` y `git diff --check` — PASS.

Los comandos de prueba se ejecutaron en entorno mínimo (`env -i`), sin variables de conexión/opt-in. **No se ejecutó SQL/DDL, consulta, migración ni acceso a DB persistente o Cloud.**

## Límites

El test ejecuta la ruta de transición del adapter con `pgx.Tx` fake, no llama a PostgreSQL. No demuestra que el SQL sea válido en runtime, semántica de orden/aislamiento SERIALIZABLE/SSI, locks, persistencia, atomicidad ni la carrera entre guardar una declaración nueva y enviar una solicitud concurrentemente. Solo caracteriza el rechazo cuando el lookup sintético observa una revisión más reciente. No acredita autenticación/grants persistidos, reinicio, storage/GCS o Cloud.

El worktree compartido contiene cambios ajenos a este subcorte; se preservaron. Sin commit, push, deploy o integración a `main`.

## Revisión independiente

La revisión source-only `deleg_7a9287ee` aprobó el delta (`passed=true`, sin hallazgos de seguridad ni errores lógicos). Confirmó que la pareja action/result coincide con `Service.Submit` y que otras transiciones quedan fuera. Reiteró los límites de fake: no demuestra semántica PostgreSQL, persistencia ni exclusión de un guardado concurrente. El revisor no ejecutó pruebas ni operaciones externas.
