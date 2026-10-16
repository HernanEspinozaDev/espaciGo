# GCP-03: scratch privado para migraciones y restauración

Estado: la infraestructura scratch fue creada conforme al plan exacto revisado. Ver [evidencia de ejecución](evidencia_aplicacion_scratch_gcp03_20261016.md) para los recursos y límites actuales. Este documento conserva la propuesta operativa para los siguientes pasos; no autoriza SQL, conexión, migración, dump, restore, importación, creación de login SQL, generación de valor/versión de secreto ni ejecución del Job.

## Ejecutor inmutable

Usar `cmd/dbmigrate` en un Cloud Run Job futuro de una tarea, paralelismo 1, reintentos 0, timeout máximo inicial 4 h, 2 vCPU y 4 GiB, región `southamerica-west1`. El Job no existe aún. La imagen OCI ya subida y verificada es `southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/dbmigrate@sha256:23c753939f12d035d83684094506eb0effcb20d5a1dab5544c12280a0fc461a0`; sus inputs corresponden al commit `8fe6ef188ee7115b11b4ccc5479c593591bbc36e`. Mantener referencia por digest; nunca `:latest` ni un tag mutable. Antes del Job, confirmar de nuevo el manifiesto, SHA-256 de V1–V61, lista de extensiones y conexión destino. El Job leería credenciales del archivo montado por Secret Manager; la URL no va en flags/logs.

La configuración prevista del binario es `DATABASE_CONNECTION_MODE=cloudsql`, `DATABASE_URL=postgres://<usuario>@localhost:5432/<base>?sslmode=disable` sin contraseña inline ni parámetros extra, `DATABASE_PASSWORD_FILE=/secrets/migrator/password`, `CLOUD_SQL_INSTANCE_CONNECTION_NAME=espacigo-testprod:southamerica-west1:espacigo-gcp03-pilot01-pg`, `CLOUD_SQL_PRIVATE_IP=true` y `DBMIGRATE_TIMEOUT=4h`. El modo Cloud SQL rechaza overrides de conexión de entorno `PG*`; la contraseña se asigna al `pgx.ConnConfig` luego de validar el URL y no aparece en la URL. El host y puerto URL son solo sintaxis de pgx; Cloud SQL Go Connector realiza el dial a la instancia por IP privada y gestiona TLS. El modo local continúa con `DATABASE_CONNECTION_MODE` ausente (`direct`), `DATABASE_URL` normal y timeout por defecto de 10 minutos.

## Destino descartable

La instancia scratch `espacigo-gcp03-pilot01-pg` ya existe separada de `espacigo-testprod-pg`, PostgreSQL 18, en `southamerica-west1`, sin IPv4 pública y conectada a la VPC `espacigo-testprod`. Su base `espacigo_scratch` aún no tiene datos de aplicación. No modificar la instancia persistente ni crear bases scratch en ella. Una pista de restore requerirá autorización y diseño propios; no cargar el origen productivo en la instancia de ensayo sin aprobación específica.

Para migraciones limpias, el runner aplica V1–V61 byte a byte y valida historial/checksums, extensiones y objetos. Para restauración, primero se debe autorizar una lectura del respaldo origen y guardar tamaño/hash/TOC; después solo restaurar a una segunda base del scratch. El tamaño no se ha medido, por lo que el disco y GCS necesitan dimensionamiento posterior. La salida de cada pista incluye manifiesto, logs saneados, conteos/consultas, catálogo de roles y evidencia de borrado del scratch.

## Red privada y APIs

- Direct VPC egress en el job, VPC `espacigo-testprod`, subnet `espacigo-testprod-serverless` (`10.80.0.0/24`), egress `PRIVATE_RANGES_ONLY`. No usar Serverless VPC Access connector, Cloud NAT ni `ALL_TRAFFIC` por defecto. El tráfico a la IP privada Cloud SQL entra a la VPC; el resto conserva el camino de egreso de plataforma. No habilitar acceso entrante al job.
- La subnet existente declara `private_ip_google_access=true`; confirmar el valor efectivo, ruta default gateway, DNS y firewall antes del ensayo. Private Google Access es una ruta de API para clientes conectados a la subnet; no reemplaza el egress de plataforma del job. Probar los endpoints usados (Secret Manager, Artifact Registry si aplica y Cloud Logging) con una imagen sin credenciales de base. Si una política de organización obliga a enrutar todo por VPC, resolver acceso a APIs con Private Google Access y rutas/DNS compatibles; crear NAT solo si hay un requerimiento probado de salida general a Internet.
- Preflight de APIs: `run.googleapis.com`, `sqladmin.googleapis.com`, `compute.googleapis.com`, `secretmanager.googleapis.com`, `storage.googleapis.com` (solo si hay dump GCS), `artifactregistry.googleapis.com` (si el manifiesto está allí), `logging.googleapis.com` y `monitoring.googleapis.com` según telemetría. No habilitar APIs durante este PR.

## IAM e identidad SQL

La cuenta de ejecución scratch creada es `gcp03-scratch-pilot01@espacigo-testprod.iam.gserviceaccount.com` y tiene `roles/cloudsql.client`. El secreto creado es `espacigo-gcp03-pilot01-migrator-password`, todavía sin versiones. Confirmar política efectiva antes de cualquier uso y no ampliar a `Editor`, `Owner` ni roles de administración. Cualquier futura identidad de restore necesita `roles/storage.objectViewer` sobre un bucket temporal concreto; no conceder permisos sobre el bucket de fotos.

