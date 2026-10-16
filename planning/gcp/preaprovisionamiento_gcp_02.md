# GCP-02: preaprovisionamiento local

Fecha de preparación: 2026-10-15 (America/Santiago). Estado: planificación local; sin cambios cloud.

## Alcance y decisiones

El trabajo autorizado cubre Terraform, inventario de solo lectura, estimación y un procedimiento reproducible. No ejecuta `apply`, `import`, habilitación de APIs, IAM, DNS, migración de state, servicios, SQL ni copia de datos. El proyecto permanece en Free Trial según declaración del usuario; no se cambió a facturación pagada. La fecha exacta de término del crédito no está disponible, y el saldo aproximado de USD 200 es declarado por el usuario, no verificado en Billing Overview.

El primer bootstrap propuesto crea solo tres buckets de state separados (bootstrap, GCP, Cloudflare), con acceso uniforme, prevención de acceso público, versionado y `prevent_destroy`. Puede comenzar sin bindings IAM adicionales. El acceso vigente del Owner a nivel de proyecto sigue siendo el mecanismo existente; no se extiende ese rol ni se inventan operadores dedicados. Se concederá `roles/storage.objectAdmin` por bucket en una revisión posterior, luego de crear y aprobar cada principal. El state bootstrap empieza local, con archivo 0600, y requiere una migración explícita posterior al bucket bootstrap.

Los nombres candidatos privados derivan del número de proyecto. Su disponibilidad global no fue comprobada y deben verificarse antes de planificar/aplicar. El archivo candidato de variables vive bajo `.local/planning/gcp/` (excluido de Git, directorio 0700/archivo 0600); `create_state_bucket=true` se usa solo para un futuro plan revisado, jamás como autorización de `apply`.

## Inventario conocido y brechas

Lectura segura del proyecto `espacigo-testprod` el 2026-10-15: proyecto activo; Cloud Storage, Service Usage y APIs de observabilidad/analítica figuraban habilitadas. La consulta previa encontró 0 buckets de Storage, 0 service accounts, 0 datasets de BigQuery visibles y 2 buckets de Logging; IAM devolvió un binding Owner sin exponer el principal. Esos resultados son parciales y no prueban ausencia de otros recursos. Las APIs Cloud Asset, Resource Manager, Org Policy, Compute, Run, SQL Admin, Secret Manager y Artifact Registry no figuraban habilitadas en esa lectura.

El inventario de políticas ya disponible no indicó restricciones efectivas para ingress/egress de Run, tipos de balanceador, peering, dominios IAM o ubicación; UBLA sí se exige. Las lecturas no están completas ni sustituyen una nueva exportación de políticas efectivas antes de abrir gates. No se habilitaron APIs para completar las brechas.

El único inventario local del almacén privado inspeccionó metadatos de filesystem (sin leer contenido ni consultar la BD): 3.245 archivos PNG y 808.005 bytes en el mount completo, que mezcla usos y no se puede atribuir íntegramente al dataset. Una evidencia privada anterior de assets de carga registró agregados por grupo de 0/0, 5/1.245 B, 1/338 B y 121/4.954.983 B; es anterior, de alcance distinto y no reconcilia el mount actual. No se afirma que esos archivos deban copiarse a GCS. El tamaño de datos/migración real requiere vínculo controlado con registros, selección de objetos y reglas de retención/versionado.

### Lecturas faltantes, APIs y permisos mínimos

Los siguientes comandos son plantillas de inventario futuro, todos de lectura. No se ejecutaron cuando requerían APIs deshabilitadas. Ejecutarlos solo con proyecto explícito y guardar la evidencia privada sin identidades/secretos en el PR:

| Comando de lectura | API requerida | Permiso mínimo pertinente |
| --- | --- | --- |
| `gcloud services list --enabled --project="$PROJECT_ID"` | `serviceusage.googleapis.com` (ya habilitada) | `serviceusage.services.list` |
| `gcloud projects describe "$PROJECT_ID" --format=json` | `cloudresourcemanager.googleapis.com` | `resourcemanager.projects.get` |
| `gcloud projects get-iam-policy "$PROJECT_ID" --format=json` | `cloudresourcemanager.googleapis.com` | `resourcemanager.projects.getIamPolicy` |
| `gcloud org-policies list --project="$PROJECT_ID" --show-unset --format=json` | `orgpolicy.googleapis.com` | `orgpolicy.policy.get` en el recurso consultado |
| `gcloud asset search-all-resources --scope="projects/$PROJECT_ID" --format=json` | `cloudasset.googleapis.com` | `cloudasset.assets.searchAllResources` |

