# Fase 4 — Marketplace público de EspaciGo

Fecha de verificación local: 2026-10-25
Rama: `frontend-next-hermes`
HEAD de origen antes del corte: `317b7695508efa1a88a95666d828aa188f2950b6`
Base PostgreSQL nativa: `espacigo_dms_stage_20261020` (sin migraciones ni cambios de datos)

## Alcance ejecutado

Se conectó el marketplace Next.js a las operaciones públicas de Go mediante Server Components/adaptador/gateway; el navegador no accede a PostgreSQL. Se implementaron portada, categorías dinámicas, explorar, filtros tipados según perfil público, búsqueda por query, orden/cursor, ficha por UUID, galería y fallback de medios, página residencial vacía, página informativa de anfitriones, cómo funciona y documentos legales. Las publicaciones y categorías visibles provienen de Go; no se añadieron anuncios, precios, direcciones, imágenes residenciales, reseñas ni disponibilidad ficticios.

Se adaptaron las vistas existentes de Your Next Store —navegación, hero, grilla, tarjeta y galería— en vez de sustituir el marketplace por fixtures. Se retiró el autocompletado local y sus datos de muestra porque no existe una operación pública de sugerencias en el contrato usado; la búsqueda se envía al catálogo Go. La galería conserva navegación, miniaturas, teclado, zoom y accesibilidad, y añade fallback de variante → original público → placeholder honesto.

La oferta residencial se presenta como categoría real pero vacía en la respuesta pública verificada. La página para propietarios muestra “¿Tienes una casa de veraneo? Publícala y arriéndala en EspaciGo.” y declara que crear/publicar anuncios aún no está habilitado; no recoge datos ni inicia captación.

## Dos niveles distintos de validación del API

### 1. API Go del checkout actual (HEAD)

El primer bloqueo de arranque quedó resuelto al pasar al servicio el archivo secreto webhook desde el mount privado preexistente. La API Go actual inicia en loopback. En la base nativa autoritativa, categorías y ubicaciones responden HTTP 200, pero el listado de publicaciones responde HTTP 500: el repositorio de HEAD consulta `public.space_media`, relación ausente en esta base (SQLSTATE `42P01`).

Este fallo es una incompatibilidad entre la revisión actual del backend y el esquema ya existente, no un fallo del frontend. No se cambió Go, OpenAPI, SQL, migraciones ni la base. Por ello no se declara validación completa del listado contra el API de HEAD.

### 2. API Go compatible histórica, contra la misma base nativa

Para comprobar la integración pública sin migrar la base, se usó localmente un binario Go construido desde el commit compatible exacto `5995a5fcdbd428220cd8927499d1d33f84466980`, anterior a la dependencia del listado respecto de `space_media`. Este nivel valida la integración frontend ↔ contrato/servicio público compatible ↔ base nativa; no reemplaza la validación pendiente con el API de HEAD.

Resultados observados:

| Operación | Resultado |
| --- | --- |
| `GET /public/space-categories` | HTTP 200; 9 categorías activas, incluida `inmuebles` |
| `GET /public/locations` | HTTP 200; 345 ubicaciones |
| `GET /public/spaces?page_size=3` | HTTP 200; 3 elementos y `next_cursor` |
| Página siguiente usando el cursor opaco | HTTP 200; sin IDs repetidos respecto de la primera página |
| `GET /public/space-categories/inmuebles/filters` | HTTP 200 |
| `GET /public/spaces?category_code=inmuebles&page_size=3` | HTTP 200; 0 publicaciones residenciales |
| `GET /public/spaces?page_size=48` | HTTP 200; 48 elementos; 46 contienen referencias de imagen |
| Detalle público de un elemento elegible | HTTP 200; DTO público y 1 referencia de imagen en el elemento ensayado |
| Variantes `display`/`thumbnail` y endpoint original para esa referencia | HTTP 404 en las tres rutas; Next muestra el placeholder y no inventa una imagen |
| `GET /auth/terms` | HTTP 200, pero cada elemento contiene metadatos sintéticos y no el objeto `document`/su contenido |

El 404 de las tres rutas de imagen corresponde a la referencia real usada en el ensayo; no se atribuye a todas las imágenes del catálogo. Sí se verificó el fallback de la UI con esa referencia y el placeholder accesible tras fallar la variante y el endpoint original.

La respuesta legal fue HTTP 200, no 503, pero carecía de contenido documental. Términos y privacidad no presentan texto legal sustituto: la UI informa que Go no entregó contenido. La rama 503 está implementada para mostrar indisponibilidad sin texto sintético, pero no se ejercitó porque el endpoint compatible respondió 200.

## Verificación frontend

Ejecutado desde `frontend/` después de los últimos cambios:

- `pnpm lint` — pasó.
- `pnpm typecheck` — pasó.
- `pnpm test` — 19/19 pasaron.
- `pnpm api:check` — inventario OpenAPI actual, 179 operaciones.
- `pnpm format:check` — pasó después de aplicar Prettier a los archivos señalados.
- `pnpm build` — compilación, TypeScript y generación de rutas pasaron con Next.js 16.4.3.
- `PHASE4_NATIVE_API_E2E=1 API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1 PLAYWRIGHT_PORT=4205 pnpm test:e2e tests/phase4-marketplace.spec.ts` — 26/26 pasaron contra la API compatible y Next standalone.
- La suite E2E completa — 52 pasaron y 9 pruebas de Fase 3 quedaron omitidas por su gate propio; ninguna falló. Las pruebas vigentes de Fase 2R que dependían de fichas/autocompletado con fixtures se actualizaron para comprobar el catálogo Go actual; la ficha real, fallback de galería, búsqueda y filtros se cubren en las pruebas de Fase 4.

Axe pasó sin violaciones en las rutas públicas cubiertas, en `390×844` y `1365×900`. Las pruebas verificaron ausencia de overflow horizontal, SSR de publicaciones del catálogo, categorías/filtros, cursor, página residencial vacía, CTAs, redirecciones legacy, fichas, fallback de imagen, páginas legales y 404. Las solicitudes externas se bloquearon/registraron y no se observaron solicitudes fuera de loopback ni llamadas JSON de catálogo desde el navegador; las lecturas de catálogo ocurren en el servidor Next hacia Go.

Las pruebas se hicieron con `build` y `start:local`/standalone; no se usó `next dev` ni se interactuó con el navegador del usuario. El listener ajeno de `4201` no se utilizó.

## Límites y cierre

- El frontend está verificado contra la API Go compatible histórica y la base nativa, no contra el listado del API de HEAD debido al error `public.space_media`/`42P01`.
- No hay publicaciones residenciales públicas en la respuesta revisada.
- La referencia de imagen ensayada no está disponible en el endpoint de variante ni en el original; el fallback visual quedó verificado, no la entrega de imágenes reales.
- La respuesta legal compatible no incluye contenido; no constituye aprobación de términos o privacidad productivos.
- La publicación de anuncios, reservas, pagos, campañas, analítica y captación no se implementaron ni activaron.
- La Fase 5 y las fases posteriores no se iniciaron.
- Sin cambios en backend, OpenAPI, SQL, migraciones, Terraform, IAM, GCP, volúmenes, secretos o datos persistentes.
- Sin PR, merge, `main` ni despliegue.

El plan editorial se mantiene separado en `planning/marketing/plan_marketing_digital_espacigo.md`; su estado ahora refleja la oferta vacía, el flujo de publicación no disponible y los límites de compatibilidad. No se activó ninguna acción de marketing.
