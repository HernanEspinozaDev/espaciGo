# Cloud SQL — procedimiento único de migraciones incrementales

Fecha: 2026-10-23. Regla operativa solicitada por el usuario; este documento no acredita ejecución de V68–V74. Alcance: actualizar el esquema de la base existente de testprod conservando sus datos.

## Fuente y precedencia

1. Este procedimiento define la ruta normal para cambios posteriores a la importación inicial.
2. `planning/imagenes/img01_migration_role_preflight.md` en PR #533 contiene el diagnóstico y propuesta de reparación del corte de imágenes. Debe alinearse a esta regla; no crea una segunda ruta.
3. `planning/gcp/gcp08_cloud_build_transition_20261022.md` registra Cloud SQL externo/manual y que Cloud Build despliega la aplicación sin ejecutar migraciones.
4. `planning/gcp/gcp08_cloud_run_restore_20261021.md` conserva la restauración inicial V67.
5. `planning/gcp/gcp03_migrator_scratch_runner_procedure.md` es diseño histórico de scratch con otro target; no receta del testprod actual. Su conector/enforcement se evalúa solo si se verifica que aplica al destino vigente; no relajar TLS ni controles de la instancia para hacer funcionar un comando.

Evidencia de chats anteriores: revisión `01a15aa7-d365-7e81-8376-2755b4d819b7`, turnos `01a15c9b-a86d-7411-85ec-9ca03956d60b` (DMS inicial), `01a15d69-fd2c-7e80-aa50-95d26cfa2f42` (forward posterior), `01a166b6-21dd-70e2-ad2b-0120135ab650` (autorización y rotación de contraseña, acceso y diagnóstico). Desarrollo `01a15ab2-f1a8-7e21-aa01-340782309659`, turno `01a166bd-0d32-7c80-ac38-f9183c26a54d` (reparación propuesta). Son evidencia reportada, no una nueva comprobación remota de este documento.

## Dos operaciones diferentes

- Importación inicial ya realizada: PostgreSQL local → túnel inverso del bastión → DMS → Cloud SQL. Los scripts privados `dms-reverse-ssh.sh` y `prepare-source-for-dms-tunnel.sh` pertenecen a esa importación. No reiniciarlos para publicar cambios de esquema.
- Actualización habitual: archivos SQL versionados del repositorio → migrador ejecutado localmente → forward SSH del bastión → Cloud SQL existente. No copia tablas ni datos locales y no necesita volver a importar la base.

## Destino conocido, obligatorio revalidarlo por sesión

| Dato | Último valor reportado |
| --- | --- |
| Proyecto / región | `espacigo-testprod` / `southamerica-west1` |
| Instancia SQL | `espacigo-testprod-db` |
| Base | `espacigo_dms_stage_20261020` |
| Bastión / zona | `espacigo-dms-bastion-v67` / `southamerica-west1-a` |
| IP privada SQL | `10.189.0.15:5432` |
| Forward local | `127.0.0.1:15432` |
| Usuario SSH histórico | `nandev` |

Obtener metadata actual mediante `gcloud sql instances describe` y `gcloud compute instances describe` con proyecto explícito; comprobar también cuenta GCP autorizada, estado, políticas de conexión/TLS y que el puerto local no pertenece a otra sesión. Una diferencia exige explicar y actualizar el dato; no crear otro bastión o instancia.

Comando de transporte recuperado del historial, después de validar los datos y la identidad SSH:

```bash
gcloud compute ssh nandev@espacigo-dms-bastion-v67   --project=espacigo-testprod --zone=southamerica-west1-a --quiet   -- -N -o ExitOnForwardFailure=yes -o ServerAliveInterval=30   -o ServerAliveCountMax=3 -L 127.0.0.1:15432:10.189.0.15:5432
```

Ejecutarlo en una sesión controlada y registrar su PID/propietario; cerrar solo ese proceso al terminar. Conservar validación de host SSH y clave conocida; no desactivar host checking. El túnel demuestra transporte, no permisos PostgreSQL. Si el target exige conector o TLS, verificar compatibilidad antes de conectar; no desactivar políticas ni proponer otra arquitectura automáticamente.

## Identidades y secretos

El revisor es único operador Cloud/SQL. Desarrollo prepara SQL, pruebas y runbook. Mantener las cuentas GitHub y la cuenta GCP como identidades independientes.

Según ejecución autorizada del chat anterior, `postgres` fue rotado y validado. Referencias actuales reportadas: `espacigo-testprod-cloudsql-postgres-admin-password:1` y `espacigo-testprod-cloudsql-migrator-password:1`. Validar metadata y versión habilitada; obtener el valor solo para autenticar al proceso autorizado, en archivo privado con directorio 0700 y archivo 0600, sin mostrarlo ni copiarlo entre chats/worktrees. Usar mecanismo soportado por el cliente (por ejemplo `PGPASSFILE` para conexión directa), sin contraseña en `DATABASE_URL` ni argumentos. Retirar archivos temporales al cerrar sesión. No imprimir entorno ni usar tracing de shell.

