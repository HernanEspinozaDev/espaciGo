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

### Tercer diagnóstico de preflight (2026-10-20)

Tras fusionarse el diagnóstico anterior, la ejecución read-only `espacigo-g03o-v64-transfer-4wx9v` volvió a detenerse antes de la transacción con endpoint `source` y componente `count_usuario`, pero sin categoría SQLSTATE. Esto acota el punto de fallo al SELECT del conteo de `public.usuario`; por sí solo no demuestra si el motivo remoto fue privilegio, objeto o estado de catálogo.

La reproducción local con PostgreSQL 18 y un rol sintético sin `SELECT` mostró el stderr de `psql -f -` como `psql:<stdin>:5: ERROR:  42501`. El parser anterior solo aceptaba líneas que empezaran directamente con `ERROR/FATAL/PANIC`, por lo que descartaba ese formato de stdin. La corrección mínima reconoce además el prefijo exacto `psql:<stdin>:<línea>:` y sigue exponiendo solo la categoría allowlisted. El código SQLSTATE 42501 fue de la fixture sintética; no es evidencia del SQLSTATE recibido por `4wx9v` ni identifica la causa Cloud.

Esta revisión añade una prueba unitaria del formato de psql observado en la reproducción local. `python3 scripts/test_gcp03_origin_v64_transfer.py` (8 pruebas), `py_compile` y `git diff --check` pasan. No se volvió a ejecutar Cloud Run ni se consultó Cloud SQL; tampoco hubo cambios a grants, IAM, secretos, buckets o datos. La revisión del nuevo PR es el siguiente paso antes de cualquier nuevo preflight.

### Cuarto diagnóstico y enum SQLSTATE seguro (2026-10-20)

El Job `espacigo-g03o-v64-transfer-b6qb9` alcanzó `count_usuario` y devolvió endpoint `source`, categoría `query_or_privilege` y `transaction_outcome=not_started`. Esa categoría no diferencia permisos de errores de objeto/sintaxis y no se recibió el SQLSTATE literal. No se atribuye una causa concreta.

El siguiente ajuste mantiene el valor únicamente de manera interna para mapear un conjunto finito de SQLSTATE conocidos a etiquetas seguras (`insufficient_privilege`, `undefined_table`, `undefined_column`, `syntax_error`, y algunos estados conocidos de conexión/autenticación/database/schema). Los códigos no incluidos se reducen a la categoría de clase conocida o `other`; nunca se emiten el código SQLSTATE, mensajes, SQL, usuario ni filas.

La validación local cubre el enum, los prefijos de psql, el fallback para códigos desconocidos y la ausencia del código literal en el detalle seguro. El motivo de `b6qb9` permanece abierto. No se ejecutaron más Jobs, no hubo consultas Cloud SQL ni cambios a grants/IAM/ACL/secretos/buckets/datos; un nuevo preflight espera revisión del PR.

### Quinto diagnóstico de acceso previo al conteo (2026-10-20)

La ejecución reportada `espacigo-g03o-v64-transfer-ckkqh` falló en `source` / `count_usuario` con `insufficient_privilege` antes de iniciar la transacción. El siguiente ajuste incorpora un precheck allowlisted de `to_regclass('public.usuario')` y `has_table_privilege(current_user,'public.usuario','SELECT')`. El reporte expone solo dos booleanos (presencia y permiso SELECT). Un guard SQL, dentro de la misma transacción `REPEATABLE READ READ ONLY`, aborta antes del marcador y consulta `count_usuario` si la tabla falta o SELECT no está permitido. No se concede ni recomienda ningún grant.

Validación local: 10 pruebas unitarias focalizadas, `py_compile`, `bash -n`, `git diff --check` y el harness de PostgreSQL 18 desechable pasaron. El caso sintético sin SELECT informó tabla presente/permiso falso y no alcanzó el conteo; con un grant de fixture, completó el reporte. El contenedor usó red desactivada y datos tmpfs, y se retiró al finalizar. No se ejecutó Cloud Run ni se consultó o modificó Cloud SQL, IAM, ACL, secretos, buckets o datos. El resultado es evidencia local de control de flujo, no una nueva lectura de estado Cloud. El siguiente paso es revisión del PR; cualquier ejecución remota requiere una autorización y liberación separadas.

### Sexto diagnóstico de contexto ACL/rol (2026-10-20)

El revisor reportó que la corrida de solo lectura `fp76j` obtuvo `usuario_present=true` y `usuario_select_allowed=false`, y se detuvo antes del conteo; no hubo escrituras. Antes de decidir entre ejecutar como el owner previsto o solicitar un permiso específico, el precheck ahora añade únicamente booleanos para `USAGE` del schema `public`, presencia del rol `espacigo_schema_owner`, membresía `MEMBER` y permiso efectivo `SET ROLE`. Para que la observación de presencia/ACL no dependa de `USAGE`, localiza la relación mediante los catálogos `pg_class`/`pg_namespace` y consulta su OID. El guard read-only detiene además el reporte antes de cualquier conteo si falta `USAGE`.

