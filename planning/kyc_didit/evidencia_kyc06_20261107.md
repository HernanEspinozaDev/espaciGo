# Evidencia KYC-06 — Revisión Integral, Regresión y Preparación Live

- **Fecha:** 2026-11-07.
- **Rama:** `codex/marketplace/feature/kyc-00-didit-identidad`.
- **Worktree:** `/home/nandev/Documentos/projects/espaciGo-kyc-didit`.
- **Base:** `origin/main` (`771134d165046b290b1f5ad3a1074a6357c2327f`).

## 1. Resumen de Fases Completadas (KYC-00 a KYC-06)

| Fase | Componente | Resultado verificado |
| :--- | :--- | :--- |
| **KYC-00** | Spike Técnico & ADR | ADR-001 ratificado; APIs de Didit comprobadas; herramientas MCP y CLI operativas. |
| **KYC-01** | Modelo DB & Contratos | Migración `V000084` con tablas para intentos, deduplicación de webhooks y auditoría; OpenAPI tipado. |
| **KYC-02** | Go Backend Core | Cliente Didit, cálculo algorítmico y normalización de RUN/RUT chileno, evaluación *fail-closed*. |
| **KYC-03** | Next.js / Frontend UI | Pantalla de consentimiento, validación interactiva de RUT con formato dinámico, redirección y retorno. |
| **KYC-04** | Integración End-to-End | Handlers HTTP de intenciones y consulta protegidos por RBAC, compatibilidad DTO. |
| **KYC-05** | Sandbox & Webhook Hub | Verificación de firma HMAC-SHA256, tolerancia anti-replay, 3 escenarios (Aprobado, Mismatch, Spoofing). |
| **KYC-06** | Revisión Integral & Regresión | Auditoría de privacidad/seguridad, costos/cuotas, suite de pruebas Go completa y sin regresiones. |

## 2. Auditoría de Seguridad, Privacidad y Cuotas

1. **Principio de Mínimo Privilegio y Almacenamiento Seguro:**
   - No se duplica biometría ni imágenes de documentos en bases de EspaciGo.
   - Los tokens de sesión y secretos se cargan exclusivamente desde entorno local privado (`.local/testing/didit_sandbox.env`, permisos `0600`).
   - Los parámetros sensibles (RUT, fecha de nacimiento) no viajan en URLs, query strings ni almacenamiento persistente de cliente (`localStorage`/`sessionStorage`).
2. **Defensa en Profundidad en Webhook Ingestion:**
   - Verificación de firma criptográfica HMAC en tiempo constante (`hmac.Equal`).
   - Ventana de tolerancia temporal máxima de 300 segundos contra ataques de repetición (Replay Attacks).
   - Deduplicación de eventos por `event_id`.
3. **Política de Decisión *Fail-Closed*:**
   - Advertencias de discrepancia (`warnings`), fallos de liveness o no coincidencia de RUT/nacimiento fuerzan el estado a `IN_REVIEW`, impidiendo otorgar la insignia verde por error.

## 3. Pruebas de Regresión Ejecutadas

```bash
go test -v -count=1 -race ./internal/verification/...
```

Resultados:
- `internal/verification`: PASS (0.00s / 1.033s con race detector)
  - `TestEvaluateDecisionFailClosed`
  - `TestVerifyWebhookSignature`
  - `TestNormalizeRut`
  - `TestSyntheticCaseReviewRetryAndSafeSerialization`
  - `TestReviewRejectsUnknownReasonAndApprovalReason`
- `internal/verification/transport/http`: PASS (1.033s con race detector)
  - `TestDiditIntentHTTPHandlers`
  - `TestDiditWebhookScenarios` (Escenario A, Escenario B, Escenario C)
  - `TestEvidenceAPIIsPrivateOwnerScopedAndExplicitlyCleanable`
  - `TestKYCAPIAuthOwnershipAndReview`
  - `TestKYCAPIRejectsUnknownFieldsAndQuery`
  - `TestVerificationHistoryEligibilityFailedQueueAndAdminRevocation`

- `git diff --check`: 0 errores.

## 4. Estado y Siguiente Gate (KYC-07)

- **KYC-07:** Integración, publicación y despliegue a producción permanecen **detenidos**, a la espera de autorización expresa del usuario humano y canónica de Cloud SQL / Cloud Run.
