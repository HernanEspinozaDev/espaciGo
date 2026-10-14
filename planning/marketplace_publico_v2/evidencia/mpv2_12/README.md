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
