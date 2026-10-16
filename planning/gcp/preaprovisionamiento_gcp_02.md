# GCP-02: preaprovisionamiento local

Preparado el 2026-10-15 (America/Santiago), actualizado el 2026-10-16 (UTC). Estado actual: GCP-02 creó los tres buckets de Terraform state mediante plan revisado; no hay otros recursos cloud del corte.

## Alcance y decisiones

El alcance autorizado incluyó Terraform, inventario de solo lectura, estimación, plan revisable y apply únicamente del plan exacto de los tres buckets. Ese apply se completó. No se ejecutaron `import`, habilitación de APIs, cambios IAM, DNS, migración de state, servicios, SQL ni copia de datos. El proyecto permanece en Free Trial según declaración del usuario; no se cambió a facturación pagada. La fecha exacta de término del crédito no está disponible, y el saldo aproximado de USD 200 es declarado por el usuario, no verificado en Billing Overview.

El bootstrap creó solo tres buckets separados (bootstrap, GCP, Cloudflare), con acceso uniforme, prevención de acceso público, versionado y `prevent_destroy`; verificación post-apply sin drift. No se agregaron bindings IAM. El acceso vigente del Owner a nivel de proyecto sigue siendo el mecanismo existente; no se extiende ese rol ni se inventan operadores dedicados. Se concederá `roles/storage.objectAdmin` por bucket en una revisión posterior, luego de crear y aprobar cada principal. El state bootstrap empieza local, con archivo 0600, y requiere una migración explícita posterior al bucket bootstrap.

Los nombres, derivados del número de proyecto, se comprobaron sin existencia justo antes del apply; la disponibilidad global solo se confirma al crear de forma atómica. El archivo candidato de variables vive bajo `.local/planning/gcp/` (excluido de Git, directorio 0700/archivo 0600); `create_state_bucket=true` se usó únicamente en el plan exacto revisado y aplicado; no autoriza otros planes ni futuros `apply`.

## Inventario conocido y brechas

Lectura inicial segura del proyecto `espacigo-testprod` el 2026-10-15 (America/Santiago): proyecto activo; Cloud Storage, Service Usage y APIs de observabilidad/analítica figuraban habilitadas. La consulta previa encontró 0 buckets de Storage, 0 service accounts, 0 datasets de BigQuery visibles y 2 buckets de Logging; IAM devolvió un binding Owner sin exponer el principal. Esos resultados son parciales y no prueban ausencia de otros recursos. Cloud Asset, Resource Manager, Org Policy, Compute, Run, SQL Admin, Secret Manager y Artifact Registry no aparecían en `gcloud services list --enabled` de esa lectura. Antes del plan de habilitación, el 2026-10-15 22:18 America/Santiago, la lista con proyecto explícito confirmó `orgpolicy.googleapis.com` y `cloudasset.googleapis.com` apagadas. El plan mínimo se ejecutó después; ambas ahora están habilitadas. No se modificó la configuración de proyecto default de gcloud.

La consulta Asset Inventory del proyecto se completó el 2026-10-15 (America/Santiago): 27 resultados dentro del scope de proyecto: 25 recursos `serviceusage.googleapis.com/Service`, 1 `cloudresourcemanager.googleapis.com/Project` y 1 `cloudbilling.googleapis.com/ProjectBillingInfo`. Es una búsqueda de los tipos visibles/soportados por Cloud Asset en ese proyecto, no lectura de datos de aplicación ni prueba universal de ausencia. No se exportaron datos a Storage. La evidencia de operaciones está en [evidencia de inventario y políticas](evidencia_inventario_apis_20261015.md).

### Estado verificable de políticas efectivas

La lectura previa del 2026-10-15 quedó bloqueada, pero la revisión mínima posterior sí se completó: `gcloud org-policies list --show-unset` y `describe --effective` se consultaron con el proyecto y proyecto de cuota explícitos. `gcloud projects describe` confirma que `espacigo-testprod` es hijo directo de la organización `644690775054` (sin folder intermedio). La lista directa mostró 0 políticas configuradas en el proyecto y 6 en la organización. Comparar ambos scopes con las respuestas efectivas del proyecto permite atribuir la herencia; los seis constraints configurados en la organización son `compute.restrictProtocolForwardingCreationForTypes`, `compute.setNewProjectDefaultToZonalDNSOnly`, `iam.automaticIamGrantsForDefaultServiceAccounts`, `iam.disableServiceAccountKeyCreation`, `iam.disableServiceAccountKeyUpload` y `storage.uniformBucketLevelAccess`.

