# GCP-03 — implementación local de captura del origen

Fecha: 2026-10-17. Estado: herramienta y protocolo preparados para revisión; no se ejecutó preflight, quiesce, dump, copia de archivos ni lectura adicional del origen. No se creó un release aprobado, no se detuvo ningún servicio y no hubo operación GCP. GCP-03 sigue parcial; GCP-04 no se inicia.

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

Crear un candidato no aprobado, sin detener servicios:

```bash
python3 scripts/gcp03_origin_capture.py release-candidate \
  --run-id "$RUN_ID" \
  --private-output-dir ".local/testing/gcp03-capture-release-$RUN_ID"
```

Revisar de forma humana cada contenedor identificado y su rol, puertos publicados, consumidores de DB, schedulers, procesos fuera de Docker y cualquier ingress externo. El candidato no es autorización. Si algún writer/ingress queda sin identificar, si hay clientes DB desconocidos, si hay servicios externos no controlables durante la ventana, o si la raíz privada no está montada con permisos 0700/0600, detener el procedimiento sin capturar.

La aprobación de una ventana separada debe completar `release-candidate.json` como un archivo nuevo, modo 0600, nunca sobrescribirlo en el sitio. El registro debe indicar `status=approved`, `reviewed_by=HernanEspinozaDev`, HEAD completo que revisó, run-id, `window_start_utc`/`window_end_utc` en ISO-8601 UTC (máximo cuatro horas), `source_change_window_authorized=true`, `all_ingress_and_writer_sources_enumerated=true`, `exclusive_docker_control=true`, `external_ingress_and_writer_sources=[]`, fingerprint/ID de DB intactos y el orden completo exacto de quiesce. La CLI solo permite ejecutar dentro de esa ventana. Rechaza un HEAD, fingerprint, contenedor, conjunto o run-id distinto y prohíbe incluir el DB. Si existe ingress/writer externo, el capturador no puede detenerlo y rechaza el release. También aborta si PostgreSQL publica puertos al host.

## Ventana: límites, captura y recuperación

Comando para ejecutar solo con release exacto, revisado y una ventana concedida por el dueño del origen:

```bash
python3 scripts/gcp03_origin_capture.py capture \
  --release-file ".local/testing/gcp03-capture-release-$RUN_ID/release-approved.json" \
  --run-id "$RUN_ID" --confirm-capture-run-id "$RUN_ID"
```

La herramienta vuelve a hacer el preflight y exige el mismo fingerprint, DB container y volumen. Detiene por ID solo todos los contenedores activos incluidos en el release: primero los que publican puertos (ingress) y luego los demás writers/workers; no detiene ni recrea PostgreSQL, no desmonta ni modifica el volumen. Espera hasta 180 s por cero conexiones de clientes a la base y cero leases de outbox/avisos. Clientes o leases restantes abortan. El dump y los árboles privados se toman mientras los escritores siguen detenidos. Al finalizar o fallar, se reanudan únicamente los contenedores cuyo stop quedó confirmado, en orden inverso, y se verifica el estado saludable previo; no se inicia un contenedor que no haya detenido este run. Un journal privado persiste la intención antes de cada stop. Ante cierre abrupto, el comando de recuperación consulta eventos de Docker y solo reanuda IDs confirmados por el journal:

```bash
python3 scripts/gcp03_origin_capture.py recover \
  --journal ".local/testing/gcp03-origin-journals/$RUN_ID.journal.json"
```

La DB permanece ejecutándose durante la captura. La consistencia depende de completar el quiesce: no aceptar captura si el preflight no pudo enumerar los clientes activos, si quedan conexiones o leases, o si una fuente nueva de escritura puede entrar en la ventana. El quiesce detiene servicios del mismo proyecto Compose o de las redes Docker de DB/backend. Al reanudar, usa healthcheck Docker si existe; para API sin healthcheck ejecuta `/api check` dentro del contenedor y para Mailpit valida HTTP `/api/v1/info` en su puerto publicado. Un servicio sin healthcheck ni sonda implementada aborta la recuperación en estado incompleto. No puede apagar componentes externos, por lo que exige que no existan y que se enumeren cero en el release. El script nunca incluye la ruta de `docker compose down`, `docker volume rm`, `docker rm` del DB ni comandos de borrado de volumen.

