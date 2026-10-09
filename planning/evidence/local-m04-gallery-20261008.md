# LOCAL-M04-GALLERY-01 — galería sintética privada de espacios propios

Issue hija #210 de #56, relacionada con el cierre parcial de #55. PR #211 abierto para revisión, commit inicial `f900cda` (la revisión puede añadir commits). No completa M04 ni publica espacios en el catálogo general.

## Contrato del corte

- Solo el Backend crea `synthetic-png-v1`; no recibe bytes del navegador. El contenido vive en el almacenamiento privado local fuera del repositorio y de rutas estáticas públicas.
- Cada listado, metadato y contenido vuelve a autorizar rol anfitrión, cuenta activa y titularidad del espacio. Una cuenta no puede leer archivos de espacios ajenos.
- Se admiten hasta diez imágenes activas por espacio. Bloqueo de espacio serializa inserciones concurrentes; clave de idempotencia reutiliza el elemento anterior. El mock muestra límite y señala que se trata de una galería privada de ensayo.
- Retirar conserva metadata/historial y encola borrado del archivo; los fallos de almacenamiento quedan para reintento. La baja existente encola estos archivos dentro de la transacción y el ZIP de identidad incluye solo imágenes activas propias con exclusiones explícitas.
- No se añade plazo de retención. El tiempo de retención de metadata histórica de galerías de espacios queda como criterio de privacidad/producto por ratificar; no se afirma borrado de respaldos ni anonimización.

## Cambios incluidos

- V32 incremental crea `espacio_galeria_sintetica_local` con FK compuesta de espacio/titular, hashes, estado y columnas de limpieza, más índice único compuesto de ownership en `espacio` para referenciarlo. No altera migraciones previas.
- Dominio, repositorio PostgreSQL, HTTP/JSON, permisos explícitos de `espacigo_runtime`, OpenAPI y conexión al almacenamiento privado existente.
- El preparador de exportación ZIP añade la sección `synthetic_space_gallery` y PNG propios; la baja local encola binarios de galería sin borrado en cascada.
- Mock: botones por espacio propio, generación sintética, consulta autenticada/preview, retirada, máximo visible y limpieza al perder sesión. No hay selector de archivos.

## Evidencia de esta rama

- `GO_TEST_RUN='TestSyntheticGalleryOwnerLimitIdempotencyAndRecoverableCleanup' bash scripts/test-m04-attributes-postgres.sh ./internal/adapters/postgres/gallery` — PASS, PostgreSQL desechable. Usa rol `espacigo_runtime`; comprueba aislamiento entre titulares, reintento idempotente, máximo de diez, serialización del último cupo y recuperación tras fallo de borrado.
- `GO_TEST_RUN='TestM02LocalSuppressionMinimizesSyntheticAccountAndRecoversFileCleanup' bash scripts/test-m04-attributes-postgres.sh ./internal/adapters/postgres/identity` — PASS, PostgreSQL desechable. Comprueba ZIP con el archivo propio y su inclusión en la cola de limpieza de baja.
- `go test ./internal/gallery/... ./internal/adapters/postgres/gallery ./internal/dbbootstrap ./internal/adapters/postgres/ownerexport ./cmd/api` — PASS.
- `go vet ./internal/gallery/... ./internal/adapters/postgres/gallery ./internal/adapters/postgres/ownerexport ./cmd/api` — PASS.
- `npm --prefix mock run build` y `npm --prefix mock run test:gallery-session` — PASS (2 casos: descarta un blob tras logout/relogin de la misma cuenta, y descarta respuesta tras cambiar espacio).
- `python3 -c 'import yaml; yaml.safe_load(open("planning/openapi.yaml"))'` — PASS.
- Smoke con API local y cuenta sintética existente (sin imprimir credenciales): login 200; alta de imagen 201; mismo Idempotency-Key reutilizado; listado devolvió un elemento; contenido con `image/png` y firma PNG válida; retirada confirmada. Se usó un espacio activo propio. La metadata permanece por la política actual; el archivo se retiró y la limpieza del storage es recuperable.
- `scripts/dev-env.sh up` reconstruyó API/mock, aplicó V32 incrementalmente y dejó PostgreSQL, backend, mock y Mailpit saludables. Se conservó el volumen `espacigo_pgdata`, secretos y otros datos sintéticos; solo se generó y retiró una imagen sintética de prueba.
- `git diff --check` — PASS.

## Dependencias y pendientes

Storage privado #47/#143, ZIP/baja #202/#203 y ownership/publicación #204/#205 están integrados. #208/#209 no bloqueaba el corte. La galería se mantiene privada y su metadata no recibe un plazo nuevo. #55/#56, #52–#61 y las Issues generales de M04 permanecen abiertas. No se ejecutó trabajo de GCP.
