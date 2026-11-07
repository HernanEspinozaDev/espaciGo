# Preflight — ensayo aislado del wizard de espacios

Estado del snapshot original (2026-11-06): **superseded por el addendum fechado al final**. La preparación del ensayo aislado está ejecutada; migraciones/DDL, ACL variante, servicios y cuentas continúan pendientes.

## Identidad y límites propuestos

- Checkout: `codex/frontend/fix/mejoras-creacion-espacios`, HEAD inicial `771134d165046b290b1f5ad3a1074a6357c2327f`.
- Proyecto/stack y base nuevos: `espacigo_wizard_20261106`; nunca reutilizar F5, PostgreSQL nativo, cloud, demos ni volúmenes retenidos.
- Bindings loopback exclusivamente: PostgreSQL `127.0.0.1:15436`, API `127.0.0.1:18086`, Next HTTPS `127.0.0.1:4206`, Mailpit HTTP `127.0.0.1:18087` y SMTP `127.0.0.1:11036`.
- Evidencia/blob storage privado y exclusivo bajo `.local/testing/wizard-improvements/`; permisos de directorio 0700 y archivos 0600. No imprimir ni exportar los datos privados.
- Máximo dos cuentas sintéticas nuevas bajo `@example.invalid`, únicamente por registro y verificación normales. No buscar, recuperar, sobrescribir ni modificar cuentas existentes. Aún no se han creado cuentas.
- Perfil API `local-space-wizard`: workers desactivados; sólo destino exacto loopback/puerto/base/rol del ensayo. Runtime API no superusuario. Migrador/setup separado del runtime.

## Inspección de solo lectura realizada

- No hay listener en los cinco puertos propuestos al momento del preflight (2026-11-06). Esto debe repetirse inmediatamente antes de arrancar; no matar ni desplazar ningún proceso ajeno.
- Imágenes instaladas relevantes incluyen `postgres:18-alpine`, `postgis/postgis:18-3.6`, `espacigo-api:ci`, `axllent/mailpit:v1.31.4` y `espacigo-web:ci`. No se inicia ni selecciona una imagen hasta pin de digest y revisión de compatibilidad.
- No existía `.local/testing/` en este checkout; se prepara una política nueva, sin copiar la de otro checkout ni incluir credenciales.
- El migrador versionado está en `cmd/dbmigrate`; las migraciones fuente son `db/migrations/`. Ejecutar el plan read-only contra la base nueva y revisar inventario/orden/checksums. Sólo `cmd/dbmigrate`, con identidad de migración/setup separada y tras aprobación, puede aplicar migraciones existentes. Nunca iniciar DDL desde API/runtime ni editar migraciones.

## Contrato de privilegios y seguridad

- La cuenta setup/admin sólo crea la DB y prepara roles mínimos tras aprobación explícita; no se usa para tráfico API.
- Runtime: login no-superuser, `NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT` (o flags equivalentes revisados), membresía/`SET ROLE` sólo al rol runtime existente, sin ownership, DDL, acceso ledger/migraciones ni grants globales.
- El rol migrador aplica exclusivamente las migraciones existentes por `cmd/dbmigrate`, con la identidad/ACL que definan sus contratos versionados. Derivar permisos del código y migraciones reales; nada de `GRANT ALL`, blanket grants o suposiciones de nombre.
- Capturar checksum e imagen por digest antes de recursos. Antes de DDL: confirmar destino, identidad, inventario migratorio inmutable, ledger vacío/esperado mediante lectura administrativa autorizada, plan secuencial, backup validado y revisión humana. El backup requiere estado inicializado; por tanto la revisión previa a cualquier DDL debe autorizar por separado sólo la creación del entorno/base vacía. Detener antes de migrar si falta gate.
- Next no recibe credenciales PostgreSQL. API únicamente loopback y configuración con allowlist de variables; secretos sólo archivos privados.

## Secuencia futura (no ejecutar sin revisión)

