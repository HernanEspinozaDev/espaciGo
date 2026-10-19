# GCP-04 `g04replay18`: matriz de estado y cierre técnico

## Actualización posterior: GCP-04-W7 gate observer local (2026-10-19)

| Capacidad | Estado | Evidencia / alcance |
| --- | --- | --- |
| W4/W7 PUBLIC preflight | Verificado localmente en PostgreSQL 18, sin Cloud | El harness W6 informa `CONNECT=true`, `CREATE=false`, `TEMPORARY=false` en la base y `USAGE=true`, `CREATE=false` en `public`, tanto antes como después del restore. W9 registró `TEMPORARY=true` antes del bootstrap; W7 adopta el estado más restrictivo que ya fijan `001/002`. W4 solo lee ACL. |
| Runner observador | Prueba local completa en ambas fases | `TestLocalObserverReadOnlyIntegration` pasó antes y después del restore. Falla cerrado para `PUBLIC TEMPORARY=true` y para cualquier miembro adicional del grupo observer; consulta read-only. No demuestra estado Cloud. |
| Canal de alta y actor | Sin canal compatible acreditado | PostgreSQL 18 deja ADMIN OPTION al actor `CREATEROLE` que crea un rol; una prueba aislada confirmó que la concesión permanece tras intentar retirarla desde el mismo actor. Cloud SQL `users.insert` requiere un rol custom existente para evitar `cloudsqlsuperuser`, pero el request documentado no expresa opciones `INHERIT/SET`. Ver [paquete de decisión W7](gcp04_w7_observer_channel_decision_20261019.md). |
| Cloud / avance | Bloqueado antes de mutaciones | No se creó login, grupo, secreto, IAM, Job ni state. Siguiente gate: decidir stewardship de membresías y contar con actor SQL cuya capacidad `CREATEROLE`/grant options se pruebe directamente, no desde `users.list`. |

**Corte local:** worktree `codex/infra/chore/gcp04-w7-observer-gate`, base `9f9e084b35f31d8710ca7665cd75362732e48c80`. Harness de dos PostgreSQL 18/PostGIS con red deshabilitada y `tmpfs`; contenedores/volúmenes retirados y verificado. Evidencia sintética privada `.local/testing/gcp04-w3-logical-baseline-w7-observer-sql-final2-20261019.json` (0600). No es aceptación de GCP-04 ni autorización de Cloud.

## Actualización posterior: GCP-04-W6 (2026-10-19)

| Capacidad | Estado | Evidencia / alcance |
| --- | --- | --- |
| Baseline lógico con bootstrap GADM | Validado en PostgreSQL 18 local efímero; Cloud pendiente | El harness integrado ejecuta `004_gadm_global_role.sql` con actor `NOSUPERUSER CREATEROLE` antes de los fixtures; `001`/`002` no crean ni comentan GADM. Restore como migrator con `SET ROLE espacigo_schema_owner`; V1–V64, datos, ACL/checksums, no-op y drift cubiertos. Prueba también rollback sin residuos, actor/settings inválidos y colisión del executor. |
| Preflight de destino | Ejecutable local, privado/read-only; remoto no ejecutado | W4 target preflight corre antes y después del restore como `espacigo_observer_login`; el reporte verifica identidad y `transaction_read_only`. La capacidad observada de fixture no representa Cloud SQL. |
| Identidad y costo remoto | Pendiente de revisión/liberación | Solo existe la referencia histórica a instancia privada candidata; no se verificaron secretos, sesión SQL, IAM ni Job en W6. Cloud SQL identity/CREATEROLE-compatible actor requiere resolver la diferencia con los atributos contractuales locales. No hay versión numérica de secreto seleccionada. Costeo regional exacto pendiente de calculador/facturación antes de autorizar. |

**Corte de lectura: 2026-10-18 21:46 UTC.** Esta matriz registra el smoke ejecutado; no declara GCP-04 completo ni aceptación del producto. El ensayo sigue activo y ningún teardown se aplicó.

## Actualización posterior: GCP-04-W5 (2026-10-19)

