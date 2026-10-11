# LOCAL-M04-COVER-RATE-01 — evidencia de implementación

Estado al 2026-10-11: Issue #256 está en revisión mediante el PR #260; el PR no está fusionado ni aceptado. #246 permanece abierta y LOCAL-1 no está cerrado.

## Criterios del corte

| Criterio | Implementación | Verificación |
|---|---|---|
| RQF-077: seleccionar y persistir portada propia | V49 añade `espacio.portada_foto_id` con FK compuesta a una imagen sintética del mismo espacio y titular. `PUT /api/v1/spaces/{spaceId}/cover` solo selecciona PNG activa propia. `GET` del espacio propio incluye la selección; el archivo sigue privado. Retirar la imagen o ejecutar la baja limpia la selección en la transacción correspondiente. | Integración PostgreSQL selecciona foto propia, rechaza foto ajena, comprueba persistencia y proyección de `GET`; retirar foto vuelve la referencia a `NULL`. Handler prueba ruta, forma JSON y CORS PUT. OpenAPI incorpora operación y campo nullable. Mock permite elegir una imagen de la galería y muestra la portada actual. |
| RQF-198: cambiar modalidad tarifaria | Las rutas de tarifa e historial aceptan espacios propios `borrador`, `activa` y `oculta`; escritura requiere rol anfitrión y titularidad. Un cambio efectivo crea versión nueva. El bloqueo cuenta→espacio compartido con horario serializa ambas escrituras. Horario semanal activo rechaza modalidad incompatible con 409 `tariff_calendar_conflict` antes de crear versión. Snapshots existentes no se recalculan. | Integración PostgreSQL usa API real: horario activo + cambio de `hora` a `dia` responde 409 y conserva una sola versión; al desactivarlo, modalidad cambia en publicación activa y luego oculta. Arrendatario recibe 403. Cotización/reserva preexistentes conservan modalidad `hora` e importe original. Prueba de handler comprueba el 409 y OpenAPI del precio. Mock describe el control de compatibilidad y reutiliza el selector de espacios propios. |

## Comprobaciones ejecutadas

Se usaron bases PostgreSQL desechables creadas por el script del proyecto; no se aplicó V49 a `espacigo_pgdata` ni se alteraron sus secretos o datos.

```text
GO_TEST_RUN='^TestLocalPublicationRequiresEffectiveKYCAndRecordsOwnerTransitions$' bash scripts/test-local-comm-mailpit.sh ./internal/adapters/postgres/spaces
PASS TestLocalPublicationRequiresEffectiveKYCAndRecordsOwnerTransitions

GO_TEST_RUN='^TestSyntheticGalleryOwnerLimitIdempotencyAndRecoverableCleanup$' bash scripts/test-local-comm-mailpit.sh ./internal/adapters/postgres/gallery
PASS TestSyntheticGalleryOwnerLimitIdempotencyAndRecoverableCleanup

go test ./internal/gallery/transport/http ./internal/spaces/transport/http ./internal/pricing -count=1
PASS

go test ./internal/gallery/transport/http ./internal/spaces/transport/http ./internal/pricing ./internal/openapicontract -count=1
PASS

npm --prefix mock run build
PASS

npm --prefix mock run test:gallery-cover
5 passed, 0 failed (3 portada + 2 protecciones de sesión/selección)

node --test mock/test/gallery-session-state.test.mjs mock/test/space-publication-state.test.mjs
3 passed, 0 failed

git diff --check
PASS

```

La regresión integral LOCAL-1 queda reservada hasta cerrar las brechas locales planificadas. No se ejecutó GCP.

## Decisiones y límites

- La portada seleccionada es una referencia al PNG privado sintético, no publica la imagen ni incluye coordenadas, dirección o URL pública en el catálogo.
- Horario semanal solo es compatible con tarifa horaria. El dueño debe desactivar el horario antes de cambiar a día o mes; tarifas diaria/mensual conservan sus reglas existentes.
- La versión actual de tarifa cambia para operaciones futuras; snapshots de cotizaciones y reservas se mantienen intactos.
- Eliminación de publicación (RQF-091/092) no forma parte de este PR. La política local de conservación se ratificó en #258; su implementación sigue pendiente para ese corte.
- La galería del mock vuelve a consultar el detalle del espacio (portada persistida) y las imágenes tras agregar o retirar una imagen, y refresca el listado propio. La respuesta solo se aplica si siguen vigentes sesión, selección y revisión de la galería.
- Pruebas enfocadas: elegir A y agregar B conserva A; retirar B conserva A; retirar A elimina la portada en galería y listado. `npm --prefix mock run test:gallery-cover` compila TypeScript y ejecuta estos casos más las protecciones existentes de sesión/selección.

## Aceptación y comprobación posterior al merge

El propietario aceptó #256 por RQF-077 y RQF-198. PR #260 se fusionó en `main` como `38c3e1300b6d3f843238f4ee6b8758bfe3b71913`; #256 quedó cerrada y Hecho únicamente por esos criterios. Después se sincronizó `main` y `./scripts/dev-env.sh up` aplicó V49 incrementalmente. La comprobación local por API usó un borrador propio: generó/seleccionó la portada y consultó su persistencia; al retirar el PNG, la portada quedó vacía. Cambió y restauró la modalidad con versiones nuevas; no había snapshots de reserva en ese borrador. No se borraron datos, volumen ni secretos. Health/readiness/HTTP fueron reutilizados de la evidencia ya vigente. #246 y los pendientes generales M04/M05 permanecen abiertos.
