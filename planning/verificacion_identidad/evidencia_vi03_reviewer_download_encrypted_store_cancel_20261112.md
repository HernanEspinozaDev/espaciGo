# Evidencia — VI-03, descarga del revisor con adapter cifrado y cancelación

- **Fecha:** 2026-11-12.
- **Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- **Rama / HEAD:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y ampliamente dirty; este subcorte añadió `internal/adapters/documentstorage/reviewer_download_test.go`, esta evidencia y su referencia en el plan.

## Alcance y resultado

`TestReviewerDownloadWithEncryptedStoreFailsClosedOnInFlightCancellation` conecta el `ReviewerAccessService` real con `documentstorage.Store` real, un caso/grant sintético exacto y un BlobStore fake:

- Control positivo: el grant `review_identity` exacto permite leer el objeto cifrado; tamaño y SHA-256 sintéticos coinciden, y se devuelve el contenido esperado con MIME `image/png`.
- Escenario de cancelación: el BlobStore fake cancela el contexto durante `Get` y devuelve el envelope. El adapter no entrega plaintext; el servicio devuelve `ErrDocumentStorage` neutral y el resultado no contiene bytes ni MIME.

Fue una caracterización de primera ejecución en verde; no se observó RED ni se cambió producción en este subcorte. La prueba complementa el test focal del adapter que sí encontró y corrigió la entrega de plaintext tras cancelación durante la lectura.

## Verificación local

- `go test ./internal/adapters/documentstorage -run '^TestReviewerDownloadWithEncryptedStoreFailsClosedOnInFlightCancellation$' -count=1` — PASS.
- `go test -race ./internal/adapters/documentstorage -count=1` — PASS.
- `go vet ./internal/adapters/documentstorage` — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt` — PASS.
- Datos, grant, envelope y BlobStore son sintéticos/en memoria.

## Límites

No prueba autenticación/grants reales, repositorio PostgreSQL, transacción, BFF, GCS, IAM/CMEK, persistencia ni cancelación de un backend remoto. No se ejecutó SQL/DDL, acceso a DB persistente, Cloud, proveedor o datos reales. Sin commit, push, integración a `main` o despliegue.
