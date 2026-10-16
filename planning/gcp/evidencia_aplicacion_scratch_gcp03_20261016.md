# GCP-03: ejecución de infraestructura scratch y evidencia

Fecha: 2026-10-16 (UTC). Proyecto `espacigo-testprod` (`447373903153`), región `southamerica-west1`. Este registro cubre únicamente la infraestructura desechable aprobada y el upload del contenedor migrador. La base Cloud SQL persistente y `espacigo_pgdata` no se modificaron.

## Plan exacto y aplicación

- PR de infraestructura #342 fue aprobado y fusionado por `HernanEspinozaDev`; HEAD revisado `16a6c7984f90ea8e62ca28e9f1b03cde07fce98f`, merge `be2f59a4d2a21c6d48163c7b51a0e789b61fbb61`.
- Se aplicó exclusivamente `.local/planning/gcp/gcp-03-scratch-20261016/gcp03-scratch-hyperdisk.tfplan`, SHA-256 `2abbf8313da6effda667f14ecd00442b7187f204a4d6f1f8805d46315977cd1c`. El resumen aprobado y ejecutado fue **8 crear, 0 cambiar, 0 destruir**. Terraform terminó código 0: `Apply complete! Resources: 8 added, 0 changed, 0 destroyed.` No se regeneró ni sustituyó el plan para el apply.
- El state corresponde al root/prefijo scratch independiente `espacigo/testprod/gcp03-scratch`; lineage `11eb96d4-a301-9029-11d1-69e1a00f6302`, serial posterior `4`. Incluye 8 recursos gestionados; los otros tres nodos del state son lecturas `data` de proyecto/VPC/subred. El root persistente no fue seleccionado.
- Terraform 1.16.6 y Google provider 8.7.0. El refresh posterior, usando el mismo archivo privado `scratch.tfvars`, terminó código 0 y `No changes. Your infrastructure matches the configuration.`
- Plan, tfvars, state, logs, respuestas completas de API, identidad y configuración de autenticación permanecen bajo `.local/planning/gcp/gcp-03-scratch-20261016/` con permisos privados. No se versionan.

## Recursos observados después del apply

Lecturas de Cloud SQL Admin, Secret Manager, Cloud Resource Manager y Cloud Run v2 se guardaron en `.local/.../postapply-api/`; el acceso de Artifact Registry se verificó con lectura del manifiesto remoto.

| Recurso | Resultado observado |
| --- | --- |
| Cloud SQL `espacigo-gcp03-pilot01-pg` | `RUNNABLE`, PostgreSQL 18, `southamerica-west1`, `db-custom-N4-2-4096`, zonal, Hyperdisk Balanced de 20 GiB, IP privada solamente, `ENCRYPTED_ONLY`, connector `REQUIRED`, protecciones de borrado habilitadas, backups/PITR deshabilitados. IP privada observada: `10.189.0.5`. |
| Base `espacigo_scratch` | Existe en la instancia scratch junto a la base de sistema `postgres`. No hubo conexión SQL; no se consultaron ni crearon tablas de aplicación. |
| Identidad scratch | `gcp03-scratch-pilot01@espacigo-testprod.iam.gserviceaccount.com`; único grant nuevo de IAM de proyecto frente al snapshot previo: `roles/cloudsql.client` para esta cuenta. |
| Secret Manager | Contenedor `espacigo-gcp03-pilot01-migrator-password` creado. La lista API de versiones devolvió cero. IAM `secretAccessor` está limitado a este secreto y a la identidad scratch. No se generó valor ni versión. |
| Artifact Registry | Repositorio `southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/dbmigrate`; Cloud Run service agent con lectura del repositorio. |
| Cloud Run Job | No existe Job `gcp03-scratch-pilot01-migrator`; `enable_job=false` y no está en el plan ni en el state. |
| Instancia persistente `espacigo-testprod-pg` | La lectura API posterior reportó `RUNNABLE`, PostgreSQL 18 y `db-custom-N4-2-4096`, igual que el snapshot pre-apply. El plan scratch no la contiene. |

El único delta en la política IAM de proyecto comparada con el snapshot previo fue el grant de Cloud SQL Client a la identidad scratch; no se removieron bindings. No se tocaron APIs, cuotas, políticas de organización, VPC, subnet, DNS o modo de facturación.

## Manifiesto de imagen verificado

