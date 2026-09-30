# Decisiones de planificación y hallazgos

## Decisiones vigentes

| Tema | Línea base para planificar | Evidencia / límite |
| --- | --- | --- |
| Prioridad | DB → Backend → API → pruebas → mock por módulo | Instrucción explícita actual del usuario. |
| Alcance | Todas las categorías/funciones desde el diseño; implementación incremental | Decisión posterior en ES2 contexto y Anexo B v2. |
| Backend | Monolito modular Go, API HTTP/JSON/OpenAPI | Propuesta ES2 v1.2; no hay aplicación implementada. |
| Persistencia | PostgreSQL 18 + PostGIS + `btree_gist` como objetivo | Sujeta a verificación local/Cloud SQL; no hay esquema físico. |
| SQL | `pgx/pgxpool` + `sqlc`, transacciones explícitas | Propuesta ES2; confirmar versiones y flujo en ticket de fundación. |
| Analítica | Outbox PostgreSQL → Pub/Sub → BigQuery; BigQuery no es fuente operacional ni garantiza inmutabilidad | ES2 separa analítica de RNF-017; retención bloqueada/hash sigue pendiente de plazo y ensayo. |
| Workers | Goroutines del monolito sobre tareas durables PostgreSQL, idempotentes | Propuesta ES2; sin despliegue ni prueba. |
| Privacidad | Ley 21.719 como criterio de diseño desde primer incremento | Decisión del usuario; no afirmar cumplimiento demostrado ni fecha de vigencia anticipada. |
| Pagos | Sandbox en desarrollo/CI/staging; contrato backend neutral | Mercado Pago Split 1:1 investigado parcialmente; medios, tarifa, KYC, reembolso/saldo y ensayo siguen abiertos. Split no se denomina Escrow por inferencia. |
| Mock | HTML/CSS/TS/DOM/fetch en contenedor separado, API pública | Solicitud actual; el mock no decide frontend definitivo. |
| Kanban | `backlog.md` es registro fuente de tarjetas listo para transferir | No se detectó tablero ni integración Hermes disponible en herramientas. No afirmar tickets creados en servicio externo. |

## Hallazgos que requieren seguimiento

1. **README de `espaciGo/` desactualizado:** menciona microservicios, Next.js definitivo, BigQuery como log inmutable, servicio separado Gotenberg/Python y Escrow por Mercado Pago. Estas afirmaciones contradicen la propuesta consolidada de ES2 o las instrucciones actuales. El README no se modifica en esta tarea; se deja recomendación de reconciliarlo en un ticket documental antes de que guíe implementación.
2. **Sin implementación ni entorno:** solo se encontró README e instrucciones de planificación dentro de `espaciGo/`. No hay fuente Go, OpenAPI, esquema/migraciones, frontend, Dockerfile/compose o tablero.
3. **Modelo lógico voluminoso:** Anexo B cubre 43 tablas, incluyendo promociones, reportería premium y NPS aún no cerrados comercialmente. No migrar todo por anticipado; diferir esas tablas/capacidades hasta resolver finalidad, regla comercial y contratos.
4. **Frontera M04/M06:** disponibilidad manual y calendario nacen en publicación, pero ocupación/reserva es un invariante transaccional. Debe haber un dueño único del calendario y colaboración contractual; no crear dos sistemas de disponibilidad.
5. **Frontera de pagos:** `reserva.estado`, `pago.estado`, `garantia`, `liquidacion` y `movimiento_financiero` son hechos distintos; no colapsarlos en un estado/tabla. El proveedor determina resultados remotos observados.
6. **Privacidad y conservación:** matrices de tratamiento y duración de retención necesitan fundamento y decisión por tipo de dato. RNF de 5 años para contratos/evidencia/auditoría no se extiende a perfil, KYC, chat o telemetría.
7. **Integraciones no habilitadas:** Registro Civil, SII, proveedor de firma y pagos requieren credenciales/contrato/sandbox. Diseñar puertos neutrales y mantener fakes; no hardcodear integraciones no probadas.
8. **Requisitos heredados a medir:** rendimiento, disponibilidad, concurrencia, respaldo/restore y portabilidad son objetivos/ensayos planeados. No tratarlos como capacidades presentes.
9. **Mapeo documental:** HUs por épica no coinciden siempre con módulos M01–M11 y algunas responsabilidades transversales (notificaciones/documentos) aparecen bajo más de un módulo. Backlog conserva trazabilidad a la ficha primaria del Anexo E/D/B y valida cobertura antes de declarar cada slice cerrado.
10. **Economía de ejecución:** la simulación de ES2 es hipotética y actualmente tiene VAN de caja negativo bajo sus supuestos; demanda/costos de proveedor aún abiertos. Mantener prestaciones futuras (promoción, NPS) separadas de producto core hasta evidencia.

