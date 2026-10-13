# Evidencia MPV2-03 — editor y metadatos públicos

Recorrido Angular → Go → PostgreSQL en FE-14, ejecutado el 2026-10-13 con el proyecto Compose desechable `espacigo_fe14_a87338035264`. La configuración resuelta usó únicamente el volumen `espacigo_fe14_a87338035264_pgdata`, puertos 18780–18782 y recursos con prefijo de ejecución; `espacigo_pgdata` no se montó ni modificó. El script retiró servicios, volumen, red, cuentas, contraseñas y datos temporales al terminar.

`FE14_PUBLIC_ONLY=1 FE14_EVIDENCE_OUTPUT_DIR=planning/marketplace_publico_v2/evidencia/mpv2_03 TMPDIR=/tmp bash scripts/test-frontend-e2e.sh` ejecutó 2 pruebas: catálogo anónimo existente y recorrido MPV2-03. Ambos pasaron. El anfitrión creó un borrador, guardó región 13/comuna 13123, editó el título, configuró `America/Santiago` y publicó. Una ventana anónima encontró el título mediante búsqueda y filtros, recibió HTTP 200 sin encabezado Authorization y mostró “Providencia, Metropolitana de Santiago”; la ficha minimizada no expuso la dirección privada. El tercer usuario fue rechazado al abrir la ruta de edición. La integración Go/PostgreSQL prueba también rechazo de escritura de otro propietario y lectura pública minimizada.

Los cuatro JPEG son capturas de página completa con metadatos JSON de viewport y documento. Anfitrión: 1365×900 y 390×844 CSS px. Visitante: 1365×900 y 390×844 CSS px. No se capturó información de acceso. Hash SHA-256:

| Archivo | Dimensiones de captura | SHA-256 |
| --- | ---: | --- |
| `fe14_mpv2_03_host_desktop.jpg` | 1365×900 | `5c08617cc94b9451f4ba138b2124c15cada761f505639ce9ae23ea64886bf629` |
| `fe14_mpv2_03_host_mobile.jpg` | 390×2526 | `9c8e74b766144051c6581aa859a39abb5b3b2dd41e6c4854315ffed30d5f47e2` |
| `fe14_mpv2_03_visitor_search_desktop.jpg` | 1365×1570 | `11c38d28e1024bb7482f928140209c8376e1daad77d13825edc70fddab7944c3` |
| `fe14_mpv2_03_visitor_search_mobile.jpg` | 390×2620 | `0f8471c10202cf247eb71b9078c9dc9d4a457bc1fd34b38889c5ba6ddd426e70` |

El corte no cambia migraciones V1–V55 ni agrega otra migración: usa el catálogo territorial nullable existente de V55. No incluye fotos reales, SSR público ni aceptación final de producto.

## Correcciones de revisión

El 2026-10-13 se corrigió la fusión concurrente de región/comuna como par indivisible y se endureció la validación para rechazar ambos sentidos de par parcialmente vacío. Verificaciones ejecutadas para esas correcciones: `go test ./...` PASS; integración PostgreSQL aislada `TestLocalPublicationRequiresEffectiveKYCAndRecordsOwnerTransitions` PASS (las dos variantes HTTP responden 422 y conservan región/comuna persistidas); suite Angular 199/199 PASS (incluye cambio solo de comuna, selección desde vacío, retirada y conservación de actualización remota); `npm run lint`, `npm run api:check` y `npm run build` PASS, sin avisos de presupuesto. Las capturas de arriba corresponden al recorrido FE-14 2/2 del 2026-10-13 anterior a estas correcciones; no se repitió FE-14 para esta revisión porque los cambios se verificaron mediante regresiones Angular y PostgreSQL enfocadas.
