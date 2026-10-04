# Planificación de desarrollo de EspaciGo

## Propósito

Este directorio contiene la línea base evolutiva para construir EspaciGo con prioridad en **Base de Datos → Backend → API → pruebas → validación visual con frontend mock**. Es planificación, no evidencia de implementación. La interfaz definitiva queda fuera de alcance y se decidirá cuando el backend y la persistencia hayan avanzado.

## Documentos

| Documento | Contenido |
| --- | --- |
| [Visión y módulos](vision_y_modulos.md) | Problema, alcance del producto, módulos y trazabilidad ES1/ES2. |
| [Contexto para agentes](contexto_para_agentes.md) | Decisiones, invariantes y precedencia de fuentes sin depender de Informes. |
| [Ejecución con Hermes](ejecucion_con_hermes.md) | Cómo cargar contexto, asignar tarjetas y separar Developer/Tester/Reviewer. |
| [Entorno de desarrollo](entorno_de_desarrollo.md) | Contexto de herramientas disponibles, separado de los requisitos del producto. |
| [Plan de Base de Datos](base_de_datos.md) | Modelo global, límites por módulo, secuencia y reglas de migración. |
| [Plan de Backend y API](backend_y_api.md) | Capas Go y contrato HTTP/JSON. |
| [Contrato HTTP común](contrato_http_api.md) | Versionado, autenticación/autorización, errores, paginación, formatos y guía para tickets de rutas. |
| [OpenAPI base](openapi.yaml) | OpenAPI 3.1 común con seguridad Bearer, esquemas y respuestas reutilizables. |
| [Estructura lógica del backend Go](estructura_backend_go.md) | CORE-BE-01: límites de paquetes, ownership SQL, transacciones, salud y workers durables. |
| [Plan de pruebas](pruebas.md) | Estrategia desde unitarias hasta recorridos funcionales. |
| [Harness de DB y API](harness_pruebas.md) | Comandos por capa, fixtures sintéticos, aislamiento, migraciones, contratos y evidencia CI. |
| [Frontend mock](frontend_mock.md) | Harness temporal por módulo, tecnología permitida y criterios de aceptación. |
| [Decisiones y hallazgos](decisiones_y_hallazgos.md) | Conflictos documentales, riesgos y temas que requieren evidencia. |
| [Grafo de dependencias](grafo_dependencias.md) | Orden, dependencias transversales y conteo del backlog. |
| [Backlog Kanban](backlog.md) | Registro completo de tarjetas con alcance, trazabilidad y aceptación. |
| [Referencias](referencias/README.md) | Snapshots de ES1/ES2 para agentes sin acceso al repositorio académico. |

`backlog.md` es el registro fuente de las 103 tarjetas del tablero Hermes Kanban local `espacigo`; la sección de estado documenta las diferencias de estado y sus gates. No se debe inferir que el tablero sea un servicio externo ni que se haya probado cada canal de notificación por el solo hecho de registrar estados.

## Fuentes de autoridad

1. Restricciones explícitas de la solicitud del usuario y `planificardesarrollo.md`.
2. ES2 vigente: propuesta consolidada de backend, sección 3, Anexo B (diccionario de 43 tablas), casos de prueba y pendientes.
3. ES1 cerrada: anexos A–E con módulos, 236 RQF, 43 RNF, 52 CU y 35 HU. ES1 se consulta como línea base; no se modifica.
4. `espaciGo/README.md` solo como descripción del repositorio, no como autoridad cuando contradiga las decisiones posteriores.

## Estado inicial

La inspección inicial de `espaciGo/` encontró el README y el archivo de instrucciones; entonces no se encontró implementación de aplicación, esquema físico, migraciones, API, compose ni tablero Kanban. Posteriormente el backlog se importó al tablero local Hermes `espacigo`. Al 2026-10-01, `PLAN-ARCH-01` está `done`, `CORE-ARCH-01` en `review` y las tarjetas posteriores siguen pendientes según sus dependencias; esto no es evidencia de implementación.

La planificación incluye los once módulos académicos y considera el producto completo, con construcción incremental. Las propuestas evolucionan mediante tickets nuevos y cambios trazables; ningún cambio de esquema se hará fuera de una migración explícita. `referencias/` incorpora los anexos completos y el diseño ES2 necesarios para que agentes trabajando solo desde este repo puedan resolver los IDs de trazabilidad.
