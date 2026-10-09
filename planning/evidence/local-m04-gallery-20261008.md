# LOCAL-M04-GALLERY-01 — galería sintética privada de espacios propios

Issue hija #210 de #56, relacionada con el cierre parcial de #55. PR #211 abierto para revisión; la corrección se publica sobre su misma rama. No completa M04 ni publica espacios en el catálogo general.

## Contrato del corte

- Solo el Backend crea `synthetic-png-v1`; no recibe bytes del navegador. El contenido vive en el almacenamiento privado local fuera del repositorio y de rutas estáticas públicas.
- Cada listado, metadato y contenido vuelve a autorizar rol anfitrión, cuenta activa y titularidad del espacio. Una cuenta no puede leer archivos de espacios ajenos.
- Se admiten hasta diez imágenes activas por espacio. Bloqueo de espacio serializa inserciones concurrentes; clave de idempotencia reutiliza el elemento anterior. El mock muestra límite y señala que se trata de una galería privada de ensayo.
- Retirar conserva metadata/historial y encola borrado del archivo; los fallos de almacenamiento quedan para reintento. La baja existente encola estos archivos dentro de la transacción y el ZIP de identidad incluye solo imágenes activas propias con exclusiones explícitas.
- No se añade plazo de retención. El tiempo de retención de metadata histórica de galerías de espacios queda como criterio de privacidad/producto por ratificar; no se afirma borrado de respaldos ni anonimización.

## Cambios incluidos

- V32 crea `espacio_galeria_sintetica_local` con FK compuesta de espacio/titular, hashes, estado y columnas de limpieza, más índice único compuesto de ownership en `espacio`. V32 conserva su checksum.
- V33 añade una tabla durable de candidatos de archivo. Cada UUID se registra antes de escribir el PNG. Una transacción de productor bloquea la fila candidata antes de `Put` y la mantiene hasta asociar la fotografía o encolar limpieza; el worker usa `FOR UPDATE SKIP LOCKED`, por lo que no borra un archivo mientras el productor escribe aunque el candidato ya supere el umbral de antigüedad. La alta exitosa elimina el candidato en la misma transacción que inserta la fotografía; rechazos y reintentos idempotentes dejan limpieza durable. El worker reclama trabajos con lease, reintenta fallos y recupera candidatos reservados antiguos tras reiniciar o caer el productor. Si `Commit` da un resultado ambiguo, solo se reclaman candidatos que siguen en la tabla: una fila consumida por un alta confirmada ya no se borra.
- Dominio, repositorio PostgreSQL, HTTP/JSON, permisos explícitos de `espacigo_runtime`, OpenAPI y conexión al almacenamiento privado existente.
- El preparador de exportación ZIP añade la sección `synthetic_space_gallery` y PNG propios; la baja local encola binarios de galería sin borrado en cascada.
- Mock: botones por espacio propio, generación sintética, consulta autenticada/preview, retirada, máximo visible y limpieza al perder sesión. No hay selector de archivos.

## Evidencia de esta rama

