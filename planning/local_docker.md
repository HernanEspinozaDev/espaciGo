# LOCAL-DOCKER — ejecución local reproducible en Compose

Fecha de inicio: 2026-10-11. LOCAL-1 fue aceptado explícitamente por el propietario dentro del alcance sintético ratificado; #246 está cerrada/Hecho en Projects. LOCAL-DOCKER es una fase local posterior de empaquetado, instalación reproducible y recuperación. No vuelve a abrir LOCAL-1 ni cierra Issues padre con criterios generales/productivos.

## Estado de Compose existente

El repositorio ya tiene servicios separados para `backend`, `mock-frontend`, `database` (PostgreSQL/PostGIS), `mailpit` y `migrate`. Este plan reutiliza esa estructura. El migrador sigue como proceso y servicio separados; utiliza `espacigo_admin`. El Backend utiliza `espacigo_runtime`. El mock solo alcanza la red de API y no recibe credenciales de base de datos.

## Cortes y aceptación

| Corte | Issue | Estado inicial | Alcance | Criterios de aceptación |
|---|---:|---|---|---|
| DOCKER-01 — Empaquetado/configuración | #268 | En curso | Builds independientes para API y mock; parámetros PostgreSQL y TLS; Compose aislado con proyecto, puertos, volumen, secretos y directorios propios; conservar roles runtime/migrador. | El target API compila sin Node/TypeScript; el mock compila TypeScript y su servidor Go en sus etapas. El renderer prueba configuraciones dev/aislada, defaults estables, TLS, secretos y rutas privadas. Los tests no arrancan servicios ni modifican volumen/secretos de desarrollo. Los comandos aislados son reproducibles y `down` conserva datos. |
| DOCKER-02 — Instalación limpia y recorrido integral | #269 | Bloqueado hasta DOCKER-01 | Arranque del proyecto aislado en volumen vacío y recorrido sintético integral sobre los servicios Compose ya existentes. | No colisiona con proyecto, puertos, volumen, secretos o directorios de desarrollo. Aplicación de migraciones parte de vacío con el servicio `migrate`; API y mock quedan saludables; el recorrido integral publicado es reproducible; se adjunta evidencia de aislamiento y limpieza acotada del stack aislado, sin borrar secretos ni volumen salvo instrucción específica. Esta es la validación integral de Docker. |
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

Los valores por defecto están separados de desarrollo: proyecto `espacigo_docker01`, puertos 18080/18081/18025, volumen `espacigo_docker01_pgdata` y estado privado `.local/docker-isolated/`. Se pueden parametrizar mediante `LOCAL_DOCKER_*`; los parámetros TLS son `DATABASE_SSLMODE`, `DATABASE_SSLROOTCERT`, `DATABASE_SSLCERT` y `DATABASE_SSLKEY`. Los certificados/keys locales se guardan en el subdirectorio privado `database-tls` y se montan read-only bajo `/run/database-tls`. El Backend construye su URL PostgreSQL con sslmode/certificados configurados; el default de desarrollo continúa `disable` para preservar el entorno existente. Los cuatro secretos sintéticos del stack aislado se generan en su propio directorio con permisos restrictivos. Nunca copies secretos del stack activo allí.

`down` no elimina el volumen ni secretos. No hay comando `clean` en el helper aislado. DOCKER-01 no ejecuta `up`, migraciones, purgas ni cambios en el stack actual.

## Límites

- No se añaden reglas de negocio ni proveedores reales.
- No se afirma compatibilidad con Cloud SQL ni con GCP; TLS configurable es únicamente una capacidad local de conexión.
- No se ejecuta la suite integral de LOCAL-1 para cada corte Docker. La validación completa corresponde a la instalación/recorrido DOCKER-02 y a recuperación DOCKER-03.
- El stack persistente de desarrollo conserva su proyecto, volumen, datos y secretos actuales.
