# GCP-04-W7 B1 — implementación local de observación

**Estado:** implementación local propuesta para revisión. El modo predeterminado es `plan`, offline y sin mutaciones. Este corte no autoriza ni ejecuta llamadas Cloud, SQL remoto, creación/lectura de secretos, IAM, imágenes, Terraform ni Jobs.

## Alcance

La ruta B1 usa un login builtin temporal nuevo, separado de `espacigo_gcp04_w7_observer` y de `postgres`. La solicitud de alta fija explícitamente `databaseRoles=["cloudsqlsuperuser"]`; el código rechaza rol omitido, distinto o fallback. El atributo PostgreSQL `SUPERUSER` se exige falso en la observación. `CREATEDB`, `CREATEROLE` e `INHERIT` se reportan como observados y no se normalizan. El resultado declara que B1 no acredita compatibilidad del observer.

El SQL fijo consulta únicamente catálogos: identidad de sesión, atributos propios, membresías directas y su cierre recursivo, owners, ACL directas/PUBLIC y capacidades efectivas de grant option para base/schema/ledger. El reporte mantiene por separado ACL directa, propietario literal y capacidad efectiva heredada; por eso puede mostrar direct grant=false y effective grant=true por la membresía `cloudsqlsuperuser`. No lee filas de negocio, secretos, settings ni estadísticas; ejecuta un único SELECT dentro de transacción de solo lectura. El hash aceptado está fijado en código y todo cambio de bytes invalida el ejecutor.

## Gating y límites

- La CLI inicia en `--mode=plan`; esa ruta no construye clientes ni consulta ADC.
- Operaciones requieren release JSON exacto, privado (`0600`), digest confirmado en CLI, etapa coincidente, responsable revisor `HernanEspinozaDev`, target/login/rol/hash fijados, una ejecución y expiración máxima de 30 minutos. Crear y limpiar exigen además sus flags de etapa.
- Cada etapa limita su contexto por el menor entre deadline del padre, `expires_at` del release y `max_duration_seconds`; se vuelve a comprobar antes de cada mutación. Cada etapa usa lease local exclusivo; una lease vencida no se roba automáticamente. Resultado API incierto requiere reconciliación antes de reintentar.
- Crear hace una sola alta de secreto, una sola versión y una sola inserción de usuario. El payload se genera en memoria; no va en argumentos, salida, journal ni logs. El journal privado registra solo metadatos saneados.
- Probe requiere archivo de credencial protegido `0600`, identidad/base/IP privada exactas, una consulta fijada y deadline SQL de 120 segundos. No habilita escritura ni acredita el contrato observer.
- El journal común del recurso registra el `CREATE_USER` que este proceso inició, con ID, tipo, instancia objetivo, login solicitado, release y ETags. Si `WaitOperation`, inventario o el guardado final falla después de `user_operation_pending`, `--mode=reconcile` requiere una liberación `b1_reconcile`, el ID exacto recuperable del journal, operación fresca `CREATE_USER/DONE` sin errores e inventario fresco exacto; agrega `user_create_reconciled` como prueba distinta y nunca inventa `create_complete`, reinserta o borra. Si falta el ID o la prueba pendiente, se detiene y exige evidencia externa. Limpieza admite esa prueba reconciliada o `create_complete`, vuelve a leer la operación en Admin API antes de borrar y también verifica `DELETE_USER/DONE` y ausencia. Una colisión 409 o login de mismo nombre/rol sin esa prueba no se adopta.
- Limpieza es una etapa separada: `users.delete` exacto una vez, espera de operación DONE y ausencia comprobada; después destruye solo la versión fijada por ETag, confirma `DESTROYED` y elimina únicamente el shell con etiquetas y ETag propios. Puede reconciliar versiones `ENABLED`, `DISABLED`, `DESTROYED`, shell vacío o ausente usando release y prueba del journal con ETags; nunca repite la destrucción de una versión ya `DESTROYED`.
- `--mode=cleanup-secret` es un release separado (`b1_cleanup_secret`) para shell/versión de este run cuando `users.list` fresco confirma que el login bootstrap no existe. Acepta solo los dos nombres de recurso fijados (número de proyecto o project ID), prueba `secret_shell_created` y ETags exactos. Para una versión separa el ETag original de creación del vigente tras `DESTROYED`, de modo que reanuda después de fallar el borrado del shell sin repetir `destroy`. Acepta fallas anteriores a la inserción o un rechazo HTTP definitivo. Una inserción de resultado incierto bloquea esta ruta aunque el usuario esté momentáneamente ausente. Este modo nunca llama `users.delete`.
- La reconciliación rechaza colisiones, recursos ajenos, versiones extra o inventario incompleto. No adopta ni modifica `postgres` ni el observer.

