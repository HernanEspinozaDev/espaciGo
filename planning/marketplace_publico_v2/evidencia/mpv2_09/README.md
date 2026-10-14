# Evidencia MPV2-09

Capturas nuevas tomadas contra la demo local de la fusión MPV2-08 `6075b7800b45f15e779d11c3ab5853dadd26238e`. Todas corresponden a visitante anónimo y datos de ensayo sintéticos, con Go y Angular reales conectados a PostgreSQL aislado.

## Manifiesto de entorno

| Componente | Versión / identidad observada |
| --- | --- |
| Docker Engine / Compose | 29.9.0 / 5.6.0 |
| Go API (imagen local) | `sha256:1386ace7e953ebd7667b871c06680083ee12c882c1578dad6ff96cc3ae26e0bd` |
| Angular SSR (imagen local) | `sha256:21ab9438ee9ff7b2acbad2630de9cd8da6126f20eff31fd3511bae556692e5be`; Angular/CLI 22.2.2; Node 24.21.0 |
| PostgreSQL/PostGIS (imagen local) | `sha256:f9a5b07f10547e721d54ef765bb3fca8b2ec67684e57f829e1f154ddcbb409d2`; PostgreSQL 18.6 |
| Playwright E2E runner (solo test, no servicio de demo) | `sha256:f22408b7928ef53f709d0cd0447c410e7e9882b1b9f1296348d55f0515f8f0ab` — build local del runner v1.56.1 |
| Mailpit | v1.31.4, imagen fijada `sha256:b68349e3a014b90c5610bfb26b2ae36f3892d7b8cf25ee140c6c71c98d2fcf48` |
| Base PostGIS | PostgreSQL 18 / PostGIS 3.6, digest `sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677` |
| Migraciones en la demo | V1–V56; máximo aplicado V56; MPV2-09 no agrega migración |

Configuración no secreta: proyecto Compose `espacigo_mpv2_09_acceptance`; volumen `espacigo_mpv2_09_acceptance_pgdata`; API/Angular/Mailpit en `127.0.0.1:18780/18782/18781`; `config.json` anuncia `apiBaseUrl=http://127.0.0.1:18780/api/v1` y `siteOrigin=http://127.0.0.1:18782`. El CORS permite el origen loopback del sitio; origen `http://not-allowed.invalid` dio 403 sin encabezado `Access-Control-Allow-Origin`. La contraseña de DB, secretos del runtime y credenciales de las cuentas no están en este manifiesto.

| Captura | Viewport CSS | Documento CSS | SHA-256 |
| --- | ---: | ---: | --- |
| `mpv2_09_catalog_desktop.jpg` | 1365×900 | 1365×1876 | `2534fa3f9273e18272f7fce9bf057d7f9e31c306793dcd0ecb2d2dc58bc420fa` |
| `mpv2_09_catalog_mobile.jpg` | 390×844 | 390×1520 | `b6ef2f17b9590109938510cbaf5ef5900110349371016e93a0296754881fdad8` |
| `mpv2_09_detail_desktop.jpg` | 1365×900 | 1365×1600 | `8c7dd9d20a4cdf6c9d0c41c52190d003af39226d3ff6741128e35224b904b7ac` |
| `mpv2_09_detail_mobile.jpg` | 390×844 | 390×1829 | `c42b47057b2c3c926b0470371dbcd8d9ad1d07903952ff91fd12c35f59cec7de` |

Los archivos JSON contiguos documentan origen de código, timestamp, rol, estado sintético, origen público y de captura, página, viewport/documento y hash. `SHA256SUMS` cubre los cuatro JPEG y sus sidecars. Ninguna captura desborda horizontalmente. La confirmación de reserva queda en almacenamiento local excluido, no aquí, porque muestra un estado privado de reserva sintética.

## Pruebas observadas

- Demo retenida: API readiness 200; Angular health 200; CORS permitido para `http://127.0.0.1:18782` y rechazado para origen no incluido (403, sin `Access-Control-Allow-Origin`); SSR catálogo/ficha 200.
- PostgreSQL de la demo reiniciado: migración máxima V56 y las tres publicaciones sintéticas permanecieron; Go readiness volvió a 200.
- Editor de anfitrión creó y publicó Oficina/hora, Bodega/día y Quincho/mes; Go generó y el anfitrión declaró públicas sus tres fotos PNG sintéticas.
- Visitante anónimo filtró y abrió ficha/imagen; el PNG respondió 200 y el detalle SSR contenía la ficha minimizada. La tercera persona fue redirigida desde edición de un espacio ajeno.
- En la misma demo, un arrendatario sintético eligió una fecha futura, recibió cotización HTTP 200 y confirmó una solicitud HTTP 201. Estado local pendiente de pago fake; sin pago real.
- Regresión FE-14 aislada: 7/7; cubrió registro/verificación y retorno, cotización/reserva, foto retirada 404, ausencia del sitemap y caída/restauración SSR 503→200. Cuentas, contenedores, red, volumen y estado temporales fueron retirados; la demo MPV2-09 se conservó.

La guía, comandos de operación, aislamiento, criterios de decisión y límites están en [aceptacion_mpv2_09.md](../../aceptacion_mpv2_09.md). La captura de reserva, archivo de cuentas y runtime privado se excluyen de Git bajo `.local/testing/mpv2-09-demo/`.
