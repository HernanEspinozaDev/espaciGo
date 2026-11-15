# Evidencia — VI-04, revocación HTTP con servicio real y repositorio sintético

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido, con numerosos cambios previos; se mantuvo sin limpieza ni reset.

## Alcance verificado

- `internal/documentverification/transport/http/reviewer_revoke_service_test.go` añade `TestReviewerRevokeTransitionsActualServiceAndReadsBackInMemoryState`. Envía `POST /revoke` al handler HTTP real, con authenticator fake, `ReviewerAccessService` real y `documentverification.Service` real.
- La prueba usa un expediente aprobado, revisor y grant identity sintéticos con scope exacto, y un repositorio in-memory. El callback real de `Service.Revoke` valida la transición; el fake aplica el snapshot/cambio de revisión en memoria y conserva el audit del servicio. El handler relee el caso por el lector fake y responde estado `revocada`, revisión `8` y ETag fuerte `"8"`.
- `TestReviewerRevokeWithInvalidGrantIsNeutralAndDoesNotReadBack` recorre cinco variantes: grant ausente, ligado a otro reviewer, scope de otro titular, capacidad `space` en lugar de `identity` y asignación dual identity/space. El servicio real deniega cada caso; el handler responde `404 not found`, el fake conserva Snapshot/auditoría y no hay lectura posterior.
- `TestReviewerRevokeReadbackFailureCanBeResolvedByGet` inyecta un error en la lectura posterior al éxito de `Revoke`: el POST responde `503` neutral, el fake conserva la transición, y una consulta GET explícita posterior devuelve `revocada`, revisión `8` y ETag `"8"` sin repetir la mutación.
- Se verifican el actor en transición/readback, una llamada a transición y una lectura posterior, el timestamp fijo de revocación, el revisor, la preservación de `VerifiedAt`/`VerifiedUntil` y los campos del evento auditado. Los puertos de lectura de documentos existen solo para construir el servicio y no se invocan.
- Son caracterizaciones; no se observó RED y no cambió código productivo. La revisión read-only inicial `deleg_60ca9041` aprobó el caso positivo y sugirió opcionalmente afirmar que `VerifiedAt`/`VerifiedUntil` no cambian; se incorporó esa aserción. La revisión follow-up `deleg_6fb0116e` aprobó el caso positivo final. La revisión final `deleg_2aabb855` aprobó también la tabla de cinco denegaciones, sin hallazgos de seguridad/lógica ni sugerencias. La revisión independiente `deleg_7abed7c0` aprobó el caso de error de readback y su recuperación GET sin hallazgos ni sugerencias.

## Verificación local

- `go test ./internal/documentverification/transport/http -run '^TestReviewerRevoke(TransitionsActualServiceAndReadsBackInMemoryState|WithInvalidGrantIsNeutralAndDoesNotReadBack|ReadbackFailureCanBeResolvedByGet)$' -count=1` — PASS.
- `go test -race ./internal/documentverification/... -count=1` — PASS.
- `go vet ./internal/documentverification/...` — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt -d internal/documentverification/transport/http/reviewer_revoke_service_test.go`, `git diff --check` y whitespace del archivo nuevo — PASS.
- Las ejecuciones Go retiraron `ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL`, `IDENTITY_TEST_DATABASE_URL`, `TEST_DATABASE_URL`, `ESPACIGO_DATABASE_URL`, `DATABASE_URL`, `PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER`, `PGPASSWORD` y `PGPASSFILE`.

## Límites

Es una caracterización del recorrido HTTP→servicio de dominio real→repositorio/lector in-memory. El authenticator, repositorio y almacenamiento son sintéticos; el fake no demuestra grants reales, atomicidad/transacción PostgreSQL, adaptador/constraints, persistencia, reinicio ni almacenamiento de objetos. No se ejecutaron SQL/DDL, migraciones, consultas/conexiones a bases, Cloud, proveedores ni datos reales. Sin commit, push, integración a `main` o despliegue. Revisión independiente read-only: pendiente.
