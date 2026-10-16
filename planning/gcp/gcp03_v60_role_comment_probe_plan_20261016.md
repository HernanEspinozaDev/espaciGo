# GCP-03: plan de probe de COMMENT de rol en Cloud SQL

Estado: propuesta técnica para revisión. No es una autorización de ejecución ni evidencia de compatibilidad administrada.

## Pregunta y alcance

El probe determina si el login PostgreSQL temporal `espacigo_scratch_migrator`, con `CREATEROLE` pero sin superusuario, puede ejecutar el `COMMENT ON ROLE` requerido por el bootstrap de GADM. Se limita al proyecto existente `espacigo-testprod`, instancia `espacigo-gcp03-pilot01-pg` y base `espacigo_scratch`, todos desechables. El destino debe seguir vacío de `schema_migrations` y de tablas de aplicación.

El programa solo admite dos fases fijas. No carga archivos SQL, no ejecuta V60 ni otra migración, no toca datos de aplicación, no importa/restaura dumps y no cambia el criterio de base vacía. La preparación crea únicamente `espacigo_gadm_bootstrap` como grupo `NOLOGIN` con todos los atributos privilegiados desactivados, y ejecuta el literal fijo del COMMENT. Una denegación SQLSTATE `42501` se registra como resultado experimental válido; cualquier otro error detiene el Job.

## Secuencia controlada

1. Revalidar por lectura el proyecto, región, instancia, base, versión PostgreSQL, estado de la instancia, ausencia de historial/tablas de aplicación, ausencia de Jobs de este run y estado de la cuenta SQL/secret. Confirmar que el historial de migraciones no cambió.
2. Revisar este PR y su HEAD. Construir desde un commit limpio y guardar el digest OCI; todavía no subir la imagen. Después de autorización operacional separada, subir al Artifact Registry de scratch y volver a leer el digest.
3. Crear el login SQL temporal y una sola versión de secreto mediante `gcp03scratch-credential`; guardar credenciales y evidencia privada solo en `.local/` con permisos 0600. Registrar el número devuelto y detenerse si no es exactamente el número configurado para el Job. No usar `latest`.
4. Validar que IAM Conditions limitan las operaciones Cloud SQL a la instancia exacta. La API Cloud SQL puede evaluar permisos a nivel de proyecto; si la condición por recurso no se demuestra efectiva para los métodos `users.*`, no conceder el permiso ni ejecutar el probe. Toda concesión humana lleva expiración explícita; los dos Jobs tienen invocación individual.
5. Generar y revisar un plan Terraform exacto para los dos Jobs y los permisos temporales. Cada Job usa una tarea, paralelismo 1, reintentos 0, timeout 600 s, 1 CPU y 512 MiB; imagen por digest y versión numérica concreta del secreto. No aplicar sin revisión operacional separada.
6. Ejecutar una vez `role_comment_prepare`. Debe comprobar PostgreSQL 16+, identidad y base exactas, `rolsuper=false`, `rolcreaterole=true`, rol ausente y esquema vacío; luego crea y verifica el grupo fijo y prueba COMMENT.
7. Invocar el reconciliador existente de membresías Cloud SQL para el login exacto. Este paso reemplaza sus roles por `cloudsqlsuperuser` y vuelve a leerlos. No alterar el rol NOLOGIN de prueba por SQL.
8. Ejecutar una vez `role_comment_after_reconcile` en una conexión nueva. Exige atributos exactos y cero membresías entrantes o salientes antes de intentar el mismo COMMENT. Si queda alguna membresía, no ejecuta SQL y falla de forma explícita.
9. Retirar el login con `--retire-role-comment-probe-login`: confirma que la API lo eliminó antes de destruir únicamente la versión numérica de secreto indicada. Si la eliminación del usuario es incierta, conserva el secreto para permitir reintento seguro.
10. Eliminar Jobs, invocaciones y permisos temporales con un plan Terraform revisado; verificar que desaparecieron. Verificar por lectura el rol fijo, su comentario/estado, ausencia de membresías, ausencia de historial/tablas y que no se ejecutó migración. El rol de prueba permanece en el scratch desechable; su retiro ocurre con el teardown revisado de esa instancia.

## Permisos y limpieza

Los Jobs usan el service account migrator existente y solo el secreto migrator de scratch. El operador recibe temporalmente métodos Cloud SQL limitados a lectura/creación/actualización/eliminación de usuarios de la instancia y operaciones de versiones limitadas al secreto exacto. El plan propuesto incluye dos roles IAM personalizados y condiciones de expiración; la concesión Cloud SQL queda bloqueada hasta validar el alcance real de IAM Conditions para cada método. El operador recibe además `run.invoker` solo en cada Job y `iam.serviceAccountUser` en el service account migrator.

El cleanup del login es idempotente: si el login ya no existe, puede destruirse la versión exacta remanente; si la API no confirma su ausencia, no destruye el secreto. No se borra ninguna otra versión, secreto, usuario, instancia, volumen ni state. La instancia completa se elimina únicamente mediante el teardown run-specific de este root, en dos fases y con confirmación del run ID.

## Evidencia y criterio

Registrar commit de código, digest de imagen, versión numérica del secreto, plan aplicado, ID/salida saneada de cada Job, versión PostgreSQL, atributos y conteos de membresías antes/después, SQLSTATE y resultado del COMMENT, y verificación de limpieza IAM/Jobs/login/secreto. Mantener secretos y evidencia sensible bajo `.local/testing/` excluido y permisos 0600. Nunca incluir credenciales en logs, PR, capturas o archivos versionados.

El resultado solo informa el comportamiento del scratch administrado en esa versión/configuración. No demuestra que el criterio actual de base vacía sea incorrecto, no acepta producto y no autoriza cambios al historial V1–V60. Una conclusión sobre el bootstrap o sobre el plan de 1.200 tratos requiere revisión explícita de la evidencia y un cambio de plan separado.

## Referencias primarias

- [PostgreSQL 18 COMMENT](https://www.postgresql.org/docs/18/sql-comment.html)
- [PostgreSQL 18 atributos de roles](https://www.postgresql.org/docs/18/role-attributes.html)
- [Cloud SQL permisos IAM para PostgreSQL](https://docs.cloud.google.com/sql/docs/postgres/iam-permissions?hl=en)
- [Cloud SQL Admin API: usuarios](https://docs.cloud.google.com/sql/docs/postgres/admin-api/rest/v1/users)
- [Cloud SQL: reemplazar roles de usuario](https://docs.cloud.google.com/sql/docs/postgres/create-manage-users?hl=en)
- [Cloud Run Jobs: secretos](https://docs.cloud.google.com/run/docs/configuring/jobs/secrets?hl=en)
