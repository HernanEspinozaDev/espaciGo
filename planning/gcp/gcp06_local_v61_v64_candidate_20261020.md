# GCP-06: preparación local de datos V61 → V64

**Estado:** implementación local candidata para revisión. No autoriza publicación de datos, importación remota ni cambio de tráfico.

## Objetivo de este corte

Preparar un exportador reproducible que lea `espacigo_local` sin escribir en el origen, restaure el snapshot en PostgreSQL 18/PostGIS desechable, sanee las filas copiadas y aplique V62–V64 con el migrador Go de producción. El archivo final queda bajo `.local/testing/` con permisos privados y un manifiesto de conteos y SHA-256.

La implementación está en `scripts/gcp06-local-stage-export.sh` y `scripts/gcp06_stage_sanitize.sql`. El contenedor de staging usa una imagen fijada por digest, red Docker deshabilitada y filesystem temporal. El dump V61 se transmite directamente entre los contenedores; nunca se escribe sin sanear en el host. La herramienta se niega a sobrescribir el directorio de salida y elimina el contenedor al finalizar.

## Política de identidad y publicación

Solo sobreviven las cuentas del namespace sintético `uNNNN@dataset.invalid`. Sus UUID y relaciones de marketplace se conservan; los correos se seudonimizan a `@transfer.invalid`, los teléfonos se limpian y todos los verificadores de contraseña se sustituyen por un valor inválido para autenticación. El estado `activo` se conserva para no cambiar la política de publicación de la cuenta. No se conservan identidades locales, roles de administrador, bootstrap global, sesiones, tokens, verificaciones originales, avisos, outbox, auditoría ni filas que dependan de ellos.

La política privada de cuentas consultada para preparar este corte autoriza solo ensayos locales. No autoriza llevar sus hashes o accesos a Cloud. Para iniciar sesión en un entorno Cloud será necesario un encargo separado y una cuenta creada por el flujo normal de registro/verificación. Este paquete no permite iniciar sesión con las cuentas seudonimizadas.

Como el catálogo requiere elegibilidad KYC, las verificaciones originales se reemplazan en el candidato por marcas fixture sintéticas: proveedor `local-fixture-v1`, referencia `fixture:<UUID>`, revisor sintético sin rol administrador y sin archivos de evidencia ni PII. Se genera una marca solo para propietarios sintéticos con publicaciones activas. No representa KYC real y nunca debe trasladarse a producción ni a un host no allowlisted. El SQL exportado incluye una transacción y un guard que rechaza cualquier base cuyo nombre no cumpla `espacigo_gcp06_candidate_<run-id>`; una futura fase aún debe validar proyecto/instancia allowlisted antes de importar.

## Comando local

Después de revisar este PR y verificar el origen local por el canal autorizado:

```bash
scripts/gcp06-local-stage-export.sh .local/testing/gcp06-stage-<run-id>
```

El comando requiere el contenedor local configurado (`espacigo-database-1` por defecto), base `espacigo_local`, PostgreSQL 18 con V1–V61 y la imagen fijada ya disponible localmente. Antes de ejecutar sobre el origen persistente, revisar el script y autorizar una ventana de lectura coherente; el harness de este PR solo usa un origen desechable sintético.

El resultado privado contiene `data.sql` (solo datos sanitizados, compatible con el esquema destino V64) y `manifest.json` (conteos, tamaño, SHA-256, política aplicada y ledger). Los archivos quedan en modo 0600 y el directorio en 0700. No subirlos, adjuntarlos al PR ni copiarlos a un worktree compartido.

## Evidencia local

`scripts/test-gcp06-stage-sanitize-postgres.sh` crea un PostgreSQL 18/PostGIS efímero sin red ni puertos publicados. Comprueba el cierre de taint por claves foráneas, exclusión de cuentas no sintéticas, administradores/sesiones/tokens, pseudonimización y hashes inutilizables; después aplica las migraciones reales V1–V61, ejecuta el exportador completo con datos sintéticos, importa el SQL a una base V64 candidata desechable, confirma ledger V64 y comprueba una fila por la consulta allowlisted equivalente a la política del catálogo. También confirma que el guard rechaza aplicar el mismo SQL a la base de origen sintética y que esta conserva sus dos filas.

Resultado del corte: prueba del fixture V61, prueba contra el esquema real V1–V61 y prueba end-to-end de exportación local finalizaron en `PASS`. No se volvió a leer ni exportar el volumen persistente para esta implementación.

## Fuera de alcance y siguientes gates

`infra/terraform/environments/testprod/gcp06-candidate` define de forma predeterminada apagada un Cloud SQL PostgreSQL 18 privado, la base run-scoped y un bucket privado. Los bindings de importación y carga también están apagados por separado: import usa solo `storage.objects.get`, carga solo `storage.objects.create`, ambos para `gcp06/<run-id>/data.sql` con condición IAM de objeto exacto y expiración UTC máxima de 48 horas. El bucket elimina el prefijo del run luego de siete días; el cleanup manual de la instancia exige autorización de destroy con el run ID exacto. La raíz no tiene backend remoto.

La validación local con `terraform init -backend=false`, `terraform validate`, `terraform test` (2 casos mock) y plan con refresh apagado confirmó configuración válida, no-op por defecto, destino/proyecto/región fijos, UBLA/PAP, objeto exacto, expiración máxima, create/get mínimos y destroy denegado por defecto. No hizo refresh ni consultó Cloud. No se provisionaron recursos, no se leyó/escribió state remoto, no se subieron artefactos, no se accedió a Cloud SQL, no se cambió IAM/Secret Manager, no se crearon cuentas, no se ejecutaron Jobs, no se modificó el servicio ni el tráfico. La imagen de transferencia G03O publicada previamente es ajena a esta ruta y no se usa en el harness.

### Estimación del candidato

La referencia de precios oficial consultada el 2026-10-20 para Santiago da US$0,0537 por vCPU-hora, US$0,0091 por GiB-hora de memoria y US$0,000157534 por GiB-hora de capacidad Hyperdisk Balanced. Para `db-custom-1-3840` (1 vCPU, 3,75 GiB) y 20 GiB durante 730 horas, la estimación es aproximadamente **US$66,41 por mes** (US$64,11 cómputo y memoria; US$2,30 almacenamiento). Un día continuo serían aproximadamente US$2,18. No incluye GCS, respaldos, IOPS/throughput adicionales, red, logging, impuestos ni conversión de moneda; los respaldos están apagados en esta raíz. El cálculo es una referencia de lista y debe actualizarse con el plan/cotizador antes de habilitar recursos. Fuente: [precios oficiales de Cloud SQL](https://cloud.google.com/sql/pricing).

Una fase posterior requiere revisar el paquete privado, definir el destino candidato aislado y su costo, revisar Terraform con gates apagados, verificar backup y rollback, decidir ubicación/custodia del artefacto, y autorizar por separado cada operación remota. Archivos privados, clave contractual, preservación del administrador global y aceptación de producto siguen sin resolverse en este corte.
