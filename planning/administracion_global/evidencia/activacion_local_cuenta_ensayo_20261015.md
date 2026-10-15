# Cuenta de ensayo local y activación GADM-04 — 2026-10-15

## Entorno

- Código: `638ac70d28265c6903ff1a836c60141a9e2cf67d` (`main`, tras PR #316).
- Destino local habitual: `espacigo_local`, volumen `espacigo_pgdata`; proyecto Compose `espacigo`.
- La configuración resuelta enlaza Angular/API/Mailpit solo a loopback en 4200/8080/8025. Los puertos estaban libres antes de iniciar esos servicios. Se iniciaron Mailpit, backend y frontend omitiendo dependencias; la base ya saludable no se recreó ni reinició.
- URL local: `http://127.0.0.1:4200/`; frontend `/healthz`, API `/health/ready` y Mailpit respondieron 200.

## Upgrade del esquema

La base tenía V1–V54. Los nombres/checksums aplicados hasta V54 coincidían con el repositorio. Se construyó el migrador desde este checkout limpio y se ejecutó el servicio normal `migrate` con `--no-deps`; aplicó V55–V59. Después, los 59 nombres y checksums coincidieron con los archivos del repositorio. El volumen persistente y el contenedor PostgreSQL se conservaron; su hora de inicio no cambió. El conteo de usuarios y espacios permaneció en 44 y 41; el tamaño pasó de 27 MB a 28 MB. V55 genera la columna derivada de búsqueda pública para espacios activos; V57 reemplaza la pequeña allowlist de filtros por su manifiesto canónico. No hubo reinicio, borrado de base ni acceso a demos retenidas.

## Registro/verificación y límite de activación

La cuenta de ensayo indicada por la política local privada no existía. Se creó por `/auth/register` (201), recibió verificación por el Mailpit local (204), y pudo iniciar/cerrar sesión con el rol ordinario de arrendatario. La cuenta quedó activa/verificada, sin bloqueo administrativo, sin membership global y sin sesión abierta. Correo y contraseñas no se incluyen aquí; el UUID y estado detallado están solo en `.local/testing/local-account-activation.json` (0600).

El bootstrap global **no se ejecutó**. La inspección de roles mostró que `espacigo_admin` tiene `rolsuper=true`, mientras `espacigo_runtime` no tiene `INSERT` sobre `administrador_global`, `administrador_global_bootstrap` ni su tabla de eventos. No existe un principal CLI de privilegio mínimo. Usar el rol superusuario en `activate-local` contravendría las reglas de seguridad del repositorio; no se cambió el propietario bootstrap ni se concedieron privilegios manuales. La tabla bootstrap permanece vacía.

## Implementación en revisión

El cambio de seguimiento añade V60 con el grupo `NOLOGIN` `espacigo_gadm_bootstrap`, membresía temporal explícita, grants por columna validados al abrir cada conexión de `activate-local`, y un trigger que impide que el permiso `UPDATE(id)` usado para `SELECT FOR UPDATE` cambie la identidad. La concesión de `SELECT(usuario_id)` en la membership global cubre la comprobación `ON CONFLICT`; las tablas bootstrap/evento no reciben grants de tabla completos.

La migración V60 **solo se aplicó en la base desechable de pruebas**. La base persistente `espacigo_local` permanece en V59: no se creó el rol ni login temporal, no se concedieron permisos y no se ejecutó bootstrap/recuperación. La cuenta verificada sigue sin membership global. La operación persistente queda pendiente de revisión del PR.

Pruebas de evidencia ejecutadas en contenedores desechables PostgreSQL 18/PostGIS 3.6:

- `TestGADM04RestrictedBootstrapRoleMigrationFromV59`: PASS; upgrade desde V59, integridad de historial V1–V59, atributos/membresías del rol y allowlist exacta de 23 grants por columna.
- `TestGlobalBootstrapDirectoryAndLastMasterConcurrency`: PASS; la misma configuración de conexión restringida que usa `activate-local` rechaza una sesión sobreprivilegiada y admite bootstrap idempotente/recovery del UUID ligado, rechaza otro UUID, no lee hash de clave ni ejecuta cambios fuera de alcance, y al final elimina login y membership. Las verificaciones negativas cubren UPDATE de estado/identidad, DELETE/TRUNCATE de bootstrap/auditoría y lectura de reservas.
- `scripts/test-local-qa-backend.sh`: PASS; suite unitaria, `go vet`, 92 integraciones obligatorias, cero pruebas omitidas, y cinco repeticiones estables de la integración de booking. Se corrigió el fixture histórico V54 para limitar su upgrade de regresión a V59, sin cambiar migraciones.

Esto prueba la implementación técnica en entorno desechable, no la activación de la cuenta persistente ni aceptación de producto.

## Higiene

El backend no registró el correo de política, contraseña, token de verificación ni token de sesión. La única notificación de verificación de esta cuenta fue borrada de la bandeja Mailpit local por su ID; no quedan mensajes coincidentes. La política original continúa excluida de Git bajo `.local/testing/` con modo 0600. No se tocó el checkout principal ajeno ni la demo GADM-04 retenida.
