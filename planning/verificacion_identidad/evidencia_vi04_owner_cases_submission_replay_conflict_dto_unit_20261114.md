# Evidencia — VI-04, replay de envío, conflicto de revisión, propagación de errores y límites DTO en casos del titular

- **Fecha:** 2026-11-14.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e0c0344d567bb44cf96ea4f` dirty.
- **Alcance:** Caracterización unitaria exhaustiva de idempotencia ante retransmisión de envío (submission replay), conflicto de concurrencia/revisión, propagación de errores de servicio, expiración en creación, montado de mux en `RegisterOwnerCasesRoutes` y validación de snapshots DTO en `internal/documentverification/transport/http/owner_cases.go`.
- **Pruebas implementadas:** `TestOwnerCasesHandlerSubmissionReplayConflictAndErrorBoundaries` en `internal/documentverification/transport/http/owner_cases_test.go`.

## Verificaciones cubiertas

1. **Replay idempotente de envío exitoso:** Cuando se reintenta un envío perdido (`If-Match: "3"`) sobre un caso ya enviado (`CaseSubmitted`) con revisión sucesora inmediata (`Revision: 4`), el manejador responde `200 OK` con cabecera `ETag: "4"` y el snapshot DTO sin invocar nuevamente el servicio mutador `Submit`.
2. **Conflicto de revisión en envío:** Cuando la revisión no coincide con la precondición y no corresponde al replay sucesor inmediato (ej. revisión desactualizada o caso no enviado), retorna `409 Conflict` con el mensaje `case revision conflict`.
3. **Propagación de errores de envío (`Submit`):** Los errores devueltos por el servicio mutador (`ErrConflict`, `ErrNotFound`, `ErrInvalid`) se mapean unificadamente a través de `ownerError` a códigos HTTP `409 Conflict`, `404 Not Found` y `400 Bad Request`.
4. **Creación con fecha de expiración poblada:** Peticiones `POST` a `/cases` con `document_expires_at` especificado propagan la fecha en `CreateInput` y devuelven `201 Created` con `ETag` correspondiente.
5. **Propagación de errores en listado (`ListOwnCases`):** Errores del servicio en la obtención de la lista de casos propios son capturados y mapeados de forma fail-closed con `ownerError`.
6. **Límites de validación en `ownerSnapshotDTO`:**
   - Casos de propósito `PurposeIdentity` legítimos responden `200 OK` con DTO válido.
   - Casos `PurposeIdentity` que contienen indebidamente `space_id` responden `404 Not Found` (fail-closed ante inconsistencias de dominio).
   - Casos `PurposeSpace` con `space_id` malformado (UUID no conforme) responden `404 Not Found`.
   - Casos con propósito desconocido responden `404 Not Found`.
7. **Montaje de rutas en ServeMux:** `RegisterOwnerCasesRoutes` monta adecuadamente tanto `/api/v1/document-verification/cases` como su prefijo `/api/v1/document-verification/cases/` en el mux estándar, despachando peticiones legítimas con `200 OK`.

## Resultados de ejecución local

- `go test -race -count=20 -run '^TestOwnerCasesHandlerSubmissionReplayConflictAndErrorBoundaries$' ./internal/documentverification/transport/http`: PASS.
- `go test -race -count=1 ./internal/documentverification/transport/http`: PASS.
- `go vet ./internal/documentverification/transport/http`: PASS.
- Cobertura de statements en `RegisterOwnerCasesRoutes`: subió a **100,0%**.
- Cobertura de statements en `OwnerCasesHandler.load`: subió a **100,0%**.
- Cobertura de statements en `ownerSnapshotDTO`: subió a **88,9%**.
- Cobertura total del paquete `./internal/documentverification/transport/http`: se sitúa en **93,6%**.

## Límites estrictos

- Entorno puramente unitario con fakes sintéticos en memoria.
- Sin llamadas SQL/DDL, sin acceso a bases persistentes ni Cloud, sin commit, push, deploy ni integración a `main`.
