# Planificación de desarrollo de EspaciGo

## Propósito

Este directorio contiene la línea base evolutiva para construir EspaciGo con prioridad en **Base de Datos → Backend → API → pruebas → validación visual con frontend mock**. Es planificación, no evidencia de implementación. La interfaz definitiva queda fuera de alcance y se decidirá cuando el backend y la persistencia hayan avanzado.

## Prioridad vigente — cierre local completo, actualizado 2026-10-10

El [plan de cierre del backend local](plan_cierre_backend_local.md) organiza el trabajo restante de M01–M11 y define la aceptación LOCAL-1. El [backlog de cierre](backlog_cierre_backend_local.md) propone paquetes, dependencias y una validación mock final por módulo; la Issue operativa #200 representa LOCAL-KYC-01. El [prompt de ejecución](prompt_cierre_backend_local.md) permite entregar esa instrucción al agente de desarrollo.

La matriz se actualizó tras los merges #213, #225 y #227. M01 conserva criterios generales abiertos pese a #182/#198; M02 incluye exportación ZIP (#196), baja/purga/replay (#192/#194) y outbox terminal (#198), pero #185/#40 siguen abiertas. LOCAL-KYC-01 (#200) implementa elegibilidad sintética; #204/#205 conecta el gate a publicación/ocultación local. #206–#212 aceptan slices locales de M04/M05 sin cerrar los padres. LOCAL-FIN-01 (#224) se aceptó tras #225/V40 para garantía/deducción fake; evidencia en [la entrega financiera](evidence/local-fin-01-20261009.md). LOCAL-ADMIN-01A (#226) se aceptó tras PR #227; evidencia en [LOCAL-ADMIN-01A](evidence/local-admin-01a-20261009.md). LOCAL-ADMIN-01B/#228 está `En curso`: consulta/exportación mínima de ledger, sin declarar cubiertos #111–#118. Evidencia de implementación y matriz de productores en [LOCAL-ADMIN-01B](evidence/local-admin-01b-20261009.md). LOCAL-ADMIN-01/M11 y LOCAL-CORE-02 no están completos/desbloqueados; aceptar slices no cierra padres generales.

**Completar todo el alcance local antes de proponer pruebas o despliegue GCP.** No ejecutar esa fase como parte del cierre. Se reutilizan capacidades aceptadas y adaptadores fake explícitos; el resultado no acredita integraciones reales ni cierre global del producto.

## Documentos

