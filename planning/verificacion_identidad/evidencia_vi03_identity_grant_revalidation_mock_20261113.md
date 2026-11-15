# Evidencia VI-03: revalidación del grant al decidir identidad

**Fecha:** 2026-11-13
**Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`
**Rama / HEAD base:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`
**Alcance:** unit tests, handler Go y `pgx.Tx` fakes sintéticos. Sin SQL/DDL ejecutado ni conexión a PostgreSQL persistente o Cloud.

## Cambio

`Repository.Transition` vuelve a leer los grants activos dentro de la transacción de mutación y verifica el actor/capability/scope exactos antes de cargar metadata de archivos o consultar el estado de identidad del titular. Si el actor no es el titular ni tiene el grant exacto vigente, devuelve `ErrNotFound`; el handler lo mantiene como 404 neutral. La comprobación domain existente permanece como defensa adicional. Las transiciones owner (`actor == subject`) no requieren grant reviewer.

`TestIdentityReviewerGrantRevokedAfterPreflightStopsBeforeFileRead` recorre la ruta HTTP Go registrada, `ReviewerAccessService`, `Service` y el `Repository` adapter reales. El primer `pgx.Tx` fake proporciona el grant identity exacto y confirma el preflight; el segundo devuelve cero grants para representar su revocación antes de la mutación. Verifica 404 sin DTO/ETag, una consulta nueva de grants, cero consultas de metadata de archivos en la transacción de mutación, cero escrituras y rollback observado por el fake. `reviewer_transition_scope_test.go` ahora da a su control positivo de space el grant exacto requerido por la nueva guarda.

## TDD y gates

- RED observado antes de cambiar producción: el test llegó a la segunda transacción con preflight/grant exactos y falló en la aserción objetivo: `mutation read 1 file metadata queries after the reviewer grant disappeared`.
- GREEN focal: `go test ./internal/adapters/postgres/documentverification -run '^TestIdentityReviewerGrantRevokedAfterPreflightStopsBeforeFileRead$' -count=1 -v` — PASS.
- Suite focal bajo `-race` del adapter: nueva regresión, auditoría typed identity positiva, scope/grant exacto, lista, transición de espacio, y lectura de aprobación actual frente a declaración antigua — PASS.
- `internal/documentverification` (`self-review`, asignación exacta y concurrencia sintética) con `-race` — PASS.
- Ruta HTTP de concurrencia con grants/repositorio fake: `-race -count=20` — PASS.
- `go vet ./internal/adapters/postgres/documentverification ./internal/documentverification ./internal/documentverification/transport/http` — PASS.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go build -o /dev/null ./cmd/api` — PASS.
- `gofmt` aplicado a los archivos Go del corte.

Los tests adapter usan transacciones y filas fakes; no prueban la semántica SQL, `FOR SHARE`, revocación concurrente real, locks, rollback durable, autorización efectiva ni atomicidad PostgreSQL. Tampoco prueban storage/GCS, MFA o Cloud. El worktree compartido ya contenía numerosos cambios ajenos a este subcorte; no se limpiaron, reemplazaron ni atribuyen a este cambio. La revisión independiente source-only `deleg_ad6bcafc` aprobó (`passed=true`, sin concerns de seguridad ni errores lógicos); recomendó conservar explícito el límite fake-only, ya documentado aquí.

No se ejecutaron SQL/DDL, tests conectados, migraciones, acceso a bases persistentes/Cloud, operaciones externas, commit, push, deploy ni integración a `main`. VI-03 y el MVP permanecen parciales.
