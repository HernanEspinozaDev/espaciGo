# GCP-03 — transporte cifrado e importación del origen sellado

Fecha: 2026-10-17. Estado: implementación local del executor preparada en una rama de PR para revisión; ninguna transferencia ni importación autorizada. La captura existente no se repite. GCP-03 sigue parcial.

## Alcance y evidencia de partida

Este corte prepara el transporte y contrato de importación de la captura local sellada existente, cuyo `source_capture_run_id` es `gcp03-merge-review-20261017` y cuyo `SEALED.json` revisado tiene SHA-256 `80a4e1e0002cf12f8e750ddaac7ceb796726b57a36405aa2b0b8e7852e928262`. El run destino `target_run_id` es `g03o-1017`; ambos IDs son distintos y se mantienen explícitos. El snapshot y `K_recovery` permanecen bajo custodia local y no se incluyen en el repositorio. La carga sintética `g03r-1016`, `cmd/gcp03restore`, su perfil `restore/retire` y `pilot01` quedan fuera de este flujo.

El bootstrap y el observer para `g03o-1017` pasaron una vez cada uno. La evidencia F3 establece una base recién bootstrappeada y observada; no valida el snapshot importado ni la compatibilidad completa de datos/owners/ACL con Cloud SQL. La ventana inicial de ocho horas se extendió por coordinación hasta las 20:15 UTC (17:15 Santiago) del 2026-10-17. La extensión no autoriza importación, staging cloud, secretos ni recursos nuevos; esos actos requieren revisión/liberación y plan exacto separados.

La entrega de código incluye `scripts/gcp03_origin_import.py` y sus pruebas unitarias. El comando `stage` valida el SHA exacto del sello, `SEALED.sha256`, el run, el allowlist cerrado de cuatro artefactos, hashes/tamaños de ciphertext, `v1_v61_preserved`, credenciales excluidas y metadata autenticada. Compara las 61 entradas `schema_migrations` de la metadata sellada (nombre + SHA-256) con los bytes actuales V1–V61 del checkout. Descifra el cifrado GPG fuente en streaming y vuelve a cifrar bloques de hasta 1 MiB en AES-256-GCM con DEK aleatoria de 256 bits por artefacto; cada chunk usa nonce aleatorio de 96 bits y AAD con run, artefacto e índice. Envuelve las DEK con AES-256-GCM usando `K_run = HKDF-SHA-256(K_recovery, salt_aleatorio_32B, info="espacigo/gcp03/origin/<run-id>/transport/v1")`. No escribe plaintext ni llama a Google Cloud. `verify-recovery` deriva `K_run` fuera de cloud y autentica todos los chunks/hash sin guardar plaintext.

El paquete resultante contiene solo ciphertext, hash/longitud fuente y transport, salt, nonces, DEK envueltas y contrato. Directorio 0700, archivos 0600. La llave `K_recovery` no se copia ni viaja; el resumen no revela rutas locales, identidades ni material criptográfico. Un fallo durante `stage` puede dejar ciphertext parcial en el directorio de salida privado; el comando retorna error y el directorio **no se puede subir**. El operador debe anotarlo en la evidencia privada y retirarlo con una acción exacta después de preservar diagnóstico no sensible.

## Preparación local (sin conexión a GCP)

Desde la raíz del repositorio, instalar la dependencia fijada en un virtualenv local aislado si el entorno no la tiene:

```sh
python3 -m venv .local/testing/gcp03-import-venv
.local/testing/gcp03-import-venv/bin/python -m pip install -r scripts/requirements-gcp03-origin-import.txt
.local/testing/gcp03-import-venv/bin/python scripts/test_gcp03_origin_import.py -v
```

Después de recibir liberación específica de staging, el operador ejecuta el mismo checkout revisado con rutas de snapshot y custodia obtenidas únicamente de su inventario privado. Los siguientes placeholders deben resolverse localmente; no se guardan en PR, logs compartidos ni argumentos copiados a tickets:

