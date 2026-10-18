# GCP-04: propuesta de smoke real de replay en Cloud

**Estado al 2026-10-18: PR #413 fusionado; PR de seguimiento de preparación local en desarrollo. La base GCP de `g04replay18` se aplicó desde el plan binario autorizado: 9 altas, 0 cambios y 0 bajas.** Cloud SQL está RUNNABLE y protegido contra borrado; el bucket privado, secreto vacío, identidad, IAM y repositorio existen. No se publicó imagen, no se creó ni ejecutó Job, no se creó una versión de secreto y no se cambió la contraseña SQL. Esta propuesta no es una liberación operativa. El Cloud SQL persistente `espacigo-testprod-pg`, el ensayo conservado `g03o-1017`, sus evidencias y el state de GCP-03 quedan fuera.

## Objetivo y alcance

El paso siguiente al harness local es comprobar, en recursos desechables, la integración del adaptador de replay con PostgreSQL 18 mediante el Cloud SQL Go Connector y con un bucket GCS real. Se propone un solo Cloud Run Job privado, de una tarea y una ejecución manual. No desplegar API, scheduler ni workers en este corte. Por ello este smoke no demuestra concurrencia entre revisiones de API, liderazgo/fencing de workers, recuperación de backup administrado ni aceptación de producto.

El binario ejecuta la prueba existente `TestGCP04ReplayTwoProcessesPostgres18`. El padre y dos procesos hijos independientes prueban el replay contra la misma base y bucket. Una barrera SQL obliga a ambos a leer la revisión inicial antes del CAS; las dos escrituras convergen y el manifiesto final conserva ambas entradas. El bucket real cubre el CAS de generación y la eliminación exitosa del objeto sintético. El modo local conserva cobertura adicional de respuesta PUT perdida, CAS HTTP 412 y DELETE remoto fallido; esos errores no se inyectan en Cloud. No se suben datos de usuarios ni artefactos de los ensayos anteriores.

Cada invocación usa prefijos exclusivos `gcp04/<run-id>/privacy` y `gcp04/<run-id>/evidence`. La restauración retrocede una base sintética mediante `CREATE DATABASE ... TEMPLATE` dentro de la instancia desechable; esto no es una restauración de backup de Cloud SQL. El smoke aplica las migraciones runtime V1–V62 en esa base. No modifica archivos SQL ni el historial de migraciones.

## Recursos propuestos y aislamiento

Región `southamerica-west1`, proyecto existente `espacigo-testprod`, red/subred existentes `espacigo-testprod` y `espacigo-testprod-serverless`. El preflight de solo lectura del 2026-10-18 confirmó APIs requeridas ya habilitadas, proyecto activo y red/subred esperadas. Los nombres candidatos estaban ausentes en esa consulta; la disponibilidad de un nombre global no queda reservada por el inventario. Los límites de cuota se leyeron donde estaban disponibles, sin datos de uso para los límites consultados.

El run candidato es `g04replay18`. El preflight verificó que los nombres siguientes estaban ausentes el 2026-10-18; no es una reserva ni autorización de crear recursos:

| Recurso | Nombre candidato |
| --- | --- |
| Cloud SQL PostgreSQL 18 | `espacigo-gcp04-g04replay18-pg` |
| Bucket privado | `espacigo-testprod-g04replay18-447373903153` |
| Service account runtime | `espacigo-gcp04-g04replay18@espacigo-testprod.iam.gserviceaccount.com` |
| Secret Manager (solo metadata Terraform) | `espacigo-gcp04-g04replay18-db-owner-password` |
| Artifact Registry | `gcp04-replay-g04replay18` |
| Cloud Run Job | `espacigo-gcp04-g04replay18-smoke` |

El root Terraform está en `infra/terraform/environments/testprod/gcp04-replay-smoke`; usa estado GCS independiente bajo `espacigo/testprod/gcp04-replay-smoke/<run-id>`. Para este run, el objeto `gs://espacigo-testprod-tfstate-447373903153-gcp/espacigo/testprod/gcp04-replay-smoke/g04replay18/default.tfstate` quedó inicializado, sin recursos, con lineage `a84a8366-a3f9-f432-7c3c-3e9bf5c005a6` y serial 1. El plan binario local de base tiene SHA-256 `6197cd0f7537052d3576b54701ca571cbe0b30d65a71e9e961f4d7b260eb955f`, propone 9 altas/0 cambios/0 bajas y tiene `enable_job=false`. Los artefactos sensibles siguen en `.local/testing/gcp04-g04replay18/`, excluidos de Git y protegidos con directorio 0700/archivos 0600. No se reutiliza ni importa state anterior.

