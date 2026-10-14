# LOCAL-DOCKER — ejecución local reproducible en Compose

Fecha de inicio: 2026-10-11. LOCAL-1 fue aceptado explícitamente por el propietario dentro del alcance sintético ratificado; #246 está cerrada/Hecho en Projects. LOCAL-DOCKER es una fase local posterior de empaquetado, instalación reproducible y recuperación. No vuelve a abrir LOCAL-1 ni cierra Issues padre con criterios generales/productivos.

## Conciliación de estado a 2026-10-14

- FE-15 sustituyó el mock por Angular como frontend predeterminado en PR #292, merge `5405a82e90bc429aa89666ead430fe2cb39f102e`. Este documento y la evidencia DOCKER-02 de 2026-10-11 describen el stack/mock de esa fecha; el próximo ensayo debe recorrer el frontend Angular vigente y conservar los registros históricos sin reescribirlos.
- **DOCKER-01/#268:** cerrado y aceptado en su alcance tras PR #271, merge `e074d78f126cd3a5bbe9c108da26178bdb726ad4`.
- **DOCKER-02/#269:** cerrada administrativamente el 2026-10-12, pero no aceptada funcionalmente. Su comentario de cierre dice que faltan liquidación post-plazo, estado `cerrada`, historial, comprobante e idempotencia. La issue padre #267 permanece abierta. Ver matriz vigente en [`cierre_local/matriz_cierre_local.md`](cierre_local/matriz_cierre_local.md).
- **DOCKER-03/#270:** cerrada y aceptada en su alcance tras PR #273, merge `b0f62b976c9586d260cea15ed9acc29ab12257a7`. Su aceptación no cierra DOCKER-02.
- La aceptación de UX/demo MPV2-13 registrada para código `736f81a` no constituye aceptación retroactiva de Docker ni del cierre local integral.

La tabla y los apartados originales describen el estado inicial/corridas fechadas del 2026-10-11. Los estados actuales de Issues se documentan arriba y en la matriz de cierre.

## Estado de Compose existente

El Compose vigente tiene servicios separados para `backend`, `angular-frontend`, `database` (PostgreSQL/PostGIS), `mailpit` y `migrate`. FE-15 retiró el mock como frontend predeterminado; no se requiere recuperarlo para cerrar Docker local. El migrador sigue como proceso y servicio separados con `espacigo_admin`; el Backend utiliza `espacigo_runtime`.

## Cortes y aceptación

| Corte | Issue | Estado al iniciar (2026-10-11) | Alcance | Criterios de aceptación |
|---|---:|---|---|---|
| DOCKER-01 — Empaquetado/configuración | #268 | Aceptado/Cerrado/Hecho | Builds independientes para API y mock; parámetros PostgreSQL y TLS; Compose aislado con proyecto, puertos, volumen, secretos y directorios propios; conservar roles runtime/migrador. | Aceptado tras PR #271 (`e074d78` en `main`). Renderer y pruebas rechazan rutas de desarrollo, usuario runtime incompatible y prefijo de imagen que colisiona antes de modificar archivos o arrancar servicios. Evidencia: `evidence/local-docker-01-20261011.md`. |
| DOCKER-02 — Instalación limpia y recorrido integral | #269 | En revisión; falta el paso temporal de liquidación/cierre | Arranque del proyecto aislado en volumen vacío y recorrido sintético integral sobre los servicios Compose ya existentes. | Se verificó un volumen nuevo, 54 migraciones desde vacío, permisos separados, servicios saludables y un recorrido de reserva mediante mock/API/PostgreSQL con mensajes Mailpit. Falta comprobar liquidación, cierre, historial, comprobante e idempotencia al cumplirse las 24 horas. Evidencia: `evidence/local-docker-02-20261011.md`. #269 sigue abierta. |
| DOCKER-03 — Recuperación y PostgreSQL sustituible | #270 | En revisión mediante PR #273; autorizado el 2026-10-11 en paralelo | Reinicios de Backend/workers; PostgreSQL/PostGIS local alternativa con TLS y usuario runtime separado; restauración aislada de DB/archivos y reaplicación del registro externo de bajas antes de exponer la API. | Corrida limpia aislada con worker real tras reinicio, operación financiera durable, TLS confiable/no confiable, migración/roles, restore y archivos verificados. Evidencia: `evidence/local-docker-03-20261011.md`. Esta autorización no acepta DOCKER-02 ni cierra #269. |

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

