# Frontend Next.js de EspaciGo

Aplicación única con App Router, React, TypeScript estricto y pnpm. La Fase 2 entrega dos shells visuales —marketplace público y panel demostrativo— sobre tokens y componentes compartidos. Todo el contenido del panel es sintético; no hay API, sesión, gateway, autenticación ni persistencia conectados.

## Rutas de demostración

- `/`: shell público, categorías y tarjetas ilustrativas.
- `/panel`: shell de administración responsive con estados de ejemplo.
- `/design-system`: paleta, componentes, estados comunes y densidades.

## Comprobaciones locales

```sh
pnpm install --frozen-lockfile
pnpm lint
pnpm typecheck
pnpm test
pnpm format:check
pnpm build
pnpm test:e2e
pnpm test:a11y
```

`pnpm test` incluye la comprobación de contraste de tokens; Playwright arranca un servidor standalone en `127.0.0.1:4203` (configurable con `PLAYWRIGHT_PORT`) y verifica teclado, skip link, estados, accesibilidad axe y overflow en `390×844` y `1365×900`. Las capturas de comprobación se guardan bajo `../.local/frontend-phase2/` y no se versionan.

Para desarrollo visual usa `pnpm dev`. Después de `pnpm build`, `pnpm start:local` sirve la salida standalone en `127.0.0.1:4201`. La imagen Docker corre como usuario `node` en el puerto 4000 y no requiere backend, base de datos ni credenciales.

Las plantillas locales en `.local/frontend-template-sources/` son referencias visuales/estructurales en los commits registrados en `../planning/frontend_next/plan_frontend_next_hermes.md`. No se incorporan sus plataformas, autenticación, dependencias de dominio ni persistencia.
