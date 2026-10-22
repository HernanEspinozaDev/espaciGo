# ADR IMG-01 — medios reales, WebP y galerías existentes

- Estado: propuesta de diseño para revisión; sin implementación ni cambios persistentes.
- Fecha: 2026-10-22.
- Base inspeccionada: `origin/main` `dc58d732a7e2883c3fe2f6dc1c365cb5026cf5f6`, migraciones hasta V67.
- Contexto y aceptación: [plan IMG-01](plan_img_01_carga_crud.md).

## Decisiones

### D1. Un agregado de lectura con fuentes sintéticas y reales separadas

No se reescriben V32, V33, V49 ni V56, y no se copian fotos sintéticas al modelo real. La galería sintética continúa en `espacio_galeria_sintetica_local`; los nuevos binarios usan tablas nuevas `media_asset`, `media_variant` y `space_media` (nombres finales se fijan en el corte DB). Go compone ambas fuentes en un DTO de galería con un discriminador de origen para la vista propietaria y URLs same-origin. El DTO público nunca devuelve el discriminador privado, object key, bucket o generation.

El cupo `MaxPhotosPerSpace=10` es el mismo para ambas fuentes. Cada operación de alta bloquea la fila propietaria/activa y luego la fila de `espacio`; en esa transacción cuenta fotos sintéticas activas, reservas sintéticas `reservado` y reservas/fotos reales en estado que consume cupo. El alta sintética debe cambiar en el corte Go para usar esa misma regla. El alta real reserva un slot antes de procesar; todo cambio de estado que confirme o retire slot vuelve a validar el lease. La prueba concurrente coloca el espacio en 9 y ejecuta altas reales/sintéticas simultáneas: solo una puede reservar el décimo.

El orden también es único entre fuentes. Una tabla aditiva de orden de galería refiere exactamente una fila sintética histórica o una relación real mediante dos FKs nullable con `CHECK` de exclusión; `(space_id, position)` es único y diferible dentro de transacción. V68 inserta una fila de orden por cada foto sintética activa con posición determinista `creada_en,id`; no modifica su fila ni su blob histórico. Ordenar/reordenar bloquea la fila del espacio y actualiza la secuencia completa. La relación real conserva alt/publicación; la tabla de orden no duplica media ni vuelve a serializar el contrato sintético.

La portada sigue siendo una sola selección. Se mantiene `espacio.portada_foto_id` y su FK histórica; V68 añade un puntero a la relación real y un `CHECK` de exclusión mutua. Seleccionar una portada toma el lock de `espacio`, limpia la referencia de la otra fuente y guarda la nueva. Las lecturas del catálogo resuelven la fuente seleccionada y aplican los mismos gates `ready`, activa, pública y publicación vigente. La portada sintética existente sigue funcionando sin conversión ni duplicación. Al retirar la portada se usa la siguiente foto activa pública según el orden global; si no hay ninguna, se deja sin portada.

### D2. Avatar conserva el contrato y autorización actuales

`GET /api/v1/profile/photo/content` hoy requiere Bearer de la cuenta propia activa; responde `image/png`, hash y `Cache-Control: no-store`. La primera versión de avatar real conserva ese acceso privado propio y el reemplazo/eliminación existentes. No se interpreta “privado por defecto” como una decisión nueva: se mantiene el alcance exacto observado. No hay avatar anónimo, catálogo, otra cuenta o acceso administrativo nuevo. Añadir lectura pública necesita una ADR/decisión de producto separada.

El avatar real cambia el puntero solo después de que el nuevo `ready` esté confirmado; el anterior permanece hasta ese punto. En caso de fallo, la lectura sigue entregando el anterior. Exportación/supresión recorren ambos modelos y todas sus variantes.

### D3. Idempotencia, cupo concurrente y escritores con fencing

La clave de operación se limita por `(actor_id, resource_type, resource_id, operation, idempotency_key)`. Se guarda SHA-256 del contenido de entrada y la huella de los parámetros normalizados de la operación. Repetir clave y huellas iguales retorna el resultado persistido. Repetir clave con contenido o parámetros distintos devuelve `409 idempotency_conflict`; no sustituye, reintenta ni deja otro archivo activo.

Las reservas tienen `lease_until` y token de fencing creciente asignado en PostgreSQL. El writer solo puede finalizar si siguen vigentes reserva, permiso, propiedad, estado publicable, cupo y fencing token. El objeto físico de cada intento incluye un identificador no reutilizable; su escritura exige precondición create-only. Un escritor vencido puede dejar un huérfano aislado a limpiar, pero no puede confirmar el asset ni eliminar objetos del intento actual. Los claims del cleanup se hacen en transacción con `FOR UPDATE SKIP LOCKED`, leases y estado terminal idempotente.

