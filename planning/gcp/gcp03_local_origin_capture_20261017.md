# GCP-03 — implementación local de captura del origen

Fecha: 2026-10-17. La herramienta se fusionó en PR #387 (`34cf8299ab18760c29d57419ad825af188bfdfbe`). El estado de preparación descrito originalmente se conserva como historia; el resultado de los preflights posteriores queda registrado a continuación. GCP-03 sigue parcial; GCP-04 no se inicia.

## Actualización del preflight read-only (2026-10-17)

El primer intento sobre el origen se detuvo antes de SQL porque `docker image inspect` no encontró localmente la imagen-ID referenciada por el contenedor de PostgreSQL (`sha256:84b8ed430e1aa7847ef5777ae9b65db374da288dd71e51714fb6ad86db550053`). El tag local `espacigo-local-database:dev` resuelve a otro digest (`sha256:9c63ab38222eaf32b937c40e95ae35abada3d8dd924386ce210b3de0746e3c70`); no se afirmó que fueran iguales ni se intentó pull/rebuild/export/commit.

La corrección propuesta en esta rama captura `missing_locally` como brecha explícita de procedencia, mantiene la correlación de versión desde el binario `postgres --version` del contenedor activo, V1–V61 desde `/migrations` del backend ejecutándose, conexión/configuración efectiva y mounts desde `docker inspect`, y separa el destino de restore: debe seleccionarse por `repository@sha256` y validarse localmente. `restore-copy` arranca solo esa imagen digest-pinned en `network none`/tmpfs, verifica PostgreSQL major y que cada extensión/version exacta del origen esté disponible antes del restore, y después compara esquema/datos/owners/ACL. El ID de imagen origen no se usa como imagen destino.

Con la corrección en el worktree, el preflight actual avanzó hasta la correlación de sesiones read-only y se detuvo: PostgreSQL informa nueve sesiones del rol runtime sin `client_addr`, por lo que no se puede probar que correspondan a la IP del backend. Una llamada HTTP GET al catálogo público devolvió 200, pero no resolvió esa correlación. No se creó candidato, no hubo dump/copias de datos/llave ni pausa/reinicio; la consulta fue de solo lectura. El detalle saneado está en `.local/testing/gcp03-preflight-image-provenance-20261017/status.json` (modo 0600). No se relaja el gate de sesión o clientes no identificados. Hasta probar la ruta con una fuente de lectura observable, captura permanece bloqueada.

La corrección tampoco afirma control exclusivo de Docker ni ausencia de ingress externo: el candidato mantiene esas decisiones en `false` y `external_ingress_and_writer_sources: null` hasta que una revisión las complete. La instancia de restore propuesta por digest no demuestra procedencia de la imagen del origen ni compatibilidad de snapshot real.

## Alcance de este corte

El corte agrega una herramienta local con gate de revisión para correlacionar el origen, capturar una copia cifrada durante una ventana autorizada y validar una restauración aislada y sanitizada. El código solo ejecuta la captura cuando un registro de release de un run concreto está aprobado por `HernanEspinozaDev`, ligado al HEAD exacto, al fingerprint actual del origen y a la lista completa de contenedores que se van a detener. Este PR no autoriza el uso de ese gate.

No se ejecuta `preflight`, `release-candidate`, `capture`, `restore-copy` ni `recover` contra el origen en esta implementación. Tampoco se genera dump, copia de payloads/llaves, salida bajo `pilot01`, cambio de infraestructura, Cloud SQL, importación ni job cloud. La inspección de este corte se limita a código, fixtures desechables y un PostgreSQL local creado sin red, puertos ni volumen Docker.

## Preflight reproducible para una futura ventana

Ejecutar solo después de fusionar este PR y obtener una ventana específica. Elegir un `run-id` nuevo, aleatorio, minúsculo y con guiones (6–64 caracteres); no reutilizarlo.

```bash
RUN_ID=gcp03-<identificador-nuevo>
python3 scripts/gcp03_origin_capture.py preflight \
  --private-output-dir ".local/testing/gcp03-capture-preflight-$RUN_ID"
```

