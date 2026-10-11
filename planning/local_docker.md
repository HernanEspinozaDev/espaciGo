# LOCAL-DOCKER — ejecución local reproducible en Compose

Fecha de inicio: 2026-10-11. LOCAL-1 fue aceptado explícitamente por el propietario dentro del alcance sintético ratificado; #246 está cerrada/Hecho en Projects. LOCAL-DOCKER es una fase local posterior de empaquetado, instalación reproducible y recuperación. No vuelve a abrir LOCAL-1 ni cierra Issues padre con criterios generales/productivos.

## Estado de Compose existente

El repositorio ya tiene servicios separados para `backend`, `mock-frontend`, `database` (PostgreSQL/PostGIS), `mailpit` y `migrate`. Este plan reutiliza esa estructura. El migrador sigue como proceso y servicio separados; utiliza `espacigo_admin`. El Backend utiliza `espacigo_runtime`. El mock solo alcanza la red de API y no recibe credenciales de base de datos.

## Cortes y aceptación

| Corte | Issue | Estado inicial | Alcance | Criterios de aceptación |
|---|---:|---|---|---|
| DOCKER-01 — Empaquetado/configuración | #268 | Aceptado/Cerrado/Hecho | Builds independientes para API y mock; parámetros PostgreSQL y TLS; Compose aislado con proyecto, puertos, volumen, secretos y directorios propios; conservar roles runtime/migrador. | Aceptado tras PR #271 (`e074d78` en `main`). Renderer y pruebas rechazan rutas de desarrollo, usuario runtime incompatible y prefijo de imagen que colisiona antes de modificar archivos o arrancar servicios. Evidencia: `evidence/local-docker-01-20261011.md`. |
| DOCKER-02 — Instalación limpia y recorrido integral | #269 | En revisión mediante PR #272 | Arranque del proyecto aislado en volumen vacío y recorrido sintético integral sobre los servicios Compose ya existentes. | Se verificó un volumen nuevo, 54 migraciones desde vacío, permisos separados, servicios saludables y un recorrido de reserva mediante mock/API/PostgreSQL con mensajes Mailpit. Pruebas y límites concretos: `evidence/local-docker-02-20261011.md`. DOCKER-03 sigue bloqueado hasta que DOCKER-02 sea aceptado. |
| DOCKER-03 — Recuperación y PostgreSQL sustituible | #270 | Bloqueado hasta DOCKER-01/02 | Reinicios de Backend/workers y conexión del Backend a una instancia PostgreSQL local alterna configurable; restauración únicamente en recursos desechables. | Trabajo durable sobrevive/reintenta sin duplicados tras reinicios; la conexión se redirige a una base alterna por configuración, con credenciales/TLS propios; migraciones y usuario runtime conservan separación; evidencia identifica datos/volúmenes descartables y no toca desarrollo. |

Issue contenedora: #267. El orden de dependencias está registrado en GitHub Projects; una integración pendiente de Project no bloquea el trabajo local autorizado. Los padres funcionales y de producto permanecen abiertos según sus criterios.

## DOCKER-01 — ejecución focalizada

Desde la raíz del repositorio:

```sh
go test ./cmd/api -run 'TestLoadConfig' -count=1
bash -n scripts/local-docker-stack.sh scripts/test-local-docker-01.sh scripts/dev-env.sh
scripts/test-local-docker-01.sh
```

El test DOCKER-01 renderiza Compose dev y aislado, verifica proyecto/puertos/volumen/secrets/TLS, comprueba el rol runtime del Backend y el rol administrativo del migrador, y confirma que el mock no tiene credenciales ni red de datos. Luego construye `api` y `mock` como targets separados. El script etiqueta temporalmente las imágenes de prueba y retira solo esas etiquetas al terminar; reutiliza capas y cachés. No inicia contenedores ni monta/elimina `espacigo_pgdata`.

La ejecución aislada futura se administra con:

