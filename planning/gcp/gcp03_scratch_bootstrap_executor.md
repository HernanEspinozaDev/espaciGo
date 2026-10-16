# GCP-03: ejecutor local de bootstrap para scratch

Estado: **implementación local para revisión**. Este corte agrega un binario Go con cuatro fases nombradas y los controles Terraform que las crean solo cuando se habilita la puerta existente. Se construyó el OCI local dos veces desde el commit `717d6f8e57404050427b1bec8bcb5ee6e294deb1`; ambas dieron manifiesto `sha256:4bd84c4b394879ed6cbe3873ba57f46bf67ae03c33c23f9660fba9281935e9d5` y archivo OCI SHA-256 `b53f273baa411be02a4a995bab117e9d40060d1f60746aeeeca82aec27cfb437`. Los archivos privados están bajo `.local/planning/gcp/gcp-03-scratch-20261016/executor-review-717d6f8-{a,b}/` con permisos 0600. No se ejecutó SQL, no se crearon usuarios ni versiones de secretos, no se ejecutaron Jobs ni se subió la imagen. La instancia `espacigo-gcp03-pilot01-pg` y su base `espacigo_scratch` existentes se conservan. Este párrafo se añadió en un commit documental posterior; el código binario empaquetado corresponde al commit citado.

## Alcance del ejecutor

`cmd/gcp03scratch` acepta solamente `bootstrap`, `migrate`, `finalize` y `retire`. El binario fija proyecto, región, nombre de instancia, database, login, URL sin password, ruta del secreto y modo de IP privada. Rechaza overrides de libpq/pgx, SQL de entorno, URLs distintas y contraseñas que no coinciden con el formato aleatorio creado por el operador. Usa el adaptador `internal/database/cloudsql` compartido y el inventario de migraciones embebido en la imagen; no expone flags de SQL arbitrario.

| Fase | Acción fija | Condición de salida |
| --- | --- | --- |
| `bootstrap` | Crea `btree_gist`, `postgis` y `unaccent`; crea o valida `espacigo_runtime NOLOGIN` restringido; confirma base, sesión y extensiones. | Las tres extensiones están disponibles y el rol runtime no tiene privilegios ni membresías inesperados. |
| `migrate` | Comprueba el bootstrap y llama al runner compartido para V1–V61 byte a byte. | El runner verifica secuencia y checksum por versión; fallas abortan la fase. |
| `finalize` | Exige historial completo V1–V61; aplica `GrantRuntimePermissions`; comprueba atributos V60, roles y permisos positivos/negativos. | Contrato de permisos runtime verificado. |
| `retire` | Vuelve a comprobar historial y grants; el SQL versionado deshabilita login, `CREATEROLE`, `CREATEDB` e `INHERIT` del login temporal. | La sesión confirma que el login ya no puede iniciar sesión ni tiene privilegios activos. El operador revoca después las membresías de Cloud SQL por Admin API. |

No se borra el rol temporal: puede ser propietario de objetos creados durante las migraciones. Tras la fase `retire`, `gcp03scratch-credential --apply --revoke-superuser --confirm-target=REVOKE:<target> --secret-version=<versión exacta>` usa Cloud SQL Admin `users.update` con membresías vacías y `revokeExistingRoles=true`, espera la operación, consulta `users.get` para confirmar cero `databaseRoles` y destruye únicamente la versión exacta indicada de Secret Manager. Una falla en la revocación o borrado requiere conciliación antes de repetir; no elimina el login ni altera ownership.

## Creación inicial de la credencial temporal

Terraform continúa administrando solo metadata del secreto. Un operador autenticado ejecuta localmente `gcp03scratch-credential --apply --confirm-target=CREATE:<target>`. El CLI genera 32 bytes aleatorios, los representa como Base64 URL sin padding, agrega una versión al recurso fijo `projects/espacigo-testprod/secrets/espacigo-gcp03-pilot01-migrator-password`, crea el login integrado exacto por Cloud SQL Admin API y espera la operación. No imprime ni recibe una contraseña en argumentos; los resultados inciertos conservan la versión para conciliación.

