# VI-04 — aprobación ligada a la declaración legal vigente

- **Fecha:** 2026-11-12.
- **Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`, rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Alcance:** solo código Go y pruebas unitarias con `pgx.Tx` sintético; sin conexión a PostgreSQL.

## Cambio

La aprobación manual deja de ser efectiva si la cuenta guarda una nueva revisión legal. `identityCurrent`, usado al decidir un caso de espacio, y `Repository.CurrentForAccount`, usado por el gate de reserva y la lectura de estado, ahora exigen que `document_verification_case.identity_declaration_revision_id` coincida con la revisión de `identity_declaration_revision` más reciente de ese titular. Un expediente pendiente ligado a la revisión nueva no satisface el filtro de estados terminales ni convierte una aprobación vieja en vigente; el servicio consumidor falla cerrado cuando no hay una aprobación terminal ligada a la revisión actual.

La prueba `current_identity_declaration_revision_test.go` usa transacciones/filas fake y estados sintéticos para cada camino: acepta aprobación cuya revisión coincide y rechaza la revisión vieja después de guardar otra. Es una prueba de contrato del adaptador y del manejo de filas sin ejecutar la consulta en PostgreSQL.

## TDD y gates

- RED observado antes del cambio: los dos casos negativos fallaron porque `identityCurrent` conservaba la aprobación y `CurrentForAccount` devolvía `aprobada` sin comprobar la revisión legal.
- GREEN: cuatro pruebas de aceptación/rechazo (dos caminos × coincidencia/desfase) pasan bajo `go test -race`.
- `go test -race ./internal/identityverification ./internal/documentverification -count=1`: PASS.
- `go vet ./internal/adapters/postgres/documentverification`, `gofmt -d` en los dos archivos tocados y `git diff --check`: PASS.
- No ejecuté `TestLocalIdentityCurrentDoesNotResurrectPriorApproval`: es un ensayo opt-in que abre una base; la prohibición vigente excluye ese acceso.

## Límites

El fake confirma que el adaptador construye la comparación titular→última revisión y cómo trata una fila ausente; no valida sintaxis/semántica SQL real, constraints/triggers, locks ni carreras frente a un guardado concurrente. No ejecuté SQL/DDL, pruebas conectadas, migraciones, ni toqué bases persistentes o Cloud. Sin datos/cuentas reales, commit, push, despliegue o integración a `main`. La aceptación completa de VI-04 sigue abierta hasta probar estas propiedades en un entorno sintético de PostgreSQL autorizado por separado.