| Capacidad | Estado | Evidencia / alcance |
| --- | --- | --- |
| Bootstrap global GADM sin SUPERUSER | Candidato local verificado; destino Cloud pendiente | PostgreSQL 18 local: `CREATEROLE` directo puede crear/comentar, pero su ADMIN automático lo otorga `postgres` y no puede revocarlo. Un intermediario transaccional `NOLOGIN CREATEROLE` con SET restringido al actor sí crea/comenta y se elimina antes del commit; final sin membresías GADM, rerun no-op y casos inesperados fail-closed. No valida Cloud SQL ni cambia replay V60. Ver [hallazgo W5](gcp04_w5_global_role_bootstrap_finding_20261019.md) y `scripts/test-gcp04-w5-global-role-bootstrap-postgres.sh`. |

## Estado de capacidades

| Capacidad | Estado | Evidencia / alcance |
| --- | --- | --- |
| Código del smoke | Fusionado | PR #415, HEAD revisado `d52035b8db25a7643193d1414bface62d2ef1655`, merge `f93a3ab1114b46b1030b81317fbf39c477adfb8f`. Pool fijo a 4; gate de capacidad de solo lectura antes de crear la base sintética. |
| Cloud SQL de ensayo | Activo | `espacigo-gcp04-g04replay18-pg`, PostgreSQL 18, `RUNNABLE`, IP privada por la red/subred existentes, protección contra borrado activa. Backups/PITR apagados. Después de la prueba, la API lista únicamente la base `postgres`; el harness limpió su base sintética. |
| Secreto de runtime | Activo | Secret Manager versión numérica `1`, `ENABLED`; `UPDATE_USER` terminó `DONE`. El valor del secreto no se leyó ni se incluyó en Terraform/logs. |
| OCI y Artifact Registry | Publicado | Tag `g04replay18-pr415`, imagen `southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp04-replay-g04replay18/smoke@sha256:048f16d617096981b239ec02879fc6231855d4fc7d9b82cdc60bd9950bc210ea`. Upload del tar revisado, sin rebuild posterior. |
| Cloud Run Job | Activo y protegido | `espacigo-gcp04-g04replay18-smoke`, `Ready=True`, 1 tarea, paralelismo 1, 900 s, cero retries, 2 vCPU/4 GiB, protección de borrado verdadera. IAM: principal operador configurado; binding `run.invoker` exclusivo. |
| Ejecución smoke | Aprobada técnicamente | Única ejecución `espacigo-gcp04-g04replay18-smoke-xsvtn`, 2026-10-18 12:22:45–12:23:09 UTC; `Completed=True`, `EXECUTION_SUCCEEDED`, 1 tarea exitosa, 0 fallidas y 0 retries. Logs: `PASS`, `Container called exit(0).` No rerun. |
| Gate SQL | Pasó dentro del smoke | El código hace el SELECT de versión/primario, base/rol y capacidad antes de `newAuthHarness`; un fallo aborta antes de crear la base o aplicar migraciones. La ejecución `PASS` indica que el gate no falló. Los valores numéricos de `max_connections`, reservas y conexiones activas no quedaron registrados; no se inventan ni infieren. Demanda configurada: 3 pools × 4 conexiones + reserva operativa de 5 = 17 slots ordinarios. |
| Replay PostgreSQL/GCS | Validado para este escenario | La prueba existente `TestGCP04ReplayTwoProcessesPostgres18` usó un padre y dos procesos hijos contra la base y GCS reales, con barrera SQL/CAS. Las migraciones V1–V62 se verificaron en el harness. El snapshot de test usa `CREATE DATABASE ... TEMPLATE`; **no es** un backup ni restore administrado de Cloud SQL. |
| Costo facturado | No consultado | El informe no afirma saldo ni total facturado. Cloud SQL continúa provisionado; el cálculo de precios del plan es indicativo, no una lectura de facturación. |
| Workers durables / leases globales | Pendiente | No se desplegó API ni worker. No se probaron dos instancias de servicio, takeover/fencing global, scheduler, migrador administrado ni coordinación general de leases. |
| Aceptación GCP-04 / producto | Pendiente | El smoke acredita una ruta de replay concreta; no cierra la arquitectura ni acepta el marketplace. |