El operador de ejecución debe tener solo `roles/run.invoker` en el job nombrado; quien crea/actualiza Job y asigna su service account necesita un rol separado y temporal, fuera de la identidad de ejecución. Cloud Run Service Agent conserva su rol de servicio; validar la capacidad de Direct VPC egress antes de añadir `roles/compute.networkUser` al agente, limitado a subnet si la plataforma exige esa alternativa. El service account del job no es quien requiere ese permiso de administración de red.

Cloud SQL built-in PostgreSQL users reciben `cloudsqlsuperuser` automáticamente cuando se crean sin roles de base personalizados; las extensiones soportadas requieren ese rol. Por ello, el ensayo debe probar primero la vía menos privilegiada en scratch: precrear extensiones soportadas con un bootstrap temporal y comprobar si V1–V61 con `CREATE EXTENSION IF NOT EXISTS` pueden continuar bajo un migrador con `CREATEROLE` pero sin membresía elevada. Si falla, registrar la sentencia/permiso concreto y usar `cloudsqlsuperuser` únicamente para el login temporal del scratch, nunca para `espacigo_runtime` ni la base persistente.

En el scratch crear `espacigo_runtime` con `NOLOGIN` o login de ensayo separado, sin `SUPERUSER`, `CREATEDB`, `CREATEROLE`, `REPLICATION`, `BYPASSRLS` ni `INHERIT`; aplicar solo `GrantRuntimePermissions`. El migrador no se usa en runtime. Antes de retirar privilegios comprobar ownership de extensiones/objetos, grants, secuencias y V60. Revocar membresía `cloudsqlsuperuser`, `CREATEROLE` y `CREATEDB`; luego eliminar el login temporal o dejarlo `NOLOGIN` solo si el teardown del scratch está bloqueado por ownership y existe justificación registrada. El secreto de migración es de ejecución única, no se crea con Terraform y se destruye al cerrar el ensayo.

## Costos: límites de la estimación

- Para Cloud SQL Santiago, el modelo local GCP-02 estima `$4.86/24 h` y `$145.59/720 h` para N4 2 vCPU/4 GiB más Hyperdisk Balanced 20 GiB con IOPS/throughput predeterminados. Es una estimación previa, no precio facturado del scratch. Backup usado, WAL/PITR, crecimiento, transferencia y soporte no están incluidos; el tamaño final debe esperar a medir el dump.
- Cloud Run Jobs factura la vida completa de la instancia, con mínimo de un minuto. La página muestra Santiago en la lista de regiones y su tabla Jobs está basada en precio por defecto Iowa; sin seleccionar y guardar el SKU regional de Santiago no declaramos tarifa regional validada. `$0.6336` por 2 vCPU/4 GiB × 4 h es solo referencia aritmética con `$0.000018/vCPU-s` y `$0.000002/GiB-s`, no cotización Santiago. Cuota gratuita agregada, impuestos y descuentos dependen de la cuenta.
- Un día de SQL scratch más esa referencia de cuatro horas da `$5.49`, antes de GCS, operaciones, logs, tráfico y variables SQL. No se ha comprobado el crédito vigente ni se ha cambiado Paid mode. Antes de ejecutar registrar export o captura de Cloud Pricing Calculator con región `southamerica-west1`, SKU de Jobs seleccionado, Cloud SQL configuración exacta, duración real supuesta, bytes de backup y tasa egress; guardar entrada/salida sin datos sensibles.

## Ejecución futura y retirada precisa

Cada apply/execute requiere PR revisado y autorización para el destino/dump concretos. Usar estado Terraform separado para el scratch, con backend prefix y nombre identificados en el manifiesto; nunca importar estos recursos al estado persistente GCP-02/GCP-03. Plan de salida esperado: solo el scratch, el job, su secreto/versión temporal y el bucket temporal.

Al finalizar y preservar la evidencia saneada: (1) detener ejecuciones y borrar solo el job nombrado; (2) enumerar y borrar objetos exactos del bucket temporal desde su manifiesto, verificar que esté vacío y borrar ese bucket; (3) destruir la versión temporal y eliminar únicamente el secret container creado para el ensayo; (4) borrar roles/login/database dentro del scratch si todavía existen; (5) quitar protección de borrado solo del nombre exacto scratch si el destroy autorizado lo requiere y borrar solo esa instancia; (6) verificar por API los cuatro recursos nombrados ausentes, estado Terraform scratch vacío y no drift en el state persistente. No borrar VPC, subnet, Private Service Access, service accounts compartidas, bucket privado de archivos, bases/volúmenes persistentes, ni objetos no listados en el manifiesto.

## Referencias oficiales

- [Go Cloud SQL Connector y limitaciones de red](https://docs.cloud.google.com/sql/docs/postgres/connect-connectors)
- [Cloud Run Direct VPC egress y roles del Service Agent](https://docs.cloud.google.com/run/docs/configuring/vpc-direct-vpc)
- [Private Google Access](https://docs.cloud.google.com/vpc/docs/configure-private-google-access)
- [Cloud Run Job y permisos de ejecución](https://docs.cloud.google.com/run/docs/execute/jobs)
- [Usuarios PostgreSQL de Cloud SQL](https://docs.cloud.google.com/sql/docs/postgres/create-manage-users)
- [Extensiones Cloud SQL PostgreSQL](https://docs.cloud.google.com/sql/docs/postgres/extensions)
- [Precios de Cloud Run](https://cloud.google.com/run/pricing)
- [Precios de Cloud SQL](https://cloud.google.com/sql/pricing)
