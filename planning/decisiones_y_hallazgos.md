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
2. **Sin implementación del producto:** al revisar `espaciGo/` solo se encontró README e instrucciones; no hay fuente Go, OpenAPI, esquema/migraciones, frontend, Dockerfile/compose o tablero. Esto no significa que el equipo no tenga herramientas locales; el entorno descrito por el usuario está resumido en `entorno_de_desarrollo.md` y deberá verificarse antes de ejecutar setup.
3. **Modelo lógico voluminoso:** Anexo B cubre 43 tablas, incluyendo promociones, reportería premium y NPS aún no cerrados comercialmente. No migrar todo por anticipado; diferir esas tablas/capacidades hasta resolver finalidad, regla comercial y contratos.
4. **Frontera M04/M06:** disponibilidad manual y calendario nacen en publicación, pero ocupación/reserva es un invariante transaccional. Debe haber un dueño único del calendario y colaboración contractual; no crear dos sistemas de disponibilidad.
5. **Frontera de pagos:** `reserva.estado`, `pago.estado`, `garantia`, `liquidacion` y `movimiento_financiero` son hechos distintos; no colapsarlos en un estado/tabla. El proveedor determina resultados remotos observados.
6. **Privacidad y conservación:** matrices de tratamiento y duración de retención necesitan fundamento y decisión por tipo de dato. RNF de 5 años para contratos/evidencia/auditoría no se extiende a perfil, KYC, chat o telemetría.
7. **Integraciones no habilitadas:** Registro Civil, SII, proveedor de firma y pagos requieren credenciales/contrato/sandbox. Diseñar puertos neutrales y mantener fakes; no hardcodear integraciones no probadas.
8. **Requisitos heredados a medir:** rendimiento, disponibilidad, concurrencia, respaldo/restore y portabilidad son objetivos/ensayos planeados. No tratarlos como capacidades presentes.
9. **Mapeo documental:** HUs por épica no coinciden siempre con módulos M01–M11 y algunas responsabilidades transversales (notificaciones/documentos) aparecen bajo más de un módulo. Backlog conserva trazabilidad a la ficha primaria del Anexo E/D/B y valida cobertura antes de declarar cada slice cerrado.
10. **Economía de ejecución:** la simulación de ES2 es hipotética y actualmente tiene VAN de caja negativo bajo sus supuestos; demanda/costos de proveedor aún abiertos. Mantener prestaciones futuras (promoción, NPS) separadas de producto core hasta evidencia.

## Separación entre proyecto y entorno

El adjunto que el usuario compartió el 30-09-2026 describe el entorno de desarrollo disponible; no corresponde a EspaciGo como producto y se retiró del paquete de fuentes del proyecto. No se deben importar desde ese documento requisitos, módulos, telemetría, decisiones de analítica ni arquitectura de marketplace. El resumen permitido de herramientas aparece en `entorno_de_desarrollo.md`; versiones, instalación y acceso se verifican cuando una tarjeta de setup lo necesite.

La arquitectura y los requisitos del producto se derivan exclusivamente de las instrucciones actuales y de los snapshots ES1/ES2 en `referencias/`.

## Propuestas de mejora para decidir con tickets futuros

- Un catálogo explícito y versionado para transiciones de reserva con tabla de transición autorizada o política de dominio testeada; `CHECK` por sí solo no implementa la máquina de estados.
- Contrato compartido `documento`/storage: propietario tipado o FK verificable y ciclo de vida de carga en dos fases.
- Vocabulario contable con invariantes de signo/moneda, estado observado y correlación del proveedor para todos los movimientos; reconciliación revisable.
- Especificar ownership y payloads de outbox/inbox, orden por agregado y retención antes de conectar Pub/Sub.
- Separar capacidad de auditoría de producto administrativo: eventos append-only operacional más exportación retenida solo después de resolver privacidad/retención irreversible.
- Acordar contratos entre módulos y consultas antes de abrir paquetes/repositorios transversales; evitar acceso arbitrario a tablas ajenas.

Estas son observaciones de planificación, no alteraciones aprobadas del modelo o arquitectura. Se convierten en cambios aceptados solo con ticket y revisión de impactos sobre RQF/RNF/CU/HU.
