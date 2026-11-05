# Auditoría Cloud Storage para el plan de espacios

**Fecha:** 2026-10-27. **Base:** `3956d2f3459b0bf4b15511f4e1a74ffd06a29975`. Solo lectura. Este documento fue contrastado y corregido por el padre; distingue observaciones de propuestas. No se modificaron buckets, objetos, IAM, Terraform, CORS ni lifecycle.

> Evidencia histórica del snapshot 2026-10-27, no inventario remoto vigente. Para el estado y alcance actual de este plan, consultar `plan_creacion_espacios_galeria_documentos.md` §8 (reconciliación 2026-10-29).

## 1. Bucket utilizado realmente por Go

Lectura de Cloud Run `espacigo-test-api`, filtrando exclusivamente configuración no secreta:

| Parámetro | Valor observado |
|---|---|
| Backend | `BLOB_STORE_BACKEND=gcs` |
| Bucket | `espacigo-test-files-447373903153-gcp02` |
| Prefijo de evidencias | `CLOUD_STORAGE_OBJECT_PREFIX=g03o-1017` |
| Identidad runtime | `espacigo-test-api@espacigo-testprod.iam.gserviceaccount.com` |
| Ubicación | `SOUTHAMERICA-WEST1` |
| Acceso uniforme / prevención acceso público | UBLA `true`; PAP `enforced` |
| Versionado | Activado |
| Soft delete | `604800` segundos observados |

**Matiz importante:** `cmd/api/runtime_storage.go:20-32` utiliza el mismo bucket para `MediaStore`, pero descarta el prefijo de evidencias. Las imágenes reales usan claves `media/v1/...` en la raíz; no debe asumirse que todo está bajo `g03o-1017/`.

`infra/terraform/environments/testprod/gcp/main.tf:608-626` declara `google_storage_bucket.private_files`, con `force_destroy=false` y `prevent_destroy=true`. El binding al runtime está en líneas 656-660 (`roles/storage.objectAdmin`). El padre leyó en memoria el state remoto `espacigo/testprod/gcp/default.tfstate` y confirmó el enlace `google_storage_bucket.private_files` a este bucket, sin imprimir ni guardar state crudo. Cloud Audit Logs de creación confirma `Terraform/1.16.6` y provider `8.7.0`, evento `2026-10-16T04:30:18.059259069Z`. **Origen Terraform comprobado por código + state + evento de creación**, no inferido solo por el nombre.

Para reproducir la consulta histórica de logs es necesario `--freshness=30d`; el rango por defecto no incluyó la creación en la primera consulta del padre.

## 2. Inventario y procedencia contrastados

El padre agregó los resultados reales de `buckets list` y los eventos `storage.buckets.create`, sin contenidos de objetos ni principal de una persona:

| Bucket | Herramienta de creación observada | Uso / decisión para este plan |
|---|---|---|
| `espacigo-test-files-447373903153-gcp02` | Terraform | Almacén privado activo; reutilizar inicialmente. |
| `espacigo-gcp03-origin-g03o-1017` | Terraform | Fundación/importación histórica; no convertirlo en bucket público por llamarse «origin». |
| `espacigo-gcp03-private-g03o-1017` | Terraform | Transporte/importación histórica; conservar. |
| `espacigo-testprod-cloud-build-447373903153` | Terraform | Fuentes, locks y evidencia de release; no archivos de usuarios. |
| `espacigo-testprod-gcp06-20261020-gcp06a` | Terraform | Importación sanitizada; no galería. |
| `espacigo-testprod-tfstate-447373903153-bootstrap` | Terraform | State; no archivos de aplicación. |
| `espacigo-testprod-tfstate-447373903153-cloudflare` | Terraform | State; no archivos de aplicación. |
| `espacigo-testprod-tfstate-447373903153-gcp` | Terraform | State; no archivos de aplicación. |
| `espacigo-testprod_cloudbuild` | gcloud/SDK | Artefactos automáticos de builds. |
| `run-sources-espacigo-testprod-southamerica-west1` | gcloud/SDK | Artefactos automáticos de deploy por fuentes. |

Agregación ejecutada: **10 buckets; 8 creados vía Terraform y 2 mediante SDK**. Evidencia privada sanitizada: `.local/testing/espacios-plan/verified-bucket-origins.json`; configuración original: `buckets-raw-metadata.json` en ese directorio, modo 0600. El vínculo de state fue contrastado por el padre para el bucket activo; no se afirma que esa lectura haya cubierto todos los backends Terraform.