| Documento | Contenido |
| --- | --- |
| [Plan de cierre local](plan_cierre_backend_local.md) | Objetivo actual M01–M11, hitos L0–L5, decisiones, pruebas locales y definición LOCAL-1; GCP queda para después. |
| [Backlog de cierre local](backlog_cierre_backend_local.md) | Paquetes propuestos, trazabilidad a Issues, aceptación y once tarjetas finales de mock; no son cambios operativos del Project. |
| [Prompt de cierre local](prompt_cierre_backend_local.md) | Instrucción para la ejecución posterior, con cuentas GitHub, primera entrega y condición de parada. |
| [Evaluación de alcance al 2026-10-07](alcance_backend_local_y_transicion_gcp_20261007.md) | Inventario de partida y comparación histórica; la secuencia vigente es la del plan de cierre local. |
| [Visión y módulos](vision_y_modulos.md) | Problema, alcance del producto, módulos y trazabilidad ES1/ES2. |
| [Contexto para agentes](contexto_para_agentes.md) | Decisiones, invariantes y precedencia de fuentes sin depender de Informes. |
| [Ejecución con Hermes](ejecucion_con_hermes.md) | Guía histórica del board Hermes archivado; no usar para asignar ni despachar trabajo. |
| [Entorno de desarrollo](entorno_de_desarrollo.md) | Contexto de herramientas disponibles, separado de los requisitos del producto. |
| [Plan de Base de Datos](base_de_datos.md) | Modelo global, límites por módulo, secuencia y reglas de migración. |
| [Entorno local aislado](entorno_local_aislado.md) | CORE-ENV-01: topología y contrato database/backend/mock, redes, readiness, configuración, DB de test, CORS y checklist. |
| [Plan de Backend y API](backend_y_api.md) | Capas Go y contrato HTTP/JSON. |
| [Contrato HTTP común](contrato_http_api.md) | Versionado, autenticación/autorización, errores, paginación, formatos y guía para tickets de rutas. |
| [OpenAPI base](openapi.yaml) | OpenAPI 3.1 común con seguridad Bearer, esquemas y respuestas reutilizables. |
| [Prototipo local M03](prototipo_local_m03.md) | KYC/KYB sintético, revisión por rol administrador y reintento local. |
| [Decisiones M03](decisiones_m03_verificacion_local.md) | Contrato fixture, privacidad y capacidades confirmadas vs pendientes. |
| [LOCAL-KYC-01](decisiones_local_kyc_eligibilidad.md) | Elegibilidad sintética persistente por tipo, subsanación, auditoría y gates de nuevas reservas/publicación owner-only; catálogo general M04/M05 sigue pendiente. |
| [Evidencia sintética M03](evidence/m03-evidencia-sintetica-local.md) | Alcance temporal, recorrido API/mock, acceso y limpieza puntual de archivos sintéticos de #47. |
| [Referencia Realmo](referencia_realmo_categorias_y_ficha.md) | Referencia de producto para categorías, características y contenido de la ficha; propuestas para evaluar. |
| [Decisiones M04 catálogo de atributos](decisiones_m04_catalogo_atributos.md) | Contrato implementado de V6, perfiles versionados, validación y límites del prototipo. |
| [Catálogo extensible y atributos](catalogo_extensible_categorias_y_atributos.md) | Catálogo versionado de características para ocho categorías y trazabilidad del corte M04-ATTR-01. |
| [Decisión M05-LOCAL-02](decisiones_m05_busqueda_filtros_locales.md) | Filtros tipados por perfil versionado, disponibilidad y total CLP estimado sobre fixtures autorizados. |
| [Estructura lógica del backend Go](estructura_backend_go.md) | CORE-BE-01: límites de paquetes, ownership SQL, transacciones, salud y workers durables. |
| [Plan de pruebas](pruebas.md) | Estrategia desde unitarias hasta recorridos funcionales. |
| [Harness de DB y API](harness_pruebas.md) | Comandos por capa, fixtures sintéticos, aislamiento, migraciones, contratos y evidencia CI. |
| [Frontend mock](frontend_mock.md) | Harness temporal por módulo, tecnología permitida y criterios de aceptación. |
| [Decisiones y hallazgos](decisiones_y_hallazgos.md) | Conflictos documentales, riesgos y temas que requieren evidencia. |
| [Decisiones M01 de identidad](decisiones_m01_identidad_sesion.md) | Autorización ratificada de correo, sesiones y tokens; trazabilidad ES1/ES2 y brechas DB02-09 abiertas. |
| [Decisión M05 de búsqueda local](decisiones_m05_busqueda_filtros_locales.md) | Filtros AND tipados para fixtures, cotización estimada inclusiva y sin mutar reservas. |
| [Evidencia M05-LOCAL-02](evidencia_m05_filtros_locales_20261006.md) | Pruebas de búsqueda y comprobación del mock local con fixtures existentes. |
| [Decisión M05-LOCAL geográfica](decisiones_m05_busqueda_geografica_local.md) | Radios PostGIS y ubicaciones sintéticas explícitas para fixtures autorizados. |
| [Evidencia M05-LOCAL geográfica](evidencia_m05_busqueda_geografica_20261006.md) | Migración V018, validaciones PostGIS/runtime, pruebas, estado del mock y pasos de aceptación. |
| [Invariantes de identidad, cuenta y sesión](invariantes_identidad_sesion.md) | AUTH-ARCH-01: flujos, estados, amenazas y reglas M01 ratificadas. |
| [Grafo de dependencias](grafo_dependencias.md) | Orden, dependencias transversales y conteo del backlog. |
| [Backlog Kanban](backlog.md) | Alcance/trazabilidad de las 103 tarjetas originales; estados históricos, no operativos. |
| [Migración a GitHub Projects](migracion_github_projects.md) | Reconciliación del snapshot Hermes, Issues, estados, dependencias, archivadas y respaldo portable. |
| [Ejecución en GitHub Projects](ejecucion_con_github_projects.md) | Flujo seguro de trabajo desde el PC local; gates y comandos de verificación. |
| [Correspondencia Hermes–GitHub](github_issue_map.csv) | Snapshot de migración: 104 IDs Hermes, Issues, estados, PR relacionado y racional; no contiene prioridades ni aristas de dependencias. |
| [Referencias](referencias/README.md) | Snapshots de ES1/ES2 para agentes sin acceso al repositorio académico. |

`backlog.md` conserva el alcance y la trazabilidad de las 103 tarjetas originales; sus estados y conteos anteriores son snapshots históricos, no el tablero operativo actual. La recuperación `t_112e6827` es la tarjeta 104. El registro operativo está en GitHub Issues + Project `EspaciGo — Desarrollo` (104 elementos, dependencias nativas de GitHub); usa [migracion_github_projects.md](migracion_github_projects.md) y [github_issue_map.csv](github_issue_map.csv) para la conciliación al 2026-10-05. Hermes Kanban queda archivado como histórico y fuera del dispatcher activo.

## Fuentes de autoridad

1. Restricciones explícitas de la solicitud del usuario y `planificardesarrollo.md`.
2. ES2 vigente: propuesta consolidada de backend, sección 3, Anexo B (diccionario de 43 tablas), casos de prueba y pendientes.
3. ES1 cerrada: anexos A–E con módulos, 236 RQF, 43 RNF, 52 CU y 35 HU. ES1 se consulta como línea base; no se modifica.
4. `espaciGo/README.md` solo como descripción del repositorio, no como autoridad cuando contradiga las decisiones posteriores.

## Estado inicial (registro histórico)

La inspección inicial de `espaciGo/` encontró el README y el archivo de instrucciones; entonces no se encontró implementación de aplicación, esquema físico, migraciones, API, compose ni tablero Kanban. Posteriormente el backlog se importó al tablero local Hermes `espacigo`. Al 2026-10-01, `PLAN-ARCH-01` está `done`, `CORE-ARCH-01` en `review` y las tarjetas posteriores siguen pendientes según sus dependencias; esto no es evidencia de implementación.

La planificación incluye los once módulos académicos y considera el producto completo, con construcción incremental. Las propuestas evolucionan mediante tickets nuevos y cambios trazables; ningún cambio de esquema se hará fuera de una migración explícita. `referencias/` incorpora los anexos completos y el diseño ES2 necesarios para que agentes trabajando solo desde este repo puedan resolver los IDs de trazabilidad.
