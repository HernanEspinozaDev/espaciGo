# GCP-08: transición a Cloud Build

## Decisión

El despliegue de aplicación se prepara como `main` → Cloud Build → Artifact Registry en `southamerica-west1` → Cloud Run. GitHub Actions conserva sus controles de calidad y validaciones de PR; deja de publicar imágenes o ejecutar Terraform para Cloud Run. Cloud Build será el único propietario de revisiones, configuración y tráfico de los tres servicios, además de la política de invocación e IAP asociada.

Terraform conserva la red compartida, Cloud SQL privado como servicio externo, registro, bucket de archivos, secretos y cuentas runtime. El PostgreSQL 18 `espacigo-testprod-db` y la base `espacigo_dms_stage_20261020` se operan mediante el procedimiento manual existente. No se administran con Terraform, no se importan, no se recrean y no se cambian sus roles, grants, esquema o filas. Los recursos Cloud Run ya registrados en state se olvidan mediante bloques `removed` con `destroy = false`; no se ejecuta ese cambio en este PR.

La ruta de publicación queda configurada en `cloudbuild.yaml`, pero no se crea una conexión, trigger, cuenta de servicio, permiso, API, revisión ni cambio de tráfico. La identidad indicada en el archivo es la cuenta de despliegue que habrá que crear y autorizar durante la activación revisada. La política de invocación/IAP también queda a cargo de esa identidad, no de Terraform.

## Parámetros y límites

- Proyecto `espacigo-testprod`, región `southamerica-west1`, repositorio Artifact Registry `espacigo-testprod`.
- API: `espacigo-test-api`, puerto 8080, 1 CPU siempre asignada, 1 GiB, mínimo/máximo 1, concurrencia 80, VPC `espacigo-testprod`, subred `api-serverless`, egress `PRIVATE_RANGES_ONLY`.
- SSR público y privado: servicios `espacigo-test-public` / `espacigo-test-private`, puerto 4000, 1 CPU y 1 GiB, mínimo 0 y máximo 1, concurrencia 40, ingreso `internal-and-cloud-load-balancing`, VPC `espacigo-testprod`, subred `serverless`, egress `ALL_TRAFFIC` por Direct VPC y Private Google Access para las claves IAP.
- Timeout configurado: 300 segundos. API privada por IAM; web pública con invocación pública y controles de aplicación; web privada con IAP. El balanceador HTTPS, NEG, Cloud Armor, certificados y DNS conservan la arquitectura de Santiago.
- PostgreSQL runtime: instancia `espacigo-testprod-db`, base `espacigo_dms_stage_20261020`, secreto de contraseña `espacigo-test-database-password-v67` versión 1. No ejecutar migraciones durante build o despliegue.
- Las versiones de las claves de contrato y webhook, valores de Mailpit, bucket y prefijos deben verificarse contra la configuración vigente antes de activar. El archivo falla cerrado mientras falten versiones numéricas y hosts revisados; no leer payloads de secretos.
- La sustitución `_IAP_MEMBERS` debe recibir la lista revisada de principales IAP ya autorizados, separada por comas, sin añadir identidades nuevas por inferencia.
- El build orquestador ejecuta un build de imágenes hijo una sola vez por commit. `cloudbuild-images.yaml` usa el campo `images` de Cloud Build y `requestedVerifyOption: VERIFIED`, por lo que exige provenance antes de marcar exitosa la publicación. El orquestador valida su presencia y despliega por digest. No usa `docker push` explícito. Habilitar Cloud Build, Artifact Registry y Container Analysis APIs solo durante la activación aprobada.

Cloud Build serializa despliegues con un objeto de lock creado condicionalmente en el bucket dedicado `espacigo-testprod-deploy-lock`. La cuenta de build necesita permisos limitados para crear, consultar y borrar ese objeto. Si cualquier paso falla, el lock permanece para impedir que una ejecución posterior se adelante; el operador revisa el build fallido y solo retira el objeto tras comprobar que no hay un despliegue activo. Un watermark compara el tiempo de creación del build con el último despliegue exitoso y rechaza ejecuciones antiguas.

## IAM a provisionar en activación separada

Crear una cuenta de servicio de usuario dedicada, `espacigo-cloud-build-deployer`, distinta de las tres identidades runtime. El trigger la selecciona explícitamente; la clave `serviceAccount` de YAML no determina la identidad de builds invocados por trigger. Conceder solamente:

