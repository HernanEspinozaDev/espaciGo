# Modelo de costos por fase: Foundation GCP-02

Estimación de lista para el footprint Foundation únicamente, sin impuestos, descuento contractual, crédito Trial ni cálculo de saldo. Es un modelo previo: no es factura, cotización, plan Terraform ni autorización de gasto. No se verificó Billing Overview; Free Trial es una declaración del usuario y no se calcula el crédito restante.

## Alcance de la fase

Contempla habilitación de las 13 APIs, VPC/subred, cinco cuentas de servicio, pool/provider WIF, bindings IAM previstos, repositorio Artifact Registry vacío, bucket privado de archivos vacío y tres contenedores de Secret Manager sin versiones. El [manifest estático](foundation_manifest_gcp02.json) enumera 41 instancias esperadas bajo sus supuestos; no es un plan guardado. Cualquier plan real puede diferir y debe revisarse antes de la aplicación.

| Componente en reposo | Modelo de costo | Consideración |
| --- | --- | --- |
| APIs, cuentas de servicio, WIF, IAM, red/subred | Sin cargo mensual fijo de recurso identificado en el modelo | Las APIs habilitadas no suponen crear sus productos; el uso posterior sí puede facturarse. |
| Artifact Registry vacío | Sin almacenamiento mientras no haya bytes; los primeros 0,5 GiB/mes por cuenta de facturación son gratuitos; excedente aproximado USD 0,000136986/GiB-h | Capas/imágenes, escaneo y transferencia quedan fuera. [Tarifas oficiales](https://cloud.google.com/artifact-registry/pricing?hl=es-419). |
| Bucket Standard Santiago vacío | Sin capacidad mientras esté vacío. Almacenamiento Standard aprox. USD 0,000027397/GiB-h (USD 0,02/GiB-mes); operaciones A USD 0,005/1.000 y B USD 0,0004/1.000 | Versionado está activo y no hay lifecycle configurado: generaciones anteriores pueden acumular bytes facturables. Egreso/recuperación se excluyen. [Tarifas oficiales](https://cloud.google.com/storage/pricing?hl=es-419). |
| Tres secretos vacíos, cero versiones activas | USD 0 por versiones mientras no se carguen valores; operaciones de administración gratuitas | Primeras 6 versiones activas/account gratis; excedente aprox. USD 0,000082192/versión-h y accesos por encima de 10.000: USD 0,03/10.000. [Tarifas oficiales](https://cloud.google.com/secret-manager/pricing). |

**Piso ilustrativo del Foundation vacío: USD 0/mes**, bajo supuestos de ningún byte, versión, acceso facturable ni consumo compartido excedente. No incluye costos de uso de APIs/produtos habilitados ni atribuye créditos. No se debe usar como cotización del entorno completo.

## Fuera de esta fase

SQL/backup/PITR, Cloud Run y su CPU/memoria, balanceador/Armor, DNS, egreso, logs, imágenes, bytes/versiones reales, operación de aplicación y cualquier costo de APIs activadas para consumos futuros están excluidos. El modelo integral por escenarios de 24 h/30 días/duración restante está en [modelo Terraform](../../infra/terraform/cost-model.md); completar calculadora oficial regional y revisar SKU actuales antes de habilitar Foundation. No se autorizó cambiar Paid Billing ni aplicar recursos.