No habilitar esas APIs como parte del inventario local. Tras aprobación de ese cambio separado, Cloud Asset ofrece una vista amplia; las APIs de producto se necesitan solo para completar atributos específicos que el inventario de Asset no entregue. Las lecturas detalladas deben usar APIs/roles de lectura correspondientes (p. ej. Storage, IAM, Compute, Run, SQL Admin, Artifact Registry y Secret Manager) y no justifican permisos Owner adicionales.

## Costos y escenarios

USD brutos de lista, sin impuestos, descuentos, free tier, Trial credits, uso variable ni compromisos. Las tarifas consultadas el 2026-10-15 se deben revalidar en la calculadora y en SKU regionales antes de habilitar recursos. 24 h, 30 días = 720 h y mes de facturación de referencia = 730 h. Días restantes de Trial (`D`) son desconocidos: el escenario es una fórmula, no una fecha inferida.

### Subtotal fijo modelable

| Componente activo | 24 h | 30 días (720 h) | 730 h |
| --- | ---: | ---: | ---: |
| Borde global + Armor + 3 reglas | $0.86 | $25.89 | $26.25 |
| Cloud SQL N4 Enterprise, 2 vCPU + 4 GiB | $3.45 | $103.54 | $104.97 |
| Hyperdisk Balanced, 20 GiB | $0.08 | $2.27 | $2.30 |
| Cloud Run API, 1 vCPU + 1 GiB, instancia continua; tarifa base publicada | $1.73 | $51.84 | $52.56 |
| **Subtotal sin ajuste regional** | **$6.12** | **$183.53** | **$186.08** |
| Cloud Run API con sensibilidad ilustrativa +20% | $2.07 | $62.21 | $63.07 |
| **Subtotal con sensibilidad +20%** | **$6.46** | **$193.90** | **$196.59** |

El +20% de Cloud Run es solo una sensibilidad provisional; no es tarifa Tier 2 validada. El subtotal omite costos relevantes, por lo que no es “costo total” ni cotización. Para `D` días del Trial, subtotal fijo: `$6.12 × D` a `$6.46 × D`, antes de costos excluidos; si se usa margen presupuestario de 25% como escenario conservador, el extremo alto es `$8.08 × D`. A 30 días, ese margen ilustrativo deja `$242.38`, por encima de los aproximadamente USD 200 declarados por el usuario. No se deduce duración restante ni si el crédito cubriría la factura. No abrir billing pagado como resultado de esta estimación.

### Variables que faltan para total

| Elemento | Modelo de cálculo/entrada pendiente |
| --- | --- |
| ALB por bytes procesados | Entrada + salida del LB: GiB × tarifa regional publicada (referencia actual Santiago: USD 0.008/GiB en cada dirección); además, egreso de Internet aparte. |
| Internet egress | GiB por destino; referencia premium South America→South America USD 0.19/GiB para primer TiB, sujeto a origen/destino/SKU. Fórmula de ejemplo: `egress_GiB × tarifa_destino`. |
| Cloud Armor requests | Millones de requests × USD 0.75; confirmar precio/SKU aplicable. |
| Backups/PITR Cloud SQL | GiB-h de backups almacenados + WAL retenido; retención configurada de 7 días, uso real desconocido. La copia DB de ~71 MB documentada antes es baseline histórico, no lectura actual ni límite de WAL. |
| Cloud Storage de fotos | GiB-h por clase/región y días, llamadas Class A/B, egress y versiones retenidas. No hay un tamaño migrable reconciliado. |
| Artifact Registry | GiB-h almacenados (primeros 0.5 GiB/mes gratis por billing account; luego tarifa publicada), transferencia y análisis de vulnerabilidades si se activa. Tamaño/historial de imágenes desconocidos. |
| Cloud Logging | GiB ingeridos; la tarifa publicada es USD 0.50/GiB con 50 GiB/proyecto/mes incluidos. Volumen y retención adicional no medidos. |
| Secret Manager | Versiones activas por ubicación-hora menos 6 gratuitas por billing account, accesos sobre 10.000/mes y rotaciones; versiones/consultas reales desconocidas. |
| Dos SSR | Instancias activas × duración y tarifa regional de request billing; min 0, max provisional 2 por servicio. No hay distribución de requests/tiempos para estimar horas activas. |
| NAT u otro egreso | Excluido: PGA para la llamada gstatic no implica Internet genérica. Si el preflight falla o se necesita otro destino, modelar NAT, procesamiento y transferencia antes de abrir servicio. |
| Mailpit VM | Fuera de GCP-02; imagen, tamaño, disco, operación y tráfico no están definidos. |