El siguiente registro corresponde únicamente al recorrido histórico con mock del 2026-10-11. Su evidencia permanece válida para esa corrida, no acredita el frontend Angular posterior ni completa los criterios post-liquidación que quedaron sin verificar.

PR #271 quedó integrado en `main` (`e074d78`), #268 fue aceptada por su alcance y DOCKER-02 se ejecuta en un proyecto nuevo. El proyecto Compose asigna tags de imagen separados por prefijo además de separar nombres de contenedores: reutilizar el tag de desarrollo haría que `up --build` reemplazara la referencia de imagen usada por el proyecto activo en una recreación futura. El helper usa el nombre de proyecto como prefijo y rechaza explícitamente `espacigo-local` antes de crear estado privado o llamar a Docker.

La instalación comprobada usa el proyecto `espacigo_docker02_20261011`, el prefijo de imágenes homónimo, los puertos `18080`, `18081` y `18025`, el volumen `espacigo_docker02_pgdata_20261011` y el estado `.local/docker-isolated/docker02-20261011`. Ese volumen estaba ausente antes de `up`; `migrate` inicializó esquema y grants antes del Backend. La evidencia registra 54/54 migraciones, los grants efectivos, el recorrido y las limitaciones temporales.

Para repetir una instalación limpia asigna nombres nuevos para proyecto, prefijo, volumen, ruta privada y puertos libres. `up` sobre el mismo conjunto es actualización idempotente, no instalación limpia. No reutilices nombres del desarrollo ni borres el volumen DOCKER-02 como limpieza rutinaria. La plantilla de variables y los pasos exactos están en [`evidence/local-docker-02-20261011.md`](evidence/local-docker-02-20261011.md).

El mock queda disponible en `http://127.0.0.1:18081/`; API y Mailpit en `http://127.0.0.1:18080/` y `http://127.0.0.1:18025/`. La liquidación conserva la regla de check-out + 24 horas: la llamada HTTP antes del límite devolvió el conflicto esperado; la ruta de liquidación y sus casos de timeout/reinicio/idempotencia pasaron en la integración obligatoria con reloj sustituible del runner. Este smoke no adelanta el reloj ni edita fechas de reservas para forzar un resultado.

## DOCKER-03 — recuperación, TLS y restore aislado

El propietario autorizó el 2026-10-11 iniciar #270 mientras #269 sigue abierta por su paso de liquidación/cierre tras 24 horas. Esto es una ejecución paralela aislada; no modifica ni sustituye el criterio de aceptación de DOCKER-02. La validación completa de DOCKER-03, recursos y comandos está en [`evidence/local-docker-03-20261011.md`](evidence/local-docker-03-20261011.md).

El stack aceptado de prueba usa un proyecto, tres volúmenes PostgreSQL separados (fuente, restore y pruebas), puertos, CA/certificado, archivos privados y secretos propios bajo `~/.local/state/espacigo/docker03-20261011r8`. El Backend y migrador conectan con `verify-full`; el migrador conserva `espacigo_admin` y la aplicación `espacigo_runtime`. Una CA independiente fue rechazada tanto por el cliente TLS de Postgres como por una instancia temporal del Backend. El restore conserva DB y árbol privado de forma aislada, requiere aplicar el sidecar externo y un marcador con hash antes de arrancar el Backend restaurado. El sidecar de esta corrida limpia tenía cero bajas completadas; la reaplicación no vacía está cubierta por la integración PostgreSQL `TestLocalSuppressionReplayAfterPgDumpRestore`.

El runner DOCKER-03 crea nombres nuevos con sufijo por corrida. No repite `dump` sobre un archivo existente y no tiene un comando que borre volúmenes o secretos. Los servicios de desarrollo y DOCKER-02 permanecen independientes. No se probó GCP/Cloud SQL ni se afirma compatibilidad con esos destinos.

## Límites

- No se añaden reglas de negocio ni proveedores reales.
- No se afirma compatibilidad con Cloud SQL ni con GCP; TLS configurable es únicamente una capacidad local de conexión.
- LOCAL-1 está aceptado; la validación Docker es una fase posterior. DOCKER-02 ejecuta la instalación/recorrido integral de esta fase; DOCKER-03 cubre reinicios y PostgreSQL sustituible.
- El stack persistente de desarrollo conserva su proyecto, volumen, datos y secretos actuales.
