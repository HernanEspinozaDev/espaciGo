# CORE-ARCH-01 — Contratos entre módulos y reglas transversales

**Estado:** diseño documental presentado para revisión. **Fecha:** 2026-10-01. No es evidencia de implementación. No incluye código, DDL, migraciones, configuración Docker ni decisión de UX.

## Decisión arquitectónica ratificada

EspaciGo se organiza como **monolito modular Go**, una unidad desplegable con límites internos por dominio. Se conservan M01–M11; no se crean microservicios por módulo. El módulo propietario define las invariantes y escrituras de sus datos y expone operaciones explícitas. PostgreSQL es la fuente de verdad operacional. Esta línea base y MAP-01–MAP-10 fueron ratificados por el usuario y aprobados/fusionados en el [PR #1](https://github.com/HernanEspinozaDev/espaciGo/pull/1); eso no prueba implementación ni autoriza DDL fuera de las tarjetas dependientes.

## Propiedad de datos y límites de escritura

La tabla resume los dueños ratificados en `mapa_dominio_y_ownership.md`. Un módulo consumidor no obtiene permiso de escritura por tener una referencia o lectura.

| Módulo | Datos de su propiedad (Anexo B ES2) |
|---|---|
| M01 Identidad y cuenta | `usuario`, `rol_usuario`, `sesion`, `token_accion`, `version_terminos`, `aceptacion_terminos` |
| M02 Perfil y privacidad | `perfil_usuario`, `cuenta_cobro`, `solicitud_titular` |
| M03 Verificación KYC/KYB | `verificacion` |
| M04 Publicaciones y disponibilidad | `categoria_espacio`, `espacio`, `politica_cancelacion`, `tramo_cancelacion`, `regla_tarifa`, `regla_comision`, `campana`, `derecho_reporte` |
| M05 Búsqueda y cotización | `cotizacion` |
| M06 Reservas y pagos | `vinculo_proveedor_vendedor`, `reserva`, `ocupacion`, `reserva_transicion`, `pago`, `evento_proveedor`, `orden_promocion` |
| M07 Contratos y firma | `contrato`, `firma_contrato` |
| M08 Operación del arriendo | `operacion_arriendo` |
| M09 Comunicación, archivos y reputación | `documento`, `mensaje_reserva`, `resena`, `reporte_resena`, `notificacion`, `entrega_notificacion`, `respuesta_nps` |
| M10 Disputas, liquidación y tributación | `disputa`, `garantia`, `liquidacion`, `movimiento_financiero`, `documento_tributario` |
| M11 Administración y auditoría | `evento_auditoria`, `outbox_evento` |

`regla_comision` pertenece a M04; M05 la consulta por contrato al cotizar; M06 consume la cotización y guarda la referencia/snapshot aceptados; M11 autoriza y audita cambios administrativos. RNF-020 requiere que la actualización se refleje en ≤60 s sin reiniciar; el mecanismo concreto y su prueba quedan para las tarjetas de implementación.

## Contratos de dependencia entre módulos

1. La API/controlador traduce solicitudes a casos de uso; no contiene SQL ni reglas de proveedor. Dominio y casos de uso dependen de contratos, no de adaptadores concretos. Persistencia y proveedores implementan puertos detrás de límites explícitos.
2. El consumidor llama al contrato público del propietario para leer o solicitar un cambio. No importa paquetes privados ajenos, no escribe tablas ajenas y no ejecuta consultas SQL cruzadas. Las lecturas intermodulares devuelven datos explícitos y mínimos; la autorización del recurso la verifica su propietario.
3. No se permiten dependencias circulares. La colaboración síncrona usa operaciones de aplicación explícitas; el trabajo durable/publicable usa eventos versionados. Si una colaboración requiere coordinar hechos duraderos de varios módulos, se registra en una tarjeta específica; no se simula una transacción distribuida.
4. El contexto de operación lleva un actor autenticado y su correlación. Cada caso de uso comprueba autorización sobre el recurso concreto; el rol por sí solo o la autorización de la interfaz no bastan. Las acciones administrativas se autorizan y auditan.
5. Los puertos de pago, firma, identidad, correo y almacenamiento no filtran SDKs ni modelos de proveedor al dominio. Los webhooks se autentican y deduplican antes de causar efectos.

## Transacciones, errores e idempotencia

- El caso de uso del módulo dueño establece el límite de una transacción local. La transacción no mantiene una conexión/bloqueo mientras se espera una red externa.
- En M06, reservar, asignar `ocupacion`, registrar la transición correspondiente y escribir el evento outbox requerido deben confirmarse o revertirse juntos. La liberación de ocupación y la transición que la causa también son atómicas. M04 solicita bloqueos manuales mediante el contrato de M06; no mantiene un calendario paralelo.
- Un error interno conserva una categoría/código estable, el contexto mínimo y la causa técnica envuelta. La capa API traducirá más adelante a su contrato HTTP versionado; esta tarjeta no fija códigos HTTP ni el JSON final. Las respuestas públicas no exponen SQL, stack traces, secretos ni contenido sensible.
- Se distinguen errores de validación, autenticación/autorización, recurso inexistente, conflicto, duplicado idempotente, timeout/resultado externo desconocido, indisponibilidad reintentable y fallo interno. El consumidor puede reintentar solo cuando la operación está clasificada como segura.
- Los comandos con efectos repetibles usan una clave idempotente estable y acotada a actor/operación; los webhooks y consumidores deduplican por identificador persistente del evento. No se asume entrega «exactamente una vez».
- Un timeout de pago no demuestra rechazo: se concilia el intento existente y no se crea otro cobro con una clave distinta hasta resolverlo. Una compensación externa es un hecho/operación nueva, no un rollback ficticio.

## Correlación, auditoría, outbox y logs

`request_id` se origina o valida en el ingreso; `correlation_id` se conserva a través de los casos de uso, transacciones, outbox, adaptadores y webhook correlacionado. Los identificadores de operación/evento permiten deduplicación y diagnóstico, pero se minimizan porque incluso un UUID puede ser vinculable.

- **Auditoría de negocio (`evento_auditoria`):** actor/servicio, acción, recurso, resultado, motivo cuando aplique y correlación; separada de logs, outbox y analítica. El escritor es append-only desde la aplicación. La inmutabilidad de infraestructura y la retención requieren validación y pruebas posteriores (RNF-017/043); este contrato no afirma que ya se cumplan.
- **Outbox (`outbox_evento`):** solo para publicar cambios que lo necesiten; el registro se escribe en la misma transacción del agregado propietario. Mensajes mínimos, identificador estable y versión de esquema; los consumidores deduplican y toleran reintentos/orden parcial. Retención, lease y esquema físico quedan para las tarjetas DB correspondientes.
- **Logs técnicos:** salida estructurada con módulo, severidad, duración, resultado y correlación. No guardar contraseñas, tokens, credenciales, datos de tarjeta/bancarios, documentos, cuerpos íntegros de solicitudes, mensajes privados ni payloads completos de proveedores. No afirmar que los logs sustituyen auditoría.
- **Analítica:** eventos minimizados fuera del OLTP; ni BigQuery ni sus vistas modifican estados de reserva, pagos o disputas.

## Trazabilidad RNF-013–043 y límites

El contrato deriva de ES2 §3.3.1–3.3.7, `backend_y_api.md`, el mapa ratificado y RNF-013–043 de ES1. La matriz indica qué restricción de arquitectura se fija y qué demostración queda para su tarjeta; no afirma que el sistema ya cumpla el requisito.

| Requisitos | Aplicación del contrato | Diferido / evidencia futura |
|---|---|---|
| RNF-013, 015, 016 | M01/API aplica el objetivo de bcrypt costo ≥12, HTTPS/TLS ≥1.2 y nunca token en URL; sesión expira tras 30 min inactivos y máximo absoluto 8 h. | Implementación y pruebas de autenticación/sesión quedan en tarjetas M01/API; no demostradas aquí. |
| RNF-014, 025 | Datos bancarios y PDF finales con objetivo de cifrado AES-256 en reposo; pagos tokenizados, sin PAN/CVV almacenados; secretos solo por referencia segura. | Cifrado/gestión de claves y tokenización se verifican en DB, infraestructura e integración; no demostradas aquí. |
| RNF-017, 043 | `evento_auditoria` separado de logs/outbox, escritor append-only y actor/acción/recurso/correlación; RNF-043 pide conservar auditoría ≥5 años. | Inmutabilidad a nivel de infraestructura (RNF-017) y retención efectiva requieren política/ensayo; no demostradas aquí. |
| RNF-018, 026, 029 | M02 coordina derechos con cada dueño; la baja aprobada activa borrado de PII, trámite ≤72 h y anonimización transaccional preservando auditoría. | Compatibilizar baja/anónimo con conservación mínima de contratos/evidencias y auditoría (RNF-042/043) requiere política competente antes de automatizar. |
| RNF-019, 030 | La meta es escalar horizontalmente con aumento de carga 200% en ≤10 min; la capacidad exigida es 500 usuarios concurrentes y 100 escrituras/s. | Benchmark y evidencia de escalado son obligatorios; no se deducen del monolito modular ni se midieron aquí. |
| RNF-020 | M04 posee `regla_comision`; M05 lee y M06 congela snapshot, con propagación objetivo ≤60 s. | Mecanismo de caché/lectura y prueba de SLA se fijan en tarjetas de módulo. |
| RNF-021 | Límites puros facilitan pruebas unitarias por capa. | Meta ≥80% de cobertura unitaria de negocio y CI debe fallar bajo el umbral; se verificará al definir harness/pipeline. |
| RNF-022 | El backend expone contratos independientes del navegador. | Validar las dos últimas versiones estables de Chrome/Firefox/Edge/Safari en pruebas de cliente/mock. |
| RNF-023, 024, 028 | API externa: timeout 5 s, máximo 2 reintentos separados 2 s; webhook con firma validada y hasta 5 reintentos exponenciales; conciliación automática cada 5 min. Timeout de pago queda por conciliar, nunca se duplica con otra clave. | Cada adaptador debe implementar y probar esos límites; esta tarjeta no ejecuta ni acredita integraciones. |
| RNF-027 | M10 es propietario del documento tributario; sistemas fiscales quedan tras un contrato. | Meta SII: boleta timbrada XML/PDF disponible ≤24 h; reglas y evidencia requieren tarjeta de integración y validación competente. |
| RNF-031 | M04 controla categorías como catálogo; proveedores se integran por puertos sustituibles. | Incorporar una categoría o método nuevo sin cambios laterales se comprobará en sus tarjetas y pruebas. |
| RNF-032, 033 | Interfaces por dominio documentadas, acíclicas; OpenAPI debe actualizarse en cada versión. | Contrato HTTP/OpenAPI se fija en CORE-API-01; esta tarjeta no crea el archivo. |
| RNF-034–037 | Empaquetar en Docker; evitar dependencias propietarias y permitir despliegue en 2 proveedores sin cambiar código; entorno local reproducible por contenedores. | Compatibilidad de despliegue, servicios locales y entornos se verifica en tarjetas de entorno; no implementado aquí. |
| RNF-038 | PostgreSQL relacional con ACID y soporte geoespacial es objetivo operacional de ES2. | Versión/extensiones, naming y perfiles se fijan en CORE-DB-01. |
| RNF-039, 040 | Código versionado en GitHub o equivalente con revisión; CI/CD ejecuta pruebas antes de cada despliegue. | Repositorio, checks y despliegues deben acreditarse en tarjetas de CI; este documento no acredita pipeline. |
| RNF-041, 042 | M09 posee metadatos/objeto; RNF-041 limita cada publicación a 50 MB y 10 imágenes de ≤5 MB; RNF-042 pide conservar contratos/evidencias ≥5 años. | Validación del límite y retención frente a baja/derechos (RNF-018/029) debe resolverse antes de automatizar; no se declara cumplimiento ni se decide plazo alternativo. |

**Criterios de aceptación cubiertos por el documento:** decisión explícita de monolito modular; owners para los 11 módulos; reglas acíclicas de dependencia e importación; transacciones y límites de red; estrategia de errores/idempotencia/correlación; auditoría/outbox diferenciados; prácticas de logs sin PII; trazabilidad completa RNF-013–043 y ES2 §3.3.1–3.3.7. La revisión estática de la matriz y sus límites es la verificación de esta tarjeta; no se ejecutaron pruebas de aplicación.

**Diferido y fuera de alcance:** contratos HTTP/códigos concretos (CORE-API-01); perfil físico PostgreSQL, migrador y esquema outbox (CORE-DB-01/03); código Go; DDL/migraciones; Docker; elección de microservicios/proveedores; decisión de UX; probar cumplimiento, inmutabilidad, cobertura, rendimiento, retención o propagación RNF-020. No se habilitan tarjetas posteriores hasta revisión y cierre de CORE-ARCH-01.