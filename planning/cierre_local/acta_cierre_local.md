# Acta de cierre local — estado de verificación

Fecha: 2026-10-14 (America/Santiago). Estado: **verificación técnica completada para LOCAL-CLOSE-02; aceptación humana de producto pendiente**.

## Alcance ensayado

Código de PR #310: `0da50a3a7c3f201b50c4aa7abcfd475bddbd9052` sobre merge #309 `701dabbf7d381c69d240f8cbed823c69de5e4ca1`; demo/UI ampliada en HEAD de ejecución `0885abf17bd31f57e914b08e48942b4eb4f64b19`. README de [`evidencia/local-close-02`](evidencia/local-close-02/README.md) y [guía de revisión](evidencia/local-close-02/guia_revision_demo.md) registran URLs, SHA, IDs, imágenes, runner, recursos, restore, resultados y límites. Migraciones V1–V57 y `planning/openapi.yaml` permanecen sin cambios. No se usó `espacigo_pgdata`.

## Resultado por grupo

- **Backend:** `bash scripts/test-local-qa-backend.sh` PASS: 132 eventos `passed`, 91 integraciones obligatorias, cero skips; cinco repeticiones PASS de check-in. Incluye regresión PostgreSQL del ciclo de booking, reclamo resuelto y terminalidad.
- **Navegador servido:** ciclo LOCAL-CLOSE-02 PASS (1/1) en proyecto independiente `espacigo_localclose02_2c120234abcd`, puertos loopback 21410–21413 y volumen propio. Angular cubrió cotización, reserva/pago fake, aprobación, firma del anfitrión y PDF privado; el resto de mutaciones fue API Playwright al Go servido, con lecturas e historial verificados en Angular. El runner verificó outage SSR/sitemap 503 y recuperación.
- **Reglas de negocio:** reloj file-backed exclusivo; operación previa a checkout+24 h rechazada; disputa, descargo, evidencia, resolución financiera, liberación fake, liquidación, recibo, cierre e idempotencia/tercero comprobados. No se manipuló estado/fecha por SQL. La garantía se mantiene retenida por cualquier reclamo, y la liberación tras resolverlo requiere conciliación admin explícita.
- **Restauración DOCKER-03:** PASS en proyecto `espacigo_docker03_localclose02_345678`, con volúmenes/destinos aislados. Readiness/API quedó cerrada hasta replay de bajas. Conteos origen/restauración `users|notices|migrations = 1|1|57`; fixture privado de 69 bytes restaurado idéntico, dump/marker 0600, TLS runtime/grants/readiness verificados, exit 0. No se tocaron demos ajenas ni la base persistente.
- **Suite histórica:** 18/18 Playwright, Angular 281 pruebas, lint, api:check, build y audit de producción se habían ejecutado sobre la base #309 y siguen consignados como evidencia histórica. No se afirma que esa suite completa se repitiera sobre `0da50a3`; la corrida nueva enfocada sí está vinculada al commit indicado.

## Aislamiento y recursos

La ejecución lifecycle usó proyecto/volumen/directorios de su ID y el runner los retiró junto a las credenciales sintéticas. El restore DOCKER-03 tuvo volúmenes origen/restore/prueba propios. La demo retenida previa `espacigo_localclose02_demo_b51175677392` se reemplazó, solo tras validar el reemplazo, por `espacigo_localclose02_demo_bb5f703b8c7c` (API/Angular/Mailpit 21440–21442, volumen propio; SSR smoke independiente en 21443 solo durante la prueba). La nueva corrida pasó 7/7; dejó reserva cerrada, reclamo resuelto y recibo de liquidación legible para arrendatario en escritorio/móvil; tercero recibió 404 en cuatro operaciones. Guía, IDs, roles sintéticos y evidencia están documentados; credenciales solo en el archivo privado mode 0600. No se modificaron otras demos.

## Exclusiones y decisión

No se usaron proveedores reales, fondos/personas reales ni GCP. No se cerraron Issue #267 ni #269 ni se afirma aceptación de DOCKER-02; la decisión final de producto sigue pendiente del usuario. La aceptación MPV2-13 sigue limitada al producto/UX que cubrió #307. El cierre integral local requiere que el usuario revise la evidencia completa y decida explícitamente; este acta no anticipa esa decisión.
