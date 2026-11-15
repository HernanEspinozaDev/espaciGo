# Evidencia VI-03 — no serializar readback cancelado de decisión/revocación

- **Fecha:** 2026-11-12
- **Rama:** `codex/marketplace/fix/kyc-06-save-identity-first`
- **HEAD base del corte:** `274f37e483a078890e92382a159fa49fd23c2fe8`
- **Cambio:** `ReviewerCasesHandler.load` vuelve a comprobar el contexto tras `service.Load` y antes de validar/serializar snapshot, fijar ETag o escribir JSON.

## TDD y comportamiento

`TestReviewerMutationReadbackDoesNotSerializeWhenLoadCancels` cubre decisión identity y revocación con un servicio fake que aplica la mutación y cancela el contexto durante el readback HTTP:

- RED observado en ambos casos: HTTP 200, ETag `"8"` y DTO (`aprobada`/`revocada`) aun cuando `Load` retornó con contexto cancelado.
- GREEN: HTTP 503 neutral, sin ETag ni cuerpo de DTO. El fake confirma que la mutación fue invocada una vez y avanzó la revisión a 8; se suprime únicamente la respuesta de readback. El cliente debe reconciliar con GET explícito/contexto nuevo.

## Verificación local

Con entorno mínimo sin variables DB/PG:

- Tests focales de readback, cancelación previa y cancelación post-mutación: PASS.
- `go test -race ./internal/documentverification/... -count=1`: PASS.
- `go vet ./internal/documentverification/...`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- `gofmt -d` y `git diff --check`: PASS.

## Límites

El fake hace visible el comportamiento del handler, no prueba cancelación de PostgreSQL ni garantiza que una mutación real haya hecho commit o rollback. No demuestra autorización, CAS, atomicidad o persistencia. No ejecuté SQL/DDL, consultas, migraciones ni conexiones a bases persistentes o Cloud; sin datos reales, commit, push, integración a `main` ni deploy.
