# Evidencia — VI-04, tipos MIME y límites de seguridad en la ruta de descarga del titular

- **Fecha:** 2026-11-14.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e0c0344d567bb44cf96ea4f` dirty.
- **Alcance:** Caracterización unitaria local exhaustiva de la ruta de descarga de documentos de titular en `internal/documentverification/transport/http/owner_download.go`.
- **Pruebas implementadas:** `TestOwnerDownloadRouteMIMETypesAndSecurityBoundaries` en `internal/documentverification/transport/http/owner_download_test.go`.

## Verificaciones cubiertas

1. **Tipos MIME soportados:** Comprobación de `image/jpeg` entregando cabecera `Content-Type: image/jpeg` y disposición de descarga `attachment; filename="space-document.jpg"` con `200 OK`.
2. **Rechazo fail-closed de MIME no autorizado:** Descargas con MIME no admitido (ej. `text/plain`) retornan `404 Not Found` sin exponer contenido ni metadatos.
3. **Métodos HTTP no permitidos:** Solicitudes `POST` son rechazadas tanto por el ruteador `http.ServeMux` (con `Allow: GET, HEAD`) como por el manejador directo `OwnerDownloadHandler` (con `Allow: GET`), respondiendo `405 Method Not Allowed`.
4. **Fallo de autorización fail-closed:**
   - Actores con error `identity.ErrForbidden` reciben `404 Not Found` para evitar enumeración o confirmación de existencia de recursos privados.
   - Fallos de autenticación `identity.ErrUnauthorized` reciben `401 Unauthorized` junto a la cabecera `WWW-Authenticate: Bearer`.
5. **Cabeceras de autorización malformadas:** Tokens con esquema inválido (`Basic`, `Token`) o sin payload (`Bearer`, vacío) son rechazados inmediatamente con `401 Unauthorized` y cabecera `WWW-Authenticate: Bearer`.
6. **Errores de dominio:** Errores de capacidad o ciclo de vida (`ErrSelfReview`, `ErrCapability`, `ErrNotFound`, `ErrTransition`) se mapean de forma unificada y segura a `404 Not Found`.
7. **Manejadores nulos / dependencias no inicializadas:** Receptor nil o dependencias nulas devuelven `503 Service Unavailable`.

## Resultados de ejecución local

- `go test -race -count=20 -run '^TestOwnerDownloadRoute' ./internal/documentverification/transport/http`: PASS.
- `go test -race -count=1 ./internal/documentverification/transport/http`: PASS.
- `go vet ./internal/documentverification/transport/http`: PASS.
- Cobertura de statements en `owner_download.go`: **100,0%** en todas sus funciones (`NewOwnerDownloadHandler`, `RegisterOwnerDownloadRoute`, `ServeHTTP`, `fixedDownloadType`).
- Cobertura total del paquete `./internal/documentverification/transport/http`: subió a **89,5%**.

## Límites estrictos

- Es test-only con fakes sintéticos en memoria.
- No prueba almacenamiento real en Cloud Storage / GCS ni persistencia PostgreSQL.
- Sin llamadas SQL/DDL, sin acceso a Cloud/producción, sin commit, push, deploy ni integración a `main`.
