# Publicación autorizada del código del plan en main

## Decisión humana vigente

El usuario autorizó expresamente integrar y publicar en main el código del plan sin ejecutar pruebas previas. Las pruebas funcionales las realizará después de publicado. Ordenó conservar hooks y controles automáticos, e informar cualquier bloqueo concreto sin omitirlos. Esta decisión sustituye el gate de pruebas manuales previas para esta publicación; no declara aprobado el producto ni obliga a pasar un CI que aún no se ha ejecutado.

También autorizó exclusivamente `storage.objects.get` temporal sobre el lock y los objetos exactos necesarios para la inspección, con revisión previa y retirada verificada. No autorizó cambios de permisos SQL, roles de base ni desbloqueo. La publicación de código no requiere conceder ese permiso: no se aplica un grant preventivo o innecesario, ni se supedita el push a una inspección que tiene su propio circuito revisado.

## Fuente publicada y preservación

- Candidato aislado: `/home/nandev/Documentos/projects/espaciGo-spaces-main-preparation`.
- Rama renombrada a `codex/marketplace/chore/spaces-01-main-preparation` para cumplir el patrón numérico del hook vigente; no se modificó ni deshabilitó el hook.
- Base del candidato `647cd491985d619370990d3f226b5df61e36b42b`, que contiene origin/main `5830c9ce2dc61d4df13a4139da984b4cafd6d9ba`.
- Main local en su worktree conserva el commit documental previo `75c8c21d30c70a4fd63f6109664d5c2a063410df`; se integrará mediante merge normal, sin descartar ese commit ni reescribir historia.
- Los worktrees fuente y sus cambios previos quedan conservados. El manifiesto de importación registra las 91 copias iniciales; no es un hash del candidato final.

## Cambios reunidos

Recuperación del primer borrador, calendario/intervalos, cola y galería privada/pública, validación y adaptadores OpenAPI, evidencia del plan y cobertura automatizada versionada. Además: selección del bucket de media en API, sincronización de su variable en el release, contrato JSON Go/Python de planes migradores, y recursos Terraform opt-in privados sin doble dirección para media.

La revisión parental detectó que usar el marcador completo de operación como scope de recuperación también cambia al refrescar roles. Se añadió `AuthFacade.getSessionIdentityScope()` basado solo en generación de sesión y cuenta, sin credenciales ni roles, y el wizard liga el pendiente al scope de origen. Se conservan el control de cuenta del enlace y el reset de ruta. Es cambio fuente revisado, no una regresión ejecutada ni una afirmación de ausencia de otras carreras.

## Migraciones y estado remoto

84 archivos SQL coinciden byte a byte con origin/main, incluido V84 Didit ya registrado. No se añadió ni reescribió ninguna migración. No se consultó ni modificó la base persistente local, no se ejecutó DDL cloud, ni se cambiaron permisos/roles SQL.

Cloud SQL no queda sincronizada por el push: el fallo histórico V84 `42501` y el lock conservado siguen requiriendo inspección actual y resolución autorizada independiente. El inspector publicado fija una generación de lock anterior a su restauración; no se ejecuta a ciegas contra el nuevo objeto. Los límites y metadata actuales están en `espacios-main-publication-gates.md`.

## Verificación efectuada y omitida

- Efectuadas: inspección source/diff, SHA-256 de copias e inventario SQL, ascendencia Git, identidad efectiva gh/Git/autor `HernanEspinozaDev`, hooks, sintaxis Python, formato Terraform y diff-check.
- **No ejecutadas antes de publicar:** pruebas unitarias, integración SQL, Playwright, navegador, build de producto, typecheck y regresiones de los últimos cambios. La evidencia histórica del plan no se presenta como ejecución sobre este candidato final.
- CI y Cloud Build conservan su configuración y se activarán normalmente por la publicación. No se cancelan, desactivan ni omiten. El resultado se consultará por el SHA exacto después del push.
- Publicado, integrado, desplegado y aceptado son estados diferentes. Ningún Ready antiguo ni éxito del push prueba despliegue o aceptación funcional.
