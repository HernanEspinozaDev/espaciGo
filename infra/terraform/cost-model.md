# Estimación de costos GCP-02

Estimación de lista en USD, precios oficiales consultados el 2026-10-16, región principal Santiago (`southamerica-west1`), sin impuestos, Trial, descuentos, CUD ni condiciones particulares de cuenta. No es cotización ni autorización de gasto. Se distinguen 24 h, 30 días (720 h), mes de referencia (730 h) y `D` días restantes de Trial, que no está verificado.

## Tres buckets de state del plan GCP-02

El plan preparado propone crear tres buckets regionales Standard sin objetos. Mientras estén vacíos, el costo de almacenamiento y operaciones modelado es USD 0; la disponibilidad global solo es comprobable atómicamente al crear, aunque la verificación actual devolvió 404 para cada nombre y no listó buckets existentes en el proyecto. No se propuso ningún binding IAM.

Escenario operativo explícito (no observado): tres state files de 100 KiB cada uno, cuatro escrituras por día y archivo durante 30 días, hasta 120 generaciones por bucket por versioning, diez lecturas por archivo y día. Aproximando todas las generaciones retenidas como si existieran todo el mes: (35.2/1024) GiB retenidos × $0.000027397/GiB-h × 730 h ≈ **$0.0007/mes**; 360 operaciones Class A × $0.005/1.000 = **$0.0018**; 900 Class B × $0.0004/1.000 = **$0.00036**. Total aproximado: **$0.003/mes**, sin egreso. Es casi cero para estos tamaños, pero la configuración de versioning no limita generaciones: crecimiento y recuperación de state requieren acordar retención/lifecycle antes de uso sostenido. La verificación de nombres y API no cambia recursos facturables.

## Piso continuo del entorno propuesto

El corte no activa estos componentes. Esta tabla muestra el piso futuro si se habilitan con los tamaños Terraform actuales; el API Cloud Run se modela como CPU siempre asignada y una instancia durante todo el tiempo. Para Cloud Run Tier 2, se publica el rate base y una sensibilidad +20% porque la página oficial clasifica Santiago Tier 2, pero la cifra regional efectiva depende del SKU/facturación. No se conserva el antiguo subtotal de $196.59 como rate comprobado.

| Componente | 24 h | 720 h | 730 h |
| --- | ---: | ---: | ---: |
| Balanceador externo global, una regla de forwarding ($0.025/h) | $0.60 | $18.00 | $18.25 |
| Armor: política global + 3 reglas configuradas | $0.26 | $7.89 | $8.00 |
| Cloud SQL Enterprise N4 zonal, 2 vCPU/4 GiB | $3.45 | $103.54 | $104.97 |
| Hyperdisk Balanced, 20 GiB (sin IOPS/throughput adicionales) | $0.08 | $2.27 | $2.30 |
| Cloud Run API 1 vCPU/1 GiB, una instancia siempre asignada; rate base publicado | $2.29 | $68.69 | $69.64 |
| Cloud Run API con sensibilidad regional +20% (no tarifa confirmada) | $2.75 | $82.43 | $83.57 |
| **Piso usando rate base** | **$6.68** | **$200.39** | **$203.16** |
| **Piso con sensibilidad +20% para Cloud Run** | **$7.14** | **$214.13** | **$217.09** |

Fórmulas: SQL `(2×$0.0537 + 4×$0.0091) × horas`; Hyperdisk `20×$0.000157534×horas`; API rate base `($0.000024 + $0.0000025) × segundos`; sensibilidad multiplica solo Cloud Run por 1.2. Armor usa $0.006849315/h por política y $0.001369863/h por regla; balanceador global usa $0.025/h. El plan SQL habilita respaldos automáticos, PITR 7 días y conserva 7 backups: esos almacenamientos, crecimiento del disco hasta 100 GiB, IOPS/throughput, transacciones y egress SQL no están en el piso. También se excluyen APIs gratuitas, VPC/Private Google Access, IP interna, secretos vacíos, Cloud Storage vacío y DNS.

## Escenarios de uso (entradas deliberadas, no métricas reales)

Para comparar sin esperar a tener telemetría, se usan supuestos editables. En cada escenario: 1.000 horas acumuladas de instancia activa SSR entre los dos servicios; salida pública de 100 GiB hacia Sudamérica; 200 GiB procesados por el balanceador (100 GiB en cada dirección); 10 millones de requests evaluados por Armor; 20 GiB de archivos Standard; 2 GiB en Artifact Registry; 10 GiB de logs; 20 GiB promedio de backups SQL. Las horas SSR son horas facturables, no horas del reloj, y los servicios tienen min 0/max 2. El monto de salida de 100 GiB usa la tarifa publicada de $0.19/GiB y no presupone cache.

