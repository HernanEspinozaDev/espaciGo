# GCP-06 candidato: preflight de cuenta y runtime

**Estado:** preparación local accionable para revisión. No ejecuta ni autoriza cambios en Cloud.
**Alcance:** únicamente la instancia candidata GCP-06 y los servicios de ensayo `espacigo-test-api`, `espacigo-test-public` y `espacigo-test-private`. La instancia histórica `espacigo-testprod-pg`, producción y sus datos quedan fuera.

## Resultado buscado

Dejar una secuencia verificable para habilitar el candidato importado como backend del API de ensayo, crear la cuenta de ensayo solicitada por un flujo permitido, y comprobar los dos frontends contra el API. El orden de trabajo es: verificar el candidato y su esquema; acordar rol y mecanismo de cuenta; preparar permisos mínimos; revisar el diff de Terraform y el costo; ejecutar en una ventana autorizada; verificar API y frontends; y conservar una reversa exacta.

PR #459 sigue abierto y contiene el estado de importación y la propuesta de observación SQL; no se considera fusionado ni su propuesta se considera implementada. Este corte añade un modo Terraform opt-in para que una revisión futura pueda dirigir el API a la candidata y un CLI Cloud distinto del `gadm-admin` local. Ninguno se ejecutó contra Cloud.

## Bloqueos que requieren resolver antes de ejecutar

1. **Rol de aplicación:** el objetivo de cuenta maestra es `administrador_global`; este rol es distinto de `administrador`. El CLI existente `gadm-admin activate-local` solo permite `espacigo_local`, por lo que no sirve para Cloud. El nuevo camino Cloud busca la cuenta exacta `hernan.espinoza.dev@gmail.com` a través del login runtime en la candidata y solo continúa si está activa, verificada y desbloqueada. Después llama la misma transacción/auditoría de `internal/adminglobal.Bootstrap` desde un login de activación restringido.
2. **Credencial compartida de usuarios:** la política privada disponible autoriza la contraseña guardada solo para ensayos locales. No se copiará ni reutilizará en Cloud. No hay versión Cloud-scoped disponible en este corte, y no existe todavía un flujo de reset masivo de cuentas importadas con historial y auditoría. Por eso el acceso común de esos usuarios queda bloqueado; no se añadirá una clave por default ni se aplicará un `UPDATE` directo.
3. **Importación y permisos:** PR #459 reporta la importación completada, pero faltan verificaciones SQL independientes de ledger, conteos, extensiones, ownership, ACL y permisos runtime. La observación anterior reportó solo el login `postgres`; no había un login runtime/observer disponible para validar. No conectar la app hasta que esa evidencia read-only pase para la instancia y base allowlisted.
4. **Cuenta maestra:** la importación elimina cuentas administrativas. Primero se debe registrar y verificar el correo propietario por el flujo normal Cloud. El CLI consulta directamente esa cuenta por correo normalizado mediante el login runtime, no imprime correo, UUID ni filas y se detiene si no encuentra exactamente una cuenta activa/desbloqueada. La transacción de bootstrap vuelve a comprobar elegibilidad y unicidad.
5. **Mecanismo mínimo de acceso:** la base es privada y el host de desarrollo no tiene ruta VPC. La observación debe ejecutarse desde una sesión temporal revisada dentro de la red de ensayo o desde Cloud SQL Studio con una identidad DB ya aprobada. El bootstrap requiere dos credenciales distintas de Secret Manager: password del login runtime de la candidata y password del login temporal miembro únicamente de `espacigo_gadm_bootstrap`. Ambos secretos montados deben ser modo 0400. El login runtime y el login/rol de activación no han sido verificados ni preparados en la candidata.

## Preflight read-only

Guardar evidencia redacted con timestamp UTC, proyecto, instancia, base y SHA del SQL importado. No guardar DSN, passwords, tokens ni filas con identidad.

