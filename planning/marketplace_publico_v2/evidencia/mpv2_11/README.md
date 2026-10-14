# Evidencia MPV2-11 — filtros tipados por categoría

Fecha: 2026-10-14. Implementación inicial probada en `632e47d94a20d62b83a0df2ea07276623d37cac9`; respuesta a revisión y pruebas de regresión en `f35c143ba95d872566093131b7d3eeae6ef1f93b`, rama `codex/marketplace/feature/mpv2-11-typed-category-filters`, basada en el merge de MPV2-10 `5029a1ab73f4e9c365d5352299849b7dbe80991f` (PR #304). El PR de este corte es [#305](https://github.com/HernanEspinozaDev/espaciGo/pull/305), creado por `HernanMEC` con revisión solicitada a `HernanEspinozaDev`; el Correcciones P2 probadas en `f35c143ba95d872566093131b7d3eeae6ef1f93b` (HEAD al abrirlo: `de7e3b8fcca645f6e860b29794c95a5cb9e50329`). No implica aceptación del producto.

## Verificaciones ejecutadas

- `go test ./...` pasó. Sin `TEST_DATABASE_URL`, los tests de integración que requieren Postgres no corren en esa invocación.
- Integración real con PostGIS: `TestMPV211MigrationWorksFromEmptyAndV56WithoutChangingHistory` pasó desde base vacía y desde V56; comprobó que V1–V56 mantienen sus checksums.
- Angular: `npm test -- --watch=false` (262 pruebas), `npm run api:check` (incluye carga YAML y pruebas de patrón decimal), `npm run lint` y `npm run build` pasaron.
- FE-14 con Compose/PostgreSQL aislados verificó las ocho categorías, metadatos y filtros tipados con paginación, booleano `false` frente a clave ausente, validaciones inválidas, controles de rango/enum/lista, ambas representaciones de enum-list (CSV y `in` repetido), URL/Atrás-Adelante, SSR con CSV y retorno contextual de inicio de sesión. También se ejecutó el chequeo SSR ante indisponibilidad y recuperación de API.
- La integración de catálogo corrió con 1.000 publicaciones sintéticas y EXPLAIN ANALYZE para predicados rango+enum a través de cursores. El plan y tiempo son de ese fixture local; no predicen carga de producción.
- `git diff --check` pasó.

## Entorno y límites

FE-14 creó un proyecto/volumen/puertos Compose desechables exclusivos (`espacigo_fe14_901a0a71bb81`, base de puertos 22140 (corrida de regresión; los anteriores ensayos FE-14 también fueron desechables)), usó cuentas y publicaciones sintéticas y retiró los recursos del ensayo al terminar. No usó `espacigo_pgdata` ni la demo persistente. Las credenciales temporales se guardaron con permisos restringidos y fueron retiradas. La prueba no mide carga real, proveedor de identidad externo ni comportamiento en GCP; no se usaron datos ni fotos reales.

## Capturas servidas

Las capturas se tomaron a 1365×900 y 390×844; dimensiones completas y SHA-256 están en los sidecars JSON. La vista móvil conserva abierto el panel de filtros al capturar la página completa; por ello el panel se superpone al contenido en esa imagen. El recorrido funcional y la captura no representan una aprobación visual del usuario.

- Escritorio: [JPG](fe14_mpv2_11_typed_filters_desktop.jpg), SHA-256 `ebfdadf67b3cc29d1597641ac6adfc39c88cfb457604e764df01299966913e70`.
- Móvil: [JPG](fe14_mpv2_11_typed_filters_mobile.jpg), SHA-256 `8cb8cbb957063c7be664c027540fd3dba131c0f2998de69d64fcd722517e97c0`.

La aceptación de producto sigue pendiente del usuario.
