# Evidencia — VI-04, validación de tipos MIME, multipart reader, límites de rol y codificación JSON en subida de evidencia

- **Fecha:** 2026-11-14.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e0c0344d567bb44cf96ea4f` dirty.
- **Alcance:** Caracterización unitaria exhaustiva de fallos de negociación de medios multipart, sintaxis de boundary en lector multipart, tamaño y metadata de roles de evidencia, nombre de archivo vacío en partes de archivo, propagación de errores de servicio de subida, validación de identificador en manejador de subida y protección fail-closed en serialización JSON (`writeOwnerJSON`) en `internal/documentverification/transport/http/owner_cases.go`.
- **Pruebas implementadas:** `TestOwnerCaseUploadMultipartReaderErrorsAndTypeValidation` en `internal/documentverification/transport/http/owner_upload_test.go`.

## Verificaciones cubiertas

1. **Tipo de contenido no multipart:** Solicitudes `POST` a `/cases/{id}/files` sin `multipart/form-data` (ej. `application/json`) devuelven fail-closed `415 Unsupported Media Type` con mensaje `multipart form required`.
2. **Multipart sin boundary:** Peticiones con `Content-Type: multipart/form-data` pero sin parámetro `boundary` válido provocan error en `r.MultipartReader()`, respondiendo inmediatamente `400 Bad Request` con mensaje `invalid multipart request`.
3. **Rol de evidencia sobredimensionado:** Cuando el campo `evidence_role` excede los 64 bytes permitidos, la petición es rechazada con `400 Bad Request` y mensaje `invalid evidence role`.
4. **Rol de evidencia con metadata de archivo:** Si la parte `evidence_role` se envía indebidamente con atributo `filename`, se rechaza con `400 Bad Request` (`invalid evidence role`).
5. **Parte de archivo sin nombre de archivo:** Si el campo `file` se envía sin `filename` (o con nombre vacío), se rechaza con `400 Bad Request` (`invalid file part`).
6. **Propagación de errores del servicio de subida:** Errores no relacionados con cuota devueltos por `upload.Upload` (ej. `ErrConflict`) se traducen unificadamente vía `ownerError` a `409 Conflict`.
7. **Validación de identificador en manejador:** Invocaciones a `uploadFile` con UUIDs no conformes responden `404 Not Found`.
8. **Protección en serialización JSON (`writeOwnerJSON`):** Tipos de datos no codificables en JSON devuelven fail-closed `503 Service Unavailable`.

## Resultados de ejecución local

- `go test -race -count=20 -run '^TestOwnerCaseUploadMultipartReaderErrorsAndTypeValidation$' ./internal/documentverification/transport/http`: PASS.
- `go test -race -count=1 ./internal/documentverification/transport/http`: PASS.
- `go vet ./internal/documentverification/transport/http`: PASS.
- Cobertura de statements en `writeOwnerJSON`: alcanzó el **100,0%**.
- Cobertura de statements en `OwnerCasesHandler.uploadFile`: subió a **91,0%**.
- Cobertura total del paquete `./internal/documentverification/transport/http`: subió a **95,2%**.

## Límites estrictos

- Entorno puramente unitario con fakes sintéticos en memoria.
- Sin llamadas SQL/DDL, sin acceso a bases persistentes ni Cloud, sin commit, push, deploy ni integración a `main`.
