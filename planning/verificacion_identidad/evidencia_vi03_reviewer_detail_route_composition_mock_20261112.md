# Evidencia VI-03 — detalle reviewer por ruta registrada y servicio real

- **Fecha:** 2026-11-12.
- **Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- **Rama / HEAD:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Cambio:** test-only; nuevo `internal/documentverification/transport/http/reviewer_detail_route_composition_test.go`; no cambió código productivo.

## Comportamiento caracterizado

`TestReviewerIdentityDetailRegisteredRouteComposesServiceAndAllowlistedDTO` manda un GET sintético de detalle por `http.ServeMux` y `RegisterReviewerCasesRoutes`, con `ReviewerCasesHandler` y `ReviewerAccessService` reales. El repositorio fake suministra el caso y una autorización con grant sintético exacto de `review_identity` al titular; el servicio hace dos revalidaciones y reenvía actor/caso exactos. El declaration-reader fake recibe titular y revisión exactos una vez. La respuesta incluye solo los cuatro campos allowlisted de la declaración, ETag de revisión y cabeceras privadas; no expone titular/revisión legal ni metadata privada comprobada. El store de archivos registra cero lecturas.

La caracterización pasó al primer intento; no se afirma RED/GREEN. `ReviewerCasesHandler.loadDetail` mide 100.0% en el paquete HTTP; la cobertura total HTTP se mantuvo en 85.7%.

## Verificación ejecutada

Bajo `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR"`:

- Test focal con `go test -race -count=1` — PASS.
- Suites completas `-race` de `internal/documentverification` y `internal/documentverification/transport/http` — PASS.
- `go vet` de ambos paquetes — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt -d` y verificación explícita de whitespace del archivo candidato — limpios.

## Revisión

Revisión independiente `deleg_471a8793` aprobó (`passed: true`) sin hallazgos. Confirmó la composición de ruta/handler/servicio y el allowlist. Recomendó mantener explícito que autenticador, grants, repository y declaración son fakes, y que el test pasó al primer intento; la evidencia conserva esos límites y no afirma RED/GREEN.

## Límites

El grant, los snapshots, la lectura de declaración y la autenticación son sintéticos. El test compone código real de servicio/handler/ruta, pero no prueba asignaciones efectivas, autenticación real, SQL, PostgreSQL, persistencia, storage, GCS ni Cloud. Sin SQL/DDL, acceso a persistencia/Cloud, datos reales, commit, push, deploy o integración a `main`. Los cambios previos ajenos del worktree se conservaron.
