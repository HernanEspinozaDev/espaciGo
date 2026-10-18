# GCP-04: propuesta de smoke real de replay en Cloud

**Estado al 2026-10-18: propuesta técnica y código local en revisión; ningún recurso de este corte está creado, ninguna imagen fue publicada y ningún Job fue ejecutado.** Esta propuesta no es una liberación operativa. El Cloud SQL persistente `espacigo-testprod-pg`, el ensayo conservado `g03o-1017`, sus evidencias y el state de GCP-03 quedan fuera.

## Objetivo y alcance

El paso siguiente al harness local es comprobar, en recursos desechables, la integración del adaptador de replay con PostgreSQL 18 mediante el Cloud SQL Go Connector y con un bucket GCS real. Se propone un solo Cloud Run Job privado, de una tarea y una ejecución manual. No desplegar API, scheduler ni workers en este corte. Por ello este smoke no demuestra concurrencia entre revisiones de API, liderazgo/fencing de workers, recuperación de backup administrado ni aceptación de producto.

El binario ejecuta la prueba existente `TestGCP04ReplayTwoProcessesPostgres18`. El padre y dos procesos hijos independientes prueban el replay contra la misma base y bucket. Una barrera SQL obliga a ambos a leer la revisión inicial antes del CAS; las dos escrituras convergen y el manifiesto final conserva ambas entradas. El bucket real cubre el CAS de generación y la eliminación exitosa del objeto sintético. El modo local conserva cobertura adicional de respuesta PUT perdida, CAS HTTP 412 y DELETE remoto fallido; esos errores no se inyectan en Cloud. No se suben datos de usuarios ni artefactos de los ensayos anteriores.

Cada invocación usa prefijos exclusivos `gcp04/<run-id>/privacy` y `gcp04/<run-id>/evidence`. La restauración retrocede una base sintética mediante `CREATE DATABASE ... TEMPLATE` dentro de la instancia desechable; esto no es una restauración de backup de Cloud SQL. El smoke aplica las migraciones runtime V1–V62 en esa base. No modifica archivos SQL ni el historial de migraciones.

## Recursos propuestos y aislamiento

Región `southamerica-west1`, proyecto existente `espacigo-testprod`, red/subred existentes `espacigo-testprod` y `espacigo-testprod-serverless`. Esos identificadores, el estado de APIs, las cuotas, la disponibilidad global del bucket y los nombres propuestos todavía requieren inventario de solo lectura antes de cualquier plan habilitado.

El run sugerido es `g04replay18`, aún no reservado. Nombres candidatos:

| Recurso | Nombre candidato |
| --- | --- |
| Cloud SQL PostgreSQL 18 | `espacigo-gcp04-g04replay18-pg` |
| Bucket privado | `espacigo-testprod-g04replay18-447373903153` |
| Service account runtime | `espacigo-gcp04-g04replay18@espacigo-testprod.iam.gserviceaccount.com` |
| Secret Manager (solo metadata Terraform) | `espacigo-gcp04-g04replay18-db-owner-password` |
| Artifact Registry | `gcp04-replay-g04replay18` |
| Cloud Run Job | `espacigo-gcp04-g04replay18-smoke` |

Los nombres son candidatos y no prueban disponibilidad. El root Terraform está en `infra/terraform/environments/testprod/gcp04-replay-smoke`; usa estado GCS independiente bajo `espacigo/testprod/gcp04-replay-smoke/<run-id>`. Todavía no existe backend inicializado, lineage, serial, plan guardado ni plan remoto: inicializar el backend también cambia Cloud y requiere gate/revisión propios. No se reutiliza ni importa state anterior.

La base tendría una instancia zonal N4 de 2 vCPU/4 GiB, disco Hyperdisk Balanced fijo de 20 GiB sin auto-crecimiento, IP pública deshabilitada, conector obligatorio, salida por la VPC existente, backups/PITR apagados y protección contra borrado activa. El bucket es Standard regional, sin acceso público, UBLA, sin versionado ni `force_destroy`. El Job usa exactamente una tarea y paralelismo 1, hasta 900 segundos, 2 vCPU/4 GiB y cero retries. La imagen debe fijarse por digest SHA-256; el digest remoto no existe todavía.

Terraform no habilita APIs ni crea proyectos, VPC, subredes, balanceador o servicios de API. Identidad runtime dedicada: `roles/cloudsql.client` condicionado a la instancia del run, `roles/storage.objectAdmin` en el bucket del run y `roles/secretmanager.secretAccessor` sobre el secreto del run. El agente de Cloud Run recibe lector solo en el repositorio del run. Un único principal operador definido en el plan recibe `roles/run.invoker` sobre ese Job. La contraseña de `postgres` se carga privadamente como versión numérica de Secret Manager, nunca como valor de variable, state o log; Terraform administra solo metadata/IAM y referencia esa versión.

