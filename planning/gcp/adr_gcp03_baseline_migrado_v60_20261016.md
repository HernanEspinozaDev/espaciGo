# ADR GCP-03-ADR-01: baseline migrado para inicializar Cloud SQL

**Estado:** propuesta para revisión; no aceptada.

**Fecha:** 2026-10-16.

**Decisión solicitada:** evaluar si las instalaciones nuevas de Cloud SQL pueden inicializarse restaurando un baseline validado, en vez de ejecutar V1–V61 en una base Cloud SQL vacía.

## Contexto

La secuencia vigente exige conservar V1–V61 byte a byte y el plan de despliegue pide validar el replay completo en una base vacía. V60 exige que `espacigo_gadm_bootstrap` no tenga membresías y, después de crearla, ejecuta `COMMENT ON ROLE`. En PostgreSQL 18, un operador sin privilegio `ADMIN OPTION` no puede comentar ese rol; el probe local con un rol `CREATEROLE NOSUPERUSER` pasó mientras conservaba la membresía creada por el bootstrap y falló después de revocarla, con el error de falta de `ADMIN OPTION`. La migración también comprueba que no quede ninguna membresía. La prueba real equivalente en Cloud SQL sigue sin ejecutarse.

Una base de datos exportada con `pg_dump` no incluye los objetos globales del clúster, como roles y tablespaces; PostgreSQL remite a `pg_dumpall` para ellos. El formato custom admite selección explícita durante `pg_restore`, y `--no-owner`/`--no-acl` separan la restauración del ownership y los grants del origen. Los comentarios de roles, los roles y sus membresías requieren un manifiesto y bootstrap separados. [Documentación oficial de `pg_dump`](https://www.postgresql.org/docs/18/app-pgdump.html), [`pg_restore`](https://www.postgresql.org/docs/18/app-pgrestore.html) y [`COMMENT`](https://www.postgresql.org/docs/18/sql-comment.html).

## Propuesta

Solo si se acepta formalmente el cambio de criterio, crear un artefacto de baseline a partir de una base vacía que el runner haya llevado hasta V61. El artefacto contendría el esquema y filas sintéticas aprobadas, incluyendo las filas originales de `schema_migrations`. No se reejecutaría V60 en destino ni se fabricarían filas de historial: los checksums, nombres, runner checksum y timestamps se conservarían exactamente como los produjo el origen. Un manifiesto separado indicaría explícitamente que el historial fue transferido desde una fuente identificada; no afirmaría que V1–V61 se ejecutaron en el destino.

El artefacto debería tener SHA-256, commit fuente, digest de la imagen/migrador, hashes de cada archivo V1–V61, conteo y digest del historial, versiones de PostgreSQL/extensiones, versión del formato y listado TOC aprobado. El origen debe pasar `VerifyAllApplied`, validación de checksums, extensiones y objetos antes de crear el baseline. La salida debe ser inmutable, firmada/publicada por un proceso revisado y reproducible. Para una instalación nueva, el flujo aprobado sería: aprovisionar roles y base vacía; preinstalar extensiones soportadas; restaurar el archive con una lista TOC revisada y un actor de restore autorizado; aplicar aparte ownership, ACL y comentarios de roles; verificar catálogo, historial, extensiones, constraints, índices, funciones, triggers y grants; luego ejecutar únicamente migraciones nuevas V62+ mediante el runner.

Las instalaciones existentes y sus cambios incrementales continuarían usando el runner con verificación de historial/checksums. Las instalaciones nuevas dejarían de probar el camino de replay V1–V61 en Cloud SQL y usarían un baseline restaurado. Esa diferencia debe quedar declarada en el criterio de aceptación y en los procedimientos de recuperación. No debe considerarse equivalente hasta validar que la restauración mantiene la misma semántica observable.

## Requisitos previos para aceptar

1. Decisión de producto/arquitectura que cambie explícitamente el criterio de instalación limpia de «replay V1–V61 en Cloud SQL» a «restore de baseline verificado». Este ADR no otorga esa aceptación.
2. Prueba Cloud SQL real en la instancia scratch: permisos del actor de restore, extensiones y versiones, objetos dependientes de extensiones, ownership, ACL, secuencias, funciones `SECURITY DEFINER`, checksums/historial y queries PostGIS. Aún no se ejecutó SQL en Cloud SQL.
3. Prueba de instalación desde baseline y upgrade V62+ en una base descartable, además de una prueba fresca V1–V61 local para seguir cubriendo la secuencia original. Fallos deben detener el corte sin avanzar punteros ni reutilizar un destino parcial.
4. Definición de quién construye, aprueba, firma, retiene y revoca cada baseline; política de retención y acceso a datos. El baseline no incluirá datos reales salvo autorización separada y manifiesto explícito.
5. Procedimiento probado de rollback: no sobrescribir una base existente; restaurar en base nueva y aislada, verificar antes de cambiar el destino y descartar solo el destino fallido bajo autorización.

## Propiedad, ACL y roles

La restauración no resuelve automáticamente los objetos globales. Provisionar antes los roles destino `NOLOGIN` con atributos explícitos y comentarios versionados en un plan de bootstrap independiente. Provisionar owner/grants/ACL según allowlist revisada; no copiar `pg_auth_members` ni contraseñas del origen. No usar superusuario PostgreSQL real ni el rol de aplicación. Un rol temporal de restauración, si Cloud SQL lo permite, debe tener alcance y duración mínimos, con revocación verificada después.

El comentario de `espacigo_gadm_bootstrap` que crea V60 no viaja en un dump de base individual. El bootstrap de destino debe conservar el texto aprobado y probar cómo aplicar el comentario bajo la política administrada real, sin dejar membresías. No debe afirmarse que V60 quedó ejecutada en destino: solo se transfirió su estado de base y se aprovisionó por separado el metadato global faltante.

## Evidencia local y límites

El ensayo reproducible está en [`demo-gcp03-v60-migrated-baseline.sh`](../../scripts/demo-gcp03-v60-migrated-baseline.sh); sus resultados están resumidos en [`evidencia_demo_baseline_migrado_v60_20261016.md`](evidencia_demo_baseline_migrado_v60_20261016.md). En PostgreSQL 18/PostGIS 3.6.4 local, el runner aplicó las 61 migraciones a un clúster desechable. Un `pg_dump -Fc --no-owner --no-acl` se restauró en otro clúster desechable con roles/extensiones preaprovisionados. Se revisó el TOC para excluir únicamente la creación/comentario de las extensiones preinstaladas y los datos de la tabla `spatial_ref_sys` que administra PostGIS. El restore usó conexión administrativa local con `SET ROLE` a `espacigo_owner`. Las 61 filas de historial (incluidos nombre, checksums de migración y runner, y `applied_at`) y las versiones de `postgis`, `btree_gist` y `unaccent` coincidieron; V60 no se reejecutó en el destino.

Esto demuestra una ruta local de transferencia de esquema, no compatibilidad con los permisos/extensiones de Cloud SQL. El origen local se migró como `postgres` superusuario, el restore se conectó como administrador local, la versión PostGIS local fue 3.6.4, no se usaron datos de aplicación y el comentario de rol se configuró por separado como administrador local. El ensayo no prueba Cloud SQL, la ruta real del rol de restauración, el bootstrap de ownership/ACL administrado, runtime/Go, seguridad del artefacto ni aceptación de un cambio en los criterios.

## Alternativas descartadas por ahora

- Modificar V60, relajar su guard, quitar el comentario o alterar checksums: rompe el historial aplicado y no es parte de esta propuesta.
- Registrar V1–V61 como aplicadas sin construir/verificar su esquema: inventa historial y queda prohibido.
- Copiar `pg_auth_members`, owners, ACL o contraseñas del origen sin allowlist: amplía privilegios y no es portable a Cloud SQL.
- Declarar que un restore local prueba Cloud SQL: no se sostiene con esta evidencia.

## Referencias técnicas

- [PostgreSQL 18 `pg_dump`](https://www.postgresql.org/docs/18/app-pgdump.html): exporta una base individual; los objetos globales del clúster se manejan aparte; explica formatos custom y `--no-owner`/`--no-acl`.
- [PostgreSQL 18 `pg_restore`](https://www.postgresql.org/docs/18/app-pgrestore.html): restauración selectiva de un archive y opciones de ownership/ACL.
- [PostgreSQL 18 `COMMENT`](https://www.postgresql.org/docs/18/sql-comment.html): permisos requeridos para comentar objetos.
- [PostgreSQL 18 `CREATE ROLE`](https://www.postgresql.org/docs/18/sql-createrole.html): permisos y capacidades de `CREATEROLE` y membresías.
- [Cloud SQL PostgreSQL: usuarios](https://docs.cloud.google.com/sql/docs/postgres/users): roles administrados por el servicio; no implica compatibilidad con un flujo de restore propuesto aquí.
