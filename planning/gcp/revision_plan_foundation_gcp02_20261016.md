# Foundation GCP-02: plan real para revisión

Este PR publica un manifest sin secretos extraído del JSON del plan autenticado. **No se aplicó.** El plan binario y su JSON completo permanecen privados en `.local/planning/gcp/foundation-plan-20261016/` (directorio `0700`, archivos `0600`). El hash de plan y del state GCP vacío quedan registrados en el [manifest](manifest_plan_foundation_gcp02_20261016.json).

## Acciones que incluiría el plan

Con `enable_foundation=true` y database, services, edge y DNS apagados, Terraform presenta 41 acciones `create`, 0 `update`, 0 `delete`. No hay recursos Cloud SQL, servicios Cloud Run, balanceador/Armor, DNS ni versiones/valores de secretos. Sí se incluyen las APIs `sqladmin.googleapis.com` y `run.googleapis.com`; habilitar una API no crea instancias SQL ni servicios Cloud Run.

El manifest enumera las 41 direcciones, nombres y acciones del plan real; incluye las cinco cuentas de servicio, VPC/subred, WIF, repositorio Artifact Registry, bucket privado candidato y tres contenedores de Secret Manager. El bucket candidato es `espacigo-test-files-447373903153-gcp02`; el lookup de solo lectura no lo encontró, pero la disponibilidad global solo se prueba al crearlo.

## APIs y propiedad única

Se planifican 13 direcciones `google_project_service`. `storage.googleapis.com` y `serviceusage.googleapis.com` ya estaban habilitadas; el plan las agrega como direcciones gestionadas del state GCP (no como import) sin duplicar ownership Terraform. No se habilitó ninguna API al generar el plan. El root `inventory-apis` es el único dueño Terraform de `cloudasset.googleapis.com` y `orgpolicy.googleapis.com`; no se incluyen. El root GCP posee `storage.googleapis.com` y Service Usage, sin solapamiento entre roots.

La evidencia previa de policies efectivas registró `iam.disableServiceAccountCreation=false`, dominios permitidos `allowAll`, ubicaciones `allowAll`, UBLA forzado y creación/subida de claves de SA bloqueadas. No se propone cambiar esas policies. Releerlas antes de cualquier apply.

## Grants previstos y criterio de revisión

El plan incluye 14 bindings: `roles/artifactregistry.writer` (1), `roles/cloudsql.client` (2), `roles/iam.serviceAccountUser` (4), `roles/iam.workloadIdentityUser` (1), `roles/run.developer` (1), `roles/secretmanager.secretAccessor` (4) y `roles/storage.objectAdmin` (1). El manifest identifica cada dirección, destino y grantee operativo. WIF usa el pool `espacigo-testprod-github`, provider `espacigo-main` y condición estricta para `HernanEspinozaDev/espaciGo` en `refs/heads/main`.

La opción elegida para este PR es provisionar conjuntamente los 14 bindings con las identidades/recursos Foundation, para dejar preparado el deployer y runtime de la siguiente fase. Eso concede acceso antes de que existan SQL/Run workloads; son acciones del plan, no permisos ya concedidos. El revisor debe aceptar cada rol, grantee, alcance y momento de los 14 antes de cualquier apply. `roles/run.developer` es project-wide y `roles/iam.serviceAccountUser` permite `actAs` sobre cuatro cuentas runtime; su riesgo/transitividad queda explícito y no se considera aprobado por este documento.

## State y locks del backend GCP

`terraform init` configuró el backend GCP en el bucket `espacigo-testprod-tfstate-447373903153-gcp`, prefijo `espacigo/testprod/gcp`. Al no existir state local para ese root, dejó un `default.tfstate` remoto vacío (serial 1, cero recursos). Su hash consta en el manifest. El plan no alteró esos bytes. El bucket versionado conserva tres generaciones no vigentes de `default.tflock`; no hay lock activo. Esto fue inicialización de un root vacío, no migración de otro state.

El state bootstrap migrado y `inventory-apis` se mantienen separados. La evidencia de la migración bootstrap está en [su PR #332 fusionado](evidencia_migracion_state_bootstrap_gcp02_20261016.md).

## Costos de esta fase

El piso idle modelado es USD 0/mes si Artifact Registry y el bucket de archivos permanecen vacíos y los secretos no tienen versiones ni operaciones por encima del uso gratuito. El modelo por unidad, límites, exclusiones y precios consultados está en [costo Foundation](costo_foundation_gcp02.md). SQL, Run, edge, DNS, egreso, logs, imágenes, bytes de archivos y secretos cargados están excluidos. No se verificó saldo ni duración de Free Trial y no se cambió billing.

Antes de cualquier apply: obtener aceptación explícita del plan binario exacto (hash en el manifest), revisar los 14 grants y las policies, confirmar disponibilidad del nombre global del bucket, recapturar el listado completo de APIs habilitadas y su hash, revisar nuevamente costos y billing, y aplicar solo el plan guardado si sigue idéntico. Este PR no autoriza ni ejecuta ese apply.
