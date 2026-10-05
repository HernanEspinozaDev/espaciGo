# AUTH-BE-01 — evidencia de dominio y repositorio

Registro histórico del servidor (no verificado desde este PC): verificación reproducible reportada como re-ejecutada el 2026-10-05 sobre el commit `ac9bf06d24f015806dfa12e6ddca194d40644e59`, en worktree temporal separado. Fixtures sintéticos; no se usaron datos personales ni credenciales persistentes.

## sqlc y verificaciones históricas del servidor

- `go generate ./internal/adapters/postgres/identity` (sqlc v1.31.1) — PASS; el diff de los archivos generados quedó limpio, confirmando reproducibilidad.
- `go test -race ./...` — PASS, ejecutado nuevamente con `TEST_DATABASE_URL` apuntando al PostgreSQL desechable.
- `go test -count=1 -v ./internal/adapters/postgres/identity` — PASS; 7 pruebas de integración, ejecutadas nuevamente sin caché.
- `go vet ./...` — PASS, ejecutado nuevamente.
- `git diff --check` — PASS, ejecutado nuevamente.
- PostgreSQL 18 desechable: imagen `postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677`. Los tests crearon y retiraron sus bases aleatorias; al finalizar no quedaron bases `auth_be01_test_*` ni sesiones de cliente.
- Cobertura de integración observada: alta atómica de cuenta/rol/aceptaciones y rollback; conflicto/errores; estados de cuenta/sesión/términos; expiración idle/absoluta; reemplazo concurrente de token, intentos, consumo y conteo.

## Corrección de limpieza reportada en el servidor (histórica)

La nota de la ejecución anterior afirmaba que PostgreSQL se había retirado, pero la inspección del 2026-10-05 encontró el contenedor `auth-be01-pg-20261004T225310` aún en ejecución desde el 2026-10-04. Se comprobó que era el recurso desechable autorizado: imagen indicada arriba, red `none`, almacenamiento de datos en `tmpfs`, solo el socket temporal montado, y ningún cliente activo ni base de prueba remanente. Tras re-ejecutar las pruebas, el contenedor y su directorio de socket fueron retirados; se verificó que ambos ya no existen. La afirmación anterior queda corregida. DSN omitido deliberadamente; fixtures y credenciales de prueba fueron efímeros y locales.

## Alcance pendiente

AUTH-DB-02 está integrada en `main` por el PR #15 (`0911b9c85643c8893950b381aec89b6249511706`). DB02-09 y RQF-213/-217/-218 continúan abiertos; no se añadió DDL para resolverlos. AUTH-BE-01 sigue pendiente de revisión e integración del PR #16; AUTH-BE-02 y AUTH-BE-03 permanecen bloqueadas hasta que esa entrega cumpla aceptación y se fusione.


## Verificación nueva en este PC — 2026-10-05

Cuenta efectiva de GitHub: `HernanMEC` (ID 152030325). Identidad local de autor/committer: `HernanMEC <152030325+HernanMEC@users.noreply.github.com>`. El checkout inicial estaba limpio; se recuperó la rama existente con `gh pr checkout 16` y se incorporó `origin/main` por merge `e03ed65f8c5f246a2316242b92e03e8c5dfc107d`, sin conflictos ni force push. Los prerrequisitos de #29 son #22 y #28, ambos cerrados; PR #15 figura MERGED con SHA `0911b9c85643c8893950b381aec89b6249511706`.

El transcript sintético completo está en [auth-be-01-pc-20261005.log](auth-be-01-pc-20261005.log), incluyendo nombres/resultados de pruebas, hashes SHA-256 de generación y comprobaciones de limpieza. No contiene secretos ni DSN.

### Reproducibilidad sqlc

Go `1.27.1 linux/amd64`, dependencias de `go.mod` descargadas y sqlc `v1.31.1`, fijado por `generate.go`. Antes de modificar SQL se compararon los SHA-256 de los cuatro archivos `dbgen/*.go` antes/después de `go generate ./internal/adapters/postgres/identity`; `cmp` y `git diff --exit-code -- internal/adapters/postgres/identity/dbgen` devolvieron 0. Después de corregir la consulta se generó el código actualizado y se volvió a regenerar: los cuatro SHA-256 antes/después fueron idénticos (`cmp` exit 0). Los hashes iniciales y finales están en el transcript; el único cambio generado es el predicado de actividad de sesión.