El comando inspecciona IDs/digests de las imágenes en ejecución, configuración efectiva no secreta del backend, identidad/conexión del DB, mounts y nombre/id del volumen, migraciones que contiene la imagen activa y checksums V1–V61, agregados de catálogo, sesiones/leases, roles/owners/ACL y huellas de los dos árboles privados. SQL se ejecuta `READ ONLY` dentro de una transacción `REPEATABLE READ`. El preflight correlaciona el alias/host con una red Docker compartida, la sesión de PostgreSQL con IP/rol/base del backend y la migración aplicada con la imagen activa; no supone que los roots Compose sean iguales. Para la llave contractual registra solo metadatos de archivo (tamaño/modo/inode/mtime); su contenido se valida y cifra separadamente después del quiesce. El JSON privado y sus directorios son `0600`/`0700`; stdout solo da resumen sin DSN, contraseñas, rutas de secretos ni filas personales.

Crear un candidato no aprobado, sin detener servicios. Antes debe seleccionarse una imagen local de restore por digest inmutable (distinta de la imagen-ID observada del origen si esta no existe localmente):

```bash
python3 scripts/gcp03_origin_capture.py release-candidate \
  --run-id "$RUN_ID" \
  --restore-image-ref "$RESTORE_IMAGE_REF" \
  --private-output-dir ".local/testing/gcp03-capture-release-$RUN_ID"
```

El candidato registra imagen source/runtime y sus límites de procedencia por separado del destino `restore_target`. El comando rechaza tags mutables, digest no presente localmente o errores del daemon que no sean explícitamente `No such image`. `external_ingress_and_writer_sources` queda sin evaluar (`null`); no se deriva su ausencia del inventario Docker.

Revisar de forma humana cada contenedor identificado y su rol, puertos publicados, consumidores de DB, schedulers, procesos fuera de Docker y cualquier ingress externo. El candidato no es autorización. Si algún writer/ingress queda sin identificar, si hay clientes DB desconocidos, si hay servicios externos no controlables durante la ventana, o si la raíz privada no está montada con permisos 0700/0600, detener el procedimiento sin capturar.

La aprobación de una ventana separada debe completar `release-candidate.json` como un archivo nuevo, modo 0600, nunca sobrescribirlo en el sitio. El registro debe indicar `status=approved`, `reviewed_by=HernanEspinozaDev`, HEAD completo que revisó, run-id, `window_start_utc`/`window_end_utc` en ISO-8601 UTC (máximo cuatro horas), `source_change_window_authorized=true`, `all_ingress_and_writer_sources_enumerated=true`, `exclusive_docker_control=true`, `external_ingress_and_writer_sources=[]`, fingerprint/ID de DB intactos y el orden completo exacto de quiesce. La CLI solo permite ejecutar dentro de esa ventana. Rechaza un HEAD, fingerprint, contenedor, conjunto o run-id distinto y prohíbe incluir el DB. Si existe ingress/writer externo, el capturador no puede detenerlo y rechaza el release. También aborta si PostgreSQL publica puertos al host.

## Ventana: límites, captura y recuperación

Comando para ejecutar solo con release exacto, revisado y una ventana concedida por el dueño del origen:

```bash
python3 scripts/gcp03_origin_capture.py capture \
  --release-file ".local/testing/gcp03-capture-release-$RUN_ID/release-approved.json" \
  --run-id "$RUN_ID" --confirm-capture-run-id "$RUN_ID"
```

La herramienta vuelve a hacer el preflight y exige el mismo fingerprint, DB container y volumen. Detiene por ID solo todos los contenedores activos incluidos en el release: primero los que publican puertos (ingress) y luego los demás writers/workers; no detiene ni recrea PostgreSQL, no desmonta ni modifica el volumen. Espera hasta el límite aprobado del perfil por cero conexiones de clientes a la base y cero leases de outbox/avisos. Clientes o leases restantes abortan. El dump y los árboles privados se toman mientras los escritores siguen detenidos. Un journal privado persiste cada intención antes de cada stop. Para resolverla, combina acuse del stop, evento exacto de Docker e inspección del estado y comprueba readiness. Si el servicio sigue detenido sin prueba de que este run lo paró, o no se puede establecer su estado, conserva `resume_incomplete`, no lo inicia a ciegas y no sella la captura. Un servicio ya en ejecución se verifica sin reiniciarlo. La recuperación reevalúa cada intent pendiente y solo se declara completa cuando todos están resueltos:

