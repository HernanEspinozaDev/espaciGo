# KYC-13 — Revalidación integral, matriz de aceptación y cierre local (2026-11-08)

## 1. Alcance, entorno y exclusiones explícitas

- **Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`
- **Rama:** `codex/marketplace/fix/kyc-06-save-identity-first`
- **HEAD base:** `274f37e483a078890e92382a159fa49fd23c2fe8`
- **Propósito:** Cerrar la revalidación integral local **KYC-13** sobre las fases correctivas **KYC-08 a KYC-12**, consolidando la matriz verificable `criterio -> test -> entorno/capa -> SHA base -> resultado` exigida por §§10, 13 (R01–R14) y 15 del plan.
- **Separación estricta de capas y exclusiones:**
  1. **Go + Criptografía + HTTP (`-race -count=1`):** verificado con reloj determinista, keyring AES-256-GCM con AAD y stubs inyectados sin tráfico externo.
  2. **PostgreSQL efímero aislado (`scripts/test-migration-runner-postgres.sh full`):** contenedor desechable `postgis/postgis:18-3.6` con `--network none`, sin puertos publicados ni volúmenes persistentes; aplica `V000001..V000087` en limpio y desde snapshots `V75`/`V76`, ejercitando `identitydeclarationpg.Repository` y `verification.DiditService` bajo `SET ROLE espacigo_runtime` y la purga acotada bajo `SET ROLE espacigo_privacy_cleanup_worker`.
  3. **Navegador / BFF Next.js + Playwright + Axe:** servidor standalone local en loopback (`127.0.0.1:4219`) con intercepción de rutas y guardia de red que falla ante cualquier request fuera de loopback (salvo la redirección mockeada e interceptada a `https://verify.didit.me`).
  4. **Sandbox externo real de Didit y entorno Live:** **NO EJECUTADOS** en este corte. Se preservó consumo cero de cuota externa de Didit. No se aplicaron migraciones pendientes (`V86`/`V87`) ni el diseño futuro de cascada/claims (`V88` pendiente de reconciliación de ledger) sobre el PostgreSQL nativo persistente (`espacigo_dms_stage_20261020`) ni sobre Cloud SQL. El diagnóstico/despliegue del pipeline (`KYC-14` / `R15`) permanece separado.

---

## 2. Matriz de criterios de aceptación (§10 y §14)

