# F4 — corpus sintético de escenarios del codec

Fecha: 2026-10-28. Checkout `main`, HEAD base `3956d2f3459b`. Prueba de regresión/medición del comportamiento existente, sin cambio de codec, DB, storage o infraestructura.

## Ejecución del padre

`go test -count=1 -v ./internal/media -run '^TestConvertSyntheticScenarioCorpus$'`, con las variables opt-in de DB retiradas: PASS, seis escenarios sintéticos, 4.797 s de paquete. Archivo: `internal/media/codec_corpus_test.go`.

Verifica decode WebP, dimensiones reales vs metadata, tamaño/digest, topes 1600/480 sin ampliar entradas pequeñas, orientación EXIF 6 y ausencia estructural de chunks EXIF/XMP/ICCP/animación en derivados. Los fixtures generados cubren bordes, textura pseudoaleatoria determinista, orientación retrato, transparencia, entrada pequeña y entrada WebP. No son fotos de personas ni archivos del usuario.

| Escenario | Entrada bytes | Display dimensiones / bytes | Thumbnail dimensiones / bytes |
| --- | --- | --- | --- |
| interior_edges, JPEG | 252174 | 1600×1067 / 18042 | 480×320 / 4588 |
| exterior_texture, JPEG | 2669203 | 1600×1200 / 1044618 | 480×360 / 98496 |
| portrait_orientation, JPEG | 138013 | 1067×1600 / 19148 | 320×480 / 4972 |
| transparent_plan, PNG | 57172 | 1600×1200 / 1934686 | 480×360 / 176864 |
| small_image, PNG | 958 | 240×160 / 846 | 240×160 / 846 |
| webp_texture, WebP | 1572184 | 1600×1200 / 1190300 | 480×360 / 91266 |

## Hallazgo y límite

Las metas de display ≤614400 bytes y thumbnail ≤102400 bytes NO se cumplen universalmente: textura JPEG/WebP y transparencia exceden display; transparencia también excede thumbnail. La entrada PNG transparente de 57172 bytes produce derivados mayores. No afirmar que convertir a WebP siempre reduce peso.

El test no convierte el SLO en límite de rechazo. Tampoco ajusta calidad/dimensiones ni cambia reglas de aceptación por inferencia: antes se necesita fijar/verificar el piso visual aprobado y medir una estrategia. El corpus sintético aporta escenarios de estrés recreables, no sustituye un corpus fotográfico representativo ni evaluación visual humana, rendimiento concurrente, storage real o navegador autenticado.
