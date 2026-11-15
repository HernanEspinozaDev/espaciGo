# Evidencia VI-04 — guard de scope owner en serialización HTTP (2026-11-12)

- **Estado:** subcorte local completado; VI-04 continúa parcial.
- **Backend:** `/home/nandev/Documentos/projects/espaciGo-main-sync`, rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Alcance del cambio:** `internal/documentverification/transport/http/owner_cases.go`, `owner_cases_test.go`; ajuste sintético de fixture en `owner_decision_mapping_test.go`. Los dos primeros archivos ya tenían cambios locales anteriores; se preservaron y esta evidencia no les atribuye el diff previo.

## Comportamiento

`ownerSnapshotDTO` vuelve a comprobar el scope antes de convertir un snapshot de servicio en DTO owner: actor autenticado no vacío y coincidente con `SubjectID`, UUID válido para el caso, coincidencia exacta con el ID solicitado cuando la ruta lo proporciona, propósito admitido (`identity` o `space`) y forma de scope correspondiente (identity sin `SpaceID`; space con `SpaceID` UUID válido). El rechazo usa el mapeo existente a 404 genérico. `ownerDTO` no cambió y permanece disponible para el flujo reviewer.

La comprobación se aplica al DTO de alta, a cada elemento de la lista antes de escribirla, al detalle GET, al snapshot previo al submit/replay y al readback posterior al submit. Una lista inválida se descarta completa; un snapshot inicial de submit fuera de scope detiene la mutación. Si el readback posterior a `Submit` no coincide, no se serializa DTO ni ETag, pero este test fake no demuestra rollback de una transición ya invocada. Tampoco se afirma que un 404 posterior a `Start` revierta una creación.

## Pruebas y TDD

- `TestOwnerSnapshotScopeMismatchFailsClosedAtHTTPBoundary` invoca el `OwnerCasesHandler` real con autenticación y servicio sintéticos. Incluye actor distinto, ID distinto al solicitado, propósito no admitido, `SpaceID` inválido, identity con `SpaceID`, ID de caso malformado en lista, rechazo de lista completa y submit que no alcanza `Submit` cuando el snapshot inicial está fuera de scope.
- Se observaron fallos RED intencionales antes de los guards: el handler devolvía 200 con ETag/DTO para snapshots de actor, lista y submit no coincidentes. Después de la implementación, el focal race pasó.
- En una extensión parent-owned, el caso de ID malformado en lista produjo RED observable: 200 y el UUID malformado en el JSON. Tras validar el UUID retornado, el focal pasó. Las pruebas adicionales de alta, readback de submit y actor autenticado ausente son caracterizaciones de las llamadas existentes al guard; no se presentan como RED/GREEN individual.
- El primer paquete HTTP después del guard reveló que el caso de mapeo `space_reason_is_not_exposed` usaba un snapshot `PurposeSpace` sin `SpaceID`. Se corrigió únicamente ese fixture con un UUID sintético válido; la regla de scope no se relajó.
- Revisión source-only independiente `deleg_958eae6c`: `passed=true`, sin concerns de seguridad ni errores lógicos. Sugirió mantener separados los gates de autorización persistida e incluir cobertura del actor ausente/ID inválido; el test final incluye actor ausente e ID inválido en lista. El revisor no ejecutó pruebas.

Comandos parent-owned, ejecutados con `env -i` sin configuración/variables de DB:

- Focal owner con `-race -count=5` (mismatch, actor ausente, alta y readback de submit) — PASS.
- `go test -race -count=1 ./internal/documentverification ./internal/documentverification/transport/http` — PASS.
- `go vet ./internal/documentverification ./internal/documentverification/transport/http` — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt -d`, `git diff --check` y comprobación explícita de whitespace para los tres archivos — PASS.

## Límites

Es defensa en profundidad en la frontera HTTP con fakes: no acredita autenticación real, grants, ownership efectivo de espacios, predicados del repositorio, SQL/PostgreSQL, persistencia, CAS/atomicidad, runtime Next/BFF, storage, scanner ni Cloud; tampoco prueba una vulnerabilidad en el adapter real. No se ejecutaron SQL/DDL, no se conectó a base persistente ni Cloud, y no se usaron datos o credenciales reales. Sin commit, push, deploy o integración a `main`.
