# Evidencia KYC-05 — Integración Sandbox y Procesamiento de Escenarios Webhook

- **Fecha:** 2026-11-07.
- **Rama:** `codex/marketplace/feature/kyc-00-didit-identidad`.
- **Worktree:** `/home/nandev/Documentos/projects/espaciGo-kyc-didit`.
- **Base:** `origin/main` (`771134d165046b290b1f5ad3a1074a6357c2327f`).

## 1. Conectividad y Configuración Sandbox Verificada

1. **Canal Seguro y Credenciales:**
   - Archivo local privado configurado con permisos `0600`: `.local/testing/didit_sandbox.env`.
   - API Key y Workflow ID verificados contra la API de Didit (`https://verification.didit.me` - HTTP 200).
   - Creación exitosa de sesión sintética inicial en Didit con token de sesión generado.
2. **Túnel y Registro de Webhook:**
   - Túnel seguro activo vía Cloudflare hacia `127.0.0.1:8080`.
   - Destino de webhook registrado en Didit Console:
     - UUID: `531360a2-31be-445a-9d14-52de29e48a69`
     - URL: `https://moon-mixed-performs-officially.trycloudflare.com/api/v1/verifications/didit/webhook`
     - Evento suscrito: `status.updated`

## 2. Implementación de Reconciliación y Seguridad Webhook

- Archivo: `internal/verification/transport/http/handler_didit.go`.
- **Validación Criptográfica Estricta (HMAC-SHA256):**
  - Soporte para cabeceras oficiales Didit `X-Signature`, `X-Signature-V2` y `X-Didit-Signature`.
  - Verificación en tiempo constante contra `DIDIT_WEBHOOK_SECRET`.
  - Ventana temporal de tolerancia anti-replay (`X-Timestamp` máx. 300 segundos).
- **Rutas Públicas Autorizadas:**
  - Bypass de token Bearer únicamente para las rutas `/api/v1/verifications/didit/webhook` y `/api/v1/webhooks/didit`, protegiéndolas exclusivamente vía HMAC.

## 3. Procesamiento y Verificación de los 3 Escenarios Oficiales

Pruebas unitarias y de integración concurrentes (`TestDiditWebhookScenarios` en `internal/verification/transport/http/handler_didit_test.go`):

1. **Escenario A: Flujo Aprobado con firma válida HMAC (Happy Path)**
   - Se crea intención para persona natural chilena con RUN/RUT válido normalizado.
   - Webhook firmado envía decisión con estado `Approved`, `liveness_passed: true`, `face_match_passed: true` y coincidencia exacta en RUT y fecha de nacimiento.
   - Resultado verificado: el estado de la intención en el BFF transiciona a `APPROVED` y la insignia resultante es `VERIFIED`.

2. **Escenario B: Rechazo o Discrepancia (Declined / Mismatch Fail-Closed)**
   - Webhook firmado envía decisión con advertencia de discrepancia (`IDENTIFICATION_NUMBER_MISMATCH_WITH_PROVIDED`) o estado `Declined`.
   - Resultado verificado: la política de cierre seguro (*fail-closed*) fuerza el estado a `IN_REVIEW`, impidiendo cualquier otorgamiento erróneo de la insignia de verificación.

3. **Escenario C: Firma Inválida / Spoofing / Tampering / Replay**
   - Intento con firma HMAC corrupta: Rechazado inmediatamente con `401 Unauthorized`.
   - Intento sin cabecera de firma: Rechazado inmediatamente con `401 Unauthorized`.
   - Intento de replay con timestamp desfasado (> 300 s): Rechazado inmediatamente con `401 Unauthorized`.

## 4. Resultados de la Suite de Pruebas Go

```
=== RUN   TestEvaluateDecisionFailClosed
--- PASS: TestEvaluateDecisionFailClosed (0.00s)
=== RUN   TestVerifyWebhookSignature
--- PASS: TestVerifyWebhookSignature (0.00s)
=== RUN   TestNormalizeRut
--- PASS: TestNormalizeRut (0.00s)
=== RUN   TestSyntheticCaseReviewRetryAndSafeSerialization
--- PASS: TestSyntheticCaseReviewRetryAndSafeSerialization (0.00s)
=== RUN   TestReviewRejectsUnknownReasonAndApprovalReason
--- PASS: TestReviewRejectsUnknownReasonAndApprovalReason (0.00s)
PASS
ok  	github.com/HernanEspinozaDev/espaciGo/internal/verification	1.018s
=== RUN   TestDiditIntentHTTPHandlers
--- PASS: TestDiditIntentHTTPHandlers (0.00s)
=== RUN   TestDiditWebhookScenarios
=== RUN   TestDiditWebhookScenarios/Escenario_A:_Webhook_Aprobado_con_firma_valida
=== RUN   TestDiditWebhookScenarios/Escenario_B:_Webhook_con_discrepancia_en_RUT_o_warnings_fail-closed
=== RUN   TestDiditWebhookScenarios/Escenario_C:_Firma_invalida_rechazada_401_Unauthorized
--- PASS: TestDiditWebhookScenarios (0.00s)
    --- PASS: TestDiditWebhookScenarios/Escenario_A:_Webhook_Aprobado_con_firma_valida (0.00s)
    --- PASS: TestDiditWebhookScenarios/Escenario_B:_Webhook_con_discrepancia_en_RUT_o_warnings_fail-closed (0.00s)
    --- PASS: TestDiditWebhookScenarios/Escenario_C:_Firma_invalida_rechazada_401_Unauthorized (0.00s)
=== RUN   TestEvidenceAPIIsPrivateOwnerScopedAndExplicitlyCleanable
--- PASS: TestEvidenceAPIIsPrivateOwnerScopedAndExplicitlyCleanable (0.02s)
=== RUN   TestKYCAPIAuthOwnershipAndReview
--- PASS: TestKYCAPIAuthOwnershipAndReview (0.00s)
=== RUN   TestKYCAPIRejectsUnknownFieldsAndQuery
--- PASS: TestKYCAPIRejectsUnknownFieldsAndQuery (0.00s)
=== RUN   TestVerificationHistoryEligibilityFailedQueueAndAdminRevocation
--- PASS: TestVerificationHistoryEligibilityFailedQueueAndAdminRevocation (0.00s)
PASS
ok  	github.com/HernanEspinozaDev/espaciGo/internal/verification/transport/http	1.042s
```

- `git diff --check`: 0 errores de formato.
