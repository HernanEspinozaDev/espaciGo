# Plan de pruebas

## Objetivo

Probar cada slice completo de DB → backend → API antes de abrir su tarjeta de frontend mock. Las 16 pruebas PT-01–PT-16 del Anexo C ES2 son catálogo planificado, no resultados. Los ensayos MD-01–MD-13 del modelo son criterios que deben mapearse a migraciones/DB real; no se ejecutan durante esta planificación.

## Niveles

| Nivel | Qué comprueba | Cuándo |
| --- | --- | --- |
| Unitarias de dominio | Transiciones válidas/ inválidas, dinero exacto, permisos, expiración, versionado y políticas puras. | Con cada caso de uso. |
| Repositorio/DB | Consultas `sqlc`, restricciones, FK, unicidad, migración desde vacío y fixtures; PostgreSQL/PostGIS real de test, aislado. | Al completar cada migración. |
| Integración de aplicación | Límite transaccional, repositorios y servicio; outbox/inbox/idempotencia; rollback de fallos locales. | Por slice vertical. |
| Contrato API | OpenAPI, validación de requests, responses, autorización, estados HTTP, paginación y errores. | Para cada endpoint. |
| Adaptador externo | Sandbox, firma de webhooks, deduplicación, timeout/conciliación y compensación cuando aplique. | Solo con acceso autorizado; fakes para unitarias. |
| Mock funcional | Operaciones públicas del API y flujo comprensible con respuestas y fallos visibles. | Al final del módulo funcional. |
| No funcional/operacional | Concurrencia, rendimiento, restore, seguridad, accesibilidad del harness y observabilidad. | Como gates de release al existir entorno/producto. |

## Gates obligatorios de riesgo

- Reserva: dos solicitudes simultáneas al mismo espacio/rango; solape rechazado, adyacencia permitida; expiración y liberación atómicas; estados y auditoría coinciden.
- Dinero: aritmética decimal exacta; idempotencia; webhook duplicado/tardío; timeout por conciliar; no asumir que transacción PostgreSQL revierte proveedor.
- Autorización: usuario no puede leer/escribir recursos ajenos; rol admin auditado; no confiar en IDs/rol entregados por cliente.
- Archivos: bucket privado, autorización de subida, tipo/tamaño/hash validados; URL firmada de alcance/vida acotados; nunca en logs/mock.
- Privacidad: finalidad y mínimo dato, requests de titular, revisión de derivados/backups/retención, PT-16; sin declarar cumplimiento por tener tests.
- Migración/continuidad: construir DB desde cero, aplicar secuencia, backup/restore, RPO/RTO medidos, compatibilidad de datos.

## Cobertura transversal de RNF ES1

| RNF | Área/tickets que los deben hacer verificables | Evidencia prevista / límite |
| --- | --- | --- |
| 001, 030 | DISC, BOOK, core de medición | Carga reproducible de búsqueda/escritura en entorno comparable; una suite funcional no demuestra tiempos/capacidad. |
| 002, 011, 012, 028 | BOOK, DIS, ADMIN/outbox | Latencia propia separada de proveedor, fallos/reintentos y conciliación durable. |
| 003 | CONT | Generación contractual medida con tamaño/tiempo; proveedor o renderer aún por decidir. |
| 004, 041 | LIST | Validación de optimización, tamaño y cantidad; sin hardening de UI definitiva. |
| 005–008 | Harness mock y futuro frontend | Son requisitos de interfaz de ES1; el mock es una herramienta mínima de API y no certifica UX/accesibilidad del frontend final. |
| 009–010, 019–020, 031, 034–040 | CORE/operación de release | Disponibilidad, backup/restore, escalabilidad/modificabilidad, adaptabilidad, portabilidad, contenedores y CI requieren entorno/ensayos posteriores. |
| 013–018, 026, 029 | AUTH, PRIV, KYC, BOOK, ADMIN | Seguridad, privacidad y normativa; se prueban controles, pero ningún resultado equivale a certificación/compliance legal. |
| 021, 032–033, 039–040 | CORE y cada módulo | Automatización, OpenAPI, límites modulares, Git/CI; calidad del proceso y código medible. |
| 022–025, 027 | Adaptadores/API externa | Compatibilidad, interoperabilidad y dependencias normativas; sandbox/asesoría externa si se habilita. |
| 042–043 | CONT, OPS, ADMIN | Retención de contratos/evidencia/auditoría según política aprobada; los cinco años de ES1 no se extienden a otros datos. |

Todos los 43 RNF aparecen en este cruce por rango o identificador. Las metas de rendimiento, disponibilidad, retención y recuperación son heredadas y deben verificarse contra los acuerdos vigentes antes de declararlas criterios de aceptación finales.

## Datos y ambiente de prueba

Usar PostgreSQL compatible con extensiones fijadas por el Anexo B; verificar PG18/PostGIS/`btree_gist` en el ambiente objetivo antes de fijar imagen. Fixtures sintéticos, sin secretos ni información personal real. Integraciones externas se reemplazan por fakes en pruebas unitarias y sandbox únicamente para escenarios proveedor autorizados.

Cada reporte futuro registra ticket, versión, entorno, seed/dataset, resultado esperado y observado, evidencia, fecha y responsable; diferencias crean tickets, no se ocultan ampliando el alcance.

## No se declara aún

No existen ejecuciones de PT/MD para el producto, mediciones KPI/SLA ni pruebas de carga/restore. Metas de ES1 (99,9 %, RPO 4 h, RTO 6 h, concurrencia) son criterios heredados por verificar. Este documento planifica evidencia; no la produce.
