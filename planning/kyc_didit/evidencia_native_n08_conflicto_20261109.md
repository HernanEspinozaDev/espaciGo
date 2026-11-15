# N08-G — conflicto RUT al guardar, rollback y journals A/B (2026-11-09)

## Alcance y baseline

Checkout `espaciGo-main-sync`, rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD base `274f37e483a078890e92382a159fa49fd23c2fe8` más árbol dirty preservado. Sin commit/push, main, despliegue, migraciones ni proveedor externo. Único destino `espacigo_dms_stage_20261020`, loopback5432, OID16388, motor180006/ledger90. Dos cuentas sintéticas existentes autorizadas, sin alta/reinicio de cuentas.

La revisión `revision_native_n08_conflicto_20261109.md` conserva entregas parciales, regresiones y adjudicación de hallazgos. Esta evidencia posterior acredita el ensayo corregido; no convierte aquellas revisiones source-only en pruebas nativas.

## Cambio ejercitado

- Runtime exige HMAC dedicado; `NewDeclarationServiceWithRutHMAC` deriva fingerprint versionado desde RUT normalizado y lo pasa al input de revisión. El constructor legado conserva su contrato para consumidores anteriores; no es la configuración de producción/runtime.
- `Repository.CreateRevision` ejecuta revisión, claim y auditoría dentro de una misma transacción. Conflicto de otro dueño revierte la revisión provisional; un UPDATE de claim con cero filas devuelve conflicto. No llama al Claim de transacción independiente ni barre claims expirados ajenos.
- Pending propio con mismo fingerprint puede actualizar la referencia de revisión. Claim propio ya verificado se conserva; otro dueño o fingerprint distinto sujeto a unicidad por cuenta se rechazan conservadoramente. Liberación/TTL y reverificación son transiciones diferentes, no aceptadas por este gate.
- Pair test-only mantiene un journal exclusivo por cuenta, escrito antes de delegar incluso si la transacción devuelve conflicto. No habilita segundas escrituras de A ni cleanup por prefijo.
- Probe exige payload realmente enviado por B con el mismo RUT normalizado seleccionado para A,409, copy neutral exacto, ausencia de resumen/acción Didit y GET404 sin datos. Contador real autenticado del fake no puede crecer por el conflicto.
- Harness verifica READ ONLY, antes de limpiar: B sin declaración/intento/evento/claim/auditoría; A con declaración propia exacta, auditoría correlacionada y claim ligado a esa declaración. Conserva verificación de un intentoIN_REVIEW/un eventoPROCESSED con hash32. Después cleanup inverso de journals B/A y comparación global del baseline outbox antes de retirar journals.

## Gates técnicos

Suite parental cero caché/race PASS en cmd/api, internal/verification/... (crypto/HTTP), adaptador identitydeclaration y fake; vet y diff-check PASS. Preflight nativo READ ONLY PASS. Node syntax/Prettier/ESLint del probe PASS; contrato adaptador7/7PASS. Legacy native adapter lifecycle siguen sin opt-in; no se ejecutaron para obtener cobertura.

Repetición final `go test -json -race -count=1` parseada por Python:506eventos de test PASS,10SKIP,0FAIL (incluye subtests, no506casos top-level independientes). Ambos lifecycle antiguos `TestRutClaimPostgresClaimLifecycle` y `TestSessionDeletionQueuePostgresLifecycle` confirmados SKIP. Vet, formato del probe y diff-check posteriores PASS. Registro local sin secretos: `kyc-n08g-final-tests.jsonl` en scratch.

Pruebas del helper transaccional usan fake pgx.Tx y acreditan secuencia/errores: no sustituyen durabilidad o locks PostgreSQL. El test de éxito confirma el fake explícitamente fuera del helper; el wrapper real es quien hace Commit/deferRollback. La evidencia de rollback PostgreSQL de B procede del ensayo nativo y su query anterior al cleanup, no de llamar Rollback manualmente al fake.

Correcciones conductuales previas: constructor legado Save falló en regresión parental y quedó corregido; consentimiento recuperó su contrato original tras RED conductual, separando validación estricta de versión HMAC. El test de UPDATE cero filas falló con retorno nil y pasó tras corrección.

## Frontend y backup

Build aislado fresco `espacigo-kyc-n08g-yy47yl6i`, sin dotenv ni secretos API. Comparación byte a byte de fuentes relevantes contra snapshot PASS antes del ensayo; Next4201, proxyHTTPS4219 y fake19091 con entornos separados. API18090 exclusivamente harness worker-free. Navegador Playwright aislado, sin perfil/pestañas del usuario.

Backups privados, modo0600, TOC y consumo completo pg_restore PASS, sin restore:

- `pre-n08-conflict-20261109.dump`: 8,323,117bytes, SHA256 `4791d3ca67ab3ceda0f0f42a1084846571934d8ec43e4cdde87bc0dd374f6058`.
- `pre-n08-conflict-retry-20261109.dump`: SHA256 `758a96c1691c0a9c5f0d870beaee20ad2cd922719e26ab5331fb43b84a4926d6`.