1. Repetir `ss -ltnp` para 15436/18086/4206/18087/11036 y revisar propietarios; abortar ante colisión.
2. Seleccionar y fijar digests compatibles de imágenes existentes; definir Compose con nombre/proyecto y recursos/volumen exclusivos. Revisar configuración resuelta antes de `up`.
3. Preparar roles separados y DB vacía únicamente con autorización; registrar identidad/flags y comprobar runtime least-privilege.
4. Preflight `cmd/dbmigrate` en modo plan/solo lectura; validar source inventory, checksums, ledger y destino exactos. Backup validado cuando haya contenido y antes de cualquier migración.
5. Tras revisión independiente específica, aplicar migraciones inmutables en orden; leer ledger/checksums después.
6. Crear/revisar API worker-free y probar sólo al destino autorizado; Next y Mailpit locales. Registrar sólo resultados sanitizados.
7. Crear hasta dos cuentas por UI/API normal y verificación Mailpit, sin copiar mensajes, enlaces o tokens a evidencia.

Comandos de revisión previstos (no son ejecución autorizada ahora): `ss -ltnp '( sport = :15436 or sport = :18086 or sport = :4206 or sport = :18087 or sport = :11036 )'`; `go test ./cmd/api -run 'TestRuntimeProfile|TestLaunchWorkers|TestValidateWorkerRegistry' -count=1`; revisar `cmd/dbmigrate` y `db/migrations` antes de preparar cualquier plan.

## Pendiente / límites

No se creó DB, rol, contenedor, volumen, API, Next ni cuenta; no se aplicó DDL ni se conectó a una base. El puerto disponible observado no es reserva. Falta revisar por padre el perfil/runbook y confirmar imagen por digest, flujo migrador concreto, rol/runtime ACL exactos, backup/restore del nuevo entorno y gates de arranque. El checkout ya tenía cambios preexistentes de wizard no relacionados; deben preservarse y revisarse por separado.

## Addendum de evidencia — 2026-11-06

Esta sección es una actualización posterior y no altera la fotografía histórica de preflight de arriba. La autorización de hasta dos cuentas sintéticas por flujo normal ya existe; su alta operativa queda condicionada a que el esquema y la API estén listos, no a una nueva aprobación humana.

- **Forward/destino:** proxy TCP asyncio propio (`python3 scripts/wizard-improvements-forward.py`, PID 188876) en `127.0.0.1:15436`, reenviando a `172.26.0.2:5432`, PostgreSQL dentro del contenedor Docker del ensayo. No es forward SSH, no usa bastión ni conecta a cloud; el proceso se conservó sin reemplazo.
- **Backup/readback:** ejecución de `resume-readback` validó identidad de destino/base y rol de sesión, ausencia de relaciones de aplicación y backup custom con `pg_restore --list` y parseo completo. Salida sanitizada: `readback and backup validated for existing isolated trial`. No se copia a esta evidencia el artefacto, passfiles ni ningún dato privado.
- **Bootstrap/preparación:** `go run ./cmd/wizardtrialsetup --action=prepare --confirm-target=espacigo_wizard_20261106` con `PGPASSFILE` administrativo privado completó el bootstrap previsto: roles/contraseñas y membresías creados/configurados y verificados mediante readback. No hubo migraciones ni aplicación de ACL runtime más allá de roles/membresías del bootstrap. El intento inicial falló por formato de parámetros y revirtió; casteos explícitos a `text` corrigieron el problema antes de completar.
- **Pruebas reportadas:** `scripts/wizard-improvements-local.py --check` PASS; 15 pruebas Python PASS; Go PASS. No se ejecutaron migraciones ni lectura/escritura del ledger; tampoco API, Next, Mailpit o creación de cuentas. El alta de hasta dos cuentas sintéticas ya está autorizada, pero espera a que esquema y API estén listos.
- **ACL y límites:** la variante ACL ya está implementada en fuente y revisada; faltan regresiones focales y verificación contra DB. No está aprobada ni aplicada en la base del ensayo. Plan `cmd/dbmigrate` únicamente de lectura sigue pendiente; identificar en código/recibo el passfile migrador exacto, sin reutilizar el passfile administrativo. Cualquier DDL/migración requiere gate separado.
