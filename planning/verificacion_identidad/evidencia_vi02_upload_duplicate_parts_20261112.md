# Evidencia — VI-02, multipart con partes duplicadas

- **Fecha:** 2026-11-12.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido, dirty; no se modificaron otros cambios.

## Alcance

`TestOwnerCaseUploadRejectsDuplicateFileAndRoleParts` envía tres escenarios multipart sintéticos al handler owner real: dos partes `file`; dos `evidence_role` (guard duplicado antes de exigir archivo); y un prefijo `file` + `evidence_role` válido seguido de un tercer `evidence_role`. Todos se rechazan con HTTP 400 antes de llamar al servicio fake de carga. Es test-only; primera ejecución esperada en verde como caracterización de los guards ya implementados (sin RED ni cambio productivo). Revisión independiente `deleg_a0720193` aprobó sin hallazgos; recomendó añadir el caso duplicado de role después de prefijo de upload válido, que se incorporó y el test focal se repitió PASS. El revisor rerun fue solo del focal; los demás gates constan como ejecución de esta sesión.

## Verificación local

- Test focal: PASS.
- `go test ./internal/documentverification/... -count=1`: PASS.
- `go test -race ./internal/documentverification/... -count=1`: PASS.
- `go vet ./internal/documentverification/... ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check, incluido archivo nuevo: PASS.
- Variables DB/PG retiradas de comandos Go.

## Límites

Caracteriza el parsing HTTP de duplicados con servicio fake; no prueba persistencia, propiedad/autorización integrada, CAS, inspección, PostgreSQL o storage. No acredita todos los posibles ataques multipart/streams ni límites de red. Sin SQL/DDL, bases persistentes, Cloud, proveedores o datos reales. Sin commit, push, integración a `main` o despliegue.
