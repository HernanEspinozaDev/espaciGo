# LOCAL-M04-PUB-01 — evidencia de publicación local

Fecha: 2026-10-08  
Issue: [#204](https://github.com/HernanEspinozaDev/espaciGo/issues/204) (hija de #55)  
Base: `main` en `ca64f11860dfd8c60e3998cc2754b3b4915ece26`; implementación en `codex/local-m04-publish-kyc`, PR [#205](https://github.com/HernanEspinozaDev/espaciGo/pull/205), commit inicial `6e84c89a2ec1fd1b57ad38f841535a3607b8e2b3` (autor HernanMEC).

## Decisión y límite

El titular con rol `arrendador` puede cambiar un espacio propio `borrador → activa`, `oculta → activa` y `activa → oculta`. Activar exige KYC sintético local efectivo; KYB no lo sustituye. El rol y la elegibilidad son controles independientes. Esta bandera local no agrega el espacio al catálogo general, no cancela ni cambia reservas existentes y no resuelve M04 ni #142.

La actualización de publicación toma el bloqueo de la cuenta activa, bloquea el espacio propio, lee la elegibilidad efectiva mediante la consulta generada `ListSyntheticEligibility` y registra cambio e historial en una transacción. Una revocación/baja que use el mismo bloqueo se serializa con la operación. Repetir el estado actual es idempotente y no inserta otro evento. El historial local es append-only.

## Evidencia ejecutada

### Correcciones en revisión para PR #205

- La transición de publicación toma bloqueo transaccional de la fila del fixture y rechaza con 409 `fixture_enabled` si está habilitado. El estado del espacio no cambia. No se amplía el catálogo: los lectores M05 y las operaciones de cotización/reserva conservan el filtro existente `estado='borrador'`.
- `TestLocalPublicationRequiresEffectiveKYCAndRecordsOwnerTransitions` añade un fixture habilitado, verifica el conflicto y estado borrador intacto, y completa listado de catálogo, cotización y reserva fake después del rechazo.
- `TestOwnerExportContainsOnlyOwnOrderedPublicationHistory` abre los ZIP reales de dos titulares y verifica orden temporal/secuencial, correspondencia exclusiva al propietario y actor minimizado como `self`.
- `TestPublicationRouteRequiresLandlordAndReturnsState` comprueba el contrato HTTP 409 `fixture_enabled`, `request_id` y `X-Request-ID`.
- La clasificación de privacidad incluye el historial de publicación propio en el ZIP y deja su plazo independiente pendiente, sin atribuirle el de auditoría.
- Ejecución focalizada real: `GO_TEST_RUN='TestLocalPublicationRequiresEffectiveKYCAndRecordsOwnerTransitions|TestOwnerExportContainsOnlyOwnOrderedPublicationHistory' bash scripts/test-m04-attributes-postgres.sh ./internal/adapters/postgres/spaces ./internal/m02local` — PASS. El script creó PostGIS con almacenamiento efímero, migró ambas bases desde cero y las retiró al terminar; la integración usó el rol `espacigo_runtime` donde corresponde. No se conectó al volumen local persistente.
- `go test ./internal/spaces/... ./internal/adapters/postgres/spaces ./internal/m02local` — PASS (pruebas unitarias; integraciones con dependencia ambiental omitidas por ausencia de `TEST_DATABASE_URL`, y ejecutadas por separado con el comando desechable anterior).
- `go vet ./internal/spaces/... ./internal/adapters/postgres/spaces ./internal/m02local` y `git diff --check` — PASS.

## Aceptación posterior al merge — 2026-10-08

- PR #205 fusionado en `main` (`545a8a9c2b08a0f7183d6d816d43504c185a1912`). Se actualizó `main` por fast-forward y `bash scripts/dev-env.sh up` aplicó V31 de forma incremental. La consulta a `schema_migrations` confirmó V30 y V31; `bash scripts/dev-env.sh verify-http` pasó para backend, PostgreSQL y mock. Volumen `espacigo_pgdata`, secretos y datos previos se conservaron.
- Mediante la API local se creó un espacio propio sin entrada en `reserva_ensayo_local_fixture`. Sin elegibilidad KYC, activar devolvió 409 `eligibility_required` y el espacio siguió en borrador. Una aprobación KYC sintética separada, otorgada por el administrador sintético y sin alterar roles, habilitó el recorrido `borrador → activa → oculta → activa`; cada estado se consultó con el endpoint autenticado de espacio propio.
- Se descargó la respuesta ZIP de la API y se abrió el JSON del archivo. `space_publication_history` incluyó los tres eventos de ese espacio en orden y únicamente con `actor: self`.
- La fila de fixture que conserva el estado privado de credenciales no pertenecía al anfitrión de este recorrido; se evitó usarla como comprobación autenticada. El conflicto de fixture habilitado y la continuidad de catálogo/cotización/reserva se aceptan con la integración PostgreSQL desechable publicada en la sección anterior: `TestLocalPublicationRequiresEffectiveKYCAndRecordsOwnerTransitions` pasó antes del merge y prueba esos cuatro resultados juntos. No se repitió contra el volumen de desarrollo.
- Este resultado acepta solo LOCAL-M04-PUB-01 (#204). M04 general, catálogo público, galería, edición de anuncios y los padres #52–#61 permanecen abiertos. Sin GCP.

- `GO_TEST_RUN='TestLocalPublicationRequiresEffectiveKYCAndRecordsOwnerTransitions' bash scripts/test-m04-attributes-postgres.sh ./internal/adapters/postgres/spaces` — PASS en PostgreSQL/PostGIS desechable, migrando desde cero y ejecutando el repositorio con `espacigo_runtime`. Incluyó: sin elegibilidad rechaza; KYB pendiente no reemplaza KYC; cuenta ajena obtiene not-found; KYC efectivo permite activar; dos publicaciones concurrentes generan una sola transición; ocultar/reactivar funciona; revocar KYC impide una nueva activación. Se comprobó historial de tres transiciones. El contenedor y su base temporal fueron retirados por el script.
- `go test ./internal/spaces/... ./internal/adapters/postgres/spaces ./internal/dbbootstrap` — PASS para paquetes locales y handler. Sin `TEST_DATABASE_URL` el caso de integración etiqueta su skip; el comando aislado anterior sí lo ejecutó realmente.
- `npm --prefix mock run build` — PASS (TypeScript compilado).
- `npm --prefix mock run test:publication-state` — PASS (1 prueba): arrendatario no ve acción; arrendador recibe publicar/ocultar para estados válidos.
- `go vet ./internal/spaces/... ./internal/adapters/postgres/spaces ./internal/dbbootstrap` — PASS.
- `git diff --check` — PASS.
- La prueba HTTP verifica rol `arrendador`, respuestas activa/oculta contra el esquema OpenAPI `SpaceDraft` y el error 409 `eligibility_required` con `request_id` coincidente con `X-Request-ID`.

## Implementación agrupada

- V31 incremental amplía el estado del espacio y crea historial append-only sin cambiar datos existentes ni agregar una tabla de calendario.
- Backend/PostgreSQL usa elegibilidad KYC efectiva, owner-scope y bloqueo de cuenta.
- API `PUT /api/v1/spaces/{spaceId}/publication`; respuestas y errores están declarados en `planning/openapi.yaml`.
- El mock de anfitrión muestra acciones de publicar/ocultar; arrendatario no recibe esas acciones. Resultados tardíos de lectura/acción se descartan al cambiar sesión. El editor continúa disponible solo para borradores.

## Pendientes

- Este corte no se ha fusionado ni aplicado al volumen persistente. No se ejecutó recorrido de navegador del nuevo endpoint.
- No expone ofertas en catálogo público/general; la búsqueda autorizada de fixtures M05 permanece separada.
- Edición de ofertas activas, galerías, eliminación comercial, reglas generales de tarifas/políticas, #142 proveedor/datos reales y los criterios padres #52–#61 siguen pendientes.
- No se ejecutó ni preparó trabajo GCP.
