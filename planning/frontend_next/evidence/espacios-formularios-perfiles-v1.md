# Perfiles de categoría v1: cobertura de renderizado UI

**Fecha:** 2026-10-28  
**Repositorio:** `/home/nandev/Documentos/projects/espaciGo`, `main`, base inspeccionada `3956d2f`  
**Estado:** cobertura de presentación dinámica con mocks; F3 permanece parcial.

## Alcance

Se añadió `frontend/tests/fixtures/category-profiles-v1.json` como fixture sintético derivado de los perfiles versionados en `V000006__m04_extensible_category_attributes.sql`, `V000007__m04_profile_attribute_order.sql` y `V000067__inmuebles_residential_rentals.sql`. El fixture contiene los nueve perfiles v1 y 116 atributos; `order` refleja la migración de ordenamiento. No se consultó ni modificó PostgreSQL para generarlo.

Se añadieron nueve casos Playwright de matriz, uno por cada perfil: `oficina`, `sala_multiproposito`, `bodega`, `estacionamiento`, `local_flexible`, `stand`, `quincho`, `parcela_eventos` e `inmuebles`. Para cada categoría, el caso valida que la entrada del fixture exista y corresponda al código y versión v1 solicitados, recorre todas las definiciones y comprueba el control correspondiente (`boolean`, `integer`, `number`, `enum`, `enum_list`), límites/pasos numéricos y opciones exactas; verifica la lectura del perfil y que el renderizado no emite `POST /api/backend/spaces`. Los atributos obligatorios de tipo `enum` se exponen con `aria-required="true"` en wizard y editor, y la matriz lo comprueba en `inmuebles`. Se agregó además un caso de validación accesible para `sala_multiproposito` v1: `sillas_disponibles=-1` bloquea el avance, enlaza el error de mínimo con el control y no emite POST. Las pruebas previas de interacción de wizard/quincho/estacionamiento siguen aportando cobertura funcional complementaria.

## Verificación ejecutada

Desde `frontend/`, usando pnpm 12.15.0:

- Matriz de perfiles: `npm exec --yes --package=pnpm@12.15.0 -- pnpm exec playwright test tests/phase7-host-listings.spec.ts -g 'perfil .* v1: renderiza todos sus atributos sin mutar'` — **9/9**.
- Validación accesible: `PLAYWRIGHT_PORT=4203 npm exec --yes --package=pnpm@12.15.0 -- pnpm exec playwright test tests/phase7-host-listings.spec.ts --grep 'perfil sala_multiproposito v1: bloquea el avance'` — **1/1**.
- Especificación completa: `npm exec --yes --package=pnpm@12.15.0 -- pnpm exec playwright test tests/phase7-host-listings.spec.ts --project=chromium-local` — **18/18** en Chromium local, servidor Next standalone en `127.0.0.1:4203`.
- Rerun del padre tras auditoría independiente (que no tenía `pnpm` directo disponible): `npm exec --yes --package=pnpm@12.15.0 -- pnpm exec playwright test tests/phase7-host-listings.spec.ts` — **18/18** en Chromium local; salida confirma Next.js 16.4.3 en `127.0.0.1:4203`.
- `pnpm build` — PASS con Next.js 16.4.3; `pnpm typecheck`, `pnpm lint`, `pnpm format:check`, `pnpm test` — **260/260**, y `pnpm api:check` — **182 operaciones**: PASS.
- `git diff --check`: PASS.

La rebanada incluye cambios de accesibilidad no visuales en los controles `select` requeridos de wizard y editor (`aria-required`). El build se rehizo antes de Playwright; la suite usó el artefacto standalone local en `127.0.0.1:4203`.

## Límites y siguiente puerta

Todos los casos usan datos sintéticos y mocks de rutas. Demuestran el comportamiento visible del renderer frente a perfiles v1 derivados de migraciones, y una regla UI de mínimo accesible en `sala_multiproposito`; no prueban la respuesta real de Go, validación/serialización de todas las combinaciones, CRUD, recuperación tras reload ni persistencia PostgreSQL. Tampoco demuestran browser→BFF→Go→PostgreSQL/storage. No se hicieron escrituras en DB, llamadas a servicios remotos, commits ni despliegues. F3 sigue abierta hasta probar el recorrido visible y persistido con un harness local seguro; F6/auditoría de inicio de API determina ese harness.

## Revisión independiente — 2026-10-28

La auditoría read-only inicial del árbol en HEAD `3956d2f3459b0bf4b15511f4e1a74ffd06a29975` aprobó los cambios inspeccionados (`review_passed=true`, sin findings). Confirmó las aserciones de `category_code`, `schema_version`, requeridos accesibles y los nueve casos de matriz. El conteo del fixture fue **9 perfiles y 116 atributos**. Gaps registrados entonces: la matriz comprobaba `aria-required` en wizard pero no demostraba equivalentemente el estado en el editor; la validación derivada del esquema cubría solo el mínimo de `sillas_disponibles`; ese audit no reejecutó gates. El PASS de revisión no cambia F3 de parcial: persistencia e integración real siguen pendientes.

## Revisión independiente adicional — 2026-10-28

Una segunda auditoría read-only del mismo checkout/HEAD no encontró bloqueos dentro del alcance de la matriz UI mock. Reportó que los nueve perfiles son v1; siete payloads concuerdan con V6 aplicando los defaults de orden V7, y los 34 códigos de `inmuebles` concuerdan con V67. También verificó el caso inválido accesible y la matriz de controles/required sin POST. El padre volvió a contar programáticamente **9 perfiles, 116 atributos y versión `[1]`**, y ejecutó la matriz más el caso inválido en Chromium local: `env -u PHASE3_NATIVE_API_E2E -u PHASE4_NATIVE_API_E2E -u PHASE5_NATIVE_API_E2E -u PHASE6_NATIVE_API_E2E -u PHASE7_NATIVE_API_E2E -u PHASE8_NATIVE_API_E2E -u PHASE9_NATIVE_API_E2E PLAYWRIGHT_PORT=4204 npm exec --yes --package=pnpm@12.15.0 -- pnpm exec playwright test tests/phase7-host-listings.spec.ts --project=chromium-local -g 'perfil .* v1: renderiza todos sus atributos sin mutar|perfil sala_multiproposito v1: bloquea el avance'` — **10/10**. Las banderas `PHASE*_NATIVE_API_E2E` se desactivaron; solo se usó Next standalone con mocks, sin API/DB. `git diff --check` pasó y el puerto 4204 quedó libre.

Este resultado valida el corte **mock UI**; no acredita Go, BFF real, CRUD ni persistencia. F3 permanece parcial hasta la integración visible y persistida bajo el gate F6.
