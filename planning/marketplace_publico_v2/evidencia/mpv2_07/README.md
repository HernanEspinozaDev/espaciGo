# Evidencia MPV2-07 · SSR y SEO público

**Estado:** revisión técnica pendiente. No constituye aceptación final de producto ni garantiza indexación o posición en Google.

## Identificación

- Base probada: `a285d4531279a8f93738712247f4f5c80cdeb9f6` (merge de MPV2-06, PR #300).
- HEAD de código probado: `9b64704ac1d9042b2f6a5decb6ada4c0b87d917f`.
- Rama: `codex/marketplace/feature/mpv2-07-public-ssr-seo`.
- Cambios de Go/OpenAPI y migraciones: ninguno; V1–V56 se conservan.

## Verificaciones ejecutadas

- Angular: `npm test -- --watch=false` — 239 pruebas en 36 archivos aprobadas.
- `npm run lint` — aprobado, incluidas reglas arquitectónicas (80 archivos de producción).
- `npm run api:check` — aprobado.
- `npm run build` — aprobado; conserva tres rutas informativas prerenderizadas.
- FE-14 conectado: `FE14_PUBLIC_ONLY=1 FE14_EVIDENCE_OUTPUT_DIR=planning/marketplace_publico_v2/evidencia/mpv2_07 bash scripts/test-frontend-e2e.sh` — 7/7 recorridos aprobados contra Angular, Go, PostgreSQL y Mailpit del stack desechable.

El E2E comprobó HTML de catálogo y ficha sin JavaScript, metadatos de origen configurado, rechazo del Host manipulado, DTO público sin credenciales/datos privados, detalle inexistente 404, sitemap frente a la página devuelta por Go, rutas privadas `noindex`/`no-store`, solicitudes concurrentes sin cruce de datos, hidratación sin volver a descargar el catálogo, y lectura/retirada de foto pública. Durante la prueba se detuvo Go: SSR y sitemap devolvieron 503; al restaurarlo, SSR volvió a 200. Se retiró el proyecto y volumen propios del ensayo; no se usó ni alteró `espacigo_pgdata`, y las credenciales sintéticas temporales se eliminaron.

Las pruebas unitarias, lint, `api:check` y build se ejecutaron antes de crear el commit de código, con contenido idéntico al HEAD descrito; el FE-14 completo se repitió después del commit y las capturas se generaron usando ese SHA.

## Capturas

Cada JSON asociado registra viewport CSS, tamaño de documento y HEAD. Capturas de escritorio: 1365×900; móvil: 390×844.

- Catálogo: [escritorio](fe14_mpv2_07_catalog_desktop.jpg) · [móvil](fe14_mpv2_07_catalog_mobile.jpg)
- Ficha: [escritorio](fe14_mpv2_07_detail_desktop.jpg) · [móvil](fe14_mpv2_07_detail_mobile.jpg)

## Límites

El sitemap recorre la API en páginas de 25, con máximo de 2.000 páginas / 49.996 fichas, límite temporal total de 25 segundos y validación de cursores/IDs. Añade cuatro rutas estáticas. Si la API falla, excede tiempo o el inventario rebasa el límite, responde 503 y no publica un sitemap parcial. No se midió un inventario de más de 49.996 publicaciones. La indexación real depende de rastreadores y configuración de despliegue; no se probó Google ni un dominio público.
