# Estimación de costos GCP-02

Precios públicos consultados el 2026-10-15. USD, 730 horas/mes; sin impuestos, créditos, ni condiciones particulares de la cuenta. Esto no es una cotización ni aprobación de gasto.

| Componente | Supuesto | Referencia mensual |
| --- | --- | ---: |
| Application Load Balancer global | Una regla global de forwarding, $0.025/h | $18.25 |
| Cloud Armor Standard | Una política global, $0.006849315/h | $5.00 |
| Cloud Armor | Tres reglas configuradas, $0.001369863/h por regla | $3.00 |
| Procesamiento de ALB en Santiago | $0.008/GiB entrada y $0.008/GiB salida | Variable por tráfico |
| Solicitudes Cloud Armor | $0.75 por millón | Variable por tráfico |

El subtotal de borde es **aprox. $26.25/mes** (unos $0.86 por día promedio), antes de tráfico. Se cuentan tres reglas porque son tres bloques Terraform configurados, incluido el allow terminal. Se usan 730 h para el mes de referencia; 30 días exactos son 720 h. Las tarifas deben verificarse en la calculadora al congelar el inventario. Este subtotal excluye el costo continuo de CPU/memoria de la API y cualquier NAT.

No se suma Cloud SQL, Cloud Run, almacenamiento, backups, Artifact Registry, logging, secretos, Mailpit ni salida a Internet: falta fijar consumo y tarifas para el SKU/región y el plan de operación. Cloud SQL se modela como N4 zonal 2 vCPU/4 GiB con Hyperdisk Balanced, 20 GiB inicial y máximo 100 GiB, incluyendo IOPS/throughput y PITR. Cloud Run API mantiene una instancia mínima con CPU siempre asignada (`cpu_idle=false`), de modo que cómputo y memoria corren continuamente mientras exista esa instancia. Cotizarla con tarifa/tiers regionales, no con request-based billing. Primero medir GCP-04 (replay/leases y CPU de worker); los dos SSR parten de cero con máximo provisional 2.

Los SSR envían todo su tráfico por Direct VPC egress para llamar al API con ingreso interno. Private Google Access está habilitado en la subred; Cloud NAT no forma parte de esta raíz. Si el runtime requiere otro destino público, estimar NAT/rutas y salida antes de permitir esa dependencia.

Antes de habilitar recursos, completar la calculadora oficial para 24 h, 30 días y duración real restante. Incluir tráfico/tamaños de fotos, mínimos/máximos de instancias, backups/PITR, crecimiento de disco, Cloud Armor, tráfico de red, retención de logs, builds/registry, Cloud Storage/versiones y correo. Este subtotal no representa el costo total del entorno.

Fuentes oficiales: [Cloud Load Balancing pricing](https://cloud.google.com/load-balancing/pricing), [Cloud Armor pricing](https://cloud.google.com/armor/pricing), [Cloud SQL pricing](https://cloud.google.com/sql/pricing), [Cloud Run pricing](https://cloud.google.com/run/pricing), [Cloud SQL machine series](https://docs.cloud.google.com/sql/docs/postgres/machine-series-overview).
