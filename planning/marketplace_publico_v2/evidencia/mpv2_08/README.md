# Evidencia MPV2-08 · Regresión y operación local

**Estado:** evidencia técnica del corte; no representa aceptación final de producto.

## Identificación del ensayo

- Base y `FE14_HEAD` registrada por Playwright: `f259d50d92d44b2e68d180c9b2aef07b40e942a3` (merge de MPV2-07, PR #301).
- Código de ensayo: cambios locales del runner y pruebas de MPV2-08 descritos en la rama `codex/marketplace/chore/mpv2-08-consolidated-regression`.
- Proyecto Compose exclusivo: `espacigo_fe14_b151dc44cf79`; puertos API/Angular/Mailpit `18780/18782/18781`; volumen propio `espacigo_fe14_b151dc44cf79_pgdata`.
- Fixture recreable: dos publicaciones públicas sintéticas de oficina, cuentas sintéticas renter/host/administrador/tercero; secretos en archivo temporal modo 0600. Se retiraron contenedores, red, volumen y secretos al finalizar.
- Todas las capturas `.jpg` tienen SHA-256 en [SHA256SUMS](SHA256SUMS). Los JSON adyacentes registran viewport, ancho/alto del documento y HEAD.

## Resultado ejecutado

- `scripts/test-local-qa-backend.sh`: aprobado. Unit tests Go, `go vet`, 89 integraciones PostgreSQL/PostGIS + Mailpit requeridas, cero skips. Se repitió cinco veces el test de lifecycle/migración V54→V56; cinco pasaron.
- La integración prueba base vacía, luego crea otra base en V54, confirma exactamente 54 filas de historial y conserva filas, nombres, checksums y timestamps V1–V54 byte por byte al aplicar V55 y V56. Valida campos territoriales y `publicada_en_catalogo` de V56.
- Angular: `npm test -- --watch=false` — 239/239 en 36 archivos; incluye axe-core WCAG A/AA para shell, catálogo escritorio, diálogo móvil e identidad. `npm run lint` (incluye estilos y arquitectura), `npm run api:check` y `npm run build` aprobaron.
- `scripts/test-frontend-e2e.sh`: 11/11 Playwright conectados contra Go/PostgreSQL/Mailpit, incluido HTML SSR sin JS, listado/ficha, foto sintética y retirada 404, filtros/historial, registro/login contextual con cotización y reserva solo tras acción expresa, formularios host, pantallas renter/host/admin/tercero, privacidad en logs, y caída/restauración preservando la sesión en la misma pestaña. El smoke SSR `npm start` usó API/origen local configurado.
- Se agregaron pruebas HTTP CORS para Origin permitido/rechazado, `OPTIONS` permitido/rechazado y SSR sin `Origin`; `go test ./internal/publiccatalog/transport/http` pasó.
- Plan PostgreSQL `EXPLAIN (ANALYZE, BUFFERS)` de elegibilidad sobre el fixture de 2 espacios: 2 filas, 0,544 ms de ejecución, 36 shared buffer hits, tarifa latest por índice `tarifa_espacio_latest_idx` y exclusiones por sus índices. PostgreSQL 18.6. Esto solo describe el fixture mínimo; no predice costos con inventarios reales.

## Capturas y procedencia

- Catálogo público SSR conectado como visitante anónimo, detalle sin datos privados: [escritorio](fe14_mpv2_07_catalog_desktop.jpg), viewport 1365×900, hash `5506962e6de6dc7182226c217b86470b44688c63619fbf5bc2c62b581d6b39cc`; [móvil](fe14_mpv2_07_catalog_mobile.jpg), 390×844, hash `7f18507e177a071bb90afce22923ee1fcdf2b9948f3a16e068d463f668184216`.
- Ficha pública sintética: [escritorio](fe14_mpv2_07_detail_desktop.jpg), 1365×900; [móvil](fe14_mpv2_07_detail_mobile.jpg), 390×844. Hashes en `SHA256SUMS`.
- Retorno privado renter tras login y navegación responsive: [escritorio](fe14_renter_desktop_b151dc44cf79.jpg), 1365×900; [móvil](fe14_renter_mobile_b151dc44cf79.jpg), 390×844.
- Host autenticado en “Mis espacios”: [escritorio](fe14_host_desktop_b151dc44cf79.jpg), 1365×900; [móvil](fe14_host_mobile_b151dc44cf79.jpg), 390×844.
- Panel admin tras login sintético: [escritorio](fe14_admin_desktop_b151dc44cf79.jpg), 1365×900; [móvil](fe14_admin_mobile_b151dc44cf79.jpg), 390×844.
- Cada imagen corresponde al ensayo FE-14 conectado; los pares JSON y el manifiesto SHA permiten verificar viewport/HEAD/contenido. El HEAD de captura es la base del corte porque los cambios locales son pruebas y operación, sin cambios visuales de Angular.

## Límites revisados

- Límites públicos actuales: página máximo 25, consulta textual máximo 120 runas, ubicación máximo 400 resultados; servidor Go limita lectura/escritura a 10 s e idle a 60 s. SSR tiene límite temporal de 25 s por inventario del sitemap.
- El handler de lectura pública no implementa cuota por IP; por eso no devuelve 429 ni `Retry-After`. Los endpoints de autenticación sí cuentan con respuestas 429/`Retry-After`. En despliegue público deberá definirse una política de cuota en un proxy confiable o un limitador distribuido antes de aumentar exposición; esta ejecución local no inventa una política ni confía en `X-Forwarded-For`.
- Las mediciones SQL son de dos publicaciones sintéticas y datos locales pequeños, sin extrapolación de capacidad/latencia. No se midió carga, concurrencia sostenida, contraste con instrumento en navegador, despliegue GCP, fotos reales ni pagos reales.
- Una ejecución intermedia encontró que el timeout global Playwright de 90 s era insuficiente para la falla de red real. El test específico se amplió a 150 s y la corrida final pasó 11/11; la pila y datos de esa corrida se retiraron.
