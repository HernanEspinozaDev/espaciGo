# Fase 2R — Integración real de las plantillas

Estado: implementación, gates, validación runtime local y limpieza completados. Fase 2R publicada en la rama autorizada; esta evidencia no equivale a aceptación de producto ni habilita Fase 3.

## Decisión humana y fuentes

- La Fase 2 anterior y su demo están **RECHAZADAS POR EL USUARIO**; su evidencia histórica se conserva con ese rótulo en `fase-2-sistema-visual.md`.
- La decisión vigente exige integrar vistas/código fuente reales, no limitarse a inspiración: Your Next Store es la base del marketplace; Next.js SaaS Starter aporta las vistas del workspace. Ambas viven en una sola app Next.js.
- Your Next Store: https://github.com/yournextstore/yournextstore, commit `1de63c614f4c57c1862b1b14710f2133784749c1`, licencia MIT, copyright 2024 Your Next Store, Inc.
- Next.js SaaS Starter: https://github.com/nextjs/saas-starter, commit `6e33e58b1e553a41fe22e6b941a7229a002de361`, licencia MIT, copyright 2025 Vercel.
- Los dos clones locales se verificaron en esos HEAD exactos y limpios. El inventario archivo-a-archivo y textos completos de ambas licencias están en `frontend/THIRD_PARTY_NOTICES.md`.
- Stack de destino: Next.js `16.4.3`, pnpm `12.15.0`.

## Procedencia y rutas integradas

### Your Next Store — marketplace

Se copiaron/adaptaron al árbol versionado `frontend/src/` el home/hero, header/nav móvil y footer, catálogo/listado, sorting, búsqueda y sugerencias, paginación, filtros facetados, ficha de espacio, galería con navegación/zoom/miniaturas, errores/not-found, skeletons, secciones informativas y primitivas UI. Las rutas de destino principales son `/`, `/products`, `/search` y `/product/[slug]`; la ruta de detalle conserva la galería y componentes visuales de la plantilla. El listado y las tarjetas usan `frontend/src/lib/marketplace-fixtures.ts` y recursos sintéticos locales.

### Next.js SaaS Starter — workspace

Se copiaron/adaptaron el shell de presentación, landing SaaS con composición y terminal visual, shell responsivo del dashboard/sidebar, resumen, general, security, activity, login y pricing, más las primitivas UI del workspace. Las rutas de destino son `/saas`, `/login`, `/dashboard`, `/dashboard/general`, `/dashboard/security`, `/dashboard/activity` y `/pricing`. Pricing y autenticación permanecen explícitamente no funcionales.

Las vistas de marketplace y SaaS usan layouts distintos dentro de una sola app, no dos proyectos desplegables. Los detalles de cada archivo origen/destino, componente, licencia y tratamiento constan en `frontend/THIRD_PARTY_NOTICES.md`.

## Adaptaciones y exclusiones

- Producto → espacio, con nombres, textos, rutas, filtros y labels en español donde el dominio lo requiere; se conservaron jerarquías y densidades de las plantillas.
- Datos de espacio, actividad, cuenta y búsqueda son fixtures tipados locales. Las imágenes de `frontend/public/spaces/` son sintéticas y locales.
- Import paths/routing, CSS del SaaS scopeado bajo `.saas-application`, tokens, marca EspaciGo, labels accesibles y pantallas demo fueron adaptados.
- Auditoría de imports e integraciones: ESLint, TypeScript y `next build` resuelven las rutas compiladas; la búsqueda de transporte/integraciones encontró únicamente `router.prefetch` en el controlador de búsqueda, usado para precargar la ruta local de Next. No hay `fetch`/Axios, SDK de servicios, base de datos, auth ni billing.
- Se excluyeron Commerce Kit/YNS remoto, Bun, carrito/checkout, publicación/telemetría, Drizzle/esquema/base de datos paralela, JWT/cookies/bcrypt, Stripe, billing y equipos. No hay llamadas a API ni persistencia en Fase 2R.
- Las acciones de login, registro, perfil, seguridad, reservas y planes están deshabilitadas o son solo visuales; no solicitan credenciales reales ni afirman garantías, disponibilidad, pagos o publicación real.
- Se retiró la UI custom de Fase 2 rechazada que no procedía de las plantillas. Componentes source con claims e-commerce no aplicables, como trust badges de envío/garantía/devolución, se excluyeron.

## Rutas y pruebas visuales

