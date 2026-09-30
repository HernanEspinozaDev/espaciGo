## Diagrama de infraestructura

La infraestructura siguiente es el diseño objetivo acordado para el backend ES2: Terraform administra los recursos de GCP y **una API modular Go** se despliega como un servicio Cloud Run. El cliente web, que se desarrollará en una fase posterior, será un consumidor separado de la API; su contenedor queda fuera del alcance del backend aquí definido. No existe evidencia de despliegue de EspaciGo [@es2cloudrunoverview; @es2terraformgcsbackend].

![Infraestructura virtual propuesta para EspaciGo](imagenes/figura-infraestructura_propuesta.png){width=6.3in} <!--#fig:es2-infraestructura--> <!--#fuente:elaboración propia a partir de la propuesta de ES1.-->

*Tabla. Recursos de diseño y evidencia para dimensionarlos.* <!--#tab:es2-recursos-infra-->

| Recurso | Función prevista | Decisión o evidencia pendiente |
| --- | --- | --- |
| Dispositivos del equipo y usuarios | Desarrollo y uso web | Inventario real de CPU, RAM, sistema operativo, conexión y navegadores; soporte de RNF-007/022 |
| Entrada HTTPS y servicio API | Acceso HTTPS al backend Go; el frontend futuro consumirá el contrato API | Dominio/certificados, CPU/RAM, concurrencia, máximos de instancias y prueba de carga |
| PostgreSQL con PostGIS | Transacciones, calendario y geodatos exigidos por RNF-038 | Servicio y versión, capacidad, acceso privado, índices, respaldos y prueba de recuperación |
| Cloud Storage privado | Imágenes, contratos y evidencias binarias; metadatos y autorización permanecen en PostgreSQL/API | Ubicación, cifrado, permisos, retención, ciclo de vida y costo |
| Outbox, Pub/Sub y BigQuery | Publicación analítica de eventos seleccionados y consulta agregada | Esquemas, duplicados, IAM, retención, costo y permisos de fila; Datastream no se usa en ES2 |
| Cloud Logging y auditoría de dominio | Diagnóstico técnico correlacionado y trazabilidad de acciones críticas | Formato de logs, minimización, exportación y mecanismo RNF-017 por probar |
| Workers goroutine | Expiración, publicación de outbox y conciliación dentro de la API Cloud Run | Facturación por instancia, mínimo 1, leases durables, reintentos idempotentes, límite de instancias/pool y pruebas de reinicio |
| Terraform, Artifact Registry y Secret Manager | State IaC, artefactos versionados y credenciales fuera de código | Bootstrap y permisos del state, retención/versionado, CI y alertas de consumo |

Los objetivos de **99,9 % mensual**, **RPO máximo de 4 horas** y **RTO máximo de 6 horas** proceden de RNF-009/010 de ES1. El diagrama no demuestra esos resultados: requieren arquitectura de respaldo, ventanas de medición, restauración ensayada y registro de incidentes. Asimismo, RNF-019 y RNF-030 fijan escalado y capacidad por verificar; sin escenarios de carga y costos no corresponde escoger CPU, memoria ni número de instancias.

Terraform mantendrá configuración separada por ambiente y estado remoto en GCS con versionado y locking. El bucket de state se prepara durante el bootstrap y recibe IAM limitado, pues contiene datos de configuración que pueden ser sensibles. Los secretos se referencian desde Secret Manager, nunca se guardan en el repositorio, imágenes o valores literales del state. Las imágenes se construyen/publican en Artifact Registry con revisión/digest identificable; cada ambiente usa cuentas de servicio con privilegio mínimo [@es2terraformgcsbackend].

Los workers internos requieren CPU fuera de la atención de solicitudes; Cloud Run permite facturación por instancia para ese patrón y mínimo de instancias, pero estas instancias tienen costo y pueden reiniciarse. Por ello el trabajo es durable en PostgreSQL, el máximo de instancias se limita y la cantidad de conexiones del pool por réplica se calcula contra el límite de Cloud SQL [@es2cloudrunbilling; @es2cloudsqlrunconnections].

La decisión de presupuesto y alertas rige desde el primer despliegue. Se configura un presupuesto por ambiente con notificaciones de umbral al **50 %, 80 % y 100 %**, además de límites de instancias, pool de base y cuotas/bytes procesados cuando estén disponibles. Las alertas de facturación informan del consumo, pero no lo interrumpen automáticamente; el control operativo requiere topes y actuación del equipo [@es2gcpbudgets].

RNF-034–036 exige portabilidad; adoptar servicios específicos de GCP requiere documentar interfaces sustituibles y comprobar la reproducción local y otro proveedor. La comparación económica de 2.1 debe incluir cómputo, base, archivos, red, registros, copias y terceros [@es2cloudrunpricing].

### Perfil presupuestario propuesto