## Evaluación del archivo `arquitectura_y_roadmap_del_marketplace.md`

El archivo adjunto se conserva íntegro en `referencias/arquitectura_y_roadmap_del_marketplace.md` como propuesta de entrada recibida el 30-09-2026. No se copian automáticamente sus decisiones al backlog. Se aplica esta resolución para agentes:

| Propuesta adjunta | Resolución vigente | Motivo / trabajo pendiente |
| --- | --- | --- |
| PostgreSQL OLTP, monolito Go, `pgxpool` + `sqlc`, Terraform, GCP | Compatible en principio y recogido en esta planificación | Verificar versiones/extensiones, costos y ensayos local/cloud antes de fijar configuración. |
| Cloud Run escala de 0 a N | No usar como premisa del worker ES2 | La propuesta ES2 presupone mínimo una instancia y CPU disponible para goroutines durables; revisar costo/capacidad con ensayo. |
| Buckets públicos y privados para multimedia | Bucket privado y autorización API para objetos | El diseño vigente protege imágenes/documentos; cualquier publicación pública requiere URL/control explícito y decisión de seguridad. |
| Cloud Logging / Log Sink a BigQuery como auditoría inmutable | Logs técnicos separados de `evento_auditoria`; BigQuery analítico no es inmutable | RNF-017 tiene propuesta separada de retención bloqueada + hash por lote, con plazo/alcance/ensayo pendientes. |
| Datastream CDC de pagos/contratos a BigQuery | Descartado en ES2 | El flujo vigente propuesto usa outbox minimizado PostgreSQL → Pub/Sub → BigQuery; no replicar datos personales/financieros por CDC sin nueva evaluación. |
| Endpoint público de telemetría `POST /api/v1/telemetry` | No se crea por existir en el documento adjunto | ES2 propone analítica, eventos de dominio y eventos agregables; el contrato de ingestión y autenticidad deben tener requisito/ticket, no se agregan endpoints al mock. |
| BigQuery responde métricas premium para CRM | Aplazado/condicionado | Se requieren entitlement activo, autorización por fila, minimización y decisión comercial; no bloquea el flujo transaccional inicial. |
| Pago y contrato dentro de una transacción ACID con rollback | Rechazado como modelo de consistencia para terceros | La transacción PostgreSQL solo cubre escrituras locales. Proveedor confirmado, timeout, webhook, conciliación y compensación son hechos distribuidos e idempotentes. |
| HTML templates + HTMX servidos desde Go para prototipar | Rechazado para esta validación | Instrucción explícita actual: mock desacoplado en su contenedor, HTML/CSS/TypeScript compilado/DOM/fetch, API JSON pública. No HTMX ni backend generador de HTML. |
| Fase de Terraform/Cloud antes de archivos/catálogo | No es el orden de trabajo actual | Prioridad actual DB → backend → API → pruebas → mock y validación local; nube productiva se aborda después de obtener un flujo local verificable. |
| `tx, _ := pool.Begin()` como patrón | No adoptar manejo de error ignorado | Transacciones explícitas, errores comprobados y rollback/commit controlados; usar la API elegida tras revisión de foundation. |

Las resoluciones de mock, prioridad y autoridad de contexto provienen de la solicitud actual del usuario. Lo compatible del roadmap no implica que ya esté implementado.

## Propuestas de mejora para decidir con tickets futuros

- Un catálogo explícito y versionado para transiciones de reserva con tabla de transición autorizada o política de dominio testeada; `CHECK` por sí solo no implementa la máquina de estados.
- Contrato compartido `documento`/storage: propietario tipado o FK verificable y ciclo de vida de carga en dos fases.
- Vocabulario contable con invariantes de signo/moneda, estado observado y correlación del proveedor para todos los movimientos; reconciliación revisable.
- Especificar ownership y payloads de outbox/inbox, orden por agregado y retención antes de conectar Pub/Sub.
- Separar capacidad de auditoría de producto administrativo: eventos append-only operacional más exportación retenida solo después de resolver privacidad/retención irreversible.
- Acordar contratos entre módulos y consultas antes de abrir paquetes/repositorios transversales; evitar acceso arbitrario a tablas ajenas.

Estas son observaciones de planificación, no alteraciones aprobadas del modelo o arquitectura. Se convierten en cambios aceptados solo con ticket y revisión de impactos sobre RQF/RNF/CU/HU.