| Variable mensual | Cálculo de referencia | Costo aproximado |
| --- | --- | ---: |
| SSR request-based | 1.000 h × 1 vCPU/1 GiB × rates Tier 2 de referencia ($0.0000288 CPU + $0.000003 memoria)/h | $114.48 |
| Requests SSR | Hasta 1 M requests a $0.40/M; el rate publicado incluye 2 M requests gratuitos mensuales según condiciones de cuenta | $0–$0.40 |
| Egress a Sudamérica | 100 GiB × $0.19/GiB | $19.00 |
| Procesamiento del balanceador | 200 GiB × $0.008/GiB | $1.60 |
| Cloud Armor requests | 10 M × $0.75/M | $7.50 |
| Fotos | 20 GiB-h × $0.000027397 × 730 h | $0.40 |
| Artifact Registry | (2−0.5) GiB × $0.000136986 × 730 h | $0.15 |
| Logging | 10 GiB ingeridos; dentro de 50 GiB/proyecto/mes incluidos | $0* |
| Secret Manager | 3 versiones activas; por debajo de 6 gratuitas, más 50.000 accesos y 10.000 incluidos | $0.12 |
| Backups SQL | 20 GiB usados × $0.000109589 × 730 h | $1.60 |

Sumado al piso continuo: **aprox. $348–$349/mes** con esos supuestos, antes de salidas SSR/API adicionales, crecimiento/retención de versiones de fotos, PITR/WAL por encima del respaldo asumido, operaciones de storage, auditoría/logs sobre free allowance, SQL IOPS/throughput y otros SKU. El escenario solo ilustra cómo influyen variables; no pronostica uso del proyecto. Para `D` días, multiplica tarifas horarias por `24×D` y reestima componentes por solicitud/GB según el periodo; no se deduce cuántos días de Trial quedan ni si el saldo declarado de ~USD 200 es vigente. El piso mensual calculado ya puede superar ese saldo si el ambiente permanece encendido.

## Variables que se deben medir o decidir antes de activar SQL/Run

- Horas SSR activas, tamaño CPU/memoria, requests, concurrencia y cargas de SSR; usar `container/billable_instance_time` una vez exista tráfico. Hacer un escenario 0/1.000/4.000 horas combinadas y verificar rate Tier 2 exacto en SKU/Calculadora antes de abrir el gate de `enable_services`.
- Volumen egress por destino, tráfico entrante/saliente de balanceador y requests Armor. Los datos de salida a Sudamérica cuestan $0.19/GiB en el primer TiB de cada SKU/periodo publicado; el procesamiento de balanceador va aparte.
- Cloud SQL backups/PITR, WAL, crecimiento de disco y solicitudes de I/O. Los precios publicados para Santiago: backup usado $0.000109589/GiB-h; Hyperdisk Balanced capacidad $0.000157534/GiB-h, IOPS $0.000027397/h, throughput $0.000109589/h. Confirmar capacidad provisionada y SKU del proveedor antes de desplegar.
- Fotos: el inventario local no es un tamaño migrable reconciliado. Aplicar Standard `0.000027397/GiB-h`; versioning retiene generaciones, salida y operaciones se cobran aparte. Establecer política de retención antes de cargar datos.
- Artifact Registry: primer 0.5 GiB gratis y luego $0.000136986/GiB-h, sujeto a cuota de cuenta. Logs: 50 GiB de ingestión mensual incluidos, luego $0.50/GiB; retención extendida puede costar aparte. Secret Manager: seis versiones activas gratuitas y 10.000 accesos gratuitos según condiciones de cuenta.
- Mailpit VM no está incluido; imagen, tipo de VM, disco, administración y egress deben tener estimación separada.
- Private Google Access es solo para servicios Google elegibles; no se incluyó Cloud NAT. Si el preflight gstatic u otro egreso falla, mantener gates cerrados y cotizar la ruta antes de habilitar servicios.

Referencias oficiales, consultadas el 2026-10-16: [Cloud Run](https://cloud.google.com/run/pricing), [Cloud SQL](https://cloud.google.com/sql/pricing), [Load Balancing](https://cloud.google.com/load-balancing/pricing), [Cloud Armor](https://cloud.google.com/armor/pricing), [network/egress](https://cloud.google.com/vpc/network-pricing), [Cloud Storage](https://cloud.google.com/storage/pricing), [Artifact Registry](https://cloud.google.com/artifact-registry/pricing), [Cloud Logging](https://cloud.google.com/stackdriver/pricing), [Secret Manager](https://cloud.google.com/secret-manager/pricing).

\* La cuota se comparte según límites oficiales del producto/cuenta; no se ha verificado saldo ni uso concurrente de la cuenta.