| Criterio (§10 / §14) | Prueba automatizada verificable | Capa / Entorno | SHA base | Resultado |
| --- | --- | --- | --- | --- |
| **C01.** Formulario chileno: DV numérico/K, normalización Módulo 11, mayoría de edad (18+), nombres/apellidos con tildes/Ñ/guiones, país `CHL` y documento `ID` | `TestNormalizeRut`, `TestValidateIdentityDeclarationInput`, `TestValidateAdultBirthDate` (`internal/verification/declaration_test.go`); `Didit verification-api & RUT contract` (`frontend/tooling/verification-api-contract.test.mjs`) | Go unit (`-race -count=1`) + Node test runner | `274f37e` + working tree | **PASS** |
| **C02.** No crear sesión con datos inválidos, menores de 18 años o sin consentimiento explícito; cuenta derivada del principal autenticado; acceso cruzado denegado | `TestDiditV85HandlerWithDeclarationAndWebhook` (`handler_didit_test.go`), `TestIdentityDeclarationRoutesRequireAuthenticatedHandlerConfiguration` (`handler_declaration_test.go`), Playwright `flujo KYC de 2 pasos` (`phase6-account.spec.ts`) | Go HTTP (`-race -count=1`) + Playwright Chromium loopback | `274f37e` + working tree | **PASS** |
| **C03.** Flujo en dos acciones separadas con readback autoritativo: `Guardar datos de identidad` (`id-consent-20261108`, cero llamadas a Didit) -> resumen confirmado con `Editar datos` -> `Verificar identidad con Didit` (`didit-consent-v1` + `Idempotency-Key`) | `TestDiditV85HandlerWithDeclarationAndWebhook` (`handler_didit_test.go`), Playwright `flujo KYC de 2 pasos` (`phase6-account.spec.ts`) | Go HTTP + Playwright Chromium loopback | `274f37e` + working tree | **PASS** |
| **C04.** Snapshot cifrado estable (`AES-256-GCM` con AAD ligado a `account_id`, `revision`, `consent_version`, `consent_given_at`); misma `Idempotency-Key` o intento activo reanuda sin duplicar sesión en Didit | `TestKeyringSealOpenAndAADBinding` (`declarationcrypto/keyring_test.go`), `TestDiditServiceFullLifecycleAndSecurityInvariants` (`didit_service_test.go`), `assertV85DeclarationRepository` (`identity_schema_v85_test.go`) | Go (`-race -count=1`) + PostgreSQL 18 efímero (`SET ROLE espacigo_runtime`) | `274f37e` + working tree | **PASS** |
| **C05.** Evaluación *fail-closed*: documento distinto de `ID`/`CHL`, RUT/DV o fecha discordante, nombres incompatibles, liveness/face match no aprobados, warnings críticos o `d=nil` nunca otorgan `VERIFIED` | `TestEvaluateDecisionFailClosed`, `TestEvaluateDecisionNilAndMissingNodesFailClosed` (`didit_test.go`), `TestDiditWebhookScenarios` (`handler_didit_test.go`) | Go (`-race -count=1`) | `274f37e` + working tree | **PASS** |
| **C06.** Webhook endurecido: `X-Signature-V2` (canonicalización recursiva Unicode/float), `X-Signature-Simple`, fallback raw-body, rotación con secreto previo, ventana temporal de 300s, correlación persistida y deduplicación por `event_id` sin guardar cuerpo crudo | `TestVerifyWebhookSignature`, `TestVerifyWebhookSignatureV2UnicodeAndFloatCanonicalization` (`didit_test.go`), `TestDiditWebhookFailsClosedWithoutConfiguredSecretAndValidTimestamp` (`handler_didit_test.go`), `assertV85DeclarationRepository` (`identity_schema_v85_test.go`) | Go (`-race -count=1`) + PostgreSQL 18 efímero | `274f37e` + working tree | **PASS** |
| **C07.** Retorno seguro `/verificaciones/retorno`: limpia query params (`window.history.replaceState`) antes de consultar red; consulta solo intento propio; error de red muestra «No pudimos consultar el estado» y botón «Consultar estado nuevamente»; fallo técnico (`FAILED`) no se etiqueta como rechazo de identidad | Playwright `flujo KYC de 2 pasos` y `aislamiento A->logout->B->logout->A, conflicto 409...` (`phase6-account.spec.ts`) | Playwright Chromium loopback | `274f37e` + working tree | **PASS** |
| **C08.** Supersesión, revocación y expiración TTL (30 min): editar y guardar una nueva revisión de declaración invalida la aprobación previa (`VERIFIED` -> `UNVERIFIED`); intento pendiente vencido (>30 min) expira y permite nuevo intento; entorno `SANDBOX` nunca otorga `VERIFIED` en modo `LIVE` | `TestDiditServiceFullLifecycleAndSecurityInvariants` (`didit_service_test.go`) | Go (`-race -count=1`) | `274f37e` + working tree | **PASS** |
| **C09.** Unicidad de RUT (1 RUT por cuenta): colisión entre cuentas distintas devuelve `409 Conflict` (`rut_already_claimed`) con mensaje uniforme sin enumeración ni llamada a Didit; misma cuenta puede reanudar/reverificar | `TestDiditServiceFullLifecycleAndSecurityInvariants`, `TestDiditV85HandlerWithDeclarationAndWebhook/5_cross_account_rut_claim_rejected_on_start_intent`, Playwright `aislamiento A->logout->B->logout->A` (`phase6-account.spec.ts`) | Go (`-race -count=1`) + Playwright Chromium loopback | `274f37e` + working tree | **PASS** |
| **C10.** Privacidad, aislamiento de sesión (`A -> logout -> B -> logout -> A`), borrado inmediato post-`VERIFIED` y retención acotada: cero RUT/fecha en `localStorage`/`sessionStorage`/URL; solicitud de borrado remoto a Didit vía outbox tras `VERIFIED`; purga SQL acotada a 100 filas bajo `espacigo_privacy_cleanup_worker` | `TestDiditServiceFullLifecycleAndSecurityInvariants`, `assertV85DeclarationRepository` (`identity_schema_v85_test.go`), `assertV87IdentityDeclarationRetentionAndWorker` (`identity_schema_v86_test.go`), Playwright `aislamiento A->logout->B->logout->A` (`phase6-account.spec.ts`) | Go + PostgreSQL 18 efímero + Playwright Chromium loopback | `274f37e` + working tree | **PASS** |
| **C11.** Separación de dominio y accesibilidad: copy sin atribuir fixtures a Didit ni prometer reservas/cobros automáticos; jerarquía de encabezados y contraste Axe sin violaciones en escritorio (`1365x900`) y móvil (`390x844`) | Playwright `@phase6 @a11y account routes fit both viewports and have no Axe violations` y `visualización de elegibilidad KYC/KYB...` (`phase6-account.spec.ts`) | Playwright Chromium loopback + `@axe-core/playwright` | `274f37e` + working tree | **PASS** |

