# Evidencia — alcance de transición de autoridad nativa V95/V96

- **Fecha:** 2026-11-14.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`, worktree compartido y dirty.
- **Destino observado:** PostgreSQL nativo `127.0.0.1:5432/espacigo_dms_stage_20261020`.
- **Alcance ejecutado:** lectura de catálogos bajo `sudo -u postgres` en transacción `READ ONLY`, más pruebas sintéticas en contenedores desechables. No se consultaron filas de negocio ni secretos.

## Autorización humana acotada

El usuario autorizó el 2026-11-14 preparar y, después de script versionado, revisión independiente, backup fresco validado y guards, aplicar en el destino nativo: transferir a `espacigo_schema_owner` el ownership de `public.document_verification_case` y `public.document_verification_event`; conceder `CREATE` en `public` por separado para V95 y V96, revocarlo y leerlo de vuelta tras cada versión, y validar un backup entre ambas fases.

Después de informar los efectos reales de PostgreSQL 18.7, el usuario autorizó también únicamente los cambios automáticos de ownership de los ocho índices y la secuencia identity ligados a esas tablas, y la reescritura del grantor existente de ACL runtime de `postgres` a `espacigo_schema_owner`, conservando grantee, privilegios y grant options efectivos. No se amplía el alcance a otras ACL, owners, roles, ledger, Cloud SQL ni recursos GCP. Esta autorización no sustituye los gates condicionantes y **todavía no se ha aplicado**.

## Readback nativo READ ONLY

La invocación `sudo id && sudo -u postgres psql ...` confirmó acceso administrativo local; la consulta read-only devolvió:

- Base `espacigo_dms_stage_20261020`, OID `16388`, `server_version_num=180007` (PostgreSQL 18.7), `session_user=current_user=postgres` para esa conexión administrativa.
- Ledger continuo V1–V94, 94 filas; V94 checksum `a4ee15a45ab352527e93c00e82763056b57b3750064483b8b90aa6891c0672a9`.
- `public` pertenece a `pg_database_owner`; `espacigo_schema_owner` no tiene `CREATE` efectivo.
- `document_verification_case` y `document_verification_event` pertenecen a `postgres`. Los ocho índices ligados y `document_verification_event_id_seq` también pertenecen a `postgres`.
- `document_verification_event_id_seq` es identity-linked (`pg_depend.deptype='i'`) a `document_verification_event.id`; conserva el grant `USAGE` de `espacigo_runtime` otorgado por `postgres`.
- Las ACL de tabla/columna observadas para runtime en las dos tablas también tienen como grantor `postgres`. No se cambió ningún grant.

## Ensayo sintético de efectos PostgreSQL 18.7

**Corrección de versión:** el primer ensayo había usado `postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677`; al verificarlo explícitamente con `postgres --version` resultó PostgreSQL 18.6, no 18.7. No se conserva como evidencia de versión 18.7. Se repitió la misma prueba de ownership/ACL en la imagen oficial fijada `postgres@sha256:c7fedce47eff6a78cb35dad6b627f0d684107f75ab15c0870e40c093766ba452`; la imagen reportó PostgreSQL 18.7 (Debian 18.7-1). El ensayo usó red `none`, `PGDATA` temporal, sin puertos ni volúmenes, y roles/tabla identity sintéticos.

- `ALTER TABLE ... OWNER TO schema_owner_probe` cambió también el owner del índice y de la secuencia identity ligada.
- Las ACL runtime mantuvieron grantee y privilegio, pero su grantor pasó de `postgres` a `schema_owner_probe`, tanto en tabla/columna como en secuencia. Reaplicar `GRANT ... GRANTED BY postgres` no restauró el grantor anterior.
- El `GRANT CREATE` al owner de prueba quedó con grantor `pg_database_owner`; `REVOKE CREATE` ejecutado como `postgres` lo retiró, y el readback devolvió `has_schema_privilege=false` con el ACL previo restablecido.
- Intentar devolver la secuencia identity a su owner anterior falló porque sigue ligada a la tabla. La prueba completa terminó con exit 0 y verificó que su contenedor propio fue retirado.
- Esta repetición confirma los efectos en PostgreSQL 18.7 para el fixture sintético; no ejecuta las migraciones ni muta/probar la persistencia del destino nativo. El digest PostGIS 18.6 queda descartado para la afirmación de versión 18.7.

## Estado y siguiente gate

V95/V96 siguen pendientes y bloqueadas por los guards hasta aplicar la transición autorizada. No se han creado scripts V95/V96 de preparación/finalización, no se ejecutó backup ni DDL nativo, y no se alteró owner, ACL o ledger. Cloud SQL sigue sin lectura vigente; su ruta canónica no se ejecutó.

El siguiente corte debe implementar y probar en PostgreSQL desechable los scripts versionados con pre/postcontratos exactos, incluyendo comparación bidireccional de ACL para demostrar que solo cambian owner/grantor permitidos y que los privilegios efectivos se conservan. Después requiere revisión independiente, backup privado fresco y validado, relectura exacta del plan nativo y cumplimiento de todos los guards antes de cualquier preparación/aplicación.

El intento de actualizar `AGENTS.md` con esta decisión fue bloqueado por el gate protegido de escritura al expirar su solicitud de aprobación; no se reintentó ni se intentó por otra vía. La autorización y sus límites quedan documentados aquí y en el plan KYC.