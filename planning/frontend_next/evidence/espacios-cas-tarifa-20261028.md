# CAS de tarifa — contrato HTTP, PostgreSQL y frontend

Fecha: 2026-10-28. Repo `/home/nandev/Documentos/projects/espaciGo`, rama `main`, HEAD base `3956d2f3459b0bf4b15511f4e1a74ffd06a29975`; cambios sin commit.

## Alcance implementado

- `PUT /api/v1/spaces/{spaceId}/tariff` valida un único `If-Match` fuerte antes de decodificar el body: ausente `428`, malformado `400`, revisión obsoleta o de otro espacio `412`.
- El handler llama sólo `UpdateRateIfRevision`; nunca cae al método legacy no versionado. Tras éxito, devuelve el ETag calculado con la revisión retornada por el servicio.
- Servicio y repositorio de tarifas comparan la revisión esperada; la implementación PostgreSQL bloquea cuenta/espacio, compara antes del no-op y del write, devuelve revisión vigente sin cambio en el no-op y hace `spaceversion.Bump` junto al historial/tarifa en la transacción cuando cambia.
- OpenAPI exige `If-Match`, documenta `ETag` de respuesta y respuestas `400/412/428`. El adapter valida el ETag fuerte antes de hacer la llamada, lo envía y rechaza éxito sin ETag válido; el error `412` se propaga sin retry. El facade actualiza la revisión seleccionada sólo tras éxito y el editor envía el ETag actual.
- Los snapshots/versiones históricas de tarifas se mantienen; el test HTTP conserva conflicto con horario y CLP mínimo.

## Verificación local ejecutada

- Go: `go test -count=1 ./internal/spaces/transport/http -run '^(TestTariffPreconditions|TestPrivatePriceSimulationRoutesAndOpenAPISchemas)$' -v` PASS; `go vet ./internal/spaces/transport/http` PASS.
- Go: con variables opt-in de DB unset, `go test -count=1 ./internal/pricing ./internal/adapters/postgres/pricing ./internal/spaces/... ./internal/adapters/postgres/spaces` PASS; `go vet` del mismo alcance y `git diff --check` PASS.
- Frontend: `api:check` PASS (183 operaciones); `format:check`, `typecheck`, `lint`, suite Node PASS (268/268) y `next build` PASS.
- Adapter: pruebas de If-Match previo al request, ETag de éxito, ausencia de ETag de respuesta y `412` sin retry: 3 casos nuevos incluidos; archivo CAS 7/7 PASS. Facade: test nuevo confirma que el ETag seleccionado avanza sólo tras éxito y permanece igual tras 412.
- Playwright, con standalone local y mocks, test enfocado `navegación por pestañas` PASS (1/1); verifica que el PUT de tarifa use el ETag del espacio vigente y que el mock retorne/renueve ETag.
- Revisión independiente del handler Go: PASS, sin defectos concretos; señaló correctamente que el fake no demuestra transacción real.

## Límites / estado

La revisión independiente frontend/OpenAPI señaló inicialmente falta de pruebas CAS específicas; se cerró añadiendo 3 pruebas de adapter y una de facade (éxito renueva ETag; 412 lo conserva). Node total actualizado a 268/268. No se reejecutó review independiente después de este cierre. Los tests Go de repositorio no usaron PostgreSQL; la atomicidad, concurrencia y rollback del CAS siguen sin evidencia de una ejecución DB real. Playwright es mock UI y no demuestra browser→BFF→Go→PostgreSQL. No se ejecutó flujo autenticado, no se hicieron cambios de DB, migraciones, cloud, commit, push ni deploy. CAS de horarios y mutaciones restantes de galería siguen pendientes.
