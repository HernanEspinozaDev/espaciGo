# Fase 2 — sistema visual y dos shells

Fecha: 2026-10-24
Rama: `frontend-next-hermes`
HEAD de código validado: `b30365c11ba8bab20d04bc50128d713d95510cc0`
Commit: `feat(frontend): add design system and dual shells`
Versiones verificadas: Next.js `16.4.3`, pnpm `12.15.0`.

## Entrega

- Sistema compartido de tokens para marca, superficies, texto, bordes, estados, foco, spacing, radios, elevación, tipografía y densidades pública/administrativa.
- Componentes base de marca, botones, tarjetas, badges, campos y paneles de estado `loading`, `empty`, `error`, `success`, `denied`, `restricted` y `offline`.
- Shell público inspirado estructuralmente en Your Next Store y shell administrativo inspirado en Next.js SaaS Starter; navegación adaptable a móvil/escritorio.
- Marca EspaciGo y favicon SVG.
- Rutas locales de demostración: `/` (marketplace), `/panel` (workspace) y `/design-system` (catálogo de componentes/tokens).
- `start:local` sirve el output standalone en loopback y copia tanto `public/` como `.next/static`, necesarios para que la demostración aplique sus estilos y activos.

Las referencias bajo `.local/frontend-template-sources/` se usaron únicamente para orientación visual/estructural; no se incorporaron sus aplicaciones ni plataformas. La demostración utiliza datos ilustrativos y no integra API, sesión, gateway, autenticación, pagos ni persistencia.

## Gates y evidencia local

Ejecutado desde `frontend/`, con pnpm `12.15.0`:

- `pnpm install --frozen-lockfile`: PASS; lockfile vigente y políticas de supply chain aprobadas.
- `pnpm lint`: PASS.
- `pnpm typecheck`: PASS (`tsc --noEmit`).
- `pnpm test`: PASS, 5/5; incluye comprobaciones de contraste WCAG AA para pares de texto/superficie y tokens obligatorios.
- `pnpm format:check`: PASS.
- `pnpm build`: PASS con Next.js `16.4.3`; páginas estáticas `/`, `/design-system`, `/icon.svg` y `/panel`.
- `pnpm test:e2e`: PASS, 8/8 en Chromium local contra `http://127.0.0.1:4203`; Playwright inicia su propio servidor y no reutiliza procesos existentes. Axe no reportó violaciones WCAG 2.0/2.1/2.2 A/AA en las tres rutas, en `390×844` y `1365×900`. También se verificaron overflow, tamaño mínimo de objetivos visibles, skip link/foco y salto al contenido, navegación móvil por teclado, movimiento reducido y respuesta del favicon.
- `pnpm test:a11y`: PASS, 3/3 rutas y ambos viewports.
- Capturas E2E guardadas fuera de Git en `.local/frontend-phase2/`: `marketplace-mobile-390x844.png`, `marketplace-desktop-1365x900.png`, `workspace-mobile-390x844.png`, `workspace-desktop-1365x900.png`. Se inspeccionaron visualmente capturas de marketplace móvil y workspace escritorio; el CSS carga y no se observa corte/overflow en esas vistas.
- `git diff --check`: PASS.

## Docker

- `docker build --tag espacigo-frontend-phase2:local .`: PASS; la imagen se construyó desde el Dockerfile existente y Next generó las cuatro rutas anteriores.
- Ejecución efímera local publicada solo en `127.0.0.1:4204`: healthcheck `healthy`, usuario runtime `node`; `/`, `/panel`, `/design-system` e `/icon.svg` devolvieron HTTP 200.
- El contenedor se configuró para terminar tras la comprobación (salida 143 por SIGTERM de `timeout`, con `--rm`); se confirmó que el contenedor y el listener del puerto 4204 ya no estaban. No se crearon ni modificaron volúmenes.

## Límites y siguiente paso

La ausencia de violaciones en axe y las comprobaciones de contraste no constituyen certificación WCAG ni cubren rutas futuras. El contenido y las interacciones son una maqueta estática; no se verificaron roles, capacidades, servicios reales ni integraciones.

No se inició Fase 3. La siguiente fase del plan —contrato API, gateway y sesión— requiere mantenerse separada y no queda implementada ni autorizada por esta evidencia.
