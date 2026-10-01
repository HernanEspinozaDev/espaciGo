# Migraciones PostgreSQL y base local reproducible

Estado: el runner ejecutable de CORE-DB-03 (`cmd/dbmigrate` e `internal/migrator`) y sus seis pruebas de integración se verificaron en PostgreSQL descartable. La tarjeta queda pendiente de revisión del usuario; no marcar Done ni avanzar a AUTH-DB-02 antes de esa revisión. Este documento complementa [base_de_datos.md](base_de_datos.md). No usar el SQL ilustrativo antiguo del Anexo B como migración vigente.

## Contrato de migraciones

- Ubicación: `db/migrations/` en el repositorio backend. Una migración por archivo SQL, UTF-8, LF, nombre inmutable `V<versión>__<descripcion_snake_case>.sql`; versión numérica positiva, única y orden lexicográfico equivalente a orden numérico (usar ancho fijo, por ejemplo `V000001__...`). No renumerar ni reutilizar versiones.
- El orden es ascendente por versión. El historial registra versión, nombre, SHA-256 del contenido exacto del archivo (bytes), fecha UTC de aplicación y checksum de la herramienta/runner. El checksum se calcula antes de ejecutar, sin normalizar saltos de línea ni comentarios.
- En cada ejecución, comparar el conjunto instalado con los archivos: checksum distinto en versión aplicada, archivo aplicado ausente/renombrado, versión repetida, hueco en la secuencia desde la primera versión o historial que no corresponde al conjunto esperado es deriva y debe abortar antes de ejecutar cualquier SQL de migración. No reparar ni actualizar checksums automáticamente. Para bases nuevas se exige secuencia completa contigua desde V000001.
- Serialización: el runner inicial toma `pg_advisory_lock` de sesión con las claves estables reservadas por la aplicación (`0x45535047`, `0x4d494752`) antes de inspeccionar/aplicar y mantiene el lock hasta terminar; vuelve a leer el historial bajo el lock y libera el lock al final. No se usa lock transaccional.
- Transaccionalidad v1: cada archivo SQL, junto con su registro de historial, corre en una transacción única. Si falla, rollback completo y no se registra como aplicado. El runner actual **no** implementa migraciones `non_transactional`; no incluir operaciones que PostgreSQL prohíba dentro de una transacción (p. ej. `CREATE INDEX CONCURRENTLY`) hasta ampliar y probar explícitamente el contrato.
- No se promete rollback automático: cambios destructivos o incompatibles siguen expandir → migrar datos → contraer en releases distintos, con respaldo restaurable y plan de forward-fix. No incluir una migración inversa ficticia. Una migración fallida transaccional se corrige en el mismo archivo solo si nunca llegó a commit en ningún entorno; cualquier migración ya aplicada se corrige con una nueva versión.
- Solo el rol de migración puede cambiar el esquema. Rol API sin `CREATEDB`, `CREATEROLE` ni propiedad de objetos; permisos mínimos por esquema/tablas/secuencias explícitos. El rol de operación es separado y de acceso humano controlado. Credenciales fuera de git y de los fixtures.

## Base vacía reproducible

La prueba usa PostgreSQL 18 y extensiones autorizadas PostGIS 3.6 y `btree_gist` 1.8, con esta imagen reproducible fijada por digest: `postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677`. El contenedor no monta volúmenes persistentes; la prueba usa almacenamiento temporal `tmpfs`, puerto aleatorio ligado a loopback y credenciales sintéticas efímeras. No depender de la instalación PostgreSQL local ni de `latest`.

El runner está implementado como `cmd/dbmigrate` y `internal/migrator`. Lee `DATABASE_URL`, acepta `-dir <directorio-de-migraciones>` (por defecto `db/migrations/`), y aplica únicamente archivos `V<seis dígitos>__<snake_case>.sql`. El conjunto de integración requiere `TEST_DATABASE_URL` apuntando a una base temporal con permisos para crear/eliminar bases; cada caso crea su propia base de prueba y la elimina al terminar.

**Verificación ejecutada el 2026-10-01:** con PostgreSQL `18.6 (Debian 18.6-1.pgdg13+2)`, PostGIS `3.6.4` y `btree_gist` `1.8` en la imagen fijada arriba, se verificó la creación/versiones de ambas extensiones y pasaron las seis pruebas de integración, sin `DATABASE_URL` y con `TEST_DATABASE_URL` apuntando solo al contenedor desechable:

- `TestRunnerAppliesFromEmptyAndRepeatIsNoop` — PASS
- `TestRunnerStoresExactMigrationAndRunnerChecksums` — PASS
- `TestRunnerRejectsChecksumDriftBeforeDDL` — PASS
- `TestRunnerRollsBackMigrationAndHistoryTogether` — PASS
- `TestRunnerSerializesConcurrentExecutions` — PASS
- `TestRunnerRejectsVersionGapBeforeDDL` — PASS

Comando de pruebas: `env -u DATABASE_URL go test -count=1 -v ./internal/migrator -run '^(TestRunnerAppliesFromEmptyAndRepeatIsNoop|TestRunnerStoresExactMigrationAndRunnerChecksums|TestRunnerRejectsChecksumDriftBeforeDDL|TestRunnerRollsBackMigrationAndHistoryTogether|TestRunnerSerializesConcurrentExecutions|TestRunnerRejectsVersionGapBeforeDDL)$'`, con `TEST_DATABASE_URL` fijada explícitamente al puerto loopback dinámico del contenedor. El contenedor temporal usó `tmpfs`, sin volumen persistente, y fue eliminado al terminar; Docker confirmó que el ID `15d936a72b8a25e2a683c9a3cc38a1fea847bbfb8ccacc64fbaa7ef48db93162` ya no existe. No se conectó a la instalación PostgreSQL local.

Las pruebas unitarias/compilación, `go test -race ./...`, `go vet ./...` y `git diff --check` también pasaron. No se avanzó a AUTH-DB-02. CORE-DB-03 queda pendiente de revisión del usuario y no debe marcarse Done antes de esa revisión.

### Limpieza y aislamiento

El procedimiento configura `--restart=no`, publica el puerto solo en `127.0.0.1`, monta `/var/lib/postgresql` como `tmpfs`, genera credencial sintética efímera y elimina el contenedor mediante `trap` incluso si falla una prueba. El comando de pruebas elimina explícitamente `DATABASE_URL` y establece `TEST_DATABASE_URL` solo para la instancia descartable. Las seis pruebas crean y eliminan sus bases aisladas; la verificación de extensiones se realizó en la base administrativa descartable del contenedor.


## Semillas y extensiones

En migraciones solo instalar extensiones autorizadas y requeridas: `postgis` y `btree_gist`; registrar versión observada, no asumir que la versión patch local coincide con la imagen. Semillas de datos únicamente para catálogos públicos controlados, deterministas e idempotentes (`INSERT ... ON CONFLICT` con claves estables); fixtures sintéticos van en pruebas y nunca en migraciones productivas. Ningún seed de cuentas, tokens, propietarios o información real.

## Recuperación operativa

Antes de aplicar migraciones en un entorno persistente: respaldo verificado/restaurable, ventana/impacto revisados, migración probada desde vacío y desde la versión soportada, y plan de forward-fix. Ante checksum divergente o historial inesperado, detener despliegue y preservar logs/backup; no borrar historial ni recalcular checksum. Tras una migración parcial no transaccional, inspeccionar estado real y redactar una nueva corrección revisada. La restauración desde respaldo es contingencia, no sustituto de diseño compatible.
