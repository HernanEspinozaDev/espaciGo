# Evidencia MPV2-04 · Fotos y ficha pública

**Fecha:** 2026-10-13. **Base:** merge de MPV2-03 #297, `0fe7785352ba91972d4ec403d24b4cd085d5477e`. **HEAD de código probado:** `18099504fe8bffecbf6a5c3d77ec9e04e9f0dd9f` (`codex/marketplace/feature/mpv2-04-public-images-detail`). Las capturas y mediciones siguientes se generaron desde ese árbol.

## Recorrido FE-14 conectado

El ensayo usó el proyecto Compose aislado `espacigo_fe14_84ba3ad02c78`, API/Angular/Mailpit en puertos `19020/19022/19021` y volumen PostgreSQL `espacigo_fe14_84ba3ad02c78_pgdata`. La configuración resuelta no montó `espacigo_pgdata`. El script retiró contenedores, red, volumen y estado temporal al terminar; las cuentas sintéticas y sus credenciales temporales también se eliminaron.

Resultado: **2/2 pruebas FE-14 aprobadas**. Un anfitrión creó un PNG sintético privado por defecto, lo declaró público y un visitante sin Bearer consultó catálogo, ficha e imagen. La respuesta binaria fue `image/png`, `Cache-Control: no-store` y `X-Content-Type-Options: nosniff`. Al retirar la publicación, la misma URL respondió 404. Los logs del Backend no incluyeron correo ni contraseña sintéticos.

## Verificaciones

- `go test ./...` — aprobado.
- PostgreSQL aislado, `./internal/adapters/postgres/gallery` — aprobado con V56; default privado, publicación/retirada por anfitrión y rechazo a otro propietario.
- PostgreSQL aislado, `./internal/adapters/postgres/spaces -run '^TestLocalPublicationRequiresEffectiveKYCAndRecordsOwnerTransitions$' -count=1` — aprobado; DTO HTTP de detalle minimizado.
- `npm test` — 199/199 pruebas aprobadas.
- `npm run api:check`, `npm run lint` y `npm run build` — aprobados.
- FE-14: `FE14_PUBLIC_ONLY=1 FE14_PORT_BASE=19020 FE14_EVIDENCE_OUTPUT_DIR=planning/marketplace_publico_v2/evidencia/mpv2_04 bash scripts/test-frontend-e2e.sh` — 2/2 aprobadas.

## Capturas

Los JSON adjuntos registran viewport CSS y dimensiones del documento. En capturas de página completa, las dimensiones JPEG son mayores que el viewport por el alto del documento.

| Captura | Viewport CSS | JPEG | SHA-256 |
| --- | ---: | ---: | --- |
| Ficha pública, escritorio | 1365×900 | 1365×1216 | `9bbacc3ea2ad4218054fd0c4b88fa0d834af4db24a1c4ab84cea467bbb297410` |
| Ficha pública, móvil | 390×844 | 390×1649 | `011964b035bf02f93e52940778de3286bfc5b5322f17d09d5c689cfe4493f4f2` |
| Galería anfitrión, escritorio | 1365×900 | 1365×900 | `dc7445e00e8b9452725150c392407f09296b61486287d937cb8836007e20210e` |
| Galería anfitrión, móvil | 390×844 | 390×2049 | `1cda80809d02c0c93793429d72298102b331a3fc2550bdeeceebfe297bd2aa82` |
| Búsqueda visitante, escritorio | 1365×900 | 1365×1570 | `de534f219a769aee5219e5dc9ec98585aebc500c04eae37cba38a44e55484ade` |
| Búsqueda visitante, móvil | 390×844 | 390×2620 | `0adfb4bb2b7fd08d0297745668cce2997784428c891f162b11dc69393606ef48` |
| Catálogo público, escritorio | 1365×900 | 1365×1773 | `e76a28046e276befaa2cdd556cb2240a867d3fd537e7ed176ac44a2006cdd48c` |
| Catálogo público, móvil | 390×844 | 390×2887 | `fee26e272c954916c55286fb15346cd856cb83cf436a14c8fa9c7f14fe13fd00` |

## Límites

Las imágenes son fixtures sintéticos, no fotografías reales. No se probaron GCP, mapas, pagos ni SSR. La retirada impide nuevas lecturas del servidor; no revoca copias que un visitante ya descargó. La fusión técnica y esta evidencia no implican aceptación de producto.
