# Evidencia FE-14 — MPV2-12

**HEAD durante la ejecución:** `2856a304fbbfdbf1b5dcb0544b7c6d504e574898`, base de `origin/main`. Las capturas y pruebas se generaron con el worktree de MPV2-12; su implementación quedó fijada en `fc3cefda529d4ebf6fecaac5c3b50401a3fa832b`, sin cambios de código posteriores a la corrida.
**Entorno:** Compose FE-14 desechable, Go + PostgreSQL, cuatro cuentas sintéticas; proyecto `espacigo_fe14_d816de2ad4b1`, puertos `19800–19803` y volumen exclusivos. `espacigo_pgdata`, la demo MPV2-09 y sus puertos no se usaron ni modificaron. El runner retiró servicios, redes, volumen, secretos temporales y estado al terminar.

## Recorrido ejecutado

- FE-14 creó 34 publicaciones públicas sintéticas: ocho categorías, tarifas hora/día/mes y atributos distintos.
- 8/8 recorridos E2E servidos pasaron: SSR/canonical, 24 resultados reales, cambio a 12, vista de cuadrícula con cuatro columnas desktop y dos móviles, ausencia de refetch al cambiar presentación, orden/cursor/Atrás-Adelante, filtros, retorno de registro/login y portada pública retirada con 404/fallback.
- La prueba PostgreSQL `TestLocalPublicCatalogQueryPlanAndSitemapPaging` pasó contra un catálogo sintético de 1.000 ofertas; se ejecutó `EXPLAIN (ANALYZE, BUFFERS)` y la consulta de portada no añadió llamadas HTTP por resultado.
- Metadatos de capturas registran viewport y documento: desktop `1365×900` (documento `1365×4947`) y móvil `390×844` (documento `390×9038`). Ambos documentos mantienen el ancho del viewport.

## Artefactos

- [Cuadrícula de cuatro columnas — escritorio](fe14_mpv2_12_grid_4_desktop.jpg) · [mediciones](fe14_mpv2_12_grid_4_desktop.json)
- [Cuadrícula de dos columnas — móvil](fe14_mpv2_12_grid_2_mobile.jpg) · [mediciones](fe14_mpv2_12_grid_2_mobile.json)

Las imágenes de espacios sintéticos usan el estado explícito “Sin fotografía pública”. Los datos/creds del ensayo fueron efímeros y no forman parte de estas capturas. Esta evidencia acredita comportamiento técnico en fixtures locales; no representa fotografías reales ni aceptación de producto.

## Revalidación de correcciones del PR #306 — 2026-10-14

**HEAD de código probado:** `05737e0f7d0715853a15b21d0ece0e34f9d47e7d`. Se ejecutó `FE14_PORT_BASE=19800 FE14_PUBLIC_ONLY=1 FE14_PUBLIC_TEST_GREP='MPV2-07 serves public catalog|MPV2-12 catalog layout|Anonymous visitor explores the Go-backed public catalog and minimized detail|MPV2-04 host declares a photo public' FE14_EVIDENCE_OUTPUT_DIR=planning/marketplace_publico_v2/evidencia/mpv2_12 bash scripts/test-frontend-e2e.sh`.

- **Playwright: 4/4 aprobadas**. La regresión SSR confirma canonical `/explorar` y `noindex, follow` para filtros y parámetros de presentación. El recorrido de layout comprobó columnas CSS calculadas para escritorio 2/3/4 y móvil 1/2, el ajuste de 4 a 2 en el ancho intermedio, ausencia de refetch por presentación y el cambio real a 12 resultados.
- La navegación por cursor comprobó Atrás/Adelante y la restauración de una y dos tarjetas. El recorrido de foto pública comprobó lectura anónima y retirada con 404.
- **SSR durante caída/restauración de Go:** aprobado; catálogo y sitemap respondieron 503 durante la caída y 200 tras restaurar el servicio.
- Entorno desechable `espacigo_fe14_8c2c9b425d44`; puertos loopback `19800–19803`, volumen `espacigo_fe14_8c2c9b425d44_pgdata` y cuatro cuentas sintéticas efímeras. La configuración Compose resolvió solo volúmenes propios y no publicó PostgreSQL. Al finalizar se retiraron contenedores, redes, volumen, secretos y estado temporal. Se verificó que `espacigo_pgdata`, la demo MPV2-09 y sus puertos permanecieron intactos.
- Las mediciones de las capturas actuales corresponden a este HEAD: desktop `1365×900` (documento `1365×4947`) y móvil `390×844` (documento `390×9108`); ambos documentos conservan el ancho del viewport.

Los dos recorridos públicos focalizados y la comprobación del estado de Go se ejecutaron contra Angular, Go y PostgreSQL reales del ensayo; no son respuestas simuladas. Las cuentas fueron sintéticas y las credenciales se eliminaron junto con el entorno.
