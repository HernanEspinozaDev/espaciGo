# GCP-03: preparación del snapshot de origen y compatibilidad

Estado: inventario de solo lectura completo; captura de backup y ensayo de restore pendientes de revisión específica. GCP-03 sigue parcial. GCP-04 no se inicia ni se declara completado con este trabajo.

## Inventario observado

El inventario reproducible está en [`aggregate-inventory.json`](evidence/gcp03-origin-snapshot-prep-20261016/aggregate-inventory.json). Su detalle privado, con roles, ACL, nombres internos y hashes por archivo, permanece en `.local/testing/gcp03-origin-snapshot-prep-20261016-02/origin-private-inventory.json` (directorio 0700 y archivo 0600). No se versiona. El comando que lo genera es:

```sh
python3 scripts/gcp03-origin-inventory.py \
  --private-output-dir .local/testing/<directorio-nuevo>
```

El script rehúsa reemplazar una carpeta existente. Consulta el catálogo en una transacción `REPEATABLE READ READ ONLY`, compara el historial con los bytes de V1–V61 y calcula inventarios y huellas de archivos. Solo escribe el manifiesto privado bajo `.local/testing`; no crea un dump, copia contenido ni llama a Google Cloud. La consulta requiere la credencial administrativa montada en el contenedor: se leyó en memoria para autenticar la conexión de solo lectura; no se imprimió, guardó ni copió. La llave de cifrado de contratos se leyó únicamente para calcular una huella; su contenido no se guardó ni copió. Las otras credenciales de origen no se leyeron.

Los agregados observados incluyen PostgreSQL 18.6, 74.5 MB, 163 relaciones públicas, 19 roles, 3 membresías, 12 rutinas `SECURITY DEFINER`, 221 columnas UUID, 46 int64 y 61 migraciones coincidentes byte por byte. El conteo de archivos privados fue 3.245. La base conserva 958 avisos pendientes. Aunque en ese instante no había sesiones vigentes, tokens de acción elegibles ni eventos pendientes en el outbox de identidad, el siguiente restore debe invalidar sesiones/tokens de forma uniforme y mantener cualquier salida externa apagada.

## Límite de consistencia detectado

La base se observó en un contenedor sano del proyecto Compose esperado. Sin embargo, el contenedor backend usa otro root de Compose y otro worktree, actualmente limpio y detached en `54de15c66c2e65dd54fcabb0ca6b756a4f495e93`, mientras el repositorio de esta preparación está en `f7b3c55b77e341567f4a81fdd13efb5375351d4b`. Ese backend también inicia workers de avisos, identidad y privacidad. Los 958 avisos pendientes hacen inseguro exponer un restore a esos workers o a proveedores externos.

El catálogo se leyó en una transacción consistente, pero la enumeración y huella de archivos ocurrió después, mientras la aplicación seguía activa. Por tanto, el manifiesto **no** es un punto atómico DB+archivos ni una copia de respaldo. Antes de una captura deberá acordarse una ventana de mantenimiento, fijar código e imagen exactos, confirmar que backend y DB corresponden a la misma revisión/configuración y detener de forma controlada todos los escritores y workers. No basta con pausar únicamente el proceso HTTP.

## Procedimiento propuesto para una captura futura

Este procedimiento queda documentado, no ejecutado. Cada acción que lea o copie payloads requiere revisión del PR y autorización concreta de captura:

1. Resolver el desajuste de Compose/worktree y registrar commit, digest de imagen, configuración no secreta, nombre lógico de DB y montajes de DB/archivos. No cambiar ni reiniciar el origen para resolverlo.
2. Acordar ventana con el usuario. Detener limpiamente todos los writers y workers del origen, verificar cero transacciones de aplicación activas y cero leases pendientes, y bloquear nuevos escritores. Si no puede probarse el quiesce, no capturar.
3. Generar un manifiesto privado previo, bajo `.local/testing` con directorio 0700 y archivos 0600. Debe incluir run-id aleatorio, hora UTC, revisión/configuración, conteos agregados, huellas de artefactos, tabla de migraciones V1–V61 y evidencia de quiesce. No incluir valores personales, contenido de credenciales ni payloads de llaves.
4. Capturar un archivo PostgreSQL custom con `pg_dump` desde la conexión autenticada de forma efímera; conservar roles, owners, ACL, ACL por defecto, membresías y atributos en el inventario privado separado. No exportar contraseñas/hashes de roles con `pg_dumpall --globals-only`. No alterar owners/ACL en el archivo para ocultar incompatibilidades. Cualquier mapeo a roles administrados de Cloud SQL se define y revisa como parte del restore.
5. En la misma ventana y con los writers bloqueados, copiar el árbol de evidencia privada y el registro de replay de privacidad a un artefacto local cifrado en reposo, generando manifiesto por ruta relativa, modo, tamaño y SHA-256. No almacenar paths absolutos. El objeto de copia no se sube a Cloud en esta etapa.
6. La llave de cifrado de contratos es necesaria para descifrar los documentos restaurados. Su captura requiere destino de custodia cifrada y autorización aparte; no meterla en el dump, Git, logs, manifiesto público o bucket. Generar credenciales destino distintas; no copiar password DB, password runtime, secreto de webhook ni credenciales de origen. TLS se configura de nuevo con certificados de destino.
7. Finalizar manifiesto y hashes; reanudar el origen solo cuando el backup local tenga integridad comprobada y todos los servicios puedan volver a operar. No borrar fuente ni volumen persistente.

