# LOCAL-M04-PUB-01 — evidencia de publicación local

Fecha: 2026-10-08  
Issue: [#204](https://github.com/HernanEspinozaDev/espaciGo/issues/204) (hija de #55)  
Base: `main` en `ca64f11860dfd8c60e3998cc2754b3b4915ece26`; implementación en `codex/local-m04-publish-kyc`, PR [#205](https://github.com/HernanEspinozaDev/espaciGo/pull/205), commit inicial `6e84c89a2ec1fd1b57ad38f841535a3607b8e2b3` (autor HernanMEC).

## Decisión y límite

El titular con rol `arrendador` puede cambiar un espacio propio `borrador → activa`, `oculta → activa` y `activa → oculta`. Activar exige KYC sintético local efectivo; KYB no lo sustituye. El rol y la elegibilidad son controles independientes. Esta bandera local no agrega el espacio al catálogo general, no cancela ni cambia reservas existentes y no resuelve M04 ni #142.

La actualización de publicación toma el bloqueo de la cuenta activa, bloquea el espacio propio, lee la elegibilidad efectiva mediante la consulta generada `ListSyntheticEligibility` y registra cambio e historial en una transacción. Una revocación/baja que use el mismo bloqueo se serializa con la operación. Repetir el estado actual es idempotente y no inserta otro evento. El historial local es append-only.

## Evidencia ejecutada

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
