# GCP-08: restauración gobernada de Cloud Run desde Artifact Registry

**Estado:** propuesta de implementación para revisión. No se creó ni modificó ningún recurso Cloud con este cambio. En lectura del 2026-10-20, `gcloud run services list` no devolvió servicios; `terraform state list` del backend canónico conserva las tres direcciones de servicio y ocho bindings de servicio/IAP relacionados. No hace falta importar ni recrear state; el plan debe reconciliar esas direcciones registradas.

## Objetivo y límites

Permitir que un operador solicite desde GitHub Actions la restauración de API, web pública y web privada usando exclusivamente digests del manifiesto de una release exitosa de `main`. El flujo no usa Cloud Build, no compila desde source durante restauración, no ejecuta migraciones, SQL, Jobs, seeds, cambios de secretos ni Terraform completo. No selecciona tags mutables ni crea recursos al invocar el asistente Cloud Run.

El workflow manual `gcp08-cloud-run-restore.yml` separa `preview` de `apply`. El preview genera un plan binario privado en el runner y publica solamente un resumen con direcciones, operaciones, digests y fingerprint. `apply` vuelve a planificar, compara el resumen con el artefacto revisado y aplica el plan recién creado. Su gate exige fingerprint exacto, confirmación escrita, `GCP08_RESTORE_APPLY_ENABLED=true` y `GCP08_RUN_CREATE_PERMISSION_REVIEWED=true`. No se ejecutó ninguna corrida.

Terraform usa únicamente el root canónico `infra/terraform/environments/testprod/gcp` y el backend `espacigo/testprod/gcp`. La lista `-target` se limita a:

- `google_cloud_run_v2_service.api[0]`, `web["public"]` y `web["private"]`;
- los bindings Cloud Run existentes declarados para invocación pública, IAP, acceso API de los SSR y actualización del deployer;
- los miembros IAP privados configurados en `iap_evaluator_members`, preservando el principal previo y sin publicar su identidad.

Antes del plan, el flujo confirma que las tres direcciones de servicio y los ocho bindings de Cloud Run/IAP siguen registradas en el state canónico; el conjunto se fija al resumen con SHA-256 sin publicar las direcciones privadas. El verificador exige las tres creaciones de servicio, imagen por digest dentro del Artifact Registry esperado, nombre/proyecto/región, cuenta de runtime esperada, ingress abierto solo con la autenticación IAM prevista, y la separación IAP pública/privada. También fija instancia/base/usuario/rol y versiones numéricas de secretos del API candidato; valida rutas `/api/v1`, la audiencia derivada del URI de API, orígenes web, allowed hosts y audiencia IAP. Cada binding se valida por servicio, rol y miembro esperado; los evaluadores IAP se identifican en el resumen solo por hash y se exigen en state antes de planificar, sin depender de que la API devuelva `before` después del borrado. Rechaza reemplazos, updates, destrucciones, IAM no allowlisted y cualquier recurso adicional con cambio. Data read queda limitado a la instancia candidata declarada. Las dependencias pueden figurar como no-op; cualquier otra creación o modificación detiene el plan.

## Gates de datos y runtime

Para que una restauración muestre publicaciones de la candidata, los inputs privados deben fijar el candidato `20261020-gcp06a`, un número de versión activo de Secret Manager para su login runtime y para la clave de contratos, `confirm_runtime_secret_versions=true`, `confirm_runtime_database_bootstrap=true` y un endpoint Mailpit privado ya existente. El flujo no lee valores secretos ni los crea. También exige `GCP08_CANDIDATE_PREFLIGHT_SHA`, referencia SHA-256 a la evidencia privada de verificación SQL/runtime revisada por separado.

La referencia de evidencia por sí sola no prueba la base. Antes de habilitarla deben comprobarse ledger/migraciones, extensiones, conteos, owner/ACL y login runtime de la candidata mediante el canal read-only aprobado, más las versiones de secretos `ENABLED`. El informe de GCP-06 del 20 de octubre dice que esa evidencia, el login y una versión runtime activa no estaban confirmados; por lo tanto, ese gate debe permanecer cerrado hasta completar revisión independiente. No se usa la instancia compartida histórica como fallback.

