# GCP-04-W7 B1 — preparación local del grupo observer y preflight

**Estado:** candidato local para revisión. Sin acceso Cloud, credenciales, cambios de IAM/ACL remotos, publicación de imagen ni ejecución de Job.

## Alcance del candidato

La ruta B parte del actor B1 temporal `espacigo_gcp04_w7_bootstrap`. Este actor no es el observer: el contrato local lo modela como un login efímero `LOGIN CREATEROLE CREATEDB`, sin atributo PostgreSQL `SUPERUSER`, y con membresía heredada a `cloudsqlsuperuser`. Sigue siendo una identidad de bootstrap con capacidades elevadas; el usuario de lectura se separa en `espacigo_gcp04_w7_observer`.

El candidato [`gcp04_w7_b1_observer_group_prepare_candidate.sql`](sql_candidates/gcp04_w7_b1_observer_group_prepare_candidate.sql) falla cerrado salvo que la sesión sea directa al database `espacigo`, el actor y la membresía coincidan, `PUBLIC TEMP` ya esté desactivado y existan grant options efectivos para `CONNECT`, `USAGE` y —si está presente— `SELECT` del ledger. No cambia la ACL de `PUBLIC`, ownership, roles de aplicación ni datos.

SHA-256 del candidato SQL: `c37dfaa95667729407911a6c4c5dc55e13ce27066d021e43d2d8f3713b126982`.

Para crear el grupo `espacigo_db_observer`, usa un executor temporal `NOLOGIN CREATEROLE`, elimina ese executor en la misma transacción y deja el grupo sin membresías ni autoridad de administración. El grupo queda `NOLOGIN`, no heredable y sin atributos elevados. Sus permisos son `CONNECT` sobre `espacigo`, `USAGE` sobre `public` y, si existe, `SELECT` sobre `public.schema_migrations`; no recibe `CREATE`, `TEMP`, opciones de grant ni permisos sobre tablas de negocio. Cualquier colisión previa se rechaza, no se adopta.

La creación del login observer y su membresía quedan fuera del SQL candidato. La fixture crea el login sintético localmente con exactamente una membresía `ADMIN=false, INHERIT=true, SET=false` para ejecutar el preflight. Esa operación modelada no prueba que Cloud SQL `users.insert` asigne esas opciones.

## Validación local

`scripts/test-gcp04-w7-b1-observer-preflight-postgres.sh` configura un proyecto Compose único con PostgreSQL 18/PostGIS fijado por digest, `network_mode=none`, datos en tmpfs y socket Unix privado. Prepara un catálogo sintético, ejecuta el candidato como el actor B1 no superusuario, comprueba que no quede el executor ni una membresía al grupo y ejecuta la consulta W4 exacta mediante `cmd/gcp04preflight` como el login observer fixture.

Resultado: `TestLocalObserverReadOnlyIntegration` pasó; el W4 preflight aceptó el contrato del observer y mantuvo `transaction_read_only=true`. El destino sintético incluye una tabla ledger para comprobar que el grupo recibe su lectura; la tabla `usuario` no recibe permisos. El ensayo confirma los SQL/código en PostgreSQL local, no compatibilidad con Cloud SQL ni el comportamiento de la Admin API.

Regresión adicional ejecutada: `bash scripts/test-gcp04-w7-observer-executor-postgres.sh`, `bash scripts/test-gcp04-w7-b1-probe-postgres.sh`, `go test ./cmd/gcp04preflight ./internal/gcp04w7b1 ./cmd/gcp04w7b1 ./cmd/gcp04w7b1probe`, `bash -n scripts/test-gcp04-w7-b1-observer-preflight-postgres.sh` y `git diff --check`; todo pasó. El harness nuevo es ejecutable y usa la imagen fijada `postgis/postgis:18-3.6`, red desactivada y almacenamiento tmpfs. El contenedor/volumen del ensayo se retiró.

## Bloqueos y límites

- El B1 actor real, sus atributos, membresías, grant options/ownership y el estado de `PUBLIC TEMP` no se han comprobado en Cloud SQL. La consulta de probe existente es solo una propuesta local hasta una liberación aparte.
- La API de Cloud SQL no documenta los flags `ADMIN/INHERIT/SET` para `databaseRoles`. El login observer real no puede crearse/usar hasta que su membresía exacta esté comprobada por catálogo.
- `espacigo_db_observer` y sus ACL tendrían duración persistente si se adopta reutilización; la política de stewardship/retirada permanece pendiente. La fixture no decide si Cloud conservará o retirará el grupo.
- El harness puede representar `cloudsqlsuperuser` como una rol PostgreSQL sin `SUPERUSER` con grants sintéticos. No replica privilegios internos, identidad, IAM, red, secretos, cuotas ni tarifas de Cloud SQL.

No se cambian migraciones V1–V64 ni infraestructura. No se consultan ni crean secretos, no se publica imagen, no se inicializa state remoto, no se crea usuario Cloud SQL y no se ejecuta Cloud Run.
