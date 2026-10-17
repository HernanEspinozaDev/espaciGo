# GCP-03: preflight de compatibilidad Cloud SQL

Fecha de lectura de destino: 2026-10-17, ventana aproximada 07:14–07:15 UTC. Código base: merge de PR #390 `1e93e82ff425479d29679168ba053c43baaa3584`. Proyecto consultado explícitamente: `espacigo-testprod` (`447373903153`), región `southamerica-west1`. Evidencia saneada: [inventario API](evidence/gcp03-cloudsql-compatibility-preflight-20261017.json).

## Alcance y resultado

La lectura actual fue de solo lectura por gcloud, con `--project=espacigo-testprod` y `--billing-project=espacigo-testprod` en cada consulta. No se cambió proyecto/quota por defecto, configuración ADC, flags de instancias ni recursos. GCP-03 continúa parcial. No se creó una conexión, job, usuario/rol, base, instancia, bucket o recurso de red; no se importó el snapshot ni se descifraron su payload/llave. V1–V61 permanecen intactas.

| Instancia | Estado y motor | Tamaño/configuración | Base(s) | Usuario(s) Cloud SQL API |
| --- | --- | --- | --- | --- |
| `espacigo-testprod-pg` | `RUNNABLE`, `POSTGRES_18` | Enterprise, N4 2 vCPU/4 GiB, disco 20 GiB Hyperdisk Balanced, solo IP privada, connector requerido, TLS `ENCRYPTED_ONLY`, protección contra borrado; backup y PITR habilitados | `postgres`, `espacigo` | `postgres` |
| `espacigo-gcp03-pilot01-pg` | `RUNNABLE`, `POSTGRES_18` | Enterprise, N4 2 vCPU/4 GiB, disco 20 GiB Hyperdisk Balanced, solo IP privada, connector requerido, TLS `ENCRYPTED_ONLY`, protección contra borrado; backups/PITR deshabilitados | `postgres`, `espacigo_scratch` | `postgres` |

La lista de usuarios es la vista Cloud SQL Admin API; no enumera necesariamente todos los roles PostgreSQL (`pg_roles`) ni sus ACL dentro de cada base. La última observación SQL versionada de `pilot01` es del 2026-10-16: PostgreSQL `180006`/18.6, PostGIS 3.6.4, `btree_gist` 1.8, `plpgsql` 1.0 y `unaccent` 1.1; 16 comprobaciones runtime ACL, 377 relaciones, 1.031 funciones y 12 `SECURITY DEFINER`. Es evidencia histórica del piloto, no una lectura actual ni de `testprod` ([informe del observer](evidencia_ejecucion_observer_gcp03_20261016.md)).

Se solicitaron dos lecturas `ExecuteSql` para consultar `current_database()` y `server_version`, una en `espacigo` y otra en `espacigo_scratch`. Ambas fueron rechazadas antes de ejecutar SQL con HTTP 400: las instancias no permiten ExecuteSql mientras `settings.dataApiAccess` no sea `ALLOW_DATA_API`. No habilité el ajuste. Por tanto, la versión menor actual, extensiones instaladas/propietarios, `pg_roles`, membresías, owners y ACL actuales no están comprobados en esta lectura. No intentaré abrir otra ruta ni crear acceso dentro de este preflight. Una operación posterior puede proponer un observador SQL fijo, read-only, en una vía privada ya disponible; si necesita tocar `dataApiAccess`, crear identidad o job, debe traer plan exacto aparte.

Comando no mutador que puede repetirse en una sesión futura con una vía de consulta autorizada (los intentos de hoy terminaron antes de ejecutar el SQL):

```bash
gcloud sql instances execute-sql espacigo-testprod-pg \
  --project=espacigo-testprod --billing-project=espacigo-testprod \
  --database=espacigo \
  --sql='SELECT current_database(), current_setting($$server_version$$)'
```

## Versiones y diferencias a comprobar

