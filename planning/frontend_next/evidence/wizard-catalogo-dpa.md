# Evidencia: cobertura del catálogo DPA del wizard

**Verificación local:** 2026-11-06. **Base:** rama `codex/frontend/fix/mejoras-creacion-espacios`, HEAD inicial `771134d165046b290b1f5ad3a1074a6357c2327f`.

## Diferencia comprobada

El snapshot histórico `db/reference/dpa_cl_cut_2018.csv` contiene 345 códigos comunales únicos; no se alteró ni se regeneró. El padre comprobó su SHA-256: `6d724d07ca9c9fa628394a6b74e6b413af444f8ab9f95068ba616d3ad11720db`. La prueba previa solo afirmaba ese conteo y por ello aceptaba la omisión. La tabla local tenía esos mismos 345 registros. El suplemento aporta exactamente la fila `12, Magallanes y de la Antártica Chilena, 12202, Antártica`. El padre descargó con curl el documento primario de Código Único Territorial alojado por SUSESO: https://www.suseso.gob.cl/606/articles-19445_recurso_1.pdf y ejecutó `pdftotext -layout`. La extracción efectiva muestra `122 Antártica`, `12201 Cabo de Hornos` y `12202 Antártica`. No se extrapoló ni inventó el código.

La fuente es un catálogo territorial histórico, por lo que acredita el identificador/nombre, no por sí sola la vigencia administrativa de toda la DPA a 2026. Las búsquedas INE/MINVU fueron realizadas por los agentes, no aportadas por el usuario; no se usan como fundamento de la fila adicional.

## Cambios acotados

- Se agregó `db/reference/dpa_cl_supplement_2026-11-06.csv`, una fila con fuente, fecha de verificación y extracto para mantener trazabilidad individual.
- Se agregó `12202 / Antártica / 12` al módulo geográfico. El código sigue siendo string y la selección regional corresponde a Magallanes.
- Se reemplazó la prueba de conteo fijo por comparación completa de códigos, nombres y regiones contra la unión de CSV base y suplemento; detecta omisiones, extras, duplicados y relaciones inconsistentes. El CSV base y su hash histórico no se tocaron.
- No se editó `listing-wizard.tsx`, helpers ni tests F1; la integración visual F2 queda pendiente del gate F1.

## Brecha backend y límites

El catálogo SQL de referencia `db/migrations/V000055__mpv2_02_public_search_and_locations.sql` se conserva intacto y representa el snapshot histórico. No se hizo una migración ni una modificación de backend por inferencia. `12202` debe tratarse como brecha del catálogo backend respecto del suplemento del wizard; verificar contrato/validación Go y resolver la reconciliación explícitamente antes de exigir persistencia de esa comuna. Las pruebas locales no demuestran integración Go/DB ni vigencia completa de la DPA al 2026.

## Pruebas

El padre repitió `node --test frontend/tooling/chile-dpa.test.mjs` junto a las pruebas F1: catálogo **3/3 PASS**, limpieza/formato **4/4 PASS**, total **7 PASS** en esa corrida. `tsc --noEmit` y `git diff --check` también PASS en el checkout combinado de esa revisión. Estos resultados no cierran la integración UI F2 ni el ensayo nativo. No hubo build ni pruebas de aplicación online.
