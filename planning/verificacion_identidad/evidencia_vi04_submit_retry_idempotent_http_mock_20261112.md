# VI-04 — reintento idempotente de envío identity

- **Fecha:** 2026-11-12.
- **Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- **Rama / HEAD:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Alcance de este subcorte:** `POST /document-verification/cases/{id}/submit`; cambio focal en handler Go con fixtures sintéticos. No atribuye otros cambios sucios del checkout.

## Comportamiento

Si el `If-Match` no coincide, pero el caso leído pertenece al titular autenticado, ya está `enviada` y la revisión actual es exactamente la inmediatamente posterior a la enviada, el handler devuelve `200`, el DTO actual y su ETag fuerte sin volver a invocar `Submit`. Esto recupera un envío ya confirmado cuya respuesta se perdió, incluido un reintento tras fallo de readback, sin emitir una segunda transición/auditoría.

El reconocimiento es estrecho: revisión cero, más de una revisión de diferencia o cualquier estado distinto de `enviada` mantiene el `409`. Los tests cubren ambos propósitos del endpoint; en identidad verifican la respuesta owner allowlisted y sus tres roles, sin `space_id`.

## TDD y gates

RED observado antes del cambio: `TestOwnerCasesSubmitRetryReturnsAlreadySubmittedCase` terminó con `409`, ETag vacío y body `case revision conflict`. GREEN después de reconocer solo el sucesor inmediato enviado, sin llamar de nuevo al servicio mutador.

- `go test -race ./internal/documentverification/... -count=1`: PASS (dominio, inspección y HTTP).
- `go vet ./internal/documentverification/...`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- `gofmt -d` y `git diff --check`: PASS.

Las pruebas se ejecutaron con `env -i` y entorno mínimo, sin variables de DB/PG ni `PGPASSFILE`. No se ejecutaron SQL/DDL, consultas conectadas, migraciones ni pruebas contra bases persistentes o Cloud.

## Límites

La ruta HTTP usa `casesServiceFake`; la prueba no demuestra transacción/CAS/atomicidad ni auth real del repositorio. Tampoco prueba BFF→Go, PostgreSQL, persistencia, concurrencia o la interfaz visual. La idempotencia de crear fue un subcorte separado. La revisión de decisión ya tiene caracterizaciones mock de conflicto y recuperación por GET sin replay, pero CAS/atomicidad/idempotencia persistida siguen sin verificar; esos gates permanecen abiertos. **VI-04 permanece parcial.**