Los archivos históricos `cloudsql_postgres.password` y `dms_migrator.password` fueron rechazados por el servidor: conservarlos como material privado histórico, no seguir probándolos. No se requiere que el usuario recuerde una clave ya custodiada en Secret Manager.

`espacigo_migrator_login` funciona como lector de ledger según el reporte, pero carece de DDL. No tratarlo como ejecutor de esquema ni resolverlo con grants amplios. Las tablas afectadas pertenecen a `postgres` según el inventario anterior. PostgreSQL requiere autoridad de owner para ALTER TABLE; revisar para este delta una sesión acotada del owner existente, con SQL/plan exactos y las autorizaciones aplicables, sin elevar runtime ni transferir globalmente ownership. La autorización anterior de reset no es autorización automática para cualquier reparación.

## Secuencia reproducible de cada corte

1. Fijar PR y SHA de código/migraciones revisados. Preservar cada migración aplicada byte a byte. Probar en PostgreSQL desechable tanto base vacía como actualización desde la versión remota, con extensiones necesarias. No usar el volumen persistente local.
2. Verificar destino y abrir el forward anterior. Conectar primero en transacción read-only; comprobar `current_database()`, `session_user`, `current_user`, versión, ledger con versiones/nombres/checksums, objetos y owners necesarios. La IP observada por PostgreSQL puede ser del bastión: cotejar metadata Cloud SQL también. No leer datos de negocio o hashes de passwords para este diagnóstico.
3. Confirmar backup recuperable satisfactorio y su antigüedad aceptable para el delta; registrar ID/estado/fecha. Si hace falta backup nuevo, usar el mecanismo ya existente y esperar éxito. Acordar recuperación antes de escribir; no borrar/recrear la base ante un error.
4. Ejecutar el plan de `cmd/dbmigrate` del SHA revisado. **Atención:** `main` en `937c49c` NO implementa `-plan-only`; PR #533 sí lo propone. Antes de usar esa opción, verificar ayuda/código y pruebas del binario exacto. No ejecutar el migrador normal como si fuera una consulta: puede escribir. Revisar/compilar el candidato apropiado para el plan; aplicar solo código revisado y las aprobaciones requeridas por el repositorio.
5. Si ledger/objetos/checksums divergen, detener la aplicación y preparar una reparación mínima versionada y probada. No editar ledger ni omitir guards. Revisar reparación y plan concreto; registrar aprobación y autoridad SQL necesarias sin pedir de nuevo autorizaciones que ya cubran esa operación exacta.
6. Aplicar únicamente el delta aprobado con `cmd/dbmigrate -dir db/migrations`, ejecutado desde el checkout/SHA correcto y con destino/identidad revisados, por el mismo forward. No ejecutar todos los archivos manualmente con un bucle `psql`. Conservar el control de concurrencia y transacciones del migrador; no lanzar dos operadores. Ante error, comprobar commit/ledger real antes de reintentar.
7. Repetir plan: cero pendientes para el SHA aplicado. Verificar objetos, constraints, roles/ACL y separación de runtime requeridos. Registrar versiones antes/después, SHA, hashes, identidad, backup, resultados y límites sin secretos.
8. Desplegar aplicación mediante el flujo Cloud Build existente cuando su versión sea compatible con el esquema. Verificar online registro → envío/recepción/verificación de correo → login → publicación con varias imágenes → lectura/borrado y persistencia. Sintéticos/locales no equivalen a E2E online. Cerrar forward y archivos temporales propios.

## Estado concreto que deben retomar los nuevos chats

- PR #533 abierto, HEAD comprobada al redactar `65ae5866b728631a7e93b46d416be6097cfc9fd2`, con cambios solicitados. Worktree existente: `/home/nandev/.codex/worktrees/img01-ledger-diagnosis/espaciGo`.
- Reporte remoto: ledger V1–V67, V64 marcada aplicada pero falta `espacigo_privacy_cleanup_worker`; `-plan-only` se detiene correctamente. La causa de la pérdida del rol no está demostrada. No atribuirla automáticamente a DMS ni a Terraform.
- `db/repair/reconcile_v64_cleanup_worker_role.sql` en #533 es propuesta probada en PostgreSQL 18 desechable, pendiente de revisión y aplicación remota. No reescribir V64 ni crear el rol fuera de esa reparación revisada.
- V68–V74 pendientes según ese reporte; comprobar estado de nuevo antes de aplicar. SQL Studio no es requisito ni se reinicia la importación DMS.
- STARTTLS/AUTH de Zoho reportados correctos; falta verificar configuración SMTP desplegada y correo real del flujo online. Imágenes/registro no se consideran cerrados hasta probarlos online.

## Diagnóstico sin cambiar de ruta

