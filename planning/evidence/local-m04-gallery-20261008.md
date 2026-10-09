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
- V33 añade una tabla durable de candidatos de archivo. Cada UUID se registra antes de escribir el PNG. La alta exitosa elimina ese candidato dentro de la misma transacción que inserta la fotografía; los rechazos y resultados idempotentes reutilizados dejan el candidato en la cola durable. El worker reclama trabajos con lease, reintenta fallos y recupera candidatos reservados antiguos tras reiniciar. Si el `Commit` da un resultado ambiguo, la cola solo puede reclamar candidatos que siguen en la tabla: una fila consumida por una alta confirmada ya no se borra.
- Dominio, repositorio PostgreSQL, HTTP/JSON, permisos explícitos de `espacigo_runtime`, OpenAPI y conexión al almacenamiento privado existente.
- El preparador de exportación ZIP añade la sección `synthetic_space_gallery` y PNG propios; la baja local encola binarios de galería sin borrado en cascada.
- Mock: botones por espacio propio, generación sintética, consulta autenticada/preview, retirada, máximo visible y limpieza al perder sesión. No hay selector de archivos.

## Evidencia de esta rama

- `GO_TEST_RUN='TestSyntheticGalleryOwnerLimitIdempotencyAndRecoverableCleanup' bash scripts/test-m04-attributes-postgres.sh ./internal/adapters/postgres/gallery` — PASS, PostgreSQL desechable. Usa rol `espacigo_runtime`; comprueba aislamiento entre titulares, reintento idempotente, máximo de diez, serialización del último cupo y recuperación tras fallo de borrado. Añade fallos de borrado después de un alta rechazada por límite y de un reintento idempotente; al limpiar con un servicio recreado recupera la cola y elimina solo los archivos candidatos, preservando el archivo de la foto confirmada. También consulta los metadatos con UUID mayúsculo a través de API → servicio → repositorio PostgreSQL.
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
