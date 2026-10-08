# Inventario acotado de copias locales — 2026-10-08

Este inventario es una inspección de ubicaciones conocidas del checkout/compose local; no representa una auditoría de todo el equipo o de dispositivos externos. No se borró ni restauró ninguna copia.

## Elementos localizados

- **Datos persistentes de desarrollo (no es un respaldo independiente):** volumen Docker `espacigo_pgdata`, montado en `/var/lib/docker/volumes/espacigo_pgdata/_data`. Se conserva sin cambios.
- **Registro externo de bajas para replays de respaldos:** bind mount `/home/nandev/.local/state/espacigo/privacy-replay`, con archivo `completed-suppressions-v1.json` modo `0600`. Este registro se utiliza para reaplicar bajas después de restaurar una copia antigua; no contiene un dump de PostgreSQL.
- **Directorio de evidencias sintéticas privadas:** bind mount `/home/nandev/.local/share/espacigo/m03-evidence`. Es almacenamiento de ensayo, no se clasifica como respaldo de base.
- **Secretos locales:** bind mounts bajo `.local/secrets/` (runtime y webhook de pago sintético). Se verificó la ruta de montaje, no se leyeron ni copiaron sus contenidos.
- **Mailpit:** contenedor local con `/data` en tmpfs; no es respaldo durable. El mensaje creado por la prueba se retiró por su ID.
- **Archivos de respaldo dentro del checkout:** una búsqueda hasta profundidad cuatro de `*.dump`, `*.backup`, `*.bak`, `*.sql.gz` y `*.tar.gz` no encontró archivos. Las pruebas PostgreSQL crean dumps dentro de directorios temporales administrados por `t.TempDir`; no se conservaron como respaldos de desarrollo.

## Ubicaciones no comprobadas

No se inspeccionaron el resto del home, otros discos/montajes, snapshots del sistema de archivos, respaldos del host, sincronización en nube, copias manuales o exportaciones fuera del checkout. Tampoco se comprobó si existen copias en otros equipos. Por tanto, se desconoce su existencia y contenido; no se afirma que estén inventariadas ni que hayan recibido reaplicación de bajas.
