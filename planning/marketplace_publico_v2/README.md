# Desarrollo local versión 2 del marketplace público

**Identificador:** MARKETPLACE-LOCAL-2 (MPV2). **Fecha:** 2026-10-13.

Este es el plan de evolución de EspaciGo después de FRONTEND-LOCAL-1. El visitante podrá buscar, filtrar y consultar espacios publicados sin cuenta; se identificará cuando decida iniciar una operación privada.

## Documentos de trabajo

- [Plan y cortes de desarrollo](plan_desarrollo_local_v2.md): diagnóstico, decisiones, contratos propuestos y secuencia base de datos → Go → Angular.
- [Matriz de aceptación](matriz_aceptacion.md): comprobaciones por corte, evidencias y bloqueos de cierre.
- [Contrato MPV2-00](contrato_mpv2_00.md): inventario trazado a `main` a68da0c, contrato público, predicado de visibilidad, matriz de campos y delta de datos propuesto.
- [Propuesta aportada por el usuario](referencias/especificacion_marketplace_publico_inmobiliario.md): copia de referencia, sin modificar.

La propuesta es una guía. Sus frases de aprobación, instrucciones al equipo, ejemplos residenciales y referencias a Cloud Run no constituyen resultados de esta revisión ni autorizan esos alcances por sí solos. El plan adapta la experiencia pública al modelo real de EspaciGo.

El flujo de ramas y cuentas se rige por [`AGENTS.md`](../../AGENTS.md): `main` es la única rama permanente. Desarrollo y PR con HernanMEC; revisión/aprobación/fusión con HernanEspinozaDev. Ejecutar `scripts/git-role.sh development` antes de trabajar y usar ramas temporales como `codex/marketplace/feature/mpv2-01-public-catalog`.

**Estado:** planificación preparada; cortes pendientes de implementación. No se han cambiado tablas, API, permisos ni componentes por elaborar estos documentos. FRONTEND-LOCAL-1 y la aceptación de FE-14 conservan su condición histórica.

## Instrucción para comenzar

> Inicia MARKETPLACE-LOCAL-2 por MPV2-00 desde `main` actualizado. Lee AGENTS.md y ejecuta scripts/git-role.sh development; verifica HernanMEC antes de commit, push y PR. Usa codex/docs/chore/mpv2-00-public-contract para el inventario/contrato inicial y solicita revisión a HernanEspinozaDev. Después entrega MPV2-01 en codex/marketplace/feature/mpv2-01-public-catalog siguiendo datos → Go/OpenAPI → Angular dentro del mismo corte. No avances al siguiente corte hasta que el anterior esté aprobado y fusionado. Conserva la arquitectura por features y hexagonal ligera; Go es la autoridad. Usa nuevos datos sintéticos y stacks aislados; las credenciales quedan locales y excluidas de Git. No basta con retirar guards: catálogo, ficha y fotos requieren proyecciones públicas seguras del servidor. Tras cada fusión, limpia rama y entorno del corte.
