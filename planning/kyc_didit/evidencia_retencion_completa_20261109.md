# R1 — Retención completa posterior a N08 (2026-11-09)

## Alcance y estado

En curso, no cerrada. Continuación humana: «resuelve en el orden recomendado y prosigue». Único destino: PostgreSQL nativo `espacigo_dms_stage_20261020`, loopback5432/socket `/var/run/postgresql`, OID16388, motor180006. Rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD base `274f37e483a078890e92382a159fa49fd23c2fe8` más cambios dirty/untracked compartidos preservados. Sin commit, push, main, Cloud SQL o proveedor externo.

## Preflight y backup

- `sudo id` directo mediante terminal Hermes: exit0, autenticación nativa disponible. No se leyó ni imprimió `SUDO_PASSWORD`.
- `IDENTITY_DECLARATION_NATIVE_PREFLIGHT_ONLY=1 GOTOOLCHAIN=local go test -count=1 ./cmd/api -run '^TestNativeIdentityExternalReadOnlyPreflight$' -v`: PASS, ledger V1–V91 exacto/source-backed bajo login migrador/SET ROLE owner y transacción READ ONLY.
- Política privada, dos receipts y directorios propios validados por metadatos 0700/0600; ambas cuentas verified y presentes en inventario autorizado, sin emitir valores privados. No cuentas nuevas.
- Backup privado `pre-expired-lifecycle-v92-20261109.dump`, ext4,0600,8.324.803 bytes; SHA256 `264fabf2194d3eb60c292eefd7a6668b8e39a3c6515cb5c751f4c346ddc90b3d`. `pg_restore --list` y parse completo `--exit-on-error --file=/dev/null` exit0. No restauración ensayada ni backup de roles globales.

## RED nativo conductual antes del delta

`db/repair/test_native_kyc_expired_lifecycle.sql` crea únicamente fixtures SQL sintéticos en BEGIN/ROLLBACK:101 revisiones vencidas, una futura, cuatro intentos vencidos en PENDING_USER_INPUT/FAILED/EXPIRED/REJECTED y un intento futuro con eventos, claim pendiente y claim verificado en la segunda cuenta. Los IDs exactos se persistieron antes en journal privado; no son datos personales ni evidencia de cifrado de aplicación. Locks de ensayo y guard de ninguna revisión ajena elegible en horizonte conservador impiden que la función general reclame filas ajenas. Invocación bajo rol NOLOGIN worker; no login/scheduler.

- Primera ejecución reprodujo la aserción, pero `psql \\quit 1` ignoró el argumento y salió0. Se considera defecto del ejecutor de prueba, no PASS. Se sustituyó por RAISE EXCEPTION bajo ON_ERROR_STOP; se conservó el log privado original.
- Ejecutor corregido: **exit3 RED esperado**, mensaje exacto `expired lifecycle first batch must purge 100 including referenced revisions`. La función V86 deja revisiones con referencias y no satisface la obligación completa; no se alteró la migración aplicada para fabricar RED.
- Readbacks READ ONLY independientes antes y después: ledger91; revisiones/intentos/eventos/claims de los journals=0; outbox total65/pendientes0 sin cambio entre ambos; owner CREATEfalse. El baseline histórico50 no se presenta como actual: hoy se observaron65, sin atribuir origen ni borrar/reprocesar filas históricas.
- Ningún fixture se confirmó, ninguna migración nueva se aplicó. No grants abiertos.

## Siguientes gates

Candidato V92 con contrato fuente/test y revisión independiente; ampliar runner exacto y cobertura de inventario sin modificar V1–V91. Ensayo funcional de candidato en transacción reversible bajo identidad requerida, negativas/rollback/concurrencia, backup fresco, aplicación de una sola versión y postcheck. Después R2 HMAC/retiro de metadatos y R3 operación acotada. La regresión y cierre local previos no sustituyen estos gates ni habilitan Didit externo o entrega.

## Candidato corregido y GREEN nativo reversible

La primera entrega source-only no pasó su contrato; no se aplicó. Padre corrigió dedup para usar `didit_session_deletion:<session>` compatible con la cola real (no una clave por intento), límite completo de120 caracteres, ownership/ACL predecessor y orden de locks. Revisión independiente detectó retención del primer advisory lock cuando fallaba el segundo. Se reemplazó CONTINUE por `RAISE lock_not_available` dentro de la subtransacción de grupo para liberar sus locks/mutaciones; no se ocultó el hallazgo.