Prepare/finalize versionados por sudo directo parental. Readback independiente posterior a prepare confirmó auditDELETE/outboxDELETE=true y ownerCREATE=false; posteriormente todo revocado.

## Primer ensayo fallido y recuperación

Primer intento falló antes del POST de B, en la aserción inmediata de formulario vacío tras loginB (`cross-account-ui-state`). El harness señaló journal B ausente porque B todavía no había intentado guardar. No es prueba de conflicto ni evidencia de fuga: ese reporte booleano no discriminaba campos ausentes, valores o marcadores. Cleanup de A completó; ambos journals ausentes, grants revocados, outbox50/pendientes0 y preflight posterior PASS.

Se corrigió la coordinación del probe: esperar el destino real `/mi-cuenta` después del login antes de navegar a verificaciones, en los tres logins; añadir diagnósticos solo booleanos de campos presentes/vacíos, consentimientos y ausencia de marcadores. No se relajó ninguna condición ni cambió producto. La carrera de navegación es una explicación compatible con el fallo; no se capturó el DOM del primer fallo para afirmar su causa exacta. No hubo replay de un guardado incierto: se inició un ensayo nuevo tras cleanup/readback demostrado y backup fresco.

## Ensayo nativo corregido: PASS

`run_kyc_n07_bff.py conflict`, flags FLOW/WEBHOOK/IDEMPOTENCY/UNICODE/CONFLICT y pair journals:

| Gate | Resultado observado |
| --- | --- |
| A guardar/leer/recargar | 201/200/200, Unicode/DV-K preservado y normalizado |
| Guardar y consentimiento no dado | Cero creates del fake |
| Iniciar + replay misma key + otra key activa | 201/mismo intento/misma URL; delta creates1 constante |
| Webhook inválido/stale/ajeno; válido/replay | 401/401/404;200/200, estado autoritativoIN_REVIEW |
| B login/formulario | 200; campos presentes/vacíos, consentimientos sin marcar y sin marcadores A |
| B enviar mismo RUT normalizado | rutConflictInputMatches=true; POST409 |
| Error visible B | Mensaje neutral exacto; sin resumen guardado ni acción Didit |
| Readback B | GET404 sin identidad; contador proveedor sin incremento |
| Retorno ajeno/retry | 404/404, query limpia y sin datos A |
| Reingreso A | GET200, datos exactos/render preservados |
| Logout final | 204, sesión401 y cookies ausentes |
| Exterior/storage | Cero peticiones browser externas inesperadas; sin campos seleccionados en storage/URL |

**probe_exit=0, harness_exit=0**, `TestNativeKYCAPIWithBFF` PASS12.39s, paquete13.472s con race. Verificaciones SQL previas al cleanup PASS; no se atribuye al navegador lectura de ciphertext/SQL.

## Readback y retirada independientes

Finalizador COMMIT/exit0. Readback administrativo READ ONLY posterior: target/OID/motor/ledger correctos, auditDELETE=false, outboxDELETE=false, ownerCREATE=false, auditSELECT=false; outbox Didit50/pendientes0, sin crecimiento. Preflight posterior PASS para ambas cuentas/artefactos. JournalsA/B ausentes; marker ready ausente y stop propio vacío0600 retirado tras validar UID/metadata. No borrar ACK históricos por prefijo.

API18090 cerrada por harness. Se verificaron UID/cwd/exe de los tres procesos propios (Next/proxy/fake) y se retiraron con SIGTERM. `ss` confirmó ausencia de listeners4201/4219/18090/19091. PostgreSQL y recursos ajenos conservados. Backups/fixtures autorizados privados conservados para trazabilidad y próximos gates.

## Snapshots efectivamente probados

| Fuente | SHA256 |
| --- | --- |
| frontend/tooling/kyc-native-flow.mjs | db0b95a9131f7fae7752014b9a59c214dc8c6df86372989e00660427c4860df0 |
| internal/adapters/postgres/identitydeclaration/repository.go | dad09e03c98b25ee2077943899bb925b45ded2a1db71bc335735cfb1552ae0f4 |
| internal/verification/declaration_service.go | 34dd200caa31176f3e377619c0e0bcd85e7b38ad2846ec1d2672adce773404c6 |
| cmd/api/identity_native_http_harness_test.go | ed501b01141dfb7000bcb6ca38e0ec28caab5092797cdc289cbf3f0eaeea1932 |

**N08-G PASS para conflicto visible B al guardar, atomicidad/rollback de ese caso y cleanup descritos.** No demuestra carrera de dos saves simultáneos, TTL/liberación de claims, nueva revisión tras aprobación ni todos los estados/errores C07–C10. N08 global permanece parcial, sin sandbox externo, aprobación biométrica ni aceptación humana.