- Se subió el OCI ya construido, sin reconstrucción: digest `sha256:23c753939f12d035d83684094506eb0effcb20d5a1dab5544c12280a0fc461a0`; SHA-256 del archivo OCI fuente `a651ca6c7be40e1e97fc12afa4d892ccda2d99573a049883558c5ce8b65e11d0`.
- Referencia de lectura: `southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/dbmigrate:source-8fe6ef1`. ORAS 1.3.0 reportó el mismo digest al copiar y `oras resolve` volvió a leer ese digest desde el registro. SHA-256 del manifiesto OCI recuperado: `23c753939f12d035d83684094506eb0effcb20d5a1dab5544c12280a0fc461a0`.
- El manifiesto conserva como commit de inputs del migrador `8fe6ef188ee7115b11b4ccc5479c593591bbc36e`. No se afirma que el contenedor se haya construido desde el HEAD de Terraform.

## Límites de esta ejecución

No se ejecutó SQL, `db/bootstrap/gcp03-scratch-bootstrap.sql`, migración, V60, conexión a PostgreSQL, dump, restore o importación. No se creó usuario SQL, valor/versión secreta, Cloud Run Job ni ejecución de Job. No se habilitó Paid mode, se cambiaron cuotas/policies ni se alteró DNS. No se tocó `espacigo_pgdata`, la base persistente, respaldos o archivos privados.

El scratch Cloud SQL permanece activo y puede acumular costo. El modelo local anterior estima alrededor de USD 4.86 por 24 horas para N4 y Hyperdisk Balanced de 20 GiB, antes de otros SKU; esto no es precio facturado ni verifica créditos disponibles. La tarifa regional de Cloud Run Job sigue sin confirmarse. La limpieza del scratch todavía no se ha realizado.

## Procedimiento siguiente para revisión separada

Cada etapa requiere su propio plan revisado y autorización para el recurso/objetivo exacto. Ningún paso abajo se ha ejecutado.

1. **Crear login temporal fuera de Terraform.** Generar una contraseña aleatoria en memoria/canal privado y crear `espacigo_scratch_migrator` únicamente en `espacigo-gcp03-pilot01-pg`, sin reutilizar `postgres` ni tocar la instancia persistente. Usar la API/flujo privado aprobado para la contraseña; no ponerla en argumentos, historial, logs ni state. Confirmar los atributos/membresías otorgados por Cloud SQL y asignar solo las capacidades requeridas por las migraciones.
2. **Crear una versión del secreto por canal privado.** Añadir el valor solo a `espacigo-gcp03-pilot01-migrator-password`, sin Terraform, y comprobar que el número de versiones pasa de cero a uno, que la cuenta scratch puede leer únicamente ese secreto y que las identidades API/persistentes no reciben acceso.
3. **Preparar SQL desde un ejecutor privado separado.** El OCI actual está diseñado para `cmd/dbmigrate`; no se asume que incluya `psql`. Antes de SQL, revisar un ejecutor efímero en la VPC existente, sin IP pública, con `psql` y Cloud SQL Auth Proxy/connector por IP privada y TLS requerido. Limitar su service account a `roles/cloudsql.client`, sus credenciales a la sesión aprobada y su vida al ensayo; verificar endpoint/identidad antes de conectar. El diseño, imagen y borrado del ejecutor van en un PR propio; no crear una VM/Job improvisado.
4. **Bootstrap y verificaciones SQL solo en scratch.** Con aprobación específica, ejecutar el bootstrap revisado contra `espacigo_scratch` y verificar versiones/disponibilidad de `postgis`, `btree_gist` y `unaccent`; atributos de roles (`rolsuper`, `rolcreaterole`, `rolcreatedb`, `rolbypassrls`, `rolinherit`), membresías, ownership, grants, secuencias y controles V60. Registrar resultados saneados y detenerse si Cloud SQL exige privilegios fuera del alcance.
5. **Activar el migrador después de esas verificaciones.** Preparar un plan nuevo con `enable_job=true`, la referencia de imagen por digest ya leída, confirmación de imagen y versión del secreto y un operador nombrado. Revisar el plan y crear el Job mediante un PR separado. La creación del Job no lo ejecuta; invocarlo requiere autorización operativa separada, comprobar una sola tarea, paralelismo 1 y reintentos 0, y verificar exclusivamente la base scratch.
6. **Retirar recursos con nombres exactos.** Detener/confirmar ninguna ejecución activa; eliminar el Job si se creó; retirar el ejecutor privado; eliminar el usuario/roles temporales mediante el canal correspondiente; destruir la versión del secreto y luego solo el contenedor de secreto nombrado; seguir los dos pasos de protección `allow_scratch_destroy=true` con `run_id=pilot01` y después `enable_scratch=false`; aplicar únicamente planes revisados del root scratch. Verificar por API que instancia, base, SA, secreto, repo y Job estén ausentes y que ese state scratch esté vacío. No borrar VPC/subnet ni recursos/volúmenes persistentes.

El éxito de esta preparación no demuestra compatibilidad/restauración de datos ni acepta el producto.
