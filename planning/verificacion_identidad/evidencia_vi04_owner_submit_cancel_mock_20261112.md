# Evidencia VI-04 — submit owner sin respuesta tras cancelación

- **Fecha:** 2026-11-12
- **Rama:** `codex/marketplace/fix/kyc-06-save-identity-first`
- **HEAD base del corte:** `274f37e483a078890e92382a159fa49fd23c2fe8`
- **Cambio:** el handler `submit` comprueba cancelación después de leer el snapshot inicial (antes de conflicto/replay), después de la mutación y después del readback, antes de cualquier DTO/ETag.

## TDD y resultados

`TestOwnerSubmitDoesNotReturnCaseAfterCancellation` usa request HTTP, auth, snapshot identity y servicio fake:

- **Replay incierto:** RED observado: un GET de preflight cancelado retornaba `200`/ETag `"8"` y el caso `enviada` como replay de una revisión anterior. GREEN: 503 genérico, sin DTO/ETag y cero segundo `Submit`.
- **Cancelación tras mutación:** si `Submit` muta a `enviada`/revisión 8 y cancela, GREEN responde 503 sin iniciar el readback implícito; el cliente reconcilia con GET separado.
- **Cancelación dentro del readback:** RED observado: HTTP 200 exponía DTO/ETag aunque la carga cancelara el contexto. GREEN: 503 genérico sin DTO/ETag.

## Verificación local

Con entorno mínimo sin variables DB/PG:

- Tests focales de submit y lecturas owner: PASS.
- `go test -race ./internal/documentverification/... -count=1`: PASS.
- `go vet ./internal/documentverification/...`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- `gofmt -d` y `git diff --check`: PASS.

## Límites

La auth, las cargas, el submit y su estado son sintéticos. El test comprueba las fronteras HTTP y el orden de llamadas en el fake; no prueba autorización integrada, mutation/commit real, CAS, auditoría, PostgreSQL o persistencia. No ejecuté SQL/DDL, consultas, migraciones ni conexiones a bases persistentes/Cloud; sin datos reales, commit, push, integración a `main` o deploy.