La release seleccionada debe provenir de `application-release.yml`, ser exitosa en `main`, disponer de su artefacto y contener ambos digests. Se permite un dispatch que solo publicó imágenes (`publish_only=true`), pero se rechaza cualquier release con migraciones. También se verifica que no hayan cambiado después otros archivos de aplicación. Un baseline de esquema `GCP07D_SCHEMA_BASELINE_SHA` válido es obligatorio.

## IAM, WIF, state y acceso privado

El proveedor WIF canónico debe incluir el workflow exacto `gcp08-cloud-run-restore.yml@refs/heads/main` dentro de la condición que ya fija repo/IDs/main/environment. Este PR modifica solo esa allowlist en Terraform; **no aplica** el cambio. La lectura del provider confirmó que hoy permite `application-release.yml` y `application-rollout.yml`, pero no el nuevo workflow; GitHub OIDC no funcionará hasta que el plan IAM de WIF se revise y se aplique por separado.

El deployer observado conserva un rol custom con `run.services.get`, `run.services.update` y `run.operations.get`; no incluye `run.services.create` ni `run.services.setIamPolicy`. El binding de acceso IAP usa además la operación `iap.webServices.setIamPolicy`. Restaurar servicios y bindings requiere esas autorizaciones. No se amplía el rol en este PR: el apply permanece desactivado y debe seguir bloqueado hasta que una revisión IAM separada determine si existe una concesión mínima aprobable para cada permiso, con alcance y revocación exactos. El workflow no crea IAM ni eleva su propia identidad. La ejecución `terraform plan` también necesita acceso de lectura al inventario y al state GCS; el state binario no se publica. Referencias: [permisos IAM de Cloud Run](https://docs.cloud.google.com/run/docs/reference/iam/permissions) y [permisos de políticas IAP por recurso](https://docs.cloud.google.com/iap/docs/managing-access).

El acceso privado se restaura solo con IAP habilitado y los miembros ya declarados en Terraform. Después del apply, una persona/grupo necesita `roles/iap.httpsResourceAccessor` en el recurso Cloud Run privado, además del invoker del agente IAP. La URL privada puede seguir redirigiendo a Google si la identidad no está autorizada.

## Estado observado, preview pendiente y bloqueos vigentes

La release manual #39514171226 tiene `publish_only=true` y `contains_migrations=true` (V65/V66), por lo que este workflow la rechazará. No hay todavía un baseline aprobado de esas migraciones. La candidata GCP-06 carece de verificación SQL/runtime aprobada y de una versión activa confirmada del secreto; el `GCP08_CANDIDATE_PREFLIGHT_SHA` no está habilitado. Ese SHA es solo un pin de referencia a una evidencia revisada fuera del workflow, no una prueba automática del contenido SQL. El rol deployer tampoco tiene `run.services.create`. Por ello este PR aporta un flujo cerrado y revisable, pero no produce un preview Terraform remoto válido ni autoriza restaurar los servicios. Cuando se despejen estos bloqueos, el primer dispatch será `preview`; se debe revisar el artefacto exacto antes de autorizar una corrida `apply` independiente.

El state tras el borrado manual se comprobó mediante `terraform state list` filtrado; permanecen las tres direcciones Cloud Run y ocho bindings: dos `api_invoker`, tres `deployer_service_updater`, `public_web_invoker`, `private_iap_invoker` y un principal IAP privado. No se publicó la identidad privada IAP. Si el plan detecta que un binding existente es diferente del valor configurado, el allowlist lo rechazará; el flujo no importa recursos ni modifica el state manualmente.

## Validación de código

La validación de este PR cubre formato Terraform, sintaxis Python/YAML y lectura del state filtrada a las direcciones de restauración. No se ejecutó un preview remoto mientras faltan el baseline, la evidencia privada y la confianza WIF; tampoco `terraform apply`, Cloud Run, Cloud Build, SQL, Secret Manager ni cambios remotos.