No se creó todavía un script de dump/copia deliberadamente: faltan el procedimiento de quiesce acordado, resolver la revisión activa y acordar custodia de la llave. El script actual es el preflight reproducible de solo lectura.

## Compatibilidad PostgreSQL/Cloud SQL

- Conservar las migraciones V1–V61 y sus bytes/checksums. El restore no debe volver a ejecutar migraciones ni transformar UUID o int64; verificar también secuencias, índices, FK, constraints, triggers y conteos después de restaurar.
- Cloud SQL administra roles privilegiados de forma distinta a un PostgreSQL local. La matriz privada de owners/ACL y las rutinas `SECURITY DEFINER` debe convertirse en un mapeo explícito de roles disponibles, privilegio mínimo y grants revisados. No reasignar roles ni ejecutar `CREATE ROLE` en el ensayo siguiente sin una aprobación distinta.
- El origen registra `postgis 3.6.4`, `postgis_tiger_geocoder 3.6.4` y `postgis_topology 3.6.4`, además de `btree_gist`, `fuzzystrmatch` y `unaccent`. La documentación de Cloud SQL indica PostGIS 3.6.0 para PostgreSQL 18 y soporte de `fuzzystrmatch` 1.2 y de la familia PostGIS, pero la disponibilidad no demuestra que el target actual tenga las mismas versiones o extensiones instaladas. Comparar `pg_extension`, `postgis_full_version()`, operators/functions y consultas espaciales antes de abrir el restore a la app. Las versiones 3.6.4 del ensayo anterior son observaciones históricas y no deben darse por garantizadas. [Documentación de extensiones de Cloud SQL](https://docs.cloud.google.com/sql/docs/postgres/extensions)
- El restore anterior observó en Cloud SQL PostgreSQL 18.6 y PostGIS 3.6.4, pero quedó pendiente la validación espacial completa y hubo un fallo de retiro no resuelto. Eso no acredita compatibilidad integral ni cambia el estado parcial de GCP-03.

## Tratamiento seguro de la copia

Todo saneamiento es exclusivamente sobre una copia restaurada. Antes de iniciar cualquier API o worker:

- Invalidar todas las sesiones y tokens de acción copiadas; comprobar cero sesiones activas y cero tokens utilizables.
- Mantener workers, correo, webhooks, pagos y proveedores deshabilitados. Hay 958 avisos pendientes y el backend inventariado los procesa automáticamente. Elegir, revisar y probar una de estas alternativas: cancelar/quarantinar las colas en la copia con restricciones de integridad validadas, o conectar a un sink privado como Mailpit con egress bloqueado. No apuntar a proveedores reales ni usar secretos de origen.
- El outbox de identidad estaba completamente entregado; eso no elimina el riesgo de `aviso_local`. Las operaciones de pago son fixtures locales agregadas, pero igualmente no deben emitir efectos externos.
- Restaurar el registro de replay de privacidad antes de cualquier API; validar sus huellas y evitar ejecutar workers de limpieza hasta que las rutas destino estén aisladas.
- Mantener evidencia privada, llaves y artefactos fuera de logs/capturas/PR. Revisar permisos 0700/0600 y borrar credenciales de destino al retirar el ensayo.

## Ensayo siguiente y costo

El siguiente ensayo debe ser uno integrado, con una sola ventana y un solo PR/revisión: checksum V1–V61; DB+archivos+llave bajo punto de consistencia aprobado; extensions/PostGIS; owners/ACL/`SECURITY DEFINER`; UUID/int64; secuencias/índices/FK/constraints; consultas espaciales y de disponibilidad/reservas; replay de privacidad; invalidación de sesiones/tokens; cuarentena de avisos/outbox/proveedores; conectividad privada/TLS; restore repetible, rollback, teardown revisado y evidencia final.

La alternativa de menor costo es continuar con `pilot01` únicamente para compatibilidad de lectura e inventario usando el mapa de roles ya existente, sin alta ni reasignación de roles SQL. Ese clúster comparte catálogo global de roles y recursos CPU/I/O; no equivale a aislamiento y puede afectar la carga compartida. Si el ensayo requiere validar roles/owners completos o aislar workers/colas, no es candidato hasta demostrar ese aislamiento; se evaluaría una instancia temporal separada solo tras revisar este procedimiento y la estimación actual. Los costos históricos no son una cotización vigente. No se cambia el Free Trial, billing, cuotas ni recursos en esta etapa. [Precios de Cloud SQL](https://cloud.google.com/sql/pricing)

## Estado y alcance

Se ejecutó únicamente la inspección local de solo lectura descrita arriba; no se creó dump, copia de archivos/llaves, recurso Cloud SQL, Job ni cambio de roles cloud/SQL. El origen permaneció en ejecución. El siguiente paso requiere revisión de este procedimiento antes de autorizar captura. GCP-03 permanece parcial y GCP-04 permanece sin iniciar; no hay aceptación humana o de producto inferida.