---

## 3. Estado de cierre del registro de brechas R01–R15 (§13)

| ID | Prioridad | Estado en código y pruebas locales | Evidencia verificable |
| --- | --- | --- | --- |
| **R01** | P1 | **Cerrado localmente** (KYC-09, KYC-12) | Declaración legal separada de `display_name`; flujo en dos acciones con readback en `verification-flow.tsx` y `phase6-account.spec.ts`. |
| **R02** | P1 | **Cerrado localmente** (KYC-09, KYC-10, KYC-11) | Repositorio PostgreSQL V85 (`identitydeclarationpg.Repository`) para declaraciones, intentos y eventos; probado tras reinicio de servicio en `cmd/api/identity_declaration_runtime_test.go` y en PostgreSQL 18 efímero (`identity_schema_v85_test.go`). |
| **R03** | P1 | **Cerrado localmente** (KYC-09, KYC-12) | OpenAPI regenerado (`202` operaciones), UUID v4 canónico, mapeo `badge_result` -> `badge_status` y pruebas de contrato (`verification-api-contract.test.mjs`). |
| **R04** | P0 | **Cerrado localmente** (KYC-08, KYC-09) | Principal autenticado obligatorio en creación/lectura de declaración e intento; denegación cruzada probada en `handler_didit_test.go` y `handler_declaration_test.go`. |
| **R05** | P1 | **Cerrado localmente** (KYC-08, KYC-10) | Eliminado fallback de sesión/URL inventada; `NewHandler` sin servicio falla cerrado (`503`); errores de proveedor devuelven `502` sin crear éxito falso. |
| **R06** | P0 | **Cerrado localmente** (KYC-08, KYC-11) | Eliminado secreto fallback embebido; timestamp obligatorio con ventana de 300s; soporte `X-Signature-V2`, `X-Signature-Simple`, raw-body y clave previa de rotación. |
| **R07** | P0 | **Cerrado localmente** (KYC-08, KYC-11) | Eliminada asignación al único intento sin correlación y prohibido `APPROVED` sin decisión autoritativa completa validada por `EvaluateDecision`. |
| **R08** | P0 | **Cerrado localmente** (KYC-11) | Inbox durable en `identity_provider_event` con deduplicación por `(provider, environment, event_id)`, consulta autoritativa `GetSessionDecision`, bloqueo de `SANDBOX` en runtime `LIVE`, supersesión por nueva revisión y borrado remoto vía outbox. |
| **R09** | P1 | **Cerrado localmente** (KYC-10) | Cliente `HTTPDiditClient` migrado al contrato oficial Didit v3 (`https://verification.didit.me/v3/session/`, `expected_details`, `DELETE /v3/session/{id}/delete/`) y validado con pruebas unitarias/HTTP. |
| **R10** | P1 | **Cerrado localmente** (KYC-11, KYC-12) | `account-verification-card.tsx` y `mi-cuenta/verificaciones/page.tsx` separan la identidad personal de la acreditación documental de espacios y eliminan promesas de arriendos/cobros o atribuciones falsas a Didit. |
| **R11** | P1 | **Cerrado localmente** (KYC-10, KYC-12) | `/verificaciones/retorno` limpia query params antes de red, consulta por `attempt_id` propio, distingue fallo de red (`No pudimos consultar el estado` + reintento) y fallo técnico (`FAILED`) sin aprobar desde query; aislamiento `A->logout->B->logout->A` probado en Playwright. |
| **R12** | P1 | **Cerrado localmente** (KYC-09) | Decoder estricto (`DisallowUnknownFields`), validación `ValidateAdultBirthDate` (18+), longitudes alineadas y `id-consent-20261108` (19 chars <= `varchar(20)`). |
| **R13** | P1 | **Cerrado localmente** (KYC-08) | Guard nil al inicio de `EvaluateDecision` probado en `TestEvaluateDecisionNilAndMissingNodesFailClosed`. |
| **R14** | P2 | **Cerrado localmente** (KYC-13) | Este documento (`evidencia_kyc13_20261108.md`) y la actualización de §§9, 13 y 15 en `plan_kyc_didit.md` reemplazan conteos generales por la matriz trazable por criterio, capa y SHA. |
| **R15** | P2 | **Pendiente en KYC-14** | Diagnóstico read-only del pipeline Cloud Build/Cloud Run y recibo `build -> SHA -> digest -> revisión -> tráfico` pertenecen al gate separado de publicación/despliegue (`KYC-14`). |