```bash
python3 scripts/gcp03_origin_capture.py recover \
  --journal ".local/testing/gcp03-origin-journals/$RUN_ID.journal.json"
```

La DB permanece ejecutándose durante la captura. La consistencia depende de completar el quiesce: no aceptar captura si el preflight no pudo enumerar los clientes activos, si quedan conexiones o leases, o si una fuente nueva de escritura puede entrar en la ventana. El quiesce detiene servicios del mismo proyecto Compose o de las redes Docker de DB/backend. Al reanudar, usa healthcheck Docker si existe; para API sin healthcheck ejecuta `/api check` dentro del contenedor y para Mailpit valida HTTP `/api/v1/info` en su puerto publicado. Un servicio sin healthcheck ni sonda implementada aborta la recuperación en estado incompleto. No puede apagar componentes externos, por lo que exige que no existan y que se enumeren cero en el release. El script nunca incluye la ruta de `docker compose down`, `docker volume rm`, `docker rm` del DB ni comandos de borrado de volumen.

El registro de release incluye `timeout_profile`, que el revisor configura dentro de límites duros y antes de autorizar la ventana. El perfil inicial acotado para el origen pequeño usa hasta 35 s por stop (15 s adicionales de margen del CLI), 180 s para drenar, 15 min para dump, 10 min para árboles privados, 60 s para llave, 120 s para SQL, 5 min para cifrar metadata y hasta 60 s de inicio + 5 min de readiness por servicio. Eso reserva 41 min fijos más 6 min 50 s por servicio; el perfil conserva topes absolutos de 60 min para dump y 30 min para árboles, por lo que la envolvente máxima total es 106 min más 6 min 50 s por servicio. Son presupuestos máximos por etapa, no una estimación del tiempo esperado para un dump histórico de 74 MB. La ventana debe cubrir la envolvente del perfil exacto antes de detener el primer servicio y no excede cuatro horas. Al agotar un límite la fase falla, intenta recuperar servicios con los mismos topes y no amplía pausas o tiempos silenciosamente. Captura rechaza destino con menos de `max(128 MiB, 2 × bytes estimados + 128 MiB)` libres.

## Artefactos y custodia

- PostgreSQL se serializa como `pg_dump --format=custom --create`; las ACL/owners de la base están dentro del dump. Roles globales, membresías y ACL/owners de catálogo se registran aparte sin consultar ni incluir hashes globales `rolpassword`. El contenido de tablas puede incluir hashes de credenciales de aplicación y datos personales; el dump completo se cifra y no se presenta como anonimizado.
- La llave contractual local se cifra como artefacto separado. GnuPG aplica AES-256 con autenticación AEAD y passphrase derivada por HMAC distinta para cada artefacto. La llave maestra aleatoria de recuperación está fuera del snapshot, bajo `~/.local/share/espacigo/gcp03-origin-custody/<run-id>/recovery.key`, modo 0600. Su copia de recuperación debe mantenerse fuera del bundle y bajo custodia del dueño; perderla impide descifrar.
- No se incluyen archivos secretos montados, secretos de DB, credenciales de conexión ni el volumen PostgreSQL. Los árboles privados permitidos son únicamente `private-evidence` y `privacy-replay`, preservados con rutas relativas, modos, tamaño y SHA-256. El recorrido rechaza symlinks, special files, cambios desde el inventario y escapes de raíz.
- `.partial` se crea con exclusividad; un run-id existente nunca se sobrescribe. Tras sellar, el bundle y sus archivos quedan en modo solo lectura para el propietario (`0500`/`0400`) y contienen solo artefactos cifrados, sello y SHA-256. Se verifican checksum, modo, autenticación, manifiesto, hashes/bytes y migraciones. Los modos no son WORM: un propietario puede cambiarlos, pero el validador detecta diferencias del sello/artefactos. Si falla una fase, los artefactos parciales quedan en almacenamiento privado para investigación; no son un snapshot utilizable.
- Para recuperar la llave contractual como archivo privado nuevo, solo después de un snapshot y una decisión específica:

