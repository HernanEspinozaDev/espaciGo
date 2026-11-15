# Evidencia VI-04: declaración de identidad por BFF→Go loopback

**Fecha:** 2026-11-13 (`America/Santiago`)

## Alcance y checkouts

Se caracterizó localmente el guardado y la lectura propia de la declaración de identidad a través del export real de la ruta Next BFF, el allowlist/operación OpenAPI generados, la ruta HTTP Go y `DeclarationService`. El test Go es `TestNextBFFOwnIdentityDeclarationPOSTThenGET`, bajo build tag `bffintegration`; el harness Node está en `frontend/tooling/identity-declaration-bff-go-integration.mjs`.

- Backend/plan: `/home/nandev/Documentos/projects/espaciGo-main-sync`, rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD `274f37e483a078890e92382a159fa49fd23c2fe8`.
- Frontend autorizado: `/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes`, rama `frontend-next-hermes`, HEAD `cf6e28a365d2495e78bc64c01686ed55efb17c8a`.
- Ambos worktrees ya tenían cambios locales amplios. Los archivos de esta característica/harness permanecen en el worktree y no se atribuyen a esos HEAD ni a cambios ajenos.

El harness envía las solicitudes por loopback no vacío `127.0.0.1` con `httptest`; usa el handler y servicio Go reales, pero autenticador, repositorio en memoria, generador de IDs, keyring sintético y reloj fijo. La prueba crea una declaración y luego hace un GET separado de lectura confirmada.

## Comportamiento verificado

- El POST usa el método/ruta y JSON contract-shaped exactos, incluyendo los campos de declaración, consentimiento explícito y versión `id-consent-20261108`; el GET posterior devuelve el readback esperado.
- El propietario se deriva del principal sintético autenticado; el request no elige una cuenta. Se verifica la normalización del RUT en el servicio y una respuesta con allowlist exacta, sin campos adicionales.
- Se verifican Bearer upstream, ausencia de Cookie reenviada, `private, no-store`, ausencia de `Set-Cookie`, y rechazo de Cookie entrante sin una llamada upstream adicional.
- Las pruebas del handler cubren el ciclo POST→GET, consentimiento vigente obligatorio antes de escribir, rechazo de menor de edad y no revelar la declaración de otra cuenta.

La revisión independiente read-only `deleg_89b4bcdb` aprobó (`passed: true`, sin preocupaciones de seguridad ni errores lógicos). Sugirió considerar una prueba separada para el override de versión de consentimiento si se considera parte del contrato. El runtime Go admite `IDENTITY_DECLARATION_CONSENT_VERSION`; el candidato del frontend usa la constante `id-consent-20261108`. Esta verificación cubre el valor por defecto, no la sincronización bajo un override.

## Gates parent-owned

En el backend, con entorno limpio y sin variables de base de datos/PG:

```text
env -i PATH="$PATH" HOME="$HOME" TMPDIR="/home/nandev/.hermes/cache/scratch" ESPACIGO_FRONTEND_ROOT="/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes/frontend" go test -race -tags=bffintegration -count=1 ./internal/verification/transport/http -run '^TestNextBFFOwnIdentityDeclarationPOSTThenGET$' -v
```

Resultado: PASS; el subtest Node reportó 1/1. También pasó, con `-race`, el filtro `^TestIdentityDeclarationHandler`: 4/4.

En el checkout frontend, parent-verified:

- `node --check tooling/identity-declaration-bff-go-integration.mjs` y Prettier focal — PASS.
- `pnpm test` — 112/112; `pnpm api:check` — PASS, inventario de 191 operaciones.
- `pnpm typecheck`, `pnpm lint`, `pnpm format:check` — PASS en la verificación del candidato; tras la última edición test-only, el E2E focal, typecheck, ESLint focal y Prettier focal volvieron a pasar.
- `pnpm build` — PASS antes de la última edición, que fue exclusivamente de aserciones de prueba; el artefacto standalone no requería reconstrucción.
- Playwright E2E en el árbol final — 87 PASS, 45 omitidas, 0 fallas (132 casos); el test A→B→logout→A actualizado pasó dentro de esta suite.

## Límites

Es caracterización local con autenticación y persistencia sintéticas. No demuestra autorización/grants productivos, propiedad consultada en PostgreSQL, atomicidad, durabilidad/reinicio, runtime standalone de producción, Cloud, ni ejecución real del override de consentimiento. Los mocks E2E de declaración reemplazan la red; el harness loopback alcanza la ruta BFF y handler Go con fakes, no un backend con datos persistidos.

No se ejecutó SQL/DDL ni se consultaron o modificaron bases persistentes/Cloud. No se accedió a proveedor, sitio público, cuentas o datos reales. Sin commit, push, despliegue o integración a `main`; VI-04 y el MVP siguen parciales.
