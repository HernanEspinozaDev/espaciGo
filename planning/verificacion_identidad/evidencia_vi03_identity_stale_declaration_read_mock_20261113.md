# Evidencia VI-03 — lecturas reviewer y declaración vigente

**Fecha:** 2026-11-13
**Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`
**Rama / HEAD observado:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`

## Cambio

`reviewerCaseDeclarationCurrent` comprueba la revisión legal más reciente para casos identity accionables (`enviada`, `en_revision`, `requiere_antecedentes`). `LoadReviewerCase`, `ListReviewerCases` y `WithReviewerCaseForRead` hacen la comprobación después de validar el grant exacto y antes de cargar metadata de archivos. Una revisión ligada obsoleta se omite de la cola o se responde como `ErrNotFound` en lecturas individuales/scoped; el error del lookup se propaga y cierra el acceso. El guard no se aplica a estados terminales: el detalle scoped conserva su lectura histórica. La cola, por contrato de `ReviewerAccessService.List`, solo presenta casos accionables.

`TestStaleIdentityDeclarationIsHiddenFromReviewerReadPaths` usa `pgx.Tx` y filas sintéticas. Sus ocho escenarios cubren stale/current en cola, lectura individual y scoped; conservación de historial terminal; y fallo del lookup sin callback ni lectura de archivos. Comprueba argumentos sujeto/revisión, metadata de archivo, commit fake y ausencia de escrituras. No se afirma RED/GREEN: las corridas iniciales no distinguieron con salida comprobable un fallo de comportamiento de un fixture incompleto.

## Verificación local

- Focal: `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=1 ./internal/adapters/postgres/documentverification -run '^TestStaleIdentityDeclarationIsHiddenFromReviewerReadPaths$' -v` — PASS, 8 escenarios.
- Repetición focal: `go test -race -count=20 ./internal/adapters/postgres/documentverification -run '^TestStaleIdentityDeclarationIsHiddenFromReviewerReadPaths$'` — PASS, en entorno `env -i`.
- Suites `-race`: `./internal/adapters/postgres/documentverification ./internal/documentverification ./internal/documentverification/transport/http` — PASS, en entorno `env -i`.
- `go vet` de los tres paquetes, `go build -o /dev/null ./cmd/api`, `gofmt` de los cuatro archivos del subcorte y `git diff --check` — PASS.

## Límites

Los `pgx.Tx`/rows son fakes: no ejecutan ni validan SQL, PostgreSQL, grants persistidos, semántica de aislamiento/locks, visibilidad concurrente de una nueva declaración, almacenamiento o reinicio. El filtro solo observa la revisión visible en la transacción; no coordina el guardado concurrente de una declaración con lecturas, decisiones o envíos. No acredita autorización efectiva, MFA, storage/GCS, Cloud, persistencia ni aceptación de producto.

## Revisión independiente

`deleg_e0f166ea` señaló inicialmente que el filtro SQL de estados activos en `ListReviewerCases` podía ocultar casos terminales. La revisión source-only posterior `deleg_3083364f` adjudicó que no es una regresión del subcorte: la colección es una cola de casos accionables (`ReviewerAccessService.List` filtra esos estados y el GET de colección es distinto del detalle GET por caso); el detalle terminal usa `WithReviewerCaseForRead`, donde la nueva guarda omite la consulta de frescura para estados terminales. El filtro SQL activo ya existía en el worktree compartido antes de este subcorte y concuerda con la cola activa; se preservó. La segunda revisión no ejecutó pruebas ni operaciones externas.

No se ejecutó SQL/DDL, migración, consulta ni acceso a DB persistente o Cloud. El worktree compartido contenía cambios ajenos; se preservaron. Sin commit, push, deploy o integración a `main`.