```sh
.local/testing/gcp03-import-venv/bin/python scripts/gcp03_origin_import.py stage \
  --snapshot '<ruta-privada-del-snapshot-existente>/gcp03-merge-review-20261017' \
  --recovery-key '<ruta-privada-de-custodia>/recovery.key' \
  --source-capture-run-id gcp03-merge-review-20261017 \
  --target-run-id g03o-1017 \
  --expected-seal-sha256 80a4e1e0002cf12f8e750ddaac7ceb796726b57a36405aa2b0b8e7852e928262 \
  --output "$PWD/.local/testing/gcp03-origin-import-g03o-1017"
.local/testing/gcp03-import-venv/bin/python scripts/gcp03_origin_import.py verify-recovery \
  --stage-dir "$PWD/.local/testing/gcp03-origin-import-g03o-1017" \
  --recovery-key '<ruta-privada-de-custodia>/recovery.key' \
  --source-capture-run-id gcp03-merge-review-20261017 \
  --target-run-id g03o-1017 \
  --expected-manifest-sha256 '<hash-manifest-copiado-del-reporte-stage>'
```

Guardar stdout JSON saneado y SHA-256 del `transport-manifest.json` en evidencia excluida y 0600. Comparar el hash impreso con el hash calculado independientemente; inspeccionar únicamente claves/nombres/tamaños/hashes del manifest, jamás plaintext ni contenido personal. No invocar estos comandos en esta revisión: aquí no se dispone del payload ni de la llave.

## Contrato de importación

El manifest generado lleva `source_capture_run_id` y `target_run_id` en campos separados. El primero selecciona el directorio/sello y deriva las passphrases GPG del snapshot original; no se renombra ni modifica. El segundo liga HKDF, AAD, rutas de objetos y contrato al destino de importación. El manifest lleva `espacigo.gcp03.origin-import-contract.v1`, `status=staged_ciphertext_only`, nombre exacto `espacigo_scratch`, exigencia de target vacío, sello/hash requerido, digest inmutable del executor requerido, evidencia bootstrap/observer hash-bound, una sola tentativa, cero reintentos, servicios/workers prohibidos y saneamiento antes de verificación. El uploader/restore executor de cloud es trabajo futuro explícito; el estado del paquete no significa importación completada.

Antes de autorizar una importación real, la revisión debe confirmar owners de roles gestionados Cloud SQL, ACL/default ACL, extensiones, `SECURITY DEFINER`, objetos incompatibles con superuser, objetos de la base, secuencias, PostGIS y archivos. El executor ya aplica gramática cerrada para SQL de owners/ACL, pero el inventario y mapa real no se generaron ni validaron aquí. Prohibido reejecutar migraciones: V1–V61 se validan como bytes/hash y se conservan como historial. Los datos sensibles solo pueden entrar al bucket cifrados y luego a la copia aislada prevista; servicios HTTP, workers, schedulers, webhooks, correo, pagos/proveedores y credenciales de origen permanecen desactivados. La futura ejecución requiere Job de una tarea con cero reintentos, imagen por digest, identidad de mínimo privilegio y bucket/secret/instancia acotados al run. Cualquier transacción incierta se concilia antes de considerar otro intento; no existe retry automático.

Secuencia futura obligatoria:

1. Revisar estado, extensión de ventana y plan Terraform refrescado. Reconciliar el costo/estado de `g03o-1017` cuya cota ya expiró antes de crear recursos nuevos.
2. Ejecutar `stage` local solo después de liberación. Revisar hash del sello, manifest y recuperación local fuera de cloud. No transferir `K_recovery`.
3. Revisar digest de imagen, Terraform de staging, policy IAM exacta, bucket destino y plan de subida ligado a run/hash. Permitir solo ciphertext/manifest; generar y fijar generación de cada objeto. El SA de staging no recibe acceso al secret de transporte.
4. Crear `K_run` como versión numérica del secret efímero mediante helper sin revelar el payload; solo el SA del restore tiene acceso a ese secret y a objetos exactos. Revisar el plan y las generaciones antes de aplicar/subir. No incluir valores de secret en plan JSON, stdout o logs.
5. Verificar objetos con generación + SHA-256 + tamaño, recuperar y autenticar todos los chunks en el executor. Confirmar target vacío, identidad y evidencia del observer antes de que el Job abra conexión mutadora. Importar una vez a `espacigo_scratch`, normalizar owners/ACL según mapa explícito y sanear sesiones, tokens, leases, avisos/outbox, pagos/providers y runtime. Mantener API/worker fuera de servicio.
6. Ejecutar una verificación read-only independiente para V1–V61, ownership/ACL, funciones y `search_path` seguro, extensiones, constraints/índices/secuencias, agregados de negocio, ocupaciones espaciales, referencias de archivos y postcondiciones de cuarentena. Revisar diferencias y evidencia antes de cualquier servicio.
7. Retirar primero la versión de `K_run` mediante plan/revisión aparte, después objetos/generaciones exactas y finalmente infraestructura run-scoped con dos planes de teardown (protección y luego eliminación), preservando snapshot sellado y evidencia. Verificar IAM ausente, bucket vacío, recursos retirados y costos detenidos. No borrar baseline, VPC/subred, Artifact Registry ni backend state compartido.

