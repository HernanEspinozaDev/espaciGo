# G03O g03o-1017 → primary V64 transfer

**Estado del corte:** implementación local preparada para revisión; sin imagen publicada, backend remoto inicializado, plan activo, IAM aplicado ni ejecución Cloud. El PR contiene un Job default-off y un ejecutor de una sola operación. La transferencia requiere una liberación de ejecución separada.

Este corte no equivale a aceptación de producto, finalización de GCP-06 ni autorización para producción, personas/proveedores/fondos reales o despliegues.

## Alcance

El corte propone copiar los datos de aplicación del snapshot retenido `espacigo_scratch` (V1–V61) a la base primaria `espacigo` (V1–V64), solo si los reportes al momento de ejecutar confirman proyecto, conexiones, versiones, ledger, conteos, semillas, rol migrador y estado vacío esperados. Las expectativas fijadas son 1.045 cuentas, 1.241 espacios, 1.228 reservas y 3.246 objetos privados por 808.343 bytes. No crea schemas, usuarios, roles, grants, extensiones ni filas de migración.

La carga usa el login existente `espacigo_migrator_login` y `SET ROLE espacigo_schema_owner`; datos, registro derivado V62 y saneamiento de sesiones/tokens/avisos/outbox se ejecutan en una única transacción. Semillas canónicas, historial V1–V64, `spatial_ref_sys`, sesiones, tokens y outbox no se importan. Antes de insertar, el Job verifica que no haya otras sesiones SQL, y toma locks ordenados sobre tablas de aplicación propiedad de `espacigo_schema_owner` para evitar escrituras concurrentes durante la copia. Los archivos se crean bajo `g03o-1017/`, con generación cero y checksum verificado; los nombres fuente se derivan de referencias SQL y el Job consulta solo esos nombres exactos, sin permiso `storage.objects.list` en los buckets de origen o destino.

El contrato cifrado se recupera desde el artefacto G03O autenticado y se agrega como una versión nueva del secreto existente. La respuesta de Secret Manager debe identificar esa versión como `ENABLED`. Su payload no aparece en salida ni estado Terraform. El Job pausa la API. Después de una transferencia exitosa, la API queda pausada hasta que una actualización revisada del root Terraform canónico configure el número exacto de versión y los prefijos `g03o-1017` y `g03o-1017/privacy-replay`; la aplicación de esa configuración reanuda el servicio conforme a su escalado declarativo. En el repositorio, el recurso API está declarado en `infra/terraform/environments/testprod/gcp`; `gcp04-runtime` declara worker/migrator, no el servicio API.

## Gates y recuperación

`enable_job=false` y `transfer_mode=preflight` son los valores por defecto. El preflight requiere confirmaciones de origen/destino, versiones numéricas de secretos, recibo/manifiesto/informes con hashes y digests inmutables. Solo hace consultas SQL y lecturas de metadata de objetos. El modo `execute` exige además un reconocimiento separado y fingerprint SHA-256 revisado.

El Job opera contra los nombres fijos de instancia/base, API y buckets. Ningún recurso Cloud se crea con este PR. La raíz nueva fija backend `gs://espacigo-testprod-tfstate-447373903153-gcp` con prefijo `espacigo/testprod/g03o-1017-v64-transfer/20261020`; no inicializarlo ni leer/escribir ese state hasta revisión. Los comandos locales de validación usaron `-backend=false` y directorios `TF_DATA_DIR` temporales. El costo de Job/transferencia/secreto se revisará antes de habilitar ejecución; este corte no crea una instancia SQL.

Si el estado de la transacción no se puede probar, la API permanece pausada y no se limpian objetos copiados. Cada proceso usa un transfer ID único; si la respuesta de rewrite es ambigua, el objeto se conserva y el siguiente preflight lo reconcilia. Si se prueba rollback y la base sigue vacía, el Job elimina únicamente generaciones confirmadas por esta invocación y restaura el escalado observado. Una versión de llave agregada antes de un fallo queda como versión `ENABLED` sin uso y requiere reconciliación posterior; no se elimina automáticamente. Ante commit confirmado con postcheck fallido, la API permanece pausada y el contenido queda retenido para inspección. Un `setval` de secuencia emitido por `pg_dump` puede avanzar posiciones aun si PostgreSQL revierte filas; eso solo deja saltos de IDs en la base vacía y no reintroduce datos.

## Validación local de este corte

- `python3 -m py_compile scripts/gcp03_origin_v64_transfer.py`: pasó.
- Una prueba unitaria enfocada verificó que el inventario destino consulta únicamente los nombres esperados por `get_blob`, fija generación en la recarga y no lista el bucket: pasó.
- `terraform fmt -check` pasó para las raíces de transferencia y API.
- `terraform validate` pasó para ambas raíces, tras `init -backend=false` en directorios temporales.
- `git diff --check`: pasó.

No se ejecutaron el build de imagen, harness de bases, plan remoto/activo, apply, Cloud Run Job, consulta SQL remota ni cambios a Cloud SQL, Cloud Run o Secret Manager. La validación local no acredita que las instancias, identidades, versiones de secreto, artefactos o costos sigan vigentes. El siguiente gate es revisión del PR y, después, una liberación de ejecución independiente con nuevas lecturas de estado y costo.
