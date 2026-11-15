# Cierre N08 — aceptación técnica local sintética (2026-11-09)

## Veredicto

**N08 completada en el alcance local sintético autorizado.** Los criterios C01–C11 están vinculados a pruebas por capa en `matriz_aceptacion_native_20261109.md`; no se presentan mocks, revisión source-only ni rollback-only como ejecución persistente del producto.

- Checkout: `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- Rama: `codex/marketplace/fix/kyc-06-save-identity-first`.
- HEAD base: `274f37e483a078890e92382a159fa49fd23c2fe8`; cambios dirty/untracked conservados. El SHA no representa esos cambios. Sin commit, push, integración o despliegue.
- Único destino de integración: `127.0.0.1:5432/espacigo_dms_stage_20261020`, OID16388, PostgreSQL180006; ledger final91, nombres/checksums comparados con fuente por preflight. No Docker ni base sustituta.

## Qué completa el cierre

| Gate | Prueba real y resultado | Evidencia |
|---|---|---|
| A–F | Validación chilena, auth/retorno, accesibilidad desktop/móvil, firma/correlación/replay, idempotencia medida en fake y Unicode/DV-K | Recibos N08 previos y matriz, conservados |
| G–J | Colisión RUT409 al guardar con claim/revisión/auditoría atómicos; error502/FAILED recuperable; TTL exactoEXPIRED; revocación de pendiente/replay/no resurrección | Evidencias conflicto, errores_retorno, expiracion y revocacion |
| K | Guardar/iniciar/aprobar sintéticamente; purga404; APPROVED SANDBOX sin badge LIVE; segundo guardado/GET confirmado invalida aprobación efectiva; Approved tardío no la restaura | `evidencia_native_n08_aprobacion_20261109.md` |
| L | Aprobación previa purgada → REVOKED → REJECTED; replay deduplicado, Approved tardío IGNORED; callback y SQL durables | Mismo recibo K/L |
| M | Tras EXPIRED, consentimiento/click explícitos crean intento UUID distinto; originalEXPIRED/nuevoPENDING_USER_INPUT, dos creates exactos y una declaración | `evidencia_native_n08_reintento_ttl_20261109.md` |
| N | Función real bajo rol worker restringido100+1 de101 revisiones propias, en BEGIN/ROLLBACK; guard/locks, readback independiente0 | `evidencia_native_n08_purga_acotada_20261109.md` |

K/L/M pasaron por navegador Playwright aislado → HTTPS4219 → Next BFF → Go18090 → PostgreSQL nativo, con autenticación normal de fixtures autorizados. Counter realfake, readbacks HTTP y SQL antes de cleanup distinguen persistencia de apariencia. Solo el proveedor y las identidades son sintéticos. Frontend standalone existente comparado byte a byte con300 archivos fuente; sin cambios funcionales de UI posteriores ni dotenv.

### Correcciones necesarias verificadas

1. **V91 append-only aplicada** mediante CLI exacto revisado, sin CREATE ni grants adicionales: admite retirada REJECTED/EXPIRED después de que V90 haya purgado snapshot y separado el intento. Mantiene estados pendientes separados prohibidos y metadata cifrada NULL. Checksum `6ceee080736bd965a27226e3810d28ede0ecf2deb52477716a504e2b425c1250`; V1–V90 inmutables.
2. **Reloj de ACK corregido por RED/GREEN:** el default capturaba `time.Now().UTC` en construcción; ahora evalúa ahora al invocarse, conservando reloj inyectado. La prueba default falla con el código anterior y pasa con el nuevo. Ensayos nativos posteriores acreditan ACK y retiro≥30d.
3. **Journals/cleanup post-purga:** receipt previo de intento exacto más declaraciones, correlaciones independientes, validación de scope antes de DELETE y retiro de journals solo después de readback/baseline. Reserva HMAC mínima histórica no se confunde con aprobación efectiva.

## Retención y privacidad: lo demostrado

`matriz_retencion_native_n08_20261109.md` conserva la auditoría previa y la actualización K–N. Antes de aprobar, observación READ ONLY acredita declaración=consent+30d e intento/evento/claim pendiente con la misma fecha. Después se observa purga local, intento sin ciphertext, claim mínimo verificado sin expiry y ACK con retirar_en≥ACK+30d. N06 conserva la prueba de rollback/recuperación desde pool nuevo/retry durable. N acredita cap SQL100 bajo rol worker, no scheduler.

Los marcadores de identidad no quedaron en los almacenamientos y URLs inspeccionados; A/B/A y logout/cookies tienen sus recibos propios. No se registraron cuerpos privados, credenciales, DOM dumps ni capturas nuevas de identidad para este cierre.

## Regresión final ejecutada

- Go `-race -count=1`: **874 eventos de tests/subtests PASS, 51 SKIP, 0 FAIL** en cmd/api, cmd/dbmigrate, cmd/privacy-cleanup-worker, internal/migrator, verification, adaptador de declaraciones y fake. Ambos lifecycle legacy nativos quedaron SKIP: no se habilitaron para obtener cobertura ni tocar su outbox.
- `go vet` de esos paquetes: exit0.
- Frontend: **485 tests, 484 PASS, 1 SKIP, 0 FAIL**; TypeScript y formato exit0; ESLint exit0 con4warnings en archivos ajenos a esta corrección, no ocultados.
- Checker de evidencia de migración:4 testsPASS; sintaxis del shell de harnessPASS. Cobertura histórica V90 preservada y transiciónV90→V91 añadida, pero harness PostgreSQL efímero **no ejecutado** por política native-only.
- `git diff --check`: exit0.
- Logs de regresión locales: scratch `kyc-n08-final-tests.jsonl` y `kyc-n08-final-frontend-tests.log`. No se atribuye CI remoto.

## Postflight y recursos

Readback independiente final: target/OID/motor exactos, ledger91; dos cuentas sintéticas autorizadas activas sin declaraciones/intentos/claims/eventos propios; outbox histórica **50 ACK/0pendientes**, sin borrado por prefijo; audit/outbox DELETE temporalesfalse, columnas audit SELECT temporalesfalse, ownerCREATEfalse. Preflight de cuentas/ledgerPASS.

Journals de ejecución y controles ready/stop ausentes. Los tres procesos propios (Next, proxy y fake) fueron identificados por UID/cwd/executable antes de SIGTERM; puertos18090/4201/4219/19091 sin listeners. El fake y sus sesiones sintéticas dejaron de existir al retirar su proceso.

Se retienen dos cuentas autorizadas, sus credenciales/política privadas y backups/recibos de rollback0600 únicamente para trazabilidad y continuación autorizada del corte; responsable operador local del corte. No retirar cuentas, secretos o backups por inferencia. Backup preV91 y backups antes de gates destructivos constan con SHA/listado/parse en los recibos; no equivalen a recuperación/restauración ensayada ni respaldo de roles globales.

## Fuera de este cierre

No Didit externo, biometría/personas/documentos reales, aprobación jurídica/privacidad, cumplimiento legal, purga general periódica de metadatos, autenticación LOGIN/scheduler de worker ni SLA operacional. Los plazos descritos son candidatos implementados/fechas observadas, no tiempo real transcurrido. No aceptación humana de producto, publicación, integración a main, Cloud SQL/GCP o despliegue. **N08 no autoriza iniciar KYC-14 ni otra fase.**

Plan vigente actualizado en §25.34 y tablaN08; los estados históricos parciales se conservan como antecedentes, no como estado actual. Revisión independiente final `deleg_31c3f5b9` source-only: approvedlocalclosure=true, sin criterio canónico local pendiente. La revisión no sustituye las ejecuciones parentales ni constituye aceptación humana.
