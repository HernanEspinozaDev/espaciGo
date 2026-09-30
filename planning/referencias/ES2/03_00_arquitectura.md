# Detalle de la arquitectura a implementar

La arquitectura se desarrollará a partir de los módulos, requisitos y flujos definidos en ES1. Las vistas de procesos, casos de uso, componentes, datos e infraestructura deberán describir un mismo alcance y conservar los identificadores de la base.

## Decisiones consolidadas

*Tabla. Decisiones técnicas de ES2 y su estado.* <!--#tab:es2-decisiones-arquitectura-->

| Decisión | Origen | Estado | Dónde se detalla |
| --- | --- | --- | --- |
| Backend modular Go en un servicio API Cloud Run | ES1 y definición consolidada de backend ES2 | Decisión arquitectónica; una aplicación desplegable con módulos internos, sin microservicios en esta fase | 3.3, 3.6 |
| PostgreSQL/PostGIS como persistencia operacional | RNF-038 de ES1 y definición consolidada de backend ES2 | Decisión arquitectónica; fuente de verdad para el estado transaccional | 3.4, 3.6 |
| Terraform para infraestructura GCP | Definición consolidada de backend ES2 | Decisión arquitectónica; state remoto GCS versionado y con locking | 3.6 |
| Cloud Storage privado para imágenes, contratos y evidencias | Definición consolidada de backend ES2 | Decisión arquitectónica; autorización del backend y acceso temporal | 3.3, 3.6 |
| Región southamerica-west1 (Santiago) para datos y ejecución | Decisión del usuario del 23-09-2026 | Vigente: tarifas publicadas aplicadas al presupuesto; +40 % en cómputo, base y Cloud Run, y +90 % en almacenamiento respecto de Iowa | 3.6; Anexo A |
| Alcance del producto y modelo | Decisión posterior del usuario | Todas las categorías y flujos de ES1 se diseñan desde ahora; construcción incremental sin limitar el producto a una demo de categoría única | 3.3, 3.4, 3.7; Anexo B |
| Pagos externos durante desarrollo e integración | Decisión del usuario | Exclusivamente sandbox; el simulador local es solo para pruebas aisladas y no acredita cobros reales | 3.3, 3.5 |
| Analítica de eventos de dominio | Definición consolidada de backend ES2 | Outbox PostgreSQL → Pub/Sub → BigQuery; Datastream descartado en esta fase | 3.3, 3.4, 3.5, 3.6 |
| Workers asíncronos | Decisión del usuario | Goroutines dentro del mismo servicio Cloud Run, con tareas durables y reclamo idempotente en PostgreSQL; sin Cloud Run Job aparte | 3.3, 3.6 |
| Presupuesto y alertas de GCP | Decisión del usuario | Configurados desde el primer despliegue junto con topes operativos; las alertas avisan y no cortan automáticamente el gasto | 3.6 |
| Calendario común `ocupacion` con intervalos semiabiertos | Decisión de diseño ES2 | Contrato lógico fijado en el Anexo B; requiere migración aplicada y prueba concurrente | 3.4; Anexo B |
| Separar la analítica de la garantía de inmutabilidad de RNF-017 | Decisión nueva de ES2 | Propuesta (SUP-13): retención bloqueada con hash por lote; sin ensayo de alteración ejecutado | 3.3, 3.6 |
| Ley 21.719 como criterio de diseño desde el primer incremento | Decisión del usuario | Vigente; sin cumplimiento probado | Anexo B; PT-16 |
| Métricas de publicaciones y acceso premium | Decisión consolidada de backend | Captura agregada de impresiones y clics para todas las publicaciones; el reporte requiere ticket premium vigente y autorización del arrendador | 3.3, 3.5 |
| Panel de métricas de prueba | Definición consolidada de backend ES2 | Looker Studio conectado a vistas BigQuery con identidad y acceso por fila verificados; limitado a vendedores de prueba | 3.3 |
| Selección de proveedor de pagos/firma/identidad | Decisión de separación por alcance | Contratos neutrales en el backend; Mercado Pago y demás proveedores se documentan en investigaciones/anexos técnicos independientes | 3.3, 3.5 |

## Coherencia entre las vistas

*Tabla. Correspondencia de elementos entre las vistas.* <!--#tab:es2-coherencia-vistas-->

| Elemento | Vistas donde aparece | Observación |
| --- | --- | --- |
| Módulos M01–M11 | 3.1, 3.2, 3.3, 3.7 | Se conservan los mismos identificadores; no se crean módulos nuevos |
| Componentes de software | 3.3, 3.5, 3.7 | Web, API modular, adaptadores y conciliación se nombran igual en las tres vistas |
| Entidades y calendario | 3.4, 3.5, 3.7 | La ocupación y las operaciones financieras del Anexo B sustentan los flujos descritos |
| Canales y webhooks | 3.5, 3.7 | El webhook se trata como evento asincrónico, no como respuesta de pago |
| Recursos de ejecución | 3.6, 3.7 | Los perfiles presupuestarios no son recursos desplegados |
| Proveedores externos | 3.2, 3.3, 3.5, 3.7 | Pago, firma e identidad siguen sin acceso ni contrato confirmado |
| Presupuesto de infraestructura | 3.6 | Tarifas de Santiago aplicadas al Anexo A por decisión del usuario; el ejercicio de Iowa se conserva solo como control documental |

La propuesta única de backend, su alcance y decisiones se detalla en [03.3](03_03_componentes.md) y en la [propuesta base del backend](../investigacion/propuesta_backend_final.md). Las vistas describen una arquitectura acordada para construir; no acreditan despliegue, integración ni resultado de prueba. La investigación específica de Mercado Pago permanece separada y no altera el contrato proveedor-neutral.

## Mecanismo propuesto para la inmutabilidad de RNF-017

La documentación de BigQuery admite operaciones `UPDATE` y `DELETE`, de modo que conservarlo como repositorio analítico no satisface por sí solo el requisito de inmutabilidad. La propuesta de trabajo (SUP-13) separa las dos funciones: BigQuery se mantiene para consulta y análisis, y la garantía se apoya en **Cloud Storage con una política de retención bloqueada** más un **hash por lote de eventos**. La documentación del proveedor indica que una política bloqueada impide reducir o quitar la retención y borrar o reemplazar objetos antes del plazo, y que el bloqueo es **irreversible**; por eso el plazo productivo no se fija aquí y debe acordarse con la política de conservación de la matriz de tratamiento del Anexo B [@es2bucketlock; @es2bigquerydml].

El ensayo previsto es acotado y explícito: exportar eventos sintéticos mínimos, aplicar la retención bloqueada por siete días e intentar alterar o borrar un objeto para comprobar el fallo esperado. **No se ha ejecutado**, no se ha contratado el servicio y el plazo definitivo sigue abierto; por lo tanto, RNF-017 no se declara cumplido. El **acceso a los proveedores de pago, firma e identidad** es un asunto distinto, depende de terceros y se registra en la sección correspondiente de [pendientes](../pendientes.md).

