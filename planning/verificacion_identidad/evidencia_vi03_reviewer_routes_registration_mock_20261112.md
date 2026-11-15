# Evidencia VI-03 — registro de rutas de cola reviewer

- **Fecha:** 2026-11-12.
- **Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- **Rama / HEAD:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Cambio:** test-only en `internal/documentverification/transport/http/reviewer_queue_route_registration_test.go`; sin cambios productivos.

## Comportamiento caracterizado

La auditoría de cobertura local detectó `RegisterReviewerCasesRoutes` sin líneas ejecutadas en la suite del paquete HTTP. Se añadió `TestReviewerQueueRoutesRegisterCollectionAndTrailingSlash`, que registra un `ReviewerCasesHandler` real en `http.ServeMux` mediante el mount helper y consulta tanto la ruta exacta de colección como la variante con `/` final. Ambas pasan por `ReviewerAccessService` real y repo/auth/store fakes; cada GET devuelve el caso sintético, llama al reader una vez con el actor del principal y mantiene `private, no-store`/`Vary: Authorization`.

El test pasó al primer intento; es una caracterización, no se afirma RED/GREEN ni hubo cambio productivo.

## Verificación ejecutada

Con `env -i`, sin heredar variables DB/PG:

- Test focal `go test -race -count=1 ./internal/documentverification/transport/http -run '^TestReviewerQueueRoutesRegisterCollectionAndTrailingSlash$' -v` — PASS.
- Suites `go test -race -count=1 ./internal/adapters/postgres/documentverification ./internal/documentverification ./internal/documentverification/transport/http` — PASS.
- `go vet` en los tres paquetes — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `go test -coverprofile=... ./internal/documentverification/transport/http` — PASS; `RegisterReviewerCasesRoutes` subió de 0.0% a 100.0% y la cobertura del paquete reportó 85.7% statements en esta corrida.
- `gofmt -d` y verificación explícita de whitespace — limpios.

Revisión independiente `deleg_991e4917` devolvió `passed: true`, sin hallazgos. Confirmó que ambas rutas registradas pasan por el handler y servicio reales, y que las aserciones de status, DTO, actor, headers y conteo son adecuadas. Las fakes no representan auth ni repositorio productivos.

## Límites

La auth y el repositorio son sintéticos; no se ejecuta el montaje completo de `cmd/api`, el adapter PostgreSQL, SQL/DDL, persistencia, BFF ni Cloud. La cobertura es de la función de registro dentro del paquete HTTP; no prueba autorización real ni reachability en runtime desplegado. No se accedió a bases persistentes/Cloud; V95/V96 siguen sin aplicar. Sin datos reales, commit, push, deploy o integración a `main`.
