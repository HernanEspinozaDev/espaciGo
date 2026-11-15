# Evidencia VI-03 — descarga bloqueada ante declaración obsoleta

**Fecha:** 2026-11-13
**Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`
**Rama / HEAD observado:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`

## Caracterización

Se añadió `TestReviewerDownloadRejectsStaleDeclarationBeforeFileOrStoreRead`, que compone el `Repository` PostgreSQL adapter real con `ReviewerAccessService.Download` y dependencias sintéticas. En el caso stale, el lookup fake detecta una revisión más reciente y la descarga responde `ErrNotFound`, sin entregar contenido/MIME, consultar metadata de archivos ni llamar al store. El control vigente descarga bytes/MIME sintéticos limpios, y afirma el scope exacto sujeto/caso/archivo, una consulta de metadata, una lectura identity y commit fake. No hubo cambio de producción ni RED/GREEN: la primera ejecución fue caracterizadora y pasó.

## Verificación local

- Focal: `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=1 ./internal/adapters/postgres/documentverification -run '^TestReviewerDownloadRejectsStaleDeclarationBeforeFileOrStoreRead$' -v` — PASS, dos escenarios.
- Repetición focal: `go test -race -count=20 ./internal/adapters/postgres/documentverification -run '^TestReviewerDownloadRejectsStaleDeclarationBeforeFileOrStoreRead$'` — PASS bajo `env -i`.
- Suites `-race`: `./internal/adapters/postgres/documentverification ./internal/documentverification ./internal/documentverification/transport/http` — PASS bajo `env -i`.
- `go vet` de los tres paquetes, `go build -o /dev/null ./cmd/api`, `gofmt` y `git diff --check` — PASS.

## Límites

El `pgx.Tx`, las filas, el grant y la revisión vigente son sintéticos; no se ejecuta ni valida SQL o PostgreSQL. Esto no prueba visibilidad transaccional/concurrente de nuevas declaraciones, grants persistidos, autenticación real, el handler HTTP, almacenamiento real, reinicio, GCS ni Cloud. La lectura correcta del store en el control positivo solo prueba la composición en memoria.

## Revisión independiente

`deleg_ccd7e389` realizó revisión source-only sin ejecutar pruebas ni modificar archivos: `findings=[]`. Confirmó que stale devuelve `ErrNotFound` neutral antes de consultar metadata o storage; el control vigente llega a metadata y al store con scope exacto, y los bytes/MIME/SHA/tamaño sintéticos son coherentes. La revisión confirma únicamente la composición descrita; no acredita DB, SQL ni storage real.

No se ejecutó SQL/DDL, migración, consulta ni acceso a DB persistente o Cloud. El worktree compartido sigue sucio; se preservaron los cambios ajenos. Sin commit, push, deploy o integración a `main`.
