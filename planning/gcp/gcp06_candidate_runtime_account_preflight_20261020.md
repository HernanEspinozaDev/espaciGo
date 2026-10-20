# GCP-06 candidato: preflight de cuenta y runtime

**Estado:** plan de corte para revisión. No ejecuta ni autoriza cambios en Cloud.
**Alcance:** únicamente la instancia candidata GCP-06 y los servicios de ensayo `espacigo-test-api`, `espacigo-test-public` y `espacigo-test-private`. La instancia histórica `espacigo-testprod-pg`, producción y sus datos quedan fuera.

## Resultado buscado

Dejar una secuencia verificable para habilitar el candidato importado como backend del API de ensayo, crear la cuenta de ensayo solicitada por un flujo permitido, y comprobar los dos frontends contra el API. El orden de trabajo es: verificar el candidato y su esquema; acordar rol y mecanismo de cuenta; preparar permisos mínimos; revisar el diff de Terraform y el costo; ejecutar en una ventana autorizada; verificar API y frontends; y conservar una reversa exacta.

Este documento solo prepara ese corte. PR #459 sigue abierto y contiene el estado de importación y la propuesta de observación SQL; no se considera fusionado ni su propuesta se considera implementada. No se cambia tráfico durante este preflight.

## Bloqueos que requieren resolver antes de ejecutar

1. **Rol de aplicación:** `administrador` y `administrador_global` son roles distintos. El objetivo escrito aquí es exactamente `administrador`; no se concederá `administrador_global`. El CLI existente `gadm-admin activate-local` solo permite `espacigo_local` y concede `administrador_global`, por lo que no sirve para Cloud. Antes de crear una cuenta, el responsable del producto/revisor debe confirmar si basta el rol `administrador` y señalar un flujo de servidor existente, auditado y aprobado para asignarlo. No se harán escrituras SQL directas ni se ampliará el CLI local.
2. **Credencial:** la política privada disponible autoriza su contraseña guardada solo para ensayos locales. No se copiará ni reutilizará en Cloud. El registro Cloud requiere una credencial específica del entorno, entregada por un canal seguro y autorizada expresamente; no se incorporará a Git, argumentos, logs, PR ni capturas. No se usará una contraseña común para varias identidades. Si se busca la misma contraseña para todos los usuarios de ensayo, el flujo se detiene hasta una política de credenciales Cloud que lo autorice de forma segura.
3. **Importación y permisos:** PR #459 reporta la importación completada, pero faltan verificaciones SQL independientes de ledger, conteos, extensiones, ownership, ACL y permisos runtime. No conectar la app hasta que esa evidencia read-only pase para la instancia y base allowlisted.
4. **Mecanismo mínimo de acceso:** la base es privada y el host de desarrollo no tiene ruta VPC. La observación debe ejecutarse desde una sesión temporal revisada dentro de la red de ensayo o desde Cloud SQL Studio con una identidad DB ya aprobada. No crear Cloud Run Job, usuario DB, rol, grant, binding IAM ni secreto como parte del preflight.

## Preflight read-only

Guardar evidencia redacted con timestamp UTC, proyecto, instancia, base y SHA del SQL importado. No guardar DSN, passwords, tokens ni filas con identidad.

1. Confirmar por `gcloud sql instances describe` que el proyecto sea `espacigo-testprod`, la instancia sea `espacigo-gcp06-20261020-gcp06a-pg`, PostgreSQL 18, `RUNNABLE`, sin IP pública y con red privada esperada. Confirmar que la base sea `espacigo_gcp06_candidate_20261020-gcp06a`.
2. Confirmar la operación de importación ya reportada y metadatos del objeto existente (bucket, objeto, generación, bytes y SHA-256). No volver a subir, importar, sobrescribir ni borrar el objeto.
3. Con un observador preexistente de solo lectura, comprobar el ledger esperado V1–V64; conteos contra el manifiesto sanitizado; extensiones/versiones; owners; ACL de runtime; ausencia de membresías ADMIN inesperadas del bootstrap; y consultas de salud requeridas. Comparar agregados, no divulgar valores personales. La consulta observadora debe limitarse a `SELECT` y catálogos, sin `SET ROLE`, DDL ni DML.
4. Revisar el servicio API actual con `gcloud run services describe`: imagen/digest, revisión estable, variables `DATABASE_CONNECTION_MODE`, instancia, base y usuario DB, montaje de secreto, cuenta de servicio, VPC/subred/egress, min/max instances, ingress, tráfico y URL. Guardar una copia JSON redacted de la revisión estable como rollback.
5. Confirmar que los frontends de ensayo apuntan al API test esperado, y anotar sus URLs y revisión/digest actuales. No hacer deploy ni cambiar DNS/CORS/ingress en esta fase.
6. Calcular el diff Terraform revisado y una estimación mensual. La referencia de lista documentada para la instancia candidata encendida continuamente es ~US$67,51/mes (730 h; 1 vCPU, 3,75 GiB y 20 GiB SSD), sin red, respaldos, logs, impuestos ni moneda. Cloud Run ya existe; cualquier cambio de min instances, CPU/memoria o servicio debe reflejar su delta por separado con precios vigentes. No presentar este cálculo histórico como factura.