```bash
python3 scripts/gcp03_origin_capture.py recover-key \
  --snapshot ".local/testing/gcp03-origin-snapshots/$RUN_ID" \
  --recovery-key "$HOME/.local/share/espacigo/gcp03-origin-custody/$RUN_ID/recovery.key" \
  --output ".local/testing/gcp03-recovered-contract-key-$RUN_ID"
```

## Restore local copy-only

No arranca la aplicación, workers ni proveedores. El comando requiere el `repository@sha256` exacto registrado en el snapshot/release y presente localmente; no hace pull ni exige que sea la imagen-ID del origen. Docker usa `--network none`, filesystem raíz de solo lectura, almacenamiento de base en `tmpfs`, sin puertos ni mounts de volumen. Genera passwords sintéticos efímeros y roles sin hashes de contraseña. Antes del restore comprueba PostgreSQL major y disponibilidad de cada versión exacta de extensión; después compara versión/extensiones instaladas, V1–V61, tipos UUID/int64, owners/ACL, SECURITY DEFINER, estados y conteos. Luego, y solo en la copia, revoca sesiones/tokens, cancela los avisos pendientes/procesando, limpia leases y asigna `NOLOGIN` al runtime. El outbox de identidad queda preservado pero no ejecutable: no se inicia ningún sender.

```bash
python3 scripts/gcp03_origin_capture.py restore-copy \
  --snapshot ".local/testing/gcp03-origin-snapshots/$RUN_ID" \
  --recovery-key "$HOME/.local/share/espacigo/gcp03-origin-custody/$RUN_ID/recovery.key" \
  --run-id "$RUN_ID" \
  --restore-image-ref "$RESTORE_IMAGE_REF"
```

La prueba de extremo a extremo crea un PostgreSQL 18 desechable y un esquema mínimo sintético, genera el bundle cifrado y ejecuta el CLI `restore-copy` completo. Comprueba la restauración, comparación de esquema/datos/ACL de ese fixture, cuarentena, red `none`, tmpfs y retirada del contenedor. No es un restore del esquema real ni verifica compatibilidad Cloud SQL. Los 958 avisos pendientes del inventario histórico se deben cancelar únicamente en una copia. Capturar o restaurar datos del origen, liberar una llave o importar datos hacia cualquier destino requiere autorización separada; el presente PR no la implica.

## Verificación realizada en este corte

- `python3 -m py_compile scripts/gcp03_origin_capture.py scripts/test_gcp03_origin_capture.py` — correcto.
- `python3 -m unittest test_gcp03_origin_capture -v` — 30 tests correctos en esta rama; incluye fixtures para image-ID ausente/tag digest diferente, errores del daemon, candidatos sin attestation de ingress, direcciones PostgreSQL IPv4-mapped y versión del binario runtime.
- El test `restore-copy` pasó de punta a punta con PostgreSQL 18 descartable: generó dump sintético cifrado, ejecutó el CLI, restauró y comparó catálogo/datos/ACL, cuarentenó un aviso pendiente, verificó red `none` y tmpfs; retiró el contenedor y su evidencia.
- PostgreSQL 18 Alpine desechable, sin red, puertos ni volumen — `SQL_CAPTURE` produjo el JSON esperado; `role_reconciliation_sql` fue aceptado y cuarentena devolvió 0 sesiones/tokens/avisos pendientes/leases, 1 aviso cuarentenado y runtime `NOLOGIN`; quedó 1 evento de identidad no entregado preservado y sin lease.
- Imagen local de base, sin red/puertos/volúmenes persistentes, raíz read-only y secretos sintéticos por bind read-only — arranque/readiness `pg_isready` exitoso. El contenedor se retiró.
- `git diff --check` — correcto en el diff final.
- No se ejecutó contra DB/origen, no se detuvo/reinició ningún servicio real, no se hizo dump/copia, no se accedió a GCP y no se modificaron V1–V61.