La base tendría una instancia zonal N4 de 2 vCPU/4 GiB, disco Hyperdisk Balanced fijo de 20 GiB sin auto-crecimiento, IP pública deshabilitada, conector obligatorio, salida por la VPC existente, backups/PITR apagados y protección contra borrado activa. El bucket es Standard regional, sin acceso público, UBLA, sin versionado ni `force_destroy`. El Job usa exactamente una tarea y paralelismo 1, hasta 900 segundos, 2 vCPU/4 GiB y cero retries. La imagen debe fijarse por digest SHA-256; el digest remoto no existe todavía.

Terraform no habilita APIs ni crea proyectos, VPC, subredes, balanceador o servicios de API. Identidad runtime dedicada: `roles/cloudsql.client` condicionado a la instancia del run, `roles/storage.objectAdmin` en el bucket del run y `roles/secretmanager.secretAccessor` sobre el secreto del run. El agente de Cloud Run recibe lector solo en el repositorio del run. Un único principal operador definido en el plan recibe `roles/run.invoker` sobre ese Job. Terraform administra el contenedor del secreto y referencia una versión numérica; nunca recibe el valor.

Cada pool del runner fija ahora `MaxConns=4` explícitamente: `pgxpool` por defecto usa `max(4, runtime.NumCPU())`, y el límite CPU de Cloud Run no demuestra el `NumCPU` visible dentro del contenedor. El padre y los dos procesos hijos pueden sostener 3 pools × 4 = 12 conexiones; la fase de restauración también alcanza 3 pools × 4. Se reservan 5 conexiones operativas, para un presupuesto agregado de 17. El default publicado de Cloud SQL para 4 GiB es 100, pero el valor real debe leerse con `pg_settings` en la instancia creada, antes de cualquier fase Job.

## Etapas con liberación separada

1. **Preparación local (PR de seguimiento):** fijar máximo de conexiones por pool y probar el cálculo; implementar preview y helper GCP-04 bloqueado por un release exacto, con state/plan verificados, journal privado y pruebas con clientes falsos. El helper no se ejecuta en esta etapa. La OCI local anterior permanece como evidencia, pero fue construida antes del cambio del límite y no se propone como imagen del Job.
2. **Preflight de solo lectura:** inventario de nombres/red/APIs/cuotas completado. La página de precios se consultó con la región Santiago seleccionada. El usuario confirmó que Free Trial sigue activo; no se activó Paid ni se modificaron billing o cuotas.
3. **Base aislada:** se aplicó una sola vez el plan binario autorizado, SHA-256 `6197cd0f7537052d3576b54701ca571cbe0b30d65a71e9e961f4d7b260eb955f`: 9 altas, 0 cambios, 0 bajas y `enable_job=false`. El state remoto mantiene lineage `a84a8366-a3f9-f432-7c3c-3e9bf5c005a6`, serial 4, generación `1792324268479574` y SHA-256 `9a3d935ca601353b72df5302fa5c0f0903f47acf291ab62e1b0bd7dd5620eb2e`. Cloud SQL `espacigo-gcp04-g04replay18-pg` quedó RUNNABLE con creación `2026-10-18T11:47:53.447Z`; la evidencia independiente de recursos/IAM se guarda localmente en `.local/testing/gcp04-g04replay18/apply-readback.json` (0600, SHA-256 `d8270b1cb8e56dab6a13d4cb584ff014f69396f493891cf375eb09b92f8a9981`); el objetivo operativo de dos horas vence `2026-10-18T13:47:53Z` (10:47:53 hora Santiago). Es una meta de operación, no límite de facturación ni permiso de borrado automático. No crear ni invocar el Job todavía.
4. **Runner:** generar credencial sintética solo tras una liberación exacta con proyecto/run/SQL/secreto/state/plan; revisar primero el release y el preview. El secreto SQL y la versión Secret Manager son cambios no atómicos: journal privado, detenerse ante resultado parcial o incierto, sin retry, compensación o rotación tácita. Construir una nueva imagen compatible con el límite revisado y publicar solo tras liberación separada; la OCI previa/hash no se sustituye. Aplicar el Job no lo ejecuta. Invocarlo requiere liberación separada para el Job y digest concretos.
5. **Cierre:** preservar evidencia y revisar resultado/contadores. El teardown requiere plan aparte, segunda lectura del state y liberación concreta; primero Job/IAM del Job y después base. Ningún Job se autoinvoca ni Terraform ejecuta Jobs.

