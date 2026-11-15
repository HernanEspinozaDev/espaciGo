# Evidencia VI-04 — versión de consentimiento configurable entre frontend y Go

Fecha: 2026-11-13 (`America/Santiago`).

## Checkout y alcance

- Backend/plan: `/home/nandev/Documentos/projects/espaciGo-main-sync`, rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD base `274f37e483a078890e92382a159fa49fd23c2fe8`.
- Frontend autorizado: `/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes`, rama `frontend-next-hermes`, HEAD base `cf6e28a365d2495e78bc64c01686ed55efb17c8a`.
- El trabajo se hizo sobre worktrees compartidos y dirty; se conservaron los cambios existentes. No se atribuyen esos cambios previos a este subcorte.
- Alcance: resolver y caracterizar el desacople entre el default de consentimiento del cliente y `IDENTITY_DECLARATION_CONSENT_VERSION` configurable en Go. No se modificó el handler/runtime productivo Go ni OpenAPI.

## Cambios de este subcorte

- `frontend/src/features/identity-declaration/domain/identity-declaration.ts` obtiene la versión desde `NEXT_PUBLIC_IDENTITY_DECLARATION_CONSENT_VERSION` y conserva `id-consent-20261108` como fallback. Rechaza configuración vacía tras trim usando el fallback, versiones de más de 20 caracteres o con whitespace interno. El request y el parser de readback usan la versión resuelta.
- `frontend/tooling/identity-declaration-bff-go-integration.mjs` carga y ejecuta el módulo de dominio frontend, construye el request real del feature con la versión de prueba `id-consent-test-v2`, envía por la ruta Next BFF real al handler/servicio Go y comprueba POST→GET con el mismo valor. También envía el default antiguo, espera 422 y verifica las cabeceras privadas.
- `internal/verification/transport/http/identity_declaration_bff_go_integration_test.go` configura el `DeclarationService` sintético con `id-consent-test-v2`, pasa la misma configuración pública al proceso Node y comprueba cuerpos upstream, autenticación, lecturas y escrituras. El intento obsoleto no agrega escritura; el repositorio queda con una única revisión válida.
- `frontend/tests/identity-declaration.spec.ts` deriva los payloads/readbacks sintéticos del mismo valor de build para verificar el comportamiento visible bajo override.

## RED/GREEN observado

- RED: antes de hacer configurable el dominio frontend, el harness BFF→Go con servicio sintético configurado en `id-consent-test-v2` falló como se esperaba: el frontend envió su literal anterior y Go respondió 422 (`invalid_input`) en lugar del 201 esperado.
- GREEN: después del cambio, el request generado por el dominio llevó `id-consent-test-v2`, el POST/GET loopback pasó, el readback fue aceptado por el parser y el POST con `id-consent-20261108` siguió rechazándose con 422 sin una escritura adicional.

## Gates ejecutados

Backend, bajo `env -i` sin variables DB/PG:

- Focal BFF→Go `go test -race -tags=bffintegration -count=1 ... -run '^TestNextBFFOwnIdentityDeclarationPOSTThenGET$'` — PASS; Node 1/1.
- Suite focal combinada de handler y loopback BFF→Go con `-race` — PASS: cuatro tests `TestIdentityDeclarationHandler*` y el test BFF→Go.
- `go test -race -count=1 ./cmd/api -run '^TestIdentityDeclarationRuntimeUsesConfiguredConsentVersion$'` — PASS, 1/1; configura el env Go real, rechaza el valor anterior antes de escribir y lee el valor override.
- `go vet -tags=bffintegration ./internal/verification/transport/http` y `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt`, `git diff --check` y revisión explícita de whitespace de los archivos tocados — PASS.

Frontend autorizado:

- Con `NEXT_PUBLIC_IDENTITY_DECLARATION_CONSENT_VERSION=id-consent-test-v2`, `pnpm build` — PASS.
- Con el mismo env, Playwright `tests/identity-declaration.spec.ts` sobre el standalone recién construido — PASS, 2/2. Son E2E locales con red interceptada y cuentas sintéticas; comprueban payload y readback visibles.
- Con el env retirado para el conjunto default: `pnpm typecheck`, `pnpm test` — PASS, 112/112; `pnpm api:check` — PASS, inventario 191 operaciones; `pnpm lint` y `pnpm format:check` — PASS.
- `node --check tooling/identity-declaration-bff-go-integration.mjs` y Prettier focal — PASS.
- El servidor E2E terminó; los puertos locales 4201/4203 quedaron libres.

## Límites y estado

El override frontend es público y queda incorporado al bundle durante el build: quien construya Next debe configurar `NEXT_PUBLIC_IDENTITY_DECLARATION_CONSENT_VERSION` con el mismo valor que `IDENTITY_DECLARATION_CONSENT_VERSION` del runtime Go. No se agregó una negociación dinámica cliente-servidor ni una comprobación de configuración de despliegue; cambiar solo el env Go después del build puede volver a causar incompatibilidad. El E2E valida UI con mocks; el harness separado recorre Next BFF y handler/servicio Go reales, pero usa autenticación, keyring, reloj, IDs y repositorio sintéticos en memoria. No prueba PostgreSQL, autorización persistida, durabilidad/reinicio, despliegue ni legalidad del consentimiento.

La revisión source-only `deleg_7f080036` aprobó este delta sin hallazgos; no ejecutó pruebas. Observó que el E2E conserva fallback al default si no recibe el env. En la ejecución registrada, `NEXT_PUBLIC_IDENTITY_DECLARATION_CONSENT_VERSION=id-consent-test-v2` se pasó explícitamente tanto a `pnpm build` como a Playwright, por lo que sí se verificó el override en ese recorrido; futuras ejecuciones de override deben conservar ambos envs.

No se ejecutó SQL/DDL ni se accedió a bases persistentes, Cloud, proveedores externos, cuentas o datos reales. Sin commit, push, merge, deploy ni integración a `main`. VI-04 y el MVP permanecen parciales.