| Restricción consultada con `--effective` en el proyecto | Valor efectivo recibido | Origen directo observado |
| --- | --- | --- |
| `run.allowedIngress`, `run.allowedVPCEgress`, `compute.restrictLoadBalancerCreationForTypes`, `compute.restrictVpcPeering`, `iam.allowedPolicyMemberDomains`, `gcp.resourceLocations`, `serviceuser.services` | `allowAll=true` | Sin policy directa en proyecto ni organización en los scopes consultados |
| `compute.disableGlobalLoadBalancing`, `iam.disableServiceAccountCreation` | `enforce=false` | Sin policy directa en proyecto ni organización en los scopes consultados |
| `storage.uniformBucketLevelAccess` | `enforce=true` | Policy directa en la organización; proyecto no tiene policy directa |
| `compute.restrictProtocolForwardingCreationForTypes` | Solo permite `INTERNAL` | Policy directa en la organización; proyecto no tiene policy directa. Es forwarding a target instances, no la restricción del tipo de Load Balancer. |
| `iam.disableServiceAccountKeyCreation`, `iam.disableServiceAccountKeyUpload`, `iam.automaticIamGrantsForDefaultServiceAccounts` | `enforce=true` | Policy directa en la organización; proyecto no tiene policy directa |
| `compute.setNewProjectDefaultToZonalDNSOnly` | `enforce=true` | Policy directa en la organización; es una restricción de defaults para proyectos nuevos |

Las lecturas efectivas/proyecto y lista de la organización fueron exitosas con la identidad ya configurada; no se otorgó ningún rol IAM. El primer intento de consulta pasó por el proyecto default ajeno de gcloud, no habilitó servicios allí y fue rechazado con la opción predeterminada `N`; las repeticiones fijaron `--billing-project=espacigo-testprod`. Los resultados completos, con nombres/IDs privados y errores, permanecen bajo `.local` modo 0600. Esta evidencia confirma solo los constraints enumerados y los scopes consultados; no reemplaza futuras revisiones al cambiar topología o políticas.

El único inventario local del almacén privado inspeccionó metadatos de filesystem (sin leer contenido ni consultar la BD): 3.245 archivos PNG y 808.005 bytes en el mount completo, que mezcla usos y no se puede atribuir íntegramente al dataset. Una evidencia privada anterior de assets de carga registró agregados por grupo de 0/0, 5/1.245 B, 1/338 B y 121/4.954.983 B; es anterior, de alcance distinto y no reconcilia el mount actual. No se afirma que esos archivos deban copiarse a GCS. El tamaño de datos/migración real requiere vínculo controlado con registros, selección de objetos y reglas de retención/versionado.

### Lecturas faltantes, APIs y permisos mínimos

Los siguientes comandos son plantillas de inventario futuro, todos de lectura. No se ejecutaron cuando requerían APIs deshabilitadas. Ejecutarlos solo con proyecto explícito y guardar la evidencia privada sin identidades/secretos en el PR:

| Comando de lectura | API requerida | Permiso mínimo pertinente |
| --- | --- | --- |
| `gcloud services list --enabled --project="$PROJECT_ID"` | `serviceusage.googleapis.com` (ya habilitada) | `serviceusage.services.list` |
| `gcloud projects describe "$PROJECT_ID" --billing-project="$PROJECT_ID" --format=json` | `cloudresourcemanager.googleapis.com` | `resourcemanager.projects.get` |
| `gcloud projects get-iam-policy "$PROJECT_ID" --billing-project="$PROJECT_ID" --format=json` | `cloudresourcemanager.googleapis.com` | `resourcemanager.projects.getIamPolicy` |
| `gcloud org-policies list --project="$PROJECT_ID" --billing-project="$PROJECT_ID" --show-unset --format=json` | `orgpolicy.googleapis.com` | `orgpolicy.policies.list` y `orgpolicy.policy.get` en el recurso consultado |
| `gcloud org-policies list --organization=644690775054 --billing-project="$PROJECT_ID" --show-unset --format=json` | `orgpolicy.googleapis.com` | `orgpolicy.policies.list` y `orgpolicy.policy.get` en organización |
| `gcloud asset search-all-resources --scope="projects/$PROJECT_ID" --project="$PROJECT_ID" --billing-project="$PROJECT_ID" --format=json` | `cloudasset.googleapis.com` | `cloudasset.assets.searchAllResources` |

