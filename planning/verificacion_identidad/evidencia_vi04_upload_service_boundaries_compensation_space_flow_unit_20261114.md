# Evidencia — VI-04, fronteras del constructor, flujo PurposeSpace, compensación CAS y validación de cuotas en UploadService

- **Fecha:** 2026-11-14.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e0c0344d567bb44cf96ea4f` dirty.
- **Alcance:** Caracterización exhaustiva de fronteras del constructor `NewUploadService`, validaciones de receptor e inputs en `Upload`, soporte integral del flujo `PurposeSpace` con compensación en fallos de persistencia CAS (`DeleteSpaceDocument`), y ramas de frontera en cuotas, snapshot no encontrado y cancelación de contexto pre-almacenamiento en `internal/documentverification/upload.go`.
- **Pruebas implementadas:** `TestUploadServiceConstructorAndParameterBoundaries` y `TestUploadSpaceDocumentFlowAndCompensation` en `internal/documentverification/upload_test.go`.

## Verificaciones cubiertas

1. **Validaciones en `NewUploadService`:** Verificación de rechazo fail-closed (`ErrInvalid`) ante dependencias nulas (`cases == nil`, `inspector == nil` o `store == nil`), e inicialización correcta con generador `newID` por defecto.
2. **Validaciones de entrada en `Upload`:** Verificación de rechazo con `ErrInvalid` ante receptor nil, contexto nil, actor no-UUID, caseID no-UUID y contenido crudo vacío.
3. **Cancelación temprana de contexto:** Detección de contexto pre-cancelado devolviendo `context.Canceled` sin invocar inspección ni almacenamiento.
4. **Conflicto de revisión:** Rechazo de revisión en el límite superior (`^uint64(0)`) devolviendo `ErrConflict`.
5. **Estados inelegibles:** Casos en estados terminales o no editables (`CaseApproved`, `CaseRejected`, `CaseInReview`, `CaseRevoked`) son rechazados con `ErrTransition`.
6. **Propósitos no conformes:** Rechazo fail-closed (`ErrInvalid`) ante propósitos no soportados.
7. **Generador de IDs defectuoso:** Rechazo con `ErrInvalid` si `newID` es nil o retorna un identificador no conforme con formato UUID v4.
8. **Limpieza con propósito desconocido:** `cleanupStoredObject` retorna `ErrInvalid` ante snapshot con propósito inválido.
9. **Caso no encontrado en `LoadOwnCase`:** UUID válido de caso inexistente en repositorio propaga `ErrNotFound` inmediatamente.
10. **Archivos pre-existentes corruptos:** Inconsistencias en archivos del snapshot existente en repositorio provocan fallo en la comprobación inicial de `ValidateQuota` devolviendo `ErrInvalid`.
11. **Límite de cuota alcanzado:** Archivos existentes alcanzando la capacidad máxima de propósito identidad no tipado devuelven `ErrQuota`.
12. **Cancelación durante inspección:** Cancelación de contexto producida durante la etapa de inspección se detecta antes de la escritura en almacenamiento, retornando `context.Canceled` con 0 llamadas a almacenamiento privado.
13. **Flujo exitoso en `PurposeSpace`:** Subida conforme en caso de verificación de espacio ejecuta `PutSpaceDocument` con los metadatos adecuados (owner, space, caseID, fileID) y actualiza el snapshot a revisión 2 con el nuevo archivo añadido.
14. **Compensación ante fallo de persistencia en `PurposeSpace`:** Fallo en la transacción CAS (`Transition`) tras escritura exitosa en almacenamiento desencadena inmediatamente la compensación `DeleteSpaceDocument` en el almacén privado.

## Resultados de ejecución local

- `go test -race -count=20 -run '^TestUpload(ServiceConstructorAndParameterBoundaries|SpaceDocumentFlowAndCompensation)$' ./internal/documentverification`: PASS (1.161s).
- `go test -race -count=1 ./internal/documentverification/...`: PASS.
- `go vet ./internal/documentverification/...`: PASS.
- `git diff --check`: PASS (sin errores de formato ni trailing whitespace).
- Cobertura de statements en `NewUploadService`: alcanzó el **100,0%**.
- Cobertura de statements en `Upload`: alcanzó el **100,0%**.
- Cobertura de statements en `cleanupStoredObject`: alcanzó el **100,0%**.
- Cobertura total del archivo `internal/documentverification/upload.go`: **100,0%**.
- Cobertura total del paquete `internal/documentverification`: **86,7%**.

## Aislamiento de entorno y límites de seguridad

- **Sin base de datos persistente ni Cloud SQL:** Todas las pruebas operan en memoria con implementaciones sintéticas (`memoryRepository`, `uploadInspectorFake`, `uploadStoreFake`).
- **Sin Cloud Storage ni llamadas de red:** El almacenamiento de blobs se valida a través de contratos de interfaz en memoria.
- **Sin DDL ni migraciones:** No se ejecutaron sentencias SQL ni cambios de esquema.
- **Sin integración en `main`:** Trabajo restringido a la rama de tarea local `codex/marketplace/fix/kyc-06-save-identity-first` con worktree compartido preservado.
