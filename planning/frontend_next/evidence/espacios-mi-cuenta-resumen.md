# Evidencia — resumen de espacios en Mi Cuenta

**Fecha:** 2026-10-27  
**Rama/HEAD:** `main` / `3956d2f`  
**Alcance:** componente Next.js y navegación; E2E con BFF simulado.

## Implementación

- `/mi-cuenta` incorpora `AccountSpacesCard`; usa `useAuth()` y `useHostListings()` para pedir el listado por la fachada existente cuando la sesión está autenticada. No hace REST directo ni contiene espacios de demostración.
- Resume los estados vigentes del tipo frontend/Go: `borrador`, `activa` (presentado como publicado) y `oculta` (etiqueta UI «archivado», alineada con el listado). Las pruebas verifican conteos plurales (2 borradores, 1 publicado y 1 archivado) y singulares (1 de cada estado). Excluye `eliminada`; no inventa un estado de dominio `archivado`.
- Incluye y prueba estados de carga, error con reintento, vacío, además de enlaces a `/anfitrion/espacios/nuevo` y `/anfitrion/espacios`.

## Verificación

- `pnpm build`: PASS (Next.js 16.4.3).
- Playwright `tests/phase6-account.spec.ts` + `tests/phase7-host-listings.spec.ts`: **15/15 PASS**, incluyendo carga diferida, error 503, retry, estado vacío, etiquetas singulares/plurales, CTAs, Axe y ausencia de overflow en desktop 1365×900 y mobile 390×844.
- `pnpm test`: **260/260 PASS**.
- `pnpm lint`, `pnpm typecheck`, `pnpm format:check`, `git diff --check`: PASS.
- Playwright usa mocks locales; no demuestra recorrido real hacia Go ni persistencia.

## Límites

No se modificaron Go, OpenAPI, SQL o DB. El PostgreSQL nativo seleccionado `127.0.0.1:5432` está autenticado y fue inspeccionado solo en transacciones `READ ONLY`; no está probado qué proceso Go local usa ese target. No se creó cuenta ni se hicieron escrituras. No hay commit, push ni despliegue. F3 sigue abierta para completar cobertura de perfiles/casos y validar el recorrido visible con persistencia real; la integración real permanece pendiente.
