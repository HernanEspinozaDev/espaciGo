# Evidencia — alineación de validaciones comunes de espacios

**Fecha:** 2026-10-27  
**Repositorio:** `/home/nandev/Documentos/projects/espaciGo`  
**Rama / HEAD base:** `main` / `3956d2f` (cambios locales sin commit)  
**Alcance:** validación de dominio Next.js comparada con `internal/spaces/domain.go`; no acredita persistencia/API real.

## Cambios verificados

- Conteo Unicode de título, descripción, normas y dirección por puntos de código, con semántica de trim concordante con Go.
- Precio base entero mayor que CLP 5.000; área entre 0,01 y 99.999.999,99 con hasta dos decimales; capacidad positiva dentro del rango `int32` (sin límite de negocio inventado).
- `usage_rules` obligatorio en el borrador; límites y tipos de `enum_list` verificados, incluyendo opciones duplicadas; reglas cruzadas frontend de `quincho` y `parcela_eventos`.
- Rutas de retorno de espacios del anfitrión aceptan UUID canónico y rechazan segmentos inválidos.
- Las reglas cruzadas solicitadas para `inmuebles` no se incorporan porque no aparecen en la fuente Go actual; no deben divergir de la autoridad backend sin decisión de contrato.

## Verificación ejecutada en el checkout padre

- `pnpm build` — PASS.
- `pnpm exec playwright test tests/phase6-account.spec.ts tests/phase7-host-listings.spec.ts` — PASS, 14/14 en Chromium; incluye validación visible del wizard/editor y Axe en desktop/mobile.
- `pnpm format:check` — PASS.
- `pnpm typecheck` — PASS.
- `pnpm lint` — PASS.
- `pnpm test` — PASS, 260/260.
- `git diff --check` — PASS.
- El E2E usa mocks locales/sintéticos: verifica que entradas inválidas no mutan; el borrador válido conserva los campos esperados y la tarifa $5.001 CLP se guarda, mientras $5.000 se rechaza.

## Límites / trabajo restante

- La validación de dominio no demuestra persistencia ni la ruta BFF→Go. El editor y todos los campos/ grupos de validación del wizard (pasos 1–4, incluidas categorías dinámicas y tramos horarios) asocian errores y foco (`aria-invalid` en controles compatibles, `aria-describedby` al texto). Playwright cubre Unicode/trim, dirección >500, superficie/capacidad, atributo numérico inválido, CLP 5.000 rechazado y 5.001 aceptado/publicado en el mock, reglas compuestas >250, zona IANA inválida y horario con cierre anterior a apertura; al corregir, el error se limpia. El recorrido usa fixtures/mocks locales sintéticos y no demuestra persistencia ni integración real.
- Las pruebas son locales y sintéticas. No se modificó Go, SQL, OpenAPI, infraestructura, base de datos ni datos persistentes. Sin commit, push o deploy.
