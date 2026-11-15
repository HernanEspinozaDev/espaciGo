# Evidencia VI-03 — cancelación después de decisión/revocación

- **Fecha:** 2026-11-12
- **Rama:** `codex/marketplace/fix/kyc-06-save-identity-first`
- **HEAD:** `274f37e483a078890e92382a159fa49fd23c2fe8`
- **Cambio:** después de una decisión o revocación exitosa, los handlers no inician el readback implícito si el contexto se canceló. Responden con 503 neutral; el cliente puede reconciliar mediante un GET explícito.

## TDD y comportamiento

`TestReviewerMutationCancellationSkipsReadbackAndRequiresExplicitGet` cubre ambos caminos con servicio fake:

- Aprobación identity: el servicio incrementa la revisión y cambia a `aprobada`, luego cancela el contexto. RED observado: POST devolvía 200/ETag tras realizar el readback implícito. GREEN: POST devuelve `temporarily unavailable`, sin ETag, y no realiza el segundo `Load`.
- Revocación identity: el servicio cambia a `revocada` y cancela. RED observado: POST también devolvía 200/ETag mediante readback; GREEN: 503 sin readback.
- En ambos casos, el test hace después un GET deliberado con contexto nuevo y confirma el estado y ETag `"8"` sin repetir la mutación.

## Verificación local

Con entorno mínimo sin variables DB/PG:

- Test focal: PASS.
- `go test -race ./internal/documentverification/... -count=1`: PASS en dominio, inspección y transporte HTTP.
- `go vet ./internal/documentverification/...`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- `gofmt -d` y `git diff --check`: PASS.

## Límites

El fake modela una mutación confirmada antes de cancelar, pero no prueba auditoría/atomicidad, CAS, autorización, concurrencia o persistencia PostgreSQL. El GET de reconciliación es HTTP con servicio mock, no readback de base. No ejecuté SQL/DDL, migraciones, consultas ni conexiones a bases persistentes o Cloud; no usé datos reales. Sin commit, push, integración a `main` ni deploy; worktree previo preservado.
