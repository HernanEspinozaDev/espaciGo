# Estimación de costos GCP-02

Estimación bruta de lista en USD antes de impuestos, créditos Free Trial, descuentos, CUD u otras condiciones de cuenta; tarifas oficiales consultadas el 2026-10-16 para Santiago (`southamerica-west1`). El usuario confirmó que Free Trial está activo y Paid mode no se habilitó; no se verificaron crédito disponible ni fecha de expiración. No es cotización ni autorización de gasto. Se distinguen 24 h, 30 días (720 h), mes de referencia (730 h) y `D` días de Trial restantes, que no se estiman.

## Buckets de state creados en GCP-02

El 2026-10-16 se crearon tres buckets regionales Standard sin objetos: `espacigo-testprod-tfstate-447373903153-{bootstrap,gcp,cloudflare}`. No se agregaron bindings IAM. Como aún están vacíos, no tienen costo de capacidad; las operaciones de verificación ocurrieron y su costo nominal no se estima aquí.

Escenario operativo explícito (no observado): tres state files de 100 KiB cada uno, cuatro escrituras por día y archivo durante 30 días, hasta 120 generaciones por bucket por versioning, diez lecturas por archivo y día. Aproximando todas las generaciones retenidas como si existieran todo el mes: (35.2/1024) GiB retenidos × $0.000027397/GiB-h × 730 h ≈ **$0.0007/mes**; 360 operaciones Class A × $0.005/1.000 = **$0.0018**; 900 Class B × $0.0004/1.000 = **$0.00036**. Total aproximado: **$0.003/mes**, sin egreso. Es casi cero para estos tamaños, pero la configuración de versioning no limita generaciones: crecimiento y recuperación de state requieren acordar retención/lifecycle antes de uso sostenido. La verificación de nombres y API no cambia recursos facturables.

## Piso continuo del entorno propuesto

El corte no activa estos componentes. Esta tabla muestra el piso futuro si se habilitan con los tamaños Terraform actuales. Se separan los modelos: API con instancia continua a tarifas de instance-based; SSR con facturación request-based y solo horas activas asumidas más abajo. Para Santiago Tier 2, se publica el rate base y una sensibilidad +20%; este margen no sustituye la verificación del SKU/facturación regional.

| Componente | 24 h | 720 h | 730 h |
| --- | ---: | ---: | ---: |
| Balanceador externo global, una regla de forwarding ($0.025/h) | $0.60 | $18.00 | $18.25 |
| Armor: política global + 3 reglas configuradas | $0.26 | $7.89 | $8.00 |
| Cloud SQL Enterprise N4 zonal, 2 vCPU/4 GiB | $3.72 | $111.48 | $113.03 |
| Hyperdisk Balanced, 20 GiB, capacidad | $0.11 | $3.18 | $3.22 |
| Hyperdisk Balanced, IOPS predeterminadas 4.000 (1.000 sobre 3.000 incluidas) | $0.92 | $27.62 | $27.99 |
| Hyperdisk Balanced, throughput predeterminado 170 MiB/s (30 sobre 140 incluidos) | $0.11 | $3.31 | $3.36 |
| Cloud Run API, instance-based, 1 vCPU/1 GiB continuo; rate base publicado | $1.73 | $51.84 | $52.56 |
| Cloud Run API con sensibilidad regional +20% | $2.07 | $62.21 | $63.07 |
| **Piso usando rate base** | **$7.44** | **$223.32** | **$226.42** |
| **Piso con sensibilidad +20% solo para API** | **$7.79** | **$233.69** | **$236.94** |

Fórmulas Santiago N4 Enterprise: SQL compute+RAM `(2×$0.05782 + 4×$0.0098) × horas`; Hyperdisk Balanced `20×$0.000220548×horas` de capacidad, 1.000 IOPS sobre el umbral incluido `×$0.000038356×horas`, y 30 MiB/s de throughput sobre el umbral incluido `×$0.000153425×horas`; API instance-based `($0.000018 + $0.000002) × segundos`; sensibilidad multiplica solo API por 1.2. Armor usa $0.006849315/h por política y $0.001369863/h por regla; balanceador global usa $0.025/h. Hyperdisk de 20–256 GiB incluye por defecto 4.000 IOPS/170 MiB/s; el baseline facturable considera 3.000 IOPS/140 MiB/s. Con el límite Terraform de 100 GiB, SQL más disco cuesta $5.28/día de 24 h, $158.29/720 h y $160.49/730 h; el piso total sería $7.87/$236.02/$239.30 (o $8.21/$246.39/$249.82 con sensibilidad API). El plan SQL habilita respaldos automáticos, PITR 7 días y conserva 7 backups: respaldo usado, WAL y egress SQL siguen variables. No se modela `7 × tamaño del disco`: los backups automáticos son incrementales. También se excluyen APIs gratuitas, VPC/Private Google Access, IP interna, secretos vacíos, almacenamiento de archivos y DNS.

## Escenarios de uso (entradas deliberadas, no métricas reales)

Para comparar sin esperar a tener telemetría, se usan supuestos editables. En cada escenario: 1.000 horas acumuladas de instancia activa SSR entre los dos servicios; salida pública de 100 GiB hacia Sudamérica; 200 GiB procesados por el balanceador (100 GiB en cada dirección); 10 millones de requests evaluados por Armor; 20 GiB de archivos Standard; 2 GiB en Artifact Registry; 10 GiB de logs; 20 GiB promedio de backups SQL. Las horas SSR son horas facturables, no horas del reloj, y los servicios tienen min 0/max 2. El monto de salida de 100 GiB usa la tarifa publicada de $0.19/GiB y no presupone cache.

