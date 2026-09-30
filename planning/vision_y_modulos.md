# Visión del producto y mapa de módulos

## Por qué existe EspaciGo

EspaciGo busca conectar dos necesidades que el arriendo tradicional atiende con fricción: propietarios que mantienen espacios comerciales o multipropósito sin uso y pymes, emprendedores y profesionales que necesitan un lugar por horas, días, semanas u otros períodos breves. La plataforma convierte la publicación y el arriendo flexible en un flujo trazable, desde verificar a las partes y encontrar disponibilidad hasta reservar, documentar el uso y resolver pagos o disputas.

La propuesta no es solo un catálogo de anuncios. La confianza operativa depende de coordinar identidad, titularidad, precio versionado, disponibilidad concurrente, acuerdo contractual, pago externo, evidencia del estado del espacio y liquidación. El proyecto también plantea protección de datos personales desde el diseño.

La brecha de mercado es la motivación de ES1; la demanda, precios por categoría, aceptación de comisión y viabilidad económica aún requieren mediciones. La simulación económica actual es hipotética y negativa bajo los supuestos publicados; no es validación de mercado ni resultado comercial.

## Límite entre entregas

- **ES1** formuló el problema, objetivos, actores y requisitos. Es una entrega cerrada e inmutable.
- **ES2** consolidó una propuesta técnica para construir el producto completo, con entrega incremental. No demuestra código, despliegue, accesos de proveedor ni cumplimiento legal.
- Este backlog convierte esa propuesta en unidades ejecutables para DB, backend, API y pruebas. Las decisiones se pueden refinar durante la ejecución mediante tickets con impacto y trazabilidad.
- Durante desarrollo, pagos externos se prueban solo en sandbox; el simulador local no acredita una integración real. No se debe afirmar custodia/Escrow hasta verificar capacidad, contrato y prueba del proveedor.

## Flujo de valor principal

```text
Cuenta y derechos
  → verificación de las partes
  → publicación de una unidad reservable y sus precios
  → búsqueda y cotización
  → reserva + ocupación atómica + pago externo conciliable
  → contrato y firma
  → check-in / uso / check-out con evidencia
  → disputa o cierre + liquidación observada
  → comunicación, reputación, notificaciones y auditoría
```

Los cambios de estado operativo se persisten en PostgreSQL. Los proveedores externos se integran mediante adaptadores y eventos idempotentes; una transacción local no revierte una operación ya aceptada por un tercero. Los eventos analíticos no son fuente de verdad financiera.

## Módulos funcionales y trazabilidad

Los nombres y rangos siguen el catálogo de ES1. La asignación de entidades sigue el diccionario oficial de diseño ES2; el cruce entre ambos es una línea base que deberá revisarse antes de migrar.

| ID / módulo | Responsabilidad | HU principales | CU | RQF ES1 | Entidades del Anexo B ES2 |
| --- | --- | --- | --- | --- | --- |
| M01 Identidad y cuenta | Registro, términos, autenticación, sesiones, credenciales y recuperación. | HU01–03 | CU-01–06, CU-50 | 001–023, 186–188, 213–218 | `usuario`, `rol_usuario`, `sesion`, `token_accion`, `version_terminos`, `aceptacion_terminos` |
| M02 Perfil y privacidad | Perfil, datos de cobro y solicitudes de derechos sobre datos. | HU31–32 | CU-07–09 | 024–037, 189–191 | `perfil_usuario`, `cuenta_cobro`, `solicitud_titular` |
| M03 Verificación KYC/KYB | Evidencia y estado de verificación, revisión manual y adaptadores externos. | HU04 | CU-10–14 | 038–059, 192–194, 219–220 | `verificacion`, referencias protegidas a proveedor; documentos en `documento` |
| M04 Publicaciones y disponibilidad | Catálogo de categorías, publicaciones, tarifa/políticas, archivos y calendario/bloqueos. | HU05–09, HU20–23; HU24 no implementada en ES1 y sin RQF/CU | CU-15–18, CU-17/CU-19 en calendario | 060–093, 195–198, 221–226 | `categoria_espacio`, `espacio`, `regla_tarifa`, `regla_comision`, `politica_cancelacion`, `tramo_cancelacion`, `documento`; `ocupacion` coordinada con M06 |
| M05 Búsqueda y cotización | Descubrimiento geográfico, filtros, detalle y cotización reproducible. | HU10–14 | CU-19–21 | 094–107 | `cotizacion`, lectura de `espacio`, `categoria_espacio`, tarifas, políticas y ocupación |
| M06 Reservas y pagos | Crear/seguir/cancelar reserva, ocupar calendario y registrar intentos de pago/garantía. | HU15–19 | CU-22–28, CU-47, CU-51 | 108–129, 199–201, 227–231 | `reserva`, `ocupacion`, `reserva_transicion`, `pago`, `evento_proveedor`, `movimiento_financiero`, `garantia`, reglas y cotización |
| M07 Contratos y firma | Generar versiones de contrato, solicitudes de firma y resguardo del resultado. | HU33 | CU-29–32 | 130–142, 202 | `contrato`, `firma_contrato`, `documento` |
| M08 Operación del arriendo | Check-in/out, confirmación de recepción y evidencia autorizada. | HU35 | CU-33–34, CU-48 | 143–152, 203–206 | `operacion_arriendo`, `documento`, `reserva_transicion` |
| M09 Comunicación y reputación | Chat por reserva, reseñas/reportes y notificaciones. | HU25, HU30, HU34 | CU-35–38, CU-49 | 153–158, 207, 232–235; avisos vinculados | `mensaje_reserva`, `resena`, `reporte_resena`, `notificacion`, `entrega_notificacion` |
| M10 Disputas, liquidación y tributación | Reclamos, descargos, resolución, liquidación observada y documento tributario. | HU28; HU29 (centro de ayuda) no tiene RQF/CU y queda fuera del core planificado | CU-39–42 | 159–177, 208–211 | `disputa`, `garantia`, `liquidacion`, `movimiento_financiero`, `documento_tributario`, `notificacion` |
| M11 Administración y auditoría | Gobierno de cuentas, moderación, reportes y trazabilidad. | HU26–27 | CU-43–46, CU-52 | 178–185, 212, 236 | `evento_auditoria`, `outbox_evento`, lecturas acotadas de otras tablas, promociones y derechos de reporte |

**Nota de asignación:** las HUs se agrupan por épica y no siempre coinciden uno a uno con los módulos funcionales. La matriz anterior es una propuesta de planificación, no una reasignación académica; las relaciones exactas RQF ↔ CU ↔ HU prevalecen en los anexos ES1. La tabla completa y verificable de IDs se mantiene en el Anexo A de ES1.

## Requerimientos transversales

Seguridad, autenticación/autorización por recurso, privacidad/Ley 21.719 como criterio desde el primer incremento, observabilidad sin PII, auditoría, idempotencia, contratos OpenAPI, accesibilidad del mock, concurrencia, backup/restore, límites de retención, migraciones reproducibles y errores consistentes cruzan varios módulos. Se planifican como fundación y condiciones de aceptación de cada módulo, no como funcionalidades opcionales posteriores.

## Exclusiones de esta planificación

- No se define framework, navegación, UX/UI, diseño visual, componentes, estado ni arquitectura de frontend de producción.
- El mock temporal usa solo HTML, CSS, TypeScript compilado, DOM y `fetch`, alojado en su contenedor separado.
- No se decide proveedor final de firma/KYC/pagos ni se afirma que Split equivalga a Escrow.
- No se despliega una nube productiva ni se fijan tamaños de capacidad sin benchmarks.
