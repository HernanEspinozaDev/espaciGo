# Fase 1 — scaffold Next.js (resultado de validación)

Fecha de validación: 2026-10-24
Rama: `frontend-next-hermes`
HEAD base del trabajo: `977e5be8520d0f81c77a7278fe1412ac90d8b648`
Alcance: sustitución controlada del árbol frontend Angular por scaffold neutral Next.js; no se inició Fase 2.

## Resultado del scaffold

- Next.js estable `16.4.2`, React `19.3.0`, pnpm `12.15.0`, TypeScript `5.9.3` estricto. Se eligió `16.4.2` en vez de `16.4.3` porque esta última aún estaba dentro de la cuarentena de antigüedad mínima de 24 h de pnpm al validar el contenedor; `16.4.2` está publicado desde el 2026-10-15 y no requiere excepción a esa política.
- App Router bajo `src/app/`, shell neutral en español, metadata/icono EspaciGo y `public/espacigo-mark.svg` conservado como único activo heredado.
- Configuración standalone, ESLint, Prettier, scripts de gates, tests Node, `.gitignore` Next/pnpm y `.dockerignore` que excluye dependencias, builds, caches y archivos de entorno.
- Dockerfile multi-stage con usuario runtime `node`, puerto 4000 y healthcheck HTTP. La instalación Docker usa `--ignore-scripts` para respetar la política de scripts de pnpm; el build Next funcionó con las dependencias precompiladas.
- `start:local` ejecuta el servidor standalone en loopback `127.0.0.1:4201` e incluye el activo público.
- `frontend/AGENTS.md` contiene reglas específicas Next.js.
- Las referencias Your Next Store (`1de63c614f4c57c1862b1b14710f2133784749c1`) y Next.js SaaS Starter (`6e33e58b1e553a41fe22e6b941a7229a002de361`) permanecen en `.local/frontend-template-sources/`, ignoradas por Git.

## Comprobaciones ejecutadas localmente

Desde `frontend/` con Next.js `16.4.2`:

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

- El primer build con Next.js `16.4.3` fue detenido por la cuarentena de 24 h de pnpm; no se relajó esa política. Se fijó Next `16.4.2`, versión estable previa, para reproducibilidad sin excepciones.
- `docker build -t espacigo-frontend-phase1:local .`: PASS. La imagen compiló usando pnpm frozen, `--ignore-scripts`, Next `16.4.2` y runtime standalone.
- `docker run --rm -d --name espacigo-frontend-phase1 -p 127.0.0.1:4202:4000 espacigo-frontend-phase1:local`: iniciado; `curl` verificó HTTP 200, shell en español, marca y SVG; `docker inspect` confirmó `user=node` y `health=healthy`.
- La sesión no permitió ejecutar `docker stop` por aprobación interactiva requerida; el contenedor de ensayo `espacigo-frontend-phase1` puede seguir activo en `127.0.0.1:4202`. No se crearon ni modificaron volúmenes. Retirarlo al disponer de una vía aprobada de parada.

## Límites y siguiente paso

Todas las comprobaciones fueron locales; no se visitó app pública/demo, no se usaron cuentas o datos reales y no se tocaron backend, SQL, infraestructura, Compose, volúmenes o datos. No se inició Fase 2.

Los gates de instalación, lint, tipos, tests, formato, build Next, servidor local y build/ejecución/healthcheck Docker pasaron. El artefacto pendiente de limpieza es únicamente el contenedor efímero ya verificado; no debe declararse detenido mientras la herramienta de ejecución no lo confirme. Siguiente: commit y publicación exclusiva en `frontend-next-hermes`, manteniendo la limitación del contenedor efímero en el registro.
