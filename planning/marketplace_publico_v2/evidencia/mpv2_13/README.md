# Evidencia MPV2-13 — regresión consolidada

**Fecha:** 2026-10-14. **Código de aplicación bajo prueba:** `c797f0eb734e2c7eda37eb70a50d4d509e82677d` (merge MPV2-12, PR #306). Los cambios MPV2-13 son tests, scripts de ensayo y documentación; no alteran el código de aplicación fusionado. **Rama de trabajo:** `codex/marketplace/chore/mpv2-13-regression-acceptance`.

## Historial de migraciones

`bash scripts/verify-mpv2-13-migrations.sh` aprobó: exactamente V1–V57 y cada archivo es byte a byte idéntico al baseline `c797f0eb734e2c7eda37eb70a50d4d509e82677d`. No se agregó V58. Los SHA-256 por archivo están en [migration-sha256.txt](migration-sha256.txt). El backend QA ejercitó instalación vacía y actualización desde V56 mediante el test de migrador existente; también se corrigió la regresión del fixture V54 de booking para incluir solo V1–V54 y comprobar la aplicación de V55–V57 sin reescritura del historial anterior.

## Comprobaciones ejecutadas

- Backend: `bash scripts/test-local-qa-backend.sh` — gate 4/4; `go test -p 1 ./...`; `go vet`; 132 paquetes Go reportados, 91 integraciones obligatorias, cero skips; repetición del ciclo de booking 5/5; higiene correcta y recursos desechables retirados.
- Frontend Angular 22.2.2 / Node 24.21.0: `npm ci --ignore-scripts --no-audit --no-fund`; `npm run lint` (ESLint, Stylelint, arquitectura); `npm run api:check`; `npm test -- --watch=false` (267 pruebas); `npm run build` (SSR y 3 rutas prerenderizadas). Todo pasó.
- FE-14 E2E contra Go/PostgreSQL real: 16/16 pasó en el proyecto desechable `espacigo_fe14_87a0a0e0761d`, puertos loopback `21020–21023`, volumen `espacigo_fe14_87a0a0e0761d_pgdata`. Incluyó ocho categorías, filtros booleanos/número/enum/lista con coincidencias y misses, tarifas, orden/paginación/historial, retorno tras login, borrador, privacidad/exportaciones, foto sintética y retirada, SSR/canonical/sitemap, caída y recuperación conservando sesión. Axe/teclado y vistas de `390×844` y `1365×900` están cubiertos por los recorridos existentes. Credenciales sintéticas eliminadas con el estado al finalizar.
- Demo nueva aislada: `scripts/prepare-mpv2-13-demo.sh` pasó 5/5 pruebas públicas y la caída/recuperación de SSR. Proyecto `espacigo_mpv2_13_demo_316ee26df160`, puertos `19900–19902`, volumen dedicado `espacigo_mpv2_13_demo_316ee26df160_pgdata`, bind en loopback, PostgreSQL sin puerto host. Propietario HernanMEC; motivo: revisión solicitada; retener hasta decisión de producto. Las cuentas sintéticas recreables se mantienen fuera de Git con modo `0600`.
- No se usaron ni modificaron la demo MPV2-09 ni el volumen persistente `espacigo_pgdata`.

## Capturas

Las capturas se generaron desde resultados reales de Go en la demo FE-14, con el checkout de aplicación en `c797f0eb734e2c7eda37eb70a50d4d509e82677d`. Al revisar la primera salida se quitó el foco del título antes de capturar; la prueba visual focalizada volvió a pasar 1/1 y regeneró estas imágenes:

- [Catálogo escritorio 1365×900](fe14_mpv2_13_catalog_desktop.jpg) · [mediciones](fe14_mpv2_13_catalog_desktop.json)
- [Catálogo móvil 390×844](fe14_mpv2_13_catalog_mobile.jpg) · [mediciones](fe14_mpv2_13_catalog_mobile.json)

Las publicaciones y fotos son sintéticas; no se validaron fotos reales, pagos reales, GCP ni aceptación de producto. Guía operativa: [aceptacion_mpv2_13.md](../../aceptacion_mpv2_13.md).
