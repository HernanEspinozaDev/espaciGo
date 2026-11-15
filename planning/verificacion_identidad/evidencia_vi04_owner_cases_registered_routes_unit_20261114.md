# Evidencia — VI-04, montaje y despacho a través de rutas registradas de casos del titular

- **Fecha:** 2026-11-14.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron cambios ajenos.

## Alcance

En `internal/documentverification/transport/http/owner_cases_test.go`, se añade `TestRegisterOwnerCasesRoutesMountsAndDispatchesToMux` para verificar el correcto registro y enrutamiento en `*http.ServeMux` de las rutas de casos del titular:

1. `RegisterOwnerCasesRoutes(mux, handler)`: monta `ownerCasesRoute` (`/api/v1/document-verification/cases`) y `ownerCasesRoute+"/"`.
2. Despacho a través de `mux.ServeHTTP` de una solicitud de listado de colección (`GET /api/v1/document-verification/cases`) con bearer de sesión sintético: responde `200 OK` y verifica la deserialización del JSON conteniendo exactamente 1 elemento con ID, propósito (`space`) y estado (`borrador`).
3. Despacho a través de `mux.ServeHTTP` de una solicitud de detalle de caso (`GET /api/v1/document-verification/cases/{id}`) con bearer sintético: responde `200 OK`, deserializa el DTO de detalle comprobando ID, propósito, estado y la cabecera `ETag: \"3\"`.

Revisión source-only independiente `deleg_15f3ff01`: atendió su observación agregando aserciones explícitas sobre los cuerpos deserializados y la cabecera ETag.

Es test-only de caracterización: pasó en su primera ejecución válida; no hubo cambio productivo ni se afirma RED/GREEN.

## Verificación local

- Focal: `go test -race -count=20 -run '^TestRegisterOwnerCasesRoutesMountsAndDispatchesToMux$' ./internal/documentverification/transport/http` — PASS.
- Paquete completo: `go test -race -count=1 ./internal/documentverification/transport/http` — PASS.
- `go vet ./internal/documentverification/transport/http` — PASS.
- Cobertura de statements en `RegisterOwnerCasesRoutes`: 100,0% (cobertura total del paquete subió a 87,5%).
- `gofmt -d` y `git diff --check` — limpios.

## Límites

Pruebas unitarias HTTP en memoria con handlers reales y fakes sintéticos de servicio/autenticación. No demuestra transporte TCP/TLS en red, integración BFF→Go en producción ni persistencia PostgreSQL. No se ejecutó SQL/DDL ni se accedió a DB persistente, Cloud o datos reales. Sin commit, push, deploy o integración a `main`.
