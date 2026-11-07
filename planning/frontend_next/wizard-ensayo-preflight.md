# Ensayo local de mejoras del wizard — preflight/setup

**Estado del snapshot original (2026-11-06): superseded por el addendum fechado al final.** El ensayo aislado fue autorizado y su preparación local se completó; migraciones, cambios ACL, API/Next/Mailpit y cuentas siguen pendientes. Este registro no autoriza ejecutarlos.

## Identidad fija del ensayo

- Compose project, container y database: `espacigo_wizard_20261106`.
- PostgreSQL sólo loopback `127.0.0.1:15436`; API `127.0.0.1:18086`; Next `127.0.0.1:4206`; Mailpit HTTP `127.0.0.1:18087`, SMTP `127.0.0.1:11036`.
- Imagen prevista PostgreSQL/PostGIS: `postgis/postgis@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677`. El script sólo reporta si está localmente disponible con ese digest; no descarga ni arranca imagen.
- No reutilizar ni consultar recursos F5/demos/volúmenes ajenos. No usar `docker compose down -v`, prune ni nombres genéricos.

## Uso actual (no muta Docker ni PostgreSQL)

Desde la raíz del checkout:

```sh
python3 scripts/wizard-improvements-local.py
# equivalente explícito:
python3 scripts/wizard-improvements-local.py preflight
```

Preflight comprueba disponibilidad local de puertos, Docker/daemon, digest de imagen, presencia del migrador/fuentes y presencia (sin leer ni revelar) de `.local/testing/test-account-policy.json`. Un puerto ocupado devuelve código 1. No se mata ningún proceso ni se reserva puerto. No imprime política, secretos ni credenciales.

El modo siguiente **siempre falla cerrado** incluso con confirmación correcta; es intencional mientras falte contrato revisado:

```sh
python3 scripts/wizard-improvements-local.py setup --confirm espacigo_wizard_20261106
```

No hay todavía Compose/runner setup seguro implementado. No improvisar grants/roles, SQL de bootstrap ni credenciales para superar ese bloqueo.

## Bloqueo técnico que debe resolverse en revisión

El CLI `cmd/dbmigrate` exige para ejecución normal `DATABASE_URL` con login `espacigo_migrator_login` y `DATABASE_SET_ROLE=espacigo_schema_owner`; no acepta migrador `postgres` como sustitución. `-plan-only` es de sólo lectura. Las migraciones fuente ya contienen validaciones de identidad/membresía y cambios ACL; `internal/dbbootstrap` contiene contrato de permisos runtime, pero no se encontró un bootstrap reutilizable aprobado para crear los roles en esta DB nueva. En particular, `espacigo_api_login` debe usar `SET ROLE espacigo_runtime`, y el rol runtime no ejecuta migraciones.

Hasta que el dueño revise una ruta exacta de provisioning/admin temporal con privilegio mínimo, su SQL fuente, membresías, owners y readback, **no crear roles ni DB mediante un procedimiento inventado**. No elevar migrator/runtime ni cambiar guards/migraciones. La futura implementación debe consumir passfile `0600`, directory `0700`; nunca pasar password por URL, argumento, entorno, logs o Compose environment. Contraseña inicial, de ser necesaria, sólo por archivo montado de modo privado (no Docker Env).

## Gate necesario antes de habilitar setup/migración

1. Repetir este preflight y comprobar manualmente propietarios de listeners; abortar ante cualquier colisión.
2. Revisar Compose resuelto y validar project/container/volume exclusivos, puertos loopback, labels de propiedad, mounts y allowlist de entorno. Congelar digest de cada imagen y checksum del manifiesto.
3. Crear únicamente recursos pertenecientes al proyecto exacto con confirmación textual; nunca adoptar un contenedor/volumen preexistente por coincidencia de nombre.
4. Provisionar DB vacía y roles sólo con procedimiento de mínimo privilegio revisado. Validar readback de DB/identidad, flags de roles/membresías/owners y límites de runtime. Runtime: login `espacigo_api_login` con `SET ROLE espacigo_runtime`; no superuser ni DDL.
5. Guardar plan/inventario/ledger en archivo privado. Ejecutar binario versionado, no `psql` para migraciones:

   ```sh
   umask 077
   env -i PATH="$PATH" HOME="$HOME" PGPASSFILE="$PRIVATE_PGPASSFILE" \
     DATABASE_CONNECTION_MODE=direct \
     DATABASE_URL='postgres://espacigo_migrator_login@127.0.0.1:15436/espacigo_wizard_20261106?sslmode=disable' \
     DATABASE_SET_ROLE=espacigo_schema_owner \
     go run -mod=readonly ./cmd/dbmigrate -plan-only > "$PRIVATE_PLAN"
   ```

   `$PRIVATE_PGPASSFILE` y `$PRIVATE_PLAN` son placeholders de rutas privadas, no valores literales; nunca imprimirlos si contienen datos sensibles. Evitar `set -x`. Validar en el plan target, identidad, estado inicial, lista completa de versiones y checksums byte a byte. Si se invoca binario construido, construirlo desde el SHA revisado y fijar su checksum.