La inicialización del backend creó únicamente el objeto remoto vacío del state autorizado; el plan guardado es una lectura de Cloud y no creó infraestructura. El archivo `.terraform.lock.hcl` fija providers y los tests usan provider mock; esos planes de test no equivalen a un plan Cloud.

## Costo, límite y retención

Estimación indicativa en USD con tablas oficiales observadas el 2026-10-18 después de seleccionar `Santiago (southamerica-west1)`: Cloud SQL Enterprise N4 CPU USD 0,05782/vCPU-h, memoria USD 0,0098/GiB-h y Hyperdisk Balanced capacidad USD 0,000220548/GiB-h. Para 2 vCPU, 4 GiB y 20 GiB, SQL suma `2×0,05782 + 4×0,0098 + 20×0,000220548 = USD 0,15925096/h`; dos horas operativas equivalen a USD 0,31850192 de esos componentes. Cloud Run Jobs en Santiago (Tier 2) usa USD 0,0000216/vCPU-s y USD 0,0000024/GiB-s; una tarea 2 vCPU/4 GiB de 900 s suma USD 0,04752 bruto. SQL durante 2h más una ejecución completa: USD 0,36602192 antes de bucket/operaciones, Artifact Registry, Secret Manager, VPC/red, transferencia, logs, retención, impuestos y cualquier crédito/free tier. No es cotización. Dos horas es meta operativa, no límite de facturación ni expiración: SQL factura hasta que se detiene/retira.

El estado de una prueba gratuita, crédito restante y cuenta de facturación no se deduce de esos precios; deben verificarse por separado. El usuario confirmó Free Trial activo. No se consultó saldo ni se alteró billing/cuotas. Fuentes: [Cloud SQL pricing](https://cloud.google.com/sql/pricing?hl=en), [Cloud Run pricing](https://cloud.google.com/run/pricing?hl=en), [Cloud Storage pricing](https://cloud.google.com/storage/pricing), [Cloud SQL PostgreSQL flags](https://docs.cloud.google.com/sql/docs/postgres/flags).

Retener solo los objetos sintéticos de este run hasta 24 horas después del resultado, para revisión humana. No hay borrado automático: conservar si la revisión lo requiere. Luego preparar un teardown exacto separado, revisar que no queden objetos ni dependencias, preservar el informe/hashes acordados y liberar el retiro por separado. El bucket con `force_destroy=false` fallará cerrado si aún contiene objetos. La instancia factura mientras siga aprovisionada. Para este run, la creación real fue `2026-10-18T11:47:53.447Z`; el objetivo operativo vence `2026-10-18T13:47:53Z`, sin borrado automático.

## Evidencia local y límites actuales

- El plan privado de base: 9 altas, 0 cambios/bajas, `enable_job=false`, protecciones activas. Su SHA y state final se registran arriba; la aplicación completó correctamente.
- Preflight del 2026-10-18: nombres candidatos ausentes, APIs ya habilitadas, VPC/subred requeridas visibles; cuotas son límites efectivos, no medidas de uso. Las instancias y Jobs GCP-03 existentes se preservan.
- `go test` y `go test -race` en `./internal/adapters/postgres/identity ./internal/gcp04credential ./cmd/gcp04replay-credential`: pasan en este HEAD de preparación. `git diff --check` no reporta errores.
- El helper preview no crea contraseñas y no hace llamadas Cloud. La ejecución requiere archivo release privado 0600, hash exacto por CLI y confirmación expresa; su código nunca se ejecutó.
- La OCI local anterior está en el directorio privado del run: digest `sha256:66ce772c2e576d69ee693cd3c4d4167959fcf2addf9543f4cdf3393b2fbcedc8`, tar SHA-256 `a7efed5b50f627507e0b7bf08b5ac9fc9bd5b1d7be19122d8a1dbe123280c143`. Se conserva como evidencia de la fuente previa; no fue publicada y no se sustituye su hash.
- La base sí se creó en GCP y su metadata fue leída independientemente; aún no hay imagen remota, Job/Cloud Run ejecutado, costo facturado calculado ni datos restaurados desde backup. La aplicación no prueba fallos reales de GCS ni el replay end-to-end.
- No prueba dos instancias Cloud Run ni el despliegue del API; no prueba leases/fencing de workers ni el migrador de producción en un servicio administrado. GCP-04 permanece pendiente de los gates anteriores y de revisión/aceptación.