El Anexo A ya incorpora una instancia mínima de API Go con facturación por instancia para los workers internos. El piloto supone SQL Enterprise General Purpose con 2 vCPU/8 GiB durante 730 horas, 20 GiB SSD y 20 GiB de copias, más API Go de 1 vCPU/0,5 GiB durante 730 horas. La actividad variable de Cloud Run se reserva para frontend/desbordes; incluye además balanceador, cinco reglas WAF, logs, compilación, secretos, correo y staging. La base zonal y una única instancia API mínima no constituyen HA ni demuestran RNF-009/010 [@es2cloudsqlpricing; @es2cloudrunpricing].

*Tabla. Escenarios de costo mensual de infraestructura en tarifas de Santiago.* <!--#tab:es2-infra-costos-->

| Entorno simulado | Tarifas de Santiago, USD | Presupuesto neto CLP | Necesidad de caja CLP con IVA supuesto |
| --- | ---: | ---: | ---: |
| Ensayo restringido | 24,65 | 24.650 | 29.334 |
| Piloto público | 307,72 | 307.724 | 366.192 |
| HA de referencia | 564,55 | 564.553 | 671.818 |

**Nota.** Elaboración propia: conversión supuesta de 1.000 CLP/USD sobre las tarifas públicas usadas para `southamerica-west1`, **sin provisión regional**. Cada tarifa y cantidad requiere auditoría por SKU. El 19 % adicional es reserva de caja, cuya facturación y crédito fiscal deben verificarse. Cada mes usa un perfil; no se suman las tres filas. La fila HA y el comparador documental son configuraciones distintas de carga, almacenamiento y respaldos; ninguna demuestra una capacidad operativa contratada. El piloto presupone una instancia API mínima de 1 vCPU/0,5 GiB por 730 horas; la facturación por instancia y la tasa de nivel regional 2 se verifican con SKU antes de aprobar gasto [@es2gcspricing; @es2lbpricing; @es2armorpricing].

La decisión del usuario del 23-09-2026 fija **Santiago** como región de operación. El ejercicio comparativo propio conserva precios de Iowa como control documental de la comparación entre Cloud Run y máquinas virtuales, no como presupuesto vigente [@es2cloudrunpricing].

Durante el primer año sin ventas se estiman seis meses de ensayo y seis de piloto, con **2.373.152 CLP de salida cloud** bajo el IVA supuesto. Se requiere confirmar los servicios de datos administrados, levantar el inventario físico y ejecutar las pruebas de carga y restauración en la región decidida; el diferencial frente a Compute Engine también debe incorporar operación, respaldo y seguridad comparables.

### Dimensionamiento propuesto

*Tabla. Perfiles de recursos y su justificación.* <!--#tab:es2-dimensionamiento-->

| Perfil | Ejecución | Datos y archivos | Supuesto que lo respalda | Estado |
| --- | --- | --- | --- | --- |
| Ensayo | API/worker del mismo monolito en ejecución restringida o local, sin servicio permanente; el cliente web futuro no se incluye aquí | PostgreSQL zonal y Cloud Storage privados | Tráfico interno y datos sintéticos | Arquitectura decidida; sin desplegar |
| Piloto | Una API Cloud Run con facturación por instancia, mínimo 1 de 1 vCPU/0,5 GiB para tareas internas y máximo de instancias acotado | PostgreSQL de 2 vCPU y 8 GiB con 20 GiB de SSD/respaldos y objetos según demanda | Perfil presupuestario recalculado; capacidad por medir | Arquitectura decidida; capacidad y costo por medir |
| Alta disponibilidad | Servicios con instancias mínimas y base con conmutación | Base HA con respaldos continuos | RNF-009/010 | Escenario comparativo; no es un diseño probado |

El dimensionamiento no se deduce de los objetivos. RNF-001 exige 200 usuarios concurrentes con búsqueda bajo 2 segundos y RNF-030 fija 500 usuarios concurrentes y 100 escrituras por segundo: solo una prueba de carga puede contrastar esas cifras. Hasta entonces, CPU, memoria, instancias mínimas y tamaño de base son **parámetros de escenario**, no capacidades contratadas. PT-09 y PT-10 describen esos ensayos.

Las tarifas públicas usadas para **Santiago** se aplicaron como **supuestos del presupuesto vigente**; falta cotejar SKU, volúmenes y condiciones de facturación. El comparador aritmético anterior, con Cloud SQL HA y Cloud Run mínimo 0, alcanzaba USD 384,23 en Santiago frente a USD 297,01 de otra configuración comparable en Iowa; queda como antecedente de un servicio sin worker persistente. El piloto de la tabla cuesta USD 307,72 porque usa SQL zonal y suma la instancia API mínima; la fila HA de USD 564,55 tiene otra carga y mayores cantidades de almacenamiento y respaldo, por lo que no es directamente comparable. El presupuesto dejó de usar la provisión regional del 35 %; su efecto está cuantificado en el Anexo A. La conversión a CLP conserva el supuesto de 1.000 CLP/USD y el tratamiento de IVA sigue sin verificar con comprobantes. El inventario físico del equipo —CPU, memoria, sistema operativo, conexión y navegadores de trabajo— todavía no se ha levantado.

[[PENDIENTE: confirmar los servicios de datos administrados y el inventario físico del equipo, y probar despliegue, escalado, respaldo y recuperación con evidencia fechada en la región decidida.]]
