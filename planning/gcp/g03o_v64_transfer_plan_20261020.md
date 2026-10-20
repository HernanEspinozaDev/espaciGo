# G03O g03o-1017 → primary V64 transfer

**Estado del paquete inicial:** implementación local preparada para revisión; en esa etapa no se había publicado imagen, inicializado backend remoto, aplicado IAM ni ejecutado Cloud. El PR contiene un Job default-off y un ejecutor de una sola operación. La transferencia requiere una liberación de ejecución separada.

**Actualización del preflight (2026-10-20):** el Job Cloud Run `espacigo-g03o-v64-transfer-22msr` llegó a levantar ambos proxies privados y terminó con `DATABASE_PREFLIGHT_QUERY_FAILED`, `transaction_outcome=not_started`. No ocurrió transferencia ni se inició una transacción. El runner ocultaba stderr y no identificaba cuál `report()` falló. La corrección local agrega solo `database_endpoint` (`source`/`target`) y una categoría SQLSTATE allowlisted cuando psql la devuelve en modo `VERBOSITY=sqlstate`; stderr se captura y nunca se imprime ni persiste. No se incluyen texto de error, SQL, filas, valores SQLSTATE ni secretos.

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

En esa validación local inicial no se ejecutaron el build de imagen, harness de bases, plan remoto/activo, apply, Cloud Run Job, consulta SQL remota ni cambios a Cloud SQL, Cloud Run o Secret Manager. La validación local no acredita que las instancias, identidades, versiones de secreto, artefactos o costos sigan vigentes. El siguiente gate era revisión del PR y, después, una liberación de ejecución independiente con nuevas lecturas de estado y costo.

### Diagnóstico del preflight e imagen local (2026-10-20)

Las pruebas unitarias focalizadas ahora cubren fallos SQLSTATE en los dos endpoints y errores de conexión sin SQLSTATE. El reporte diferencia `source` de `target` y reduce los códigos reconocidos a categorías amplias; no propaga el stderr, el SQLSTATE literal ni el mensaje de PostgreSQL. Se ejecutaron cinco pruebas del módulo de transferencia y `py_compile`; pasaron.

Se construyó localmente una imagen OCI Linux/amd64 desde el Dockerfile de este corte. Digest de manifiesto OCI local: `sha256:8b2aa88e92daa7c3b12af869e5eda9665101c14d2c91df83fc84e752df761398`; SHA-256 del archivo OCI: `1f8d8ad851342f373dbd3442e45b4b2009be9077c4a9842abaee9e2cc94d6688` (260.938.752 bytes). El artefacto está solo en `.local/testing/g03o-1017-preflight-diagnostic-20261020/` y no se publicó en Artifact Registry; este digest no acredita una referencia remota. No se actualizó el Job Cloud Run ni se repitió la ejecución. Publicación de imagen y aplicación de un digest nuevo requieren el flujo de release revisado por separado.

### Segundo diagnóstico de preflight (2026-10-20)

El Job `espacigo-g03o-v64-transfer-m27pf` falló antes de iniciar la transacción con `DATABASE_PREFLIGHT_QUERY_FAILED`, `database_endpoint=source` y `sqlstate_category=query_or_privilege`. La clase SQLSTATE 42 no identifica por sí sola un privilegio: también agrupa errores de sintaxis, objetos y reglas de acceso. No se recibió un componente de consulta ni un SQLSTATE concreto que permita atribuir la causa; por ello no se añaden grants.

La corrección de seguimiento divide únicamente el reporte de origen en componentes estáticos identificados y los ejecuta bajo una sola transacción `REPEATABLE READ READ ONLY`, conservando un snapshot común entre consultas aunque haya escrituras concurrentes. Si falla un componente, la salida saneada incluye su identificador allowlisted, el endpoint y, cuando está disponible, la categoría amplia SQLSTATE. El reporte no expone stderr, SQL, filas ni el código SQLSTATE literal. El target conserva su reporte agregado y se identifica como `target_report` si falla.

Pruebas locales del corte: siete pruebas focalizadas, `py_compile` y `git diff --check` pasaron. Además, el SQL exacto se ejecutó y parseó contra PostgreSQL 18 desechable en un contenedor sin red con datos en tmpfs; esa fixture valida el generador y parser con tablas sintéticas, no replica Cloud SQL ni sus ACL. No se repitió Cloud Run, no se consultó Cloud SQL y no se modificaron IAM, ACL, secretos, buckets o datos. El siguiente paso es revisión del PR; solo con una liberación separada y nueva revisión se podrá considerar otro preflight remoto.