Comandos efectivos requeridos después de la fase de habilitación, para cada restricción relevante (ejemplos a adaptar según topología):

```sh
gcloud org-policies describe run.allowedIngress --project="$PROJECT_ID" --billing-project="$PROJECT_ID" --effective --format=json
gcloud org-policies describe run.allowedVPCEgress --project="$PROJECT_ID" --billing-project="$PROJECT_ID" --effective --format=json
gcloud org-policies describe compute.restrictLoadBalancerCreationForTypes --project="$PROJECT_ID" --billing-project="$PROJECT_ID" --effective --format=json
gcloud org-policies describe compute.restrictVpcPeering --project="$PROJECT_ID" --billing-project="$PROJECT_ID" --effective --format=json
gcloud org-policies describe iam.allowedPolicyMemberDomains --project="$PROJECT_ID" --billing-project="$PROJECT_ID" --effective --format=json
gcloud org-policies describe gcp.resourceLocations --project="$PROJECT_ID" --billing-project="$PROJECT_ID" --effective --format=json
gcloud org-policies describe serviceuser.services --project="$PROJECT_ID" --billing-project="$PROJECT_ID" --effective --format=json
gcloud org-policies describe compute.disableGlobalLoadBalancing --project="$PROJECT_ID" --billing-project="$PROJECT_ID" --effective --format=json
gcloud org-policies describe iam.disableServiceAccountCreation --project="$PROJECT_ID" --billing-project="$PROJECT_ID" --effective --format=json
gcloud org-policies describe iam.disableServiceAccountKeyCreation --project="$PROJECT_ID" --billing-project="$PROJECT_ID" --effective --format=json
gcloud org-policies describe iam.disableServiceAccountKeyUpload --project="$PROJECT_ID" --billing-project="$PROJECT_ID" --effective --format=json
gcloud org-policies describe iam.automaticIamGrantsForDefaultServiceAccounts --project="$PROJECT_ID" --billing-project="$PROJECT_ID" --effective --format=json
gcloud org-policies describe compute.setNewProjectDefaultToZonalDNSOnly --project="$PROJECT_ID" --billing-project="$PROJECT_ID" --effective --format=json
gcloud org-policies describe storage.uniformBucketLevelAccess --project="$PROJECT_ID" --billing-project="$PROJECT_ID" --effective --format=json
```

El `--project` consulta el resultado efectivo para el proyecto, que puede heredar valores de folder/organización. Para atribuir de qué nivel proviene cada ajuste, comparar además la policy explícita en el proyecto, el folder (si existe) y `--organization=644690775054`; `list --show-unset` por sí sola no hace esa atribución ni muestra el valor efectivo. Las restricciones del ejemplo corresponden a las superficies que usa la topología Terraform propuesta; no se declara que hayan sido leídas ni aprobadas.

El root separado `infra/terraform/environments/testprod/inventory-apis` solo admite `orgpolicy.googleapis.com` y `cloudasset.googleapis.com` en `espacigo-testprod`; `disable_on_destroy=false`, lista permitida vacía por defecto, backend local en `.local` privada. Tras plan autenticado con refresh y verificación de 2 altas/0 cambios/0 eliminaciones, se aplicó el mismo plan: 2 APIs añadidas, 0 cambios, 0 eliminaciones. Un plan autenticado posterior no detectó cambios. Los dos servicios ya están habilitados; verificar su estado antes de cualquier nuevo plan. Los logs registran solo las dos APIs solicitadas; el snapshot previo no guardó el listado completo, por lo que no se infiere un delta agregado ni dependencias no reflejadas en esos eventos. No se crearon recursos de infraestructura. Costo directo de esta habilitación/consulta: USD 0. La evidencia y los límites están en [evidencia de inventario y políticas](evidencia_inventario_apis_20261015.md).