Cloud SQL asigna inicialmente `cloudsqlsuperuser` a un usuario PostgreSQL integrado cuando no se especifican roles personalizados. La retirada documentada por Google usa `users.update` con `databaseRoles: []` y `revokeExistingRoles: true`; aquí solo ocurre después de la fase SQL que ya dejó el login `NOLOGIN`. Referencias: [crear y administrar usuarios PostgreSQL](https://docs.cloud.google.com/sql/docs/postgres/create-manage-users) y [método REST users.update](https://cloud.google.com/sql/docs/postgres/admin-api/rest/v1beta4/users/update).

## Jobs e IAM

El root aislado define cuatro Jobs Cloud Run independientes (`bootstrap`, `migrate`, `finalize`, `retire`), cada uno con una tarea, paralelismo 1, cero reintentos, cuatro horas, 2 vCPU/4 GiB y digest inmutable. El job permanece apagado por defecto y Terraform exige el scratch exacto, digest de la imagen y confirmación de que existe una versión del secreto.

- Identidad runtime: `roles/cloudsql.client` en el proyecto y `roles/secretmanager.secretAccessor` únicamente en el secreto temporal. No recibe Cloud SQL Admin, Secret Manager de aplicación, bucket de archivos ni `Editor`/`Owner`.
- Service agent de Cloud Run: `roles/artifactregistry.reader` limitado al repositorio scratch.
- Operador de Jobs: `roles/run.invoker` en cada Job y `roles/iam.serviceAccountUser` solo sobre la cuenta runtime scratch.
- CLI de credenciales: rol custom temporal, fuera de Terraform, con `cloudsql.users.create`, `cloudsql.users.update`, `cloudsql.users.get` y `cloudsql.instances.get` (para esperar operaciones) en el proyecto. Cloud SQL aplica estos permisos al proyecto y no permite acotarlos a una instancia; el permiso de usuario, por tanto, puede afectar otras instancias de ese proyecto durante la ventana temporal. Secret Manager: `roles/secretmanager.secretVersionManager` en el secreto fijo incluye `secretmanager.versions.add`/`destroy` y también `disable`/`enable`/`get`/`list`, sin permiso para acceder al payload. No se añade este set al service account runtime. No otorgar roles predefinidos amplios como Cloud SQL Admin o Secret Manager Admin. La limitación de alcance de Cloud SQL y métodos está documentada por Google en [permisos IAM para Cloud SQL](https://docs.cloud.google.com/sql/docs/postgres/iam-permissions); el alcance mínimo del rol de versiones está en [acceso de Secret Manager](https://docs.cloud.google.com/secret-manager/docs/access-control).

La imagen se construye desde un commit limpio con Go 1.27.1 y builder OCI fijado por digest. El manifiesto privado registra el digest OCI, hashes individuales/agregado de V1–V61, hashes del SQL fijo y nombres de fases. Las dos construcciones locales registraron inventario V1–V61 SHA-256 `505f24b385ec8a98a6d27b0367178476a940bce4e45c8a5305fc0a64de72c4a9` e inventario SQL de bootstrap SHA-256 `6909be8d7627b5bd2e22b66d2289cdbb8671a7db3f122de431f8d415e12a276c`. El script no publica la imagen y no se subió ningún artefacto a Artifact Registry.

## Precio regional (consulta 2026-10-16)

La página oficial incluye Santiago (`southamerica-west1`) en la tabla regional de Cloud Run Jobs; los Jobs se facturan al precio de instancia durante toda la vida del contenedor, con mínimo de un minuto. La tarifa de lista publicada para Jobs es USD 0.000018 por vCPU-segundo y USD 0.000002 por GiB-segundo en la tabla base consultada. Con 2 vCPU y 4 GiB eso da USD 0.000044/segundo (USD 0.00264/minuto) antes de descuentos/free tier aplicable. Cuatro fases suman al menos cuatro minutos facturables por invocación completa, con un mínimo aritmético de USD 0.01056 antes de descuentos/free tier; el tiempo real por fase aún se desconoce. La tarifa no es cotización ni costo total; transferencia, logging y servicios auxiliares pueden sumar cargos. Fuente: [precios de Cloud Run](https://cloud.google.com/run/pricing), que enumera Santiago y describe el mínimo facturable de Jobs.

La referencia de Cloud SQL N4 + Hyperdisk Balanced de la documentación previa sigue siendo un modelo aproximado (~USD 4.86/24 h, ~USD 145.59/720 h antes de backups, WAL/PITR, tráfico y crecimiento), no se recalculó con el pricing calculator en esta revisión. Fuente oficial para validar la configuración y tarifas: [precios de Cloud SQL](https://cloud.google.com/sql/pricing). No se evaluó ni cambió el modo Free Trial/Paid.

## IAM temporal de operador de credenciales

La creación/revocación de usuario y secreto no se hace desde Terraform ni desde el Job. La identidad local necesita las acciones de Admin API de usuarios y operaciones, además de versiones del secreto. Cloud SQL documenta que sus permisos de Admin API se aplican al proyecto, sin condición por instancia; este es un riesgo temporal explícito. Antes de cualquier ejecución debe calcularse un binding temporal de duración corta usando un rol custom revisado; evitar `roles/cloudsql.admin`, `roles/secretmanager.admin` o permisos de proyecto amplios. Retirar el binding después de crear/conciliar el usuario y luego de revocar la membresía y destruir la versión. El service account runtime permanece sin esos permisos.

## Verificaciones y límites de esta revisión

Las pruebas locales cubren guardas exactas, orden/selección de fases, secretos, resultados inciertos, historia/checksums, roles/grants y los cuatro recursos Terraform nombrados. El plan de Terraform no se generó ni aplicó. Aún requiere revisión del PR antes de crear una nueva imagen, usuario/versión, Jobs o ejecutar cualquiera de las fases. La ejecución posterior debe validar por API el scratch exacto y sus IAM actuales, además del digest leído del registry, y mantener separado cualquier restore/import de datos.