Playwright/axe cubre las 11 rutas principales en `390×844` y `1365×900`, verifica carga HTTP local, landmark principal, overflow horizontal, ausencia de solicitudes fuera de loopback y axe; las pruebas de interacción cubren teclado, selección de sugerencias, filtros, galería, navegación móvil y acciones demo deshabilitadas. Resultado final: `pnpm test:e2e` PASS, 29/29; `pnpm test:a11y` PASS, 22/22.

Se generaron y se inspeccionaron visualmente las 22 capturas full-page en `.local/frontend-phase2r/captures/`:

- `390x844__home.png`, `390x844__products.png`, `390x844__search.png`, `390x844__space-detail.png`, `390x844__saas-landing.png`, `390x844__login.png`, `390x844__dashboard.png`, `390x844__dashboard-general.png`, `390x844__dashboard-security.png`, `390x844__dashboard-activity.png`, `390x844__pricing-demo.png`.
- `1365x900__home.png`, `1365x900__products.png`, `1365x900__search.png`, `1365x900__space-detail.png`, `1365x900__saas-landing.png`, `1365x900__login.png`, `1365x900__dashboard.png`, `1365x900__dashboard-general.png`, `1365x900__dashboard-security.png`, `1365x900__dashboard-activity.png`, `1365x900__pricing-demo.png`.

Inspección visual: se conservan las composiciones del catálogo, búsqueda, ficha/galería, landing, login, dashboard, general, seguridad, actividad y pricing. No se observaron vistas rotas, overflow/clipping ni etiquetas de interfaz residuales sin traducir; `Coworking` cabe en búsqueda, tarjetas y filtros en ambos tamaños. La terminal ilustrativa de `/saas` se espera hasta que sus seis líneas estén visibles antes de capturar. Las acciones de cuenta/seguridad/login/pricing siguen deshabilitadas y los datos son sintéticos.

## Gates

- `pnpm install --frozen-lockfile`: PASS (pnpm `12.15.0`; lockfile vigente).
- `pnpm lint`: PASS.
- `pnpm typecheck`: PASS.
- `pnpm test`: PASS, 7/7.
- `pnpm format:check`: PASS.
- `pnpm build`: PASS con Next.js `16.4.3`; se generaron las rutas `/`, `/products`, `/search`, `/product/[slug]`, `/saas`, `/login`, `/dashboard`, `/dashboard/general`, `/dashboard/security`, `/dashboard/activity` y `/pricing`.
- `pnpm test:e2e`: PASS, 29/29 en Chromium local.
- `pnpm test:a11y`: PASS, 22/22; axe sin violaciones en 11 rutas y ambos viewports.
- `docker build --progress=plain --tag espacigo-frontend-phase2r:final .`: PASS.
- Última imagen Docker: `espacigo-frontend-phase2r:final`, ID `sha256:7e50e22480dac7d9cfa88ca2f8200b2cd4e7a3f464289e133503a86250bdb803`.
- Validación runtime final, ejecutada por el padre: contenedor `espacigo-frontend-phase2r-final-check` en `127.0.0.1:4204`, `health=healthy`, usuario `node`; las 11 rutas principales (`/`, `/products`, `/search?q=estudio`, `/product/sala-luz-providencia`, `/saas`, `/login`, `/dashboard`, `/dashboard/general`, `/dashboard/security`, `/dashboard/activity`, `/pricing`) devolvieron HTTP 200.
- Limpieza final, ejecutada por el padre: contenedor detenido/eliminado; el puerto `4204` quedó libre. Verificación local posterior confirmó que no hay contenedor de validación ni listeners en 4203/4204. No se montaron ni alteraron volúmenes.
- `git diff --check`: PASS.
- Secret scan básico (claves privadas, tokens GitHub/AWS y asignaciones típicas de secretos) en el código, pruebas, config, assets y notices del frontend, además de `planning/frontend_next`: sin coincidencias.
- `frontend/next-env.d.ts`: sin diff incidental.
- Identidad consultada mediante `gh api user --jq .login`: `HernanEspinozaDev`.
- Publicación: commit de integración `41484bf4d63225931e23db5505bd0e2c4d60217a`, publicado exclusivamente en `frontend-next-hermes` mediante GitHub API; autor y committer verificados como `HernanEspinozaDev`. El commit de cierre de esta evidencia registra la comprobación final de rama/árbol.

## Límites y siguiente paso

Validación de aplicación exclusivamente local: sin navegador del usuario/CUA, pruebas online del producto, backend, SQL, Compose, infraestructura, cuentas reales, datos persistentes ni volúmenes. La fase no inicia ni autoriza Fase 3; cualquier trabajo posterior requiere una instrucción separada.
