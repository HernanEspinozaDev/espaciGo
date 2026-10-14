# Acta de cierre local — estado de verificación

Fecha: 2026-10-14 (America/Santiago). Estado: **cierre del desarrollo local aceptado por el usuario dentro del alcance sintético documentado**. LOCAL-CLOSE-00–02 están fusionados; no equivale a lanzamiento/aceptación productiva ni autorización GCP.

## Alcance ensayado

Código de PR #310: `0da50a3a7c3f201b50c4aa7abcfd475bddbd9052` sobre merge #309 `701dabbf7d381c69d240f8cbed823c69de5e4ca1`; la imagen de app corresponde a `0da50a3a7c3f201b50c4aa7abcfd475bddbd9052`. La corrida inicial 7/7 ocurrió en un worktree con HEAD base `0885abf17bd31f57e914b08e48942b4eb4f64b19` y cambios de test lifecycle/reloj/runner aún sin commit; estos se versionaron después en `b2613d5d75d13aba4df8b46b26dfbd5d49b74931`. Los sidecars reflejan el HEAD base, no el hash del harness sin commit. La verificación read-only reproducible sobre la demo se ejecutó luego desde `6bf465ae0566702c3e40f78c61c1ea424706a479`. README de [`evidencia/local-close-02`](evidencia/local-close-02/README.md) y [guía de revisión](evidencia/local-close-02/guia_revision_demo.md) registran URLs, SHA, IDs, imágenes, runner, recursos, restore, resultados y límites. Migraciones V1–V57 y `planning/openapi.yaml` permanecen sin cambios. No se usó `espacigo_pgdata`.

## Aceptación humana y cierre de Issues

El 2026-10-14, después de revisar y fusionar PR #310, el usuario confirmó: «Acepto el cierre del desarrollo local de backend, frontend y base de datos, dentro del alcance sintético documentado». Release: merge `3bf7588d45563145d2e64441446d34070646bd38`; HEAD de aplicación/evidencia revisado `1b8744af73df98b6aa5a7759e13b94ced3c90963`. El registro versionado está en [aceptación de cierre local](aceptacion_cierre_local_20261014.md).

DOCKER-02/#269 y su padre #267 se cerraron como completadas por la evidencia nueva de #310 y esta aceptación, no por el cierre administrativo previo de #269. Comentarios: [#269](https://github.com/HernanEspinozaDev/espaciGo/issues/269#issuecomment-6180587238), [#267](https://github.com/HernanEspinozaDev/espaciGo/issues/267#issuecomment-6180588057). No se recuperó el volumen original DOCKER-02 ni se completó aquella reserva histórica; los resultados nuevos no reescriben esa evidencia.

## Resultado por grupo

- **Backend:** `bash scripts/test-local-qa-backend.sh` PASS: 132 eventos `passed`, 91 integraciones obligatorias, cero skips; cinco repeticiones PASS de check-in. Incluye regresión PostgreSQL del ciclo de booking, reclamo resuelto y terminalidad.
- **Navegador servido:** ciclo LOCAL-CLOSE-02 PASS (1/1) en proyecto independiente `espacigo_localclose02_2c120234abcd`, puertos loopback 21410–21413 y volumen propio. Angular cubrió cotización, reserva/pago fake, aprobación, firma del anfitrión y PDF privado; el resto de mutaciones fue API Playwright al Go servido, con lecturas e historial verificados en Angular. El runner verificó outage SSR/sitemap 503 y recuperación.
- **Reglas de negocio:** reloj file-backed exclusivo; operación previa a checkout+24 h rechazada; disputa, descargo, evidencia, resolución financiera, liberación fake, liquidación, recibo, cierre e idempotencia/tercero comprobados. No se manipuló estado/fecha por SQL. La garantía se mantiene retenida por cualquier reclamo, y la liberación tras resolverlo requiere conciliación admin explícita.
- **Restauración DOCKER-03:** PASS en proyecto `espacigo_docker03_localclose02_345678`, con volúmenes/destinos aislados. Readiness/API quedó cerrada hasta replay de bajas. Conteos origen/restauración `users|notices|migrations = 1|1|57`; fixture privado de 69 bytes restaurado idéntico, dump/marker 0600, TLS runtime/grants/readiness verificados, exit 0. No se tocaron demos ajenas ni la base persistente.
- **Suite histórica:** 18/18 Playwright, Angular 281 pruebas, lint, api:check, build y audit de producción se habían ejecutado sobre la base #309 y siguen consignados como evidencia histórica. No se afirma que esa suite completa se repitiera sobre `0da50a3`; la corrida nueva enfocada sí está vinculada al commit indicado.

## Aislamiento y recursos

La ejecución lifecycle usó proyecto/volumen/directorios de su ID y el runner los retiró junto a las credenciales sintéticas. El restore DOCKER-03 tuvo volúmenes origen/restore/prueba propios. La demo retenida previa `espacigo_localclose02_demo_b51175677392` se reemplazó, solo tras validar el reemplazo, por `espacigo_localclose02_demo_bb5f703b8c7c` (API/Angular/Mailpit 21440–21442, volumen propio; SSR smoke independiente en 21443 solo durante la prueba). La corrida de reemplazo pasó 7/7; luego, desde harness versionado, una comprobación read-only pasó 21 checks y confirmó reserva cerrada, reclamo resuelto y recibo legible para arrendatario en desktop/móvil. En lifecycle el tercero obtuvo 404 en cuatro operaciones antes de liquidar. En la revalidación terminal las lecturas fueron 404; el POST tardío obtuvo 422 por estado terminal y un GET autorizado confirmó que nada cambió. Guía, IDs, roles sintéticos y evidencia están documentados; credenciales solo en el archivo privado mode 0600. No se modificaron otras demos.

## Exclusiones y decisión

El usuario aceptó el cierre local sintético documentado el 2026-10-14 y #267/#269 se cerraron como completadas por esa evidencia nueva, no por el cierre administrativo anterior. No se recuperó el volumen original DOCKER-02 ni se completó aquella reserva histórica. La aceptación MPV2-13 sigue limitada al producto/UX de #307; el cierre local no acepta el marketplace completo ni autoriza GCP. No se usaron proveedores reales ni fondos/personas reales.