### D4. Una frontera Go privada, sin carga directa desde el navegador

Angular carga un archivo por multipart al API Go con Bearer de aplicación y clave idempotente. El browser no recibe credencial Google, signed URL ni permiso GCS. El API tiene límites de bytes de cuerpo y de parte, decodifica el formato real, y limita megapíxeles, lados, CPU, memoria, tiempo y concurrencia por instancia. No descarga URLs suministradas por el cliente ni registra bytes, nombres de archivo, EXIF o correo.

La cifra inicial es 10 MiB por archivo/solicitud más overhead multipart. El límite efectivo es el mínimo de Go, gateway/proxy/LB y Cloud Run. La cuota oficial de Cloud Run limita requests HTTP/1 a 32 MiB y servicio a timeout configurable hasta 60 min; esta cuota no significa que el proxy intermedio permita 10 MiB. El servicio actual `espacigo-test-api` fue observado el 2026-10-22 con 1 vCPU, 1 GiB, concurrencia 80 y timeout 300 s. No se cambiará Cloud Run en este corte: el corte de integración medirá esos recursos, fijará un semáforo CPU/memoria y probará el timeout/cuerpo por la ruta real antes de cualquier activación.

### D5. Encoder de evaluación y salida WebP

La opción inicial elegida para prueba técnica es `github.com/deepteams/webp` v1.2.8: upstream declara encode/decode lossy y lossless con alpha, implementación pure Go sin CGo y Go 1.24.2 mínimo; licencia MIT con aviso BSD de libwebp derivado conservado en el paquete. Su API recibe `image.Image`. La elección no se considera lista para producción hasta probar fixtures, orientación/transparencia, comportamiento de límites, memoria/tiempo con imágenes límite, round-trip, fuzz/corpus y el build Linux/amd64 `CGO_ENABLED=0` del Dockerfile scratch actual. La versión se fija exacta en el PR de Go y solo se incorpora si esos gates pasan; cualquier incompatibilidad actualiza este ADR y se revisa antes de continuar.

La entrada permite JPEG/PNG/WebP estáticos. La salida transforma píxeles decodificados, rota según orientación EXIF antes del encode y no copia perfiles ni EXIF/GPS. Los derivados son display (máximo lado 1600), thumbnail (480) y avatar (512); no se amplía. Quality 80 es punto de inicio, no garantía de peso. Se busca <=600 KiB para display y <=100 KiB para thumbnail como objetivos medidos. Si exceden, el encoder puede reducir dimensiones/calidad solo hasta el piso de legibilidad/artefactos previamente probado. Si el archivo todavía excede, el API responde error accionable mostrando el límite/resultado sin conservar el original; no rechaza automáticamente por superar un objetivo de compresión. Entrada GIF animada, SVG, AVI/vídeo, AVIF y HEIC queda fuera del primer corte.

### D6. Metadatos relacionales, gránulos de propiedad y grants

La migración añade identificadores UUID del servidor, propietario/propósito/estado/fechas UTC, token de versión, huella idempotente, variantes con generation/checksum/tamaño/MIME/dimensiones y una relación espacio-imagen con orden/texto alternativo/visibilidad. Un avatar activo se señala de forma única por cuenta. Las FKs hacen explícito ownership; las referencias a cuentas/espacios aplican `ON DELETE RESTRICT` y sus flujos actuales de supresión deciden la retirada.

`space_media` es dueña de alt/publicación para los assets reales. `gallery_item_order` es dueña del orden global y guarda una FK nullable a la fuente sintética o a `space_media`, con un `CHECK` que admite exactamente una. Las FKs compuestas fijan el mismo espacio/propietario y la posición es única al commit. No se guardan URLs firmadas permanentes ni object keys en respuesta. En PostgreSQL se guarda la key privada por variante y la generation exacta solo para el adaptador/cleanup.

No se crea un login, rol Cloud SQL ni permiso de superusuario. El runtime existente `espacigo_runtime` obtiene grants solo en las tablas nuevas necesarios para lectura y transacciones de la aplicación (sin GRANT a `PUBLIC`, ni escritura a tablas ajenas). La definición de columnas/operaciones se incorpora explícitamente en `internal/dbbootstrap/runtime_permissions.go`; el runner de migración sigue siendo operador externo, no proceso de inicio. El worker de limpieza reutiliza el worker ya ejecutado en API en esta entrega; si requiere una identidad separada o endpoint SQL nuevo, se detiene y propone otro corte.

