# Fase 1 — scaffold Next.js (resultado de validación)

Fecha de validación: 2026-10-24
Rama: `frontend-next-hermes`
HEAD de publicación inicial: `b2448771dd9ddfe9d2703d439e5d793144b2dd5a`
Alcance: sustitución controlada del árbol frontend Angular por scaffold neutral Next.js; no se inició Fase 2.

## Resultado del scaffold

- Next.js estable más reciente `16.4.3`, React `19.3.0`, pnpm `12.15.0`, TypeScript `5.9.3` estricto. La excepción de antigüedad mínima está limitada por `pnpm-workspace.yaml` a Next `16.4.3`, `eslint-config-next@16.4.3` y paquetes `@next/*` correspondientes; no se desactivó ni amplió globalmente la política.
- App Router bajo `src/app/`, shell neutral en español, metadata/icono EspaciGo y `public/espacigo-mark.svg` conservado como único activo heredado.
- Configuración standalone, ESLint, Prettier, scripts de gates, tests Node, `.gitignore` Next/pnpm y `.dockerignore` que excluye dependencias, builds, caches y archivos de entorno.
- Dockerfile multi-stage con usuario runtime `node`, puerto 4000 y healthcheck HTTP. La instalación Docker usa `--ignore-scripts` para respetar la política de scripts de pnpm; el build Next funcionó con las dependencias precompiladas.
- `start:local` ejecuta el servidor standalone en loopback `127.0.0.1:4201` e incluye el activo público.
- `frontend/AGENTS.md` contiene reglas específicas Next.js.
- Las referencias Your Next Store (`1de63c614f4c57c1862b1b14710f2133784749c1`) y Next.js SaaS Starter (`6e33e58b1e553a41fe22e6b941a7229a002de361`) permanecen en `.local/frontend-template-sources/`, ignoradas por Git.

## Comprobaciones ejecutadas localmente

Desde `frontend/` con Next.js `16.4.3`:

- `pnpm install --frozen-lockfile`: PASS, pnpm `12.15.0`.
- `pnpm lint`: PASS.
- `pnpm typecheck`: PASS.
- `pnpm test`: PASS, 2/2.
- `pnpm format:check`: PASS.
- `pnpm build`: PASS, Next generó rutas estáticas `/` y `/_not-found`.
- `pnpm start:local`: PASS. `curl` a `http://127.0.0.1:4201/` devolvió HTTP 200 y se verificaron marca, texto y `lang="es"`; el SVG `/espacigo-mark.svg` también devolvió HTTP 200. El proceso local se detuvo al finalizar la comprobación.
- Sustitución Angular: escaneo del árbol fuente/configuración (sin `.next`, `node_modules`, lockfile ni AGENTS) no encontró `@angular`, `angular.json`, `ng build`, `ng serve` o Vitest.
- No hay `node_modules` ni `tsconfig.tsbuildinfo` versionados. `node_modules/`, `.next/` y `*.tsbuildinfo` están ignorados; el build puede generar localmente `tsconfig.tsbuildinfo`.
- `git diff --check`: PASS.

## Docker

- Antes del ensayo se verificó que no existía el contenedor `espacigo-frontend-phase1`.
- `docker build -t espacigo-frontend-phase1:local .`: PASS. El Dockerfile copia `pnpm-workspace.yaml` antes de instalar, por lo que pnpm 12.15.0 aplica las exclusiones acotadas de cuarentena. Instalación frozen con `--ignore-scripts`, Next `16.4.3` y runtime standalone completó; el lockfile pasó la verificación de políticas.
- `docker run --rm -d --name espacigo-frontend-phase1 -p 127.0.0.1:4202:4000 espacigo-frontend-phase1:local`: iniciado; `curl` verificó HTTP 200, shell en español y SVG; `docker inspect` confirmó usuario `node`, estado `running` y `health=healthy`.
- Tras completar el healthcheck, el padre detuvo y eliminó el contenedor exacto `espacigo-frontend-phase1`. Una comprobación posterior de `docker ps -a` confirmó que ya no existe. No se crearon ni modificaron volúmenes.

## Límites y siguiente paso

Todas las comprobaciones fueron locales; no se visitó app pública/demo, no se usaron cuentas o datos reales y no se tocaron backend, SQL, infraestructura, Compose, volúmenes o datos. No se inició Fase 2.

Los gates de instalación, lint, tipos, tests, formato, build Next, servidor local y build/ejecución/healthcheck Docker pasaron con Next `16.4.3`. El contenedor efímero fue retirado y la Fase 1 queda cerrada. No se inició Fase 2.
