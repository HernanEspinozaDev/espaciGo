# VI-04 — reintento idempotente al iniciar caso identity

- **Fecha:** 2026-11-12.
- **Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`, rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Alcance:** crear/reintentar caso identity con una declaración vigente; tests unitarios con `pgx.Tx` fake. Preservé el resto de cambios locales.

## Cambio y sensibilidad

`Repository.CreateCase` busca, bajo la transacción serializable y el lock ya usado para el titular, un caso identity activo del mismo titular y revisión de declaración. Si coincide también el formato declarado y la versión de consentimiento, carga sus metadatos de archivos y devuelve el caso existente sin insertar otra fila/evento. Si el formato del reintento difiere, devuelve `ErrConflict` sin carga ni escrituras. La primera creación conserva el camino de inserción.

RED observado antes del cambio: `TestCreateCaseRetryReturnsExistingActiveIdentityCase` terminó con exit code 1; el diagnóstico detallado de esa ejecución no quedó preservado, así que no atribuyo una causa más específica. GREEN: reintento, primer alta, incompatibilidad de formato y casos relacionados de declaración/revisión pasan con fakes.

## Gates ejecutados

- Suite focal `go test -race ./internal/adapters/postgres/documentverification -run ... -count=1`, incluyendo los nuevos tests de reintento, el control de primera creación, la lectura de declaración y los tests de revisión de declaración más reciente: PASS.
- `go test -race ./internal/identityverification ./internal/documentverification -count=1`: PASS.
- `go vet ./internal/adapters/postgres/documentverification` y `go build -o /dev/null ./cmd/api`: PASS.
- `gofmt -d` en los archivos Go tocados y `git diff --check`: PASS.

No ejecuté la suite completa del adaptador ni `TestLocalIdentityCurrentDoesNotResurrectPriorApproval`, que requiere DB. Las pruebas se corrieron con entorno `env -i`, sin variables DB ni `PGPASSFILE`; no ejecuté SQL/DDL, migraciones ni consultas, tampoco accedí a bases persistentes o Cloud.

## Límites

La prueba es un contrato con filas/tx sintéticas: no prueba la semántica real del índice único, transacciones, reintentos concurrentes ni persistencia. Cubre el inicio de caso; los reintentos de envío/decisión y la concurrencia entre revisores siguen abiertos. La respuesta HTTP mantiene el mismo recurso/estado, pero el fake no verifica el recorrido HTTP. La aceptación completa de VI-04 sigue parcial.
