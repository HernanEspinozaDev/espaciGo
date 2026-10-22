# IMG-01 — matriz de capacidades, controles, pruebas y cortes

- Estado: propuesta autorizada para revisión; las filas de pruebas son criterios futuros, no resultados.
- Base de diseño: [`plan_img_01_carga_crud.md`](plan_img_01_carga_crud.md) y [`adr_img_01_imagenes_webp.md`](adr_img_01_imagenes_webp.md).
- Base de código inspeccionada: `dc58d732a7e2883c3fe2f6dc1c365cb5026cf5f6`; V67 última migración y V68 siguiente libre a esa fecha.

## Inventario y brechas

| Capacidad | Actual en la base | IMG-01 propuesta | Restricción |
| --- | --- | --- | --- |
| Galería de espacio sintética | V32/V33, `internal/gallery`, `internal/adapters/postgres/gallery`; PNG fijo, propietario, candidato durable y cleanup | Mantener como una fuente; DTO compuesto con media real | No cambiar fixture/MIME histórico; migración aditiva; un cupo total 10. |
| Portada sintética | V49 `espacio.portada_foto_id` y FK a fila sintética | Selección común para filas sintéticas/reales | FK y selección histórica intactas; nueva referencia con exclusión mutua. |
| Publicación de foto | V56 bandera booleana explícita; actual catálogo minimiza y filtra | Seguir decisión explícita para display/thumbnail reales | Nunca inferir público desde activo; nueva foto privada hasta acción confirmada. |
| Perfil sintético | V30 y `GET /api/v1/profile/photo/content` de la propia cuenta activa | Avatar WebP reemplazable dentro de alcance actual | No permitir público/otra cuenta/administración en esta entrega. |
| Blob GCS | Bucket privado ya enlazado a GCP-02, prefix histórico `g03o-1017`; objeto con UUID | Namespace dedicado `media/v1` y generation por variante | No cambiar bucket, IAM, prefix global, soft delete ni versionado. |
| UI | Hosting/account consumen endpoints sintéticos | Añadir CRUD/carga múltiple por feature existente | Un archivo/request, concurrencia cliente 2, sin HTTP desde presentación. |
| Exportación/supresión | Flujos de privacidad existentes reconocen archivos sintéticos | Enumerar ambos modelos, avatar, cada variante y generación | Export correcto MIME/dimensiones/tamaño; supresión revoca API y cleanup asíncrono. |

## Capacidades y pruebas futuras

`P` indica permitir/resultado correcto esperado; `N` denegar, rechazar o conservar estado. Son casos que el corte debe implementar y ejecutar.

| Área/caso | Propietario activo | Otro propietario | Anónimo | Cuenta bloqueada/inactiva | Prueba/criterio |
| --- | --- | --- | --- | --- | --- |
| Lista/lee la galería propia | P en ambas fuentes; DTO unificado | N/404 minimizado | N para inventario privado | N | Sintéticas siguen apareciendo sin duplicación; metadata ordenada y fuente privada separada. |
| Sube imagen real | P en publicación propia y dentro del cupo común | N | N | N | POST multipart; actor, sesión, recurso y estado revalidados en Go. |
| Carga sintética existente | P bajo misma regla/cuota | N | N | N | 9 existentes + carga real y sintética concurrentes: solo una obtiene el cupo 10. |
| Reintenta operación | P si misma clave/digest/parámetros | N | N | N | Misma operación devuelve ID/resultado; clave reutilizada con bytes distintos da 409 y no altera activos. |
| Edita orden/alt/portada/publicación | P con versión vigente | N | N | N | conflicto de versión devuelve 409; solo una portada entre ambos orígenes; default no público. |
| Lee catálogo | — | — | P solo espacio publicado + foto `ready` + variante display/thumbnail explícita | — | Lo no publicado, deleting, avatar y query/path/variant inválidos reciben N; JSON/URLs no filtran bucket, object key ni generation. |
| Lee avatar | P por `/profile/photo/content` actual | N | N | N | Response privado `no-store`; mantiene MIME/hash correctos y puntero anterior ante fallo de reemplazo. |
| Elimina foto | P; baja lógica revoca API en commit | N | N | N | inmediato no-cache/read-denied; cleanup posterior reintentable con generation exacta; slot se libera en baja lógica. |
| Exporta/suprime identidad | según el flujo privado autorizado | N | N | según política vigente | Incluye todas las variantes y both sources; MIME/extensión real, no fijo `.png`; sin claves internas. |