## Inventario actual y state

El state remoto exacto del root `infra/terraform/environments/testprod/gcp04-replay-smoke` es:

- URI: `gs://espacigo-testprod-tfstate-447373903153-gcp/espacigo/testprod/gcp04-replay-smoke/g04replay18/default.tfstate`
- Lineage: `a84a8366-a3f9-f432-7c3c-3e9bf5c005a6`; serial: `5`.
- SHA-256 del state: `7c3a5c1a73b21f7f7d4f486273f20af27ef70c4055c4a2f1a6c805cf4dac2c1f`.
- Generación del objeto state: `1792326100525834`.
- Recursos Terraform administrados: `11` (Job, IAM del Job, Cloud SQL, bucket, secreto, Artifact Registry, service account y sus permisos run-scoped).

El objeto `default.tfstate`, el bucket de state, la red/subred compartidas, Cloud SQL baseline, GCP-03, sus imágenes y sus datos no forman parte del inventario ni de los planes de teardown.

### Objetos GCS del run

El bucket `espacigo-testprod-g04replay18-447373903153` es regional, privado, UBLA, sin versionado y con `force_destroy=false`. El inventario de todas las versiones encontró:

| Estado | Objeto | Generación | Tamaño | Retención / verificación |
| --- | --- | ---: | ---: | --- |
| Vivo | `gcp04/g04replay18/privacy/privacy-replay-v1.json` | `1792326183393359` | 525 B | `STANDARD`, metageneración 1. Payload no leído. |
| Soft-deleted | `gcp04/g04replay18/privacy/privacy-replay-v1.json` | `1792326183015598` | 525 B | Hard delete informado `2026-10-25T12:23:03.129Z`. |
| Soft-deleted | `gcp04/g04replay18/privacy/privacy-replay-v1.json` | `1792326183118863` | 525 B | Hard delete informado `2026-10-25T12:23:03.405Z`. |
| Soft-deleted | `gcp04/g04replay18/evidence/f2040000-0000-4000-8000-000000000012.blob` | `1792326182554561` | 31 B | Hard delete informado `2026-10-25T12:23:03.576Z`. |

