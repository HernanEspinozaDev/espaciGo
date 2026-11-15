# VI-03 — cancelación tras el callback de lectura

- **Fecha:** 2026-11-12.
- **Checkout/rama/HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- Worktree compartido ampliamente dirty; se preservaron cambios preexistentes.

## Hallazgo y corrección

`TestReviewerDownloadFailsClosedWhenContextCancelsAfterReaderReturns` usa un expediente y grant identity exactos con bytes sintéticos limpios y un Store fake que devuelve contenido íntegro. El repositorio fake ejecuta correctamente el callback de lectura y cancela el contexto inmediatamente después, antes de devolver éxito al servicio. RED observado: `ReviewerAccessService.Download` devolvía bytes y MIME con error nil, aunque el callback había terminado y el contexto ya estaba cancelado.

`Download` ahora comprueba `ctx.Err()` después de que `WithReviewerCaseForRead` retorna con éxito. Ante cancelación devuelve el error neutral `ErrDocumentStorage` y un resultado vacío. El test incluye un control positivo sin cancelación que confirma que los mismos datos sintéticos sí son descargables.

## Verificación

Con variables PostgreSQL/opt-in retiradas:

- Focal antes del cambio — **RED**, bytes y MIME entregados tras cancelación.
- Focal después del cambio — **PASS**.
- `go test -race ./internal/documentverification ./internal/documentverification/transport/http -count=1` — **PASS**.
- `go vet ./internal/documentverification ./internal/documentverification/transport/http` — **PASS**.
- `go build -o /dev/null ./cmd/api` — **PASS**.
- `gofmt` y `git diff --check` de los archivos del subcorte — **PASS**.

## Límites

Prueba unitaria con repositorio/Store fakes; no demuestra transacción, commit, PostgreSQL, GCS, almacenamiento durable ni cancelación de red. La comprobación reduce la ventana antes del retorno, pero no establece exclusión atómica con una cancelación concurrente posterior. Sin SQL/DDL, bases persistentes, Cloud, migraciones, datos reales, commit, push, despliegue o integración a `main`.