## Validación técnica por frontera

| Frontera | Casos mínimos | Evidencia futura requerida |
| --- | --- | --- |
| Migración / Postgres | DB vacía; upgrade V67→V68; V1–V67 hashes intactos; owner FK; grants runtime exactos; cupo/portada común; reserva concurrente; fencing expirado/reclamado | Comandos ejecutados, versión PostgreSQL/PostGIS, SHA de esquema, dump de constraints/grants allowlisted. Ninguna escritura Cloud SQL. |
| Go / multipart / OpenAPI | MIME/extension falsos, firma real, archivos corruptos/truncados, límites bytes/píxeles/dimensiones, static WebP; error si GIF/SVG/vídeo; orientación EXIF, strip GPS/EXIF; round-trip; calidad/peso/tiempo/heap; idempotencia igual/distinto; revocación durante carga; GCS/DB falla/commit ambiguo | `api:check`, unitarias, integración Postgres aislado, adaptador GCS fake/contract y build `CGO_ENABLED=0` scratch. No usar solo mocks para aceptar GCS. |
| GCS / cleanup | create-only unique keys, same candidate retry, generation exacta, 404 idempotente, precondition conflict, cleanup leased, crash/restart | Bucket privado; demostrar generation y namespace propios con entorno controlado, sin mutar bucket existente durante los PR de planificación/código. |
| Egress/request path | 10 MiB + overhead multipart atravesando servicio/gateway real; sobrelímite Cloud Run/proxy; timeout; un CPU/1 GiB; concurrencia; CPU/heap/cancelación/rate-limit | Capturar configuración efectiva del revision/gateway y rendimiento con fixtures sintéticas. No cambiar Cloud Run en los cortes de código. |
| Angular / API | selección múltiple, retiro local y `URL.revokeObjectURL`, 2 concurrentes, progreso individual, error/reintento, orden/alt/cover/visibilidad/eliminar; refresh del estado desde Go | Playwright desktop y móvil con API/DB real; no usar respuesta simulada como evidencia de integración. |
| Privacidad y catálogo | anónimo ve solo publicación efectiva y variante allowlisted; avatar propio only; owner/otros; suppression/export; user block | Tabla de matriz HTTP status/DTO/caches y recorrido Angular→Go→Postgres→GCS. |
| Soft-delete y costo | eliminar una variante; GCS version/soft deleted; reintentos/concurrencia; cuota y acumulación en `media/v1` | Confirmar que API revocó lectura tras transaction y el resultado de delete conforme al bucket; documentar retención 7 días sin prometer purge inmediato. |

## Cortes secuenciales

| ID | Contenido del PR | Entrada | Salida / gate siguiente |
| --- | --- | --- | --- |
| IMG-01A | Esta planificación, ADR y matriz. Solo archivos versionados; sin DDL, runtime code o cambios Cloud. | `main` actualizado; plan del propietario. | Revisor ratifica cupo común, avatar, idempotencia, límite, generación y encoder de evaluación. Fusionar antes del corte DB. |
| IMG-01B | V68 aditiva: assets/variantes/relaciones/perfil, orden común con backfill determinista, portada real mutuamente excluyente, triggers de reserva serializados para media real y galería sintética (incluidas transiciones de estado) y ACL explícita al runtime existente. | IMG-01A fusionado. | Revisión CHANGES_REQUESTED atendida en el mismo PR: script PostgreSQL aislado cubre fresh V1–V68, upgrade V67→V68, backfill, competencia concurrente sintético/real con nueve cupos ocupados, reactivación fallida con galería llena, ACL y reapply no-op. No aplicar en Cloud SQL. Fusionar antes de Go. |
| IMG-01C | Go dominio/servicios/repositorio, multipart, conversion WebP, adaptador GCS bajo `media/v1`, read/delete/cleanup, OpenAPI + SDK; evolución de los handlers sintéticos y avatar para reglas comunes. | IMG-01B fusionado. | Pruebas de encoder/dimensión/concurrencia/fencing/authorization y build scratch. Fusionar antes de Angular. |
| IMG-01D | Angular hosting/account: selección múltiple, preview local, hasta 2 cargas concurrentes con progreso/reintento, CRUD de galería en una secuencia/cupo común, avatar privado, DTOs/adaptadores y lecturas autenticadas. | IMG-01C fusionado (`7c00d374...`). | Lint/unitarias/build y UX accesible; comprobar servicio local aislado. No presentar filesystem local ni pruebas simuladas como GCS. |
| IMG-01E | Integración real con DB/GCS aislados, cuotas/caches/supresión/export, E2E, evidencia de variantes/revocación; paquete de release. | IMG-01D fusionado. | Evidencia de bytes WebP+metadata+UI+revocación; PR revisado. Aplicación persistente, cambios Cloud o release se rigen por gate separado. |