El prefijo `evidence/` no tiene objetos vivos. El bucket tiene soft-delete por `604800` segundos; no se leyeron payloads de objetos vivos o soft-deleted. El servicio conserva buckets y objetos borrados durante el periodo de soft-delete; borrar un bucket así no equivale a purgarlo inmediatamente ([soft delete](https://docs.cloud.google.com/storage/docs/use-soft-delete), [borrado de buckets](https://docs.cloud.google.com/storage/docs/deleting-buckets)).

La evidencia local del ensayo (OCI tar, release, planes, inventarios y logs) permanece bajo `.local/testing/gcp04-g04replay18/`, excluida de Git. El inventario no contiene payloads, contraseñas, tokens ni hashes de credenciales. Hashes principales:

| Archivo privado | SHA-256 |
| --- | --- |
| `job-final-verification.json` | `21e8ef569004cf2fa12c805967714fb7cf0ec6aaf52696adc018dc4d3fa143a9` |
| `current-inventory/inventory.json` | `0a5e147a1fcbb29697ee4d807167315c72d4910aed4c0cdad65a218c1a462734` |
| `current-inventory/gcs-full-inventory.json` | `22dfd92e3bb6ac4d36587e20b7406ebb68ae374f0ec6bd4e5a4c1c98a7a9815c` |
| `current-inventory/teardown-plan-review.json` | `242dd38e2da1170d9aef0eed42373810f2ef818a4e278458192d73c9e5494ccd` |

## Plan de retiro para revisión, no aplicado

Los dos binarios actuales están bajo `.local/testing/gcp04-g04replay18/current-inventory/`, con permisos `0600`. Ambos se generaron contra el state serial 5; **no aplicar el plan de destrucción** después de bajar protecciones. La etapa de reducción cambia el state, por lo que la siguiente destrucción requiere nueva lectura y nuevo plan.

| Plan | SHA-256 | Acciones | Estado |
| --- | --- | --- | --- |
| `deprotection.plan` | `4b50dc618f60dc7eac858977346503cf109c96c3471362c30711d59ea18d626c` | 6 actualizaciones en sitio, 5 no-ops, 0 altas, 0 bajas | No aplicado. Cambia `deletion_protection` SQL/Job/secreto y `deletion_policy` SQL/secreto/bucket/service account/Artifact Registry de `PREVENT` a `DELETE`. |
| `selective-destroy.plan` | `65ce38ea4db342a17a60c45cbad9bb79436aa951e644857850ae6623e61a9a4f` | 11 bajas | Vista de destroy sobre state serial 5; solo los 11 recursos run-scoped enumerados por Terraform. No es un plan para aplicar tras la etapa de protección. |

El bucket contiene una generación viva, y `force_destroy=false`: Terraform no vaciará objetos automáticamente. Antes de cualquier retiro hay que acordar y preservar privadamente las generaciones requeridas, verificar las copias sin imprimir contenidos, retirar solo generaciones exactas con liberación separada, volver a inventariar y generar un plan binario nuevo. Las generaciones soft-deleted tienen la retención indicada arriba. El state bucket y su objeto siempre quedan fuera.

Secuencia propuesta para una revisión futura:

1. Mantener el bucket y el Job disponibles mientras se revisan logs/OCI/inventario privados; el objetivo horario `2026-10-18T13:47:53Z` no fue autorización automática y ya venció.
2. Revisar y, solo con liberación concreta, aplicar el plan de reducción de protecciones de este run. Leer de vuelta cada protección cambiada.
3. Revisar la preservación por generación y liberar por separado cualquier retiro de objetos vivo o restore de versiones soft-deleted. No leer el secreto.
4. Releer SQL, Job, GCS, Artifact Registry, Secret Manager, IAM y state; crear un plan destroy nuevo y revisar SHA, 11 recursos, dependencias y estado del bucket.
5. Aplicar el nuevo plan de teardown solo tras liberación concreta. Mantener intactos state bucket, redes, GCP-03 y baselines. Registrar el state final vacío sin borrar el objeto de state.

## Siguiente corte dentro de GCP-04: propuesta GCP-04-W1

El siguiente corte de código propuesto es aislar el **worker de borrado de evidencia de privacidad**. `cmd/api/main.go` inicia `RunSuppressionCleanupWorkerWithStore` como goroutine cada 10 s. La tabla `baja_archivo_pendiente_local` (V25) almacena disponibilidad/intentos pero no un lease/claim fencing; `PendingSuppressionFiles` lee pendientes sin claim `SKIP LOCKED`, y `CompleteSuppressionFile` bloquea la fila pero no exige una generación de claim. Esto deja abierta la repetición de una eliminación externa entre procesos.

Entregables y aceptación propuestos:

1. Un ejecutable/servicio worker separado, inicialmente solo para `baja_archivo_pendiente_local`; no mover a la vez las familias completas de reconciliadores del API.
2. Claim SQL atómico con owner, vencimiento y generación monotónica de fencing; completion/retry condicionado al token vigente. El borrado GCS ocurre fuera de la transacción y es idempotente.
3. Añadir V63 únicamente tras revisión del esquema; conservar V1–V62 byte a byte y probar base vacía y upgrade V62→V63.
4. Pruebas PostgreSQL 18 con dos procesos: claims distintos, expiración/takeover, rechazo del completion de worker viejo, crash entre borrado externo y ack, fallo externo y reinicio convergente.
5. Antes de cualquier Cloud worker, presupuestar conexiones agregadas de API+worker+migrador con los valores configurados, definir roles/secretos por identidad y revisar plan run-scoped. Una ejecución de Job no valida dos réplicas de worker ni lease global.

GCP-04 continúa pendiente de este worker durable, de la coordinación global de leases/migraciones, del despliegue API y de las fases de disponibilidad/restore/costos/aceptación. No se llama GCP-05 a este trabajo ni se declara que ya esté implementado.

## Actualización posterior: retiro lógico de `g04replay18` y corte W1

**Actualizado: 2026-10-18 22:05:06 UTC.** Esta sección supersede el estado de recursos y planes de retiro descrito arriba, que corresponde al corte de lectura de las 21:46 UTC. No modifica el resultado histórico del smoke.

El revisor independiente confirmó el retiro selectivo del run: el state conserva lineage `a84a8366-a3f9-f432-7c3c-3e9bf5c005a6`, serial `9`, cero recursos administrados y SHA-256 `78ef07d3fde85defd63dd90dc1fd6fdfcae3b521c25edbdd8048b0ba64e47a86`. La evidencia privada de cierre tiene SHA-256 `15e7d2ae1cbd255e81fdba0254d547cd96827ece41aee541130151843884cd5b`. El plan aplicado informó `remove=11`, `add/change/import/action_fail=0`, once eventos `apply_complete` y cero diagnósticos. La evidencia independiente comprobó que las APIs de los recursos run-scoped responden 404 y que las tres instancias SQL preservadas (`GCP03origin`, `testprod`, `pilot`) permanecen `RUNNABLE`.

La salida de Terraform se conservó como JSONL y los checks de state/inventario quedaron registrados. El código de salida numérico del proceso no quedó capturado en la terminal, por lo que no se afirma ese valor ni se repitió un teardown para obtenerlo. Las generaciones GCS retiradas siguen bajo el soft-delete configurado; no se purgaron, restauraron ni se cambió la retención. No se tocó GCP-03, baseline, red compartida ni bucket/state remoto.

El retiro lógico ya no es un trabajo pendiente del smoke. La implementación local de GCP-04-W1 está en revisión en el corte de código que acompaña esta matriz; todavía no implica despliegue Cloud, workers globales, coordinación del migrador ni aceptación de GCP-04/producto. V63 se considera parte del diseño local de fencing de esta rama y debe conservar V1–V62 byte a byte.

### GCP-04-W1 — worker local de limpieza durable

Implementación y evidencia revisable: [PR #417](https://github.com/HernanEspinozaDev/espaciGo/pull/417), abierto hacia `main`, autor `HernanMEC`; revisión solicitada a `HernanEspinozaDev`. El PR no contiene deploy Cloud.

El corte separa la eliminación de archivos de `baja_archivo_pendiente_local` del proceso API. La API deja el trabajo en la cola; `privacy-cleanup-worker` reclama en lotes con `FOR UPDATE SKIP LOCKED`, lease, owner y generación monotónica. El ack y el backoff verifican el owner/generación vigentes y la vigencia del lease. La nueva V63 añade esos campos y el índice de pendientes reclamables; V1–V62 permanecen inmutables. El export periódico del registro replay sigue separado en API, sin reclamar la cola.

Cloud Storage lee la generación y envía `ifGenerationMatch` al borrar. Esto protege contra borrar una generación reemplazante entre lectura y borrado; el borrado externo sigue siendo al menos una vez y no se afirma exactamente una vez. Un borrado externo exitoso seguido de crash antes del ack se recupera por takeover; el borrado de una generación ya ausente es idempotente. V63 no activa una instancia o worker en Cloud.

Verificación del corte en PostgreSQL 18/PostGIS efímero: `internal/adapters/postgres/identity` completo pasó secuencialmente (incluye dos procesos de sistema operativo reclamando filas distintas y el caso de crash/takeover, con rechazo de ack y retry viejos); `internal/migrator` pasó después (base vacía V1–V63 y upgrade V62→V63 con fila pendiente y checksums previos preservados). El replay GCP-04 integrado con GCS falso pasó con dos procesos, reinicio, CAS/respuesta perdida, delete fallido y restore. `go test ./...`, Compose config y el build Docker del target worker también pasaron. La corrida usa infraestructura de ensayo local y no prueba IAM, Cloud SQL ni Cloud Run.

### Estado posterior W1 y preparación W2 — 2026-10-18

PR #417 fue aprobado por `HernanEspinozaDev` y fusionado a `main` como `e17a035c5ef593fb38f48ea41d7436d3dde79e1c`; el HEAD revisado fue `28f0bf4440b23a4819bbea003307d6088108614a`. Esto cierra W1, sin desplegar API/worker ni aceptar GCP-04.

W2 añade V64 para separar el rol del worker, una imagen de migrador distinta del runner histórico GCP-03 y una raíz Terraform opcional que referencia la instancia privada ya existente. El preflight Cloud fue de solo lectura: `espacigo-testprod-pg`, `RUNNABLE`, PostgreSQL 18, N4 2 vCPU/4 GiB, 20 GiB, IP privada, connector requerido, TLS cifrado, backups/PITR y protección de borrado; existe la DB `espacigo`. El API de Cloud SQL mostró solo el usuario integrado `postgres`. No se consultaron passwords, secretos ni catálogos SQL. No se crean usuarios ni se aplica V64 en Cloud.

La configuración limita el worker a una instancia manual y 2 conexiones; API queda en máximo 1 instancia, pool 4, con `espacigo_api_login` propuesto pero no creado. Para el presupuesto de rollout se asumen conservadoramente tres instancias por workload durante solapamiento: `3×4 + 3×2 + 1` migrador = 19; reserva de 5, total 24. El `max_connections` efectivo y la ocupación actual no se observaron. Antes de desplegar, medirlos con lectura SQL y continuar solo si quedan al menos 24 slots disponibles.

La raíz Terraform nueva consulta la red, subred y SQL existentes; no declara una instancia SQL. Worker Pool y Job están apagados por defecto; el Job es de una tarea y el corte no lo ejecuta. El plan local por defecto no requiere credenciales ni escribe state remoto. Antes de habilitar recursos hay que configurar un backend remoto aislado para esta raíz. La cuenta de worker, los logins SQL, secretos/versiones numéricas y ownership para migrar aún no existen o no se verificaron. Las otras familias de background de la API siguen en proceso; el despliegue API permanece bloqueado hasta revisar su coordinación global.

Costos: el worker pool manual factura con instancia activa aun ociosa. La tarifa exacta de `southamerica-west1`, descuentos/free tier, transferencias y duración de Job quedan para la revisión de costos antes de habilitarlo; no se incluye una Cloud SQL nueva. Google documenta Worker Pools para tareas continuas, sin URL ni autoscaling integrado y actualmente en Beta/Pre-GA ([documentación de despliegue](https://docs.cloud.google.com/run/docs/deploy-worker-pools), [precios](https://cloud.google.com/run/pricing)).

Validación local del W2 en curso: `go test ./...` pasó; PG18/PostGIS efímero pasó para `identity` completo y `migrator` completo. Las pruebas dirigidas de V64 verificaron fresh V1–V64, upgrade V63→V64 preservando fila pendiente/checksums, mínimos privilegios y rechazo del rol runtime; el ensayo de migrador en dos procesos confirmó DDL e historial únicos. `terraform validate` pasó y el plan por defecto de `gcp04-runtime` fue `No changes` con ambos recursos deshabilitados. La imagen se construyó localmente y se comprobó dentro del contenedor: 64 migraciones contiguas V1–V64. Ninguna prueba aplicó Cloud SQL, escribió en GCP o desplegó Cloud Run. El costo regional exacto, roles/secretos de la instancia y el rollout de API siguen pendientes.

### GCP-04-W7 — runner de preflight privado (2026-10-19)

Implementación/documentación revisable en el corte W7: [runner y procedimiento](gcp04_w7_private_preflight_runner_20261019.md). El binario Go ejecuta solo la consulta W4 de solo lectura y fija su SHA-256, target privado, base y login; requiere presupuesto positivo, versión numérica de secreto, deadline acotado y concurrencia 1. Verifica `RemoteAddr` contra la IP privada fijada antes de consultar, valida esquema completo, PostgreSQL 18, coherencia de capacidad y contrato observer de mínimo privilegio. Expone IP esperada/observada y separa `destination_candidate_observed` de `restore_approval=not_granted`. La raíz Terraform nueva compara la IP metadata actual además del connection name/región y propone Cloud Run Job de una tarea, service account e IAM mínimo condicionado, Direct VPC, imagen digest-pinned y montaje de la versión exacta. Todo queda apagado por defecto. `terraform validate` pasó, el plan default-off aislado dio `No changes`, y dos pruebas enabled con mock provider verificaron cinco recursos planificados y el rechazo de un cambio de IP sin APIs ni state remoto.

La metadata de Cloud SQL volvió a listar solo `postgres`; no se inspeccionaron catálogos ni grants SQL. Como Cloud SQL asigna `cloudsqlsuperuser` a un usuario builtin creado sin roles personalizados, no se propone crear una cuenta así. La Admin API admite `databaseRoles` si el grupo personalizado ya existe. La identidad observer remota sigue bloqueando la ejecución hasta verificar el grupo por vía autorizada. Regresión del harness local PG18/PostGIS, integración del preflight (observer válido, ACL excesiva DELETE/CREATE, propiedad por login/grupo, privilegio/membresía excesivos, reporte incompleto y capacidad incoherente) y build Docker local pasaron; no prueban compatibilidad Cloud SQL ni la IP remota. No hubo SQL remoto, creación de usuario/secreto/IAM/Job, publicación de imagen, state remoto ni apply. Ver precios/referencias y costos por ejecución en el documento W7.

### GCP-04-W9 — paquete operacional de liberación (2026-10-19)

La tabla refleja el estado luego de la fase E. Los detalles observados y los límites del alcance se registran en la actualización fechada al final de esta sección.

| Capacidad | Estado | Evidencia / alcance |
| --- | --- | --- |
| Reader metadata y CLI dry-run | Implementado localmente; dry-run y preflight operacionales reconciliados | El release aprobado fijó target, query hash y shell exactos. La lectura previa permitió reconciliar el shell preparado; un intento inicial con alias de número de proyecto se resolvió con lectura metadata-only y no se repitió una mutación incierta. |
| Alta exacta usuario/secreto y compensación | Ejecutada una vez y retirada | `users.insert` creó el login builtin con `databaseRoles=[pg_read_all_settings]`; operación `CREATE_USER` terminó `DONE`. Se creó la versión 1. Tras la corrida, `DELETE_USER` terminó `DONE`, la versión quedó `DESTROYED` con ETag y el shell se borró con ETag fresco. El login, shell y versión activa no permanecen. |
| Job e IAM | Aplicados desde plan revisado y retirados | Terraform aplicó cinco altas exactas. El Job usó el digest remoto fijado, tarea/paralelismo 1, retries 0 y 120 s. Tras la ejecución, el plan destroy verificado enumeró solo los cinco recursos propios; cinco bajas, cero cambios/altas. Job y service account responden ausentes, IAM temporal removido, state con cero recursos. |
| Prueba temporal `pg_read_all_settings` | Una ejecución real completada; no reutilizar esa identidad como observer W7 | El Job completó una corrida W8 de solo lectura (`query_sha256=3fc825934660bbfc965e8d97b2ef7f4648baf2cded6c8871ae6953bd18c70d16`). El reporte muestra `login` como miembro de `pg_read_all_settings` y, por separado, `pg_monitor` como miembro de `pg_read_all_settings`; no reporta al login como miembro de `pg_monitor`. El grant W9 permite leer settings y no satisface el contrato custom mínimo W7. |
| Artefacto contenedor | Publicado, fijado por digest y conservado | Artifact Registry confirmó `sha256:9139098697d8437715460bb1912141c3bfee141c6531d0c38a29132fadfd5991`. El artefacto permanece; los recursos temporales de ejecución no. El tar local previo conserva su SHA histórico `1525135e1088e19bd3cd04770c1a7088f380dcb76e41cb49e845368a82d9d4b9`. |
| Verificación | Pruebas locales y una corrida remota controlada | `go test -race` helper/CLI, Terraform fmt/validate/default plan `No changes`, Docker build/`--help`; además, una ejecución real observada por Cloud Logging y retirada verificada por API/Terraform. No demuestra compatibilidad del observer W7, restore, API, workers ni cierre de GCP-04. |

#### Actualización posterior: fase E, evidencia real y retirada (2026-10-19)

El release exacto `gcp04-w9-20261019-00155501c3` se ejecutó una sola vez. La versión del secreto se creó a las 08:06:15 UTC; la corrida dejó el reporte saneado a las 08:08:39 UTC y el shell fue eliminado a las 08:10:55 UTC (menos de cinco minutos de ventana activa, dentro del límite de 30 minutos). El query reportó base `espacigo`, PostgreSQL `180006` (18.6) y transacción `READ ONLY`. Cloud Logging confirmó el hash W8 fijado y `status=observed`.

La consulta reportó sin lectura de filas de negocio, catálogo de configuración, estadísticas, password/secreto ni mutaciones. La cuenta temporal tenía `LOGIN INHERIT`, sin `SUPERUSER`, `CREATEDB`, `CREATEROLE`, `REPLICATION` ni `BYPASSRLS`. La membresía reportada para esa identidad es `login → pg_read_all_settings`; el reporte también contiene la relación distinta `pg_monitor → pg_read_all_settings`. No se debe invertir esa relación ni inferir que el login heredó `pg_monitor`. El grant `pg_read_all_settings` por sí solo permite leer settings y no satisface el contrato custom mínimo W7, por lo que la identidad W9 no se reutiliza. El reporte también observó privilegios de `PUBLIC` sobre `espacigo` (`CONNECT`, `TEMPORARY`) y el schema `public` (`USAGE`), sin grant option; no fueron grants directos al login ni se cambiaron. Deben permanecer visibles y evaluarse explícitamente en el contrato W7.

La lista allowlisted de roles del reporte no incluyó `espacigo_db_observer` ni roles del ledger; esto no sustituye un inventario exhaustivo ni confirma su inexistencia fuera de ese alcance. El corte W7 siguiente debe verificar el observer con canal autorizado de mínimo privilegio, validar membresías directas/transitivas y ACL efectivas (incluido `PUBLIC`), y ejecutar el runner fijado solo tras revisar identidad, secret version, digest, IP, capacidad y plan exactos. `restore_approval` permanece `not_granted`; no comenzar restore ni despliegue API basándose únicamente en W9.

Entregables concretos para ese gate:

1. Verificar que el login W7 y `espacigo_db_observer` existen y que el cierre efectivo de membresías coincide exactamente con el contrato W7: solo el grupo observer aprobado, sin roles builtin no autorizados como `pg_read_all_settings` o `pg_monitor`, ni membresías administrativas. Si falta el grupo o el canal no permite comprobarlo, detenerse y diseñar una operación separada de rol mínimo; no reutilizar el login W9.
2. Revisar con el runner W7 ACL directas, efectivas y por defecto para base/schema/ledger, ownership y grants `PUBLIC`; aceptar únicamente el contrato W7 documentado o bloquear con evidencia concreta. Conservar la observación W9 como baseline, no cambiar grants durante el preflight.
3. Antes de cualquier restore o despliegue API, emitir una liberación W7 nueva que fije versión numérica de secreto sin leer payload, digest remoto, IP privada esperada, presupuesto y hash SQL; inspeccionar el plan run-scoped exacto y ejecutar como máximo la tarea W7 autorizada. El resultado `passed` solo habilita una decisión de restore separada; `restore_approval` sigue en `not_granted` hasta entonces.

Terraform aplicó cinco recursos temporales y retiró exactamente esos cinco. El state run-scoped conserva su lineage, serial `4` y cero recursos; el usuario SQL y el shell responden ausentes, la versión 1 está `DESTROYED`, el Job y service account están ausentes, y los bindings temporales fueron retirados. El binding preexistente `roles/run.serviceAgent`, la instancia/base, VPC/subred, repositorio/imagen, bucket y objeto de state se conservaron. El detalle de ejecución ya no era describible después de retirar el Job; el éxito queda evidenciado por la respuesta `--wait`, el listado de una ejecución antes de la retirada y el reporte de Cloud Logging.

El resumen JSON versionado está en [evidencia W9](evidence/gcp04-w9-operational-experiment-20261019.json). Los logs/API brutos, aprobaciones con ETag, journal y state permanecen en `.local/testing/gcp04-w9/`, excluidos de Git y con permisos privados; no se versionan payload, ETag, credencial ni respuesta no saneada.