Permisos de habilitación: `serviceusage.services.enable` y `serviceusage.services.get` sobre el proyecto (Google recomienda `roles/serviceusage.serviceUsageAdmin`; preferir un rol personalizado mínimo cuando la organización lo permita). Para preflight, `serviceusage.services.list`; para inventario de políticas directas, `orgpolicy.policies.list` y `orgpolicy.policy.get` en el recurso; para `describe --effective`, `orgpolicy.policy.get`; para el inventario de recursos, `cloudasset.assets.searchAllResources` en `projects/espacigo-testprod`. El rol `roles/orgpolicy.policyViewer` es la opción predefinida de solo lectura de Organization Policy, pero requiere el API y acceso en el alcance consultado. No incluir roles `orgpolicy.policies.update` ni Owner. La habilitación de APIs no crea recursos facturables; Cloud Asset Inventory declara gratuito su uso. Estas consultas no exportan datos a almacenamiento. Costo directo modelado de habilitar ambos y realizar estas lecturas: USD 0; costos de recursos descubiertos quedan fuera y se contabilizan por separado. Referencias oficiales: [habilitación de servicios/permisos Service Usage](https://docs.cloud.google.com/service-usage/docs/enable-disable), [roles y permisos Org Policy](https://docs.cloud.google.com/iam/docs/roles-permissions/orgpolicy), [Cloud Asset search y permiso](https://docs.cloud.google.com/asset-inventory/docs/reference/rest/v1/TopLevel/searchAll), [precios Cloud Asset (uso gratuito)](https://cloud.google.com/asset-inventory/pricing).

Las lecturas detalladas deben usar APIs/roles de lectura correspondientes (p. ej. Storage, IAM, Compute, Run, SQL Admin, Artifact Registry y Secret Manager) y no justifican permisos Owner adicionales.

## Costos y escenarios

USD de lista, sin impuestos, descuentos, Trial ni condiciones particulares de la cuenta. Se modelan 24 h, 30 días (720 h), 730 h y `D` días restantes de Trial; `D` y el saldo actual no fueron verificados. La estimación completa, sus fórmulas, supuestos de SSR/egress/backups y límites está en [modelo de costos Terraform](../../infra/terraform/cost-model.md).

El piso continuo actualizado separa modelos de Cloud Run: API con `cpu_idle=false`/instancia continua a tarifa instance-based ($0.000018/vCPU-s + $0.000002/GiB-s) y SSR request-based, con horas activas variables. Para el footprint fijo, el rate base suma **$186.08/730 h**, y +20% ilustrativo aplicado solo a API da **$196.60/730 h**. Cloud SQL N4 (2 vCPU/4 GiB) representa $104.97/730 h, Hyperdisk inicial 20 GiB $2.30/730 h y balanceador/Armor $26.25/730 h (una regla global, una política Armor y tres reglas). La tarifa exacta Tier 2 debe verificarse en SKU/calculadora antes de activar servicios. El piso no incluye backups/PITR, IOPS, transferencia ni SSR.

El escenario de referencia explícito usa 1.000 horas facturables SSR combinadas a la tarifa request-based base ($95.40; sensibilidad +20%: $114.48), 100 GiB de egress a Sudamérica, 200 GiB procesados por ALB, 10 M requests Armor, 20 GiB fotos, 2 GiB de imágenes en Artifact Registry, 10 GiB logs, 50.000 accesos a secretos y 20 GiB promedio de backups. La suma reproducible es **$311.85–$312.25/mes base** y **$341.45–$341.85/mes sensible**; no representa demanda medida. Este resultado y la posibilidad de superar el crédito declarado (~$200) justifican mantener gates de SQL/Run sujetos a decisión/cotización; los tres buckets se aplicaron dentro de su autorización limitada.

Los tres buckets existen y permanecen vacíos; costo de capacidad actual: $0; las operaciones de verificación no se estiman. Escenario operativo no observado: tres estados de 100 KiB, cuatro escrituras por día por state y 30 días de versioning (hasta 120 generaciones/bucket), diez lecturas por día. La cota aproximada de almacenar todas las generaciones el mes completo es $0.0007 más $0.0022 de operaciones, alrededor de **$0.003/mes**. El versioning no configura retención; lifecycle/retención debe definirse antes de uso sostenido.

No se calcula la duración restante (`D`) ni se usa el saldo declarado como dato del Billing Overview. No se habilita Paid Billing.

## Plan autenticado y ejecución de buckets

Comprobación previa al apply, el 2026-10-16: `storage.googleapis.com` figuró `ENABLED`; `gcloud storage buckets list` no devolvió buckets del proyecto y los tres `describe` candidatos devolvieron 404. Los nombres dejaron de estar disponibles para crear porque ahora existen; la comprobación histórica de 404 no reservaba disponibilidad global. Nombre candidato: `espacigo-testprod-tfstate-447373903153-{bootstrap,gcp,cloudflare}`.

El root `bootstrap` ahora tiene backend local explícito, guardado solo bajo `.local/`, y ya no declara `google_project_service.storage`: el root GCP existente es el único owner Terraform de ese API. `storage_api_preverified=true` es un gate de plan y solo se incluye en el tfvars privado después de la lectura anterior. Operators están vacíos. El plan autenticado con refresh se generó, revisó y se aplicó sin cambios en el artefacto planificado: exactamente tres altas de `google_storage_bucket.terraform_state`, cero IAM/API/otros cambios. SHA-256 `963e5bcfa01cc0f4b3a0f386c50758f00780067d1265cacd9e3e42af502b270f`. El archivo binario, JSON y texto quedaron privados bajo `.local/`; la evidencia de preparación sigue en [evidencia del plan](evidencia_plan_buckets_gcp02_20261016.md) y la ejecución/verificación en [evidencia post-apply](evidencia_ejecucion_buckets_gcp02_20261016.md). El state de inventario sigue separado y preservado.

El estado `inventory-apis` y su snapshot siguen en `.local/planning/gcp/inventory-apis-exec-20261015/`; no se migran ni eliminan. Init/plan/apply de bootstrap usaron `umask 077`, path de backend local explícito dentro de `.local/`, tfvars/plan/evidencia privados (directorio 0700, archivos 0600). Revisar `git check-ignore` y que no se haya escrito un `terraform.tfstate*` fuera de `.local`.

## Fuentes oficiales de tarifas

Consulta del 2026-10-15. Tarifas y condiciones pueden cambiar; recalcular antes de cualquier habilitación: [Cloud Run](https://cloud.google.com/run/pricing), [Cloud SQL](https://cloud.google.com/sql/pricing), [Cloud Load Balancing](https://cloud.google.com/load-balancing/pricing), [Cloud Armor](https://cloud.google.com/armor/pricing), [VPC/network egress](https://cloud.google.com/vpc/network-pricing), [Cloud Storage](https://cloud.google.com/storage/pricing), [Artifact Registry](https://cloud.google.com/artifact-registry/pricing), [Cloud Logging/Observability](https://cloud.google.com/products/observability/pricing), [Secret Manager](https://cloud.google.com/secret-manager/pricing).

## Siguientes cortes y trabajos históricos

- GCP-03: restauración/compatibilidad de datos y persistencia local, artefactos e identidad operativa. El conjunto sintético de cuentas/espacios y el almacén privado actual no se deben reconstruir ni regenerar al pasar de corte; cualquier migración requiere inventario reconciliado y autorización específica.
- GCP-04: la ejecución histórica del piloto y la carga persistente de 1.200 tratos con reloj aislado ya está registrada en `planning/dataset_loader_espacigo_1000.md` (commits `235b9c8` y `f20cc73`) con sus conteos, conciliación y baseline previo. No repetir ni regenerar ese dataset al cambiar a GCP-02/GCP-03; esa evidencia no habilita cambios cloud.

No se declara integración cloud, aceptación de producto, carga de datos ni aprobación de gasto.