6. Antes del primer DDL, backup custom a directorio privado persistente fuera de tmpfs, validar `pg_restore --list` y schema-only parse/exit-code, y guardar checksum/manifest readback. Si no se puede validar, detenerse. Repetir plan y ledger inmediatamente antes de aplicar.
7. Sólo tras autorización/revisión específica, invocar la misma CLI con mismas variables/passfile y sin `-plan-only`; no usar flags de admin/F5/runtime-ACL. Tras ello hacer plan read-only y comparar ledger/checksums/owners contra fuentes congeladas. No manipular ledger ni reparar a mano.
8. Arranque API worker-free separado con `espacigo_api_login` y `SET ROLE espacigo_runtime`; verificar destino/puerto exactos y ausencia de workers antes de interfaz. Crear máximo dos cuentas sintéticas sólo mediante flujo normal y política privada local; no registrar cuentas en esta fase.

El ejecutor deberá registrar resultado sanitizado, SHA de checkout, digests/checksums y ledger sin secretos. Si identidad, ownership, ACL, backup o checksums divergen, parar y conservar evidencia privada; no conceder permisos adicionales.

## Addendum de estado — 2026-11-06

Este addendum actualiza el estado sin reescribir la fotografía histórica anterior. La autorización humana recibida cubre **ese ensayo local nuevo y aislado**, con máximo dos cuentas sintéticas normales; no equivale a aceptación de producto. La creación de esas cuentas ya está autorizada dentro del alcance, pero su provisionamiento operativo queda para cuando el esquema y la API estén listos.

- Checkout de ensayo: rama `codex/frontend/fix/mejoras-creacion-espacios`, base SHA `771134d165046b290b1f5ad3a1074a6357c2327f`. El código fuente tenía cambios locales preexistentes; no se atribuyen a este registro.
- El proxy TCP asyncio propio, ejecutado como `python3 scripts/wizard-improvements-forward.py` (PID 188876), escucha en `127.0.0.1:15436` y reenvía a `172.26.0.2:5432`, PostgreSQL dentro del contenedor Docker del ensayo. No es SSH, no usa bastión ni conecta a cloud. El proceso existente, de propiedad verificada del ensayo, se conservó sin detenerlo ni reemplazarlo.
- Evidencia real de `resume-readback`: identidad destino/base y rol de sesión verificados; ausencia de relaciones de aplicación confirmada; backup custom ya existente validado con `pg_restore --list` y parseo completo (`pg_restore --file=/dev/null`). Resultado reportado: `readback and backup validated for existing isolated trial`. El artefacto y credenciales permanecen bajo el directorio privado excluido; sus valores no se reproducen aquí.
- `prepare` se ejecutó con confirmación exacta `espacigo_wizard_20261106` y `PGPASSFILE` administrativo privado. Bootstrap realizado: creó/configuró los roles y contraseñas y las membresías previstos por el setup, con readback de identidad y roles. No se ejecutaron migraciones ni se aplicó ACL runtime adicional fuera de ese bootstrap de roles/membresías. Una primera ejecución falló por formato de parámetros y revirtió íntegramente; casteos explícitos a `text` corrigieron el problema antes de la ejecución completada.
- Verificación local del ejecutor: script `--check` PASS, 15 pruebas Python PASS y pruebas Go PASS. No se ejecutó `cmd/dbmigrate`, ni se leyó/aplicó ledger, ni se ejecutaron API/Next/Mailpit, ni se crearon cuentas. Las cuentas sintéticas ya están autorizadas, pero el alta operativa espera a que esquema y API estén listos.
- La variante ACL ya está implementada en fuente y fue revisada. Faltan regresiones focales y verificación contra DB; no está aprobada ni aplicada en la base del ensayo. No describirla como pendiente de implementación.
- Siguiente paso permitido: preparar/revisar un plan read-only con `cmd/dbmigrate`, usando el passfile migrador propio del flujo; identificar nombre/ruta vigente en código/recibo privado antes de invocarlo y no reutilizar el passfile administrativo. Mantener secretos y salida sensible privados. Cualquier migración/DDL requiere gate separado.
- Base de código de referencia: SHA `771134d165046b290b1f5ad3a1074a6357c2327f`; el checkout presenta cambios de fuente locales, por lo que no se afirma que ese SHA identifique el árbol de trabajo íntegro.