## Deploy y estado externo (solo observación)

Read-only, 2026-10-22: `espacigo-test-api`, `espacigo-test-private` y `espacigo-test-public` informaron una revisión `Ready` con 100% de tráfico. `espacigo-test-api` tenía 1 CPU, 1 GiB, concurrencia 80 y timeout 300 s. Esto registra el estado observado que el propietario reportó funcionando, no una nueva aceptación de producto ni evidencia de manejo de imágenes.

El documento vigente [GCP-08 edge prerequisites](../gcp/gcp08_edge_activation_prerequisites_20261022.md) conserva DNS pendiente: inventario de zona/records y autorización ausentes; `manage_dns_records=false`, sin cambios. `gcloud dns managed-zones list` no pudo leer Cloud DNS porque la API está deshabilitada/permisos actuales no bastan; no se habilitó. Ninguno de los cortes IMG-01 modifica deploy, DNS, Terraform, Cloud Run, bucket, IAM, Cloud SQL ni datasets.

## Evidencia de implementación IMG-01C (fusionado en PR #528; 2026-10-22)

Base: rama `codex/marketplace/feature/img-1-01c-media-api`, iniciada desde el merge de IMG-01B `5995a5fcdbd428220cd8927499d1d33f84466980`. PR #528 fue revisado y fusionado por `HernanEspinozaDev`; HEAD aprobado `51687c20acfca7e38cd216ac40b3b940c9071798`, merge `7c00d374b53466d2b2b7e41e86635d712ddd0197`, cuatro checks `SUCCESS`. Esto cierra el corte C; no supone aceptación del producto ni evidencia de GCS o recorrido integral.

- Código añadido: V69 permite `filesystem` solo para el adaptador de almacenamiento privado local. V70 agrega intenciones de escritura persistidas antes del `Put`, con generación opcional hasta confirmar el registro de variante; V71 marca cuándo el escritor terminó para que cleanup distinga escrituras en curso de escrituras terminales sin objeto, y pueda resolver/eliminar las escrituras ambiguas que sí dejaron objeto. V1–V68 no se modificaron.
- Verificación SQL aislada: evidencia anterior cubrió base fresca V1–V70 y upgrade V1–V68→V69–V70. Para esta actualización, `pg_virtualenv` aplicó base fresca V1–V71 y, por separado, V1–V70→V71; ambos esquemas terminaron en versión 71. En base fresca con `espacigo_runtime` creado previamente se confirmó `SELECT,INSERT,UPDATE,DELETE` para `media_variant_intent`. Una prueba de repositorio reclamó primero el asset (incrementando fencing), luego marcó terminal la intención exacta y confirmó que permanece consultable para el siguiente ciclo de cleanup; las consultas en esquema vacío también pasaron. Los clústeres temporales se retiraron; no se usó Cloud SQL ni una base persistente.
- Verificación Go observada: `go test ./...` pasó. El adaptador de filesystem cubre escritura create-only, resolución de generación, rechazo de generación equivocada, path traversal y MIME. Codec tiene round-trip WebP con alpha, EXIF orientación/strip, dimensiones, APNG y fuzz: `go test ./internal/media -run '^$' -fuzz FuzzValidateInputNeverPanics -fuzztime=3s` ejecutó 384.212 casos sin fallo.
- Límite HTTP de carga: el request multipart máximo es 11 MiB; el servidor permite 2 minutos para leerlo (mínimo aproximado 94 KiB/s) y 4 minutos hasta completar respuesta, reservando otros 2 minutos para conversión/almacenamiento/DB. `TestHTTPServerTimeoutsCoverBoundedMultipartUpload` comprueba que estos plazos cubren el límite de bytes sin quitar el tope de tamaño.
- Rendimiento en este host (AMD Ryzen 7 PRO 4750U): fixture JPEG 1600x1200, 532 ms y 56,1 MB allocados; fixture JPEG 5000x5000/25 MP, 1,43 s y 140,3 MB allocados, una iteración con `-benchmem`. Son muestras locales, no una prueba sostenida Cloud Run de 1 CPU/1 GiB.
- `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o /tmp/espacigo-api-img01c ./cmd/api` pasó; `docker build --target api -t espacigo-img01c-api-scratch:local .` construyó la imagen `scratch` correctamente y se retiró el tag de prueba; `npm run api:check` regeneró SDK y pasó typecheck/contrato; `git diff --check` pasó.
- Pendiente para IMG-01E: runtime de repositorio con identidad/espacio y backend GCS, catálogo público con foto real y cobertura de exportación/supresión, E2E navegador con foto real en DB+GCS, revocación y lifecycle end-to-end. No se mutó GCS ni Cloud SQL durante C. Las verificaciones aisladas de V71 y V70→V71 quedaron completadas antes de fusionar; la integración de producto permanece separada de esa evidencia técnica.

