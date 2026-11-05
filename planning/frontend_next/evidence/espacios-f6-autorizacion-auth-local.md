# F6 — autorización y preflight del recorrido autenticado

Fecha: 2026-10-28. Base `main` / `3956d2f3459b`, cambios locales sin commit.

> **Alcance de esta evidencia:** registra autorización y preflight F6, no declara F6 terminada ni autoriza usar su DB para F5. La fase F6 sigue parcial y el estado vigente por fase está en `plan_creacion_espacios_galeria_documentos.md` §8; para F5, consultar su propio delta/gate.

## Autorización nueva y acotada

El owner respondió explícitamente mediante el formulario de autorización:

1. Autorizar recrear exclusivamente PostgreSQL `espacigo_f6_synth_20261028`, loopback `15432`, tmpfs sin volumen nombrado, aplicar V1–V78 después de backup validado y ejecutar Go worker-free `18083` / Next-Playwright `4205` con filesystem privado scratch.
2. Autorizar identidades exclusivamente sintéticas y Mailpit temporal SMTP `11025` / UI `11080`, datos tmpfs, para registro, verificación explícita, login y aislamiento A/B. No cuentas/correos/contraseñas/tokens reales; secretos sintéticos privados y fuera de Git/logs/chat.

No extiende a DB nativa persistente, cloud, scanner real nuevo, migraciones posteriores a V78 ni release. Al finalizar sólo retirar recursos exactos creados, preservar evidencias/backups.

## Preflight y ejecución del padre

- Puertos reservados libres y contenedor exacto ausente antes de crear.
- Contenedor `espacigo-f6-synthetic-20261028-db`, imagen local `postgis/postgis:18-3.6`, puerto enlazado sólo `127.0.0.1:15432`, sin restart automático, memoria/CPU acotadas.
- `HostConfig.Tmpfs` monta `/var/lib/postgresql`; `stat -f` dentro de PGDATA confirmó `tmpfs`. No hay mounts de tipo volume; el único bind es un secreto sintético privado. No se tocó el stack Docker retenido.
- Identidad autenticada confirmada: DB exacta y PostgreSQL18.6. El primer plan falló durante arranque inicial; el padre repitió sólo preflight read-only por TCP, no el script de creación. El plan confirmó cero aplicadas y secuencia exacta V1–V78.
- Backup previo `scratch/f6-auth-integration-20261028/before-v1-v78.dump`, custom, modo0600, 961bytes, `pg_restore --list` y parseo a `/dev/null` válidos; SHA256 `bf7ef99d98655ed9aa5be40c798e414950af3d5957e02fb86ce38e89f2637606`.
- Se creó únicamente el rol runtime requerido en ese cluster sintético y se ejecutó `cmd/dbmigrate`, no SQL de migración manual. Postplan comprobado: **78 aplicadas / 0 pendientes**. No migraciones en DB nativa/cloud.
- Política de cuentas nueva en `.local/testing/test-account-policy.json`, ignorada por Git, directorio0700/archivo0600: máximo2 cuentas sintéticas, operaciones acotadas y material privado en scratch. No credenciales en esta evidencia.

## Estado al corte 2026-10-28

PostgreSQL temporal está activo y migrado. API/Next/Mailpit todavía no se han iniciado para este recorrido; no se creó cuenta y no hay prueba autenticada completa todavía. Se está comprobando la receta de runtime least-privilege y transporte SMTP/TLS antes de iniciarlos. F6 sigue parcial. Las observaciones anteriores de ausencia de autorización quedan superadas sólo para este target y estas operaciones exactas.

## Revalidación de target y puerta de runtime — 2026-10-29 05:27 -03

- `espacigo-f6-synthetic-20261028-db` permanece `running`, PostgreSQL en `127.0.0.1:15432`, PGDATA sobre tmpfs de 1 GiB y sin volumen nombrado; no se recreó, reinició ni modificó. No se inspeccionó el contenido del bind privado de contraseña.
- En una transacción READ ONLY se confirmó `current_database=espacigo_f6_synth_20261028`, usuario administrativo `postgres`, PostgreSQL `18.6`; ledger `schema_migrations`: 78 filas, V1–V78 contiguas, 0 faltantes y 0 versiones >V78.
- Backup antes de cualquier nueva escritura: `/home/nandev/.hermes/cache/scratch/f6-revalidation-20261029/current-before-integration.dump`, formato custom, modo `0600`, 513,989 bytes, SHA-256 `9f1ec1681ed137967f9e9e558b548ac1d1b30cd131338848a98e980c4b25b9e4`; `pg_restore --list` y parseo schema-only a `/dev/null` terminaron correctamente.
- La política de cuentas sintéticas está en `.local/testing/test-account-policy.json` (directorio `0700`, archivo `0600`, ignorado por Git): máximo dos cuentas `example.invalid`, flujo normal y verificación explícita, y sólo acciones listadas allí. No se creó ninguna cuenta ni mensaje SMTP.
- **Puerta real de integración:** la inspección de roles read-only encontró `espacigo_runtime` como `NOLOGIN`, sin membresías y sin permisos SELECT/INSERT en `public.usuario` ni `public.espacio`; no hay login API dedicado disponible. No iniciar API con `postgres`/superusuario ni ejecutar bootstrap, `GRANT`, `CREATE ROLE` o cambio de password sin autorización explícita para el delta exacto. El bootstrap versionado `db/bootstrap/gcp04-runtime/002_local_restore_role_contract.sql` se leyó sin ejecutarlo: exige `current_database()='espacigo'` y crea varios logins/membresías, por lo que no es aplicable sin cambios al target `espacigo_f6_synth_20261028` ni se ejecutó. Por esa puerta, API, Next y Mailpit siguen sin iniciarse y la prueba autenticada permanece pendiente.
- Decisión del owner — 2026-10-29: mantener F6 parcial por ahora. No se autoriza crear el login API ni ampliar membresías/grants en este corte; API/Next/Mailpit y el recorrido autenticado permanecen sin iniciar. Reabrir solo con autorización explícita para el login y el delta mínimo exacto en este target F6, sin V79+, F5, cloud/IAM/deploy ni scanner. La evidencia previa de creación del grupo `NOLOGIN` no equivale a autorización para ampliar sus grants.
