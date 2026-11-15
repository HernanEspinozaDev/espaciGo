# Evidencia — VI-03, ruta HTTP del revisor con adapter cifrado

- **Fecha:** 2026-11-12.
- **Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- **Rama / HEAD:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y ampliamente dirty; este subcorte añadió únicamente `internal/documentverification/transport/http/reviewer_download_encrypted_store_test.go` y esta evidencia, además de la referencia en el plan.

## Alcance y resultado

`TestReviewerDownloadHTTPUsesEncryptedStoreAndFailsClosedOnCancellation` ejerce la ruta HTTP real de descarga, el `ReviewerAccessService` real y `documentstorage.Store` real; repository/auth/grant y BlobStore usan fixtures/fakes sintéticos.

- Control positivo: grant identity exacto, evidencia cifrada, comprobación real de tamaño/SHA, HTTP 200, bytes esperados, MIME/Content-Length/Content-Disposition y `private, no-store`.
- Cancelación durante `BlobStore.Get`: HTTP 503 neutral; no entrega bytes privados, MIME binario, `Content-Disposition` ni `Content-Length`; conserva `private, no-store`, `nosniff` y `Vary: Authorization`.

Es caracterización en primera ejecución verde; no se observó RED ni hubo cambio productivo en este subcorte. Complementa la regresión de adapter que sí observó y corrigió el plaintext retornado tras cancelación en la lectura.

## Verificación local

Con variables de DB, PG y opt-in de integración retiradas:

- Test focal HTTP — PASS.
- `go test -race ./internal/adapters/documentstorage ./internal/documentverification ./internal/documentverification/transport/http -count=1` — PASS en los tres paquetes.
- `go vet` de esos tres paquetes — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt` — PASS.

## Límites

El repository/auth/grant y el BlobStore son sintéticos; no se valida PostgreSQL, autorización real, BFF, GCS, IAM/CMEK, persistencia ni cancelación de red remota. No se ejecutó SQL/DDL ni se accedió a base persistente, Cloud, proveedor o datos reales. Sin commit, push, integración a `main` ni despliegue.