- `GO_TEST_RUN='TestSyntheticGalleryOwnerLimitIdempotencyAndRecoverableCleanup' bash scripts/test-m04-attributes-postgres.sh ./internal/adapters/postgres/gallery` — PASS, PostgreSQL desechable. Usa rol `espacigo_runtime`; comprueba aislamiento entre titulares, reintento idempotente, máximo de diez, serialización del último cupo y recuperación tras fallo de borrado. Una prueba determinista pausa `Put` antes de crear el archivo con un candidato ya vencido para limpieza; el worker no reclama la fila bloqueada, y tras liberar la escritura la imagen confirmada sigue accesible y el candidato desaparece. También simula un productor abandonado antes de confirmar, recrea el servicio y verifica la recuperación del archivo huérfano. Conserva las comprobaciones de fallos de borrado tras rechazo y reintento idempotente, y consulta metadatos con UUID mayúsculo por API → servicio → repositorio PostgreSQL.
- `GO_TEST_RUN='TestM02LocalSuppressionMinimizesSyntheticAccountAndRecoversFileCleanup' bash scripts/test-m04-attributes-postgres.sh ./internal/adapters/postgres/identity` — PASS con V33 en base desechable; comprueba ZIP con el archivo propio y su inclusión en la cola de limpieza de baja.
- `go test ./internal/gallery/... ./internal/adapters/postgres/gallery ./internal/dbbootstrap ./cmd/api` — PASS.
- V32 no cambió: el blob previo en `f900cda` y el archivo actual comparten Git object `4f86280fa13e67e7fb1aa48e43c65675ffb458e1`. V33 es aditiva y las pruebas PostgreSQL la aplican en bases desechables. El volumen local persistente conserva su V32 sin adelantar la migración correctiva antes de la revisión.
- `go vet ./internal/gallery/... ./internal/adapters/postgres/gallery ./internal/adapters/postgres/ownerexport ./cmd/api` — PASS.
- `npm --prefix mock run build` y `npm --prefix mock run test:gallery-session` — PASS (2 casos: descarta un blob tras logout/relogin de la misma cuenta, y descarta respuesta tras cambiar espacio).
- `python3 -c 'import yaml; yaml.safe_load(open("planning/openapi.yaml"))'` — PASS.
- Smoke con API local y cuenta sintética existente (sin imprimir credenciales): login 200; alta de imagen 201; mismo Idempotency-Key reutilizado; listado devolvió un elemento; contenido con `image/png` y firma PNG válida; retirada confirmada. Se usó un espacio activo propio. La metadata permanece por la política actual; el archivo se retiró y la limpieza del storage es recuperable. El smoke es anterior al cambio V33; las nuevas rutas de rechazo/reintento se verifican con la integración PostgreSQL desechable.
- `scripts/dev-env.sh up` reconstruyó API/mock, aplicó V32 incrementalmente y dejó PostgreSQL, backend, mock y Mailpit saludables. Se conservó el volumen `espacigo_pgdata`, secretos y otros datos sintéticos; solo se generó y retiró una imagen sintética de prueba.
- `git diff --check` — PASS.

## Dependencias y pendientes

Storage privado #47/#143, ZIP/baja #202/#203 y ownership/publicación #204/#205 están integrados. #208/#209 no bloqueaba el corte. La galería se mantiene privada y su metadata no recibe un plazo nuevo. #55/#56, #52–#61 y las Issues generales de M04 permanecen abiertas. No se ejecutó trabajo de GCP.

## Aceptación posterior al merge #211 — 2026-10-09

- Merge confirmado en `main`: `47b04cf71a1e2a6b362f401e3708aea1713938e2`. `main` se actualizó por fast-forward.
- `scripts/dev-env.sh up` aplicó V33 mediante el servicio de migración existente. `schema_migrations` confirma V32 checksum `803d2026c722795b0c008cc9ab81ac9fa796642dc83505161d614d7df0b3f994` y V33 checksum `f3b0d590ba5b92d4483c0e173a6de758f6f3b7d69bff3923950bca5bb9ee0425`; `scripts/dev-env.sh verify-http` pasó. Se conservaron `espacigo_pgdata`, secretos y datos anteriores; no se ejecutó limpieza.
- Recorrido real del mock con anfitrión sintético existente: generar una imagen en “Galería sintética”, consultar contenido PNG verificado por API autenticada, retirar y confirmar `0 de 10 imágenes sintéticas activas`. El archivo de prueba quedó retirado por el mecanismo normal de la aplicación.
- #210 se cerró y quedó **Hecho** en Projects para este slice. #52–#61 y #62–#68 siguen abiertas por sus criterios generales.
- Siguiente subentrega: #212 LOCAL-M05-DISC-01, hija de #65 y `En curso` en Projects. La brecha concreta era que búsqueda/detalle M05 consumían fixtures expresamente habilitados, no publicaciones M04 activas y elegibles. #212 reutiliza tarifa/snapshot, filtros/paginación, disponibilidad y reserva existentes; mantiene borradores/ocultos privados y no expone dirección, coordenadas, propietario ni galería. Evidencia del avance: `local-m05-published-catalog-20261009.md`.
