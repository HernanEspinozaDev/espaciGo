# Evidencia VI-04 — GET de colección owner identity BFF→Go local (2026-11-12)

- **Estado:** caracterización test-only ejecutada y verificada por el proceso principal; gates locales PASS. Revisión independiente final source-only `deleg_178ae6f7` aprobada (`passed=true`, sin concerns, errores lógicos ni sugerencias); confirmó que los sentinels añadidos coinciden con valores poblados por el fake y no pertenecen al DTO owner permitido.
- **Backend:** `/home/nandev/Documentos/projects/espaciGo-main-sync`, rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Frontend autorizado:** `/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes/frontend`, rama `frontend-next-hermes`, HEAD `cf6e28a365d2495e78bc64c01686ed55efb17c8a`.
- **Archivos nuevos:** `internal/documentverification/transport/http/identity_owner_list_bff_go_integration_test.go` y `frontend/tooling/identity-owner-list-bff-go-integration.mjs`. El único delta de revisión posterior fue agregar aserciones de ausencia para valores privados sintéticos; no se modificó producción ni otros archivos de la tarea.

## Recorrido caracterizado

El test Go con build tag `bffintegration` inicia un `httptest.NewServer` con `RegisterOwnerCasesRoutes`, `OwnerCasesHandler` y `documentverification.Service.ListOwnCases` reales. La autenticación y el repositorio permanecen sintéticos/en memoria. Un proceso Node transpila e invoca el export `GET` real de Next y la operación generada `listOwnDocumentVerificationCases`; la configuración upstream y los imports se sustituyen por módulos temporales de la prueba. La solicitud BFF llega por loopback a la ruta Go owner registrada.

El caso verifica exactamente una llamada upstream `GET /api/v1/document-verification/cases`, Bearer sintético y ausencia de Cookie, Content-Type y body. Comprueba una autorización con actividad `UserOperation`, el actor sintético propagado a una sola llamada `ListOwnCases` y cero llamadas a `LoadOwnCase`, `CreateCase` o `Transition`. El repositorio fake devuelve una sola snapshot identity con un archivo sintético únicamente para el actor esperado.

La respuesta exige 200, una lista con un caso, claves allowlisted exactas para el DTO owner y para su DTO de archivo, valores esperados de identidad/estado/revisión/formato/copy, y ausencia de campos y valores centinela privados de actor, reviewer, revisión de declaración, consentimiento, fecha de consentimiento, vencimiento y motivo interno. También verifica `Cache-Control: private, no-store`, `Vary: Authorization` y ausencia de `Set-Cookie`.

## Pruebas parent-owned

- Focal final repetido: `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" ESPACIGO_FRONTEND_ROOT=/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes/frontend go test -race -tags=bffintegration -count=5 ./internal/documentverification/transport/http -run '^TestNextBFFListsOwnIdentityCasesThroughRegisteredGoRoute$' -v` — PASS 5/5; el Node harness reportó 1/1 en cada ejecución.
- Suites tagged: `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" ESPACIGO_FRONTEND_ROOT=/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes/frontend go test -race -tags=bffintegration -count=1 ./internal/documentverification ./internal/documentverification/transport/http` — PASS.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go vet -tags=bffintegration ./internal/documentverification ./internal/documentverification/transport/http` — PASS.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go build -o /dev/null ./cmd/api` — PASS.
- En el frontend autorizado: `node --check tooling/identity-owner-list-bff-go-integration.mjs` y `./node_modules/.bin/prettier --check tooling/identity-owner-list-bff-go-integration.mjs` — PASS.
- `gofmt -d` del test Go y comprobación explícita de whitespace en los dos archivos nuevos — PASS.
- Después de ejecutar los gates se actualizaron únicamente el plan y esta evidencia; tras esos cambios documentales se repitieron `git diff --check` y el escaneo explícito de whitespace, sin volver a ejecutar pruebas de código.

La primera ejecución de la caracterización también pasó. Este es un test de comportamiento existente; no se afirma RED/GREEN ni cambio correctivo de producción. La primera invocación directa de Node sin `ESPACIGO_TEST_API_BASE_URL` falló por omitir el entorno que normalmente instala el runner Go; el comando previsto Go→Node, que proporciona la URL de `httptest`, pasó y fue el que se verificó repetidamente.

## Límites

La prueba ejecuta el export de ruta Next transpilado y la operación generada con imports/configuración reemplazados por módulos temporales; no demuestra build/resolución normal de Next, middleware ni runtime standalone. No recorre UI ni adapter cliente. Authenticator y repositorio son fakes: el actor ligado en el fake no demuestra autenticación/grants efectivos, ownership SQL, autorización persistida o comportamiento de PostgreSQL. No demuestra persistencia/reinicio, concurrencia/atomicidad, scanner, storage/GCS ni Cloud. Todo el contenido es sintético y local. No se ejecutó SQL/DDL, no se accedió a bases persistentes ni Cloud, y no hubo commit, push, deploy o integración a `main`. VI-04 y el MVP permanecen parciales.

## Siguiente paso

Tras registrar la re-revisión independiente del árbol final, reauditar el plan y las fuentes actuales para elegir la siguiente brecha local VI-03+; esta caracterización no cierra los gates de autorización ni persistencia.
