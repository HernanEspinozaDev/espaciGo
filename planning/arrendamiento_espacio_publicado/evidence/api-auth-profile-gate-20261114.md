# Evidencia — Perfil de autenticación del flujo API de arriendo

- **Fecha:** 2026-11-14
- **Worktree:** `/home/nandev/Documentos/projects/espaciGo-rental-demo-reconciled`
- **Rama:** `codex/marketplace/feature/rental-contract-preview-adapters`
- **HEAD observado:** `aa53379db0da0e3b872ce23e8258becb324a2b4e` (cambios locales sin commit)
- **Alcance:** corregir la regresión de prueba del gate de autenticación y verificar el contrato OpenAPI. Solo entorno local; no se inició API ni se conectó a una base en esta continuación.

## Cambio

`cmd/api/commercial_rental_flow_test.go` ya no inyecta `authenticationEnabled=true` en la tabla de casos. Cada caso deriva ese valor mediante `authPrototypeEnabled` con su configuración: el caso local usa `LOCAL_AUTH_PROTOTYPE=1` y el caso de prueba usa realmente `APP_PROFILE=cloud-test`. Esto evita que el test etiquetado `cloud test authenticated runtime` pase sin configurar dicho perfil.

No se cambió el comportamiento productivo. `cmd/api/profile.go` habilita el autenticador prototipo solo con `LOCAL_AUTH_PROTOTYPE=1` o `APP_PROFILE=cloud-test`; `cmd/api/main.go` monta las rutas comerciales dentro de ese bloque y solo cuando `COMMERCIAL_RENTAL_FLOW_ENABLED=1` satisface el gate. Por ello, estas pruebas no prueban un autenticador de producción ni justifican habilitar el flujo comercial fuera de los perfiles de ensayo.

## Verificación ejecutada

- `go test ./cmd/api -run '^TestCommercialRentalFlowConfigurationGate$' -count=1`: pasó.
- `go test -race ./cmd/api -run '^TestCommercialRentalFlow' -count=1`: pasó después del cambio.
- `go vet ./cmd/api`: pasó después del cambio.
- `go test ./internal/openapicontract -run '^TestPublishedRentalContractsSeparatePublicAvailabilityFromAuthenticatedDraft$' -count=1`: pasó.
- `git diff --check`: pasó.
- También pasó la corrida race previa del conjunto `./cmd/api ./internal/rentalflow/... ./internal/adapters/postgres/rentalflow/... ./internal/adapters/postgres/identity/... ./internal/migrator`, con `DATABASE_URL`, `TEST_DATABASE_URL` y los flags opt-in de DB unset. Esa corrida fue anterior al cambio test-only; después del cambio se rerunearon los tests API focalizados indicados arriba.

## Límites y siguiente gate

- No se creó ni consultó una cuenta de ensayo y no se obtuvo una sesión sintética persistida. Por tanto, no se ejecutó el proceso API completo ni el recorrido autenticado; los tests actuales cubren el gate/configuración, el montaje HTTP y el handler con identidades sintéticas en sus capas respectivas.
- La suite OpenAPI focalizada está sincronizada con disponibilidad pública y cotización/preview autenticadas. La matriz generada de montajes del frontend permanece desfasada. No se modificó el worktree `frontend-next-hermes`, que conserva cambios ajenos.
- Antes de declarar listo el flujo para un despliegue autenticado, se necesita verificar un perfil de autenticación de producción y su autorización efectiva; `cloud-test` y `LOCAL_AUTH_PROTOTYPE` no son esa verificación. El recorrido visible local contra API compuesta sigue pendiente.
- No se accedió a PostgreSQL persistente ni a Cloud SQL; no hubo commit, push, integración, release o despliegue.

## Actualización posterior — matriz API y pruebas de continuación

