# VI-03 — descarga HTTP ante cancelación tras el callback de lectura

- **Fecha:** 2026-11-12.
- **Checkout/rama/HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- Worktree compartido ampliamente dirty; se preservaron cambios ajenos al subcorte.

## Cobertura

`TestReviewerDownloadHTTPFailsClosedWhenContextCancelsAfterReadTransaction` recorre la ruta HTTP real, el `ReviewerAccessService` real y `documentstorage.Store` real cifrado con BlobStore/repository/auth fakes. El control positivo de la misma fixture devuelve el archivo identity sintético con HTTP 200. En el escenario negativo, el repositorio fake cancela el contexto solo después de que el callback autorizado ya leyó el objeto y retornó correctamente; se exige segunda lectura del BlobStore, HTTP 503 neutral, ausencia de bytes y headers de descarga, y conservación de `private, no-store`, `nosniff` y `Vary: Authorization`.

Caracterización en verde; sin cambio productivo en este subcorte. Valida la integración local entre el guard de `ReviewerAccessService.Download` agregado en el subcorte anterior y la respuesta HTTP.

## Verificación

Con variables PostgreSQL/opt-in retiradas:

- Test focal — **PASS**.
- `go test -race ./internal/documentverification ./internal/documentverification/transport/http -count=1` — **PASS**.
- `go vet ./internal/documentverification ./internal/documentverification/transport/http` — **PASS**.
- `go build -o /dev/null ./cmd/api` — **PASS**.
- `gofmt` y `git diff --check` — **PASS**.

## Límites

El control de repositorio/auth es sintético; el Store usa BlobStore fake local. No demuestra transacción/commit, autorización PostgreSQL, BFF, GCS/IAM/CMEK ni cancelación de red. No se ejecutó SQL/DDL, se accedió a bases persistentes o Cloud, ni se usaron datos reales. Sin migraciones, commit, push, despliegue o integración a `main`.