Cada límite 2–7 requiere plan y liberación específica; esta PR y sus tests solo implementan preparación/verificación local. No se acepta el producto ni se declara GCP-03 completo.

## Duración, costo y límites

Referencia de precio regional registrada 2026-10-17: instancia N4 2 vCPU/4 GiB + 20 GiB Hyperdisk Balanced = USD 0.202209863/hora (aprox. USD 1.62/8h). El bucket Standard regional cuesta USD 0.03/GiB-mes: para `S` GiB ciphertext y `T` horas, estimado `0.03 * S * T / (30*24)` más operaciones y transferencia. El Cloud Run Job con 1 vCPU/1 GiB tiene costo mínimo de 60s; a las tasas registradas equivale aproximadamente USD 0.00144 por ejecución mínima. Secret Manager payload/operaciones, logging, egress, soporte y descuentos no se incluyen. Cifras antes de impuestos/créditos; no son límite duro ni autorización de gasto.

Ventana futura propuesta: máximo 8 horas desde creación SQL, con reservar primera hora para setup/reconciliación, dos horas para una sola importación, dos horas para verify, y tres horas como margen/teardown. El stage local es previo y no consume Cloud SQL. El tamaño del payload no está disponible en este checkout: antes de autorizar subida, calcular suma exacta de `transport_cipher_bytes` y exigir medición de throughput que proyecte transferencia + restore + teardown dentro de 5 horas. Si no cabe, no empezar; preparar nueva ventana/revisión. Cualquier timeout o outcome incierto detiene el run sin retry. La ventana de operación `g03o-1017` fue extendida por coordinación hasta `2026-10-17T20:15:00Z` (17:15 Santiago); la hora inicial 15:55:05 UTC no es la vigente. A las 18:26 UTC (15:26 Santiago), la extensión sigue activa, pero no autoriza staging/importación ni recursos/secretos nuevos. Estos requieren revisión del PR, plan exacto y liberación específica.

## Pruebas realizadas para la implementación local

`.local/testing/gcp03-import-venv/bin/python scripts/test_gcp03_origin_import.py -v`: 5 pruebas pasan (streaming GPG sintético de extremo a extremo con source/target run IDs distintos y recuperación fuera de cloud, comprobación V1–V61 byte/hash, round-trip y alteración de chunk, binding del hash del snapshot y derivación atada al run). `.local/testing/gcp03-import-venv/bin/python -m py_compile scripts/gcp03_origin_import.py scripts/test_gcp03_origin_import.py`: pasa. Estas pruebas usan datos sintéticos recreables; no leen el snapshot ni la llave real, no invocan GCP ni prueban Cloud SQL. Ejecutar de nuevo contra la rama/HEAD final al preparar PR.

## Actualización executor de importación — 2026-10-17

La corrección autenticada de metadata exige que el nombre fuente coincida con `preflight.source_projection.configured_database_name`, que sea un identificador PostgreSQL UTF-8 de hasta 63 bytes, y que el `release.run_id` sea el run de captura sellado. Las pruebas usan deliberadamente nombres distintos para la fuente (`espacigo_local`) y destino (`espacigo_scratch`); no se conserva la suposición anterior de un nombre fijo de origen.

La rama siguiente añade `scripts/gcp03_origin_cloud_import_executor.py`, adaptador de objetos GCS por generación inmutable, runner PostgreSQL 18 (`pg_restore`/`psql`) mediante loopback, SQL de saneamiento de la copia y dependencias fijadas en `scripts/requirements-gcp03-origin-executor.txt`. El executor valida receipt/manifest y sus hashes fijados, IDs fuente/destino y sello, las cuatro referencias de artefacto, generaciones, tamaños y SHA-256, el contrato de intento único, ciphertext y AEAD de todos los chunks antes de abrir conexión mutadora, allowlist del TOC contra el archivo y SQL de owners/ACL ligada al hash aprobado por configuración. Luego exige destino exacto vacío, extensiones/roles preparadas, PostgreSQL 18, identidad `postgres` y ningún schema de aplicación; hace restore en una transacción, ejecuta normalización y cuarentena, y requiere que sesiones/tokens/avisos/leasing sean inutilizables y que el runtime no tenga login. Toda salida es un reporte acotado con estado/etapa/código; no incluye metadata, roles origen, SQL ni datos. Fallos posteriores al inicio de mutación terminan como inciertos y no retryables.

