# Formularios dinámicos: soporte de enum_list

**Fecha:** 2026-10-27  
**Estado:** rebanada UI local verificada; F3 continúa abierta.

## Contrato comprobado

El perfil local `oficina` v1, leído en transacción PostgreSQL `READ ONLY` y documentado en `postgres-local-readonly-profiles.md`, incluye `climatizacion` de tipo `enum_list` con opciones exactas `aire_acondicionado`, `calefaccion`, `ventilacion_mecanica`. No se fijó una versión v2 ni se cambió la base.

## Implementación

- `listing-wizard.tsx`: representa `enum_list` como un grupo accesible de checkboxes; cada opción tiene etiqueta asociada, el estado se guarda como arreglo limitado a las opciones del perfil y se serializa en el orden del esquema.
- `listing-editor.tsx`: permite cargar, modificar y guardar el mismo tipo de atributo conservando la versión del perfil y el orden de opciones.
- Los demás tipos de atributo mantienen su renderer existente.

## Verificación ejecutada

Desde `frontend/`, con Node/pnpm del proyecto:

- Build de producción local `pnpm build`: PASS.
- Prueba E2E del wizard `pnpm exec playwright test tests/phase7-host-listings.spec.ts --grep 'asistente en 5 pasos'`: PASS (1/1); verifica `puestos_trabajo=6`, `wifi=true`, `climatizacion=["calefaccion"]` y `attribute_schema_version=1` en la solicitud de creación.
- Prueba E2E del perfil `estacionamiento` v1: PASS (1/1); fixture derivado del perfil local leído en transacción `READ ONLY`; comprueba los tres controles numéricos y sus límites, las opciones de `vehiculos_admitidos` `enum_list`, opciones del enum `tipo_acceso`, controles booleanos y ausencia de POST durante el renderizado.
- Prueba E2E de edición `pnpm exec playwright test tests/phase7-host-listings.spec.ts --grep 'navegación por pestañas'`: PASS (1/1); carga `calefaccion`, añade `ventilacion_mecanica` y verifica `climatizacion=["calefaccion","ventilacion_mecanica"]` junto con `attribute_schema_version=1` en el PUT.
- Fases Playwright 6 y 7: PASS (16/16), incluyendo pruebas Axe y viewports.
- `pnpm typecheck`, `pnpm lint`, `pnpm format:check`, `pnpm test` (260/260) y `pnpm build`: PASS.

Los E2E usan el servidor local Next y mocks de API/fixtures sintéticos. Comprueban el comportamiento visible y los payloads del BFF, **no** demuestran persistencia real ni la ruta browser→BFF→Go→PostgreSQL. No se hicieron escrituras en PostgreSQL. Los perfiles restantes, los recorridos con Go real y las otras puertas de salida de F3 siguen pendientes.
