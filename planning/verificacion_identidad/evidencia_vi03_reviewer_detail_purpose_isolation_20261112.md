# Evidencia VI-03 — aislamiento por propósito en detalle de revisor

- Fecha: 2026-11-12.
- Checkout: `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- Rama: `codex/marketplace/fix/kyc-06-save-identity-first`.
- HEAD: `274f37e483a078890e92382a159fa49fd23c2fe8`.
- Alcance: cerrar un límite de privacidad en el DTO HTTP del detalle de revisor; no alterar contrato persistente ni comportamiento de almacenamiento.

## Cambio

`ReviewerCasesHandler.loadDetail` serializa `identity_declaration` únicamente cuando el snapshot tiene propósito `identity`. `TestReviewerDetailDoesNotExposeIdentityDeclarationForSpaceCase` configura un fake que devuelve declaración sintética para un snapshot `space`, atraviesa el handler HTTP y verifica el conjunto exacto de cinco claves permitidas y ausencia de los cuatro valores privados. `TestAssignedIdentityDetailIncludesOnlyAllowlistedDeclaration` mantiene el control positivo identity y `TestReviewerListDoesNotExposeIdentityDeclaration` la omisión en lista. Como extensión de servicio, `TestReviewerSpaceDetailDoesNotReadIdentityDeclaration` usa un caso space sintético con asignación válida y lector que devolvería PII si se invocara; `LoadDetail` entrega solo el snapshot y el contador del lector permanece en cero.

## RED/GREEN y gates locales

- RED observado inicialmente: el test HTTP nuevo falló antes de la guarda y mostró en JSON nombre, apellido, RUT y fecha de nacimiento sintéticos para el caso `space`.
- GREEN focal con `-race`: los tres tests HTTP de detalle identity/space y lista pasaron; `TestReviewerSpaceDetailDoesNotReadIdentityDeclaration` pasó en el paquete de servicio.
- `go test -race ./internal/documentverification ./internal/documentverification/transport/http -count=1`: PASS.
- `go vet ./internal/documentverification ./internal/documentverification/transport/http`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS (compilación; no se ejecutó la API).
- `gofmt` sobre los archivos Go tocados y `git diff --check` focal: PASS.
- Las pruebas/build se ejecutaron con variables de conexión PostgreSQL/DATABASE/PGPASSFILE retiradas. No se buscó ni usó conexión a bases.
- Revisión independiente `deleg_820e00f8` validó las fronteras HTTP y de servicio; señaló como riesgo no confirmado que faltara `files` en el DTO exacto. Se verificó `ownerDTO` (`owner_cases.go:70`) inicializa `Files: []ownerFileDTO{}` y el campo no lleva `omitempty`; además, el test HTTP focal se repitió y pasó con el set exacto de cinco claves. No se necesitó cambiar código.

## Límites

La prueba es sintética y de handler/serializador: induce intencionalmente una combinación incoherente desde el fake para comprobar que la capa HTTP falla cerrada. No demuestra autenticación o asignaciones reales, el lector de declaraciones integrado, autorización en PostgreSQL, concurrencia, persistencia, almacenamiento ni acceso cloud. No se ejecutaron SQL/DDL, consultas, migraciones, Cloud, proveedor externo, navegador, despliegue, commit, push o integración a `main`. El worktree ya contenía numerosos cambios ajenos; se preservaron sin resetear ni limpiar.