1. Confirmar por `gcloud sql instances describe` que el proyecto sea `espacigo-testprod`, la instancia sea `espacigo-gcp06-20261020-gcp06a-pg`, PostgreSQL 18, `RUNNABLE`, sin IP pública y con red privada esperada. Confirmar que la base sea `espacigo_gcp06_candidate_20261020-gcp06a`.
2. Confirmar la operación de importación ya reportada y metadatos del objeto existente (bucket, objeto, generación, bytes y SHA-256). No volver a subir, importar, sobrescribir ni borrar el objeto.
3. Con un observador preexistente de solo lectura, comprobar el ledger esperado V1–V64; conteos contra el manifiesto sanitizado; extensiones/versiones; owners; ACL de runtime; ausencia de membresías ADMIN inesperadas del bootstrap; y consultas de salud requeridas. Comparar agregados, no divulgar valores personales. La consulta observadora debe limitarse a `SELECT` y catálogos, sin `SET ROLE`, DDL ni DML.
4. Revisar el servicio API actual con `gcloud run services describe`: imagen/digest, revisión estable, variables `DATABASE_CONNECTION_MODE`, instancia, base y usuario DB, montaje de secreto, cuenta de servicio, VPC/subred/egress, min/max instances, ingress, tráfico y URL. Guardar una copia JSON redacted de la revisión estable como rollback.
5. Confirmar que los frontends de ensayo apuntan al API test esperado, y anotar sus URLs y revisión/digest actuales. No hacer deploy ni cambiar DNS/CORS/ingress en esta fase.
6. Calcular el diff Terraform revisado y una estimación mensual. La referencia de lista documentada para la instancia candidata encendida continuamente es ~US$67,51/mes (730 h; 1 vCPU, 3,75 GiB y 20 GiB SSD), sin red, respaldos, logs, impuestos ni moneda. Cloud Run ya existe; cualquier cambio de min instances, CPU/memoria o servicio debe reflejar su delta por separado con precios vigentes. No presentar este cálculo histórico como factura.

Si no existe una credencial observer previamente aprobada, detenerse aquí: no cambiar password de `postgres`, no habilitar autenticación IAM ni crear un usuario nuevo para desbloquear la lectura.

## Implementación local preparada en este corte

- `infra/terraform/environments/testprod/gcp` acepta `api_gcp06_candidate_run_id="20261020-gcp06a"` y una versión numérica precreada del secreto `espacigo-test-database-password-gcp06`. El data source fija proyecto, región, instancia, PostgreSQL 18 y la IP privada observada `10.189.0.13`. El API cambia a `espacigo_gcp06_candidate_20261020-gcp06a`; se conserva `enable_database=true` para mantener en estado la instancia histórica y evitar que el plan la retire. El acceso Secret Manager es solo para la identidad API.
- Los servicios web público/privado mantienen sus nombres, imágenes, URLs y configuración. El modo candidato solo compone el API; el plan de aplicación debe mostrar cambios únicamente al API y al contenedor/permisos del secreto candidate cuando correspondan. Cualquier cambio de frontend, instancia antigua, IAM ajeno al secreto, ingress, DNS o tráfico fuera del API cancela el apply y requiere revisar el diff.
- `cmd/gcp06adminbootstrap` y `Dockerfile.gcp06-admin-bootstrap` preparan un binario e imagen localmente compilable, pero no lo hacen ejecutable en Cloud por sí solos: este PR no crea Cloud Run Job, service account, mounts/IAM de secretos, ni los logins PostgreSQL necesarios. El CLI fija run/instancia/base y login efímero; lee dos secretos montados 0400, asume `espacigo_runtime` para localizar la cuenta y delega el bootstrap transaccional/auditado. La migración V65 habilita el actor de auditoría distinto `cli_gcp06` manteniendo `cli_local`. Hasta que V65 y los roles estén instalados/verificados, no ejecutar el binario. El CLI solo permite la cuenta activa/verificada del correo solicitado y una frase de confirmación exacta; no imprime correo/UUID, no crea la cuenta y no implementa reset masivo de contraseñas.
- Los secretos tienen propósitos independientes: (a) password SQL runtime de la DB candidata, (b) password SQL del login temporal de activación, y (c) password de aplicación que se quería compartir entre cuentas importadas. Este corte solo prepara lectura/montaje de (a) y (b); (c) no está disponible ni implementado. No reutilizar un valor entre estos usos.
- La opción de candidato queda apagada por default. El plan futuro debe usar estado Terraform aprobado y no puede cambiar `api_gcp06_candidate_run_id` a otro run. `api_gcp06_candidate_password_version` debe ser un número, nunca una carga de secreto en Terraform.