### Defecto corregido y pruebas ejecutadas

`TouchSession` aceptaba fechas anteriores a la última actividad, retrocediendo la ventana idle ante operaciones fuera de orden. La prueba `TestTouchSessionDoesNotMoveActivityBackwards` falló contra la consulta anterior (`updated=true` para una actividad antigua) y pasó con el predicado `ultima_actividad_en <= activity_at`. Comprueba rechazo de actividad antigua y anterior a la creación, persistencia de la última actividad, conservación del vencimiento absoluto y una renovación válida dentro de la ventana idle preservada. No se cambió el esquema ni se implementaron casos de uso de #30/#31.

Todas las pruebas DB siguientes recibieron `TEST_DATABASE_URL` solo en el proceso, mediante socket Unix temporal; `DATABASE_URL` se retiró del entorno. La URI se omite de esta evidencia.

| Comprobación | Resultado real |
| --- | --- |
| `go test ./internal/adapters/postgres/identity -count=1 -v` | PASS: 8 pruebas de integración, 0 omitidas. |
| `go test ./internal/migrator -run '^(TestM01MigrationPersistsAuthorizedSchemaAndConstraints\|TestM01FKIntegrityRejectsUnknownUser)$' -count=1 -v` | PASS: 2 pruebas M01, 0 omitidas. |
| `go test -race -count=1 -v ./...` | PASS: 54 pruebas, 0 SKIP, 0 FAIL; incluye adaptador y las 8 pruebas DB del migrador, más unitarias. |
| `go vet ./...` | PASS, exit 0. |
| `git diff --check` | PASS, exit 0; repetido sobre la evidencia final antes del commit. |

No se ejecutaron navegador/mock, build TypeScript, harness OpenAPI dedicado ni proveedores externos: fuera de #29/#120. GitHub no reporta checks CI para PR #16; estos resultados corresponden a ejecución local, no a CI.

### Instancia y limpieza local verificadas

Imagen de prueba `postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677`, PostgreSQL `18.6`, PostGIS `3.6.4`, `btree_gist` `1.8`, verificados por consulta. Contenedor `espacigo-auth-be01-pc-20261005`, red `none`, sin puertos publicados, datos en `tmpfs` `/var/lib/postgresql`, único bind del socket. El directorio padre temporal tenía modo 0700. Autenticación trust solo en esta instancia sintética sin acceso TCP, sin credenciales persistentes.

El entrypoint falló en dos intentos iniciales porque su inicialización requería el socket `/var/run/postgresql`; ambos contenedores fallidos se retiraron. El intento correcto mantuvo los sockets `/var/run/postgresql` y `/test-socket`, sin escucha TCP. Un primer intento de la prueba encontró la instancia caída; no se contabilizó como prueba válida. La regresión se reprodujo posteriormente con PostgreSQL listo y las suites finales pasaron.

Al terminar: consulta `pg_database` confirmó cero bases `auth_be01_test_*`/`migrator_test_*`; `pg_stat_activity` confirmó cero sesiones de cliente adicionales. Se eliminó el contenedor, la imagen descargada y el directorio temporal completo (socket, hashes y logs auxiliares). `docker container inspect` y `docker image inspect` devolvieron exit 1 con `No such`; el directorio y socket no existen. No se crearon redes ni volúmenes persistentes. Las dependencias de Go permanecen en su caché normal para desarrollo; no son recursos temporales de PostgreSQL.

### Pendiente separado del servidor y revisión

Esta limpieza acredita exclusivamente los recursos creados en este PC. No hubo acceso ni operaciones sobre el servidor: el contenedor histórico `auth-be01-pg-20261004T225310`, su socket/directorio, worktrees y cualquier otro recurso remoto pendiente requieren inspección y evidencia independiente. El apartado histórico que reportó su retirada se conserva como antecedente; esta ejecución no lo corrobora ni da por cerrado ese pendiente.

GitHub Projects se leyó correctamente con HernanMEC: #29/#120 ya están `En revisión` y #30/#31 `Bloqueado`; se conservan esos estados, sin reimportar ni cambiar el tablero. Issues y Project son el registro operativo, sin nuevas transiciones en documentos históricos. DB02-09 permanece abierto; no se añadió DDL ni se resolvieron RQF-213/-217/-218. La aprobación y el merge del PR #16 quedan para HernanEspinozaDev.
