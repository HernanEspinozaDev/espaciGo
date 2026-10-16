# Estimación de costos GCP-02

Precios públicos consultados el 2026-10-15. USD, 730 horas/mes; sin impuestos, créditos, ni condiciones particulares de la cuenta. Esto no es una cotización ni aprobación de gasto.

| Componente | Supuesto | Referencia mensual |
| --- | --- | ---: |
| Application Load Balancer global | Una regla global de forwarding, $0.025/h | $18.25 |
| Cloud Armor Standard | Una política global, $0.006849315/h | $5.00 |
| Cloud Armor | Tres reglas configuradas, $0.001369863/h por regla | $3.00 |
| Procesamiento de ALB en Santiago | $0.008/GiB entrada y $0.008/GiB salida | Variable por tráfico |
| Solicitudes Cloud Armor | $0.75 por millón | Variable por tráfico |

El subtotal de borde es **aprox. $26.25/mes** (unos $0.86 por día promedio), antes de tráfico. Se cuentan tres reglas porque son tres bloques Terraform configurados, incluido el allow terminal. Se usan 730 h para el mes de referencia; 30 días exactos son 720 h. Este subtotal excluye el costo continuo de CPU/memoria de la API y cualquier NAT.

## Piso fijo indicativo (Santiago, sin descuentos ni crédito)

El siguiente piso añade los componentes con una tarifa pública utilizable y configuración ya fijada. Es una referencia de lista bruta mensual, no un total de factura ni aprobación de gasto:

| Componente | Cálculo a 730 h | Estimación |
| --- | --- | ---: |
| Borde | Una regla global + Cloud Armor y tres reglas, según tabla anterior | $26.25 |
| Cloud SQL N4 Enterprise zonal | 2 vCPU × $0.0537/h + 4 GiB × $0.0091/GiB-h | $104.97 |
| Hyperdisk Balanced de Cloud SQL | 20 GiB × $0.000157534/GiB-h | $2.30 |
| Cloud Run API | 1 vCPU + 1 GiB, una instancia continua; tarifa Santiago Tier 2 modelada como 1.2× tarifa base | $63.07 |
| **Piso fijo indicativo** | Sin tráfico, backups, logs, almacenamiento de archivos ni frontend | **$196.59/mes** |

El costo real de Cloud SQL incluye crecimiento del disco hasta el límite de 100 GiB, almacenamiento de backups y registros PITR según uso y SKU regional. No se fija un volumen de fotos ni retención efectiva, así que Cloud Storage, operaciones, versionado, salida, Artifact Registry y Logging siguen como variables. El Cloud Run API usa `cpu_idle=false`, `min_instance_count=1`, 1 vCPU y 1 GiB; Santiago está en la tabla Tier 2 y la cifra aplica un factor regional provisional de 1.2 a la tarifa publicada para billing por instancia. Confirmar las tarifas efectivas y el crédito Free Trial en la calculadora/cuenta antes de abrir cualquier gate. Las fuentes oficiales describen los rates de Cloud Run y tiers regionales, y las tarifas regionales de Cloud SQL N4/Hyperdisk: [Cloud Run](https://cloud.google.com/run/pricing), [Cloud SQL](https://cloud.google.com/sql/pricing).

El piso sí suma Cloud SQL y Cloud Run API con los supuestos mostrados arriba; no suma Cloud Storage, backups PITR, Artifact Registry, logging, secretos, Mailpit, procesamiento/egress de red ni el costo de los dos SSR (inician en cero y máximo provisional 2). Cloud SQL se modela como N4 zonal 2 vCPU/4 GiB con Hyperdisk Balanced, 20 GiB inicial y máximo 100 GiB, incluyendo IOPS/throughput y PITR que aún requieren confirmación del SKU y consumo. Primero medir GCP-04 (replay/leases y CPU de worker).

Los SSR envían todo su tráfico por Direct VPC egress para llamar al API con ingreso interno. Además, el SSR privado descarga en ejecución la clave IAP desde `https://www.gstatic.com/iap/verify/public_key`. Google incluye los archivos servidos desde `*.gstatic.com` entre los destinos elegibles de Private Google Access; la topología propone PGA para esta llamada concreta y no incluye Cloud NAT. Antes de habilitar servicios, `confirm_pga_iap_key_preflight` exige una prueba real de egreso hacia esa URL desde la ruta/subred prevista. Si el preflight no funciona, no activar el servicio privado: mantener el gate cerrado y cotizar NAT o una ruta alternativa. Para cualquier otro destino de Internet se requiere estimar NAT/rutas y salida; PGA no se trata como Internet genérica. Referencias: [opciones de acceso privado para servicios](https://docs.cloud.google.com/vpc/docs/private-access-options) y [Private Google Access](https://docs.cloud.google.com/vpc/docs/private-google-access).

Antes de habilitar recursos, completar la calculadora oficial para 24 h, 30 días y duración real restante. Incluir tráfico/tamaños de fotos, mínimos/máximos de instancias, backups/PITR, crecimiento de disco, Cloud Armor, tráfico de red, retención de logs, builds/registry, Cloud Storage/versiones y correo. Este subtotal no representa el costo total del entorno.

Fuentes oficiales: [Cloud Load Balancing pricing](https://cloud.google.com/load-balancing/pricing), [Cloud Armor pricing](https://cloud.google.com/armor/pricing), [Cloud SQL pricing](https://cloud.google.com/sql/pricing), [Cloud Run pricing](https://cloud.google.com/run/pricing), [Cloud SQL machine series](https://docs.cloud.google.com/sql/docs/postgres/machine-series-overview).
