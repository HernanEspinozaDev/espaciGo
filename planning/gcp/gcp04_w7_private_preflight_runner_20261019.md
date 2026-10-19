# GCP-04-W7 — runner privado de preflight SQL

**Estado:** implementación local y plan de infraestructura OFF revisables; ejecución contra Cloud SQL bloqueada hasta acreditar un login PostgreSQL dedicado de mínimo privilegio. No es aceptación de GCP-04 ni autorización de despliegue.

## Alcance implementado

`cmd/gcp04preflight` ejecuta únicamente `db/query/gcp04_w4_target_preflight.sql`, con transacción PostgreSQL `READ ONLY`, presupuesto de conexiones explícito y salida JSON saneada. Verifica en código el SHA-256 del archivo fuente (`3e9783799360c4438cdb7399ec78fce6aaf8ee62d404cf70e3dfe90da6f2b869`), extrae el cuerpo SQL del wrapper de `psql`, reemplaza sus tres parámetros psql por un parámetro `$1` de `pgx` y rechaza cambios de hash o de forma. La identidad del proyecto, connection name, IP privada, base y login está fijada en el binario. El login candidato es `espacigo_gcp04_w7_observer`; su existencia y grants en Cloud SQL no están comprobados. La consulta requiere `GCP04_PREFLIGHT_SECRET_VERSION` numérico, archivo montado y protegido, deadline de máximo 15 minutos y concurrencia exactamente 1. La configuración Terraform limita el tiempo a 900 segundos, fija por defecto 120, y usa un solo task/paralelismo 1/retries 0.

El proceso devuelve `passed` con código 0, `gate_failed` con código 2 cuando el presupuesto conservador no pasa y `failed` con código 1 ante error. Errores operativos se sanitizan; nunca se imprime la contraseña ni detalles de conexión. El digest de imagen remoto queda sujeto al Artifact Registry aprobado; el scaffold exige la ruta W7 y digest inmutable.

## Metadata observada y límite de identidad

La lectura de metadata de Cloud SQL confirmó proyecto `espacigo-testprod`, instancia `espacigo-testprod-pg`, región `southamerica-west1`, PostgreSQL 18, estado `RUNNABLE`, conexión `espacigo-testprod:southamerica-west1:espacigo-testprod-pg`, IP privada `10.189.0.3` y bases `postgres` y `espacigo`. La lista de usuarios devolvió solo `postgres`. No se observó catálogo SQL, roles/grants, contraseña, ni lectura de payloads de secretos. No hubo conexión SQL remota.

La documentación oficial establece que un usuario builtin creado por Cloud SQL sin `databaseRoles` personalizados recibe `cloudsqlsuperuser`; esa cuenta no sirve como observer. Cloud SQL Admin API permite `users.insert` con `databaseRoles[]`, pero la documentación requiere que el rol de base exista previamente. La existencia de `espacigo_db_observer` solo aparece en fixture local, no en metadata. Por eso no se crea el login ni se configura contraseña. No resetear `postgres`, no usar SUPERUSER/impersonación, no crear roles por SQL remoto y no leer secretos. La creación segura requiere verificar primero el grupo de solo lectura con una identidad ya autorizada; entonces revisar una operación Admin API explícita para crear únicamente el login con `databaseRoles=["espacigo_db_observer"]`, sin omitir roles y sin asignar `cloudsqlsuperuser`. Si el grupo no existe, detenerse para una decisión y secuencia separada revisada.

