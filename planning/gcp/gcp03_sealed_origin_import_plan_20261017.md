# GCP-03 — transporte cifrado e importación del origen sellado

Fecha: 2026-10-17. Estado: propuesta de implementación local para revisión; ninguna transferencia ni importación autorizada. La captura existente no se repite. GCP-03 sigue parcial.

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

Antes de diseñar el executor, revisión de implementación debe fijar el tratamiento de owners, roles gestionados Cloud SQL, ACL/default ACL, extensiones, `SECURITY DEFINER`, objetos incompatibles con superuser, objetos de la base, secuencias, PostGIS y archivos. Prohibir reejecutar migraciones: V1–V61 son historial byte a byte. Los datos sensibles incluidos por la captura solo pueden entrar al bucket cifrados y, después, a la copia aislada prevista; servicios HTTP, workers, schedulers, webhooks, correo, pagos/proveedores y credenciales de origen permanecen desactivados. El restore/normalización y saneamiento corren como Job de una tarea con cero reintentos, imagen por digest, identidad de mínimo privilegio y bucket/secret/instancia acotados al run. La transacción puede quedar incierta: reconciliar antes de una nueva ejecución; nunca reintentar automáticamente.

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