La suite executor usa GCS y base falsos con ciphertext AES-GCM sintético recreable. Sus 12 casos cubren orden de una ejecución, target no vacío, hash/generación incorrectos antes de tocar la base, normalización ligada al hash, allowlist TOC y rechazo de extensión, y resultado de saneamiento incumplido con outcome incierto. La verificación ejecutada en este checkout: `PYTHONPATH=scripts python3 -m unittest scripts.test_gcp03_origin_import scripts.test_gcp03_origin_cloud_import_executor -v` (12 pruebas, pasa) y `python3 -m py_compile scripts/gcp03_origin_import.py scripts/gcp03_origin_cloud_import_executor.py scripts/test_gcp03_origin_import.py scripts/test_gcp03_origin_cloud_import_executor.py scripts/test_gcp03_origin_cloud_import_postgres.py` (pasa). Estos dobles siguen siendo pruebas unitarias y no acreditan Cloud SQL real.

En la revisión de PR #396 se añadieron cinco integraciones reales mediante `scripts/test-gcp03-origin-postgres.sh`, sobre PostgreSQL 18/PostGIS 3.6 efímero con red `none`, filesystem `tmpfs`, socket bajo `/tmp`, sin puertos publicados ni volumen persistente. El setup verifica que los schemas generados por Tiger/Topology no se confundan con schemas de aplicación; además comprueba rechazo de schema/relación de aplicación, listado y restore de un custom dump sintético mayor que 3 MiB por stream AES-GCM, autenticación completa aun cuando `pg_restore --list` cierra pronto el pipe, owners de tabla/función con firma, ACL de INSERT/UPDATE/DELETE, default ACL efectivo, rollback de normalización fallida, fallo de restore sin dejar objetos y saneamiento real de sesiones/tokens/avisos/leases/login de runtime. La ejecución completa del harness pasó: suite Go `internal/gcp03origin` y 5 tests Python PostgreSQL. Usa datos sintéticos; no consume origen/GCP. El harness retiró su contenedor y filesystem temporal al terminar; los servicios preexistentes del proyecto no se tocaron.

Este corte no incorpora todavía un Cloud Run Job/Terraform gate, no construye/publica una imagen digest-pinned, no configura el sidecar Cloud SQL Auth Proxy, ni crea IAM de uploader/restore, versiones Secret Manager, objetos o generaciones. El runner requiere que el proxy ya exista en `127.0.0.1:5432`; la conexión cloud real no es ejecutable por esta rama aislada sin esos componentes y una revisión/liberación posterior. El role/ACL plan debe generarse fuera del payload aquí inspeccionado y aprobarse por hash antes de fijarlo en receipt/config. La carga `private-files.tar` se autentica completa, pero esta implementación aún no la extrae ni adapta al bucket privado; la compatibilidad de archivos sigue abierta.

El SEALED previo registra 7,226,580 bytes de ciphertext fuente en sus cuatro objetos; ese número no incluye sobrecarga del envelope nuevo y no predice el tamaño total transferido. El tar privado no publica tamaño plaintext en el descriptor disponible. Medir los cuatro `transport_cipher_bytes` del manifest real y ejecutar una prueba de throughput/restauración antes de proponer duración/costo de fase; no se estima una ventana de importación a partir del ciphertext fuente.

No se repitió el staging autorizado previamente ni se usó el snapshot. No se llamó a Google Cloud, ni hubo SQL/pg_restore contra Cloud SQL/origen, transferencia, carga de secretos ni mutación de infraestructura/base fuera del harness descartable. El diagnóstico D1 ya ejecutado permanece metadata-only, autenticado en memoria y sin plaintext en disco; la única discrepancia fue que la fuente declara su propio nombre configurado de base y no el valor fijo supuesto por el helper. El alcance del PR actual no convierte ese diagnóstico ni las integraciones PostgreSQL sintéticas en evidencia de importación o aceptación GCP-03.