Si no existe una credencial observer previamente aprobada, detenerse aquí: no cambiar password de `postgres`, no habilitar autenticación IAM ni crear un usuario nuevo para desbloquear la lectura.

## Ejecución futura, sujeta a autorización y revisión independiente

Cada operación remota debe tener un diff exacto, actor, objetivo y rollback revisados antes de ejecutar. La secuencia propuesta es:

1. Resolver los bloqueos de rol y credencial anteriores. Crear/iniciar la cuenta de ensayo con registro/verificación normal en el entorno de ensayo. Usar el hasher canónico `internal/adapters/password/bcrypt.Hasher` (bcrypt cost 12); nunca fabricar hashes en SQL ni aceptar hashes importados. Aplicar `administrador` únicamente con el flujo de servidor aprobado, motivo/auditoría y verificación de estado/correo; no conceder `administrador_global`.
2. Acordar y revisar un contrato runtime de PostgreSQL de mínimo privilegio compatible con V60/V64. El API debe seguir usando su login runtime propio (`espacigo_api_login`) con solo los grants requeridos; el observador SQL no es el runtime. No usar `postgres`, `cloudsqlsuperuser`, `CREATEROLE` ni credenciales de migración para runtime. El bloqueo V60 por membresía ADMIN automática debe resolverse en un diseño aparte aprobado; no omitir ni debilitar sus checks.
3. Preparar un plan Terraform que modifique únicamente los tres servicios `espacigo-test-*` necesarios y la referencia a la base candidata. Mostrar plan, sustituciones, variables/secret references y el rollback. No cambiar la instancia antigua, ingress público, IAM global, versiones de secretos, políticas de acceso ni tráfico de frontends salvo que una diferencia concreta lo exija y reciba autorización propia.
4. Aplicar solo el plan aprobado. Primero crear una revisión nueva del API con tráfico cero o canary seguro; verificar readiness, conexión DB y endpoints de ensayo; luego dirigir tráfico del API. Mantener revisión estable y configuración previa disponibles. Los frontends se despliegan solo si sus imágenes/configuración actuales no apuntan al API correcto.
5. Verificar autenticación de la cuenta creada, rol exacto, endpoints públicos y privados previstos, health/readiness, recorridos existentes de ambos frontends y ausencia de datos fuera del fixture. Registrar SHA de imagen, revisiones Cloud Run, HEAD y resultados con datos redacted.

## Rollback

Antes de aplicar, capturar revisión/configuración estable de cada servicio y el mapeo de tráfico. Si falla cualquier gate, restaurar el tráfico del API a la revisión estable previa y la referencia de conexión/variables de esa revisión; restaurar tráfico/configuración frontend solo si el corte la modificó. No tocar ni borrar la instancia candidata ni la antigua durante rollback. Revocar la cuenta de ensayo por el flujo normal si se creó y conservar su evento de auditoría. No limpiar artefacto importado ni recursos persistentes sin autorización específica. Registrar el motivo y detenerse.

## Criterios para cerrar el corte

- Evidencia read-only identifica instancia/base y demuestra ledger, conteos, extensiones, ownership, ACL y runtime sin divulgar PII.
- La cuenta de ensayo se creó por flujo aprobado, se verificó, tiene solo el rol acordado y su secreto no aparece en artefactos compartidos.
- El API usa la base candidata con identidad runtime de privilegio mínimo; los dos frontends completan los recorridos de ensayo acordados.
- Se registran costo estimado revisado, revisiones/digests, HEAD, pruebas realmente ejecutadas, límites, rollback y aceptación pendiente del usuario.

Hasta entonces, GCP-06 queda parcial y el candidato no es backend habilitado de la aplicación.
