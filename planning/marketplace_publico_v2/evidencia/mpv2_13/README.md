# Evidencia MPV2-13 — regresión consolidada

**Fecha:** 2026-10-14. **Código bajo prueba:** `736f81a367eed79bd87950e2dc64716adc6469db` en PR #307, rama `codex/marketplace/chore/mpv2-13-regression-acceptance`; base `c797f0eb734e2c7eda37eb70a50d4d509e82677d`. La decisión de producto anotada ese día fija cuatro columnas en escritorio, lista de una columna en móvil y selector 12/24/48 junto al orden. La aceptación de producto sigue pendiente.

## Historial de migraciones

`bash scripts/verify-mpv2-13-migrations.sh` aprobó: exactamente V1–V57 y cada archivo es byte a byte idéntico al baseline `c797f0eb734e2c7eda37eb70a50d4d509e82677d`. No se agregó V58. Los SHA-256 por archivo están en [migration-sha256.txt](migration-sha256.txt). El backend QA ejercitó instalación vacía y actualización desde V56 mediante el test de migrador existente; también se corrigió la regresión del fixture V54 de booking para incluir solo V1–V54 y comprobar la aplicación de V55–V57 sin reescritura del historial anterior.

## Comprobaciones ejecutadas

- Backend: `bash scripts/test-local-qa-backend.sh` — gate 4/4; `go test -p 1 ./...`; `go vet`; 132 paquetes Go reportados, 91 integraciones obligatorias, cero skips; repetición del ciclo de booking 5/5; higiene correcta y recursos desechables retirados.
- Frontend Angular 22.2.2 / Node 24.21.0: `npm ci --ignore-scripts --no-audit --no-fund`; `npm run lint` (ESLint, Stylelint, arquitectura); `npm run api:check`; `npm test -- --watch=false` (267 pruebas); `npm run build` (SSR y 3 rutas prerenderizadas). Todo pasó.
- FE-14 E2E contra Go/PostgreSQL real: 16/16 pasó en el proyecto desechable `espacigo_fe14_6f2c9a80b413`, puertos loopback `21240–21242`, volumen `espacigo_fe14_6f2c9a80b413_pgdata`. Incluyó ocho categorías, filtros booleanos/número/enum/lista con coincidencias y misses, tarifas, orden/paginación/historial, retorno tras login, borrador, privacidad/exportaciones, foto sintética y retirada, SSR/canonical/sitemap, caída y recuperación conservando sesión. La misma secuencia sensible al cambio de layout/page size pasó además en la demo de revisión. Se midieron los viewports `390×844`, `768`, `900`, `1024` y `1365×900`, overflow y teclado. Credenciales sintéticas eliminadas con el estado al finalizar.
- Demo de revisión aislada: `scripts/prepare-mpv2-13-demo.sh` pasó 5/5 pruebas públicas más caída/recuperación SSR. Proyecto `espacigo_mpv2_13_demo_a2bf4ff1f4ef`, puertos loopback `20000–20002`, volumen dedicado `espacigo_mpv2_13_demo_a2bf4ff1f4ef_pgdata`, PostgreSQL sin puerto host. URL Angular `http://127.0.0.1:20002/explorar`; API `20000`, Mailpit `20001`. Propietario HernanMEC; retener hasta decisión de producto. Cuentas recreables fuera de Git con modo `0600`.
- La demo previa `espacigo_mpv2_13_demo_316ee26df160` (puertos `19900–19902`) quedó intacta. No se usaron ni modificaron la demo MPV2-09 ni el volumen persistente `espacigo_pgdata`.

## Capturas

Las capturas se generaron desde resultados reales de Go en FE-14 con el código `736f81a367eed79bd87950e2dc64716adc6469db`. Los JSON registran el viewport, el tamaño del documento y ese SHA. La suite conectada pasó 16/16 y la demo de revisión 5/5, ambas con la prueba de 48 elementos:

- [Catálogo escritorio 1365×900](fe14_mpv2_13_catalog_desktop.jpg) · [mediciones](fe14_mpv2_13_catalog_desktop.json)
- [Catálogo móvil 390×844](fe14_mpv2_13_catalog_mobile.jpg) · [mediciones](fe14_mpv2_13_catalog_mobile.json)

Las publicaciones y fotos son sintéticas; no se validaron fotos reales, pagos reales, GCP ni aceptación de producto. Los recorridos prueban el contrato conectado a Go y PostgreSQL reales; las capturas solo muestran una vista del catálogo. Guía operativa: [aceptacion_mpv2_13.md](../../aceptacion_mpv2_13.md).
