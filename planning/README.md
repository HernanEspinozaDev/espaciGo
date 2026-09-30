# Planificación de desarrollo de EspaciGo

## Propósito

Este directorio contiene la línea base evolutiva para construir EspaciGo con prioridad en **Base de Datos → Backend → API → pruebas → validación visual con frontend mock**. Es planificación, no evidencia de implementación. La interfaz definitiva queda fuera de alcance y se decidirá cuando el backend y la persistencia hayan avanzado.

## Documentos

| Documento | Contenido |
| --- | --- |
| [Visión y módulos](vision_y_modulos.md) | Problema, alcance del producto, módulos y trazabilidad ES1/ES2. |
| [Contexto para agentes](contexto_para_agentes.md) | Decisiones, invariantes y precedencia de fuentes sin depender de Informes. |
| [Ejecución con Hermes](ejecucion_con_hermes.md) | Cómo cargar contexto, asignar tarjetas y separar Developer/Tester/Reviewer. |
| [Plan de Base de Datos](base_de_datos.md) | Modelo global, límites por módulo, secuencia y reglas de migración. |
| [Plan de Backend y API](backend_y_api.md) | Capas Go, contratos HTTP/JSON y convenciones previstas. |
| [Plan de pruebas](pruebas.md) | Estrategia desde unitarias hasta recorridos funcionales. |
| [Frontend mock](frontend_mock.md) | Harness temporal por módulo, tecnología permitida y criterios de aceptación. |
| [Decisiones y hallazgos](decisiones_y_hallazgos.md) | Conflictos documentales, riesgos y temas que requieren evidencia. |
| [Grafo de dependencias](grafo_dependencias.md) | Orden, dependencias transversales y conteo del backlog. |
| [Backlog Kanban](backlog.md) | Registro completo de tarjetas con alcance, trazabilidad y aceptación. |
| [Referencias](referencias/README.md) | Snapshots de ES1/ES2 para agentes y copia del roadmap recibido. |

La tarjeta fuente es `backlog.md`; se puede trasladar después a Hermes u otro tablero. En esta sesión no hay conector de Hermes Kanban disponible, así que no se afirma que exista una carga en un tablero externo.

## Fuentes de autoridad

1. Restricciones explícitas de la solicitud del usuario y `planificardesarrollo.md`.
2. ES2 vigente: propuesta consolidada de backend, sección 3, Anexo B (diccionario de 43 tablas), casos de prueba y pendientes.
3. ES1 cerrada: anexos A–E con módulos, 236 RQF, 43 RNF, 52 CU y 35 HU. ES1 se consulta como línea base; no se modifica.
4. `espaciGo/README.md` solo como descripción del repositorio, no como autoridad cuando contradiga las decisiones posteriores.

## Estado inicial

La inspección inicial de `espaciGo/` encontró el README y el archivo de instrucciones; esta carpeta ahora contiene la documentación de planificación. No se encontró implementación de aplicación, esquema físico, migraciones, API, compose ni tablero Kanban existente. Las tarjetas se entregan inicialmente `ready` solo para la revisión y refinamiento del mapa global; las demás comienzan `todo` hasta que se resuelvan sus dependencias.

La planificación incluye los once módulos académicos y considera el producto completo, con construcción incremental. Las propuestas evolucionan mediante tickets nuevos y cambios trazables; ningún cambio de esquema se hará fuera de una migración explícita. `referencias/` incorpora los anexos completos y el diseño ES2 necesarios para que agentes trabajando solo desde este repo puedan resolver los IDs de trazabilidad.
