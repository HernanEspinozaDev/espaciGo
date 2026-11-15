# Evidencia VI-04 — lecturas owner sin serializar tras cancelación

- **Fecha:** 2026-11-12
- **Rama:** `codex/marketplace/fix/kyc-06-save-identity-first`
- **HEAD base del corte:** `274f37e483a078890e92382a159fa49fd23c2fe8`
- **Alcance:** lecturas owner de lista y detalle de casos, con snapshot identity sintético.

## Cambios y TDD

- `Service.ListOwnCases` y `Service.LoadOwnCase` rechazan contexto nulo/precancelado antes de consultar `OwnCaseReader`, y descartan resultados si el lector retorna después de cancelarse el contexto. Propagan `ctx.Err()`.
- `OwnerCasesHandler.list` y `.load` vuelven a comprobar el contexto tras el servicio y antes de construir/escribir DTO o ETag.
- `TestOwnerCaseReadsStopWhenContextCancels`: RED observado en los cuatro escenarios (precancelado y cancelado al retornar reader para lista/detalle); antes se devolvían snapshots. GREEN: `context.Canceled`, resultado vacío y cero lecturas cuando ya estaba cancelado.
- `TestOwnerCaseReadHandlersDoNotSerializeAfterCancellation`: RED observado con respuestas HTTP 200 que incluían ID, estado, IDs/roles de evidencias y ETag (detalle) aunque el fake cancelaba antes de retornar. GREEN: ambas rutas responden 503 genérico, sin DTO ni ETag.

## Verificación local

Con entorno mínimo sin variables DB/PG:

- Tests focales de servicio y handlers: PASS.
- `go test -race ./internal/documentverification/... -count=1`: PASS.
- `go vet ./internal/documentverification/...`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- `gofmt -d` y `git diff --check`: PASS.

## Límites

La identidad, auth, casos y lector son sintéticos. El handler evita serializar cuando observa cancelación y el servicio evita retornar resultados cancelados, pero los tests no demuestran autorización real, SQL, transacción, cancelación del driver ni persistencia. No ejecuté SQL/DDL, consultas, migraciones ni conexiones a bases persistentes/Cloud; sin datos reales, commit, push, integración a `main` o deploy.