## Etapas con liberación separada

1. **Preparación local (este PR):** revisión de código, pruebas con mocks, formatter/validate y ejecución del harness en Postgres desechable. No hay acceso de escritura Cloud.
2. **Preflight de solo lectura:** comprobar nombres, red, cuotas, billing/trial y precios vigentes. Si un inventario no coincide o falta la disponibilidad de un nombre, detenerse y revisar el diseño.
3. **Base aislada:** después de revisar un plan exacto, inicializar su backend independiente y crear solo SQL, bucket privado, secreto vacío, identidad y repositorio. No crear ni invocar el Job todavía. Cada operación de state/plan/aplicación debe estar dentro de su gate explícito.
4. **Runner:** generar credencial sintética por canal privado, construir y publicar la imagen solo tras liberación de esa acción; verificar de forma independiente el digest de Artifact Registry y revisar el plan del Job. Aplicar el Job no lo ejecuta. Invocarlo requiere una liberación separada para el Job y el digest concretos.
5. **Cierre:** preservar evidencia y revisar resultado/contadores. El teardown requiere plan aparte, segunda lectura del state y liberación concreta; primero Job/IAM del Job y después base. Ningún Job se autoinvoca ni Terraform ejecuta Jobs.

El pipeline local no crea artefactos Terraform remotos. La configuración de backend aparece en `versions.tf`, pero no se ha inicializado. El archivo `.terraform.lock.hcl` fija providers y los tests usan provider mock; esos planes de test no equivalen a un plan Cloud.

## Costo, límite y retención

Estimación orientativa antes de impuestos, red, operaciones, logs, cuotas y diferencias regionales, con precios públicos consultados el 2026-10-18: Cloud SQL N4 a USD 0,0413/vCPU-h y USD 0,007/GiB-h más Hyperdisk Balanced a USD 0,000157534/GiB-h: `2×0,0413 + 4×0,007 + 20×0,000157534 = USD 0,11375068/h`. Un techo operativo propuesto de 2 horas para SQL representa aproximadamente USD 0,23. Cloud Run Jobs a USD 0,000018/vCPU-s y USD 0,000002/GiB-s da hasta USD 0,0396 para una tarea de 2 vCPU/4 GiB por 900 s. El subtotal aritmético aproximado sería USD 0,27, antes de bucket/operaciones, VPC, transferencia, logs, impuestos y cualquier permanencia sobre el tope. No es cotización ni límite de facturación: recalcular con Pricing Calculator y región exacta antes de revisar un plan.

El estado de una prueba gratuita, crédito restante y cuenta de facturación no se deduce de esos precios; deben verificarse por separado. Google indica que la prueba gratuita ofrece USD 300 por 90 días y que los recursos se detienen al expirar el periodo/crédito, pero esa descripción no prueba que la cuenta actual tenga saldo o cobertura vigente. Fuentes: [Cloud SQL pricing](https://cloud.google.com/sql/pricing?hl=en), [Cloud Run pricing](https://cloud.google.com/run/pricing), [Cloud Storage pricing](https://cloud.google.com/storage/pricing), [Free Cloud features](https://docs.cloud.google.com/free/docs/free-cloud-features).

Retener solo los objetos sintéticos de este run hasta 24 horas después del resultado, para revisión humana. No hay borrado automático: conservar si la revisión lo requiere. Luego preparar un teardown exacto separado, revisar que no queden objetos ni dependencias, preservar el informe/hashes acordados y liberar el retiro por separado. El bucket con `force_destroy=false` fallará cerrado si aún contiene objetos. La instancia factura mientras siga aprovisionada; registrar propietario y hora límite en la evidencia de la operación antes de crearla.

## Evidencia local y límites actuales

- `terraform validate` y `terraform test`: 3 escenarios de arquitectura pasan; provider mock, sin llamadas Cloud.
- `bash scripts/test-gcp04-replay-postgres.sh`: prueba integrada local ejecutada en un contenedor PostgreSQL 18 desechable; las migraciones usan su historial runtime.
- `go test`/compilación de paquetes relevantes se registrará con HEAD en el PR.
- Sin GCS real, Cloud SQL real, IAM real, imagen remota, Cloud Run, costo facturado o datos restaurados desde backup. Los fallos reales de GCS no se inyectan; las pruebas locales del emulador no acreditan comportamiento de IAM/servicio.
- No prueba dos instancias Cloud Run ni el despliegue del API; no prueba leases/fencing de workers ni el migrador de producción en un servicio administrado. GCP-04 permanece pendiente de los gates anteriores y de revisión/aceptación.