La API de instancias confirma PostgreSQL 18 como versión mayor, pero no informa el patch menor actual. El inventario read-only histórico del origen, observado antes de la ventana de captura (`2026-10-17T02:38:24.478965Z`), registró PostgreSQL 18.6 y estas extensiones: `btree_gist` 1.8, `fuzzystrmatch` 1.2, `plpgsql` 1.0, `postgis` 3.6.4, `postgis_tiger_geocoder` 3.6.4, `postgis_topology` 3.6.4 y `unaccent` 1.1 ([inventario agregado histórico](evidence/gcp03-origin-snapshot-prep-20261016/aggregate-inventory.json)). Es una observación histórica, no una lectura del contenido del snapshot sellado en la ventana posterior.

La política vigente de [versiones Cloud SQL](https://docs.cloud.google.com/sql/docs/db-versions) lista PostgreSQL 18 con 18.6 como default; la tabla de [extensiones soportadas por Cloud SQL PostgreSQL](https://docs.cloud.google.com/sql/docs/postgres/extensions) indica para PostgreSQL 18: PostGIS 3.6.0, `btree_gist` 1.8, `fuzzystrmatch` 1.2, `plpgsql` 1.0 y `unaccent` 1.1. Documenta actualizaciones manuales de extensiones; una versión soportada no demuestra que esté instalada en una instancia concreta. La diferencia publicada entre PostGIS 3.6.4 del origen/piloto observado y 3.6.0 de la tabla oficial es una brecha a resolver, no evidencia de incompatibilidad ni de equivalencia. PostGIS incluye extensiones relacionadas; deben verificarse versión y owner por separado en `pg_extension`/`pg_depend`.

La comparación read-only integral que falta debe limitarse a metadatos y agregados saneados, contra el manifiesto del origen, sin exportar filas personales o definiciones SQL al PR:

- Motor y extensiones: `server_version_num`, `postgis_full_version()`, `extname`, `extversion`, `extowner` y membresía en extensiones, incluidas `postgis_tiger_geocoder`/`postgis_topology` y objetos como `spatial_ref_sys`.
- Roles/ACL: atributos sin hashes de contraseña, memberships/options, ACL de database/schema/relations/sequences/functions y ACL por defecto; comparar owner por tipo de objeto. No interpretar la lista de SQL users API como inventario completo de roles.
- Esquema: por columna, tipo/UDT, typmod, nullability, default, identity/generated; por constraint, definición y valid/deferrable; por índice, definición, opclass, predicate y flags valid/ready; por secuencia, definición, owner, dependencia y estado de asignación. Comparar hashes saneados en privado, no contenido de tablas.
- Rutinas: owner, lenguaje, `prosecdef`, firma, hash local de cuerpo/definición, `proconfig`/`search_path` y ACL `EXECUTE`; incluir configuración y referencias a objetos de extensión. El informe del 16/10 registró 12 `SECURITY DEFINER` con `search_path=pg_catalog, public` y distintos grupos de grants, pero no reemplaza la comparación del snapshot ni incluye aquí los cuerpos.
- Funcional: V1–V61 y checksums byte a byte; conteos/estados seleccionados; validación de secuencias, FK, constraints e índices; consultas de geometría/intersección/distancia y consultas de disponibilidad/reservas con fixtures sintéticos. Los checks deben fallar si aparecen objetos/owners/grants inesperados o se pierden privilegios requeridos.

Las 18 comparaciones de captura/restore local son catálogo/ACL y agregados seleccionados. No equivalen a hashes de todas las filas ni a definiciones exhaustivas de esquema, funciones, constraints o consultas. La evidencia de 3.246 archivos debe verificarse contra el manifiesto sellado y un punto coherente DB↔archivos durante el restore aprobado; esta lectura API no vuelve a validar contenido de archivos.

## Mapa propuesto, no aplicado

El origen tiene 19 roles y 3 memberships según su inventario agregado. Su usuario local de diagnóstico `espacigo_admin` no debe trasladarse como superusuario: Cloud SQL no ofrece el superusuario PostgreSQL estándar. El destino `postgres` se reserva para bootstrap controlado en un ensayo aislado. La lista de usuarios destino solo muestra `postgres`; los roles internos actuales siguen desconocidos por el bloqueo de catálogo.

Propuesta para revisar antes de importar:

| Responsabilidad | Mapeo candidato |
| --- | --- |
| Propietario de objetos de aplicación | Un rol dedicado `espacigo_owner NOLOGIN`, si un preflight SQL confirma que puede crearse y que los objetos fuente no necesitan propietarios incompatibles. Restaurar objetos de aplicación con `--no-owner --no-acl` y `--role=espacigo_owner`; emitir grants revisados por separado. No convertirlo en login ni miembro de `cloudsqlsuperuser`. |
| Runtime | `espacigo_runtime` restringido y grants equivalentes a `internal/dbbootstrap.GrantRuntimePermissions`; principal de login/IAM separado, concedido solo después de verificar la identidad de ejecución. Nunca `cloudsqlsuperuser`. |
| Rol V60 | `espacigo_gadm_bootstrap NOLOGIN` con atributos/memberships/grants exactos de V60. No activar una cuenta real ni usarlo para la migración. |
| Migración/restore | Login efímero distinto de runtime; usar privilegio mínimo y elevar únicamente el bootstrap aislado si pruebas previas demuestran que una extensión administrada lo requiere. No persistirlo como propietario ni conservar su permiso al acabar. |
| Extensiones | Preinstalar únicamente las extensiones soportadas en la versión destino. Mantener owners y objetos administrados por extensión según el owner real de `pg_extension`; no reasignarlos al owner de aplicación. Verificar pertenencia mediante `pg_depend`. |
| `SECURITY DEFINER` | Conservar cuerpo/hash, firma, lenguaje, owner, `proconfig`/`search_path` y `EXECUTE` ACL. Mapear al owner restringido previsto por la rutina; no otorgar EXECUTE global ni cambiar la política de triggers por conveniencia. |

El TOC exacto no se reconstruyó ni descifró en este corte. En una operación posterior aprobada, generar `pg_restore --list` dentro del entorno privado, registrar hash y revisar cada ID. Tras preinstalar una extensión con plan revisado, se puede seleccionar por ID únicamente su DDL `CREATE/COMMENT EXTENSION` correspondiente. No excluir datos por ser propiedad de una extensión: comparar `extconfig`/`extcondition`, dependencias y filas de configuración/datos custom (incluidos posibles SRID añadidos en `spatial_ref_sys`), y preservarlos, restaurarlos o mapearlos explícitamente con evidencia. Si se perdería o no se puede determinar un dato de extensión, abortar. No usar filtros amplios de schema ni omitir tablas de aplicación. Restaurar estructura/datos en el rol de owner y aplicar ACL allowlist después. Ningún TOC/restore se ejecutó aquí.

## Dónde ensayar, costo y procedimiento

No usar `espacigo-testprod-pg` (`espacigo`), porque es la base persistente del entorno, ni escribir sobre `espacigo_scratch` de `pilot01`: sus roles son de clúster y la lectura actual no estableció el estado completo de objetos/ACL. Aunque el observer anterior vio PostGIS 3.6.4 en `pilot01`, no basta para autorizar un restore en su instancia compartida.

**Opción recomendada para el restore futuro:** una instancia Cloud SQL temporal nueva, run-scoped y aislada en `southamerica-west1`, PostgreSQL 18, Enterprise zonal, red privada existente, sin IPv4 pública, `connectorEnforcement=REQUIRED`, TLS `ENCRYPTED_ONLY`, protección contra borrado y ninguna API/servicio de aplicación. Una tarea Cloud Run one-shot, sin entrada, en la VPC existente usaría SQL Auth Proxy/connector hacia esa instancia y tendría acceso mínimo a los artefactos solo tras una autorización distinta para descifrar/transferir el snapshot. Confirmar Private Google Access/rutas antes de elegir almacenamiento de entrada; no asumir que la subred permite alcanzar bucket/endpoints. Seleccionar tier/disco solo después de estimar carga; N4 2 vCPU/4 GiB y 20 GiB Hyperdisk Balanced sirven como referencia de comparación porque son la forma actual de las dos instancias existentes, no como configuración aprobada. Primero confirmar en una instancia scratch vacía la versión exacta de PostGIS/otras extensiones, roles y ACL; si la versión soportada no satisface los usos requeridos, abortar y someter alternativa a revisión. No se aprueba aquí crearla.

El costo incremental debe modelar horas de instancia PostgreSQL y disco, backups/PITR elegidos, almacenamiento/operaciones/transferencia del bundle si después se autoriza subirlo, el ejecutor de restore de una sola tarea, Artifact Registry y Cloud Logging. Los dos SQL actuales ya están `RUNNABLE`; `testprod` tiene backup/PITR y `pilot01` no. No calculé una cotización vigente en este PR ni trato el saldo Free Trial como tope. Antes de cualquier apply debe adjuntarse exportación de la calculadora oficial para 24 h, 30 días y duración real, con región, SKU, tráfico, backup, retención y supuestos documentados.

Secuencia propuesta, pendiente de plan/operación revisada:

1. Releer con API explícita inventario, SQL versions, protectores, network, cuotas y costos; obtener una vía existente y autorizada para el observador SQL read-only. No habilitar Data API ni crear usuarios para desbloquearlo sin plan revisado.
2. Comparar manifiesto sellado y hashes sin divulgar contenido; la autorización de esta tarea no permite descifrar ni transferir el payload. Separar después esa autorización de la creación de infraestructura/importación.
3. Generar un Terraform plan run-scoped exacto para la instancia aislada y ejecutor one-shot en red privada; validar estado, recursos añadidos, precio y plan de teardown. No usar provisioners para restaurar.
4. Solo después de aprobación concreta del plan y autorización de uso de snapshot, preparar roles/extensiones; revisar TOC por ID, ACL/owners y pruebas. Ejecutar restore una vez en copia, sin app/workers, conservar salida saneada y cifrar/retener los artefactos según custodia.
5. Verificar los checks de metadatos, funciones, índices/FK/secuencias, queries espaciales/reservas, invalidación de sesiones/tokens, cuarentena y hashes privados DB↔archivos. Cualquier diferencia no explicada detiene el ensayo; no aplicar cambios al origen.
6. Teardown futuro solo con plan revisado que enumere los recursos nuevos exactos: retirar primero el job/identidades/artefactos run-scoped, y al final la instancia tras quitar protecciones en etapa aprobada. Conservar los backends Terraform, red/subred, repositorio compartido, `espacigo-testprod-pg`, `espacigo-gcp03-pilot01-pg` y snapshot original. No `destroy` global.

## Criterios de aceptación y aborto

**Aceptar el ensayo de compatibilidad** solo si el patch/extensiones están registrados, todas las diferencias de roles/owners/ACL/`SECURITY DEFINER` tienen mapeo aprobado, V1–V61 permanecen idénticas, índices/FK/constraints/secuencias son válidos, las consultas espaciales/reservas pasan con datos de prueba, y los hashes privados DB↔archivos coinciden en el punto de corte, sin writers ni proveedores activos.

**Abortar** ante Data API/red/acceso no aprobados, extensión o función ausente/incompatible, owner que requiera privilegio no aceptado, grant más amplio o faltante, cualquier diferencia no explicada de definición/índice/secuencia, query funcional fallida, hash de bundle divergente, snapshot no sellado o inconsistencia DB↔archivos, superficie pública inesperada, cuota/costo sin confirmar o plan que afecte una instancia/estado ajeno. No ejecutar bootstrap, `CREATE/ALTER ROLE`, `CREATE EXTENSION`, import, `apply` o teardown por inferencia de este documento.

## Coordinación vigente

La comunicación del proyecto usa los IDs autorizados vigentes y sus límites están en [AGENTS.md](../../AGENTS.md). Los IDs anteriores se mantienen allí solo como referencia histórica/consulta.