- Candidato V92 actual SHA256 `bf7db344af0752ac479b4422dfa0e770cd11623c3be2b51c4449dba89c93cc03`; fingerprint V1–V92 `a49c75125218df000104597604c86c95df94efd9aaf3f584392488307cc23f4e`. V1–V91 sin cambios. Runner extendido minApplied91/maxVersion92, CREATE requerida, target revalidado bajo lock y dentro de la transacción; pines actualizados después del fix, no se usaron los anteriores.
- Fuente/runner focalizados `go test -race -count=1 ./cmd/dbmigrate ./internal/migrator -run 'TestNativeKYCTail|TestNativeV86|TestIdentityV92'` PASS; vet de ambos paquetes y diff-check PASS. No equivalen a aplicación.
- Backup fresco privado `pre-v92-rehearsal-20261109.dump`:0600/ext4,8.324.803 bytes, SHA256 `5ff96a3e8190ce435bd1311039c07cdb5c54a5a9691b1ddd4d31731af31ec20c`; listado y parse completo pg_restore exit0.
- Preparación y finalización versionadas V92 abrieron solo CREATE temporal del owner, con readback true al abrir y false tras revocar; sin grants extra de observación. La cuenta activa/política se comprobó por preflight administrativo READ ONLY; el ensayo owner usa únicamente las columnas outbox que el definer ya podía leer.
- **Ensayo normal exit0** con login real `espacigo_migrator_login`→`SET LOCAL ROLE espacigo_schema_owner`: candidato instalado dentro de BEGIN y descartado junto a fixtures por ROLLBACK. Compiló y ejecutó la función real contra el esquema nativo. Colisión de clave outbox ligada a otro intento produjo unique_violation y rollback de toda la llamada, sin borrar grupos previos. Luego100+1+0 borró revisiones/eventos/intentos/claim pendiente de fixtures y conservó cadena futura/claim verificado. No prueba cifrado de aplicación ni worker LOGIN.
- **Contención nativa exit0**: conexión separada autenticada mantuvo el advisory lock namespace1 de la cuenta de ensayo. El script exigió que no pudiera adquirirlo antes y después de invocar el purgador; retornó0, conservó102/5/5 filas y ningún advisory lock del grupo quedó retenido. No es solo una lectura de pg_locks sin holder. Tras el ensayo se retiró únicamente el proceso propio; se detectó que su backend seguía en pg_sleep, se verificó PID/rol/DB/query/lock y se terminó ese backend exacto. Readback del lock quedó en0filas.
- **Readbacks independientes tras ambos ensayos**: ledger91, artefactos journaled0, outbox65/pendientes0, ownerCREATEfalse. No se confirmó ningún fixture ni el DDL candidato. Logs y scripts con IDs permanecen privados0600.

V92 todavía no aplicada al registrar estos resultados. Pendientes: sincronización del harness histórico (sin ejecutarlo en entorno efímero), revisión final del conjunto exacto, backup/apply/postcheck por runner y ensayo bajo worker después de aplicar. R2/R3 no iniciados.

## Aplicación y cierre del mecanismo R1

El estado de candidato anterior queda superado por esta ejecución, sin borrar su secuencia:

- Revisión final independiente `deleg_4f68bb27`: approved_apply=true; checksum/fingerprint calculados independientemente, rango91→92, target bajo lock/transacción, CREATE temporal y ACL worker-only aprobados. No sustituyó el ensayo parental ni afirmó DB aplicada.
- Regresión parental con entorno explícito sin opt-ins privados: **891 eventos de tests/subtests PASS,51SKIP,0FAIL**, `-race -count=1`; vet de API/migrador/worker/verificación/adaptador/fake PASS. Los skips DB/harness no se cuentan como ejecuciones. Checker Python6PASS, bash sintaxis y diff-check PASS. Log scratch `kyc-r1-v92-tests.jsonl`.
- Preflight actualizado por RED→GREEN para aceptar exclusivamente prefijos source-backed V89–V92 en lectura flexible; escrituras de los harnesses actuales exigen V92 exacta. Se conservan checksums históricos. Preparador N07 queda fijado a92 para futuras pruebas, no se ejecutó en este tramo.
- Harness histórico sincronizado con release-v91-v92, catálogo92 y no-op final; checker requiere los resultados de las tres baselines y conservaV90→V91. No se ejecutó PostgreSQL efímero/Docker.
- Backup inmediatamente previo a apply: privado `pre-v92-apply-20261109.dump`,0600/ext4,8.324.803 bytes, SHA256 `7f6790d3cd8754e54057778ffacf0b157dbc27531506d4579658ec1daf3c2135`; listado/parse completo exit0. No restauración ensayada.
- Preflight autenticado READ ONLY pasó antes de abrir CREATE; lectura independiente acreditó CREATEtrue temporal. CLI `cmd/dbmigrate -native-kyc-tail-version 92` con target/fingerprint fijados terminó **exit0**: V1–V92 exactas, sin pendientes posteriores del inventario92. **V92 aplicada únicamente en el PostgreSQL nativo seleccionado.** Ahora es inmutable; no Cloud SQL.
- Finalizador versionado terminó0; readback independiente ledger92, ownerCREATEfalse, artefactos journaled0, outbox65/pendientes0 preservada.
- **Aceptación post-apply bajo SET LOCAL ROLE worker NOLOGIN exit0**: SQL versionado ejecutó el negativo de binding outbox y los lotes100+1+0 de fixtures, preservó cadena futura/claim verificado y acabó en ROLLBACK. Readback independiente posterior sin residuos y preflight autenticado source-backed92PASS. Este ensayo acredita función instalada y permisos worker, no LOGIN/scheduler ni borrado remoto real.

**R1 completado para el mecanismo de retención de declaraciones vencidas referenciadas y sus dependencias.** No acredita la purga general de metadatos desacoplados, retiro de ACK, rotación HMAC, worker periódico ni SLA24h: siguen en R2/R3. No se inició proveedor externo, publicación o despliegue. Se retienen backups/recibos privados para trazabilidad; no se borraron filas ajenas ni outbox histórica.