En PostgreSQL 18, `MEMBER` y `SET` responden preguntas distintas: `MEMBER` confirma membresía directa o indirecta, y `SET` indica si `SET ROLE` es posible. La documentación oficial describe ambos checks en [pg_has_role](https://www.postgresql.org/docs/18/functions-info.html#FUNCTIONS-INFO-ACCESS-TABLE).

Validación local: 10 pruebas unitarias focalizadas, `py_compile`, `bash -n` y el harness PostgreSQL 18 aislado pasaron. La fixture cubrió owner ausente, membership con `MEMBER=true/SET=false`, y `SET=true`; probó también ausencia de `USAGE` antes de conteos y un reporte completo con permisos sintéticos. No se ejecutó Cloud Run ni se consultó o cambió Cloud SQL, IAM, ACL, secretos, buckets o datos; no se concedieron permisos. La clasificación observada en `fp76j` no se convierte en una propuesta de grant.

### Séptimo diagnóstico del propietario real de `usuario` (2026-10-20)

El revisor reportó que la corrida `6f7gr` encontró tabla presente, `SELECT=false`, `USAGE=true`, y el rol esperado `espacigo_schema_owner` ausente; el guard detuvo el reporte antes del conteo, sin escrituras. La consulta añade tres booleanos derivados del OID propietario real en `pg_class`: si el rol propietario existe, si su OID coincide con `current_user` y si el usuario actual puede `SET ROLE` a ese OID. No muestra el nombre del propietario ni nombres arbitrarios. Se conserva el reporte previo del rol fijo esperado como booleanos separados.

### Octavo corte local: propietario común para lecturas y dump (2026-10-20)

La prevalidación `8kwgj` confirmó que `public.usuario` existe, `SELECT=false`, `USAGE=true`, el rol fijo esperado está ausente y el login actual puede hacer `SET ROLE` al propietario real. El informe detuvo el conteo y no escribió datos. Este resultado motiva un cambio local separado: inventariar las relaciones requeridas por el reporte, las referencias de archivos y `pg_dump`; seguir solo con inventario completo, un único propietario y permiso efectivo `SET`. Las consultas de origen y `pg_dump` usarán ese propietario. Diagnósticos y salida segura conservarán solo booleanos; el nombre se mantiene en memoria para ejecutar la conexión.

Este corte no ejecuta Cloud SQL, grants, transferencia ni cambios Cloud. El harness PostgreSQL 18 ejecuta la consulta real de inventario y valida propietario único/mixto, acceso efectivo, `SET ROLE` permitido/denegado y RLS forzada; además llama `pg_dump` con el nombre crudo del propietario descubierto y verifica que exporte la fila fixture esperada. Una revisión encontró incompatibilidad de tipo en el registro sintético de relaciones ausentes; se corrigió alineando el cast con `pg_class.relkind` (`"char"`). Otra revisión detectó que `pg_dump --role` requiere el nombre sin comillas SQL; el harness valida la invocación real corregida.

Validación local: 10 pruebas unitarias focalizadas, `py_compile`, `bash -n`, `git diff --check` y el harness PostgreSQL 18 aislado pasaron. La fixture cubre owner real distinto con `SET=false`, `MEMBER=true/SET=false` para el grupo esperado, `SET=true` y owner real igual a `current_user`; comprueba que la falta de `USAGE` se informe sin impedir leer el catálogo, y que el guard no ejecute conteos cuando falta acceso. No se ejecutó Cloud Run ni se consultó o cambió Cloud SQL, IAM, ACL, secretos, buckets o datos; no se concedieron permisos. El siguiente paso es revisar el nuevo PR; una ejecución Cloud queda detenida hasta revisar si el owner real es utilizable bajo el contrato.

### Noveno corte local: diagnóstico por conteos y modo de lectura seguro (2026-10-20)

El revisor reportó que el preflight read-only `qsdrm` se detuvo en `source_preflight` antes de transacción/escrituras con relaciones completas=false, propietario único=false, SET permitido=true, acceso completo=false y RLS completa=true. Esa salida no permitía separar relaciones requeridas ausentes, owner mixto o ambos. No se ejecutarán más lecturas Cloud para este diagnóstico.

El inventario local agrega códigos fijos y conteos acotados por grupo (`report` y `file_reference`) y un bucket de propietarios (`0`, `1`, `2+`), sin nombres de tablas ni roles en informes/errores. Las relaciones faltantes siempre bloquean el reporte y el dump. Para owner mixto o `SET ROLE` no disponible, solo se admite el login actual si ya tiene `USAGE`/`SELECT` sobre cada relación requerida y puede leerla íntegramente según RLS; en ese modo no se ejecuta `SET ROLE` ni se pasa `pg_dump --role`. No se solicitan ni crean grants. Si ninguna de esas rutas es comprobable, el preflight bloquea.

Si aparecen relaciones faltantes, el siguiente paso de diseño es revisar la integridad del esquema restaurado contra el inventario versionado y decidir una reparación/restauración revisada; no inferir nombres desde logs, renombrar objetos ni conceder permisos. Si hay owner mixto sin acceso completo por el login, detener y preparar una solución separada de mínimo privilegio para revisión antes de otro preflight.

Validación local del corte: pruebas unitarias focalizadas y harness PostgreSQL 18 para inventario real, faltantes en ambos grupos, owners únicos/mixtos, fallback de login validado/no validado, SET y RLS; `py_compile`, `bash -n` y `git diff --check`. No se ejecutó Cloud, grants ni transferencia en este corte.