- En este worktree de tarea, `node tooling/api-check.mjs` primero reprodujo una matriz de operaciones/montajes desfasada por cambios de línea en `cmd/api/main.go`; `node tooling/generate-api-inventory.mjs` regeneró únicamente `planning/frontend_next/evidence/matriz-operaciones-api-fase3.md` (216 operaciones). La repetición de `node tooling/api-check.mjs` pasó y confirmó sincronía de tipos OpenAPI generados, allowlist y matriz.
- El worktree `/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes` permaneció intacto; la regeneración se hizo en el worktree de tarea de arriendo. El gate confirma sincronía local de artefactos, no montaje/ruta en API en ejecución.
- Pasaron después `node --test tooling/rental-demo-*.test.mjs tooling/public-rental-availability.test.mjs tooling/public-rental-contract-preview.test.mjs tooling/rental-plan-e2e.test.mjs` (41/41), `go test -count=1 ./...`, `go vet ./...`, suite Python GCP con `/usr/bin/python3` (172/172) y `git diff --check`.
- No se inició la API, no se ejecutó navegador, no se usó PostgreSQL persistente/Cloud SQL y no hubo publicación o despliegue.

## Inspección estática del perfil de autenticación — 2026-11-14

- Se trazó la composición vigente de `cmd/api/main.go` y `cmd/api/profile.go`: `authPrototypeEnabled` acepta solo configuración local de prototipo o `APP_PROFILE=cloud-test`; rechaza `APP_PROFILE=production`. La autenticación, handlers `/api/v1/` y montaje comercial se crean únicamente dentro de `if prototypeEnabled` (`main.go:240-295`). El gate comercial exige ese mismo booleano (`commercial_rental_flow.go:18-38`).
- Conclusión: en este checkout no hay un perfil de autenticación de producción que habilite este montaje; no es solo una prueba de producto pendiente. El adaptador reusa `AuthenticationService`, pero eso no convierte `cloud-test`/prototipo en un perfil productivo ni acredita su autorización efectiva. No se inventó otro perfil ni se cambió el selector.
- Próximo gate separado: una decisión/implementación autorizada de autenticación productiva antes de pretender habilitar rutas comerciales para producción. Para el recorrido autenticado local, sigue faltando la política privada `.local/testing/test-account-policy.json`; no se creó ni alteró cuenta, sesión o datos.
- Este hallazgo es de lectura del código local, no verificación en API viva ni autorización de despliegue. Sin cambios de código en esta actualización.

## Adjudicación de revisión independiente — 2026-11-14

- La revisión read-only `deleg_ca2cff22` encontró que `APP_PROFILE=cloud-test` habilitaba el flujo mediante autenticación prototipo. Se convirtió el caso en regresión: la prueba falló primero con `enabled=true, err=nil` cuando debía rechazarse.
- `commercialRentalFlowEnabled` ahora rechaza cualquier `APP_PROFILE` no vacío antes de montar rutas; el `cloud-test` prototipo no puede habilitar este flujo. El modo local explícito `LOCAL_AUTH_PROTOTYPE=1` se conserva solo como ruta de ensayo; no se afirma que sea autenticación productiva ni apta para despliegue.
- El modo local exige además `HTTP_ADDR` explícito en una IP loopback, `DATABASE_CONNECTION_MODE=direct` y `DATABASE_HOST` loopback. Las regresiones nuevas observaron RED para listener externo, Cloud SQL y host DB no loopback; el paquete API pasó después del guard. Estos checks limitan exposición/target de red, pero no certifican que una base loopback sea descartable: no iniciar API hasta verificar y autorizar el target exacto.
- La re-revisión independiente read-only `deleg_8e215559` aprobó el gate y confirmó que `cloud-test`, listener no loopback y modo Cloud SQL no llegan al montaje de rutas. Hallazgo residual explícito: loopback/dirección directa demuestra localidad, no identidad ni aislamiento de la base; el gate no valida nombre, puerto, rol o instancia exclusiva. No hubo cambios ni pruebas en esa revisión; la autorización y preflight del target siguen siendo necesarios antes de iniciar API.
- Después del cambio pasaron `go test -race ./cmd/api -run '^TestCommercialRentalFlow' -count=1`, `env -u TEST_DATABASE_URL -u TEST_RENTALFLOW_POSTGRES_ISOLATED -u TEST_RENTALFLOW_POSTGRES_ISOLATION_TOKEN go test -race ./cmd/api -count=1`, `go vet ./cmd/api`, `gofmt -d` sobre los dos archivos modificados y `git diff --check` de esos archivos.
- No se inició el proceso API, no se creó cuenta ni sesión, y no se accedió a PostgreSQL persistente/Cloud SQL ni a la aplicación online. Autenticación productiva y recorrido visible local siguen pendientes.
