# Evidencia — VI-04, fronteras de autorización y descarga de documentos de espacio en SpaceDownloadService y AccessPolicy

- **Fecha:** 2026-11-14.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e0c0344d567bb44cf96ea4f` dirty.
- **Alcance:** Caracterización exhaustiva de fronteras de validación del constructor `NewSpaceDownloadService`, validación de parámetros y campos internos en `Download`, propagación de errores del repositorio y del almacén de documentos privados (`ErrDocumentStorage`), detección de discrepancia de identificadores de caso, y validación estricta de límites de autorización temporal y de snapshot en `AuthorizeSpaceOwnerDownloadAt` dentro de `internal/documentverification/access_policy.go` y `internal/documentverification/download.go`.
- **Pruebas implementadas:**
  - `TestAuthorizeSpaceOwnerDownloadAtValidationBoundaries` en `internal/documentverification/access_policy_test.go`.
  - `TestSpaceDownloadConstructorAndParameterValidation` y `TestSpaceDownloadStorageReadErrorAndCaseMismatch` en `internal/documentverification/download_test.go`.

## Verificaciones cubiertas

1. **Validaciones en `AuthorizeSpaceOwnerDownloadAt` (`TestAuthorizeSpaceOwnerDownloadAtValidationBoundaries`):**
   - Rechazo fail-closed (`ErrInvalid`) si el actor ID no es un UUID v4 válido (`ValidateID(actor) != nil`).
   - Rechazo fail-closed (`ErrInvalid`) si el file ID no es un UUID v4 válido (`ValidateID(fileID) != nil`).
   - Rechazo fail-closed (`ErrInvalid`) si el snapshot no es válido (`!validSnapshot(x)`), como por ejemplo identificador de caso malformado.
   - Rechazo fail-closed (`ErrInvalid`) si la marca temporal de evaluación es cero (`now.IsZero()`).
   - Rechazo con `ErrCapability` si el propósito del snapshot no es `PurposeSpace` (ejercitado con snapshot válido de `PurposeIdentity`).
   - Rechazo con `ErrCapability` si el caso fue superseded (`SupersededAt != nil`).
   - *(Complementado por la suite de políticas: denegación con `ErrCapability` ante actor que no coincide con `SpaceOwnerID` o tras cambio de titularidad, rechazo con `ErrNotFound` ante archivo inexistente o retención vencida en `AuthorizeSpaceOwnerDownloadAt`, y rechazo con `ErrFilesNotClean` ante archivos en estado no limpio)*.
2. **Validaciones en `NewSpaceDownloadService`:**
   - Rechazo fail-closed (`ErrInvalid`) ante repositorio nulo (`repository == nil`).
   - Rechazo fail-closed (`ErrInvalid`) ante almacén de documentos nulo (`store == nil`).
3. **Validaciones de entrada y receptor en `SpaceDownloadService.Download`:**
   - Rechazo fail-closed (`ErrInvalid`) ante receptor nulo (`s == nil`).
   - Rechazo fail-closed (`ErrInvalid`) si el campo interno `s.repository` es nil.
   - Rechazo fail-closed (`ErrInvalid`) si el campo interno `s.store` es nil.
   - Rechazo fail-closed (`ErrInvalid`) ante `actor` no conforme con UUID v4.
   - Rechazo fail-closed (`ErrInvalid`) ante `caseID` no conforme con UUID v4.
   - Rechazo fail-closed (`ErrInvalid`) ante `fileID` no conforme con UUID v4.
4. **Flujos de error en repositorio y almacenamiento:**
   - Propagación directa del error devuelto por `repository.WithSpaceCaseForDownload`.
   - Detección de discrepancia entre el `caseID` solicitado y el `ID` del snapshot retornado por el repositorio, devolviendo `ErrNotFound` antes de cualquier lectura de blob.
   - Mapeo estricto del fallo de lectura en almacenamiento (`store.ReadSpaceDocument`) hacia el centinela `ErrDocumentStorage`.

## Resultados de ejecución local

- `go test -race -count=20 -run '^Test(AuthorizeSpaceOwnerDownloadAtValidationBoundaries|SpaceDownloadConstructorAndParameterValidation|SpaceDownloadStorageReadErrorAndCaseMismatch)$' ./internal/documentverification`: PASS (1.058s).
- `go test -race -count=1 ./internal/documentverification/...`: PASS.
- `go vet ./internal/documentverification/...`: PASS.
- `git diff --check`: PASS (sin errores de formato ni espacios residuales).
- Cobertura de statements en `AuthorizeSpaceOwnerDownload`: **100,0%**.
- Cobertura de statements en `AuthorizeSpaceOwnerDownloadAt`: **100,0%**.
- Cobertura total de `internal/documentverification/access_policy.go`: **100,0%**.
- Cobertura de statements en `NewSpaceDownloadService`: **100,0%**.
- Cobertura de statements en `Download`: **96,0%**.
- Cobertura total de `internal/documentverification/download.go`: **96,9%**.
- Cobertura total del paquete `internal/documentverification`: **87,8%**.

## Aislamiento de entorno y límites de seguridad

- **Sin base de datos persistente ni Cloud SQL:** Todas las pruebas emplean fakes sintéticos en memoria (`downloadCaseRepo`, `errDownloadRepo`, `syntheticDocumentStore`, `failingDocumentStore`).
- **Sin llamadas a Cloud Storage / GCS:** Verificación de contratos mediante interfaces Go puras.
- **Sin DDL ni sentencias SQL:** Cero modificaciones de esquema.
- **Sin commits ni pushes:** Trabajo acotado localmente en la rama `codex/marketplace/fix/kyc-06-save-identity-first` preservando el estado uncommitted previo.