```sh
scripts/local-docker-stack.sh config
scripts/local-docker-stack.sh build-backend
scripts/local-docker-stack.sh build-mock
scripts/local-docker-stack.sh up
scripts/local-docker-stack.sh status
scripts/local-docker-stack.sh down
```

Los valores por defecto están separados de desarrollo: proyecto `espacigo_docker01`, puertos 18080/18081/18025, volumen `espacigo_docker01_pgdata` y estado privado `.local/docker-isolated/`. Se pueden parametrizar mediante `LOCAL_DOCKER_*`; los parámetros TLS son `DATABASE_SSLMODE`, `DATABASE_SSLROOTCERT`, `DATABASE_SSLCERT` y `DATABASE_SSLKEY`. Los certificados/keys locales se guardan en el subdirectorio privado `database-tls` y se montan read-only bajo `/run/database-tls`. El Backend construye su URL PostgreSQL con sslmode/certificados configurados; el default de desarrollo continúa `disable` para preservar el entorno existente. Los cuatro secretos sintéticos del stack aislado se generan en su propio directorio con permisos restrictivos. Antes de crear archivos o llamar a Compose, el helper resuelve symlinks/rutas reales y rechaza solapamientos con secretos, evidencias privadas, replay de privacidad o TLS del entorno de desarrollo. También rechaza un override del rol runtime; Compose fija `espacigo_runtime`. Nunca copies secretos del stack activo allí.

`down` no elimina el volumen ni secretos. No hay comando `clean` en el helper aislado. DOCKER-01 no ejecuta `up`, migraciones, purgas ni cambios en el stack actual.

## DOCKER-02 — instalación y recorrido sintético

PR #271 quedó integrado en `main` (`e074d78`), #268 fue aceptada por su alcance y DOCKER-02 se ejecuta en un proyecto nuevo. El proyecto Compose asigna tags de imagen separados por prefijo además de separar nombres de contenedores: reutilizar el tag de desarrollo haría que `up --build` reemplazara la referencia de imagen usada por el proyecto activo en una recreación futura. El helper usa el nombre de proyecto como prefijo y rechaza explícitamente `espacigo-local` antes de crear estado privado o llamar a Docker.

La instalación comprobada usa el proyecto `espacigo_docker02_20261011`, el prefijo de imágenes homónimo, los puertos `18080`, `18081` y `18025`, el volumen `espacigo_docker02_pgdata_20261011` y el estado `.local/docker-isolated/docker02-20261011`. Ese volumen estaba ausente antes de `up`; `migrate` inicializó esquema y grants antes del Backend. La evidencia registra 54/54 migraciones, los grants efectivos, el recorrido y las limitaciones temporales.

Para repetir una instalación limpia asigna nombres nuevos para proyecto, prefijo, volumen, ruta privada y puertos libres. `up` sobre el mismo conjunto es actualización idempotente, no instalación limpia. No reutilices nombres del desarrollo ni borres el volumen DOCKER-02 como limpieza rutinaria. La plantilla de variables y los pasos exactos están en [`evidence/local-docker-02-20261011.md`](evidence/local-docker-02-20261011.md).

El mock queda disponible en `http://127.0.0.1:18081/`; API y Mailpit en `http://127.0.0.1:18080/` y `http://127.0.0.1:18025/`. La liquidación conserva la regla de check-out + 24 horas: la llamada HTTP antes del límite devolvió el conflicto esperado; la ruta de liquidación y sus casos de timeout/reinicio/idempotencia pasaron en la integración obligatoria con reloj sustituible del runner. Este smoke no adelanta el reloj ni edita fechas de reservas para forzar un resultado.

## Límites

- No se añaden reglas de negocio ni proveedores reales.
- No se afirma compatibilidad con Cloud SQL ni con GCP; TLS configurable es únicamente una capacidad local de conexión.
- LOCAL-1 está aceptado; la validación Docker es una fase posterior. DOCKER-02 ejecuta la instalación/recorrido integral de esta fase; DOCKER-03 cubre reinicios y PostgreSQL sustituible.
- El stack persistente de desarrollo conserva su proyecto, volumen, datos y secretos actuales.
