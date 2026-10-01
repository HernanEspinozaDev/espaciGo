# Migraciones PostgreSQL y base local reproducible

Estado: el contrato y el runner inicial de CORE-DB-03 están en esta rama (`cmd/dbmigrate` y `internal/migrator`). Las pruebas unitarias sí se ejecutaron; las pruebas de integración contra PostgreSQL 18 **no** se ejecutaron porque la aprobación para iniciar la base temporal fue retirada. La tarjeta permanece incompleta y no debe marcarse Done hasta obtener esa evidencia y la revisión del usuario. Este documento complementa [base_de_datos.md](base_de_datos.md). No usar el SQL ilustrativo antiguo del Anexo B como migración.

## Contrato de migraciones

- Ubicación: `db/migrations/` en el repositorio backend. Una migración por archivo SQL, UTF-8, LF, nombre inmutable `V<versión>__<descripcion_snake_case>.sql`; versión numérica positiva, única y orden lexicográfico equivalente a orden numérico (usar ancho fijo, por ejemplo `V000001__...`). No renumerar ni reutilizar versiones.
- El orden es ascendente por versión. El historial registra versión, nombre, SHA-256 del contenido exacto del archivo (bytes), fecha UTC de aplicación y checksum de la herramienta/runner. El checksum se calcula antes de ejecutar, sin normalizar saltos de línea ni comentarios.
- En cada ejecución, comparar el conjunto instalado con los archivos: checksum distinto en versión aplicada, archivo aplicado ausente/renombrado, versión repetida, hueco en la secuencia desde la primera versión o historial que no corresponde al conjunto esperado es deriva y debe abortar antes de ejecutar cualquier SQL de migración. No reparar ni actualizar checksums automáticamente. Para bases nuevas se exige secuencia completa contigua desde V000001.
- Serialización: el runner inicial toma `pg_advisory_lock` de sesión con las claves estables reservadas por la aplicación (`0x45535047`, `0x4d494752`) antes de inspeccionar/aplicar y mantiene el lock hasta terminar; vuelve a leer el historial bajo el lock y libera el lock al final. No se usa lock transaccional.
- Transaccionalidad v1: cada archivo SQL, junto con su registro de historial, corre en una transacción única. Si falla, rollback completo y no se registra como aplicado. El runner actual **no** implementa migraciones `non_transactional`; no incluir operaciones que PostgreSQL prohíba dentro de una transacción (p. ej. `CREATE INDEX CONCURRENTLY`) hasta ampliar y probar explícitamente el contrato.
- No se promete rollback automático: cambios destructivos o incompatibles siguen expandir → migrar datos → contraer en releases distintos, con respaldo restaurable y plan de forward-fix. No incluir una migración inversa ficticia. Una migración fallida transaccional se corrige en el mismo archivo solo si nunca llegó a commit en ningún entorno; cualquier migración ya aplicada se corrige con una nueva versión.
- Solo el rol de migración puede cambiar el esquema. Rol API sin `CREATEDB`, `CREATEROLE` ni propiedad de objetos; permisos mínimos por esquema/tablas/secuencias explícitos. El rol de operación es separado y de acceso humano controlado. Credenciales fuera de git y de los fixtures.

## Base vacía reproducible

La prueba usa PostgreSQL 18 y extensiones autorizadas PostGIS 3.6 y `btree_gist` 1.8, con la imagen fijada por digest en `base_de_datos.md`. No depender de la instalación local ni de `latest`.

El runner está implementado como `cmd/dbmigrate` y `internal/migrator`. Lee `DATABASE_URL`, acepta `-dir <directorio-de-migraciones>` (por defecto `db/migrations/`), y aplica únicamente archivos `V<seis dígitos>__<snake_case>.sql`. El conjunto de integración requiere `TEST_DATABASE_URL` apuntando a una base temporal con permisos para crear/eliminar bases; cada caso crea su propia base de prueba y la elimina al terminar.

**Verificaciones ejecutadas:** `go test ./...` pasó sin `TEST_DATABASE_URL` (pruebas unitarias/compilación; las pruebas de integración quedan omitidas). `git diff --check` también pasó. **Pendiente:** ejecutar el conjunto con `TEST_DATABASE_URL` contra PostgreSQL 18.6/PostGIS/`btree_gist`, revisar la salida real y después actualizar el estado de la tarjeta. No afirmar que la migración fue aplicada contra PostgreSQL todavía.

### Verificación PostgreSQL pendiente — requiere aprobación explícita

Las pruebas de integración requieren `TEST_DATABASE_URL` contra la imagen de PostgreSQL/PostGIS fijada en `base_de_datos.md`. El comando de inicio que se solicitó recuperar no pudo encontrarse en la sesión histórica accesible; para evitar sustituirlo por una propuesta no aprobada, no se incluye ni ejecuta otro comando. No se creó contenedor ni base temporal.

Sin esa evidencia, CORE-DB-03 permanece incompleta y AUTH-DB-02 continúa bloqueada.


## Semillas y extensiones

En migraciones solo instalar extensiones autorizadas y requeridas: `postgis` y `btree_gist`; registrar versión observada, no asumir que la versión patch local coincide con la imagen. Semillas de datos únicamente para catálogos públicos controlados, deterministas e idempotentes (`INSERT ... ON CONFLICT` con claves estables); fixtures sintéticos van en pruebas y nunca en migraciones productivas. Ningún seed de cuentas, tokens, propietarios o información real.

## Recuperación operativa

Antes de aplicar migraciones en un entorno persistente: respaldo verificado/restaurable, ventana/impacto revisados, migración probada desde vacío y desde la versión soportada, y plan de forward-fix. Ante checksum divergente o historial inesperado, detener despliegue y preservar logs/backup; no borrar historial ni recalcular checksum. Tras una migración parcial no transaccional, inspeccionar estado real y redactar una nueva corrección revisada. La restauración desde respaldo es contingencia, no sustituto de diseño compatible.