Tiempo máximo configurado (sin contar preflight ni esperas humanas): 35 s por stop (15 s adicionales de margen del CLI), hasta 180 s para drenar, 60 min para dump, 30 min para los árboles privados, 60 s para cifrar la llave, 120 s para el reporte SQL final, 5 min para cifrar metadata y hasta 60 s de inicio + 5 min de readiness por servicio restaurado. La herramienta exige que quede la envolvente completa calculada para el número de servicios más cinco minutos de margen, antes de detener el primero. La ventana no excede cuatro horas. Captura rechaza destino con menos de `max(128 MiB, 2 × bytes estimados + 128 MiB)` libres.

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

No arranca la aplicación, workers ni proveedores. El comando requiere la imagen exacta de base del snapshot ya presente localmente; no hace pull. Docker usa `--network none`, filesystem raíz de solo lectura, almacenamiento de base en `tmpfs`, sin puertos ni mounts de volumen. Genera passwords sintéticos efímeros y roles sin hashes de contraseña. Compara versión/extensiones, V1–V61, tipos UUID/int64, owners/ACL, SECURITY DEFINER, estados y conteos. Luego, y solo en la copia, revoca sesiones/tokens, cancela los avisos pendientes/procesando, limpia leases y asigna `NOLOGIN` al runtime. El outbox de identidad queda preservado pero no ejecutable: no se inicia ningún sender.

```bash
python3 scripts/gcp03_origin_capture.py restore-copy \
  --snapshot ".local/testing/gcp03-origin-snapshots/$RUN_ID" \
  --recovery-key "$HOME/.local/share/espacigo/gcp03-origin-custody/$RUN_ID/recovery.key" \
  --run-id "$RUN_ID"
```

La prueba de fixture actual comprueba el SQL de reconciliación de roles y la cuarentena en PostgreSQL 18 local con `--network none`, `tmpfs` y sin volúmenes; no es un restore del esquema real ni verifica compatibilidad Cloud SQL. Los 958 avisos pendientes del inventario histórico se deben cancelar únicamente en una copia. Capturar o restaurar datos del origen, liberar una llave o importar datos hacia cualquier destino requiere autorización separada; el presente PR no la implica.

## Verificación realizada en este corte

- `python3 -m py_compile scripts/gcp03_origin_capture.py scripts/test_gcp03_origin_capture.py` — correcto.
- `python3 scripts/test_gcp03_origin_capture.py` — 17 fixtures correctos: bundle cifrado/custodia externa, artefacto corrupto, dump fallido con reanudación, árbol privado alterado con reanudación, interrupción, escape/symlink, journal que intenta incluir DB, recuperación de stop confirmado, finalización de snapshot sellado luego de recuperar, lista desconocida/DB, exclusividad de Docker, presupuesto y zona UTC de ventana, estabilidad de catálogo/llave, drift de expiración y rol builtin.
- PostgreSQL 18 Alpine desechable, sin red, puertos ni volumen — `SQL_CAPTURE` produjo el JSON esperado; `role_reconciliation_sql` fue aceptado y cuarentena devolvió 0 sesiones/tokens/avisos pendientes/leases, 1 aviso cuarentenado y runtime `NOLOGIN`; quedó 1 evento de identidad no entregado preservado y sin lease.
- Imagen local de base, sin red/puertos/volúmenes persistentes, raíz read-only y secretos sintéticos por bind read-only — arranque/readiness `pg_isready` exitoso. El contenedor se retiró.
- `git diff --check` — correcto en archivos tracked; volver a ejecutar después de agregar los documentos.
- No se ejecutó contra DB/origen, no se detuvo/reinició ningún servicio real, no se hizo dump/copia, no se accedió a GCP y no se modificaron V1–V61.
