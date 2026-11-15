# Evidencia — VI-04, cuotas, archivo vacío y límites de seguridad en subida de documentos del titular

- **Fecha:** 2026-11-14.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e0c0344d567bb44cf96ea4f` dirty.
- **Alcance:** Caracterización unitaria exhaustiva de límites de cuota, archivos vacíos, disponibilidad y control de acceso en la subida de evidencia de titulares en `internal/documentverification/transport/http/owner_cases.go`.
- **Pruebas implementadas:** `TestOwnerCaseUploadQuotaEmptyFileAndSecurityBoundaries` en `internal/documentverification/transport/http/owner_upload_test.go`.

## Verificaciones cubiertas

1. **Límite de documentos excedido (Quota):** Cuando el servicio de subida retorna `documentverification.ErrQuota`, el manejador responde `409 Conflict` con el mensaje explícito `document limit reached`.
2. **Archivo vacío (0 bytes):** Partes de archivo con contenido vacío (`len(content) == 0`) son rechazadas fail-closed con `400 Bad Request` y mensaje `empty file`, sin invocar el servicio de subida.
3. **Servicio de subida no configurado:** Solicitudes de subida a manejadores instanciados sin capacidad de subida (`h.upload == nil`) responden `503 Service Unavailable`.
4. **Disponibilidad fail-closed:** Receptor nil `(*OwnerCasesHandler)(nil)` y dependencias nulas (`auth == nil` o `service == nil`) devuelven inmediatamente `503 Service Unavailable`.
5. **Autenticación no prohibitiva:** Errores de autenticación distintos de `ErrForbidden` (ej. `ErrUnauthorized`) retornan `401 Unauthorized` acompañado de la cabecera `WWW-Authenticate: Bearer`.
6. **Fallo de autorización fail-closed:** Error `identity.ErrForbidden` retorna `404 Not Found` para proteger la privacidad y no confirmar existencia del caso.
7. **Identificador de caso malformado:** Rutas con UUIDs no conformes en la ruta de subida (`/cases/not-a-valid-uuid/files`) son rechazadas con `404 Not Found`.
8. **Parte de archivo faltante:** Solicitudes multipart que únicamente incluyen la parte `evidence_role` sin adjuntar el archivo son rechazadas con `400 Bad Request` y mensaje `file part is required`.
9. **Método no permitido en subida de archivos:** Peticiones `GET` sobre `/cases/{id}/files` devuelven `405 Method Not Allowed` y cabecera `Allow: POST`.
10. **Método no permitido en envío de caso:** Peticiones `GET` sobre `/cases/{id}/submit` devuelven `405 Method Not Allowed` y cabecera `Allow: POST`.

## Resultados de ejecución local

- `go test -race -count=20 -run '^TestOwnerCaseUploadQuotaEmptyFileAndSecurityBoundaries$' ./internal/documentverification/transport/http`: PASS.
- `go test -race -count=1 ./internal/documentverification/transport/http`: PASS.
- `go vet ./internal/documentverification/transport/http`: PASS.
- Cobertura de statements en `OwnerCasesHandler.ServeHTTP`: subió a **100,0%**.
- Cobertura de statements en `ownerError`: subió a **100,0%**.
- Cobertura total del paquete `./internal/documentverification/transport/http`: subió a **94,6%**.

## Límites estrictos

- Entorno puramente unitario con fakes sintéticos en memoria.
- Sin llamadas SQL/DDL, sin acceso a bases persistentes ni Cloud, sin commit, push, deploy ni integración a `main`.