Referencias: [usuarios Cloud SQL](https://docs.cloud.google.com/sql/docs/postgres/users), [crear/administrar usuarios y roles](https://docs.cloud.google.com/sql/docs/postgres/create-manage-users), [recurso Admin API users](https://docs.cloud.google.com/sql/docs/postgres/admin-api/rest/v1/users).

## Infraestructura propuesta, apagada

La raíz nueva `infra/terraform/environments/testprod/gcp04-w7-preflight` usa backend GCS declarado pero no inicializado. `enable_preflight=false` y `confirm_preflight_release=false`; ninguna identidad, IAM binding, Job, secreto o state remoto fue creado. Al habilitarlo en una liberación posterior, la raíz consulta la instancia/red/subred/repositorio existentes, crea una service account dedicada, condiciona `roles/cloudsql.client` a la instancia, concede `secretAccessor` solo al secreto W7, concede lectura Artifact Registry al service agent, y declara un Cloud Run Job privado con Direct VPC egress por red/subred existentes, archivo de versión numérica fijada, una tarea y 1 vCPU/512 MiB. No crea ni modifica Cloud SQL, base, red, secreto o bucket de state.

Terraform `validate` pasó. La prueba de plan OFF se hizo en una copia temporal de la configuración excluyendo `backend.tf`, con `init -backend=false`, `plan -refresh=false -lock=false`; informó `No changes`. Conservar el backend GCS no inicializado evita leer o modificar state remoto. No se intentó un plan activo: falta login/versión secreta/digest remoto y el gate requiere confirmación explícita en un corte posterior.

## Costos de referencia

Cloud Run Jobs factura por duración con mínimo facturable de un minuto. Con 1 vCPU y 0.5 GiB, usando tarifas publicadas de `$0.000018/vCPU-s` y `$0.000002/GiB-s`, una ejecución mínima cuesta aproximadamente `$0.00114` brutos; el timeout configurado de 120 s limita el cómputo a `$0.00228` por ejecución, antes de free tier/CUD. La cuota gratuita agregada de Jobs es 240,000 vCPU-s y 450,000 GiB-s en `us-central1`; el consumo de la cuenta y su aplicación al workload no se verificaron. Secret Manager indica primeros 6 versiones activas y 10,000 accesos gratis por mes por cuenta; sobre el umbral se aplican cargos por versión-hora y por 10,000 accesos. Cloud SQL existente no añade costo de instancia por este cambio; Direct VPC egress no añade costo de conector, aunque el egreso de red puede cobrar según ruta/región. Artifact Registry queda condicionado al tamaño y retención de una imagen aún no publicada.

Referencias: [Cloud Run pricing](https://cloud.google.com/run/pricing), [Secret Manager pricing](https://cloud.google.com/secret-manager/pricing), [Direct VPC egress](https://docs.cloud.google.com/run/docs/configuring/vpc-direct-vpc), [conectividad y precios VPC](https://docs.cloud.google.com/run/docs/configuring/connecting-vpc).

## Validación local, entorno y evidencia

En el worktree aislado `codex/infra/feature/gcp-04-w7-private-preflight-runner`, desde `main` `9dc4e43ea7d75f2a152125a95220dc6a22cf8305`:

- `go test ./cmd/gcp04preflight ./internal/database/cloudsql`: pasó.
- `bash -n scripts/test-gcp04-w3-logical-baseline-postgres.sh`: pasó.
- `bash scripts/test-gcp04-w3-logical-baseline-postgres.sh`: pasó. Corrió dos clústeres PostgreSQL 18/PostGIS efímeros con red deshabilitada y sockets privados; ejecutó la integración de preflight como `espacigo_observer_login`; verificó replay V1–V64, inventario/datos/ACL y retiros del ensayo. Evidencia privada bajo `.local/testing/gcp04-w3-logical-baseline-*.json` con permisos 0600.
- `docker build --target gcp04-preflight -t espacigo-gcp04-preflight:w7-local .`: pasó local; no se publicó la imagen.
- `terraform fmt`, `validate` y plan OFF en copia local: pasaron; `No changes`.
- `git diff --check`: pendiente al cierre final del PR. V1–V64 permanecen byte por byte; no se propone migración.

La prueba local demuestra compatibilidad con ese PostgreSQL 18 local y el fixture observador; no demuestra compatibilidad, conexión, roles, capacidad ni credenciales del Cloud SQL remoto. No se ejecutó SQL contra Cloud, no se creó/ejecutó Job, no se creó usuario/secret/IAM, no se habilitaron APIs, no se inicializó ni escribió state remoto, no se aplicó Terraform ni se publicó una imagen.

## Siguiente paso y retiro futuro

Un corte de ejecución solo puede comenzar tras verificar, mediante un canal de mínimo privilegio aprobado, el rol observer existente y su `CONNECT/USAGE/SELECT` efectivo; elegir y verificar de forma segura una versión numérica de secreto sin leerla; construir/publicar y fijar digest inmutable; actualizar metadata de destino y presupuesto; inspeccionar el plan activo y recibir revisión/liberación separada. Ejecutar una sola tarea y confirmar que la respuesta reporta target, digest SQL y estado. Ante fallo de identidad, capacidad o SQL, parar.

El retiro futuro, bajo plan y liberación independientes, sería deshabilitar el Job (`enable_preflight=false`), inspeccionar un plan exacto que retire solo Job, service account y sus bindings W7, aplicar solo tras revisión, y retirar por separado el login/secret/versiones exactas únicamente si fueron creados y aprobados. Preservar instancia, DB, datos, VPC/subred, red, bucket de state y recursos ajenos.
