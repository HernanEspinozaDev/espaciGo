# Evidencia VI-03 — error y actor inválido en lista reviewer

- **Fecha:** 2026-11-12.
- **Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- **Rama / HEAD:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Cambio:** test-only en `internal/documentverification/reviewer_access_list_repository_error_test.go`; sin cambios productivos.

## Hueco y comportamiento caracterizado

La medición local inicial marcó `ReviewerAccessService.List` con 85.7% de cobertura en `internal/documentverification`. Se añadieron dos caracterizaciones:

- `TestReviewerAccessListDiscardsPartialResultsOnRepositoryError`: el reader fake devuelve un candidato parcial y un error centinela; el servicio conserva el error original, retorna resultado nil y no lee documentos.
- `TestReviewerAccessListRejectsInvalidActorBeforeReadingRepository`: un actor malformado devuelve `ErrInvalid` antes de llamar al reader o al store.

Ambos tests pasaron en primera ejecución; no se afirma RED/GREEN ni se cambió producción. La medición final `go test -coverprofile=... ./internal/documentverification` reportó `ReviewerAccessService.List` en 100.0% y el paquete en 82.4% de statements.

## Verificación ejecutada

Con `env -i`, sin variables DB/PG:

- Focal `go test -race -count=1 ./internal/documentverification -run '^TestReviewerAccessList(DiscardsPartialResultsOnRepositoryError|RejectsInvalidActorBeforeReadingRepository)$' -v` — PASS.
- Suites `go test -race -count=1 ./internal/adapters/postgres/documentverification ./internal/documentverification ./internal/documentverification/transport/http` — PASS.
- `go vet` de los tres paquetes — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt -d` y verificación explícita de whitespace — limpios.

## Revisión

Revisión independiente `deleg_42937fcb` devolvió `passed: true`, sin hallazgos de seguridad/lógica. Confirmó que ambos tests ejercitan `ReviewerAccessService.List` real, preservan el error y detienen accesos antes de exponer resultados o llamar al reader/store para actor inválido. La evidencia se mantiene limitada a dependencias sintéticas.

## Límites

La respuesta parcial/error viene de un reader fake; la prueba no acredita errores del adapter PostgreSQL ni su semántica transaccional. No ejecuté SQL/DDL, persistencia, Cloud ni datos reales; V95/V96 siguen sin aplicar. Sin commit, push, deploy o integración a `main`.
