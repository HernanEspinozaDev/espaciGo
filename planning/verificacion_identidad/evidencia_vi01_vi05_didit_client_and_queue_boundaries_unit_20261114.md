# Evidencia — VI-01 / VI-05, fronteras de HTTPDiditClient, cola de borrado en memoria y descifrado de decisiones en verification

- **Fecha:** 2026-11-14.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / commit `c9873fc4` (worktree local).
- **Identidad de Git verificada:** `HernanEspinozaDev` (autor y committer alineados con política del repositorio).
- **Modalidad:** Test-only local con servidor HTTP de pruebas (`httptest.Server`) y fakes en memoria (`memorySessionDeletionQueue`, `Keyring`). Cero persistencia PostgreSQL, cero SQL/DDL, sin llamadas a servicios Cloud, sin mutación de bases persistentes, sin commit, push, deploy ni integración a `main`.

## 1. Alcance cubierto

Se implementó la suite [didit_client_and_queue_boundaries_test.go](file:///home/nandev/Documentos/projects/espaciGo-main-sync/internal/verification/didit_client_and_queue_boundaries_test.go) en `internal/verification`:

1. **Fronteras y métodos HTTP de HTTPDiditClient (`TestHTTPDiditClientBoundaries`):**
   - `GetSessionDecision`: solicitud exitosa `GET /v3/session/{id}/decision/`; decodificación de JSON en `DiditDecisionResponse`. El test no exige rechazo de campos desconocidos ni valida exhaustivamente el payload.
   - Error ante la respuesta HTTP 500. No cubre 4xx ni todos los códigos no-2xx.
   - `DeleteSession`: validación fail-closed de identificador de sesión vacío o compuesto solo de espacios en blanco (`ErrInvalid`); y despacho de método `DELETE` a la ruta `/v3/session/{trimmed}/delete/` con recorte de espacios (100,0%).

2. **Cola de borrado de sesiones en memoria y concurrencia (`TestMemorySessionDeletionQueueBoundaries`):**
   - `NewMemorySessionDeletionQueue`: inicialización segura (100,0%).
   - `EnqueueSessionDeletion`: encolado de sesiones con deduplicación idempotente para evitar colisiones (100,0%).
   - `PendingSessionDeletions`: copia defensiva desacoplada del slice interno (100,0%).
   - `AcknowledgeSessionDeletion`: remoción atómica de la sesión confirmada y comportamiento fail-safe ante sesiones no existentes (100,0%).
   - Concurrencia protegida mediante `sync.Mutex`, ahora verificada bajo `-race` con fase concurrente de encolado y aserción exacta del estado final, seguida de fase concurrente de ACK y aserción exacta. Este alcance caracteriza solo la cola en memoria; no demuestra borrado remoto durable.

3. **Descifrado de instantánea de decisión (`TestDiditServiceDecryptAttemptDecisionBoundaries`):**
   - Protección contra receptor nil -> `ErrNotFound`.
   - Protección contra carga cifrada vacía o algoritmo de decisión no admitido -> `ErrNotFound`.
   - Validación canónica de AAD (`BuildDecisionAAD`) con UUIDs válidos; rechazo de identificadores corruptos con `ErrInvalid`.
   - Cifrado autenticado AEAD (AES-GCM) vía `Keyring.Seal`, apertura segura con `keys.Open` y deserialización válida de `EncryptedDecisionSnapshot` (91,7%).
   - Detección de manipulación de texto cifrado (bit flipping) con fallo estricto de autenticación.

4. **Revocación preservando registro histórico (`TestMemoryRutClaimRegistryRevokeVerifiedIsSafeNoop`):**
   - Comprobación de invariante de no regresión: `RevokeVerified` no borra ni degrada reservas históricas permanentes de RUT.

## 2. Cobertura de statements en internal/verification

Cobertura en funciones ejercitadas:
- `GetSessionDecision`: **100,0%**
- `DeleteSession`: **100,0%**
- `NewMemorySessionDeletionQueue`: **100,0%**
- `EnqueueSessionDeletion`: **100,0%**
- `PendingSessionDeletions`: **100,0%**
- `AcknowledgeSessionDeletion`: **100,0%**
- `DecryptAttemptDecision`: **91,7%**

Cobertura total de statements del paquete `internal/verification`:
- Incrementó al **80,9%** (desde 77,9%).

## 3. Comandos de verificación ejecutados

```bash
# Tests focales con race detector
go test -race -v -run 'TestHTTPDiditClientBoundaries|TestMemorySessionDeletionQueueBoundaries|TestDiditServiceDecryptAttemptDecisionBoundaries|TestMemoryRutClaimRegistryRevokeVerifiedIsSafeNoop' ./internal/verification

# Suite completa de paquetes afectados con race detector
go test -race -count=1 ./internal/verification/... ./internal/privacy/... ./internal/identityverification/... ./internal/documentverification/...

# Verificación de sintaxis y formato
git diff --check
```

Resultados:
- Todos los tests pasaron exitosamente en verde (PASS).
- `git diff --check` limpio.

## 4. Estado y compuertas pendientes

El subcorte VI-01 / VI-05 permanece en ámbito local con fakes en memoria a la espera de compuertas de persistencia y migración. Sin alteraciones de esquemas de bases de datos persistentes o Cloud SQL, sin SQL/DDL, sin credenciales reales ni integración a `main`.

## 5. Addendum de precisión y regresión determinista — 2026-11-14

En el checkout observado `/home/nandev/Documentos/projects/espaciGo-main-sync`, rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD `274f37e483a078890e92382a159fa49fd23c2fe8` con árbol de trabajo compartido y sucio, se reforzó `TestMemorySessionDeletionQueueBoundaries`: 20 encolados concurrentes de la misma sesión deben terminar con exactamente una copia; luego 20 ACK concurrentes deben removerla sin alterar la sesión previa restante. El bloque no afirma durabilidad remota.

Se ajustó la descripción de `TestHTTPDiditClientBoundaries`: el test muestra decodificación del JSON de éxito en la estructura, una respuesta 500 errónea y el método/ruta de DELETE; no prueba strict decoding, errores 4xx ni validación exhaustiva del payload.

Comando ejecutado en esta sesión: `go test -race -count=1 ./internal/verification -run 'TestMemoryEvidenceRepoRejectsOtherOwners|TestEvidenceService(Content|Delete|UploadCompensationDeleteFailure).*|TestMemorySessionDeletionQueueBoundaries|TestHTTPDiditClientBoundaries'` — PASS. No se re-ejecutó aquí la suite completa documentada en la sección 3. Sin PostgreSQL, SQL/DDL, Cloud, proveedores externos, datos reales, commit, push, deploy o integración a `main`.