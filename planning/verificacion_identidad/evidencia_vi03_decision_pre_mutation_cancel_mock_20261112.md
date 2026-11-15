# Evidencia VI-03 — no iniciar decisión/revocación tras cancelación

- **Fecha:** 2026-11-12
- **Rama:** `codex/marketplace/fix/kyc-06-save-identity-first`
- **HEAD:** `274f37e483a078890e92382a159fa49fd23c2fe8`
- **Cambio:** el handler de decisión devuelve 503 neutral si el contexto se cancela durante la lectura previa del caso, antes de invocar `ReviewIdentity`/`Review`. El handler de revocación verifica cancelación tras decodificar el body y antes de llamar `Revoke`.

## TDD y comportamiento

- `TestReviewerIdentityDecisionDoesNotMutateWhenPreflightLoadCancels`: el servicio fake cancela el contexto justo antes de devolver el snapshot de preflight. RED observado: el handler aun invocaba la decisión y el fake avanzaba de `enviada`/revisión 7 a `aprobada`/8. GREEN: HTTP 503 neutral, una lectura, cero decisiones y estado/revisión intactos.
- `TestReviewerRevocationDoesNotMutateWhenRequestAlreadyCanceled`: RED observado: con contexto precancelado, el handler invocaba `Revoke` y el fake mutaba a `revocada`/8. GREEN: HTTP 503 neutral, cero invocaciones de revocación y estado/revisión intactos.

## Verificación local

Con entorno mínimo sin variables DB/PG:

- Tests focales: PASS.
- `go test -race ./internal/documentverification/... -count=1`: PASS en dominio, inspección y transporte HTTP.
- `go vet ./internal/documentverification/...`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- `gofmt -d` y `git diff --check`: PASS.

## Límites

Las pruebas usan auth/servicios sintéticos que ignoran deliberadamente la cancelación para alcanzar la guardia del handler. No demuestran atomicidad, autorización, CAS ni comportamiento de una mutación PostgreSQL real. No ejecuté SQL/DDL, migraciones, consultas ni conexiones a bases persistentes o Cloud; no usé datos reales. Sin commit, push, integración a `main` ni deploy; worktree previo preservado.