La primera migración libre es V68 al HEAD base inspeccionado. Se conserva V1–V67 byte por byte, se prueba DB vacía y upgrade V67, y se confirma hash antes/después. Ningún DDL/DML llega a Cloud SQL durante la planificación o los PR de implementación. Aplicación persistente requiere procedimiento, backup/rollback, SHA/migración exacta y gate revisado e independiente.

### D7. Publicación, lectura y revocación

El browser solicita `/api/v1/publications/{id}/photos/{photoId}/{variant}` sin exponer storage. La allowlist pública acepta exclusivamente GET/HEAD de display/thumbnail de una publicación publicada y variante `ready`, publicada y asociada a espacio vigente. El handler no permite query parameter, path arbitrario ni variantes desconocidas y no filtra claves/metadata internas. El resto (uploads, reordenar, alt, portada, visibilidad y borrado) requiere Bearer propietario activo y autorización Go.

Las respuestas públicas usan MIME verificado, nosniff, ETag por checksum y cache corto con revalidación; el flujo privado de avatar usa no-store. DELETE cambia primero la visibilidad/estado transaccionalmente para revocar acceso a través del API, retira portada si aplica y programa cleanup. El cliente ya no puede leer aunque los bytes existan en GCS.

Cleanup ejecuta `Delete` con generation exacta almacenada. Bucket inspeccionado: privado UBLA/PAP, versionado activo y soft delete 7 días. Por ello, éxito de Delete/lógica API no equivale a purga física inmediata; un objeto soft-deleted puede seguir retenido y costar hasta que venza la política. Retención, reintentos y cuota se miden por el namespace IMG-01. No cambiar bucket, lifecycle global ni versionado en esta entrega. Los conflictos de generación se reportan y no autorizan borrar otra versión.

### D8. Invariantes de seguridad y datos

- Solo el propietario activo edita o carga; el servidor comprueba actor, recurso, propiedad/estado y límite en cada comando.
- Datos públicos mínimos y explícitos. Reservas, identidad, contratos, pagos, evidencia y administración no se exponen por el nuevo gateway.
- No se almacena original ni EXIF/GPS. No se aceptan archivos por MIME/extensión declarados ni SVG con contenido activo.
- Supresión/exportación recorre sintéticas + assets reales + cada variante, responde MIME y tamaños verdaderos, retira API y encola cleanup por generation.
- Sin personas/fotos personales en fixture o prueba. Se usarán imágenes sintéticas recreables. No abrir/leer política local de cuentas de ensayo porque no se requieren cuentas nuevas durante planificación.
- No añadir CORS, permisos bucket, cuenta de servicio, IAM, DNS ni Terraform en el corte.

## Alternativas descartadas

| Alternativa | Motivo |
| --- | --- |
| Sobrescribir V32/V33/V49/V56 o cambiar MIME de registros sintéticos | Cambia historial y falsea un contrato fijo `synthetic-png-v1`. |
| Dos galerías con cupo de diez cada una | Permitiría hasta veinte y selección duplicada de portada. |
| Guardar original o devolver path/nombre del cliente | Retiene EXIF/GPS y habilita nombres/información interna como interfaz pública. |
| URL pública/signed URL persistente o acceso directo del browser a GCS | El avatar/política de publicación no lo autorizan; aumenta costo de revocación y exposición de credenciales. |
| Error por cualquier WebP mayor a objetivo | Un objetivo de bytes no prueba que la foto sea ilegible/inaceptable; el corte debe medir y reportar alternativa segura. |
| Migración SQL desde Terraform o arranque Cloud Run | Mezcla release con infraestructura y quita el gate independiente de revisión/rollback. |

## Evidencia requerida para decidir si se implementó

Las pruebas futuras no se sustituyen por este ADR. Registrar cada evidencia de runtime por SHA después del corte de código: DB/Go para uploads, conversión y cleanup; Angular para lectura/CRUD; recorrido real con GCS privado y PostgreSQL aislado; la activación persistente mantiene gate aparte. Las fotos reales no se presentan como implementadas hasta mostrar bytes WebP derivados en GCS, metadata PostgreSQL, autorización de variantes, UI integrada y revocación/cleanup demostrados.

## Fuentes primarias

- [Cuotas y límites oficiales de Cloud Run](https://docs.cloud.google.com/run/quotas): límite HTTP/1 de request 32 MiB; consultar el límite efectivo de toda la ruta.
- [Timeout oficial de Cloud Run](https://docs.cloud.google.com/run/docs/configuring/request-timeout): configurable por servicio/revisión.
- [Encoder upstream elegido como candidato, licencia y compatibilidad](https://github.com/deepteams/webp).
- [Cloud Storage Public Access Prevention](https://docs.cloud.google.com/storage/docs/public-access-prevention) y [versiones](https://docs.cloud.google.com/storage/docs/object-versioning).
