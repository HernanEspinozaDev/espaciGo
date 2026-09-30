# Plan de Backend y API

## Arquitectura objetivo de planificación

La fuente vigente es `Informes/ES2PT/investigacion/propuesta_backend_final.md` v1.2 y secciones 3.3–3.7. La propuesta es **un monolito modular Go** con API HTTP/JSON/OpenAPI y una unidad desplegable Cloud Run. Los dominios se separan por paquetes e interfaces, no por microservicio/contendor por módulo. Persistencia prevista con `pgx/pgxpool` y `sqlc` para SQL revisable; llamadas dinámicas parametrizadas solo con allowlist. Workers durables se ejecutan como goroutines dentro del servicio y reclaman trabajo desde PostgreSQL; no se depende de memoria ni entrega exactamente una vez.

No hay aún código de aplicación en `espaciGo/`. Nombres de paquetes y librerías concretas siguen sujetos a ticket de fundación/validación local; no se crea estructura de código en esta fase.

## Capas previstas por módulo

1. **Dominio:** tipos y reglas propias, estados, invariantes y errores de negocio sin HTTP/SDK Cloud.
2. **Aplicación:** casos de uso, validación contextual, autorización y límite transaccional.
3. **Persistencia:** interfaces y adaptadores PostgreSQL/`sqlc`; transacciones explícitas y consultas del dueño lógico.
4. **API:** esquemas de entrada/salida, controladores delgados, OpenAPI versionado, códigos de error estables y autenticación/autorización por recurso.
5. **Adaptadores:** proveedor de pago/firma/KYC/correo/almacenamiento detrás de contratos internos. Sandbox en desarrollo/CI/staging. Credenciales solo por referencia segura.
6. **Eventos/operación:** outbox en la transacción de dominio, inbox de webhooks deduplicable, idempotencia, correlación y logs sin datos personales.

## Contrato HTTP

El API entrega JSON; el mock y clientes futuros no dependen de HTML generado por servidor. Los tickets de contrato deben fijar versionado, autenticación, permisos/roles, paginación/filtros, fechas/zona horaria, dinero, conflictos, idempotency keys y errores. OpenAPI es la fuente revisable de request/response y se actualiza con cada cambio. Las rutas concretas y códigos se acuerdan en cada ticket antes de implementación y no se inventan aquí en conflicto con CU/RQF.

Los endpoints externos (webhooks) validan firma/origen según proveedor, deduplican por identificador, guardan el evento antes de procesar y devuelven la respuesta adecuada al proveedor. Un timeout no significa que el pago falló: pasa a conciliación; nunca se repite el cobro con una clave nueva sin resolver el intento previo.

## Orden de construcción

Fundación de dominio, errores, configuración, persistencia y pruebas → M01 autenticación → M02/M03 perfil y verificación → M04 catálogo/publicación → M05 búsqueda/cotización → M06 reserva/pago → M07 contrato → M08 operación → M09 comunicaciones/reputación → M10 disputa/liquidación → M11 administración/reportes. Auditoría/outbox se establecen temprano como capacidades transversales; se amplían en M11. Integraciones proveedoras se habilitan solo cuando existan acceso, contrato y casos de sandbox.

Los módulos pueden avanzar en paralelo solo después de que sus contratos y tablas compartidas estén estables. DB va primero dentro de cada slice; backend/API y pruebas siguen la migración, y la tarjeta mock queda al final.

## Dependencias que cruzan módulos

- M02/M03 dependen de cuenta/sesión M01.
- M04 necesita titularidad/permisos M01–M03 y categorías; M05 consulta la oferta M04.
- M06 necesita M01–M05; búsqueda/cotización y ocupación confluyen en una transacción local.
- M07 y M08 dependen de estados y datos snapshot de M06; evidencia binaria usa contrato de almacenamiento.
- M09 necesita reserva/participantes para chat, reseña y notificaciones.
- M10 depende de pago, operación, evidencia y disputa; liquidación refleja observación del proveedor.
- M11 consume capacidades y vistas acotadas de todos los anteriores, con autorización administrativa auditada.

## Fuera de alcance

No se decide framework/arquitectura frontend final ni se modifican contratos para facilitar el mock. No se separan servicios, no se da por seleccionada integración de proveedor ni se implementa infraestructura en esta etapa de planificación.