- `roles/artifactregistry.writer` sobre el repositorio de imágenes existente.
- Un rol custom limitado a `run.services.create`, `run.services.get`, `run.services.list`, `run.services.update`, `run.services.getIamPolicy`, `run.services.setIamPolicy`, `run.operations.get`, `run.revisions.get` y `run.revisions.list`. No conceder `run.services.delete` ni permisos `run.jobs.*`.
- `cloudbuild.builds.create` y `cloudbuild.builds.get` para enviar y esperar el build hijo de imágenes; `iam.serviceAccounts.actAs` sobre sí misma para ese build hijo.
- `roles/iam.serviceAccountUser` sobre las identidades runtime API, web pública y web privada.
- Escritura de logs de Cloud Build, lectura del repositorio y permisos de objeto mínimos sobre los buckets dedicados de lock y staging (`espacigo-testprod-deploy-lock` y `espacigo-testprod-build-source`).
- `iap.webServices.getIamPolicy` y `iap.webServices.setIamPolicy` para el servicio privado de Cloud Run en la región elegida; el enlace del agente IAP requiere `run.services.setIamPolicy`. No conceder permisos para administrar IAP de balanceadores, túneles o servicios ajenos.
- `cloudbuild.builds.get` para que el paso de orden compare el `createTime` del build invocador.

Las identidades runtime conservan sus permisos actuales de Cloud SQL, Secret Manager y Storage. La cuenta de build no recibe permisos de lectura de secretos de aplicación ni de acceso SQL. Cloud Build será el único propietario externo de las revisiones/configuración Cloud Run, políticas `run.invoker` e IAP. La transferencia debe verificar que API siga privada, web pública mantenga su acceso en el balanceador y allowlist de aplicación, y web privada exija IAP.

Google documenta `roles/run.admin`, `roles/iap.admin`, Artifact Registry Reader y Service Account User como roles suficientes de referencia para configurar IAP en Cloud Run; aquí se usan roles custom más pequeños, limitados a los permisos y recursos anteriores. Las políticas de acceso de usuarios IAP se conservan y se versionan bajo ese único dueño durante la activación. [Permisos Cloud Run](https://docs.cloud.google.com/run/docs/reference/iam/permissions), [configurar IAP en Cloud Run](https://docs.cloud.google.com/run/docs/securing/identity-aware-proxy-cloud-run), [gestionar acceso IAP](https://docs.cloud.google.com/iap/docs/managing-access), [cuenta de servicio de Cloud Build](https://docs.cloud.google.com/build/docs/securing-builds/configure-user-specified-service-accounts).

## Secuencia manual de activación

1. Revisar este PR y el plan canónico; confirmar que el inventario real de Cloud Run, IAM, IAP, hosts y políticas coincide con los nombres y valores documentados.
2. Revisar el plan Terraform local del cambio de state: solo debe olvidar bindings Cloud Run/IAP conocidos sin destruirlos. Confirmar cero cambios de SQL, datos, IAM cloud, APIs, red y tráfico. Esa transición requiere aprobación explícita separada antes de aplicar.
3. Completar versiones habilitadas de secretos por metadatos, endpoint de Mailpit, bucket/prefijos, hosts, audiencia IAP y flags de entorno. Confirmar que el backend soporta configuración SSR y que no hay migraciones pendientes.
4. Preparar los buckets dedicados de lock y staging con acceso privado; comprobar las precondiciones de generación `0`, lectura de watermark y borrado condicional en un bucket de ensayo sin despliegues. Mantener el watermark `last-successful-main.json`; un build antiguo no puede reemplazar un despliegue posterior. Si falla un build o pierde la conexión, el lock se conserva y requiere inspección manual antes de retirarlo.
5. Configurar la cuenta de despliegue con IAM mínimo y auditar `actAs`, Artifact Registry, Cloud Run, IAP, logging y lock. Las cuentas runtime mantienen acceso exclusivo a sus secretos.
6. En Cloud Build Console, habilitar APIs solo tras revisión, conectar exactamente `HernanEspinozaDev/espaciGo`, usar la región `southamerica-west1`, branch regex `^main$`, seleccionar explícitamente la cuenta de build de usuario y apuntar a `cloudbuild.yaml`. Evitar el primer build automático hasta completar pasos 1–5.
7. Hacer una ejecución manual controlada para un SHA confirmado de `main`; cotejar SHA, digests, provenance, variables, IAM y revisión saludable. No desplegar migraciones. Verificar API privada, web pública y web privada con IAP; comprobar `/health/ready`, `/healthz` y el recorrido acordado.
8. Recién después, habilitar el trigger de `main`, confirmar la exclusión/retirada del publicador y rollout de Actions, y conservar checks de CI.

La configuración de Cloudflare/DNS mantiene CNAME hacia el balanceador y los certificados administrados. No apuntar DNS directamente a `run.app`, no usar valores de región/CPU/escalado del asistente de Cloud Run y no cambiar billing. [Cloud Build provenance](https://docs.cloud.google.com/build/docs/securing-builds/generate-validate-build-provenance) requiere declarar `images` y no hacer `docker push` explícito; `requestedVerifyOption: VERIFIED` hace fallar el build si falta provenance.

## Estado y evidencia

Este PR solo prepara archivos versionados y controles locales. No ejecuta Cloud Build, Terraform remoto, APIs, conexión GitHub, IAM, Cloud Run, IAP, DNS, SQL, secretos ni billing. Las versiones de contrato/webhook y valores operativos que no constan como verificados quedan como bloqueo explícito de la futura activación. PR #515 y los previews anteriores permanecen sin cambios y están supersedidos por esta estrategia.
