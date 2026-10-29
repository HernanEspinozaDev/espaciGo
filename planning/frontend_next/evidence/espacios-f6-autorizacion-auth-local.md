# F6 — autorización y preflight del recorrido autenticado

Fecha: 2026-10-28. Base `main` / `3956d2f3459b`, cambios locales sin commit.

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

## Estado al corte

PostgreSQL temporal está activo y migrado. API/Next/Mailpit todavía no se han iniciado para este recorrido; no se creó cuenta y no hay prueba autenticada completa todavía. Se está comprobando la receta de runtime least-privilege y transporte SMTP/TLS antes de iniciarlos. F6 sigue parcial. Las observaciones anteriores de ausencia de autorización quedan superadas sólo para este target y estas operaciones exactas.