La inspección delegada no encontró grants públicos por bucket ni backend buckets de Compute para entregar GCS mediante Cloud CDN. Eso no debe ampliarse a «no existe ningún CDN» ni a ausencia de signed URLs. El bucket de aplicación tiene PAP enforced.

## 3. Inventario de datos: límite de la evidencia

Se observaron prefijos `g03o-1017/` y `g03o-1017/privacy-replay/`. **Un listado de prefijos no demuestra que no exista ningún archivo o documento de usuario.** No se descargaron contenidos ni se hizo inventario personal. El plan no autoriza vaciar, renombrar ni reconstruir el bucket suponiéndolo vacío. Antes de una eventual migración de objetos: inventario completo/paginado autorizado y cotejo con referencias de DB, preservando cada generación.

## 4. Adaptadores y entrega que ya existen

- `cmd/api/runtime_storage.go`: inyección del bucket privado para evidencias y media.
- `internal/adapters/cloudstorage/media_store.go`: WebP procesado, escrituras `DoesNotExist`, SHA-256 y generación exacta; lecturas/borrados condicionados a generación. Validación de namespace `media/v1/...`.
- `internal/media/domain.go`: entrada máxima **10 MiB**, 25 millones de píxeles, lado máximo 12000, **10 imágenes actuales**. El requisito de 30 aún exige ampliar Go/SQL.
- `internal/media/service.go`: reserva/idempotencia, tokens de fencing y finalización; no sustituirlo por una subida directa no autorizada.
- `internal/publiccatalog/service.go:76-96`: entrega de variantes reales con revalidación de publicación y digest, sin abrir IAM del bucket.
- `internal/publiccatalog/transport/http/handler.go:43`: `Cache-Control: no-store` actual.

Una precondición de no existencia protege de sobrescritura; la idempotencia del flujo completo depende además del registro de reserva, hash y resultado en PostgreSQL.

## 5. Decisión recomendada y alternativas no adoptadas

1. **Inicial:** reutilizar el bucket privado activo y organizar objetos por ID de usuario/espacio mediante un namespace nuevo y compatible. No crear un bucket por usuario ni uno por espacio; la autorización sigue siendo Go + DB, no el nombre del prefijo.
2. **No convertir imágenes de borradores en objetos públicos.** Mantener entrega del propietario por BFF y entrega de imágenes publicadas por el endpoint público de Go que valida su publicación. No hacen falta CORS ni un bucket/CDN nuevo para el flujo `Next BFF → Go → GCS` actual.
3. No almacenar masters RAW/originales por defecto: conservar el comportamiento de sanitizar y guardar derivados. Una conservación de originales necesitaría finalidad y retención explícitas.
4. Separar documentos a otro bucket solo si las políticas de acceso/retención lo requieren y tras revisar el plan Terraform; reservar ahora su namespace y contrato privado no habilita KYC arbitrario.
5. `timestamp UTC + UUID + fencing/version` por nueva carga; IDs/tiempo emitidos por Go. **No usar posición de galería en la clave**, porque reordenar no debe mover objetos. Los reintentos de la misma carga reutilizan su reserva/clave, no generan archivos cada vez.
6. Mantener lectura de `media/v1` y nuevo namespace; no ejecutar `mv`. Si se necesita copiar: precondiciones de generación, digest, actualización transaccional de referencia, gracia y limpieza diferida.
7. No cambiar lifecycle general ni soft delete/versionado por este plan. Un TTL de staging, no actuales o documentos necesita una política aprobada y debe excluir objetos todavía referenciados, retenidos o sujetos a restauración. No se adoptan los plazos exploratorios de 3/30 días del primer borrador de auditoría.
8. CDN, cache pública anual, tres buckets nuevos, signed URLs y CMEK son **alternativas futuras**, no prerrequisitos ni cambios autorizados. Las propuestas iniciales de nuevos buckets públicos y slots numéricos en rutas quedan sustituidas por el plan principal.

## 6. Referencia de implementación

La propuesta ejecutable consolidada y las fases de validación están en `../plan_creacion_espacios_galeria_documentos.md`. Esta auditoría no acredita una subida real ni una migración de datos. No se aplicaron cambios de infraestructura.
