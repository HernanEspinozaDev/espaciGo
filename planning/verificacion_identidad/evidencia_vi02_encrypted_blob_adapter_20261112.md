# Evidencia — VI-02, adapter local de documentos cifrados sobre BlobStore

- **Fecha:** 2026-11-12.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y ampliamente dirty; se conservaron los cambios ajenos. Los archivos nuevos de este subcorte permanecen sin commit.

## Alcance

Se creó `internal/adapters/documentstorage.Store`, compatible por interfaz con los puertos de carga y lectura privada de `internal/documentverification`. El adapter sella contenido con el códec AES-GCM existente antes de llamar a `BlobStore.Put`, abre envelopes con AAD de identidad/espacio, convierte fallos de backend y envelopes corruptos/no autenticados en `ErrStorage` sin exponerlos; una cancelación de contexto se propaga como `context.Canceled` y no se remapea. Deriva un UUID de objeto estable desde el AAD canónico. El borrado vuelve a derivar ese mismo UUID desde el scope completo; no elimina el objeto al recibir owner/case/file de otro scope válido. El contrato exige que `BlobStore.Put` sea create-only; `cloudstorage.Store` ya implementa esa condición en fuente y compila contra las mismas firmas, pero no se ejecutó ni instanció un cliente GCS.

TDD del borrado por scope: `TestStoreDeleteRequiresExactIdentityScope` primero falló porque la versión inicial usaba `fileID` directamente y un owner alternativo borraba el objeto; después de derivar la clave del scope completo, el test focal pasó. La ampliación posterior cubre en modo caracterización owner, case y file alternativos por separado y el control positivo de scope exacto. Otros tests usan fakes en memoria para verificar envelope no plaintext y round-trip de identity/space, rechazo de scope distinto, fallo/corrupción de lectura sin contenido y borrado positivo de espacio/identity. Revisión independiente `deleg_4a0fad03` aprobó sin bloqueos; señaló que las cancelaciones mantienen su error de contexto y pidió cubrir variantes de case/file, ya añadidas y verificadas con el test focal. No se modificó código productivo tras esa revisión.

## Verificación local

Con variables `ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL`, `DATABASE_URL` y `PG*` retiradas:

- `go test ./internal/adapters/documentstorage ./internal/adapters/cloudstorage ./internal/adapters/documentcodec ./internal/documentverification/...` — PASS.
- Tras ampliar las negativas de borrado para owner/case/file: test focal `^TestStoreDeleteRequiresExactIdentityScope$`, suite `documentstorage`, `-race` y `go vet ./internal/adapters/documentstorage` — PASS.
- `go vet ./internal/adapters/documentstorage ./internal/adapters/cloudstorage ./internal/adapters/documentcodec ./internal/documentverification/...` — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt` — PASS; `git diff --check` — PASS para archivos versionados modificados. `git diff --no-index --check /dev/null` para cada archivo Go/evidencia nueva — sin diagnósticos (código 1 solo por diff); los nuevos `.go` se formatearon con gofmt.

## Límites y gates pendientes

La evidencia es unitaria y usa un BlobStore sintético. No prueba llamadas HTTP a Cloud Storage, IAM, bucket privado/CMEK, metadata/generaciones, key-version/rotación, wiring runtime (`newRuntimeDocumentStore` aún selecciona filesystem), persistencia, autorización integrada, PostgreSQL, workers, borrado físico/forense, backups o reinicio. No se ejecutó SQL/DDL, base persistente, Cloud, proveedor externo ni dato real; no se hizo commit, push, integración a `main` ni deploy. VI-02 permanece parcial y el gate de cloud/real data sigue cerrado.
