# GCP-03: revocación de membresía retirada — ejecución y postcheck

## Aprobación, apply y ejecución

PR #364 fue aprobado y fusionado por `HernanEspinozaDev` con merge commit `f2c7f1200c82ff47a4621a3e76d13dd1e5ebed94`. La revisión confirmó el plan binario exacto, su JSON, el manifiesto del OCI, los inventarios V1–V61 y los cuatro SQL de bootstrap.

Se aplicó únicamente el plan aprobado, SHA-256 `dce4ae0215341af73329cd0fb97b6af1f1f4c04c061c4fe9e4d6944a1b32bc8b` (JSON `54511667830793e03dc026a385de9ef7d2e84321739cdc7e6664a0953ba9a15f`). Terraform informó **2 recursos añadidos, 0 cambiados, 0 eliminados**: el Cloud Run Job y su único invoker. El state mantuvo lineage `11eb96d4-a301-9029-11d1-69e1a00f6302` y pasó del serial 16 al 17.

El Job se ejecutó una sola vez, sin overrides: `espacigo-gcp03-pilot01-revoke-retired-membership-7gtrj`. Cloud Run reportó `Completed=True`, una tarea exitosa. La ejecución usó el digest revisado `sha256:9bd186ce3eb0d83e58f825d7af29579d08b68f02f56c0f0f0227adcffa75918f`, la cuenta de servicio ACL existente, Secret Manager versión 1, destino privado `espacigo-testprod:southamerica-west1:espacigo-gcp03-pilot01-pg/espacigo_scratch`, una tarea y cero reintentos.

## Informe íntegro y resultados

Se recuperaron las entradas de Cloud Logging filtradas por el ID exacto de ejecución. El manifiesto NDJSON pasó integridad: 20 fragmentos secuenciales, 237739 bytes, SHA-256 `5d4d9023c046fb3d379b75effc05572465eff8b91e598e66c5611681d6c94d12`, `passed=true`. `dbbootstrap.ReadIntegrityNDJSON` reconstruyó el tipo completo `RetiredMembershipRevocationReport` y confirmó precheck, membresía esperada, intento de mutación, sentencia REVOKE exitosa, commit, postcheck y diagnóstico; `error_code` quedó vacío.

El postcheck reporta:

- Migraciones V1–V61 verificadas; membresías del migrador: cero.
- El migrador continúa `NOLOGIN`, `NOSUPERUSER`, `NOCREATEDB`, `NOCREATEROLE`, `NOREPLICATION`, `NOBYPASSRLS`, `NOINHERIT`.
- `TEMP=false` y `CREATE` en `public=false` para el migrador.
- Las 16 comprobaciones runtime pasaron.
- Owners sin cambio entre el informe anterior y el postcheck: base `cloudsqlsuperuser`, schema `public` `pg_database_owner`, tabla `schema_migrations` `espacigo_scratch_migrator`.

Como lectura de metadatos previa al cierre, las versiones migrador 4 y owner 1 figuraban `ENABLED`; no se leyeron payloads. Tras la autorización independiente, se destruyó únicamente migrador v4 y el CLI revisado rotó la contraseña de `postgres` con 32 bytes aleatorios en memoria, esperó la operación Cloud SQL Admin `UPDATE_USER` `534ed46d-ac69-4aba-b72f-170a0000004b` (`DONE`, sin error) y destruyó únicamente owner v1. Las lecturas posteriores confirman ambas versiones `DESTROYED`. No se reactivó el login, no se concedió `ADMIN OPTION`, no se renovó IAM y no se volvió a llamar a Cloud SQL Users API para intentar otra revocación.

## Evidencia privada y límites pendientes

Las respuestas del Job, logs filtrados, NDJSON completo, informe JSON reconstruido, plan aplicado y state posterior se conservan bajo `.local/testing/gcp03-revoke-retired-plan-20261016/`, con directorios `0700` y archivos `0600`. No se guardó material secreto en el repositorio.

No se retiraron Jobs o service accounts. El stage 1 con gate y plan exacto se prepara en este PR y queda pendiente de revisión; no se aplicó. El plan stage 2 se refrescará después de un stage 1 revisado y aplicado. El scratch, la protección de Cloud SQL, los recursos metadata de secretos, las service accounts y los datos permanecen conservados.

La versión migrador 4 y owner 1 tienen metadata pre/post archivada en `migrator-secret-v4-{pre,post}.json` y `owner-secret-v1-{pre-rotation,post-rotation}.json`. Ambas terminaron `DESTROYED`. Cloud SQL Admin `operations list` confirmó la operación `UPDATE_USER` `534ed46d-ac69-4aba-b72f-170a0000004b` en `DONE` sin error. El log privado del CLI registra únicamente el mensaje de éxito; no contiene contraseña.

## Cleanup Terraform por etapas

El plan de stage 1 incluido en este PR solo cambia `deletion_protection=true` a `false` en los nueve Jobs existentes (cinco fases migrator, observer, grant-owner-history, verify-retired y revoke-retired-membership): **0 altas, 9 cambios in-place, 0 eliminaciones**. El plan JSON confirma que cada recurso cambia únicamente ese atributo. Los 28 recursos administrados restantes, incluidos SQL, database, metadata de ambos secretos, service accounts, accessors, `roles/cloudsql.client`, invokers, actAs y los tres bindings temporales con expiración `2026-10-16T18:00:00Z`, quedan no-op. State previo: lineage `11eb96d4-a301-9029-11d1-69e1a00f6302`, serial 17. SHA-256 plan binario `d4d2a01c9ae980606d71987776a9d543ec68989b786b457490889b40781bfd8e`; JSON `54763df0215af06f463ed858f620dad1f7d2d25aea3b1efc359fcc2f7b84bc2f`. **Stage 1 no se aplicó.**

Los tests mock simulan stage 1 con los nueve Jobs presentes y stage 2 con su retiro junto con los nueve invokers, dos actAs y los cinco recursos IAM temporales de replay (dos custom roles y tres bindings). Ese escenario conserva SQL, database, metadata de secretos, SAs, accessors y grants Cloud SQL client. El plan Terraform stage 2 binario no se genera ni aplica en este corte: después de aplicar stage 1 bajo revisión se hará un refresh y un plan nuevo; sus eliminaciones se revisarán y autorizarán por separado.