## Preflight antes de cualquier uso futuro

El bundle local fija proyecto `espacigo-testprod`, instancia `espacigo-testprod-pg`, PostgreSQL 18, región `southamerica-west1`, conexión `espacigo-testprod:southamerica-west1:espacigo-testprod-pg`, IP privada `10.189.0.3`, base `espacigo`, login `espacigo_gcp04_w7_bootstrap`, rol de alta `cloudsqlsuperuser` y secreto `espacigo-gcp04-w7-b1-bootstrap-credential` con réplica administrada únicamente en `southamerica-west1`.

Antes de una eventual ejecución se necesita una liberación por etapa, inventario metadata exacto, revisión de la cuenta activa de publicación, presupuesto aprobado y plan de compensación. Este PR no fija ni publica digest de imagen/binario remoto, no prepara IAM y no define una concesión de red. Cualquier contenedor/Job futuro deberá aprobar imagen por digest, identidad runtime, ingress/red privada, CPU/memoria, timeout, concurrencia 1 y reintentos 0 en un cambio revisado separado. No hay Job ejecutable en este corte.

## Costo y duración

Las llamadas habilitadas por el código son acotadas por release y contexto, con un máximo de 1,800 segundos por etapa; la consulta SQL tiene tope de 120 segundos. No se modifica la instancia Cloud SQL ni se afirma TTL automático de usuario o secreto. No se publica una cifra de precio: el costo marginal depende de tarifas/cuotas vigentes, llamadas efectivas, retención de Secret Manager y cualquier infraestructura futura, que este corte no crea. Una ejecución futura debe adjuntar cálculo vigente y presupuesto al release antes de activar una etapa.

## Evidencia local y límites

Las pruebas unitarias usan mocks HTTP locales para confirmar requests explícitos, no reintento, ETags, expiración y propiedad por operación/ID/tipo/target. Las pruebas de cleanup cubren colisión de mismo nombre y rol, `ENABLED`/`DISABLED`/`DESTROYED`, shell vacío/ausente y reanudación sin repetir destroy ni borrar sin prueba. `scripts/test-gcp04-w7-b1-probe-postgres.sh` usa PostgreSQL 18 local con red deshabilitada y almacenamiento temporal; el rol fixture `cloudsqlsuperuser` es `NOLOGIN` y no tiene atributo nativo `SUPERUSER`. El harness prueba permisos heredados con `INHERIT` y los contrasta con `NOINHERIT`, separando ACL directa de capacidad efectiva. Estas pruebas comprueban SQL y lógica del paquete local. No prueban catálogo, endpoint, credenciales, IAM, Secret Manager, costo ni comportamiento Cloud SQL real. No hay evidencia de ejecución Cloud en este corte.

## Archivos implementados

- `cmd/gcp04w7b1`: CLI offline por defecto y adaptadores SQL Admin/Secret Manager estrictamente acotados.
- `internal/gcp04w7b1`: releases, leases, journal privado, reconciliación, creación/probe/cleanup y matriz de observación.
- `db/query/gcp04_w7_b1_actor_probe.sql`: consulta única fijada por SHA-256.
- `scripts/test-gcp04-w7-b1-probe-postgres.sh`: harness local PostgreSQL 18 sin red.