---

## 4. Comandos ejecutados y resultados verificados

1. **Backend Go (compilación, análisis estático y pruebas con detector de carreras sin caché):**
   - `GOTOOLCHAIN=local go vet ./internal/verification/... ./internal/adapters/postgres/identitydeclaration/... ./internal/migrator/... ./cmd/api/... ./cmd/privacy-cleanup-worker/...` -> **PASS** (exit 0).
   - `GOTOOLCHAIN=local go test -race -count=1 ./internal/verification/... ./internal/adapters/postgres/identitydeclaration/... ./cmd/api/... ./cmd/privacy-cleanup-worker/...` -> **PASS** (exit 0).
2. **Harness PostgreSQL 18 efímero aislado (`V1..V87` + contratos de runtime y worker):**
   - `GOTOOLCHAIN=local bash scripts/test-migration-runner-postgres.sh full && python3 scripts/gcp/test_check_migration_ci.py` -> **PASS** (120 resultados DB requeridos, 0 omitidos, contenedor efímero sin red eliminado y verificado; 4 pruebas Python de CI en verde).
3. **Frontend Next.js (contrato OpenAPI, tipos, lint, formato, tests de herramientas, build de producción y Playwright E2E + Axe):**
   - `node tooling/api-generate.mjs && node tooling/generate-api-inventory.mjs && node tooling/generate-api-allowlist.mjs && node tooling/api-check.mjs` -> **PASS** (`202` operaciones sincronizadas).
   - `./node_modules/.bin/tsc --noEmit` -> **PASS** (0 errores).
   - `./node_modules/.bin/prettier --check .` -> **PASS**.
   - `./node_modules/.bin/eslint src playwright.config.ts playwright.public-calendar.config.ts tests tooling next.config.ts eslint.config.mjs` -> **PASS** (0 errores).
   - `node --test tooling/*.test.mjs` -> **PASS** (`482` pruebas en verde, 0 fallos).
   - `pnpm build && PLAYWRIGHT_PORT=4219 ./node_modules/.bin/playwright test tests/phase6-account.spec.ts tests/verification-flow-ux.spec.ts` -> **PASS** (`29` pruebas E2E + Axe en verde en escritorio y móvil).
4. **Higiene Git:**
   - `git diff --check` -> **PASS** (0 errores).

---

## 5. Límites y pendientes antes de KYC-14 / Producción

- **Migración append-only futura (`V000088`):** las migraciones `V000001..V000087` se conservaron intactas byte a byte. La cascada transaccional en la función SQL de purga (`identity_provider_event` -> `identity_provider_attempt` -> `identity_declaration_revision`), la tabla SQL para claims HMAC de RUT y el `REVOKE EXECUTE` sobre `espacigo_runtime` se diseñarán y aplicarán en una migración append-only posterior únicamente tras reconciliar el ledger del destino y recibir autorización expresa de aplicación.
- **Sandbox real externo y Live (`KYC-14`):** no se invocó la API externa de Didit ni se modificaron bases persistentes ni Cloud SQL. La publicación de rama, integración a `main` y despliegue en Cloud Run requieren autorización explícita del usuario.
