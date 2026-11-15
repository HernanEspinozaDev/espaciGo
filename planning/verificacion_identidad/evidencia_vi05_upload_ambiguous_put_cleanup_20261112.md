# Evidencia — VI-05, compensación ante Put ambiguo de evidencia

- **Fecha:** 2026-11-12.
- **Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- **Rama / HEAD:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y ampliamente dirty; se preservaron los demás cambios existentes.

## Cambio

En `UploadService.Upload`, si el Store devuelve error durante `Put`, ahora se intenta borrar el objeto con el mismo cleanup acotado (5 s, contexto independiente de la petición) que se usa cuando falla la escritura de metadata. Esto cubre el resultado ambiguo donde el backend pudo crear el objeto antes de perderse la respuesta; el error HTTP/domino sigue siendo `ErrDocumentStorage` y no expone el detalle interno. La limpieza se rutea por propósito identity/space y el helper común conserva el comportamiento existente de compensación posterior a fallo CAS/persistencia.

`TestUploadCompensatesObjectWhenPutReturnsAmbiguousFailure` simula para ambos propósitos que el fake registra el objeto y luego devuelve error. Comprueba que la compensación lo elimina y no cambia Snapshot/revisión. RED observado antes del cambio: `deleteCalls:0`, objeto sintético retenido; GREEN después.

## Verificación local

Con variables de DB/PG/opt-in de integración retiradas:

- Test focal de compensación identity/space — RED esperado antes, PASS después.
- `go test -race ./internal/documentverification/... -count=1` — PASS en servicio, inspector y HTTP.
- `go vet ./internal/documentverification/...` — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt` — PASS.

## Límites

Fake-only para el resultado ambiguo; no acredita comportamiento real de GCS, creación condicional remota, idempotencia del borrado, transacción PostgreSQL, durabilidad ni lifecycle worker. No se ejecutó SQL/DDL, no se accedió a bases persistentes, Cloud, proveedores o datos reales. Sin commit, push, integración a `main` o despliegue.