La cifra previa de $196.59/730 h es el extremo de una sensibilidad del subtotal fijo (con +20% no validado en Cloud Run), no presupuesto total. Esta nota actualiza su interpretación, no valida gasto ni reemplaza medición/cotización.

## Plan mínimo de bootstrap y state

Se ejecutó un plan local con `-refresh=false`, backend desactivado y state local vacío; resultado: `4 to add, 0 to change, 0 to destroy` (declaración de `storage.googleapis.com` y tres buckets; cero bindings IAM extra). El plan se guarda en `.local/planning/gcp/bootstrap-candidate.tfplan` con permisos 0600 y no se incluye en Git. Al no refrescar ni consultar el state remoto, **no** verifica APIs vigentes, existencia/conflicto global de nombres, ni inventario; no es plan remoto ni autorización de apply. Aún falta inventario global completo, disponibilidad de los nombres y revisión de principales. Secuencia reproducible local:

1. `cd infra/terraform/environments/testprod/bootstrap`; verificar proyecto activo explícitamente y los tres nombres únicos; mantener los operators vacíos en primer plan.
2. `terraform init -backend=false -input=false -lockfile=readonly`; `terraform validate` para checks de schema sin state remoto.
3. Con tfvars privado revisado, ejecutar `terraform plan -refresh=false -lock=false -input=false -no-color -var-file=/ruta/privada/bootstrap-candidate.tfvars -out=/ruta/privada/bootstrap-candidate.tfplan`; inspeccionar y conservar el plan local privado. `-refresh=false` evita lecturas de recursos existentes, por lo que un plan con state vacío es solo estructura candidata, no inventario. No ejecutar `apply` automáticamente ni usar backend previo/ajeno.
4. Solo con revisión del inventario, disponibilidad de nombres y aprobación explícita, el primer apply crea los tres buckets. Después planificar grants individuales, verificar IAM efectivo, y migrar bootstrap state con `terraform init -migrate-state` en paso independiente revisado.

La configuración aplicable mínima declara `storage.googleapis.com` y tres buckets con UBLA/PAP/versioning/prevent_destroy; primer plan con cero `google_storage_bucket_iam_member`. El servicio ya figuraba habilitado en una lectura del proyecto del 2026-10-15. El primer apply necesitaría credencial con permisos de creación/listado de bucket y Service Usage, y comprobación de nombres globales; no se entrega ni ejecuta un apply.

## Fuentes oficiales de tarifas

Consulta del 2026-10-15. Tarifas y condiciones pueden cambiar; recalcular antes de cualquier habilitación: [Cloud Run](https://cloud.google.com/run/pricing), [Cloud SQL](https://cloud.google.com/sql/pricing), [Cloud Load Balancing](https://cloud.google.com/load-balancing/pricing), [Cloud Armor](https://cloud.google.com/armor/pricing), [VPC/network egress](https://cloud.google.com/vpc/network-pricing), [Cloud Storage](https://cloud.google.com/storage/pricing), [Artifact Registry](https://cloud.google.com/artifact-registry/pricing), [Cloud Logging/Observability](https://cloud.google.com/products/observability/pricing), [Secret Manager](https://cloud.google.com/secret-manager/pricing).

## Siguientes cortes

- GCP-03: restauración/compatibilidad de datos y persistencia local, artefactos e identidad operativa; no usar el almacén privado existente como origen de una migración sin inventario reconciliado y autorización específica.
- GCP-04: replay, leases, reloj controlado y prueba aislada de transiciones del API. El usuario ha autorizado en su hilo de origen el piloto y posteriormente la carga de 1.200 tratos con reloj aislado en la base persistente; esa ejecución debe conservarse en un procedimiento/evidencia aparte y no forma parte de este GCP-02 ni autoriza cambios cloud.

No se declara integración cloud, aceptación de producto, carga de datos ni aprobación de gasto.
