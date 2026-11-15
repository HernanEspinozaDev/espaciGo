# KYC-10, KYC-11 y alineación contractual frontend — evidencia local (2026-11-08)

## 1. Alcance y referencia

- **Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`
- **Rama:** `codex/marketplace/fix/kyc-06-save-identity-first`
- **HEAD base:** `274f37e483a078890e92382a159fa49fd23c2fe8` (cambios en árbol de trabajo local)
- **Restricciones respetadas:** solo datos sintéticos y mocks/stubs locales (`httptest.Server` y repositorios de prueba). Cero llamadas a la API real de Didit, cero consumo de cuota externa, sin conexión ni DDL sobre PostgreSQL persistente o Cloud SQL, y sin publicación ni despliegue.

## 2. Implementación realizada

### 2.1 Cliente y evaluación autoritativa Didit V3 (`internal/verification/didit.go`)
- Se actualizó el cliente HTTP a las rutas V3 de Didit:
  - `POST /v3/session/` (`CreateSession`) con `workflow_id`, `vendor_data` opaco (UUID de correlación, sin RUT ni PII), `callback` y `expected_details` desde el snapshot declarado en el servidor.
  - `GET /v3/session/{sessionId}/decision/` (`GetSessionDecision`) para recuperar la decisión autoritativa server-to-server.
  - `DELETE /v3/session/{sessionId}/delete/` (`DeleteSession`) para solicitar el borrado inmediato de medios en el proveedor tras alcanzar `VERIFIED` (con outbox en memoria para reintento si la llamada de borrado falla).
- `EvaluateDiditDecision` evalúa de forma fail-closed los arreglos V3 (`id_verifications[]`, `liveness_checks[]`, `face_matches[]`), exige documento chileno (`CHL`, `ID` / `IDENTITY_CARD`), normaliza y valida el RUN/RUT extraído con Módulo 11, compara nombres/apellidos normalizados (Unicode NFD / sin diacríticos) y fecha civil `YYYY-MM-DD`, y rechaza o envía a `IN_REVIEW` ante cualquier discrepancia o warning crítico (`FULL_NAME_MISMATCH_WITH_PROVIDED`, `DOB_MISMATCH_WITH_PROVIDED`, `COUNTRY_MISMATCH_WITH_PROVIDED`, `IDENTIFICATION_NUMBER_MISMATCH_WITH_PROVIDED`).

### 2.2 Servicio de dominio KYC-10 / KYC-11 (`internal/verification/didit_service.go`)
- **Inicio y reanudación idempotente (`StartOrResume`):**
  - Requiere declaración vigente guardada en `DeclarationService` (`409 conflict` si no existe o si los campos enviados difieren del snapshot guardado) y mayoría de edad (`ValidateAdultBirthDate` 18+ UTC).
  - Soporta versión de consentimiento separada de Didit (`didit-consent-v1`) y verifica versión de consentimiento de declaración (`id-consent-20261108`).
  - Deriva UUID determinista por `(account_id, declaration_id, idempotency_key)` para deduplicar reintentos y doble clic sin crear sesiones duplicadas en Didit.
  - Valida estrictamente que `redirect_url` sea HTTPS bajo `verify.didit.me` o `verification.didit.me`; la URL de sesión se mantiene solo en memoria efímera y nunca se persiste en PostgreSQL.
  - Expiración determinista de intentos estancados tras el TTL configurado (30 minutos por defecto).
- **Guardia de unicidad de RUT (`RutClaimStore` + HMAC-SHA256):**
  - Calcula `ComputeRutHMAC` (`HMAC-SHA256` con prefijo de dominio y `key_version`) sin almacenar el RUT en claro.
  - Bloquea que una cuenta B inicie o apruebe verificación con un RUT ya reclamado como `VERIFIED` por una cuenta A (`409 rut_already_claimed`), y libera el reclamo en revocación.
- **Inbox durable de webhooks, decisión cifrada y estado efectivo (`ProcessWebhook`, `GetAttemptForAccount`, `RevokeAttempt`):**
  - Registra cada entrega autenticada en `identity_provider_event` con `body_sha256` (32 bytes, sin almacenar el payload ni medios del webhook) y deduplica entregas repetidas.
  - Nunca aprueba por el estado superficial del webhook: consulta `GetSessionDecision`, ejecuta `EvaluateDiditDecision` contra la declaración descifrada con AAD vinculado a `(account_id, declaration_id, revision, schema_version)`, y sella el resumen mínimo de decisión con AES-256-GCM y AAD vinculado a `(account_id, attempt_id, declaration_id, schema_version)` (`BuildDecisionAAD`).
  - Al confirmar `APPROVED`, registra el reclamo HMAC del RUT, purga la URL efímera e invoca `DeleteSession` (con cola de reintento `RetryPendingSessionDeletions` si falla la red).
  - Soporta `RequireLiveForVerifiedBadge` para impedir que sesiones `SANDBOX` otorguen insignia `VERIFIED` productiva cuando la política lo exija, y soporta revocación (`RevokeAttempt`) retirando inmediatamente `VERIFIED`.

### 2.3 Repositorio PostgreSQL V85 y transporte HTTP (`internal/adapters/postgres/identitydeclaration/provider_attempt.go`, `internal/verification/transport/http/handler_didit.go`, `cmd/api`)
- Se implementaron las operaciones SQL sobre `public.identity_provider_attempt` y `public.identity_provider_event` respetando los permisos de mínimo privilegio de `espacigo_runtime` definidos en V85.
- `handler_didit.go` delega en `DiditService` cuando está configurado, valida firma `X-Signature-V2` / `X-Signature-Simple` y ventana de frescura `X-Timestamp`, sirve tanto `/api/v1/webhooks/didit` (contrato OpenAPI) como `/api/v1/verifications/didit/webhook`, y mantiene `Cache-Control: no-store`.
- `cmd/api/identity_declaration_runtime.go` y `cmd/api/main.go` cablean `newRuntimeDiditService` de forma fail-closed.

### 2.4 Contrato y flujo frontend KYC-12 (`frontend/src/features/verifications/*`, `frontend/src/features/account/ui/account-verification-card.tsx`, `frontend/src/app/(workspace)/mi-cuenta/verificaciones/page.tsx`, `frontend/src/app/verificaciones/retorno/page.tsx`)
- `verification-api.ts` alinea tipos y funciones con `DiditIntentResponse`, `DiditIntentStatusResponse` (`badge_result` -> `badge_status`), `IdentityDeclarationPayload` y `IdentityDeclarationResponse`, validando UUID canónico y allowlist HTTPS de redirección Didit.
- `verification-flow.tsx` separa el flujo en dos acciones con readback real desde Go (`Guardar datos de identidad` con `id-consent-20261108` -> resumen confirmado con `Editar datos` -> `Verificar identidad con Didit` con `didit-consent-v1` e `Idempotency-Key`), sin modificar ni deducir nombres desde `displayName` público, y aísla el estado privado por `accountId` autenticado (`key={accountScopeKey}`) purgando el formulario al cerrar sesión o cambiar de cuenta (A -> logout -> B).
- `account-verification-card.tsx` y `mi-cuenta/verificaciones/page.tsx` eliminan atribuciones de verificación por fixture a Didit y retiran promesas de habilitar reservas/arriendos/cobros que pertenecen a gates manuales separados.
- `retorno/page.tsx` limpia los query params de la barra de direcciones antes de consultar la red y hace polling únicamente por `attempt_id` propio hasta estado terminal o `IN_REVIEW`.

## 3. Verificación ejecutada

- `GOTOOLCHAIN=local go vet ./internal/verification/... ./internal/adapters/postgres/identitydeclaration/... ./internal/migrator/... ./cmd/api/...` — **PASS**.
- `GOTOOLCHAIN=local go test -race -count=1 ./internal/verification/... ./internal/adapters/postgres/identitydeclaration/... ./internal/migrator/... ./cmd/api/...` — **PASS** (incluye `TestDiditServiceFullLifecycleAndSecurityInvariants` y `TestDiditV85HandlerWithDeclarationAndWebhook`).
- `GOTOOLCHAIN=local bash scripts/test-migration-runner-postgres.sh full && python3 scripts/gcp/test_check_migration_ci.py` — **PASS** (120 resultados DB requeridos en PostgreSQL efímero sin red ni volúmenes externos, incluyendo `DiditService` + `identitydeclarationpg.Repository` bajo `SET ROLE espacigo_runtime` en `assertV85DeclarationRepository`: `StartOrResume`, replay idempotente, `ProcessWebhook`, `GetStatus` y verificación de `encrypted_decision` sin texto plano).
- Frontend (`frontend/`):
  - `node tooling/api-generate.mjs && node tooling/generate-api-inventory.mjs && node tooling/generate-api-allowlist.mjs && node tooling/api-check.mjs` — **PASS** (202 operaciones sincronizadas).
  - `./node_modules/.bin/tsc --noEmit` — **PASS** (0 errores).
  - `./node_modules/.bin/prettier --check .` — **PASS**.
  - `./node_modules/.bin/eslint src playwright.config.ts playwright.public-calendar.config.ts tests tooling next.config.ts eslint.config.mjs` — **PASS** (0 errores).
  - `node --test tooling/*.test.mjs` — **PASS** (482 pruebas en verde, 0 fallos, incluyendo `tooling/verification-api-contract.test.mjs`).
  - `pnpm build && PLAYWRIGHT_PORT=4219 ./node_modules/.bin/playwright test tests/phase6-account.spec.ts tests/verification-flow-ux.spec.ts` — **PASS** (29 pruebas E2E en verde, incluyendo el flujo completo KYC-12 de 2 pasos: Mi cuenta -> Verificar cuenta -> bloqueo de menor de 18 años sin llamada de red -> Guardar datos de identidad sin llamada a Didit -> readback tras recarga -> Editar datos -> Verificar identidad con Didit con consentimiento `didit-consent-v1` e `Idempotency-Key` -> redirección interceptada -> `/verificaciones/retorno` con limpieza de URL y polling hasta `VERIFIED`, más aislamiento `A -> logout -> B -> logout -> A`, conflicto `409 rut_already_claimed` sin redirección, recuperación ante fallo de red `503` y fallo técnico `FAILED` en `/verificaciones/retorno`, y Axe sin violaciones en escritorio y móvil).
- `git diff --check` — **PASS**.
