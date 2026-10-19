# GCP-04-W8 — primera sonda privada de catálogos

**Estado:** implementación local y manifest de liberación propuesto; rol candidato y compatibilidad con Cloud SQL Admin API sin resolver. No hay autorización ni ejecución de mutaciones Cloud.

## Objetivo y separación de W7

W8 agrega una sonda inicial separada de `cmd/gcp04preflight` y de su login W7. `cmd/gcp04w8catalogprobe` fija proyecto, instancia, IP, base, login candidato, archivo SQL, digest, secreto numérico y timeout. Comprueba la IP observada del socket del Cloud SQL Connector antes de la transacción; comienza una transacción PostgreSQL `READ ONLY`; verifica base, login, PostgreSQL 18 y `transaction_read_only`; y emite únicamente JSON de una consulta allowlisted. No acepta un SQL arbitrario. La imagen Docker local copia solo esta consulta y no incorpora un cliente de mutación de Cloud SQL Admin API.

La consulta enumera roles seleccionados (`espacigo_db_observer`, `pg_read_all_settings`, y el nuevo login candidato), el cierre recursivo de membresías heredadas/accesibles desde el login, las aristas relacionadas y ACL de la base actual, schema `public` y ledger `public.schema_migrations` cuando existe. No lee filas de negocio, vistas de configuración o estadísticas, catálogos de autenticación sensibles, comentarios, contraseñas o payloads de secretos. El hash fijado en el binario/manifiesto es `3fc825934660bbfc965e8d97b2ef7f4648baf2cded6c8871ae6953bd18c70d16`.

## Decisión pendiente sobre identidad

Cloud SQL documenta que una cuenta builtin creada sin roles personalizados recibe `cloudsqlsuperuser`; por ello la propuesta nunca omite `databaseRoles`. El Admin API modela `databaseRoles[]` como strings y la guía indica que un rol personalizado debe existir antes de asignarlo. La documentación no confirma de forma explícita que `users.insert` acepte `pg_read_all_settings` (rol predefinido de PostgreSQL) para la creación de este usuario builtin. El request de dry-run muestra el valor explícito `databaseRoles=["pg_read_all_settings"]`, pero no se envía a Cloud.

PostgreSQL documenta que `pg_read_all_settings` permite leer todas las variables de configuración, incluidas las normalmente visibles solo para superusuarios. Esa amplitud deja un riesgo residual aun cuando la consulta W8 no lea vistas de configuración. Por tanto el rol no está aprobado: antes de una operación de creación, revisión debe decidir si la amplitud es aceptable o pedir un rol más estrecho y confirmar compatibilidad del API. No se usará `pg_monitor`, `pg_read_all_stats`, `pg_read_all_data`, `cloudsqlsuperuser` ni una lista vacía.

## Helper y manifest

`internal/gcp04w8release` solo contiene un lector Admin API (interfaz sin método mutador) y un builder `dry_run_only`. Relee metadata exacta de instancia, inventario de usuarios y del secreto regional, además del inventario de versiones. Aborta por drift, inventario incompleto, colisión del login o versiones, ausencia del secreto regional o errores; no genera password, no crea versión, no cambia usuario y no llama a un mutador. El request propuesto contiene tipo `BUILT_IN` y un array `databaseRoles` exacto y no vacío. Los mocks cubren target incorrecto, colisiones, rechazo de lista de roles vacía o alternativa, y errores sanitizados.

El manifest JSON registra target/connection/IP, request propuesto, digest SQL, ruta regional exacta, digest de imagen aún pendiente, identidad del Job/SA, IAM acotado, secuencia de creación propuesta, abortos/reconciliación y teardown solo de recursos exactos W8. La contraseña se describe como memoria efímera y el secreto como versión regional numérica; el payload no aparece en Terraform, argumentos, logs o salida. No hay recurso Terraform para una versión secreta, porque el proveedor guarda `secret_data` en state sin procesar. La publicación del digest, generación/adición de secreto, `CREATE_USER`, IAM, Job y ejecución requieren un corte y autorización posteriores.

Secuencia de propuesta: (1) releer metadata/user list/secreto; (2) abortar ante colisión sin reset ni update; (3) revisión de rol y soporte Admin API; (4) generar password en proceso; (5) agregar una versión regional y conservar solo su número; (6) crear exclusivamente el usuario nuevo con roles explícitos; (7) preparar IAM y Job aislado con digest aprobado; (8) ejecutar la consulta una vez bajo autorización aparte. Un resultado incierto se reconcilia por recursos exactos y no se repite a ciegas.

## Entorno, costos y límites

Metadata de solo lectura observada: proyecto `espacigo-testprod`, instancia `espacigo-testprod-pg`, región `southamerica-west1`, PostgreSQL 18, estado `RUNNABLE`, conexión `espacigo-testprod:southamerica-west1:espacigo-testprod-pg`, IP `10.189.0.3`. La lista Cloud SQL de usuarios no devolvió el login W8. No se abrió sesión SQL remota ni se leyó catálogo, contraseña o payload secreto.

Cloud Run Jobs en Santiago publica `$0.000018/vCPU-s` y `$0.000002/GiB-s`; un Job de 1 vCPU/0.5 GiB con un minuto mínimo calcula ~$0.00114 bruto, con máximo de 120 s ~$0.00228, antes del free tier agregado. Una versión regional de Secret Manager activa durante 24 h cuesta ~$0.001973 si se excede la cuota gratuita de cuenta; uso actual de cuota es desconocido. Esto no incluye Artifact Registry ni egreso de red.

Referencias oficiales: [usuarios Cloud SQL](https://docs.cloud.google.com/sql/docs/postgres/create-manage-users), [recurso User Admin API](https://docs.cloud.google.com/sql/docs/postgres/admin-api/rest/v1/users), [users.insert](https://docs.cloud.google.com/sql/docs/postgres/admin-api/rest/v1/users/insert), [roles predefinidos PostgreSQL 18](https://www.postgresql.org/docs/18/predefined-roles.html), [pg_roles](https://www.postgresql.org/docs/18/view-pg-roles.html), [pg_auth_members](https://www.postgresql.org/docs/18/catalog-pg-auth-members.html), [Secret Manager regional AddSecretVersion](https://docs.cloud.google.com/secret-manager/regional-secrets/add-secret-version-rs), [precios Secret Manager](https://cloud.google.com/secret-manager/pricing), [precios Cloud Run](https://cloud.google.com/run/pricing), [recurso Terraform de versión regional](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/secret_manager_regional_secret_version).

## Verificación local pendiente

Tests locales ejecutados: `go test -race ./internal/gcp04w8release ./cmd/gcp04w8catalogprobe` pasó; `bash -n scripts/test-gcp04-w8-catalog-probe-postgres.sh` pasó; el harness de PostgreSQL 18 aislado sin red ejecutó la consulta como el login de sonda con membresía explícita en la fixture y pasó. `docker build` de la imagen final pasó y el contenedor sin credenciales respondió con error JSON genérico sin intentar Cloud; la imagen local se retiró. `git diff --check` pasó. No se aplicó Terraform, no se publicó imagen y no hubo operación Cloud.
