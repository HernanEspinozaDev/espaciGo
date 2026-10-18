# GCP-04 `g04replay18`: matriz de estado y cierre técnico

**Corte de lectura: 2026-10-18 21:46 UTC.** Esta matriz registra el smoke ejecutado; no declara GCP-04 completo ni aceptación del producto. El ensayo sigue activo y ningún teardown se aplicó.

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