| Resultado | Acción |
| --- | --- |
| SSH o forward fallan | Revisar sesión/VM/ruta/puerto existente; sin DDL ni infraestructura alternativa |
| Password authentication failed | Verificar referencia/versión/usuario del secreto vigente; no repetir archivos rechazados ni resetear por rutina |
| Permission denied / owner | Identificar operación y objeto; revisar identidad adecuada, sin elevar runtime |
| Ledger checksum / objeto ausente | Reparación revisada del drift, conservando historial |
| Plan sin pendientes | Postcheck y release compatible; no reimportación |

Un cambio de red, autenticación, ownership o runner exige evidencia del fallo, propuesta acotada y revisión documentada. El objetivo es resolver el bloqueo dentro de este flujo, no iniciar ciclos de herramientas alternativas.

Referencia oficial consultada: [conectividad DMS PostgreSQL](https://docs.cloud.google.com/database-migration/docs/postgres/networking-methods). Describe el túnel inverso para conectar DMS al origen; no prescribe su reutilización para migraciones incrementales de la aplicación.

## Registro vigente del esquema — verificación 2026-11-05

Esta entrada sustituye los valores operativos remotos históricos de §Estado concreto; no borra aquellos diagnósticos. La versión del esquema corresponde al ledger, no al motor ni a la aplicación desplegada.

| Destino separado | Motor / ledger verificado | Fuente y pendientes |
| --- | --- | --- |
| Cloud SQL `espacigo-testprod-db`, base `espacigo_dms_stage_20261020`; SSH propio `127.0.0.1:15432` → `10.189.0.15:5432` | PostgreSQL18.6; 83 entradas contiguas V1–V83, lectura READ ONLY posterior al commit | Candidato ejecutado `dfcecfdb6fd0a373769135dbc46bfb1dbff0e5bf`; 83 nombres/checksums coincidentes, cero pendientes. Migraciones idénticas a `9edd61c405bc01f7f5ac5c46e616a6d3063bdf5f`. |
| Ensayo `espacigo-spaces-cloud-recovery-20261105`, namespace aislado sin puertos host; loopback15432 interno | PostgreSQL18.6; V83/cero cuentas | Restore solo schema autorizado + metadata literal del ledger75; CLI real75→83 y fallo/reanudación78, sin cuentas cloud. No sustituye al destino remoto. |
| Ensayo previo `espacigo-spaces-cloud-delta-20261105`, namespace aislado sin puertos host | PostgreSQL18.6; V83/cero cuentas | Ensayo real de commit82 y fallo83/rollback de función + recuperación83; conservado. |
| PostgreSQL nativo persistente `127.0.0.1:5432`, base homónima | **No inspeccionado en este corte** | No inferir su versión a partir de Cloud SQL ni de contenedores; no se ejecutó DDL ni API sobre él. |

- Aplicación remota real: V76–V80 mediante modo Cloud Owner con inventario pinned y mínimo/máximo bajo lock; preparación exacta V81; V81 y luego V82–V83 mediante `espacigo_migrator_login`/`SET ROLE espacigo_schema_owner`. Repairs versionados delimitan y cierran la ventana V82/V83. No se utilizó la excepción F5 ni un wrapper local en Cloud SQL.
- Postcheck remoto: propietario de función restaurado a postgres; CREATE/membership/grant-option temporales retirados; el grant adicional UPDATE(id) a runtime es directo de postgres sin GO para la tabla reviewer; helpers privados con owner NOLOGIN y EXECUTE runtime. Inventario ACL de14columnas en catálogo compara todos los objetos previos/owners/defaults/PUBLIC/initprivs; los únicos cambios previos son los seis prescritos por preparación (cuatro celdas de columnas, USAGE de public y ledger), no drift adicional. Roles/memberships exactos.
- Backups: gestionado `1793899104062` SUCCESSFUL; custom remoto `/home/nandev/.local/share/espacigo-cloudsql-backups/spaces-v75-pre-delta-20261105.dump`, modo0600, 7472032bytes, SHA256 `a077fd6e75f476af3db59fe18f7228cadc42eaaadadfea9cfc06aa80f10e650d`, listado y parse pg_restore PASS. No confundir validación del archivo con una restauración cloud ejecutada.
- Evidencia y límites: `espacios_cloudsql_v83_aplicacion_20261105.json`, `espacios_cloudsql_v83_ensayo_local_20261105.json` y `espacios_cloudsql_v83_ejecucion_20261105.md`. Nueve funciones y147constraints remotos coinciden con el clon ensayado. Integración a main, Cloud Run y recorrido funcional online no se cierran por este cambio; este último lo realizará el usuario.
- Transporte retirado después del postcheck: forward host15432 cerrado, bastión devuelto a TERMINATED (su estado inicial) y credenciales transitorias propias eliminadas. La ruta de la tabla describe el transporte **utilizado**, no un listener actualmente activo. Para otra captura/release, revalidar y abrir una nueva sesión propia por el mismo procedimiento; no reutilizar passfiles retirados.
