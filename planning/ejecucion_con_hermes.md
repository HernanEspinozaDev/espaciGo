# Guía para ejecutar el backlog con Hermes

## ¿Le basta el plan actual a un agente sin acceso a Informes?

**El backlog solo, no.** Tiene IDs RQF/CU/HU y acceptance criteria resumidos, pero las fichas completas y varios límites están en ES1/ES2. Darle únicamente `backlog.md` haría que el agente rellene vacíos por inferencia. Para resolverlo, este repositorio incluye snapshots de los anexos completos en [`referencias/`](referencias/README.md), además del contexto operativo. Así Hermes no necesita leer el repositorio hermano `Informes/` para entender los requisitos de producto y arquitectura que gobiernan estas tareas.

El contexto sí basta para ejecutar el plan **solo si Hermes recibe estos documentos locales**. No elimina decisiones externas abiertas: acceso/cotización de proveedores, validación contable/legal, demanda, criterios docentes y ensayos reales requieren evidencia cuando aplique.

## Contexto que se entrega a cada agente

### Al iniciar sesión de trabajo

1. `planning/README.md`
2. `planning/contexto_para_agentes.md`
3. `planning/grafo_dependencias.md`
4. `planning/backlog.md`
5. `AGENTS.md` u otras instrucciones vigentes del repositorio, si existen.

### Por tarjeta asignada

Entregar al agente el ID y sección íntegra de la tarjeta, más:

- su módulo en `planning/vision_y_modulos.md`;
- documento de capa relevante (`base_de_datos.md`, `backend_y_api.md`, `pruebas.md` o `frontend_mock.md`);
- referencias exactas identificadas por la tarjeta: requisito RQF, CU/HU, tabla en ES2;
- estado de dependencias desde Hermes y el repositorio.

No hace falta mandar los 100+ tickets y todos los anexos al contexto de cada worker si Hermes puede leer el repositorio. Si Hermes no tiene lectura del workspace, adjuntar el contenido local mínimo de esos archivos en el prompt.

## Ciclo recomendado de ejecución

1. **Orquestador:** elige una tarjeta `ready` sin dependencias abiertas, confirma que no haya otra tarea editando los mismos archivos y asigna un solo Developer.
2. **Developer:** recibe una tarjeta, implementa solo su alcance, añade sus pruebas, registra decisiones y evidencia; si descubre trabajo extra, propone ticket nuevo en vez de agrandar silenciosamente el actual.
3. **Tester:** revisa los criterios de aceptación y ejecuta pruebas pertinentes en entorno reproducible; distingue fake, sandbox y proveedor real.
4. **Reviewer:** valida seguridad, privacidad, transacciones, migraciones, contrato API y trazabilidad. No aprueba una tarjeta por existencia de código solamente.
5. **Orquestador:** sincroniza estado de Hermes con el log versionado del repositorio, captura findings/dependencias, y habilita sucesores solo cuando Developer/Tester/Reviewer y criterios de cierre lo permitan.

Database va primero dentro de cada módulo. Para cambiar de módulo, respeta el grafo; M02/M03 pueden avanzar en paralelo después de M01 si el equipo separa ownership. No paralelizar migraciones incompatibles o dos tareas sobre el mismo dominio sin coordinación.

## Tarjeta que puede comenzar

La única tarjeta `ready` inicial es `PLAN-ARCH-01 — Revisar y fijar mapa global de dominio`. Eso es refinamiento de arquitectura, no autorización para comenzar implementación. Al cerrarla, el backlog identifica `CORE-ARCH-01` y después las convenciones/modelado de DB como siguientes pasos. El usuario debe dar la orden explícita para iniciar desarrollo, de acuerdo con la condición de terminación del archivo `planificardesarrollo.md`.

## Prompt base para una asignación

```text
Trabaja solo en la tarjeta <ID> del repositorio espaciGo.
Lee planning/contexto_para_agentes.md, la tarjeta completa en planning/backlog.md,
la referencia exacta RQF/CU/HU/tablas indicada allí y la instrucción de tu rol.
Respeta dependencias, fuera de alcance, prioridad DB → Backend → API → pruebas → mock,
y no decidas el frontend definitivo. No inventes evidencia externa.
Entrega: cambios, comandos/pruebas ejecutadas con resultados, riesgos, decisiones,
trazabilidad y tickets adicionales propuestos. No marques la tarjeta done por cuenta propia.
```

El coordinador adapta el prompt al perfil Developer, Tester o Reviewer y proporciona los permisos/contexto reales del entorno Hermes. No se supone un comando, API, configuración de workers ni modo de aislamiento específico de Hermes en este documento.

## Estado y trazabilidad

- `backlog.md` es el log de planificación versionado; Hermes representa el estado de ejecución cuando se configure el tablero.
- Al importar, conservar IDs, tipos, dependencias, aceptación y estado inicial. No crear tarjeta genérica que reemplace estos criterios.
- Cada PR/entrega enlaza ID del ticket y RQF/CU/HU tocados. Cada migración referencia ticket y decisión de modelo.
- Actualizar el backlog cuando cambien alcance/dependencias. Mantener hallazgos y pendientes externos en documentos de planificación del repo, con fecha/evidencia.
- No entregar secretos/PII en contexto de agente. Usar fixture sintética y sandbox.

## Contexto del entorno

El documento que el usuario compartió con la descripción de su entorno no forma parte de las fuentes de producto y no se incorpora como roadmap de EspaciGo. Consulta [entorno_de_desarrollo.md](entorno_de_desarrollo.md) para verificar herramientas. Las reglas del mock de este repositorio prevalecen sobre técnicas mencionadas como ejemplo de ambiente.
