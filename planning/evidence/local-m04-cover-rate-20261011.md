# LOCAL-M04-COVER-RATE-01 — evidencia de implementación

Estado al 2026-10-11: PR #256 preparado para revisión; no fusionado ni aceptado. #246 permanece abierta y LOCAL-1 no está cerrado.

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
- Eliminación de publicación (RQF-091/092) no forma parte de este PR. La propuesta concreta de conservación histórica está en la Issue #258 y espera ratificación antes de implementar.