| Variable mensual | Cálculo de referencia | Costo aproximado |
| --- | --- | ---: |
| SSR request-based, horas activas | 1.000 h combinadas × 1 vCPU/1 GiB × ($0.000024 CPU + $0.0000025 memoria)/s × 3.600 s/h | $95.40 |
| SSR request-based, sensibilidad Tier 2 +20% | 1.000 h combinadas × ($0.0000288 CPU + $0.000003 memoria)/s × 3.600 s/h | $114.48 |
| Requests SSR | Hasta 1 M requests a $0.40/M; el rate publicado incluye 2 M requests gratuitos mensuales según condiciones de cuenta | $0–$0.40 |
| Egress a Sudamérica | 100 GiB × $0.19/GiB | $19.00 |
| Procesamiento del balanceador | 200 GiB × $0.008/GiB | $1.60 |
| Cloud Armor requests | 10 M × $0.75/M | $7.50 |
| Fotos | 20 GiB-h × $0.000027397 × 730 h | $0.40 |
| Artifact Registry | (2−0.5) GiB × $0.000136986 × 730 h | $0.15 |
| Logging | 10 GiB ingeridos; dentro de 50 GiB/proyecto/mes incluidos | $0* |
| Secret Manager | 3 versiones activas; por debajo de 6 gratuitas, más 50.000 accesos y 10.000 incluidos | $0.12 |
| Backups SQL | 20 GiB usados × $0.000153425 × 730 h | $2.24 |

Composición reproducible (tarifa base): piso continuo `$226.42` + SSR activo `$95.40` + las otras variables `$31.01–$31.41` = **$352.83–$353.23/mes**. Escenario sensible con +20% en API y SSR: piso `$236.94` + SSR `$114.48` + otras variables `$31.01–$31.41` = **$382.43–$382.83/mes**. Ambos escenarios usan las mismas entradas deliberadas y excluyen salidas SSR/API adicionales, crecimiento/retención de versiones de fotos, PITR/WAL por encima del respaldo asumido, operaciones de storage, auditoría/logs sobre free allowance y otros SKU. El piso ya incluye IOPS/throughput predeterminados de Hyperdisk; no son un extra opcional con la configuración actual. No son pronósticos de uso. Para `D` días, multiplica tarifas horarias por `24×D` y reestima componentes por solicitud/GB según el periodo; no se deduce cuántos días de Trial quedan ni si el saldo declarado de ~USD 200 es vigente.

## Variables que se deben medir o decidir antes de activar SQL/Run

- Horas SSR activas, tamaño CPU/memoria, requests, concurrencia y cargas de SSR; usar `container/billable_instance_time` una vez exista tráfico. Hacer un escenario 0/1.000/4.000 horas combinadas y verificar rate Tier 2 exacto en SKU/Calculadora antes de abrir el gate de `enable_services`.
- Volumen egress por destino, tráfico entrante/saliente de balanceador y requests Armor. Los datos de salida a Sudamérica cuestan $0.19/GiB en el primer TiB de cada SKU/periodo publicado; el procesamiento de balanceador va aparte.
- Cloud SQL backups/PITR, WAL, crecimiento de disco y solicitudes de I/O. Tarifas oficiales Santiago consultadas el 2026-10-16: backup usado $0.000153425/GiB-h; Hyperdisk Balanced capacidad $0.000220548/GiB-h, IOPS $0.000038356/h, throughput $0.000153425/h. El SKU configurado de 20 GiB incluye 4.000 IOPS/170 MiB/s; contra los umbrales facturables 3.000/140, el piso incorpora 1.000 IOPS y 30 MiB/s. Confirmar capacidad y rendimiento reales en facturación tras despliegue.
- Fotos: el inventario local no es un tamaño migrable reconciliado. Aplicar Standard `0.000027397/GiB-h`; versioning retiene generaciones, salida y operaciones se cobran aparte. Establecer política de retención antes de cargar datos.
- Artifact Registry: primer 0.5 GiB gratis y luego $0.000136986/GiB-h, sujeto a cuota de cuenta. Logs: 50 GiB de ingestión mensual incluidos, luego $0.50/GiB; retención extendida puede costar aparte. Secret Manager: seis versiones activas gratuitas y 10.000 accesos gratuitos según condiciones de cuenta.
- Mailpit VM no está incluido; imagen, tipo de VM, disco, administración y egress deben tener estimación separada.
- Private Google Access es solo para servicios Google elegibles; no se incluyó Cloud NAT. Si el preflight gstatic u otro egreso falla, mantener gates cerrados y cotizar la ruta antes de habilitar servicios.

Referencias oficiales, consultadas el 2026-10-16: [Cloud Run](https://cloud.google.com/run/pricing), [Cloud SQL pricing](https://cloud.google.com/sql/pricing), [Cloud SQL Hyperdisk](https://docs.cloud.google.com/sql/docs/postgres/storage-options-overview), [Load Balancing](https://cloud.google.com/load-balancing/pricing), [Cloud Armor](https://cloud.google.com/armor/pricing), [network/egress](https://cloud.google.com/vpc/network-pricing), [Cloud Storage](https://cloud.google.com/storage/pricing), [Artifact Registry](https://cloud.google.com/artifact-registry/pricing), [Cloud Logging](https://cloud.google.com/stackdriver/pricing), [Secret Manager](https://cloud.google.com/secret-manager/pricing).

\* La cuota se comparte según límites oficiales del producto/cuenta; no se ha verificado saldo ni uso concurrente de la cuenta.