## Ejecución futura, sujeta a autorización y revisión independiente

Cada operación remota debe tener un diff exacto, actor, objetivo y rollback revisados antes de ejecutar. La secuencia propuesta es:

1. Completar la verificación read-only de SQL y habilitar el login runtime mínimo; cargar fuera de Terraform la versión específica del secreto DB de candidato y confirmar metadatos sin leer el valor. Sin esos gates, no activar el modo candidato.
2. Resolver el registro/verificación de la cuenta maestra. Preparar y verificar el login runtime y el login temporal restringido, aplicar V65, y definir en una revisión separada el Cloud Run Job, su identidad mínima y mounts 0400 de Secret Manager antes de habilitar su ejecución. Solo después ejecutar el CLI una vez; el singleton y auditoría transaccional bloquean otro bootstrap a un destino distinto. El acceso compartido de usuarios importados sigue bloqueado hasta que exista una contraseña autorizada Cloud y un reset con historial/auditoría revisado, que use `internal/adapters/password/bcrypt.Hasher` (bcrypt cost 12). No fabricar hashes en SQL.
3. Preparar un plan Terraform con el ID de run fijo, `enable_database=true`, `api_gcp06_candidate_password_version` numérico y los mismos digests/revisiones actuales de API/web. Guardar el plan; verificar que las únicas diferencias sean `google_cloud_run_v2_service.api` y el secret container/accessor candidate si aún no existen. Rechazar cualquier plan que cambie instancia antigua, web, ingress, IAM amplio, secrets existentes o tráfico frontend.
4. Tras aprobación humana del HEAD y del plan exacto, aplicar el plan aprobado. Mantener la revisión estable y configuración previa disponibles. La revisión API nueva debe verificarse antes de permitir 100% de tráfico.
5. Verificar autenticación, rol global exacto del correo confirmado, readiness, conexión a la base por el conector privado, endpoints test y recorridos de los frontends públicos/privados existentes. No redeplegar frontends si no cambian.

## Rollback

Antes de aplicar, capturar revisión/configuración estable de cada servicio y el mapeo de tráfico. Si falla cualquier gate, restaurar el tráfico del API a la revisión estable previa y la referencia de conexión/variables de esa revisión; restaurar tráfico/configuración frontend solo si el corte la modificó. No tocar ni borrar la instancia candidata ni la antigua durante rollback. Revocar la cuenta de ensayo por el flujo normal si se creó y conservar su evento de auditoría. No limpiar artefacto importado ni recursos persistentes sin autorización específica. Registrar el motivo y detenerse.

## Criterios para cerrar el corte

- Evidencia read-only identifica instancia/base y demuestra ledger, conteos, extensiones, ownership, ACL y runtime sin divulgar PII.
- La cuenta de ensayo se creó por flujo aprobado, se verificó, tiene solo el rol acordado y su secreto no aparece en artefactos compartidos.
- El API usa la base candidata con identidad runtime de privilegio mínimo; los dos frontends completan los recorridos de ensayo acordados.
- Se registran costo estimado revisado, revisiones/digests, HEAD, pruebas realmente ejecutadas, límites, rollback y aceptación pendiente del usuario.

Hasta entonces, GCP-06 queda parcial y el candidato no es backend habilitado de la aplicación. Este PR prepara el modo Terraform opt-in y la implementación local del CLI; no implementa ni autoriza un cutover, un bootstrap ejecutable Cloud o un Job Cloud Run. En particular, este PR no pone a disposición la clave compartida de usuarios importados: no hay un secreto Cloud-scoped aprobado ni una mutación auditada preparada para actualizar hashes.
