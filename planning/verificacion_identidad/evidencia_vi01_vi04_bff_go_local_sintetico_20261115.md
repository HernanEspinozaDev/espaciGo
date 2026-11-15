# Evidencia VI-01/VI-04 — transporte local Next BFF → Go

- **Fecha:** 2026-11-15.
- **Checkout de tarea:** `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- **Rama/HEAD:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Checkout frontend:** `/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes/frontend`; worktree `frontend-next-hermes`, HEAD `cf6e28a365d2495e78bc64c01686ed55efb17c8a`. Ambos worktrees ya estaban dirty; se preservaron.

## Verificación ejecutada

Con `env -i` y solo `PATH`, `HOME`, `TMPDIR`, `ESPACIGO_FRONTEND_ROOT` y, en los casos indicados, `ESPACIGO_OWNER_SUBMIT_SCENARIO`:

1. `go test -race -tags=bffintegration -count=1 ./internal/documentverification/transport/http -run '^(TestNextBFFCreatesOwnerIdentityCaseThroughRegisteredGoRoute|TestNextBFFGetsOwnIdentityCaseThroughRegisteredGoRoute|TestNextBFFListsOwnIdentityCasesThroughRegisteredGoRoute|TestNextBFFSubmitsOwnerIdentityCaseThroughActualGoService|TestNextBFFUploadsOwnerIdentityThroughActualUploadService|TestOwnerHTTPUploadCompensatesSuccessfulPutAfterRequestCancellation)$' -v` — **6 PASS, 0 FAIL**, paquete 3.741 s.
2. El test `TestNextBFFSubmitsOwnerIdentityCaseThroughActualGoService` con `ESPACIGO_OWNER_SUBMIT_SCENARIO=stale` — **1 PASS**, paquete 1.563 s; el conflicto no ejecutó transición ni readback.
3. El mismo test con `ESPACIGO_OWNER_SUBMIT_SCENARIO=retry` — **1 PASS**, paquete 1.557 s; el replay inmediato no mutó el repositorio.
4. `go test -race -tags=bffintegration -count=1 ./internal/verification/transport/http -run '^TestNextBFFOwnIdentityDeclarationPOSTThenGET$' -v` — **1 PASS**, paquete 1.618 s; cubre guardado, lectura separada, consentimiento obsoleto y rechazo de Cookie.
5. `go vet -tags=bffintegration ./internal/documentverification/transport/http ./internal/verification/transport/http` — exit 0.

Total: nueve ejecuciones de pruebas PASS contando por separado los escenarios éxito, stale y retry del test de envío. Los subtests Node fueron ejecutados desde los scripts del checkout frontend.

## Límite de la evidencia

El export real de Next BFF se transpila en un directorio temporal y reenvía a un `httptest.NewServer` Go en loopback con rutas/handlers y servicios reales. Las URLs se validan como `http://127.0.0.1:<puerto>/api/v1`, sin credenciales, query ni fragment; el entorno hijo se limita explícitamente. Authenticator, repositorios, grants, scanner y storage son sintéticos o en memoria. No hubo consulta a PostgreSQL nativo/desechable/Cloud SQL, SQL/DDL, GCS, proveedor externo ni aplicación online. Esto valida contratos y forwarding BFF→Go, no autorización efectiva, persistencia, CAS durable, scanner real, storage durable o supervivencia a reinicio. No cierra los gates pendientes VI-01/VI-02.

No se modificó código productivo, ACL, esquema o recursos. No hubo commit, push, publicación, integración a `main` ni despliegue.
