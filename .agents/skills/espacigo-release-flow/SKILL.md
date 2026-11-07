---
name: espacigo-release-flow
description: Aplicar el flujo vigente de EspaciGo al preparar entregas, integrar a main, versionar migraciones PostgreSQL y diagnosticar releases de Cloud SQL/Cloud Run sin bastión. Usar solo en el repositorio EspaciGo; no define despliegues para otros proyectos.
---

# Flujo de entrega de EspaciGo

Usar el repositorio de la tarea como fuente de verdad; no asumir que el checkout habitual o un recibo antiguo contienen la última versión. Esta skill fija el procedimiento, no concede autorización para desplegar trabajo ajeno al encargo.

## Antes de actuar

Leer `AGENTS.md`, `planning/gcp/cloudsql_migraciones_incrementales.md` y `planning/flujo_entornos/operacion_main_sin_bastion.md` desde la raíz del repositorio seleccionado. Las rutas de esta skill son relativas a esa raíz. Si falta una fuente o hay divergencia, comprobar rama/remoto y preservar cambios ajenos; no volver a un runbook SSH histórico.

Main es la única rama permanente. Desarrollar/probar/revisar en una rama temporal `codex/<área>/<tipo>/<identificador>-<descripción>` y completar sus gates antes de integrar normalmente a main, sin PR ni force-push. Usar `scripts/git-role.sh direct` y verificar `check-direct`, hooks, diff, pruebas y refs antes de publicar como HernanEspinozaDev. Respetar restricciones específicas de integración del corte, incluida la del rediseño frontend. Un push de rama de tarea no equivale a integración ni despliegue.

## Preparar código y migraciones

Los datos permanecen en Cloud SQL; Git transporta código y SQL versionado. No exportar/restaurar la base local sobre la compartida. Cambiar structs o tablas locales no genera automáticamente una migración.

Conservar SQL aplicado byte a byte. Para SQL nuevo, revisar `db/release-policy.json` y añadir la migración con nombre/checksum exactos y `strategy=expand`; probar base vacía y actualización en PostgreSQL aislado, compatibilidad con el backend activo y autoridad DDL por objeto. No inventar migraciones para un release sin delta. Repairs, cambios destructivos y permisos nuevos necesitan su corte acotado y revisado; no editar ledger, saltar guards ni elevar runtime.

## Publicación automática

Consultar `cloudbuild.yaml` y `scripts/gcp/release_main.py`, sin recrear su secuencia manualmente:

- Publicar main activa el único trigger y espera calidad del mismo SHA.
- Clasificación docs: sin SQL ni rollout; web: comprueba backend intacto; full: lock global por generación y todas las imágenes del mismo SHA con digests/provenance.
- Full ejecuta plan privado → backup válido → Job `cmd/dbmigrate` → ledger/checksums/catálogo IMG-01 → revisiones sin tráfico → promoción verificada → evidencia inmutable y limpieza propia confirmada.

El Job conecta por red privada con identidad migradora y secreto montado en runtime. No entregar payload SQL al builder ni ejecutar DDL en el arranque de la API. El configurador delegado tiene autoridad efectiva DDL; cambios al pipeline son cambios de infraestructura que deben revisarse.

El bastión, su disco y reglas exclusivas están eliminados. No arrancar/recrear VM, abrir SSH/forward ni migrar desde el portátil. No usar DMS como actualización de esquema. Los runbooks anteriores son historia, nunca fallback.

## Fallos y cierre

Ante fallo o resultado incierto, detener nuevos intentos de aplicación e inspeccionar build, ID de ejecución, recibos, ledger/catálogo y generación del lock. No deducir ausencia de escritura de la cancelación del padre o de logs faltantes. No cancelar un Job que podría escribir ni borrar locks por nombre. `scripts/gcp/recover_release_lock.py` solo permite recuperación preapply demostrada; revisar la guía antes de usar su modo mutante.

SQL no se revierte automáticamente. Rollback de aplicación solo a revisión compatible; si hay drift, preparar corrección versionada. Conservar evidencia y recursos inciertos para diagnóstico; limpiar solo recursos propios confirmados.

Aplicar los umbrales vigentes del verificador de backups: automáticos ≥7 copias, metadata ≤5 minutos y backup exitoso ≤24 horas sin SQL pendiente; con SQL pendiente, ON_DEMAND exitoso iniciado después del lock. Retención propia conserva siete más recientes y ≥7 días según recibos; no borrar ajenos/históricos/fallos. No inferir PITR ni RPO/RTO de un backup exitoso.

Registrar SHA, pruebas realmente ejecutadas, build/ejecuciones, esquema, revisiones, resultado y límites sin secretos ni datos de negocio. Consultar CI antes de declararlo exitoso. Ready y metadata no prueban aceptación funcional. QA online del producto sigue reservada al usuario: no usar navegador del usuario, cuentas reales ni aplicación pública para comprobar el producto.

## Etapa actual

Una instancia Cloud SQL de desarrollo, sin develop ni staging. El nombre histórico de la base no crea un entorno staging. No apagar Cloud SQL compartido tras pruebas. Antes de primeros usuarios preparar staging, PITR, retención y recuperación según un nuevo alcance; no presentarlos como activos. Leer el recibo `planning/flujo_entornos/evidence/cierre_tecnico_flujo_inicial.json` solo para antecedentes de activación; verificar estado actual para cada operación.
