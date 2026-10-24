# Frontend Next.js — reglas de desarrollo

Estas instrucciones se aplican a `frontend/` y complementan el `AGENTS.md` raíz. La instrucción humana más reciente prevalece.

## Alcance vigente

- El frontend Angular fue rechazado y será sustituido por una aplicación Next.js nueva.
- Trabajar exclusivamente en la rama `frontend-next-hermes` y en su worktree aislado.
- Para esta rama, el usuario autorizó a `HernanEspinozaDev` a desarrollar, commitear y publicar directamente.
- No escribir en `main`, abrir PR, fusionar, desplegar ni probar online sin autorización posterior expresa.
- Todo desarrollo y prueba debe ejecutarse localmente con datos sintéticos.
- No usar el navegador o las pestañas del usuario; Playwright solo contra URL y puerto locales comprobados.
- No modificar backend, SQL, Terraform, IAM, GCP, volúmenes o datos persistentes por inferencia desde una tarea frontend.

## Stack

- Next.js estable con App Router.
- React y TypeScript estricto.
- pnpm fijado mediante `packageManager` y `pnpm-lock.yaml`.
- Server Components por defecto; Client Components solo cuando requieran estado, eventos o APIs del navegador.
- No usar versiones canary salvo autorización expresa y evidencia de necesidad.

## Arquitectura

- Construir una sola aplicación Next.js con route groups para marketplace público, identidad, cuenta, anfitrión, reservas y administración.
- Mantener el flujo `UI → aplicación/fachada → puerto → adaptador → cliente generado → gateway → Go` en features con lógica.
- Componentes y páginas no construyen rutas REST ni importan directamente transporte generado.
- Go es la única autoridad para identidad, autorización, propiedad, capacidades, disponibilidad, precios, estados y transiciones.
- No introducir PostgreSQL, ORM, autenticación, pagos o persistencia paralelos en Next.js.
- Generar el cliente desde `planning/openapi.yaml`; no editar archivos generados.

## Seguridad y sesión

- El Bearer opaco vive solo en memoria.
- Prohibido guardar tokens en cookies propias, `localStorage`, `sessionStorage`, IndexedDB, URLs, HTML SSR o logs.
- No inventar refresh tokens, JWT o sesiones NextAuth/Auth.js sin un contrato nuevo aprobado.
- Las áreas privadas usan shell más CSR autenticado; nunca compartir Bearer entre solicitudes SSR.
- Navegación y controles administrativos dependen de capacidades confirmadas por Go, no solo de roles.
- Respetar `restricted_mode`, revocaciones, respuestas tardías y expiración por generación de sesión.
- No registrar cuerpos sensibles, credenciales, secretos ni cabeceras Authorization.

## Contrato y datos

- JSON de request debe ser exacto: no enviar propiedades internas de UI.
- Controlar errores por `error.code`, con fallback seguro por status y `X-Request-ID`.
- Preservar `Retry-After` e información de validación disponible.
- Reutilizar `Idempotency-Key` y payload durante reintentos de la misma intención.
- Montos CLP exactos: no usar `float` ni conversiones inseguras de `int64`.
- Go decide cotizaciones, vencimientos, disponibilidad y reglas de calendario.
- Usar RFC 3339 y zonas IANA; cubrir `America/Santiago` y DST.
- Cursores son opacos y no se reutilizan tras cambiar filtros.
- Binarios y descargas privadas usan Blob URLs temporales, revocadas al cambiar sesión/recurso o desmontar.

## Plantillas

- Your Next Store y Next.js SaaS Starter son referencias visuales y estructurales, no dependencias de dominio.
- Clonarlas únicamente bajo `.local/frontend-template-sources/`, ignorado por Git.
- No copiar `.git`, lockfiles, secretos, configuraciones de despliegue ni instrucciones AGENTS de las plantillas.
- No incorporar Commerce Kit, YNS, Bun, Drizzle, JWT/cookies propios, bcrypt, Stripe, billing o equipos SaaS.
- Registrar atribuciones MIT en `THIRD_PARTY_NOTICES.md` cuando se copie código reconocible.

## Marca

Del frontend anterior solo se permite recuperar `public/espacigo-mark.svg` y favicon derivados de esa marca. No restaurar componentes, estilos, páginas, lógica Angular o SDK Angular.

## UI y accesibilidad

- Marketplace público y aplicación SaaS comparten tokens y componentes base, aunque tengan densidades distintas.
- Componentes pequeños, composición explícita y props tipadas; evitar `any`.
- HTML semántico, landmarks, skip link, labels persistentes y foco visible.
- Navegación completa por teclado y objetivos táctiles adecuados.
- No depender solo del color; respetar movimiento reducido.
- Objetivo WCAG 2.2 AA y AXE sin violaciones conocidas en rutas principales.
- Imágenes con dimensiones/sizes correctos y prioridad solo para LCP real.

## Calidad

Antes de cerrar un corte ejecutar según corresponda:

```bash
pnpm install --frozen-lockfile
pnpm lint
pnpm typecheck
pnpm test
pnpm format:check
pnpm build
```

Cuando existan los harness correspondientes, añadir Playwright, AXE, validación OpenAPI, arranque local y Docker.

- Los comandos de CI deben comprobar, no reescribir archivos.
- No elevar presupuestos o desactivar reglas para ocultar fallos.
- No afirmar integración por mocks o capturas: distinguir unitario, presentación, API local real y E2E.
- Registrar HEAD, pruebas, resultados, límites y siguiente paso en la evidencia de cada fase.
