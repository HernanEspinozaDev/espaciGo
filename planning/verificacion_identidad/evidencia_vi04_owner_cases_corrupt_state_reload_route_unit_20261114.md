# Evidencia — VI-04, recarga tras envío, manejo de estado inconsistente y fronteras de ruteo en casos del titular

- **Fecha:** 2026-11-14.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e0c0344d567bb44cf96ea4f` dirty.
- **Alcance:** Caracterización unitaria exhaustiva de fallos en pre-verificación de envío, recarga de snapshot tras mutación exitosa en `submit`, snapshots con inconsistencias de pertenencia/actor en `create`, `list` y `submit`, ruteo fail-closed ante prefijos no conformes y validaciones directas de frontera en `ownerSnapshotDTO` en `internal/documentverification/transport/http/owner_cases.go`.
- **Pruebas implementadas:** `TestOwnerCasesHandlerCorruptStateReloadAndRouteBoundaries` en `internal/documentverification/transport/http/owner_cases_test.go`.

## Verificaciones cubiertas

1. **Fallo en pre-comprobación de `submit`:** Si `LoadOwnCase` retorna error en el precheck previo a `Submit` (ej. `ErrNotFound`), el manejador responde inmediatamente fail-closed `404 Not Found` sin invocar la mutación.
2. **Snapshot corrupto en pre-comprobación de `submit`:** Si `LoadOwnCase` devuelve un caso cuyo actor no coincide con el sujeto (`SubjectID != actor`), `ownerSnapshotDTO` retorna `ErrNotFound` y el manejador devuelve `404 Not Found`.
3. **Fallo en recarga tras mutación exitosa:** Cuando `Submit` muta exitosamente el caso pero la recarga posterior `LoadOwnCase` falla (simulada vía cola secuencial `loadQueue`), el error se propaga vía `ownerError` devolviendo `404 Not Found`.
4. **Snapshot corrupto en recarga post-mutación:** Si la recarga posterior a `Submit` devuelve un snapshot inconsistente, `ownerSnapshotDTO` falla y el manejador responde `404 Not Found`.
5. **Creación con snapshot inconsistente:** Si `Start` retorna un snapshot con discrepancia de sujeto, la respuesta es `404 Not Found`.
6. **Listado con snapshot inconsistente:** Si `ListOwnCases` retorna elementos con discrepancia de sujeto, el manejador responde fail-closed `404 Not Found`.
7. **Prefijo de ruta inválido:** Peticiones a rutas fuera del prefijo `/cases` (ej. `/other-cases`) son rechazadas inmediatamente por `parseOwnerCasesRoute` devolviendo `404 Not Found`.
8. **Fronteras directas de `ownerSnapshotDTO`:** Validación directa de rechazo (`ErrNotFound`) ante actor vacío, identificador UUID malformado o identificador solicitado no coincidente.

## Resultados de ejecución local

- `go test -race -count=20 -run '^TestOwnerCasesHandlerCorruptStateReloadAndRouteBoundaries$' ./internal/documentverification/transport/http`: PASS.
- `go test -race -count=1 ./internal/documentverification/transport/http`: PASS.
- `go vet ./internal/documentverification/transport/http`: PASS.
- Cobertura de statements en `ownerSnapshotDTO`: alcanzó el **100,0%**.
- Cobertura de statements en `parseOwnerCasesRoute`: alcanzó el **100,0%**.
- Cobertura de statements en `create`: alcanzó el **100,0%**.
- Cobertura de statements en `list`: alcanzó el **100,0%**.
- Cobertura de statements en `load`: alcanzó el **100,0%**.
- Cobertura de statements en `submit`: alcanzó el **100,0%**.
- Cobertura total del paquete `./internal/documentverification/transport/http`: alcanzó el **97,7%**.

## Límites estrictos

- Entorno puramente unitario con fakes sintéticos en memoria.
- Sin llamadas SQL/DDL, sin acceso a bases persistentes ni Cloud, sin commit, push, deploy ni integración a `main`.