## Evidencia de implementación IMG-01D (en preparación; 2026-10-22)

Base: worktree `codex/frontend/feature/img-1-01d-angular-media`, desde `main` tras el merge C `7c00d374b53466d2b2b7e41e86635d712ddd0197`. Código revisado/pruebas locales en `5d7257f1cf23c242fd502a9c9d48c218fa8ee18a`; la actualización documental posterior no cambia código. El trabajo incluye UI/API para galería mixta y avatar privado, pruebas de adaptador de multipart/progreso, validación de formularios y manejo de URLs temporales. La implementación sigue pendiente de revisión; no equivale a aceptación del corte.

- Verificación frontend: `npm run api:check`, `npm run build`, `npm run lint` y `npm test -- --watch=false` pasaron; suite 41 archivos/317 pruebas. La compilación local quedó bajo el presupuesto de estilos. Se añadieron pruebas para multipart Bearer+Idempotency-Key/progreso de galería y avatar, y para encolar/subir una imagen en la fachada con API simulada.
- Entorno aislado: Compose con proyecto `espacigo_img01d_20261022`, volumen exclusivo `espacigo_img01d_pgdata_20261022`, puertos 18080/18420/18025 y almacenamiento filesystem local privado. Migración y servicios arrancaron; `scripts/verify-http.py` pasó salud, CORS, SSR y runtime config. Se retiraron proyecto, contenedores, redes, volumen y archivos de secretos/ensayo; PostgreSQL persistente local, Cloud SQL y GCS no se usaron.
- Alcance del recorrido navegador: Chromium 155 se abrió headless; el flujo de registro y verificación por Mailpit en la base efímera pasó. No quedó confirmado el inicio de sesión ni la carga/lectura de avatar en navegador, y la galería de anfitrión no se alcanzó. El alta normal asigna solamente `arrendatario` (`RegistrationRoles()`); las rutas Angular del panel exigen `arrendador`. No se encontró un flujo autorizado de alta de ese rol en el entorno efímero. No se modificaron roles por SQL ni se reutilizó la base persistente.
- Pendiente de evidencia para D antes de cerrar el corte: recorrido browser autenticado con una identidad de anfitrión permitida por la política privada; subir, leer y comprobar al menos una variante de galería real por Angular→Go→PostgreSQL/filesystem. La prueba completa de perfil-avatar tampoco está confirmada en navegador. Filesystem local sirve para ejercicio aislado, no demuestra backend GCS.
- IMG-01E permanece detrás de la fusión de D y requiere pruebas DB/GCS aisladas, variantes WebP, cuotas, idempotencia/concurrencia, revocación/borrado por generación, export/supresión y evidencia de navegador. No se inició E.
