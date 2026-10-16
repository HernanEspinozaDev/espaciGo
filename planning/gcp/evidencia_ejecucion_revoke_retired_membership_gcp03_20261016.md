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

## Estado de GCP-03

Este resultado acredita la revocación de la membresía y el contrato de roles/runtime en la base scratch vacía con V1–V61, además de la verificación PostGIS y el cierre de las credenciales temporales. **GCP-03 permanece parcial**: el restore verificable de snapshot, la verificación de calendario/solapes y las integraciones/criterios de compatibilidad restantes de la matriz siguen pendientes. Esta evidencia no sustituye esos gates ni marca GCP-03 completo.

Como lectura de metadatos previa al cierre, las versiones migrador 4 y owner 1 figuraban `ENABLED`; no se leyeron payloads. Tras la autorización independiente, se destruyó únicamente migrador v4 y el CLI revisado rotó la contraseña de `postgres` con 32 bytes aleatorios en memoria, esperó la operación Cloud SQL Admin `UPDATE_USER` `534ed46d-ac69-4aba-b72f-170a0000004b` (`DONE`, sin error) y destruyó únicamente owner v1. Las lecturas posteriores confirman ambas versiones `DESTROYED`. No se reactivó el login, no se concedió `ADMIN OPTION`, no se renovó IAM y no se volvió a llamar a Cloud SQL Users API para intentar otra revocación.

## Evidencia privada y límites pendientes

Las respuestas del Job, logs filtrados, NDJSON completo, informe JSON reconstruido, plan aplicado y state posterior se conservan bajo `.local/testing/gcp03-revoke-retired-plan-20261016/`, con directorios `0700` y archivos `0600`. No se guardó material secreto en el repositorio.

El stage 1 se aplicó después de la fusión y revisión de PR #365 (`f2a77eadba5b2338ee83ff3a03fa94c6703a81ce`). El stage 2 fue planificado de nuevo contra el state resultante y **no se aplicó**. El scratch, Cloud SQL, database, metadata de secretos, service accounts y datos permanecen conservados.

La versión migrador 4 y owner 1 tienen metadata pre/post archivada en `migrator-secret-v4-{pre,post}.json` y `owner-secret-v1-{pre-rotation,post-rotation}.json`. Ambas terminaron `DESTROYED`. Cloud SQL Admin `operations list` confirmó la operación `UPDATE_USER` `534ed46d-ac69-4aba-b72f-170a0000004b` en `DONE` sin error. El log privado del CLI registra únicamente el mensaje de éxito; no contiene contraseña.

## Cleanup Terraform por etapas

El plan stage 1 solo cambió `deletion_protection=true` a `false` en los nueve Jobs existentes: **0 altas, 9 cambios in-place, 0 eliminaciones**. El JSON confirmó que cada recurso solo cambiaba ese atributo. Los otros 28 recursos administrados quedaron no-op. State previo: lineage `11eb96d4-a301-9029-11d1-69e1a00f6302`, serial 17. SHA-256 binario `d4d2a01c9ae980606d71987776a9d543ec68989b786b457490889b40781bfd8e`; JSON `54763df0215af06f463ed858f620dad1f7d2d25aea3b1efc359fcc2f7b84bc2f`.

PR #365 fue revisado y fusionado por `HernanEspinozaDev`; HEAD revisado `613b425ee6b1aec0d477f84a476fc13507f045f6`, merge `f2a77eadba5b2338ee83ff3a03fa94c6703a81ce`. Se aplicó ese plan exacto y Terraform informó **0 altas, 9 cambios, 0 eliminaciones**. La lectura posterior confirmó los nueve Jobs presentes con protección Terraform desactivada. La instancia SQL sigue `RUNNABLE` y su protección nativa API permanece habilitada; SQL, database, secretos, cuentas de servicio, grants de cliente SQL y datos se preservan. El state mantuvo lineage y avanzó de serial 17 a 18. SHA posterior privado del state: `ea6e926a2cf4e3577979d361207d1cb0d4e70747a352eb59a0c25bbc37b1d568`.

Los tests mock cubren el stage 2 con el retiro de nueve Jobs, nueve invokers, dos actAs y cinco recursos IAM temporales de replay (dos custom roles y tres bindings). El plan fresco stage 2 se generó tras stage 1: **0 altas, 0 cambios, 25 eliminaciones**. El conjunto exacto contiene nueve Jobs, nueve invokers, dos actAs, dos roles custom y tres bindings temporales. SQL, database, metadata de secretos, SAs, accessors y `roles/cloudsql.client` son no-op.

Hashes privados stage 2: binario `f2ffa8af7a7672673b95c95d3cdd8e5325cee7e9bb57c7d26d5d65e802a0a132`; JSON `b5c017b5204ae4d62a52ee8a737af0b99de6ea312ee3bc5ee9e60a48d19c99cb`. El state de entrada mantiene lineage `11eb96d4-a301-9029-11d1-69e1a00f6302`, serial 18; SHA privado `69726e27c8196187f73116e63a5f4a4db30f7289b9bae797038a4bc5307bead8`. La validación automática confirmó conjunto exacto de direcciones, acciones solo `delete` y no-op de los recursos preservados. Plan binario, JSON, tfvars, logs y state quedan bajo `.local/testing/gcp03-revoke-retired-plan-20261016/` (privado, permisos 0600). **Stage 2 no se aplicó y queda pendiente de revisión/decisión separada.**

La preparación de restauración sintética de GCP-03 se describe como propuesta separada; no se ejecutó restore ni se usó el origen persistente. GCP-04 conserva su alcance original de backend y ejecución durable en destino. Esta documentación no autoriza desplegar la aplicación, cargar el dataset persistente ni aplicar stage 2.
